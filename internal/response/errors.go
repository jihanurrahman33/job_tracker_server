package response

import (
	"net/http"
)

// Standard error codes used across the API
const (
	ErrCodeValidationError = "VALIDATION_ERROR"
	ErrCodeUnauthorized    = "UNAUTHORIZED"
	ErrCodeForbidden       = "FORBIDDEN"
	ErrCodeNotFound        = "NOT_FOUND"
	ErrCodeConflict        = "CONFLICT"
	ErrCodeInvalidStatus   = "INVALID_STATUS"
	ErrCodeInternalError   = "INTERNAL_ERROR"
)

// ErrorResponse represents the standardized JSON error structure.
type ErrorResponse struct {
	Error ErrorBody `json:"error"`
}

// ErrorBody contains the details of an error response.
type ErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details any    `json:"details,omitempty"`
}

// Error sends a standardized JSON error response.
func Error(w http.ResponseWriter, status int, code, message string, details any) {
	JSON(w, status, ErrorResponse{
		Error: ErrorBody{
			Code:    code,
			Message: message,
			Details: details,
		},
	})
}
