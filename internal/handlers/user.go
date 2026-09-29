package handlers

import (
	"net/http"
	"strings"

	"github.com/abuamar142/auth-service/internal/response"
	"github.com/abuamar142/auth-service/internal/services"
)

// UserHandler serves lookups other services need.
type UserHandler struct {
	AuthService *services.AuthService
}

func NewUserHandler(svc *services.AuthService) *UserHandler {
	return &UserHandler{AuthService: svc}
}

// FindByEmail handles GET /api/v1/internal/users?email=...
//
// Service-to-service only; the route sits behind APIKey middleware. Used by
// cafe-service when an admin grants a warung to an owner who telephoned
// instead of using the site — the admin knows the email, and only this
// service can turn it into an account id.
//
// Returns a deliberately thin payload. The caller needs to identify an
// account, not read it: no password hash, no timestamps, nothing that would
// make this a way to enumerate the user base if a key ever leaked.
func (h *UserHandler) FindByEmail(w http.ResponseWriter, r *http.Request) {
	email := strings.TrimSpace(r.URL.Query().Get("email"))
	if email == "" {
		response.Error(w, http.StatusBadRequest, "VALIDATION_ERROR", "email query parameter is required", "")
		return
	}

	user, err := h.AuthService.FindByEmail(r.Context(), email)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "INTERNAL", "lookup failed", err.Error())
		return
	}
	if user == nil {
		response.Error(w, http.StatusNotFound, "NOT_FOUND", "no account with that email", "")
		return
	}

	// Nullable in the schema; the API says "" rather than making every caller
	// handle a pointer for a display string.
	username := ""
	if user.Username != nil {
		username = *user.Username
	}
	displayName := ""
	if user.DisplayName != nil {
		displayName = *user.DisplayName
	}

	response.JSON(w, http.StatusOK, "user found", map[string]string{
		"id":           user.ID,
		"username":     username,
		"display_name": displayName,
	})
}
