package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// writeJSON sends a JSON response with the given status code.
func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

// writeError sends a JSON error response.
func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// errorf creates a simple error with a formatted message.
func errorf(format string, args ...interface{}) error {
	return fmt.Errorf(format, args...)
}
