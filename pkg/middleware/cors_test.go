package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"job-tracker/pkg/middleware"
)

func TestCORSMiddleware(t *testing.T) {
	opts := middleware.DefaultCORSOptions()
	opts.AllowedOrigins = []string{"https://example.com", "http://localhost:3000"}

	dummyHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	handler := middleware.CORS(opts)(dummyHandler)

	// 1. Preflight OPTIONS request
	req := httptest.NewRequest(http.MethodOptions, "/api/v1/applications", nil)
	req.Header.Set("Origin", "https://example.com")
	req.Header.Set("Access-Control-Request-Method", "POST")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Errorf("expected preflight status 204 No Content, got %d", rr.Code)
	}
	if rr.Header().Get("Access-Control-Allow-Origin") != "https://example.com" {
		t.Errorf("expected allow origin https://example.com, got %s", rr.Header().Get("Access-Control-Allow-Origin"))
	}

	// 2. Normal GET request
	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/applications", nil)
	getReq.Header.Set("Origin", "http://localhost:3000")
	rrGet := httptest.NewRecorder()

	handler.ServeHTTP(rrGet, getReq)

	if rrGet.Code != http.StatusOK {
		t.Errorf("expected status 200 OK, got %d", rrGet.Code)
	}
	if rrGet.Header().Get("Access-Control-Allow-Origin") != "http://localhost:3000" {
		t.Errorf("expected allow origin http://localhost:3000, got %s", rrGet.Header().Get("Access-Control-Allow-Origin"))
	}
}
