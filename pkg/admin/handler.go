package admin

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"job-tracker/pkg/response"
)

// Store is what the handler needs from the data layer.
type Store interface {
	Overview(ctx context.Context) (*Overview, error)
	ListUsers(ctx context.Context, search string, page, limit int) ([]UserRow, int, error)
	SetDisabled(ctx context.Context, id string, disabled bool) error
	ListActivity(ctx context.Context, f ActivityFilter) ([]ActivityItem, int, error)
	ListReviews(ctx context.Context, page, limit int) ([]ReviewRow, int, error)
	DeleteReview(ctx context.Context, id string) error
}

type Handler struct {
	store Store
}

func NewHandler(store Store) *Handler {
	return &Handler{store: store}
}

var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// actionPattern limits filters to the names the activity log can contain.
var actionPattern = regexp.MustCompile(`^[a-z_]+(\.([a-z_]+)?)*$`)

func paging(r *http.Request, defLimit int) (page, limit int) {
	page, _ = strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	limit, _ = strconv.Atoi(r.URL.Query().Get("limit"))
	if limit < 1 || limit > 100 {
		limit = defLimit
	}
	return page, limit
}

func pagination(page, limit, total int) map[string]int {
	pages := (total + limit - 1) / limit
	if pages < 1 {
		pages = 1
	}
	return map[string]int{"page": page, "limit": limit, "total": total, "total_pages": pages}
}

func serverError(w http.ResponseWriter) {
	response.Error(w, http.StatusInternalServerError, response.ErrCodeInternalError, "Request failed", nil)
}

// Overview handles GET /api/v1/admin/overview.
func (h *Handler) Overview(w http.ResponseWriter, r *http.Request) {
	o, err := h.store.Overview(r.Context())
	if err != nil {
		serverError(w)
		return
	}
	response.JSON(w, http.StatusOK, o)
}

// Users handles GET /api/v1/admin/users.
func (h *Handler) Users(w http.ResponseWriter, r *http.Request) {
	page, limit := paging(r, 20)
	search := strings.TrimSpace(r.URL.Query().Get("search"))
	if len(search) > 100 {
		search = search[:100]
	}
	users, total, err := h.store.ListUsers(r.Context(), search, page, limit)
	if err != nil {
		serverError(w)
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": users, "pagination": pagination(page, limit, total)})
}

// UpdateUser handles PATCH /api/v1/admin/users/{id}. It can suspend or
// restore a regular account. Roles are never changed through the API.
func (h *Handler) UpdateUser(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !uuidPattern.MatchString(id) {
		response.Error(w, http.StatusBadRequest, response.ErrCodeValidationError, "Invalid user id", nil)
		return
	}

	var req struct {
		Disabled *bool `json:"disabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Disabled == nil {
		response.Error(w, http.StatusBadRequest, response.ErrCodeValidationError, "Validation failed", map[string]string{
			"disabled": "disabled must be true or false",
		})
		return
	}

	if err := h.store.SetDisabled(r.Context(), id, *req.Disabled); err != nil {
		if errors.Is(err, ErrNotFound) {
			response.Error(w, http.StatusNotFound, response.ErrCodeNotFound, "User not found, or an admin account", nil)
			return
		}
		serverError(w)
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"id": id, "disabled": *req.Disabled})
}

// Activity handles GET /api/v1/admin/activity.
func (h *Handler) Activity(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page, limit := paging(r, 25)

	f := ActivityFilter{Page: page, Limit: limit}
	if v := q.Get("action"); v != "" {
		if len(v) > 64 || !actionPattern.MatchString(v) {
			response.Error(w, http.StatusBadRequest, response.ErrCodeValidationError, "Invalid action filter", nil)
			return
		}
		f.Action = v
	}
	if v := q.Get("user_id"); v != "" {
		if !uuidPattern.MatchString(v) {
			response.Error(w, http.StatusBadRequest, response.ErrCodeValidationError, "Invalid user id", nil)
			return
		}
		f.UserID = v
	}
	if v := q.Get("ip"); v != "" {
		if len(v) > 64 {
			response.Error(w, http.StatusBadRequest, response.ErrCodeValidationError, "Invalid ip filter", nil)
			return
		}
		f.IP = v
	}
	if v := q.Get("min_status"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 100 || n > 599 {
			response.Error(w, http.StatusBadRequest, response.ErrCodeValidationError, "Invalid status filter", nil)
			return
		}
		f.MinStatus = n
	}

	items, total, err := h.store.ListActivity(r.Context(), f)
	if err != nil {
		serverError(w)
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": items, "pagination": pagination(page, limit, total)})
}

// Reviews handles GET /api/v1/admin/reviews.
func (h *Handler) Reviews(w http.ResponseWriter, r *http.Request) {
	page, limit := paging(r, 20)
	reviews, total, err := h.store.ListReviews(r.Context(), page, limit)
	if err != nil {
		serverError(w)
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": reviews, "pagination": pagination(page, limit, total)})
}

// DeleteReview handles DELETE /api/v1/admin/reviews/{id}.
func (h *Handler) DeleteReview(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !uuidPattern.MatchString(id) {
		response.Error(w, http.StatusBadRequest, response.ErrCodeValidationError, "Invalid review id", nil)
		return
	}
	if err := h.store.DeleteReview(r.Context(), id); err != nil {
		if errors.Is(err, ErrNotFound) {
			response.Error(w, http.StatusNotFound, response.ErrCodeNotFound, "Review not found", nil)
			return
		}
		serverError(w)
		return
	}
	response.NoContent(w)
}
