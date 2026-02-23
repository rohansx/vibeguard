package handlers

import (
	"encoding/json"
	"net/http"
)

// Placeholder returns a handler that indicates an endpoint is not yet implemented.
func Placeholder(name string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotImplemented)
		json.NewEncoder(w).Encode(map[string]string{
			"error":    "not implemented",
			"endpoint": name,
		})
	}
}
