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

func (h *APIKeyHandler) List(w http.ResponseWriter, r *http.Request) {
	keys, err := h.AuthService.ListAPIKeys(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to list API keys", "")
		return
	}
	writeJSON(w, http.StatusOK, "api keys retrieved", keys)
}

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
