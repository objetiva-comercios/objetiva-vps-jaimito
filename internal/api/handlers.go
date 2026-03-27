package api

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/chiguire/jaimito/internal/config"
	"github.com/chiguire/jaimito/internal/db"
	"github.com/go-chi/chi/v5"
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

// ThresholdInfo represents warning/critical thresholds in API responses.
type ThresholdInfo struct {
	Warning  *float64 `json:"warning,omitempty"`
	Critical *float64 `json:"critical,omitempty"`
}

// MetricResponse is returned by GET /api/v1/metrics for each metric.
type MetricResponse struct {
	Name       string         `json:"name"`
	Category   string         `json:"category"`
	Type       string         `json:"type"`
	LastValue  *float64       `json:"last_value"`
	LastStatus string         `json:"last_status"`
	UpdatedAt  string         `json:"updated_at"`
	Thresholds *ThresholdInfo `json:"thresholds,omitempty"` // omitempty — Phase 11 expects no null
}

// ReadingItem is a single reading in the readings response.
type ReadingItem struct {
	Value      float64 `json:"value"`
	RecordedAt string  `json:"recorded_at"`
}

// ReadingsListResponse is returned by GET /api/v1/metrics/{name}/readings.
type ReadingsListResponse struct {
	Metric   string        `json:"metric"`
	Since    string        `json:"since"`
	Readings []ReadingItem `json:"readings"`
}

// IngestRequest is the JSON body for POST /api/v1/metrics.
type IngestRequest struct {
	Name  string   `json:"name"`
	Value *float64 `json:"value"` // pointer to detect missing vs zero
}

// IngestResponse is returned by POST /api/v1/metrics on success (201).
type IngestResponse struct {
	Name       string  `json:"name"`
	Value      float64 `json:"value"`
	RecordedAt string  `json:"recorded_at"`
}

// maxRetention is the maximum duration accepted by the ?since= parameter.
const maxRetention = 7 * 24 * time.Hour

// MetricsListHandler returns an http.HandlerFunc that handles GET /api/v1/metrics.
// It returns all metrics with their thresholds from the config (not from DB).
func MetricsListHandler(database *sql.DB, cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Build threshold lookup map from config definitions.
		threshMap := make(map[string]*config.Thresholds)
		if cfg.Metrics != nil {
			for _, def := range cfg.Metrics.Definitions {
				if def.Thresholds != nil {
					threshMap[def.Name] = def.Thresholds
				}
			}
		}

		rows, err := db.ListMetrics(r.Context(), database)
		if err != nil {
			WriteError(w, http.StatusInternalServerError, "failed to list metrics")
			return
		}

		result := make([]MetricResponse, 0, len(rows))
		for _, m := range rows {
			mr := MetricResponse{
				Name:       m.Name,
				Category:   m.Category,
				Type:       m.Type,
				LastValue:  m.LastValue,
				LastStatus: m.LastStatus,
				UpdatedAt:  m.UpdatedAt,
			}
			if t, ok := threshMap[m.Name]; ok {
				mr.Thresholds = &ThresholdInfo{
					Warning:  t.Warning,
					Critical: t.Critical,
				}
			}
			result = append(result, mr)
		}

		WriteJSON(w, http.StatusOK, result)
	}
}

// ReadingsHandler returns an http.HandlerFunc that handles GET /api/v1/metrics/{name}/readings.
// Accepts optional ?since= parameter (duration string, default 24h, max 7d).
func ReadingsHandler(database *sql.DB, cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		metricName := chi.URLParam(r, "name")

		exists, err := db.MetricExists(r.Context(), database, metricName)
		if err != nil {
			WriteError(w, http.StatusInternalServerError, "failed to check metric")
			return
		}
		if !exists {
			WriteError(w, http.StatusNotFound, "metric not found")
			return
		}

		sinceStr := r.URL.Query().Get("since")
		if sinceStr == "" {
			sinceStr = "24h"
		}

		dur, err := config.ParseDuration(sinceStr)
		if err != nil {
			WriteError(w, http.StatusBadRequest, "invalid since parameter: "+err.Error())
			return
		}

		if dur > maxRetention {
			WriteError(w, http.StatusBadRequest, "max retention is 7d")
			return
		}

		since := time.Now().UTC().Add(-dur)
		readings, err := db.QueryReadings(r.Context(), database, metricName, since)
		if err != nil {
			WriteError(w, http.StatusInternalServerError, "failed to query readings")
			return
		}

		items := make([]ReadingItem, 0, len(readings))
		for _, rr := range readings {
			items = append(items, ReadingItem{
				Value:      rr.Value,
				RecordedAt: rr.RecordedAt,
			})
		}

		WriteJSON(w, http.StatusOK, ReadingsListResponse{
			Metric:   metricName,
			Since:    sinceStr,
			Readings: items,
		})
	}
}

// IngestHandler returns an http.HandlerFunc that handles POST /api/v1/metrics.
// Requires Bearer auth. Inserts a reading and updates metric status to "ok".
func IngestHandler(database *sql.DB, cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

		var req IngestRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			WriteError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}

		if req.Name == "" {
			WriteError(w, http.StatusBadRequest, "name is required")
			return
		}
		if req.Value == nil {
			WriteError(w, http.StatusBadRequest, "value is required")
			return
		}

		exists, err := db.MetricExists(r.Context(), database, req.Name)
		if err != nil {
			WriteError(w, http.StatusInternalServerError, "failed to check metric")
			return
		}
		if !exists {
			WriteError(w, http.StatusNotFound, "metric not found")
			return
		}

		if err := db.InsertReading(r.Context(), database, req.Name, *req.Value); err != nil {
			WriteError(w, http.StatusInternalServerError, "failed to insert reading")
			return
		}

		// Update status to "ok" — evaluator is the only one that evaluates thresholds (D-10).
		if err := db.UpdateMetricStatus(r.Context(), database, req.Name, *req.Value, "ok"); err != nil {
			WriteError(w, http.StatusInternalServerError, "failed to update metric status")
			return
		}

		recordedAt := time.Now().UTC().Format(time.RFC3339)
		WriteJSON(w, http.StatusCreated, IngestResponse{
			Name:       req.Name,
			Value:      *req.Value,
			RecordedAt: recordedAt,
		})
	}
}
