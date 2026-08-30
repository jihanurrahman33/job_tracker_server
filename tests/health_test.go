package tests

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"job-tracker/internal/response"
)

func TestHealthEndpoints(t *testing.T) {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		response.JSON(w, http.StatusOK, map[string]any{
			"name":    "Job Application Tracker API",
			"version": "1.0",
			"status":  "healthy",
		})
	})

	healthHandler := func(w http.ResponseWriter, r *http.Request) {
		response.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}
	mux.HandleFunc("GET /health", healthHandler)
	mux.HandleFunc("GET /healthz", healthHandler)

	endpoints := []string{"/health", "/healthz"}

	for _, ep := range endpoints {
		t.Run(ep, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, ep, nil)
			rr := httptest.NewRecorder()

			mux.ServeHTTP(rr, req)

			if rr.Code != http.StatusOK {
				t.Errorf("expected status 200 OK, got %d", rr.Code)
			}

			var res map[string]string
			if err := json.NewDecoder(rr.Body).Decode(&res); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}

			if res["status"] != "ok" {
				t.Errorf("expected status 'ok', got %q", res["status"])
			}
		})
	}

	t.Run("root route /", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rr := httptest.NewRecorder()

		mux.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status 200 OK, got %d", rr.Code)
		}

		var res map[string]any
		if err := json.NewDecoder(rr.Body).Decode(&res); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if res["status"] != "healthy" {
			t.Errorf("expected status 'healthy', got %v", res["status"])
		}
	})
}
