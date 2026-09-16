package services

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"

	"github.com/abuamar142/auth-service/internal/models"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrEmailExists        = errors.New("email already exists")
	ErrUsernameExists     = errors.New("username already exists")
	ErrIdentifierRequired = errors.New("email or username is required")
	ErrInvalidRefreshToken = errors.New("invalid or expired refresh token")
	ErrUserNotFound       = errors.New("user not found")
)

type AuthService struct {
	DB          *pgxpool.Pool
	JWTSecret   []byte
	BcryptCost  int
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration
}

func NewAuthService(db *pgxpool.Pool, jwtSecret string, bcryptCost int) *AuthService {
	return &AuthService{
		DB:               db,
		JWTSecret:        []byte(jwtSecret),
		BcryptCost:       bcryptCost,
		AccessTokenTTL:  15 * time.Minute,
		RefreshTokenTTL: 7 * 24 * time.Hour,
	}
}

// Register creates a new user. At least one of email/username must be provided.
func (s *AuthService) Register(ctx context.Context, email, username, password, displayName string) (*models.User, error) {
	email = strings.TrimSpace(email)
	username = strings.TrimSpace(username)
	displayName = strings.TrimSpace(displayName)

	if email == "" && username == "" {
		return nil, ErrIdentifierRequired
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), s.BcryptCost)
	if err != nil {
		return nil, fmt.Errorf("hashing password: %w", err)
	}

	var user models.User
	err = s.DB.QueryRow(ctx, `
		INSERT INTO users (email, username, password_hash, display_name)
		VALUES (NULLIF($1, ''), NULLIF($2, ''), $3, NULLIF($4, ''))
		RETURNING id, email, username, password_hash, display_name, created_at, updated_at`,
		email, username, string(hash), displayName,
	).Scan(&user.ID, &user.Email, &user.Username, &user.PasswordHash, &user.DisplayName, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		if strings.Contains(err.Error(), "users_email_key") {
			return nil, ErrEmailExists
		}
		if strings.Contains(err.Error(), "users_username_key") {
			return nil, ErrUsernameExists
		}
		return nil, fmt.Errorf("inserting user: %w", err)
	}
	return &user, nil
}

// Login authenticates by email OR username, returns JWT access + refresh token.
func (s *AuthService) Login(ctx context.Context, identifier, password string) (accessToken, refreshToken string, err error) {
	identifier = strings.TrimSpace(identifier)

	var user models.User
	err = s.DB.QueryRow(ctx, `
		SELECT id, email, username, password_hash, display_name, created_at, updated_at
		FROM users WHERE email = $1 OR username = $1`,
		identifier,
	).Scan(&user.ID, &user.Email, &user.Username, &user.PasswordHash, &user.DisplayName, &user.CreatedAt, &user.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return "", "", ErrInvalidCredentials
	}
	if err != nil {
		return "", "", fmt.Errorf("querying user: %w", err)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return "", "", ErrInvalidCredentials
	}

	accessToken, err = s.signAccessToken(&user)
	if err != nil {
		return "", "", fmt.Errorf("signing access token: %w", err)
	}

	refreshToken, err = s.createRefreshToken(ctx, user.ID)
	if err != nil {
		return "", "", fmt.Errorf("creating refresh token: %w", err)
	}

	return accessToken, refreshToken, nil
}

// Refresh rotates a valid refresh token, issuing new access + refresh pair.
func (s *AuthService) Refresh(ctx context.Context, rawToken string) (accessToken, refreshToken string, err error) {
	tokenHash := hashToken(rawToken)

	var userID string
	var expiresAt time.Time
	var revoked bool

	err = s.DB.QueryRow(ctx, `
		SELECT user_id, expires_at, revoked FROM refresh_tokens WHERE token_hash = $1`,
		tokenHash,
	).Scan(&userID, &expiresAt, &revoked)
	if errors.Is(err, sql.ErrNoRows) {
		return "", "", ErrInvalidRefreshToken
	}
	if err != nil {
		return "", "", fmt.Errorf("querying refresh token: %w", err)
	}
	if revoked || time.Now().After(expiresAt) {
		return "", "", ErrInvalidRefreshToken
	}

	// Revoke old token
	_, err = s.DB.Exec(ctx, `UPDATE refresh_tokens SET revoked = true WHERE token_hash = $1`, tokenHash)
	if err != nil {
		return "", "", fmt.Errorf("revoking old token: %w", err)
	}

	// Fetch user for new access token
	var user models.User
	err = s.DB.QueryRow(ctx, `
		SELECT id, email, username, password_hash, display_name, created_at, updated_at
		FROM users WHERE id = $1`, userID,
	).Scan(&user.ID, &user.Email, &user.Username, &user.PasswordHash, &user.DisplayName, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		return "", "", fmt.Errorf("fetching user: %w", err)
	}

	accessToken, err = s.signAccessToken(&user)
	if err != nil {
		return "", "", fmt.Errorf("signing access token: %w", err)
	}

	refreshToken, err = s.createRefreshToken(ctx, userID)
	if err != nil {
		return "", "", fmt.Errorf("creating refresh token: %w", err)
	}

	return accessToken, refreshToken, nil
}

