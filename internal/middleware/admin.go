package middleware

import (
	"net/http"

	"github.com/abuamar142/auth-service/internal/response"
)

// RequireAdmin rejects the request unless the authenticated user is an admin.
//
// Must be mounted after Auth, which is what puts the user in the context.
// The check reads the flag from the database-backed user record, not from the
// token: a token issued before someone was demoted must stop working
// immediately, and a claim baked into a JWT would keep working until it
// expired.
//
// Answers 403, not 404. Hiding the route would be a lie about what the service
// exposes, and the caller is already authenticated — there is nothing left to
// conceal from them.
func RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := GetUser(r.Context())
		if user == nil {
			response.Error(w, http.StatusUnauthorized, "MISSING_TOKEN", "authentication required", "")
			return
		}
		if !user.IsAdmin {
			response.Error(w, http.StatusForbidden, "FORBIDDEN", "admin rights required", "")
			return
		}
		next.ServeHTTP(w, r)
	})
}
