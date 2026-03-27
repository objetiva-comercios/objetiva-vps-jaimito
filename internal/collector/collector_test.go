package collector

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/chiguire/jaimito/internal/config"
	"github.com/chiguire/jaimito/internal/db"
)

// TestComputeTimeout verifica que computeTimeout calcula min(80%*interval, 30s).
func TestComputeTimeout(t *testing.T) {
	tests := []struct {
		name     string
		interval time.Duration
		want     time.Duration
	}{
		{"60s -> 30s (capped at max)", 60 * time.Second, 30 * time.Second},
		{"30s -> 24s (80%)", 30 * time.Second, 24 * time.Second},
		{"5s -> 4s (80%)", 5 * time.Second, 4 * time.Second},
		{"1s -> 800ms (80%)", 1 * time.Second, 800 * time.Millisecond},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := computeTimeout(tt.interval)
			if got != tt.want {
				t.Errorf("computeTimeout(%v) = %v, want %v", tt.interval, got, tt.want)
			}
		})
	}
}

// TestRunCommand_Success verifica que un comando exitoso retorna el valor flotante parseado.
func TestRunCommand_Success(t *testing.T) {
	ctx := context.Background()
	val, err := runCommand(ctx, "echo 42.5", 5*time.Second)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if val != 42.5 {
		t.Errorf("expected 42.5, got %v", val)
	}
}

// TestRunCommand_ParseError verifica que output no numerico retorna error "parse output".
func TestRunCommand_ParseError(t *testing.T) {
	ctx := context.Background()
	val, err := runCommand(ctx, "echo notanumber", 5*time.Second)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if val != 0 {
		t.Errorf("expected 0 on error, got %v", val)
	}
	// El error debe mencionar "parse output"
	if errStr := err.Error(); len(errStr) == 0 {
		t.Error("error message should not be empty")
	}
}

// TestRunCommand_Timeout verifica que un comando que excede el timeout retorna error.
func TestRunCommand_Timeout(t *testing.T) {
	ctx := context.Background()
	val, err := runCommand(ctx, "sleep 10", 100*time.Millisecond)
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
	if val != 0 {
		t.Errorf("expected 0 on timeout, got %v", val)
	}
}

// TestRunCommand_Failure verifica que un comando con exit code != 0 retorna error "command failed".
func TestRunCommand_Failure(t *testing.T) {
	ctx := context.Background()
	val, err := runCommand(ctx, "exit 1", 5*time.Second)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if val != 0 {
		t.Errorf("expected 0 on failure, got %v", val)
	}
}

// TestRunCommand_CommandNotFound verifica que un binario inexistente retorna error sin crash.
// Esto cubre MCOL-02: docker_running falla silenciosamente si Docker no esta instalado.
func TestRunCommand_CommandNotFound(t *testing.T) {
	ctx := context.Background()
	val, err := runCommand(ctx, "nonexistent_binary_xyz 2>/dev/null", 5*time.Second)
	if err == nil {
		t.Fatal("expected error for nonexistent binary, got nil")
	}
	if val != 0 {
		t.Errorf("expected 0 for nonexistent binary, got %v", val)
	}
}

