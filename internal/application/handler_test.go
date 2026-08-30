package application_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"job-tracker/internal/application"
	"job-tracker/internal/middleware"
)

type memoryAppRepo struct {
	apps   map[string]*application.Application
	events map[string][]application.Event
}

func newMemoryAppRepo() *memoryAppRepo {
	return &memoryAppRepo{
		apps:   make(map[string]*application.Application),
		events: make(map[string][]application.Event),
	}
}

func (m *memoryAppRepo) Create(ctx context.Context, app *application.Application) error {
	app.ID = "app-uuid-1"
	app.CreatedAt = time.Now()
	app.UpdatedAt = time.Now()
	m.apps[app.ID] = app
	return nil
}

func (m *memoryAppRepo) GetByID(ctx context.Context, userID, id string) (*application.Application, error) {
	if app, ok := m.apps[id]; ok && app.UserID == userID {
		return app, nil
	}
	return nil, nil
}

func (m *memoryAppRepo) List(ctx context.Context, userID string, filter application.Filter) ([]application.Application, int, error) {
	var results []application.Application
	for _, app := range m.apps {
		if app.UserID == userID {
			results = append(results, *app)
		}
	}
	return results, len(results), nil
}

func (m *memoryAppRepo) Update(ctx context.Context, userID, id string, input application.UpdateInput) (*application.Application, error) {
	app, ok := m.apps[id]
	if !ok || app.UserID != userID {
		return nil, nil
	}

	if input.Company != nil {
		app.Company = *input.Company
	}
	if input.Status != nil {
		app.Status = *input.Status
	}
	app.UpdatedAt = time.Now()
	return app, nil
}

func (m *memoryAppRepo) Delete(ctx context.Context, userID, id string) error {
	app, ok := m.apps[id]
	if !ok || app.UserID != userID {
		return application.ErrNotFound
	}
	delete(m.apps, id)
	return nil
}

func (m *memoryAppRepo) CreateEvent(ctx context.Context, event *application.Event) error {
	event.ID = "event-uuid"
	event.CreatedAt = time.Now()
	m.events[event.ApplicationID] = append(m.events[event.ApplicationID], *event)
	return nil
}

func (m *memoryAppRepo) ListEvents(ctx context.Context, userID, applicationID string) ([]application.Event, error) {
	return m.events[applicationID], nil
}

func TestApplicationHandler_CreateAndGet(t *testing.T) {
	repo := newMemoryAppRepo()
	service := application.NewService(repo)
	handler := application.NewHandler(service)

	// 1. Unauthenticated Request -> 401
	body := bytes.NewBufferString(`{"company":"Google","position":"Backend Engineer","status":"APPLIED"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/applications", body)
	rr := httptest.NewRecorder()
	handler.Create(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401 Unauthorized, got %d", rr.Code)
	}

	// 2. Authenticated Valid Request -> 201
	body = bytes.NewBufferString(`{"company":"Google","position":"Backend Engineer","status":"APPLIED"}`)
	req = httptest.NewRequest(http.MethodPost, "/api/v1/applications", body)
	ctx := context.WithValue(req.Context(), middleware.UserIDKey, "test-user-1")
	rr = httptest.NewRecorder()
	handler.Create(rr, req.WithContext(ctx))

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected status 201 Created, got %d (body: %s)", rr.Code, rr.Body.String())
	}

	var created application.Application
	if err := json.NewDecoder(rr.Body).Decode(&created); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if created.Company != "Google" || created.ID != "app-uuid-1" {
		t.Errorf("unexpected created app: %+v", created)
	}

	// 3. Validation Error -> 400
	badBody := bytes.NewBufferString(`{"company":"","position":""}`)
	req = httptest.NewRequest(http.MethodPost, "/api/v1/applications", badBody)
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, "test-user-1"))
	rr = httptest.NewRecorder()
	handler.Create(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 Bad Request, got %d", rr.Code)
	}

	// 4. List Applications -> 200
	listReq := httptest.NewRequest(http.MethodGet, "/api/v1/applications", nil)
	listReq = listReq.WithContext(context.WithValue(listReq.Context(), middleware.UserIDKey, "test-user-1"))
	rr = httptest.NewRecorder()
	handler.List(rr, listReq)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200 OK, got %d", rr.Code)
	}
}
