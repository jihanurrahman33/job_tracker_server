package review

import (
	"encoding/json"
	"net/http"
	"strings"
	"unicode/utf8"

	"job-tracker/pkg/middleware"
	"job-tracker/pkg/response"
)

const (
	maxCommentRunes = 600
	publicListLimit = 6
)

// Handler serves the review endpoints.
type Handler struct {
	repo Repository
}

// NewHandler creates a review Handler.
func NewHandler(repo Repository) *Handler {
	return &Handler{repo: repo}
}

type saveRequest struct {
	Rating  int    `json:"rating"`
	Comment string `json:"comment"`
}

// List handles GET /api/v1/reviews (public): the aggregate rating plus the
// latest written reviews.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	summary, err := h.repo.Summary(r.Context())
	if err != nil {
		response.Error(w, http.StatusInternalServerError, response.ErrCodeInternalError, "Failed to fetch reviews", nil)
		return
	}
	reviews, err := h.repo.ListPublic(r.Context(), publicListLimit)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, response.ErrCodeInternalError, "Failed to fetch reviews", nil)
		return
	}

	// Short cache so a busy landing page doesn't hit the database each time.
	w.Header().Set("Cache-Control", "public, max-age=30")
	response.JSON(w, http.StatusOK, map[string]any{"summary": summary, "data": reviews})
}

// GetMine handles GET /api/v1/reviews/me.
func (h *Handler) GetMine(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		response.Error(w, http.StatusUnauthorized, response.ErrCodeUnauthorized, "Authentication required", nil)
		return
	}

	rev, err := h.repo.GetByUser(r.Context(), userID)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, response.ErrCodeInternalError, "Failed to fetch review", nil)
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": rev})
}

// SaveMine handles PUT /api/v1/reviews/me. It creates the review or replaces
// the existing one.
func (h *Handler) SaveMine(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		response.Error(w, http.StatusUnauthorized, response.ErrCodeUnauthorized, "Authentication required", nil)
		return
	}

	var req saveRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10)).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, response.ErrCodeValidationError, "Invalid JSON payload", nil)
		return
	}
	req.Comment = strings.TrimSpace(req.Comment)

	details := map[string]string{}
	if req.Rating < 1 || req.Rating > 5 {
		details["rating"] = "rating must be between 1 and 5"
	}
	if utf8.RuneCountInString(req.Comment) > maxCommentRunes {
		details["comment"] = "comment must be 600 characters or fewer"
	}
	if len(details) > 0 {
		response.Error(w, http.StatusBadRequest, response.ErrCodeValidationError, "Validation failed", details)
		return
	}

	rev, err := h.repo.Upsert(r.Context(), userID, req.Rating, req.Comment)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, response.ErrCodeInternalError, "Failed to save review", nil)
		return
	}
	response.JSON(w, http.StatusOK, rev)
}

// DeleteMine handles DELETE /api/v1/reviews/me.
func (h *Handler) DeleteMine(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		response.Error(w, http.StatusUnauthorized, response.ErrCodeUnauthorized, "Authentication required", nil)
		return
	}

	if err := h.repo.DeleteByUser(r.Context(), userID); err != nil {
		response.Error(w, http.StatusInternalServerError, response.ErrCodeInternalError, "Failed to delete review", nil)
		return
	}
	response.NoContent(w)
}
