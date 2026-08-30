package interview

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"job-tracker/internal/middleware"
	"job-tracker/internal/response"
)

// Handler serves HTTP endpoints for interview scheduling.
type Handler struct {
	service *Service
}

// NewHandler creates a new interview Handler.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

type createInterviewRequest struct {
	Type            string `json:"type"`
	ScheduledAt     string `json:"scheduled_at"`
	DurationMinutes *int   `json:"duration_minutes"`
	Location        string `json:"location"`
	MeetingURL      string `json:"meeting_url"`
	Notes           string `json:"notes"`
}

type updateInterviewRequest struct {
	Type            *string `json:"type"`
	ScheduledAt     *string `json:"scheduled_at"`
	DurationMinutes *int    `json:"duration_minutes"`
	Location        *string `json:"location"`
	MeetingURL      *string `json:"meeting_url"`
	Notes           *string `json:"notes"`
}

// Create handles POST /api/v1/applications/{id}/interviews
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	if _, ok := middleware.UserIDFromContext(r.Context()); !ok {
		response.Error(w, http.StatusUnauthorized, response.ErrCodeUnauthorized, "Authentication required", nil)
		return
	}

	appID := r.PathValue("id")
	if appID == "" {
		response.Error(w, http.StatusBadRequest, response.ErrCodeValidationError, "Missing application ID", nil)
		return
	}

	var req createInterviewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, response.ErrCodeValidationError, "Invalid JSON payload", nil)
		return
	}

	req.Type = strings.TrimSpace(req.Type)
	if req.Type == "" || req.ScheduledAt == "" {
		response.Error(w, http.StatusBadRequest, response.ErrCodeValidationError, "Validation failed", map[string]string{
			"type":         "type is required",
			"scheduled_at": "scheduled_at is required",
		})
		return
	}

	scheduledAt, err := time.Parse(time.RFC3339, req.ScheduledAt)
	if err != nil {
		response.Error(w, http.StatusBadRequest, response.ErrCodeValidationError, "scheduled_at must be RFC3339 formatted", nil)
		return
	}

	interview := &Interview{
		ApplicationID:   appID,
		Type:            Type(req.Type),
		ScheduledAt:     scheduledAt,
		DurationMinutes: req.DurationMinutes,
		Location:        strings.TrimSpace(req.Location),
		MeetingURL:      strings.TrimSpace(req.MeetingURL),
		Notes:           strings.TrimSpace(req.Notes),
	}

	if err := h.service.Create(r.Context(), interview); err != nil {
		response.Error(w, http.StatusInternalServerError, response.ErrCodeInternalError, "Failed to create interview", nil)
		return
	}

	response.JSON(w, http.StatusCreated, interview)
}

// ListByApplication handles GET /api/v1/applications/{id}/interviews
func (h *Handler) ListByApplication(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		response.Error(w, http.StatusUnauthorized, response.ErrCodeUnauthorized, "Authentication required", nil)
		return
	}

	appID := r.PathValue("id")
	interviews, err := h.service.ListByApplication(r.Context(), userID, appID)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, response.ErrCodeInternalError, "Failed to fetch interviews", nil)
		return
	}

	if interviews == nil {
		interviews = []Interview{}
	}

	response.JSON(w, http.StatusOK, map[string]any{"data": interviews})
}

// Get handles GET /api/v1/interviews/{id}
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		response.Error(w, http.StatusUnauthorized, response.ErrCodeUnauthorized, "Authentication required", nil)
		return
	}

	id := r.PathValue("id")
	interview, err := h.service.Get(r.Context(), userID, id)
	if err != nil {
		if errors.Is(err, ErrInterviewNotFound) {
			response.Error(w, http.StatusNotFound, response.ErrCodeNotFound, "Interview not found", nil)
			return
		}
		response.Error(w, http.StatusInternalServerError, response.ErrCodeInternalError, "Failed to fetch interview", nil)
		return
	}

	response.JSON(w, http.StatusOK, interview)
}

// Update handles PATCH /api/v1/interviews/{id}
func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		response.Error(w, http.StatusUnauthorized, response.ErrCodeUnauthorized, "Authentication required", nil)
		return
	}

	id := r.PathValue("id")
	current, err := h.service.Get(r.Context(), userID, id)
	if err != nil {
		if errors.Is(err, ErrInterviewNotFound) {
			response.Error(w, http.StatusNotFound, response.ErrCodeNotFound, "Interview not found", nil)
			return
		}
		response.Error(w, http.StatusInternalServerError, response.ErrCodeInternalError, "Failed to fetch interview", nil)
		return
	}

	var req updateInterviewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, response.ErrCodeValidationError, "Invalid JSON payload", nil)
		return
	}

	if req.Type != nil {
		current.Type = Type(*req.Type)
	}
	if req.ScheduledAt != nil {
		parsed, err := time.Parse(time.RFC3339, *req.ScheduledAt)
		if err != nil {
			response.Error(w, http.StatusBadRequest, response.ErrCodeValidationError, "scheduled_at must be RFC3339 formatted", nil)
			return
		}
		current.ScheduledAt = parsed
	}
	if req.DurationMinutes != nil {
		current.DurationMinutes = req.DurationMinutes
	}
	if req.Location != nil {
		current.Location = *req.Location
	}
	if req.MeetingURL != nil {
		current.MeetingURL = *req.MeetingURL
	}
	if req.Notes != nil {
		current.Notes = *req.Notes
	}

	if err := h.service.Update(r.Context(), userID, id, current); err != nil {
		response.Error(w, http.StatusInternalServerError, response.ErrCodeInternalError, "Failed to update interview", nil)
		return
	}

	response.JSON(w, http.StatusOK, current)
}

// Delete handles DELETE /api/v1/interviews/{id}
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		response.Error(w, http.StatusUnauthorized, response.ErrCodeUnauthorized, "Authentication required", nil)
		return
	}

	id := r.PathValue("id")
	if err := h.service.Delete(r.Context(), userID, id); err != nil {
		response.Error(w, http.StatusNotFound, response.ErrCodeNotFound, "Interview not found", nil)
		return
	}

	response.NoContent(w)
}
