package api

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/chiguire/jaimito/internal/config"
	"github.com/chiguire/jaimito/internal/db"
	"github.com/google/uuid"
)

// NotifyRequest is the JSON body accepted by POST /api/v1/notify.
type NotifyRequest struct {
	Title    *string        `json:"title"`
	Body     string         `json:"body"`
	Channel  string         `json:"channel"`
	Priority string         `json:"priority"`
	Tags     []string       `json:"tags,omitempty"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

// NotifyResponse is returned by POST /api/v1/notify on success (202 Accepted).
type NotifyResponse struct {
	ID string `json:"id"`
}

// HealthResponse is returned by GET /api/v1/health (200 OK).
type HealthResponse struct {
	Status  string `json:"status"`
	Service string `json:"service"`
}

// validPriority reports whether p is an accepted priority value.
func validPriority(p string) bool {
	return p == "low" || p == "normal" || p == "high"
}

// NotifyHandler returns an http.HandlerFunc that handles POST /api/v1/notify.
// It validates the request, enqueues the message, and responds 202 with the message ID.
func NotifyHandler(database *sql.DB, cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Limit body size to 1 MB.
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

		var req NotifyRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			WriteError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}

		// Apply defaults per locked decisions in CONTEXT.md.
		if req.Channel == "" {
			req.Channel = "general"
		}
		if req.Priority == "" {
			req.Priority = "normal"
		}

		// Validate required fields and enumerations.
		if req.Body == "" {
			WriteError(w, http.StatusBadRequest, "body is required")
			return
		}
		if !cfg.ChannelExists(req.Channel) {
			WriteError(w, http.StatusBadRequest, fmt.Sprintf("unknown channel: %q", req.Channel))
			return
		}
		if !validPriority(req.Priority) {
			WriteError(w, http.StatusBadRequest, fmt.Sprintf("invalid priority: %q", req.Priority))
			return
		}

		// Generate UUID v7 as the stable message ID.
		id, err := uuid.NewV7()
		if err != nil {
			WriteError(w, http.StatusInternalServerError, "failed to generate message ID")
			return
		}

		// Enqueue the message into the database.
		if err := db.EnqueueMessage(r.Context(), database, id.String(), req.Channel, req.Priority, req.Title, req.Body, req.Tags, req.Metadata); err != nil {
			WriteError(w, http.StatusInternalServerError, "failed to enqueue message")
			return
		}

		WriteJSON(w, http.StatusAccepted, NotifyResponse{ID: id.String()})
	}
}

// HealthHandler handles GET /api/v1/health and returns a simple status response.
func HealthHandler(w http.ResponseWriter, r *http.Request) {
	WriteJSON(w, http.StatusOK, HealthResponse{Status: "ok", Service: "jaimito"})
}
