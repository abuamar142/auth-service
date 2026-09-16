package middleware

import (
	"net/http"
	"strings"

	"github.com/abuamar142/auth-service/internal/response"
	"github.com/abuamar142/auth-service/internal/services"
)

// APIKey validates X-API-Key header.
func APIKey(svc *services.AuthService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := r.Header.Get("X-API-Key")
			if key == "" {
				response.Error(w, http.StatusUnauthorized, "MISSING_API_KEY", "missing X-API-Key header", "")
				return
			}
			valid, err := svc.ValidateAPIKey(r.Context(), strings.TrimSpace(key))
			if err != nil || !valid {
				response.Error(w, http.StatusUnauthorized, "INVALID_API_KEY", "invalid or expired API key", "")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
