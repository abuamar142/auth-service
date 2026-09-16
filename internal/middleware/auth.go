package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/abuamar142/auth-service/internal/models"
	"github.com/abuamar142/auth-service/internal/response"
	"github.com/abuamar142/auth-service/internal/services"
)

type contextKey string

const UserKey contextKey = "user"

// Auth validates Bearer JWT and injects user into context.
func Auth(svc *services.AuthService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")
			if header == "" || !strings.HasPrefix(header, "Bearer ") {
				response.Error(w, http.StatusUnauthorized, "MISSING_TOKEN", "missing or invalid Authorization header", "expected: Bearer <token>")
				return
			}
			token := strings.TrimPrefix(header, "Bearer ")
			user, err := svc.ValidateAccessToken(r.Context(), token)
			if err != nil {
				response.Error(w, http.StatusUnauthorized, "INVALID_TOKEN", "invalid or expired access token", "")
				return
			}
			ctx := context.WithValue(r.Context(), UserKey, user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetUser extracts user from context (set by Auth middleware).
func GetUser(ctx context.Context) *models.User {
	user, _ := ctx.Value(UserKey).(*models.User)
	return user
}
