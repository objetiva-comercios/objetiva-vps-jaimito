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
	// Rehidratar estados desde DB (D-09): evita alertas duplicadas post-restart.
	states := hydrateStates(ctx, database, cfg.Definitions)

	// Upsert todas las metricas en la DB antes de arrancar los loops.
	// Esto asegura que metric_readings puede FK-referenciar a metrics.
	for _, def := range cfg.Definitions {
		if err := db.UpsertMetric(ctx, database, def.Name, category(def), metricType(def)); err != nil {
			slog.Error("upsert metric failed", "metric", def.Name, "error", err)
			// Continuar — no abortar por un error de upsert individual.
		}
	}

	// Lanzar una goroutine por metrica (D-01).
	for _, def := range cfg.Definitions {
		def := def
		state := states[def.Name]
		if state == nil {
			state = &metricState{currentLevel: "ok"}
		}
		go runMetricLoop(ctx, database, cfg, def, state)
	}
}

// runMetricLoop ejecuta el loop de recoleccion para una metrica individual.
// Por D-01: ticker independiente por metrica.
// Patron startup-then-interval (igual que cleanup.Start): coleccion inmediata al arrancar,
// luego a intervalos regulares.
func runMetricLoop(ctx context.Context, database *sql.DB, cfg *config.MetricsConfig, def config.MetricDef, state *metricState) {
	interval := resolveInterval(def, cfg)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Coleccion inmediata al arrancar (startup-then-interval).
	collectAndEvaluate(ctx, database, cfg, def, state)

	for {
		select {
		case <-ticker.C:
			collectAndEvaluate(ctx, database, cfg, def, state)
		case <-ctx.Done():
			return
		}
	}
}

// collectAndEvaluate ejecuta el comando de una metrica, persiste el resultado y evalua alertas.
// Flujo collect-then-evaluate per D-10:
//  1. Ejecutar comando con timeout
//  2. Si falla: loguear con slog.Error y return (MCOL-05)
//  3. Evaluar nivel con evaluateLevel
//  4. UpdateMetricStatus (last_value + last_status con nivel real)
//  5. InsertReading (timeseries)
//  6. Si shouldAlert: transition + sendAlert
//
// Este comportamiento garantiza que docker_running falla silenciosamente cuando
// Docker no esta instalado (MCOL-02): runCommand retorna error y el loop continua.
func collectAndEvaluate(ctx context.Context, database *sql.DB, cfg *config.MetricsConfig, def config.MetricDef, state *metricState) {
	timeout := computeTimeout(resolveInterval(def, cfg))
	value, err := runCommand(ctx, def.Command, timeout)
	if err != nil {
		slog.Error("metric collection failed",
			"metric", def.Name,
			"error", err,
		)
		return
	}

	// 3. Evaluar nivel contra umbrales configurados
	newLevel := evaluateLevel(value, def.Thresholds)

	// 4. Persistir estado en DB (siempre, D-10)
	if err := db.UpdateMetricStatus(ctx, database, def.Name, value, newLevel); err != nil {
		slog.Error("update metric status failed", "metric", def.Name, "error", err)
	}

	// 5. Insertar reading en timeseries
	if err := db.InsertReading(ctx, database, def.Name, value); err != nil {
		slog.Error("insert reading failed", "metric", def.Name, "error", err)
	}

	// 6. Evaluar si se debe alertar
	cooldown := 30 * time.Minute // default
	if cfg.AlertCooldown != "" {
		if d, err := config.ParseDuration(cfg.AlertCooldown); err == nil {
			cooldown = d
		}
	}

	if state.shouldAlert(newLevel, cooldown) {
		fromLevel := state.currentLevel
		state.transition(newLevel)
		if err := sendAlert(ctx, database, def, value, fromLevel, newLevel); err != nil {
			slog.Error("send alert failed", "metric", def.Name, "error", err)
		}
	} else if state.currentLevel != newLevel {
		// Actualizar el nivel sin enviar alerta (ej: recovery silenciosa dentro de cooldown)
		state.transition(newLevel)
	}
}
