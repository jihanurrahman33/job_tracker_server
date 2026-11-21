package review

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"job-tracker/pkg/middleware"
)

func TestDisplayName(t *testing.T) {
	cases := map[string]string{
		"":                  "A JobTracker user",
		"   ":               "A JobTracker user",
		"Jihanur":           "Jihanur",
		"Md Jihanur Rahman": "Md R.",
		"sarah connor":      "sarah C.",
		"Élodie  Évrard":    "Élodie É.",
	}
	for in, want := range cases {
		if got := DisplayName(in); got != want {
			t.Errorf("DisplayName(%q) = %q, want %q", in, got, want)
		}
	}
}

// fakeRepo records calls so handler validation can be tested without a database.
type fakeRepo struct{ upserts int }

func (f *fakeRepo) Upsert(_ context.Context, _ string, rating int, comment string) (*Review, error) {
	f.upserts++
	return &Review{ID: "r1", Rating: rating, Comment: comment}, nil
}
func (f *fakeRepo) GetByUser(context.Context, string) (*Review, error) { return nil, nil }
func (f *fakeRepo) DeleteByUser(context.Context, string) error         { return nil }
func (f *fakeRepo) ListPublic(context.Context, int) ([]PublicReview, error) {
	return []PublicReview{}, nil
}
func (f *fakeRepo) Summary(context.Context) (*Summary, error) {
	return &Summary{Distribution: map[string]int{}}, nil
}

func saveAs(t *testing.T, repo *fakeRepo, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPut, "/api/v1/reviews/me", bytes.NewBufferString(body))
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, "user-1"))
	rec := httptest.NewRecorder()
	NewHandler(repo).SaveMine(rec, req)
	return rec
}

func TestSaveMineValidation(t *testing.T) {
	repo := &fakeRepo{}

	for _, body := range []string{
		`{"rating":0,"comment":"x"}`,
		`{"rating":6}`,
		`not json`,
	} {
		if rec := saveAs(t, repo, body); rec.Code != http.StatusBadRequest {
			t.Errorf("body %s: status %d, want 400", body, rec.Code)
		}
	}
	if repo.upserts != 0 {
		t.Fatalf("invalid reviews must not be saved, got %d upserts", repo.upserts)
	}

	if rec := saveAs(t, repo, `{"rating":5,"comment":"Helpful"}`); rec.Code != http.StatusOK {
		t.Errorf("valid review: status %d, want 200", rec.Code)
	}
	if repo.upserts != 1 {
		t.Errorf("valid review should be saved once, got %d", repo.upserts)
	}
}
