package middleware

import (
	"log/slog"
	"net/http"
	"runtime/debug"

	"job-tracker/pkg/response"
)

// Recovery catches panics, logs the stack trace, and sends a 500 error response.
func Recovery(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					reqID := GetRequestID(r.Context())
					logger.Error("panic recovered",
						slog.Any("error", rec),
						slog.String("request_id", reqID),
						slog.String("stack", string(debug.Stack())),
					)

					response.Error(w, http.StatusInternalServerError, response.ErrCodeInternalError, "An unexpected error occurred", nil)
				}
			}()

			next.ServeHTTP(w, r)
		})
	}
}
