package admin

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

type fakeStore struct {
	disabledCalls int
	setErr        error
	lastFilter    ActivityFilter
}

func (f *fakeStore) Overview(context.Context) (*Overview, error) { return &Overview{}, nil }
func (f *fakeStore) ListUsers(context.Context, string, int, int) ([]UserRow, int, error) {
	return []UserRow{}, 0, nil
}
func (f *fakeStore) SetDisabled(context.Context, string, bool) error {
	f.disabledCalls++
	return f.setErr
}
func (f *fakeStore) ListActivity(_ context.Context, fl ActivityFilter) ([]ActivityItem, int, error) {
	f.lastFilter = fl
	return []ActivityItem{}, 0, nil
}
func (f *fakeStore) ListReviews(context.Context, int, int) ([]ReviewRow, int, error) {
	return []ReviewRow{}, 0, nil
}
func (f *fakeStore) DeleteReview(context.Context, string) error { return f.setErr }

const goodID = "a4452dd4-1c19-4c8d-99e2-90e6ccb1692d"

func patchUser(t *testing.T, h *Handler, id, body string) *httptest.ResponseRecorder {
	t.Helper()
	// The id goes in through the path value, as the router would supply it, so
	// hostile ids need not be valid in a URL.
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/admin/users/x", bytes.NewBufferString(body))
	req.SetPathValue("id", id)
	rec := httptest.NewRecorder()
	h.UpdateUser(rec, req)
	return rec
}

func TestUpdateUserValidation(t *testing.T) {
	store := &fakeStore{}
	h := NewHandler(store)

	for name, tc := range map[string]struct{ id, body string }{
		"bad id":         {"not-a-uuid", `{"disabled":true}`},
		"sql in id":      {"1' OR '1'='1", `{"disabled":true}`},
		"missing field":  {goodID, `{}`},
		"wrong type":     {goodID, `{"disabled":"yes"}`},
		"not json":       {goodID, `nope`},
		"role not taken": {goodID, `{"role":"admin"}`},
	} {
		if rec := patchUser(t, h, tc.id, tc.body); rec.Code != http.StatusBadRequest {
			t.Errorf("%s: status %d, want 400", name, rec.Code)
		}
	}
	if store.disabledCalls != 0 {
		t.Fatalf("invalid requests must not reach the store, got %d calls", store.disabledCalls)
	}

	if rec := patchUser(t, h, goodID, `{"disabled":true}`); rec.Code != http.StatusOK {
		t.Errorf("valid request: status %d, want 200", rec.Code)
	}

	store.setErr = ErrNotFound
	if rec := patchUser(t, h, goodID, `{"disabled":true}`); rec.Code != http.StatusNotFound {
		t.Errorf("admin or missing target: status %d, want 404", rec.Code)
	}
}

func TestActivityFilterValidation(t *testing.T) {
	store := &fakeStore{}
	h := NewHandler(store)

	get := func(params url.Values) int {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/activity?"+params.Encode(), nil)
		rec := httptest.NewRecorder()
		h.Activity(rec, req)
		return rec.Code
	}

	for _, bad := range []url.Values{
		{"action": {"x'; DROP TABLE users;--"}},
		{"action": {"AUTH.LOGIN"}},
		{"user_id": {"nope"}},
		{"min_status": {"42"}},
		{"min_status": {"abc"}},
	} {
		if code := get(bad); code != http.StatusBadRequest {
			t.Errorf("%v: status %d, want 400", bad, code)
		}
	}

	if code := get(url.Values{"action": {"auth."}, "min_status": {"400"}, "limit": {"500"}}); code != http.StatusOK {
		t.Fatalf("valid filters: status %d, want 200", code)
	}
	if store.lastFilter.Action != "auth." || store.lastFilter.MinStatus != 400 {
		t.Errorf("filter not passed through: %+v", store.lastFilter)
	}
	if store.lastFilter.Limit != 25 {
		t.Errorf("an over-large limit should fall back to the default, got %d", store.lastFilter.Limit)
	}
}

func TestDeleteReviewValidation(t *testing.T) {
	h := NewHandler(&fakeStore{})
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/admin/reviews/zzz", nil)
	req.SetPathValue("id", "zzz")
	rec := httptest.NewRecorder()
	h.DeleteReview(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("bad id: status %d, want 400", rec.Code)
	}
}

func TestEscapeLike(t *testing.T) {
	if got := escapeLike(`100%_a\b`); got != `100\%\_a\\b` {
		t.Errorf("escapeLike = %q", got)
	}
}
