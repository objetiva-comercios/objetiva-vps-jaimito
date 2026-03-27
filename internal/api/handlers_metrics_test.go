package api

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/chiguire/jaimito/internal/config"
	"github.com/chiguire/jaimito/internal/db"
	_ "modernc.org/sqlite"
)

// newMetricsTestDB crea una DB SQLite in-memory con schema aplicado para tests de metricas.
func newMetricsTestDB(t *testing.T) *sql.DB {
	t.Helper()
	database, err := sql.Open("sqlite", ":memory:?_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	if err := db.ApplySchema(database); err != nil {
		t.Fatal(err)
	}
	return database
}

// testMetricsConfig retorna un Config minimo valido con metricas para tests.
func testMetricsConfig() *config.Config {
	w := 80.0
	c := 95.0
	return &config.Config{
		Telegram: config.TelegramConfig{Token: "test-token"},
		Channels: []config.ChannelConfig{
			{Name: "general", ChatID: 123, Priority: "normal"},
		},
		Server: config.ServerConfig{Listen: "127.0.0.1:8080"},
		SeedAPIKeys: []config.SeedAPIKey{
			{Name: "test", Key: "sk-testkey123456789012345678901234"},
		},
		Metrics: &config.MetricsConfig{
			Retention:       "7d",
			AlertCooldown:   "30m",
			CollectInterval: "60s",
			Definitions: []config.MetricDef{
				{Name: "disk_root", Command: "echo 50", Category: "system", Type: "gauge",
					Thresholds: &config.Thresholds{Warning: &w, Critical: &c}},
				{Name: "custom_no_thresh", Command: "echo 1", Category: "custom", Type: "gauge"},
			},
		},
	}
}

// setupRouter crea el router completo con DB y config para tests.
func setupRouter(t *testing.T) (*sql.DB, *config.Config, http.Handler) {
	t.Helper()
	database := newMetricsTestDB(t)
	cfg := testMetricsConfig()

	ctx := context.Background()
	if err := db.SeedKeys(ctx, database, cfg.SeedAPIKeys); err != nil {
		t.Fatalf("SeedKeys: %v", err)
	}

	router := NewRouter(database, cfg)
	return database, cfg, router
}

// bearerHeader devuelve el header Authorization con el token de test.
func bearerHeader() string {
	return "Bearer sk-testkey123456789012345678901234"
}

// TestMetricsListHandler_OK verifica que GET /api/v1/metrics retorna 200 con metricas y thresholds.
func TestMetricsListHandler_OK(t *testing.T) {
	database, _, router := setupRouter(t)
	ctx := context.Background()

	// Insertar 2 metricas en DB
	if err := db.UpsertMetric(ctx, database, "disk_root", "system", "gauge"); err != nil {
		t.Fatal(err)
	}
	if err := db.UpsertMetric(ctx, database, "custom_no_thresh", "custom", "gauge"); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/metrics", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var metrics []MetricResponse
	if err := json.Unmarshal(w.Body.Bytes(), &metrics); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(metrics) != 2 {
		t.Fatalf("expected 2 metrics, got %d", len(metrics))
	}

	// Buscar disk_root — debe tener thresholds
	var diskRoot *MetricResponse
	var noThresh *MetricResponse
	for i := range metrics {
		if metrics[i].Name == "disk_root" {
			diskRoot = &metrics[i]
		}
		if metrics[i].Name == "custom_no_thresh" {
			noThresh = &metrics[i]
		}
	}

	if diskRoot == nil {
		t.Fatal("disk_root not found in response")
	}
	if diskRoot.Thresholds == nil {
		t.Fatal("disk_root should have thresholds")
	}
	if diskRoot.Thresholds.Warning == nil || *diskRoot.Thresholds.Warning != 80.0 {
		t.Errorf("disk_root warning threshold: expected 80.0")
	}
	if diskRoot.Thresholds.Critical == nil || *diskRoot.Thresholds.Critical != 95.0 {
		t.Errorf("disk_root critical threshold: expected 95.0")
	}

	if noThresh == nil {
		t.Fatal("custom_no_thresh not found in response")
	}
	if noThresh.Thresholds != nil {
		t.Fatal("custom_no_thresh should NOT have thresholds (omitempty)")
	}
}

