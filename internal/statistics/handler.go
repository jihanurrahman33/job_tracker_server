package statistics

import (
	"net/http"

	"job-tracker/internal/middleware"
	"job-tracker/internal/response"
)

// Handler serves HTTP endpoints for statistics.
type Handler struct {
	service *Service
}

// NewHandler creates a new statistics HTTP Handler.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// Get handles GET /api/v1/statistics
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		response.Error(w, http.StatusUnauthorized, response.ErrCodeUnauthorized, "Authentication required", nil)
		return
	}

	stats, err := h.service.Get(r.Context(), userID)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, response.ErrCodeInternalError, "Failed to compute statistics", nil)
		return
	}

	response.JSON(w, http.StatusOK, stats)
}
