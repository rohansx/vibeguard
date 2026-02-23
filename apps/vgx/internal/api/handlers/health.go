package handlers

import (
	"encoding/json"
	"net/http"
)

type HealthHandler struct{}

func (h *HealthHandler) Check(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "ok",
		"service": "vgx",
		"version": "0.1.0",
	})
}
