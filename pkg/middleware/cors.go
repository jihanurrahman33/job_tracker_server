package middleware

import (
	"net/http"
	"strings"
)

// CORSOptions configures the CORS middleware.
type CORSOptions struct {
	AllowedOrigins   []string
	AllowedMethods   []string
	AllowedHeaders   []string
	AllowCredentials bool
	MaxAge           int
}

// DefaultCORSOptions provides sensible defaults allowing common frontend origins.
func DefaultCORSOptions() CORSOptions {
	return CORSOptions{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"},
		AllowedHeaders: []string{
			"Accept",
			"Authorization",
			"Content-Type",
			"X-CSRF-Token",
			"X-Request-ID",
			"X-Auth-Token",
			"Origin",
		},
		AllowCredentials: true,
		MaxAge:           86400, // 24 hours
	}
}

// CORS wraps an http.Handler with Cross-Origin Resource Sharing headers and OPTIONS preflight handling.
func CORS(opts CORSOptions) func(http.Handler) http.Handler {
	methodsStr := strings.Join(opts.AllowedMethods, ", ")
	headersStr := strings.Join(opts.AllowedHeaders, ", ")

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")

			// Check allowed origins
			allowOrigin := ""
			for _, o := range opts.AllowedOrigins {
				if o == "*" || (origin != "" && (o == origin || strings.TrimSpace(o) == "*")) {
					allowOrigin = origin
					if o == "*" && !opts.AllowCredentials {
						allowOrigin = "*"
					}
					break
				}
			}

			if allowOrigin == "" && len(opts.AllowedOrigins) > 0 {
				if opts.AllowedOrigins[0] == "*" {
					allowOrigin = "*"
					if origin != "" {
						allowOrigin = origin
					}
				}
			}

			if allowOrigin != "" {
				w.Header().Set("Access-Control-Allow-Origin", allowOrigin)
				w.Header().Set("Vary", "Origin")
			}

			if opts.AllowCredentials && allowOrigin != "*" && allowOrigin != "" {
				w.Header().Set("Access-Control-Allow-Credentials", "true")
			}

			w.Header().Set("Access-Control-Allow-Methods", methodsStr)
			w.Header().Set("Access-Control-Allow-Headers", headersStr)
			w.Header().Set("Access-Control-Expose-Headers", "X-Request-ID, X-RateLimit-Limit, X-RateLimit-Remaining, X-RateLimit-Reset")

			// Handle preflight OPTIONS requests immediately
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
