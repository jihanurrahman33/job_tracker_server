package middleware

import (
	"net/http"
	"regexp"
)

// SecurityHeaders adds defensive headers to every response. The API serves
// JSON only, so a strict policy that forbids any embedded content is safe.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
		h.Set("Cross-Origin-Resource-Policy", "same-site")
		// Responses can hold personal data; handlers that are safe to cache
		// (the public reviews list) set their own Cache-Control.
		if h.Get("Cache-Control") == "" {
			h.Set("Cache-Control", "no-store")
		}
		next.ServeHTTP(w, r)
	})
}

// MaxBodyBytes caps how much of a request body any handler can read.
func MaxBodyBytes(limit int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Body != nil {
				r.Body = http.MaxBytesReader(w, r.Body, limit)
			}
			next.ServeHTTP(w, r)
		})
	}
}

var safeRequestID = regexp.MustCompile(`^[A-Za-z0-9._-]{1,64}$`)
