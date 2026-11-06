package server

import (
	"encoding/json"
	"net/http"
)

// writeJSON answers status with v as JSON.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// apiError is the body of every error answer: {"error": "..."}.
type apiError struct {
	Error string `json:"error"`
}

// writeError answers status with {"error": msg}.
func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, apiError{Error: msg})
}
