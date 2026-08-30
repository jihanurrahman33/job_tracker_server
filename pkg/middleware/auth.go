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
				msg := "Authentication required"
				if err != nil && err.Error() != "" {
					msg = err.Error()
				}
				response.Error(w, http.StatusUnauthorized, response.ErrCodeUnauthorized, msg, nil)
				return
			}

			ctx := context.WithValue(r.Context(), UserIDKey, userID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// UserIDFromContext extracts the authenticated user ID from context.
func UserIDFromContext(ctx context.Context) (string, bool) {
	userID, ok := ctx.Value(UserIDKey).(string)
	return userID, ok && userID != ""
}
