package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/abuamar142/auth-service/internal/middleware"
	"github.com/abuamar142/auth-service/internal/services"
)

type AuthHandler struct {
	AuthService *services.AuthService
}

func NewAuthHandler(svc *services.AuthService) *AuthHandler {
	return &AuthHandler{AuthService: svc}
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email       string `json:"email"`
		Username    string `json:"username"`
		Password    string `json:"password"`
		DisplayName string `json:"display_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_JSON", "invalid request body", "")
		return
	}
	if req.Password == "" {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "password is required", "")
		return
	}
	if len(req.Password) < 8 {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "password too short", "must be at least 8 characters")
		return
	}

	user, err := h.AuthService.Register(r.Context(), req.Email, req.Username, req.Password, req.DisplayName)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrIdentifierRequired):
			writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error(), "")
		case errors.Is(err, services.ErrEmailExists):
			writeError(w, http.StatusConflict, "CONFLICT", "email already exists", "")
		case errors.Is(err, services.ErrUsernameExists):
			writeError(w, http.StatusConflict, "CONFLICT", "username already exists", "")
		default:
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to register user", "")
		}
		return
	}

	writeJSON(w, http.StatusCreated, "user registered", user)
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Identifier string `json:"identifier"`
		Password   string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_JSON", "invalid request body", "")
		return
	}
	if req.Identifier == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "identifier and password are required", "")
		return
	}

	accessToken, refreshToken, err := h.AuthService.Login(r.Context(), req.Identifier, req.Password)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrInvalidCredentials):
			writeError(w, http.StatusUnauthorized, "INVALID_CREDENTIALS", "invalid email/username or password", "")
		default:
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to login", "")
		}
		return
	}

	writeJSON(w, http.StatusOK, "login successful", map[string]string{
		"access_token":  accessToken,
		"refresh_token": refreshToken,
	})
}

func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_JSON", "invalid request body", "")
		return
	}
	if req.RefreshToken == "" {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "refresh_token is required", "")
		return
	}

	accessToken, refreshToken, err := h.AuthService.Refresh(r.Context(), req.RefreshToken)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrInvalidRefreshToken):
			writeError(w, http.StatusUnauthorized, "INVALID_REFRESH_TOKEN", "invalid or expired refresh token", "")
		default:
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to refresh token", "")
		}
		return
	}

	writeJSON(w, http.StatusOK, "token refreshed", map[string]string{
		"access_token":  accessToken,
		"refresh_token": refreshToken,
	})
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required", "")
		return
	}

	var req struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_JSON", "invalid request body", "")
		return
	}
	if req.RefreshToken == "" {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "refresh_token is required", "")
		return
	}

	err := h.AuthService.Logout(r.Context(), req.RefreshToken)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "INVALID_REFRESH_TOKEN", "invalid refresh token", "")
		return
	}

	writeJSON(w, http.StatusOK, "logged out", nil)
}

func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required", "")
		return
	}
	writeJSON(w, http.StatusOK, "user retrieved", user)
}

// parseDuration parses a simple duration string like "24h", "7d", "30d".
func parseDuration(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, nil
	}
	if strings.HasSuffix(s, "d") {
		n := 0
		for _, c := range s[:len(s)-1] {
			if c >= '0' && c <= '9' {
				n = n*10 + int(c-'0')
			}
		}
		if n > 0 {
			return time.Duration(n) * 24 * time.Hour, nil
		}
	}
	return time.ParseDuration(s)
}
