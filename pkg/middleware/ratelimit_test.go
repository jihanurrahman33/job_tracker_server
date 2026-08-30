package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"job-tracker/pkg/middleware"
)

func TestRateLimiter(t *testing.T) {
	// 2 tokens burst capacity, 1 token per second
	limiter := middleware.NewRateLimiter(1, 2)

	dummyHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	handler := limiter.Limit()(dummyHandler)

	// First 2 requests should succeed (burst = 2)
	for i := 1; i <= 2; i++ {
		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		req.RemoteAddr = "192.168.1.100:1234"
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("request %d expected status 200 OK, got %d", i, rr.Code)
		}
	}

	// 3rd request should exceed rate limit -> 429
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.RemoteAddr = "192.168.1.100:1234"
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusTooManyRequests {
		t.Errorf("expected rate limited status 429, got %d", rr.Code)
	}
}
