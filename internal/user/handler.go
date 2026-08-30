package user

import (
	"net/http"

	"job-tracker/internal/middleware"
	"job-tracker/internal/response"
)

// Handler handles HTTP requests for user profiles.
type Handler struct {
	service *Service
}

// NewHandler creates a new user HTTP Handler.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// Me returns the current authenticated user's profile.
func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		response.Error(w, http.StatusUnauthorized, response.ErrCodeUnauthorized, "Authentication required", nil)
		return
	}

	u, err := h.service.GetByID(r.Context(), userID)
	if err != nil {
		response.Error(w, http.StatusNotFound, response.ErrCodeNotFound, "User not found", nil)
		return
	}

	response.JSON(w, http.StatusOK, u)
}
