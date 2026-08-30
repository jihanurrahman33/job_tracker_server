package auth_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"job-tracker/internal/auth"
)

func TestAuthHandler(t *testing.T) {
	repo := newMockUserRepo()
	sessions := auth.NewMemorySessionStore(1 * time.Hour)
	service := auth.NewService(repo, sessions)
	handler := auth.NewHandler(service)

	// 1. Register User -> 201
	regBody := bytes.NewBufferString(`{"name":"Bob","email":"bob@example.com","password":"mypassword123"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", regBody)
	rr := httptest.NewRecorder()
	handler.Register(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected status 201 Created, got %d (body: %s)", rr.Code, rr.Body.String())
	}

	var regRes map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&regRes); err != nil {
		t.Fatalf("failed to decode json: %v", err)
	}
	token, ok := regRes["token"].(string)
	if !ok || token == "" {
		t.Fatalf("expected token in register response, got none")
	}

	// 2. Login User -> 200
	loginBody := bytes.NewBufferString(`{"email":"bob@example.com","password":"mypassword123"}`)
	req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", loginBody)
	rr = httptest.NewRecorder()
	handler.Login(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200 OK, got %d (body: %s)", rr.Code, rr.Body.String())
	}

	// 3. Logout -> 204
	logoutReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	logoutReq.Header.Set("Authorization", "Bearer "+token)
	rr = httptest.NewRecorder()
	handler.Logout(rr, logoutReq)

	if rr.Code != http.StatusNoContent {
		t.Errorf("expected status 204 No Content, got %d", rr.Code)
	}
}
