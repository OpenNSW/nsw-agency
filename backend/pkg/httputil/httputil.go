// Package httputil provides common HTTP utilities for Agency services.
package httputil

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

// WriteJSONResponse sends a JSON-encoded response with the given status code.
func WriteJSONResponse(w http.ResponseWriter, statusCode int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		slog.Error("failed to encode JSON response", "error", err)
	}
}

// WriteJSONError sends a JSON-encoded error response with the given status code.
func WriteJSONError(w http.ResponseWriter, statusCode int, message string) {
	WriteJSONResponse(w, statusCode, map[string]string{
		"error": message,
	})
}
