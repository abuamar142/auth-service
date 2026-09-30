package services

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/bcrypt"

	"github.com/abuamar142/auth-service/internal/models"
)

// Errors this file can return. Named so handlers can map them to status codes
// without matching on message text.
var (
	ErrCannotDeleteSelf = errors.New("cannot delete your own account")
	ErrCannotDemoteSelf = errors.New("cannot remove your own admin rights")
	ErrLastAdmin        = errors.New("at least one admin must remain")
	ErrPasswordTooShort = errors.New("password must be at least 8 characters")
	ErrNothingToUpdate  = errors.New("no fields to update")
	ErrUserIDRequired   = errors.New("user id is required")
)

// userColumns is the SELECT list every user read shares.
//
// Written once because a column added to the struct but missed in one query
// produces a zero value that looks like real data — is_admin false would read
// as "not an admin" and silently lock someone out of the page.
const userColumns = `id, email, username, password_hash, display_name, is_admin, created_at, updated_at`

func scanUser(row interface {
	Scan(dest ...any) error
}) (*models.User, error) {
	var u models.User
	err := row.Scan(&u.ID, &u.Email, &u.Username, &u.PasswordHash, &u.DisplayName, &u.IsAdmin, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// ListUsers returns every account, newest first.
//
// Deliberately unpaginated: this service holds the accounts for one person's
// projects, so the row count is small. A cursor here would be scaffolding for
// a scale that does not exist yet.
func (s *AuthService) ListUsers(ctx context.Context) ([]models.User, error) {
	rows, err := s.DB.Query(ctx, `SELECT `+userColumns+` FROM users ORDER BY created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("querying users: %w", err)
	}
	defer rows.Close()

	users := []models.User{}
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, fmt.Errorf("scanning user: %w", err)
		}
		users = append(users, *u)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating users: %w", err)
	}
	return users, nil
}

// GetUser returns one account by id.
func (s *AuthService) GetUser(ctx context.Context, id string) (*models.User, error) {
	u, err := scanUser(s.DB.QueryRow(ctx, `SELECT `+userColumns+` FROM users WHERE id = $1`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("querying user: %w", err)
	}
	return u, nil
}

// CountAdmins counts accounts with admin rights.
func (s *AuthService) CountAdmins(ctx context.Context) (int, error) {
	var n int
	if err := s.DB.QueryRow(ctx, `SELECT count(*) FROM users WHERE is_admin`).Scan(&n); err != nil {
		return 0, fmt.Errorf("counting admins: %w", err)
	}
	return n, nil
}

// UpdateUserInput describes a partial update: nil means "leave unchanged".
//
// Pointers rather than zero values because "" and false are legitimate values
// someone may want to set, and with plain strings there is no way to tell
// "clear this field" from "do not touch it".
type UpdateUserInput struct {
	Email       *string
	Username    *string
	DisplayName *string
	IsAdmin     *bool
}

// UpdateUser applies a partial update to an account.
//
// actorID is the caller, needed for two guards that exist to prevent a
// self-inflicted lockout: you cannot remove your own admin rights, and the
// last admin cannot be demoted. Both are checked here rather than in the
// handler so every caller gets them.
func (s *AuthService) UpdateUser(ctx context.Context, actorID, id string, in UpdateUserInput) (*models.User, error) {
	if strings.TrimSpace(id) == "" {
		return nil, ErrUserIDRequired
	}
	if in.Email == nil && in.Username == nil && in.DisplayName == nil && in.IsAdmin == nil {
		return nil, ErrNothingToUpdate
	}

	current, err := s.GetUser(ctx, id)
	if err != nil {
		return nil, err
	}

	// Demoting yourself would take away the rights you are using right now,
	// and on a single-admin instance it locks everyone out of this page.
	if in.IsAdmin != nil && !*in.IsAdmin {
		if id == actorID {
			return nil, ErrCannotDemoteSelf
		}
		if current.IsAdmin {
			admins, err := s.CountAdmins(ctx)
			if err != nil {
				return nil, err
			}
			if admins <= 1 {
				return nil, ErrLastAdmin
			}
		}
	}

	// NULLIF turns "" into NULL so clearing a field works and the unique
	// constraints still ignore it (Postgres treats NULLs as distinct).
	var email, username, displayName *string
	if in.Email != nil {
		v := strings.TrimSpace(*in.Email)
		email = &v
	}
	if in.Username != nil {
		v := strings.TrimSpace(*in.Username)
		username = &v
	}
	if in.DisplayName != nil {
		v := strings.TrimSpace(*in.DisplayName)
		displayName = &v
	}

	u, err := scanUser(s.DB.QueryRow(ctx, `
		UPDATE users SET
			email        = CASE WHEN $2::boolean THEN NULLIF($3, '') ELSE email        END,
			username     = CASE WHEN $4::boolean THEN NULLIF($5, '') ELSE username     END,
			display_name = CASE WHEN $6::boolean THEN NULLIF($7, '') ELSE display_name END,
			is_admin     = CASE WHEN $8::boolean THEN $9              ELSE is_admin     END,
			updated_at   = now()
		WHERE id = $1
		RETURNING `+userColumns,
		id,
		in.Email != nil, email,
		in.Username != nil, username,
		in.DisplayName != nil, displayName,
		in.IsAdmin != nil, in.IsAdmin,
	))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		if strings.Contains(err.Error(), "users_email_key") {
			return nil, ErrEmailExists
		}
		if strings.Contains(err.Error(), "users_username_key") {
			return nil, ErrUsernameExists
		}
		// The CHECK constraint requires at least one identifier; clearing both
		// email and username at once violates it.
		if strings.Contains(err.Error(), "users_at_least_one_identifier") {
			return nil, ErrIdentifierRequired
		}
		return nil, fmt.Errorf("updating user: %w", err)
	}
	return u, nil
}

// SetPassword replaces an account's password. Used by the admin page when
// someone needs a reset without access to their email.
func (s *AuthService) SetPassword(ctx context.Context, id, password string) error {
	if len(password) < 8 {
		return ErrPasswordTooShort
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), s.BcryptCost)
	if err != nil {
		return fmt.Errorf("hashing password: %w", err)
	}

	result, err := s.DB.Exec(ctx, `UPDATE users SET password_hash = $2, updated_at = now() WHERE id = $1`, id, string(hash))
	if err != nil {
		return fmt.Errorf("updating password: %w", err)
	}
	if result.RowsAffected() == 0 {
		return ErrUserNotFound
	}

	// Changing a password does not by itself end sessions — a stolen refresh
	// token would keep working for up to 7 days. Revoking here is what makes
	// "reset the password" mean the same thing it does to a person: whoever
	// had the old password is now out.
	if _, err := s.DB.Exec(ctx, `UPDATE refresh_tokens SET revoked = true WHERE user_id = $1 AND revoked = false`, id); err != nil {
		return fmt.Errorf("revoking sessions: %w", err)
	}
	return nil
}

// DeleteUser removes an account. The refresh tokens go with it (ON DELETE
// CASCADE), which is the point: a deleted account must not keep a live session.
//
// Refuses to delete the caller's own account. That is not only about avoiding
// an accidental self-lockout — it also means the request always has an admin
// left to attribute it to.
func (s *AuthService) DeleteUser(ctx context.Context, actorID, id string) error {
	if strings.TrimSpace(id) == "" {
		return ErrUserIDRequired
	}
	if id == actorID {
		return ErrCannotDeleteSelf
	}

	target, err := s.GetUser(ctx, id)
	if err != nil {
		return err
	}

	if target.IsAdmin {
		admins, err := s.CountAdmins(ctx)
		if err != nil {
			return err
		}
		if admins <= 1 {
			return ErrLastAdmin
		}
	}

	result, err := s.DB.Exec(ctx, `DELETE FROM users WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("deleting user: %w", err)
	}
	if result.RowsAffected() == 0 {
		return ErrUserNotFound
	}
	return nil
}
