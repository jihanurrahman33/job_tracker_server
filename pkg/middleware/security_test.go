package middleware_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"job-tracker/pkg/middleware"
)

func ok() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
}

func TestSecurityHeaders(t *testing.T) {
	rec := httptest.NewRecorder()
	middleware.SecurityHeaders(ok()).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	for k, want := range map[string]string{
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":        "DENY",
		"Referrer-Policy":        "no-referrer",
		"Cache-Control":          "no-store",
	} {
		if got := rec.Header().Get(k); got != want {
			t.Errorf("%s = %q, want %q", k, got, want)
		}
	}
	if rec.Header().Get("Content-Security-Policy") == "" {
		t.Error("missing Content-Security-Policy")
	}
}

func TestSecurityHeadersKeepHandlerCacheControl(t *testing.T) {
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=30")
	})
	// The header is set by the inner handler after SecurityHeaders ran, so
	// SecurityHeaders must not clobber one that is already present.
	rec := httptest.NewRecorder()
	rec.Header().Set("Cache-Control", "public, max-age=30")
	middleware.SecurityHeaders(h).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if got := rec.Header().Get("Cache-Control"); got != "public, max-age=30" {
		t.Errorf("Cache-Control = %q", got)
	}
}

func TestMaxBodyBytes(t *testing.T) {
	var readErr error
	h := middleware.MaxBodyBytes(10)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, readErr = io.ReadAll(r.Body)
	}))
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/", strings.NewReader(strings.Repeat("x", 100))))
	if readErr == nil {
		t.Error("expected an error reading a body over the limit")
	}
}

func TestRequestIDSanitised(t *testing.T) {
	h := middleware.RequestID(ok())

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Request-ID", "good-id_1.2")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if got := rec.Header().Get("X-Request-ID"); got != "good-id_1.2" {
		t.Errorf("a plain ID should be kept, got %q", got)
	}

	for _, bad := range []string{"has space", "<script>", strings.Repeat("a", 65)} {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("X-Request-ID", bad)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if got := rec.Header().Get("X-Request-ID"); got == bad || got == "" {
			t.Errorf("unsafe ID %q should be replaced, got %q", bad, got)
		}
	}
}

func TestClientIPIgnoresSpoofedHeaders(t *testing.T) {
	t.Setenv("PROXY_SHARED_SECRET", "")
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "203.0.113.9:4444"
	req.Header.Set("CF-Connecting-IP", "1.2.3.4")
	req.Header.Set("X-Forwarded-For", "5.6.7.8")
	req.Header.Set("X-Real-IP", "9.9.9.9")

	// With no trusted proxy configured (the test environment), headers a
	// client can set must not be believed.
	if got := middleware.ClientIP(req); got != "203.0.113.9" {
		t.Errorf("ClientIP = %q, want the TCP peer 203.0.113.9", got)
	}
}

func TestClientIPFromSignedProxy(t *testing.T) {
	t.Setenv("PROXY_SHARED_SECRET", "s3cret-value")

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.1:1"
	req.Header.Set("X-Client-IP", "198.51.100.7")

	req.Header.Set("X-Proxy-Secret", "wrong")
	if got := middleware.ClientIP(req); got != "10.0.0.1" {
		t.Errorf("wrong secret: ClientIP = %q, want the peer", got)
	}

	req.Header.Set("X-Proxy-Secret", "s3cret-value")
	if got := middleware.ClientIP(req); got != "198.51.100.7" {
		t.Errorf("right secret: ClientIP = %q, want 198.51.100.7", got)
	}

	req.Header.Set("X-Client-IP", "not an ip")
	if got := middleware.ClientIP(req); got != "10.0.0.1" {
		t.Errorf("invalid forwarded IP: ClientIP = %q, want the peer", got)
	}
}

func TestRequireAdmin(t *testing.T) {
	check := func(admin bool, err error) func(context.Context, string) (bool, error) {
		return func(context.Context, string) (bool, error) { return admin, err }
	}
	serve := func(h http.Handler, withUser bool) int {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		if withUser {
			req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, "u1"))
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec.Code
	}

	if code := serve(middleware.RequireAdmin(check(true, nil))(ok()), true); code != http.StatusOK {
		t.Errorf("admin: %d, want 200", code)
	}
	if code := serve(middleware.RequireAdmin(check(false, nil))(ok()), true); code != http.StatusForbidden {
		t.Errorf("non-admin: %d, want 403", code)
	}
	if code := serve(middleware.RequireAdmin(check(true, nil))(ok()), false); code != http.StatusUnauthorized {
		t.Errorf("no user: %d, want 401", code)
	}
	if code := serve(middleware.RequireAdmin(check(false, context.DeadlineExceeded))(ok()), true); code != http.StatusInternalServerError {
		t.Errorf("lookup error must not grant access: %d, want 500", code)
	}
}
