package handlers

import (
	"net/http"

	_ "github.com/abuamar142/auth-service/internal/models"
	"github.com/abuamar142/auth-service/internal/response"
)

type HealthHandler struct{}

func NewHealthHandler() *HealthHandler {
	return &HealthHandler{}
}

// Health godoc
// @Summary      Health check
// @Description  Returns service health status
// @Tags         health
// @Produce      json
// @Success      200 {object} models.SwaggerResponse
// @Router       /api/health [get]
func (h *HealthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	response.JSON(w, http.StatusOK, "service healthy", map[string]string{"status": "ok"})
}
