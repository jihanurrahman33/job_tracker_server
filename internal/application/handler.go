package application

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"job-tracker/internal/middleware"
	"job-tracker/internal/response"
)

// Handler serves HTTP endpoints for job applications.
type Handler struct {
	service *Service
}

// NewHandler creates a new application HTTP Handler.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

type createRequest struct {
	Company        string   `json:"company"`
	Position       string   `json:"position"`
	Location       string   `json:"location"`
	JobURL         string   `json:"job_url"`
	SalaryMin      *float64 `json:"salary_min"`
	SalaryMax      *float64 `json:"salary_max"`
	SalaryCurrency string   `json:"salary_currency"`
	Status         string   `json:"status"`
	AppliedAt      *string  `json:"applied_at"`
	Notes          string   `json:"notes"`
}

type updateRequest struct {
	Company        *string  `json:"company"`
	Position       *string  `json:"position"`
	Location       *string  `json:"location"`
	JobURL         *string  `json:"job_url"`
	SalaryMin      *float64 `json:"salary_min"`
	SalaryMax      *float64 `json:"salary_max"`
	SalaryCurrency *string  `json:"salary_currency"`
	Status         *string  `json:"status"`
	Notes          *string  `json:"notes"`
}

type paginationResponse struct {
	Page       int `json:"page"`
	Limit      int `json:"limit"`
	Total      int `json:"total"`
	TotalPages int `json:"total_pages"`
}

type listResponse struct {
	Data       []Application      `json:"data"`
	Pagination paginationResponse `json:"pagination"`
}

// Create handles POST /api/v1/applications
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		response.Error(w, http.StatusUnauthorized, response.ErrCodeUnauthorized, "Authentication required", nil)
		return
	}

	var req createRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, response.ErrCodeValidationError, "Invalid JSON payload", nil)
		return
	}

	valErrs := ValidateApplicationInput(req.Company, req.Position, req.Status, req.JobURL, req.SalaryMin, req.SalaryMax)
	if len(valErrs) > 0 {
		response.Error(w, http.StatusBadRequest, response.ErrCodeValidationError, "Validation failed", valErrs)
		return
	}

	var appliedAt *time.Time
	if req.AppliedAt != nil && *req.AppliedAt != "" {
		parsed, err := time.Parse(time.RFC3339, *req.AppliedAt)
		if err != nil {
			response.Error(w, http.StatusBadRequest, response.ErrCodeValidationError, "applied_at must be RFC3339 formatted date", nil)
			return
		}
		appliedAt = &parsed
	}

	status := StatusApplied
	if req.Status != "" {
		if s, ok := ValidateStatus(req.Status); ok {
			status = s
		}
	}

	app, err := h.service.Create(r.Context(), CreateInput{
		UserID:         userID,
		Company:        strings.TrimSpace(req.Company),
		Position:       strings.TrimSpace(req.Position),
		Location:       strings.TrimSpace(req.Location),
		JobURL:         strings.TrimSpace(req.JobURL),
		SalaryMin:      req.SalaryMin,
		SalaryMax:      req.SalaryMax,
		SalaryCurrency: strings.TrimSpace(req.SalaryCurrency),
		Status:         status,
		AppliedAt:      appliedAt,
		Notes:          strings.TrimSpace(req.Notes),
	})
	if err != nil {
		response.Error(w, http.StatusInternalServerError, response.ErrCodeInternalError, "Failed to create application", nil)
		return
	}

	response.JSON(w, http.StatusCreated, app)
}

// Get handles GET /api/v1/applications/{id}
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		response.Error(w, http.StatusUnauthorized, response.ErrCodeUnauthorized, "Authentication required", nil)
		return
	}

	id := r.PathValue("id")
	if id == "" {
		response.Error(w, http.StatusBadRequest, response.ErrCodeValidationError, "Missing application ID", nil)
		return
	}

	app, err := h.service.Get(r.Context(), userID, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			response.Error(w, http.StatusNotFound, response.ErrCodeNotFound, "Application not found", nil)
			return
		}
		response.Error(w, http.StatusInternalServerError, response.ErrCodeInternalError, "Failed to fetch application", nil)
		return
	}

	response.JSON(w, http.StatusOK, app)
}