// TestMetricsListHandler_ThresholdsOmitted verifica que thresholds es null en JSON cuando no aplica.
func TestMetricsListHandler_ThresholdsOmitted(t *testing.T) {
	database, _, router := setupRouter(t)
	ctx := context.Background()

	if err := db.UpsertMetric(ctx, database, "custom_no_thresh", "custom", "gauge"); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/metrics", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	// Verificar que el JSON raw no contiene "thresholds" para custom_no_thresh
	bodyStr := w.Body.String()
	var raw []map[string]interface{}
	if err := json.Unmarshal([]byte(bodyStr), &raw); err != nil {
		t.Fatal(err)
	}
	if len(raw) == 0 {
		t.Fatal("expected at least 1 metric")
	}
	if _, ok := raw[0]["thresholds"]; ok {
		t.Error("thresholds key should be omitted from JSON when nil (omitempty)")
	}
}

// TestMetricsListHandler_Empty verifica que sin metricas retorna 200 con array vacio.
func TestMetricsListHandler_Empty(t *testing.T) {
	_, _, router := setupRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/metrics", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	// Debe ser array JSON (puede ser [] o [])
	body := w.Body.String()
	if body[0] != '[' {
		t.Errorf("expected JSON array, got: %s", body)
	}
}

// TestReadingsHandler_OK verifica que GET /api/v1/metrics/{name}/readings retorna 200.
func TestReadingsHandler_OK(t *testing.T) {
	database, _, router := setupRouter(t)
	ctx := context.Background()

	if err := db.UpsertMetric(ctx, database, "disk_root", "system", "gauge"); err != nil {
		t.Fatal(err)
	}
	if err := db.InsertReading(ctx, database, "disk_root", 42.0); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/metrics/disk_root/readings", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp ReadingsListResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Metric != "disk_root" {
		t.Errorf("expected metric=disk_root, got %q", resp.Metric)
	}
	if len(resp.Readings) != 1 {
		t.Fatalf("expected 1 reading, got %d", len(resp.Readings))
	}
	if resp.Readings[0].Value != 42.0 {
		t.Errorf("expected value=42.0, got %f", resp.Readings[0].Value)
	}
}

// TestReadingsHandler_DefaultSince verifica que sin ?since se usa 24h por defecto.
func TestReadingsHandler_DefaultSince(t *testing.T) {
	database, _, router := setupRouter(t)
	ctx := context.Background()

	if err := db.UpsertMetric(ctx, database, "disk_root", "system", "gauge"); err != nil {
		t.Fatal(err)
	}

	// Insertar reading de hace 23h (dentro del default 24h)
	_, err := database.ExecContext(ctx,
		`INSERT INTO metric_readings (metric_name, value, recorded_at) VALUES (?, ?, ?)`,
		"disk_root", 50.0, time.Now().UTC().Add(-23*time.Hour).Format(time.RFC3339))
	if err != nil {
		t.Fatal(err)
	}

	// Insertar reading de hace 25h (fuera del default 24h)
	_, err = database.ExecContext(ctx,
		`INSERT INTO metric_readings (metric_name, value, recorded_at) VALUES (?, ?, ?)`,
		"disk_root", 60.0, time.Now().UTC().Add(-25*time.Hour).Format(time.RFC3339))
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/metrics/disk_root/readings", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp ReadingsListResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}

	if len(resp.Readings) != 1 {
		t.Fatalf("expected 1 reading (within 24h), got %d", len(resp.Readings))
	}
	if resp.Readings[0].Value != 50.0 {
		t.Errorf("expected value=50.0 (23h ago), got %f", resp.Readings[0].Value)
	}
}

// TestReadingsHandler_CustomSince verifica que ?since=2h filtra correctamente.
func TestReadingsHandler_CustomSince(t *testing.T) {
	database, _, router := setupRouter(t)
	ctx := context.Background()

	if err := db.UpsertMetric(ctx, database, "disk_root", "system", "gauge"); err != nil {
		t.Fatal(err)
	}

	// Reading de hace 1h (dentro de ?since=2h)
	_, err := database.ExecContext(ctx,
		`INSERT INTO metric_readings (metric_name, value, recorded_at) VALUES (?, ?, ?)`,
		"disk_root", 55.0, time.Now().UTC().Add(-1*time.Hour).Format(time.RFC3339))
	if err != nil {
		t.Fatal(err)
	}

	// Reading de hace 3h (fuera de ?since=2h)
	_, err = database.ExecContext(ctx,
		`INSERT INTO metric_readings (metric_name, value, recorded_at) VALUES (?, ?, ?)`,
		"disk_root", 65.0, time.Now().UTC().Add(-3*time.Hour).Format(time.RFC3339))
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/metrics/disk_root/readings?since=2h", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp ReadingsListResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}

	if len(resp.Readings) != 1 {
		t.Fatalf("expected 1 reading (within 2h), got %d", len(resp.Readings))
	}
	if resp.Readings[0].Value != 55.0 {
		t.Errorf("expected value=55.0 (1h ago), got %f", resp.Readings[0].Value)
	}
	if resp.Since != "2h" {
		t.Errorf("expected since=2h in response, got %q", resp.Since)
	}
}

