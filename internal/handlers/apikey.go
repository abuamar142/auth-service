package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/abuamar142/auth-service/internal/services"
)

type APIKeyHandler struct {
	AuthService *services.AuthService
}

func NewAPIKeyHandler(svc *services.AuthService) *APIKeyHandler {
	return &APIKeyHandler{AuthService: svc}
}

// List godoc
// @Summary      List API keys
// @Description  Returns all API keys for the authenticated user.
// @Tags         api-keys
// @Produce      json
// @Security     BearerAuth
// @Success      200 {object} Response
// @Failure      401 {object} Response
// @Failure      500 {object} Response
// @Router       /api/v1/api-keys [get]
func (h *APIKeyHandler) List(w http.ResponseWriter, r *http.Request) {
	keys, err := h.AuthService.ListAPIKeys(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to list API keys", "")
		return
	}
	writeJSON(w, http.StatusOK, "api keys retrieved", keys)
}

// Create godoc
// @Summary      Create API key
// @Description  Generate a new API key. The raw key is shown once — store it securely.
// @Tags         api-keys
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body body object true "API key payload"
// @Success      201 {object} Response
// @Failure      400 {object} Response
// @Failure      401 {object} Response
// @Router       /api/v1/api-keys [post]
func (h *APIKeyHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name      string `json:"name"`
		ExpiresIn string `json:"expires_in,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_JSON", "invalid request body", "")
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "name is required", "")
		return
	}

	var expiresAt *time.Time
	if req.ExpiresIn != "" {
		d, err := parseDuration(req.ExpiresIn)
		if err != nil {
			writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid expires_in format", "use e.g. '30d', '24h'")
			return
		}
		t := time.Now().Add(d)
		expiresAt = &t
	}

	rawKey, apiKey, err := h.AuthService.CreateAPIKey(r.Context(), req.Name, expiresAt)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to create API key", "")
		return
	}

	writeJSON(w, http.StatusCreated, "api key created", map[string]interface{}{
		"api_key": apiKey,
		"key":     rawKey,
	})
}

// Delete godoc
// @Summary      Delete API key
// @Description  Permanently delete an API key by ID.
// @Tags         api-keys
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "API key ID"
// @Success      200 {object} Response
// @Failure      400 {object} Response
// @Failure      401 {object} Response
// @Failure      404 {object} Response
// @Router       /api/v1/api-keys/{id} [delete]
func (h *APIKeyHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "API key ID is required", "")
		return
	}

	err := h.AuthService.DeleteAPIKey(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "API key not found", "")
		return
	}

	writeJSON(w, http.StatusOK, "api key deleted", nil)
}
