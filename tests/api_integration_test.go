package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"job-tracker/pkg/application"
	"job-tracker/pkg/auth"
	"job-tracker/pkg/middleware"
	"job-tracker/pkg/response"
	"job-tracker/pkg/statistics"
	"job-tracker/pkg/user"
)

type inMemoryIntegrationDB struct {
	users        map[string]*user.User
	usersByEmail map[string]*user.User
	apps         map[string]*application.Application
	events       map[string][]application.Event
}

func newInMemoryIntegrationDB() *inMemoryIntegrationDB {
	return &inMemoryIntegrationDB{
		users:        make(map[string]*user.User),
		usersByEmail: make(map[string]*user.User),
		apps:         make(map[string]*application.Application),
		events:       make(map[string][]application.Event),
	}
}

// User repo methods
func (db *inMemoryIntegrationDB) Create(ctx context.Context, u *user.User) error {
	u.ID = "user-100"
	u.CreatedAt = time.Now()
	u.UpdatedAt = time.Now()
	db.users[u.ID] = u
	db.usersByEmail[u.Email] = u
	return nil
}

func (db *inMemoryIntegrationDB) GetByID(ctx context.Context, id string) (*user.User, error) {
	return db.users[id], nil
}

func (db *inMemoryIntegrationDB) GetByEmail(ctx context.Context, email string) (*user.User, error) {
	return db.usersByEmail[email], nil
}

// App repo methods
type appRepoAdapter struct {
	db *inMemoryIntegrationDB
}

func (a *appRepoAdapter) Create(ctx context.Context, app *application.Application) error {
	app.ID = "app-200"
	app.CreatedAt = time.Now()
	app.UpdatedAt = time.Now()
	a.db.apps[app.ID] = app
	return nil
}

func (a *appRepoAdapter) GetByID(ctx context.Context, userID, id string) (*application.Application, error) {
	if app, ok := a.db.apps[id]; ok && app.UserID == userID {
		return app, nil
	}
	return nil, nil
}

func (a *appRepoAdapter) List(ctx context.Context, userID string, filter application.Filter) ([]application.Application, int, error) {
	var list []application.Application
	for _, app := range a.db.apps {
		if app.UserID == userID {
			list = append(list, *app)
		}
	}
	return list, len(list), nil
}

func (a *appRepoAdapter) Update(ctx context.Context, userID, id string, input application.UpdateInput) (*application.Application, error) {
	app, ok := a.db.apps[id]
	if !ok || app.UserID != userID {
		return nil, nil
	}
	if input.Status != nil {
		app.Status = *input.Status
	}
	app.UpdatedAt = time.Now()
	return app, nil
}

func (a *appRepoAdapter) Delete(ctx context.Context, userID, id string) error {
	delete(a.db.apps, id)
	return nil
}

func (a *appRepoAdapter) CreateEvent(ctx context.Context, event *application.Event) error {
	event.ID = "event-300"
	event.CreatedAt = time.Now()
	a.db.events[event.ApplicationID] = append(a.db.events[event.ApplicationID], *event)
	return nil
}

func (a *appRepoAdapter) ListEvents(ctx context.Context, userID, applicationID string) ([]application.Event, error) {
	return a.db.events[applicationID], nil
}

func TestFullAPIWorkflow(t *testing.T) {
	db := newInMemoryIntegrationDB()
	appRepo := &appRepoAdapter{db: db}
	sessionStore := auth.NewMemorySessionStore(1 * time.Hour)

	authService := auth.NewService(db, sessionStore)
	appService := application.NewService(appRepo)
	statsService := statistics.NewService(appRepo)

	authHandler := auth.NewHandler(authService)
	appHandler := application.NewHandler(appService)
	statsHandler := statistics.NewHandler(statsService)

	mux := http.NewServeMux()

	// Public
	mux.HandleFunc("POST /api/v1/auth/register", authHandler.Register)
	mux.HandleFunc("POST /api/v1/auth/login", authHandler.Login)

	// Protected
	authMiddleware := middleware.Authenticate(authHandler)
	wrapAuth := func(h http.HandlerFunc) http.Handler {
		return authMiddleware(h)
	}

	mux.Handle("POST /api/v1/applications", wrapAuth(appHandler.Create))
	mux.Handle("GET /api/v1/applications", wrapAuth(appHandler.List))
	mux.Handle("GET /api/v1/statistics", wrapAuth(statsHandler.Get))

	// 1. Register
	regBody := bytes.NewBufferString(`{"name":"Integration Tester","email":"tester@example.com","password":"mypassword123"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", regBody)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected status 201 Created on register, got %d", rr.Code)
	}

	var regRes struct {
		Token string `json:"token"`
	}
	_ = json.NewDecoder(rr.Body).Decode(&regRes)
	token := regRes.Token

	// 2. Create Application with Bearer Token
	appBody := bytes.NewBufferString(`{"company":"Stripe","position":"Backend Engineer","status":"APPLIED"}`)
	req = httptest.NewRequest(http.MethodPost, "/api/v1/applications", appBody)
	req.Header.Set("Authorization", "Bearer "+token)
	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected status 201 Created on create app, got %d (body: %s)", rr.Code, rr.Body.String())
	}

	// 3. List Applications
	req = httptest.NewRequest(http.MethodGet, "/api/v1/applications", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200 OK on list apps, got %d", rr.Code)
	}

	// 4. Statistics
	req = httptest.NewRequest(http.MethodGet, "/api/v1/statistics", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200 OK on stats, got %d", rr.Code)
	}

	var statsRes response.ErrorResponse
	if rr.Code != 200 {
		_ = json.NewDecoder(rr.Body).Decode(&statsRes)
		t.Fatalf("stats failed with error: %+v", statsRes)
	}
}
