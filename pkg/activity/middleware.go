package activity

import (
	"net/http"
	"time"

	"job-tracker/pkg/middleware"
)

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

// Middleware records requests to the activity log. It must wrap the auth
// middleware, since the user is learned while the request is handled.
func Middleware(rec *Recorder) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ctx, meta := middleware.WithRequestMeta(r.Context())
			sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}

			next.ServeHTTP(sw, r.WithContext(ctx))

			if !ShouldRecord(r.Method, r.URL.Path, sw.status) {
				return
			}
			rec.Record(ctx, Event{
				RequestID:  middleware.GetRequestID(ctx),
				UserID:     meta.UserID,
				Action:     Classify(r.Method, r.URL.Path, sw.status),
				Method:     r.Method,
				Path:       r.URL.Path,
				Status:     sw.status,
				DurationMS: int(time.Since(start).Milliseconds()),
				IP:         middleware.ClientIP(r),
				UserAgent:  r.UserAgent(),
				Subject:    meta.Subject,
			})
		})
	}
}
