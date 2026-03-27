package api

import (
	"database/sql"
	"net/http"
	"time"

	"github.com/chiguire/jaimito/internal/config"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// NewRouter builds and returns the chi router with middleware stack and routes registered.
// The health endpoint is unauthenticated; the notify endpoint requires a valid Bearer token.
func NewRouter(database *sql.DB, cfg *config.Config) http.Handler {
	r := chi.NewRouter()

	// Global middleware stack.
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(30 * time.Second))

	// Unauthenticated routes.
	r.Get("/api/v1/health", HealthHandler)
	r.Get("/api/v1/metrics", MetricsListHandler(database, cfg))
	r.Get("/api/v1/metrics/{name}/readings", ReadingsHandler(database, cfg))

	// Authenticated routes.
	r.Group(func(r chi.Router) {
		r.Use(BearerAuth(database))
		r.Post("/api/v1/notify", NotifyHandler(database, cfg))
		r.Post("/api/v1/metrics", IngestHandler(database, cfg))
	})

	return r
}
