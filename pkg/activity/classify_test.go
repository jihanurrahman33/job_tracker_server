package activity

import "testing"

func TestClassify(t *testing.T) {
	cases := []struct {
		method, path string
		status       int
		want         string
	}{
		{"POST", "/api/v1/auth/login", 200, "auth.login"},
		{"POST", "/api/v1/auth/login", 401, "auth.login_failed"},
		{"POST", "/api/v1/auth/login", 403, "auth.login_disabled"},
		{"POST", "/api/v1/auth/login", 429, "auth.login_blocked"},
		{"POST", "/api/v1/auth/login", 400, "auth.login_error"},
		{"POST", "/api/v1/auth/register", 201, "auth.register"},
		{"POST", "/api/v1/auth/register", 409, "auth.register_conflict"},
		{"POST", "/api/v1/auth/register", 429, "auth.register_blocked"},
		{"POST", "/api/v1/auth/logout", 204, "auth.logout"},
		{"POST", "/api/v1/applications", 201, "application.create"},
		{"PATCH", "/api/v1/applications/abc", 200, "application.update"},
		{"DELETE", "/api/v1/reminders/abc", 204, "reminder.delete"},
		{"POST", "/api/v1/applications/abc/interviews", 201, "interview.create"},
		{"PUT", "/api/v1/reviews/me", 200, "review.update"},
		{"PATCH", "/api/v1/admin/users/abc", 200, "admin.user.update"},
		{"DELETE", "/api/v1/admin/reviews/abc", 204, "admin.review.delete"},
		{"GET", "/api/v1/admin/overview", 200, "admin.overview.read"},
		{"GET", "/api/v1/applications", 401, "security.unauthorized"},
		{"GET", "/api/v1/admin/users", 403, "security.forbidden"},
		{"GET", "/api/v1/applications", 429, "security.rate_limited"},
		{"GET", "/nope", 404, "api.other"},
	}
	for _, c := range cases {
		if got := Classify(c.method, c.path, c.status); got != c.want {
			t.Errorf("Classify(%s %s %d) = %q, want %q", c.method, c.path, c.status, got, c.want)
		}
	}
}

func TestShouldRecord(t *testing.T) {
	cases := []struct {
		method, path string
		status       int
		want         bool
	}{
		{"GET", "/health", 200, false},
		{"GET", "/ready", 503, false},
		{"OPTIONS", "/api/v1/applications", 204, false},
		{"GET", "/api/v1/applications", 200, false}, // ordinary read
		{"GET", "/api/v1/reviews", 200, false},
		{"GET", "/api/v1/applications", 401, true}, // failed read
		{"GET", "/api/v1/admin/overview", 200, true},
		{"POST", "/api/v1/applications", 201, true},
		{"POST", "/api/v1/auth/login", 401, true},
		{"DELETE", "/api/v1/reminders/abc", 204, true},
	}
	for _, c := range cases {
		if got := ShouldRecord(c.method, c.path, c.status); got != c.want {
			t.Errorf("ShouldRecord(%s %s %d) = %v, want %v", c.method, c.path, c.status, got, c.want)
		}
	}
}
