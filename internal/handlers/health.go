package handlers

import "net/http"

type HealthHandler struct{}

func NewHealthHandler() *HealthHandler {
	return &HealthHandler{}
}

func (h *HealthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, "service healthy", map[string]string{"status": "ok"})
}
