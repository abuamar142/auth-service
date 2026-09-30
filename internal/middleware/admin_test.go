package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/abuamar142/auth-service/internal/models"
)

// withUser puts an account in the context, which is what the Auth middleware
// does on the routes RequireAdmin protects.
func withUser(ctx context.Context, u *models.User) context.Context {
	return context.WithValue(ctx, UserKey, u)
}

// RequireAdmin is the only thing standing between an ordinary signed-in account
// and everyone else's password, so each branch below is a real outcome: a
// missing 403 here would let any logged-in user reset any password.
func TestRequireAdmin(t *testing.T) {
	var reached bool
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reached = true
		w.WriteHeader(http.StatusTeapot)
	})
	handler := RequireAdmin(next)

	cases := []struct {
		name string
		user *models.User
		want int
	}{
		{"no user in context", nil, http.StatusUnauthorized},
		{"non-admin", &models.User{ID: "u1", IsAdmin: false}, http.StatusForbidden},
		{"admin", &models.User{ID: "u2", IsAdmin: true}, http.StatusTeapot},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			reached = false

			req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users", nil)
			if tc.user != nil {
				req = req.WithContext(withUser(req.Context(), tc.user))
			}
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != tc.want {
				t.Errorf("status = %d, want %d", rec.Code, tc.want)
			}

			if tc.want == http.StatusTeapot {
				if !reached {
					t.Error("admin request did not reach the handler")
				}
				return
			}
			if reached {
				t.Errorf("handler ran for a caller that should have been stopped (%d)", tc.want)
			}
		})
	}
}