// TestReadingsHandler_InvalidSince verifica que ?since=abc retorna 400.
func TestReadingsHandler_InvalidSince(t *testing.T) {
	database, _, router := setupRouter(t)
	ctx := context.Background()

	if err := db.UpsertMetric(ctx, database, "disk_root", "system", "gauge"); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/metrics/disk_root/readings?since=abc", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

// TestReadingsHandler_ExceedsMax verifica que ?since=30d retorna 400.
func TestReadingsHandler_ExceedsMax(t *testing.T) {
	database, _, router := setupRouter(t)
	ctx := context.Background()

	if err := db.UpsertMetric(ctx, database, "disk_root", "system", "gauge"); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/metrics/disk_root/readings?since=30d", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}

	var errResp ErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &errResp); err != nil {
		t.Fatal(err)
	}
	if errResp.Error != "max retention is 7d" {
		t.Errorf("expected error 'max retention is 7d', got %q", errResp.Error)
	}
}

// TestReadingsHandler_NotFound verifica que un nombre inexistente retorna 404.
func TestReadingsHandler_NotFound(t *testing.T) {
	_, _, router := setupRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/metrics/nonexistent/readings", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

// TestIngestHandler_Success verifica POST con Bearer valido y body correcto retorna 201.
func TestIngestHandler_Success(t *testing.T) {
	database, _, router := setupRouter(t)
	ctx := context.Background()

	if err := db.UpsertMetric(ctx, database, "disk_root", "system", "gauge"); err != nil {
		t.Fatal(err)
	}

	body := bytes.NewBufferString(`{"name":"disk_root","value":75.5}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/metrics", body)
	req.Header.Set("Authorization", bearerHeader())
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp IngestResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Name != "disk_root" {
		t.Errorf("expected name=disk_root, got %q", resp.Name)
	}
	if resp.Value != 75.5 {
		t.Errorf("expected value=75.5, got %f", resp.Value)
	}
	if resp.RecordedAt == "" {
		t.Error("expected recorded_at to be set")
	}
}

// TestIngestHandler_Unauthorized verifica POST sin Authorization retorna 401.
func TestIngestHandler_Unauthorized(t *testing.T) {
	_, _, router := setupRouter(t)

	body := bytes.NewBufferString(`{"name":"disk_root","value":50.0}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/metrics", body)
	req.Header.Set("Content-Type", "application/json")
	// Sin Authorization header

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", w.Code, w.Body.String())
	}
}

// TestIngestHandler_NotFound verifica POST con metrica inexistente retorna 404.
func TestIngestHandler_NotFound(t *testing.T) {
	_, _, router := setupRouter(t)

	body := bytes.NewBufferString(`{"name":"nonexistent","value":50.0}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/metrics", body)
	req.Header.Set("Authorization", bearerHeader())
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

// TestIngestHandler_InvalidBody verifica POST con body invalido retorna 400.
func TestIngestHandler_InvalidBody(t *testing.T) {
	_, _, router := setupRouter(t)

	body := bytes.NewBufferString(`not-json`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/metrics", body)
	req.Header.Set("Authorization", bearerHeader())
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

// TestIngestHandler_MissingValue verifica POST con {name: "x"} sin value retorna 400.
func TestIngestHandler_MissingValue(t *testing.T) {
	database, _, router := setupRouter(t)
	ctx := context.Background()

	if err := db.UpsertMetric(ctx, database, "disk_root", "system", "gauge"); err != nil {
		t.Fatal(err)
	}

	body := bytes.NewBufferString(`{"name":"disk_root"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/metrics", body)
	req.Header.Set("Authorization", bearerHeader())
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

// TestIngestHandler_MissingName verifica POST sin name retorna 400.
func TestIngestHandler_MissingName(t *testing.T) {
	_, _, router := setupRouter(t)

	val := 50.0
	bodyMap := map[string]interface{}{"value": val}
	bodyBytes, _ := json.Marshal(bodyMap)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/metrics", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Authorization", bearerHeader())
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}
