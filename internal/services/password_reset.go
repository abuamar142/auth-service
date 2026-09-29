package services

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// Reset token lifetime. Short because the token is a full account takeover
// while it is valid, and long enough that someone can find the email and
// act on it without rushing.
const passwordResetTTL = 1 * time.Hour

var (
	ErrInvalidResetToken = errors.New("invalid or expired reset token")
	ErrEmailRequired     = errors.New("email is required")
	ErrWeakPassword      = errors.New("password must be at least 8 characters")
)

// EmailSender is what the service needs from an email provider. An interface
// so tests can assert what would have been sent, and so a missing provider
// configuration degrades to a no-op rather than a panic.
type EmailSender interface {
	SendPasswordReset(to, token string) error
}

// ForgotPassword issues a reset token and emails it.
//
// Returns nil whether or not the address belongs to an account. That is
// deliberate: a form that answers differently for known and unknown emails is
// an enumeration oracle, and this endpoint is unauthenticated. The caller
// cannot tell the difference, and neither can anyone probing for addresses.
func (s *AuthService) ForgotPassword(ctx context.Context, email string) error {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return ErrEmailRequired
	}

	var userID string
	err := s.DB.QueryRow(ctx, `SELECT id FROM users WHERE lower(email) = $1`, email).Scan(&userID)
	if err != nil {
		// Unknown address, or a lookup failure. Both look the same from
		// outside, so neither is reported.
		slog.Info("password reset requested", "found", err == nil)
		return nil
	}

	raw, hash, err := generateResetToken()
	if err != nil {
		return fmt.Errorf("generating reset token: %w", err)
	}

	// Newest-wins: any outstanding token is dropped first, so at most one is
	// ever valid. Otherwise a user who requests three resets leaves three
	// working tokens in circulation.
	if _, err := s.DB.Exec(ctx, `DELETE FROM password_reset_tokens WHERE user_id = $1`, userID); err != nil {
		return fmt.Errorf("clearing old reset tokens: %w", err)
	}

	_, err = s.DB.Exec(ctx,
		`INSERT INTO password_reset_tokens (user_id, token_hash, expires_at) VALUES ($1, $2, $3)`,
		userID, hash, time.Now().Add(passwordResetTTL),
	)
	if err != nil {
		return fmt.Errorf("saving reset token: %w", err)
	}

	// The token exists at this point; a delivery failure is logged and the
	// request still reports success. Reporting it would tell the caller that
	// the address exists, which is the thing this function avoids.
	if s.Email != nil {
		if err := s.Email.SendPasswordReset(email, raw); err != nil {
			slog.Error("failed to send reset email", "error", err)
		}
	} else {
		slog.Warn("no email sender configured — reset email not sent")
	}

	return nil
}

// ResetPassword consumes a token and sets a new password.
//
// The token is single-use: it is marked used inside the same statement that
// finds it, so two requests racing on one token cannot both succeed.
func (s *AuthService) ResetPassword(ctx context.Context, rawToken, newPassword string) error {
	if len(newPassword) < 8 {
		return ErrWeakPassword
	}
	if strings.TrimSpace(rawToken) == "" {
		return ErrInvalidResetToken
	}

	hash := hashToken(rawToken)

	var userID string
	// used_at IS NULL and the expiry are part of the WHERE clause rather than
	// checks afterwards: a token that fails either condition simply matches
	// no row, so there is no window between reading and using it.
	err := s.DB.QueryRow(ctx, `
		UPDATE password_reset_tokens
		SET used_at = now()
		WHERE token_hash = $1 AND used_at IS NULL AND expires_at > now()
		RETURNING user_id
	`, hash).Scan(&userID)
	if err != nil {
		return ErrInvalidResetToken
	}

	bcryptHash, err := bcrypt.GenerateFromPassword([]byte(newPassword), s.BcryptCost)
	if err != nil {
		return fmt.Errorf("hashing password: %w", err)
	}

	if _, err := s.DB.Exec(ctx,
		`UPDATE users SET password_hash = $2, updated_at = now() WHERE id = $1`,
		userID, string(bcryptHash),
	); err != nil {
		return fmt.Errorf("updating password: %w", err)
	}

	// Every session is invalidated. A password reset is usually a response to
	// "someone else may have my account", and leaving existing refresh tokens
	// alive would keep that someone logged in.
	if _, err := s.DB.Exec(ctx,
		`UPDATE refresh_tokens SET revoked = true WHERE user_id = $1 AND revoked = false`,
		userID,
	); err != nil {
		slog.Error("failed to revoke sessions after reset", "error", err)
	}

	return nil
}

// generateResetToken returns the token to email and the hash to store.
//
// 32 bytes from crypto/rand: long enough that guessing is not a strategy, and
// the raw value never touches the database.
func generateResetToken() (raw, hash string, err error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", "", err
	}
	raw = hex.EncodeToString(b)
	return raw, hashToken(raw), nil
}
