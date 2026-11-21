package activity

import (
	"net/http"
	"strings"
)

var resourceNames = map[string]string{
	"applications": "application",
	"interviews":   "interview",
	"reminders":    "reminder",
	"reviews":      "review",
	"statistics":   "statistics",
	"me":           "profile",
}

func verb(method string) string {
	switch method {
	case http.MethodPost:
		return "create"
	case http.MethodPut, http.MethodPatch:
		return "update"
	case http.MethodDelete:
		return "delete"
	default:
		return "read"
	}
}

// Classify names what a request was, from its route and outcome. Sign-in and
// security outcomes get their own names so they are easy to filter and count.
func Classify(method, path string, status int) string {
	p := strings.TrimSuffix(path, "/")

	switch p {
	case "/api/v1/auth/login":
		switch {
		case status == http.StatusOK:
			return "auth.login"
		case status == http.StatusTooManyRequests:
			return "auth.login_blocked"
		case status == http.StatusUnauthorized:
			return "auth.login_failed"
		case status == http.StatusForbidden:
			return "auth.login_disabled"
		default:
			return "auth.login_error"
		}
	case "/api/v1/auth/register":
		switch {
		case status == http.StatusCreated:
			return "auth.register"
		case status == http.StatusConflict:
			return "auth.register_conflict"
		case status == http.StatusTooManyRequests:
			return "auth.register_blocked"
		default:
			return "auth.register_failed"
		}
	case "/api/v1/auth/logout":
		return "auth.logout"
	}

	switch status {
	case http.StatusTooManyRequests:
		return "security.rate_limited"
	case http.StatusUnauthorized:
		return "security.unauthorized"
	case http.StatusForbidden:
		return "security.forbidden"
	}

	rest := strings.TrimPrefix(p, "/api/v1/")
	if rest == p {
		return "api.other"
	}
	parts := strings.Split(rest, "/")

	if parts[0] == "admin" {
		res := "overview"
		if len(parts) > 1 {
			res = strings.TrimSuffix(parts[1], "s")
		}
		return "admin." + res + "." + verb(method)
	}

	// /applications/{id}/interviews and /applications/{id}/events act on the
	// nested resource.
	res := parts[0]
	if len(parts) >= 3 {
		res = parts[2]
	}
	name, ok := resourceNames[res]
	if !ok {
		return "api.other"
	}
	return name + "." + verb(method)
}

// ShouldRecord decides which requests are worth a row. Every change, every
// failure and every admin action is recorded. Ordinary successful reads are
// not: they would swamp the log and add a database write to each page view.
func ShouldRecord(method, path string, status int) bool {
	switch path {
	case "/", "/health", "/healthz", "/ready":
		return false
	}
	if method == http.MethodOptions || method == http.MethodHead {
		return false
	}
	if method != http.MethodGet {
		return true
	}
	return status >= 400 || strings.HasPrefix(path, "/api/v1/admin/")
}
