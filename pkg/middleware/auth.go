package middleware

import (
	"context"
	"net/http"

	"job-tracker/pkg/response"
)

const UserIDKey contextKey = "user_id"

// Authenticator provides an interface for validating authentication credentials.
type Authenticator interface {
	ValidateTokenOrSession(r *http.Request) (string, error)
}

// Authenticate verifies the user's authentication and injects userID into request context.
func Authenticate(auth Authenticator) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID, err := auth.ValidateTokenOrSession(r)
			if err != nil || userID == "" {
				// Never echo the underlying error: it can contain database text.
				response.Error(w, http.StatusUnauthorized, response.ErrCodeUnauthorized, "Authentication required", nil)
				return
			}

			ctx := context.WithValue(r.Context(), UserIDKey, userID)
			SetUserID(ctx, userID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// UserIDFromContext extracts the authenticated user ID from context.
func UserIDFromContext(ctx context.Context) (string, bool) {
	userID, ok := ctx.Value(UserIDKey).(string)
	return userID, ok && userID != ""
}

// RequireAdmin allows a request through only when the authenticated user is an
// active admin. It checks the database on every request, so removing a role or
// disabling an account takes effect immediately. It must run after Authenticate.
func RequireAdmin(isAdmin func(ctx context.Context, userID string) (bool, error)) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID, ok := UserIDFromContext(r.Context())
			if !ok {
				response.Error(w, http.StatusUnauthorized, response.ErrCodeUnauthorized, "Authentication required", nil)
				return
			}
			admin, err := isAdmin(r.Context(), userID)
			if err != nil {
				response.Error(w, http.StatusInternalServerError, response.ErrCodeInternalError, "Failed to verify permissions", nil)
				return
			}
			if !admin {
				response.Error(w, http.StatusForbidden, response.ErrCodeForbidden, "Admin access required", nil)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