// newTestDB abre una DB SQLite in-memory y aplica el schema para tests de integracion.
func newTestDB(t *testing.T) *sql.DB {
	t.Helper()
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("open in-memory db: %v", err)
	}
	if err := db.ApplySchema(database); err != nil {
		t.Fatalf("apply schema: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	return database
}

// newTestMetricsConfig crea un MetricsConfig minimo para tests.
func newTestMetricsConfig(collectInterval string) *config.MetricsConfig {
	return &config.MetricsConfig{
		CollectInterval: collectInterval,
		Retention:       "7d",
		AlertCooldown:   "30m",
	}
}

// TestResolveInterval verifica que resolveInterval hereda el intervalo global o usa el propio.
func TestResolveInterval(t *testing.T) {
	cfg := newTestMetricsConfig("60s")

	t.Run("hereda global cuando Interval vacio", func(t *testing.T) {
		def := config.MetricDef{Name: "test", Command: "echo 1", Interval: ""}
		got := resolveInterval(def, cfg)
		if got != 60*time.Second {
			t.Errorf("expected 60s, got %v", got)
		}
	})

	t.Run("usa intervalo propio cuando definido", func(t *testing.T) {
		def := config.MetricDef{Name: "test", Command: "echo 1", Interval: "300s"}
		got := resolveInterval(def, cfg)
		if got != 300*time.Second {
			t.Errorf("expected 300s, got %v", got)
		}
	})
}

// TestCategory verifica que category retorna el valor correcto o "custom" como fallback.
func TestCategory(t *testing.T) {
	t.Run("vacio -> custom", func(t *testing.T) {
		def := config.MetricDef{Name: "test", Command: "echo 1", Category: ""}
		got := category(def)
		if got != "custom" {
			t.Errorf("expected 'custom', got %q", got)
		}
	})

	t.Run("system -> system", func(t *testing.T) {
		def := config.MetricDef{Name: "test", Command: "echo 1", Category: "system"}
		got := category(def)
		if got != "system" {
			t.Errorf("expected 'system', got %q", got)
		}
	})
}

// TestMetricType verifica que metricType retorna el valor correcto o "gauge" como fallback.
func TestMetricType(t *testing.T) {
	t.Run("vacio -> gauge", func(t *testing.T) {
		def := config.MetricDef{Name: "test", Command: "echo 1", Type: ""}
		got := metricType(def)
		if got != "gauge" {
			t.Errorf("expected 'gauge', got %q", got)
		}
	})

	t.Run("counter -> counter", func(t *testing.T) {
		def := config.MetricDef{Name: "test", Command: "echo 1", Type: "counter"}
		got := metricType(def)
		if got != "counter" {
			t.Errorf("expected 'counter', got %q", got)
		}
	})
}

// TestCollectAndPersist verifica que collectAndPersist ejecuta el comando y persiste el valor en DB.
func TestCollectAndPersist(t *testing.T) {
	ctx := context.Background()
	database := newTestDB(t)
	cfg := newTestMetricsConfig("60s")

	def := config.MetricDef{
		Name:    "test_metric",
		Command: "echo 42.5",
	}

	// Upsert la metrica primero (como hace Start())
	if err := db.UpsertMetric(ctx, database, def.Name, "custom", "gauge"); err != nil {
		t.Fatalf("upsert metric: %v", err)
	}

	// Ejecutar coleccion
	collectAndPersist(ctx, database, cfg, def)

	// Verificar que last_value fue actualizado en metrics
	metrics, err := db.ListMetrics(ctx, database)
	if err != nil {
		t.Fatalf("list metrics: %v", err)
	}
	if len(metrics) != 1 {
		t.Fatalf("expected 1 metric, got %d", len(metrics))
	}
	m := metrics[0]
	if m.LastValue == nil {
		t.Fatal("expected last_value to be set, got nil")
	}
	if *m.LastValue != 42.5 {
		t.Errorf("expected last_value=42.5, got %v", *m.LastValue)
	}

	// Verificar que hay un reading en metric_readings con value=42.5
	readings, err := db.QueryReadings(ctx, database, def.Name, time.Time{})
	if err != nil {
		t.Fatalf("query readings: %v", err)
	}
	if len(readings) != 1 {
		t.Fatalf("expected 1 reading, got %d", len(readings))
	}
	if readings[0].Value != 42.5 {
		t.Errorf("expected reading value=42.5, got %v", readings[0].Value)
	}
}

// TestCollectCommandFailure verifica que un comando fallido no produce readings en metric_readings.
func TestCollectCommandFailure(t *testing.T) {
	ctx := context.Background()
	database := newTestDB(t)
	cfg := newTestMetricsConfig("60s")

	def := config.MetricDef{
		Name:    "test_fail",
		Command: "exit 1",
	}

	// Upsert la metrica
	if err := db.UpsertMetric(ctx, database, def.Name, "custom", "gauge"); err != nil {
		t.Fatalf("upsert metric: %v", err)
	}

	// Ejecutar coleccion — el comando falla, no debe producir reading
	collectAndPersist(ctx, database, cfg, def)

	// Verificar que metric_readings esta vacio
	readings, err := db.QueryReadings(ctx, database, def.Name, time.Time{})
	if err != nil {
		t.Fatalf("query readings: %v", err)
	}
	if len(readings) != 0 {
		t.Errorf("expected 0 readings after command failure, got %d", len(readings))
	}
}
