package collector

import (
	"context"
	"database/sql"
	"log/slog"
	"time"

	"github.com/chiguire/jaimito/internal/config"
	"github.com/chiguire/jaimito/internal/db"
)

// resolveInterval retorna el intervalo de recoleccion para una metrica.
// Si la metrica tiene Interval definido, lo usa; si no, hereda el CollectInterval global.
// Si hay error de parse (no deberia, ya validado en config.Load), usa 60s como fallback.
func resolveInterval(def config.MetricDef, cfg *config.MetricsConfig) time.Duration {
	src := cfg.CollectInterval
	if def.Interval != "" {
		src = def.Interval
	}
	d, err := config.ParseDuration(src)
	if err != nil {
		slog.Error("invalid interval, using 60s fallback",
			"metric", def.Name,
			"interval", src,
			"error", err,
		)
		return 60 * time.Second
	}
	return d
}

// category retorna la categoria de la metrica.
// Si no esta definida, retorna "custom" como valor por defecto.
func category(def config.MetricDef) string {
	if def.Category != "" {
		return def.Category
	}
	return "custom"
}

// metricType retorna el tipo de la metrica.
// Si no esta definido, retorna "gauge" como valor por defecto.
func metricType(def config.MetricDef) string {
	if def.Type != "" {
		return def.Type
	}
	return "gauge"
}

// Start lanza el loop de recoleccion de metricas.
// Por D-01: una goroutine por metrica con ticker independiente.
// Por D-02: arranca automaticamente si cfg != nil (gestionado por el caller en serve.go).
// Fire-and-forget — no retorna nada, igual que dispatcher.Start() y cleanup.Start().
func Start(ctx context.Context, database *sql.DB, cfg *config.MetricsConfig) {
	// Upsert todas las metricas en la DB antes de arrancar los loops.
	// Esto asegura que metric_readings puede FK-referenciar a metrics.
	for _, def := range cfg.Definitions {
		if err := db.UpsertMetric(ctx, database, def.Name, category(def), metricType(def)); err != nil {
			slog.Error("upsert metric failed", "metric", def.Name, "error", err)
			// Continuar — no abortar por un error de upsert individual.
		}
	}

	// Lanzar una goroutine por metrica.
	for _, def := range cfg.Definitions {
		go runMetricLoop(ctx, database, cfg, def)
	}
}

// runMetricLoop ejecuta el loop de recoleccion para una metrica individual.
// Por D-01: ticker independiente por metrica.
// Patron startup-then-interval (igual que cleanup.Start): coleccion inmediata al arrancar,
// luego a intervalos regulares.
func runMetricLoop(ctx context.Context, database *sql.DB, cfg *config.MetricsConfig, def config.MetricDef) {
	interval := resolveInterval(def, cfg)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Coleccion inmediata al arrancar (startup-then-interval).
	collectAndPersist(ctx, database, cfg, def)

	for {
		select {
		case <-ticker.C:
			collectAndPersist(ctx, database, cfg, def)
		case <-ctx.Done():
			return
		}
	}
}

// collectAndPersist ejecuta el comando de una metrica y persiste el resultado en la DB.
// Flujo collect-then-write per D-10:
//  1. Ejecutar comando con timeout
//  2. Si falla: loguear con slog.Error y return (MCOL-05)
//  3. UpdateMetricStatus (last_value + last_status)
//  4. InsertReading (timeseries)
//
// Este comportamiento garantiza que docker_running falla silenciosamente cuando
// Docker no esta instalado (MCOL-02): runCommand retorna error y el loop continua.
func collectAndPersist(ctx context.Context, database *sql.DB, cfg *config.MetricsConfig, def config.MetricDef) {
	timeout := computeTimeout(resolveInterval(def, cfg))
	value, err := runCommand(ctx, def.Command, timeout)
	if err != nil {
		slog.Error("metric collection failed",
			"metric", def.Name,
			"error", err,
		)
		return
	}

	// Nota: en este plan el status es siempre "ok".
	// La state machine real (ok/warning/critical) se implementa en Plan 02.
	if err := db.UpdateMetricStatus(ctx, database, def.Name, value, "ok"); err != nil {
		slog.Error("update metric status failed",
			"metric", def.Name,
			"error", err,
		)
	}

	if err := db.InsertReading(ctx, database, def.Name, value); err != nil {
		slog.Error("insert reading failed",
			"metric", def.Name,
			"error", err,
		)
	}
}
