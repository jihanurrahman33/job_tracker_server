package reminder

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"job-tracker/pkg/middleware"
	"job-tracker/pkg/response"
)

// Handler serves HTTP endpoints for reminders.
type Handler struct {
	service *Service
}

// NewHandler creates a new reminder Handler.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

type createReminderRequest struct {
	ApplicationID *string `json:"application_id"`
	Title         string  `json:"title"`
	Description   string  `json:"description"`
	RemindAt      string  `json:"remind_at"`
}

type updateReminderRequest struct {
	Title       *string `json:"title"`
	Description *string `json:"description"`
	RemindAt    *string `json:"remind_at"`
	Completed   *bool   `json:"completed"`
}

// Create handles POST /api/v1/reminders
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		response.Error(w, http.StatusUnauthorized, response.ErrCodeUnauthorized, "Authentication required", nil)
		return
	}

	var req createReminderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, response.ErrCodeValidationError, "Invalid JSON payload", nil)
		return
	}

	req.Title = strings.TrimSpace(req.Title)
	if req.Title == "" || req.RemindAt == "" {
		response.Error(w, http.StatusBadRequest, response.ErrCodeValidationError, "Validation failed", map[string]string{
			"title":     "title is required",
			"remind_at": "remind_at is required",
		})
		return
	}

	remindAt, err := time.Parse(time.RFC3339, req.RemindAt)
	if err != nil {
		response.Error(w, http.StatusBadRequest, response.ErrCodeValidationError, "remind_at must be RFC3339 formatted", nil)
		return
	}

	rem := &Reminder{
		UserID:        userID,
		ApplicationID: req.ApplicationID,
		Title:         req.Title,
		Description:   strings.TrimSpace(req.Description),
		RemindAt:      remindAt,
		Completed:     false,
	}

	if err := h.service.Create(r.Context(), rem); err != nil {
		response.Error(w, http.StatusInternalServerError, response.ErrCodeInternalError, "Failed to create reminder", nil)
		return
	}

	response.JSON(w, http.StatusCreated, rem)
}

// List handles GET /api/v1/reminders
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		response.Error(w, http.StatusUnauthorized, response.ErrCodeUnauthorized, "Authentication required", nil)
		return
	}

	reminders, err := h.service.List(r.Context(), userID)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, response.ErrCodeInternalError, "Failed to fetch reminders", nil)
		return
	}

	if reminders == nil {
		reminders = []Reminder{}
	}

	response.JSON(w, http.StatusOK, map[string]any{"data": reminders})
}

// Update handles PATCH /api/v1/reminders/{id}
func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		response.Error(w, http.StatusUnauthorized, response.ErrCodeUnauthorized, "Authentication required", nil)
		return
	}

	id := r.PathValue("id")
	current, err := h.service.Get(r.Context(), userID, id)
	if err != nil {
		if errors.Is(err, ErrReminderNotFound) {
			response.Error(w, http.StatusNotFound, response.ErrCodeNotFound, "Reminder not found", nil)
			return
		}
		response.Error(w, http.StatusInternalServerError, response.ErrCodeInternalError, "Failed to fetch reminder", nil)
		return
	}

	var req updateReminderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, response.ErrCodeValidationError, "Invalid JSON payload", nil)
		return
	}

	if req.Title != nil {
		current.Title = strings.TrimSpace(*req.Title)
	}
	if req.Description != nil {
		current.Description = strings.TrimSpace(*req.Description)
	}
	if req.RemindAt != nil {
		parsed, err := time.Parse(time.RFC3339, *req.RemindAt)
		if err != nil {
			response.Error(w, http.StatusBadRequest, response.ErrCodeValidationError, "remind_at must be RFC3339 formatted", nil)
			return
		}
		current.RemindAt = parsed
	}
	if req.Completed != nil {
		current.Completed = *req.Completed
	}

	if err := h.service.Update(r.Context(), userID, id, current); err != nil {
		response.Error(w, http.StatusInternalServerError, response.ErrCodeInternalError, "Failed to update reminder", nil)
		return
	}

	response.JSON(w, http.StatusOK, current)
}

// Delete handles DELETE /api/v1/reminders/{id}
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		response.Error(w, http.StatusUnauthorized, response.ErrCodeUnauthorized, "Authentication required", nil)
		return
	}

	id := r.PathValue("id")
	if err := h.service.Delete(r.Context(), userID, id); err != nil {
		response.Error(w, http.StatusNotFound, response.ErrCodeNotFound, "Reminder not found", nil)
		return
	}

	response.NoContent(w)
}
