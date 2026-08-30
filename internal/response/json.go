package response

import (
	"encoding/json"
	"net/http"
)

// JSON sends a JSON response with the given status code and payload.
func JSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if data != nil {
		if err := json.NewEncoder(w).Encode(data); err != nil {
			http.Error(w, `{"error":{"code":"INTERNAL_ERROR","message":"Failed to encode response"}}`, http.StatusInternalServerError)
		}
	}
}

// NoContent sends a 204 No Content response.
func NoContent(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}
