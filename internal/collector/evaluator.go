package collector

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/chiguire/jaimito/internal/config"
	"github.com/chiguire/jaimito/internal/db"
	"github.com/google/uuid"
)

// metricState almacena el estado de alerta para una metrica individual.
// Por D-09, D-10, D-11: protegido por mutex para acceso concurrente desde goroutines.
type metricState struct {
	mu           sync.Mutex
	currentLevel string    // "ok" | "warning" | "critical"
	lastAlert    time.Time // timestamp del ultimo envio de alerta
}

// shouldAlert determina si se debe enviar una alerta para una transicion de nivel.
// Retorna true solo cuando hay una transicion real de nivel fuera del cooldown,
// o cuando se produce una recuperacion (cualquier nivel -> ok).
func (s *metricState) shouldAlert(newLevel string, cooldown time.Duration) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Mismo nivel: no alertar
	if newLevel == s.currentLevel {
		return false
	}

	// Recovery (warning/critical -> ok): siempre alertar
	if newLevel == "ok" && s.currentLevel != "ok" {
		return true
	}

	// ok -> ok: no alertar
	if newLevel == "ok" && s.currentLevel == "ok" {
		return false
	}

	// Transicion de nivel (ok->warning, ok->critical, warning->critical):
	// verificar cooldown
	if !s.lastAlert.IsZero() && time.Since(s.lastAlert) < cooldown {
		return false
	}

	return true
}

// transition actualiza el nivel actual y registra el timestamp de la alerta.
// Debe llamarse DESPUES de shouldAlert retornar true y ANTES de sendAlert.
func (s *metricState) transition(newLevel string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.currentLevel = newLevel
	s.lastAlert = time.Now()
}

// evaluateLevel clasifica un valor contra los umbrales configurados.
// Retorna "critical" si value >= critical, "warning" si value >= warning, "ok" en otro caso.
// Si thresholds es nil, siempre retorna "ok".
func evaluateLevel(value float64, thresholds *config.Thresholds) string {
	if thresholds == nil {
		return "ok"
	}
	if thresholds.Critical != nil && value >= *thresholds.Critical {
		return "critical"
	}
	if thresholds.Warning != nil && value >= *thresholds.Warning {
		return "warning"
	}
	return "ok"
}

// hydrateStates carga el estado previo de las metricas desde la DB al arrancar.
// Por D-09: evita alertas duplicadas post-restart al conocer el ultimo level.
// Si ListMetrics falla, retorna un mapa con todos los estados en "ok" (safe default).
func hydrateStates(ctx context.Context, database *sql.DB, defs []config.MetricDef) map[string]*metricState {
	states := make(map[string]*metricState, len(defs))

	// Inicializar todos en "ok" por defecto
	for _, def := range defs {
		states[def.Name] = &metricState{currentLevel: "ok"}
	}

	rows, err := db.ListMetrics(ctx, database)
	if err != nil {
		slog.Error("hydrateStates: failed to list metrics, using ok defaults", "error", err)
		return states
	}

	// Construir mapa de name -> LastStatus desde la DB
	dbStatus := make(map[string]string, len(rows))
	for _, row := range rows {
		if row.LastStatus != "" {
			dbStatus[row.Name] = row.LastStatus
		}
	}

	// Aplicar el estado de la DB a cada def
	for _, def := range defs {
		if status, ok := dbStatus[def.Name]; ok {
			states[def.Name] = &metricState{currentLevel: status}
		}
	}

	return states
}

// levelToPriority convierte un nivel de alerta a una prioridad de mensaje.
// Por D-07: warning -> "high" (emoji rojo), critical -> "critical" (emoji rojo), ok -> "normal".
func levelToPriority(level string) string {
	switch level {
	case "warning":
		return "high"
	case "critical":
		return "critical"
	default:
		return "normal"
	}
}

// sendAlert encola un mensaje de alerta en la DB via db.EnqueueMessage.
// Por D-06, D-07, D-08: formato compacto con nombre de metrica, transicion y valor.
// El canal siempre es "general".
func sendAlert(ctx context.Context, database *sql.DB, def config.MetricDef, value float64, fromLevel, toLevel string) error {
	hostname, _ := os.Hostname()

	title := fmt.Sprintf("%s: %s -> %s", def.Name, fromLevel, toLevel)

	var body string
	if toLevel == "ok" {
		// Mensaje de recovery
		body = fmt.Sprintf("valor: %.2f | recuperado | host: %s", value, hostname)
	} else {
		// Determinar el umbral relevante segun el nivel destino
		var thresholdValue float64
		if toLevel == "critical" && def.Thresholds != nil && def.Thresholds.Critical != nil {
			thresholdValue = *def.Thresholds.Critical
		} else if def.Thresholds != nil && def.Thresholds.Warning != nil {
			thresholdValue = *def.Thresholds.Warning
		}
		body = fmt.Sprintf("valor: %.2f | umbral %s: %.1f | host: %s", value, toLevel, thresholdValue, hostname)
	}

	priority := levelToPriority(toLevel)
	id := uuid.New().String()

	return db.EnqueueMessage(ctx, database, id, "general", priority, &title, body, []string{"metrics", def.Name}, nil)
}