// List handles GET /api/v1/applications
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		response.Error(w, http.StatusUnauthorized, response.ErrCodeUnauthorized, "Authentication required", nil)
		return
	}

	query := r.URL.Query()
	page, _ := strconv.Atoi(query.Get("page"))
	if page <= 0 {
		page = 1
	}
	limit, _ := strconv.Atoi(query.Get("limit"))
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	sortBy := query.Get("sort")
	sortDesc := false
	if strings.HasPrefix(sortBy, "-") {
		sortDesc = true
		sortBy = strings.TrimPrefix(sortBy, "-")
	}

	filter := Filter{
		Status:   query.Get("status"),
		Company:  query.Get("company"),
		Location: query.Get("location"),
		SortBy:   sortBy,
		SortDesc: sortDesc,
		Page:     page,
		Limit:    limit,
	}

	if fromStr := query.Get("from"); fromStr != "" {
		if parsed, err := time.Parse("2006-01-02", fromStr); err == nil {
			filter.From = &parsed
		}
	}
	if toStr := query.Get("to"); toStr != "" {
		if parsed, err := time.Parse("2006-01-02", toStr); err == nil {
			filter.To = &parsed
		}
	}

	apps, total, err := h.service.List(r.Context(), userID, filter)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, response.ErrCodeInternalError, "Failed to list applications", nil)
		return
	}

	totalPages := (total + limit - 1) / limit
	if totalPages == 0 {
		totalPages = 1
	}

	if apps == nil {
		apps = []Application{}
	}

	response.JSON(w, http.StatusOK, listResponse{
		Data: apps,
		Pagination: paginationResponse{
			Page:       page,
			Limit:      limit,
			Total:      total,
			TotalPages: totalPages,
		},
	})
}

// Update handles PATCH /api/v1/applications/{id}
func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		response.Error(w, http.StatusUnauthorized, response.ErrCodeUnauthorized, "Authentication required", nil)
		return
	}

	id := r.PathValue("id")
	var req updateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, response.ErrCodeValidationError, "Invalid JSON payload", nil)
		return
	}

	var statusPtr *Status
	if req.Status != nil {
		s, valid := ValidateStatus(*req.Status)
		if !valid {
			response.Error(w, http.StatusBadRequest, response.ErrCodeInvalidStatus, "Invalid status value", nil)
			return
		}
		statusPtr = &s
	}

	updated, err := h.service.Update(r.Context(), userID, id, UpdateInput{
		Company:        req.Company,
		Position:       req.Position,
		Location:       req.Location,
		JobURL:         req.JobURL,
		SalaryMin:      req.SalaryMin,
		SalaryMax:      req.SalaryMax,
		SalaryCurrency: req.SalaryCurrency,
		Status:         statusPtr,
		Notes:          req.Notes,
	})
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			response.Error(w, http.StatusNotFound, response.ErrCodeNotFound, "Application not found", nil)
			return
		}
		response.Error(w, http.StatusInternalServerError, response.ErrCodeInternalError, "Failed to update application", nil)
		return
	}

	response.JSON(w, http.StatusOK, updated)
}

// Delete handles DELETE /api/v1/applications/{id}
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		response.Error(w, http.StatusUnauthorized, response.ErrCodeUnauthorized, "Authentication required", nil)
		return
	}

	id := r.PathValue("id")
	if err := h.service.Delete(r.Context(), userID, id); err != nil {
		response.Error(w, http.StatusNotFound, response.ErrCodeNotFound, "Application not found", nil)
		return
	}

	response.NoContent(w)
}

// Events handles GET /api/v1/applications/{id}/events
func (h *Handler) Events(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		response.Error(w, http.StatusUnauthorized, response.ErrCodeUnauthorized, "Authentication required", nil)
		return
	}

	id := r.PathValue("id")
	events, err := h.service.ListEvents(r.Context(), userID, id)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, response.ErrCodeInternalError, "Failed to fetch events", nil)
		return
	}

	if events == nil {
		events = []Event{}
	}

	response.JSON(w, http.StatusOK, map[string]any{"data": events})
}
