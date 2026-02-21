// Package api provides the HTTP API layer for jaimito.
package api

import (
	"encoding/json"
	"net/http"
)

// ErrorResponse is the standard error envelope returned by API endpoints.
type ErrorResponse struct {
	Error string `json:"error"`
}

// WriteJSON encodes data as JSON and writes it to w with the given status code.
// IMPORTANT: WriteHeader must be called before Encode, otherwise Go auto-sends 200.
func WriteJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data) //nolint:errcheck
}

// WriteError writes a JSON error response with the given status code and message.
func WriteError(w http.ResponseWriter, status int, message string) {
	WriteJSON(w, status, ErrorResponse{Error: message})
}
