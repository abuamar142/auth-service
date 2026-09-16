package middleware

import (
	"net/http"
	"strings"

	"github.com/abuamar142/auth-service/internal/services"
)

// APIKey validates X-API-Key header.
func APIKey(svc *services.AuthService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := r.Header.Get("X-API-Key")
			if key == "" {
				http.Error(w, `{"error":{"code":"UNAUTHORIZED","message":"missing X-API-Key header"}}`, http.StatusUnauthorized)
				return
			}
			valid, err := svc.ValidateAPIKey(r.Context(), strings.TrimSpace(key))
			if err != nil || !valid {
				http.Error(w, `{"error":{"code":"UNAUTHORIZED","message":"invalid API key"}}`, http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
