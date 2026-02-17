package internal

import (
	"encoding/json"
	"net/http"
)

// WriteJSONResponse writes a JSON response with the given status code
func WriteJSONResponse(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

// WriteJSONError writes a JSON error response
func WriteJSONError(w http.ResponseWriter, statusCode int, message string) {
	WriteJSONResponse(w, statusCode, map[string]string{
		"error": message,
	})
}