// Logout revokes a refresh token.
func (s *AuthService) Logout(ctx context.Context, rawToken string) error {
	tokenHash := hashToken(rawToken)
	result, err := s.DB.Exec(ctx, `UPDATE refresh_tokens SET revoked = true WHERE token_hash = $1 AND revoked = false`, tokenHash)
	if err != nil {
		return fmt.Errorf("revoking token: %w", err)
	}
	rows := result.RowsAffected()
	if rows == 0 {
		return ErrInvalidRefreshToken
	}
	return nil
}

// ValidateAccessToken verifies a JWT and returns the user.
func (s *AuthService) ValidateAccessToken(ctx context.Context, tokenStr string) (*models.User, error) {
	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return s.JWTSecret, nil
	})
	if err != nil {
		return nil, fmt.Errorf("parsing token: %w", err)
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token claims")
	}

	// Validate issuer
	iss, _ := claims["iss"].(string)
	if iss != "auth.abuamar.online" {
		return nil, errors.New("invalid token issuer")
	}

	userID, _ := claims["sub"].(string)
	if userID == "" {
		return nil, errors.New("missing subject in token")
	}

	var user models.User
	err = s.DB.QueryRow(ctx, `
		SELECT id, email, username, password_hash, display_name, created_at, updated_at
		FROM users WHERE id = $1`, userID,
	).Scan(&user.ID, &user.Email, &user.Username, &user.PasswordHash, &user.DisplayName, &user.CreatedAt, &user.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("fetching user: %w", err)
	}
	return &user, nil
}

// CreateAPIKey generates a new API key, returns the raw key (only shown once).
func (s *AuthService) CreateAPIKey(ctx context.Context, name string, expiresAt *time.Time) (string, *models.APIKey, error) {
	raw, err := randomBytes(32)
	if err != nil {
		return "", nil, fmt.Errorf("generating key: %w", err)
	}
	keyStr := "ak_" + hex.EncodeToString(raw)
	keyHash := hashToken(keyStr)
	prefix := keyStr[:12] // "ak_" + 9 hex chars

	var apiKey models.APIKey
	err = s.DB.QueryRow(ctx, `
		INSERT INTO api_keys (name, key_hash, prefix, expires_at)
		VALUES ($1, $2, $3, $4)
		RETURNING id, name, key_hash, prefix, expires_at, created_at`,
		name, keyHash, prefix, expiresAt,
	).Scan(&apiKey.ID, &apiKey.Name, &apiKey.KeyHash, &apiKey.Prefix, &apiKey.ExpiresAt, &apiKey.CreatedAt)
	if err != nil {
		return "", nil, fmt.Errorf("inserting api key: %w", err)
	}
	return keyStr, &apiKey, nil
}

// ListAPIKeys returns all API keys.
func (s *AuthService) ListAPIKeys(ctx context.Context) ([]models.APIKey, error) {
	rows, err := s.DB.Query(ctx, `SELECT id, name, key_hash, prefix, expires_at, created_at FROM api_keys ORDER BY created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("querying api keys: %w", err)
	}
	defer rows.Close()

	var keys []models.APIKey
	for rows.Next() {
		var k models.APIKey
		if err := rows.Scan(&k.ID, &k.Name, &k.KeyHash, &k.Prefix, &k.ExpiresAt, &k.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning api key: %w", err)
		}
		keys = append(keys, k)
	}
	return keys, nil
}

// DeleteAPIKey removes an API key by ID.
func (s *AuthService) DeleteAPIKey(ctx context.Context, id string) error {
	result, err := s.DB.Exec(ctx, `DELETE FROM api_keys WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("deleting api key: %w", err)
	}
	if rows := result.RowsAffected(); rows == 0 {
		return errors.New("api key not found")
	}
	return nil
}

// ValidateAPIKey checks an API key against stored hashes.
func (s *AuthService) ValidateAPIKey(ctx context.Context, rawKey string) (bool, error) {
	keyHash := hashToken(rawKey)
	var exists bool
	err := s.DB.QueryRow(ctx, `
		SELECT EXISTS(SELECT 1 FROM api_keys WHERE key_hash = $1 AND (expires_at IS NULL OR expires_at > now()))`,
		keyHash,
	).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("checking api key: %w", err)
	}
	return exists, nil
}

// --- private helpers ---

func (s *AuthService) signAccessToken(user *models.User) (string, error) {
	now := time.Now()
	claims := jwt.MapClaims{
		"sub": user.ID,
		"iss": "auth.abuamar.online",
		"iat": now.Unix(),
		"exp": now.Add(s.AccessTokenTTL).Unix(),
	}
	if user.Email != nil {
		claims["email"] = *user.Email
	}
	if user.Username != nil {
		claims["username"] = *user.Username
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.JWTSecret)
}

func (s *AuthService) createRefreshToken(ctx context.Context, userID string) (string, error) {
	raw, err := randomBytes(64)
	if err != nil {
		return "", err
	}
	rawStr := hex.EncodeToString(raw)
	tokenHash := hashToken(rawStr)
	expiresAt := time.Now().Add(s.RefreshTokenTTL)

	_, err = s.DB.Exec(ctx, `
		INSERT INTO refresh_tokens (user_id, token_hash, expires_at)
		VALUES ($1, $2, $3)`,
		userID, tokenHash, expiresAt,
	)
	if err != nil {
		return "", err
	}
	return rawStr, nil
}

func randomBytes(n int) ([]byte, error) {
	b := make([]byte, n)
	_, err := rand.Read(b)
	return b, err
}

func hashToken(raw string) string {
	h := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(h[:])
}
