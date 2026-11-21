package auth_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"job-tracker/pkg/auth"
)

func newHandler(repo *mockUserRepo) *auth.Handler {
	return auth.NewHandler(auth.NewService(repo, auth.NewMemorySessionStore(time.Hour)))
}

func post(h http.HandlerFunc, path, body string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h(rec, httptest.NewRequest(http.MethodPost, path, bytes.NewBufferString(body)))
	return rec
}

func TestEmailIsCaseInsensitive(t *testing.T) {
	h := newHandler(newMockUserRepo())

	if rec := post(h.Register, "/r", `{"name":"Bob","email":"Bob@Example.COM","password":"longenough1"}`); rec.Code != http.StatusCreated {
		t.Fatalf("register: %d %s", rec.Code, rec.Body.String())
	}
	if rec := post(h.Login, "/l", `{"email":"bob@example.com","password":"longenough1"}`); rec.Code != http.StatusOK {
		t.Errorf("login with different case: %d, want 200", rec.Code)
	}
	if rec := post(h.Register, "/r", `{"name":"Bob 2","email":"BOB@example.com","password":"longenough1"}`); rec.Code != http.StatusConflict {
		t.Errorf("same address in another case must be a duplicate: %d, want 409", rec.Code)
	}
}

func TestRegisterValidation(t *testing.T) {
	h := newHandler(newMockUserRepo())
	cases := map[string]string{
		"short password":    `{"email":"a@b.co","password":"1234567"}`,
		"long password":     `{"email":"a@b.co","password":"` + strings.Repeat("x", 129) + `"}`,
		"not an email":      `{"email":"not-an-email","password":"longenough1"}`,
		"display-name form": `{"email":"Name <a@b.co>","password":"longenough1"}`,
		"long name":         `{"name":"` + strings.Repeat("n", 101) + `","email":"a@b.co","password":"longenough1"}`,
		"missing email":     `{"password":"longenough1"}`,
	}
	for name, body := range cases {
		if rec := post(h.Register, "/r", body); rec.Code != http.StatusBadRequest {
			t.Errorf("%s: status %d, want 400", name, rec.Code)
		}
	}
	if rec := post(h.Register, "/r", `{"email":"ok@b.co","password":"12345678"}`); rec.Code != http.StatusCreated {
		t.Errorf("an 8-character password is allowed: %d", rec.Code)
	}
}

func TestDisabledAccountCannotSignIn(t *testing.T) {
	repo := newMockUserRepo()
	h := newHandler(repo)
	post(h.Register, "/r", `{"email":"dis@b.co","password":"longenough1"}`)
	repo.byEmail["dis@b.co"].Disabled = true

	if rec := post(h.Login, "/l", `{"email":"dis@b.co","password":"longenough1"}`); rec.Code != http.StatusForbidden {
		t.Errorf("correct password, disabled: %d, want 403", rec.Code)
	}
	// Without the right password the response must not reveal the account is disabled.
	if rec := post(h.Login, "/l", `{"email":"dis@b.co","password":"wrongpassword"}`); rec.Code != http.StatusUnauthorized {
		t.Errorf("wrong password, disabled: %d, want 401", rec.Code)
	}
	if rec := post(h.Login, "/l", `{"email":"nobody@b.co","password":"wrongpassword"}`); rec.Code != http.StatusUnauthorized {
		t.Errorf("unknown email: %d, want 401", rec.Code)
	}
}

func TestTokenInQueryStringIsRejected(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/me?token=abc123", nil)
	if got := auth.ExtractBearerToken(req); got != "" {
		t.Errorf("a token in the URL must be ignored, got %q", got)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	req.Header.Set("Authorization", "Bearer abc123")
	if got := auth.ExtractBearerToken(req); got != "abc123" {
		t.Errorf("Authorization header token = %q", got)
	}
}

type fakeLimiter struct{ loginBlocked, registerBlocked bool }

func (f fakeLimiter) LoginBlocked(context.Context, string, string) (bool, error) {
	return f.loginBlocked, nil
}
func (f fakeLimiter) RegisterBlocked(context.Context, string) (bool, error) {
	return f.registerBlocked, nil
}

func TestLockoutBlocksLoginAndRegister(t *testing.T) {
	repo := newMockUserRepo()
	h := newHandler(repo)
	post(h.Register, "/r", `{"email":"lock@b.co","password":"longenough1"}`)

	h.SetLimiter(fakeLimiter{loginBlocked: true, registerBlocked: true})

	rec := post(h.Login, "/l", `{"email":"lock@b.co","password":"longenough1"}`)
	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("blocked login: %d, want 429", rec.Code)
	}
	if rec.Header().Get("Retry-After") == "" {
		t.Error("blocked login should say when to retry")
	}
	if rec := post(h.Register, "/r", `{"email":"new@b.co","password":"longenough1"}`); rec.Code != http.StatusTooManyRequests {
		t.Errorf("blocked register: %d, want 429", rec.Code)
	}
	if _, exists := repo.byEmail["new@b.co"]; exists {
		t.Error("a blocked registration must not create the account")
	}

	h.SetLimiter(fakeLimiter{})
	if rec := post(h.Login, "/l", `{"email":"lock@b.co","password":"longenough1"}`); rec.Code != http.StatusOK {
		t.Errorf("unblocked login: %d, want 200", rec.Code)
	}
}
