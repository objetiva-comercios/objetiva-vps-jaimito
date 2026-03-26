package db_test

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	"github.com/chiguire/jaimito/internal/db"
)

// newTestDB creates a temporary SQLite DB with the schema applied.
func newTestDB(t *testing.T) *sql.DB {
	t.Helper()
	f, err := os.CreateTemp("", "jaimito-metrics-test-*.db")
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	t.Cleanup(func() {
		os.Remove(f.Name())
		os.Remove(f.Name() + "-wal")
		os.Remove(f.Name() + "-shm")
	})
	database, err := db.Open(f.Name())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := db.ApplySchema(database); err != nil {
		t.Fatalf("ApplySchema: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	return database
}

func TestUpsertMetric(t *testing.T) {
	database := newTestDB(t)
	ctx := context.Background()

	// First upsert — should succeed
	if err := db.UpsertMetric(ctx, database, "disk_root", "system", "gauge"); err != nil {
		t.Fatalf("UpsertMetric first call: %v", err)
	}

	// Second upsert — idempotent, should not error
	if err := db.UpsertMetric(ctx, database, "disk_root", "system", "gauge"); err != nil {
		t.Fatalf("UpsertMetric second call (same): %v", err)
	}

	// Upsert with different category — should update
	if err := db.UpsertMetric(ctx, database, "disk_root", "infra", "gauge"); err != nil {
		t.Fatalf("UpsertMetric update category: %v", err)
	}

	// Verify category was updated
	metrics, err := db.ListMetrics(ctx, database)
	if err != nil {
		t.Fatalf("ListMetrics: %v", err)
	}
	if len(metrics) != 1 {
		t.Fatalf("Expected 1 metric, got %d", len(metrics))
	}
	if metrics[0].Category != "infra" {
		t.Errorf("Expected category 'infra', got %q", metrics[0].Category)
	}
}

func TestInsertReading(t *testing.T) {
	database := newTestDB(t)
	ctx := context.Background()

	if err := db.UpsertMetric(ctx, database, "disk_root", "system", "gauge"); err != nil {
		t.Fatalf("UpsertMetric: %v", err)
	}

	if err := db.InsertReading(ctx, database, "disk_root", 42.5); err != nil {
		t.Fatalf("InsertReading: %v", err)
	}
}

func TestInsertReading_NoMetric(t *testing.T) {
	database := newTestDB(t)
	ctx := context.Background()

	// InsertReading without UpsertMetric should fail (FK constraint)
	err := db.InsertReading(ctx, database, "nonexistent_metric", 42.5)
	if err == nil {
		t.Fatal("Expected error for FK constraint violation, got nil")
	}
}

func TestQueryReadings_FilterByTime(t *testing.T) {
	database := newTestDB(t)
	ctx := context.Background()

	if err := db.UpsertMetric(ctx, database, "disk_root", "system", "gauge"); err != nil {
		t.Fatalf("UpsertMetric: %v", err)
	}

	now := time.Now().UTC()
	ts3hAgo := now.Add(-3 * time.Hour).Format(time.RFC3339)
	ts1hAgo := now.Add(-1 * time.Hour).Format(time.RFC3339)
	tsNow := now.Format(time.RFC3339)

	// Insert readings with specific timestamps via SQL
	for _, ts := range []string{ts3hAgo, ts1hAgo, tsNow} {
		_, err := database.ExecContext(ctx,
			`INSERT INTO metric_readings (metric_name, value, recorded_at) VALUES (?, ?, ?)`,
			"disk_root", 50.0, ts,
		)
		if err != nil {
			t.Fatalf("insert reading at %s: %v", ts, err)
		}
	}

	since := now.Add(-2 * time.Hour)
	readings, err := db.QueryReadings(ctx, database, "disk_root", since)
	if err != nil {
		t.Fatalf("QueryReadings: %v", err)
	}
	if len(readings) != 2 {
		t.Errorf("Expected 2 readings since 2h ago, got %d", len(readings))
	}
}

func TestQueryReadings_Empty(t *testing.T) {
	database := newTestDB(t)
	ctx := context.Background()

	if err := db.UpsertMetric(ctx, database, "disk_root", "system", "gauge"); err != nil {
		t.Fatalf("UpsertMetric: %v", err)
	}

	readings, err := db.QueryReadings(ctx, database, "disk_root", time.Now().Add(-1*time.Hour))
	if err != nil {
		t.Fatalf("QueryReadings: %v", err)
	}
	if readings == nil {
		t.Error("QueryReadings on empty metric should return empty slice, not nil")
	}
	if len(readings) != 0 {
		t.Errorf("Expected 0 readings, got %d", len(readings))
	}
}

func TestQueryReadings_OrderByTime(t *testing.T) {
	database := newTestDB(t)
	ctx := context.Background()

	if err := db.UpsertMetric(ctx, database, "disk_root", "system", "gauge"); err != nil {
		t.Fatalf("UpsertMetric: %v", err)
	}

	now := time.Now().UTC()
	ts1 := now.Add(-2 * time.Hour).Format(time.RFC3339)
	ts2 := now.Add(-1 * time.Hour).Format(time.RFC3339)
	ts3 := now.Format(time.RFC3339)

	for _, ts := range []string{ts3, ts1, ts2} { // Insert out of order
		_, err := database.ExecContext(ctx,
			`INSERT INTO metric_readings (metric_name, value, recorded_at) VALUES (?, ?, ?)`,
			"disk_root", 10.0, ts,
		)
		if err != nil {
			t.Fatalf("insert reading at %s: %v", ts, err)
		}
	}

	since := now.Add(-3 * time.Hour)
	readings, err := db.QueryReadings(ctx, database, "disk_root", since)
	if err != nil {
		t.Fatalf("QueryReadings: %v", err)
	}
	if len(readings) != 3 {
		t.Fatalf("Expected 3 readings, got %d", len(readings))
	}
	// Should be sorted ASC
	if readings[0].RecordedAt > readings[1].RecordedAt || readings[1].RecordedAt > readings[2].RecordedAt {
		t.Errorf("Readings not in ASC order: %v", []string{readings[0].RecordedAt, readings[1].RecordedAt, readings[2].RecordedAt})
	}
}

func TestListMetrics(t *testing.T) {
	database := newTestDB(t)
	ctx := context.Background()

	if err := db.UpsertMetric(ctx, database, "disk_root", "system", "gauge"); err != nil {
		t.Fatalf("UpsertMetric disk_root: %v", err)
	}
	if err := db.UpsertMetric(ctx, database, "ram_used", "system", "gauge"); err != nil {
		t.Fatalf("UpsertMetric ram_used: %v", err)
	}

	metrics, err := db.ListMetrics(ctx, database)
	if err != nil {
		t.Fatalf("ListMetrics: %v", err)
	}
	if len(metrics) != 2 {
		t.Fatalf("Expected 2 metrics, got %d", len(metrics))
	}

	// Should be ordered by name ASC
	if metrics[0].Name != "disk_root" {
		t.Errorf("Expected first metric 'disk_root', got %q", metrics[0].Name)
	}
	if metrics[1].Name != "ram_used" {
		t.Errorf("Expected second metric 'ram_used', got %q", metrics[1].Name)
	}
	if metrics[0].Category != "system" || metrics[0].Type != "gauge" {
		t.Errorf("Wrong fields: category=%q type=%q", metrics[0].Category, metrics[0].Type)
	}
}

func TestListMetrics_Empty(t *testing.T) {
	database := newTestDB(t)
	ctx := context.Background()

	metrics, err := db.ListMetrics(ctx, database)
	if err != nil {
		t.Fatalf("ListMetrics on empty DB: %v", err)
	}
	if metrics == nil {
		t.Error("ListMetrics on empty DB should return empty slice, not nil")
	}
	if len(metrics) != 0 {
		t.Errorf("Expected 0 metrics, got %d", len(metrics))
	}
}

func TestPurgeOldReadings(t *testing.T) {
	database := newTestDB(t)
	ctx := context.Background()

	if err := db.UpsertMetric(ctx, database, "disk_root", "system", "gauge"); err != nil {
		t.Fatalf("UpsertMetric: %v", err)
	}

	now := time.Now().UTC()
	// Insert old readings (10 days ago)
	oldTs := now.Add(-10 * 24 * time.Hour).Format(time.RFC3339)
	for i := 0; i < 3; i++ {
		_, err := database.ExecContext(ctx,
			`INSERT INTO metric_readings (metric_name, value, recorded_at) VALUES (?, ?, ?)`,
			"disk_root", float64(i), oldTs,
		)
		if err != nil {
			t.Fatalf("insert old reading: %v", err)
		}
	}
	// Insert recent readings
	recentTs := now.Add(-1 * time.Hour).Format(time.RFC3339)
	for i := 0; i < 2; i++ {
		_, err := database.ExecContext(ctx,
			`INSERT INTO metric_readings (metric_name, value, recorded_at) VALUES (?, ?, ?)`,
			"disk_root", float64(i+10), recentTs,
		)
		if err != nil {
			t.Fatalf("insert recent reading: %v", err)
		}
	}

	count, err := db.PurgeOldReadings(ctx, database, 7*24*time.Hour)
	if err != nil {
		t.Fatalf("PurgeOldReadings: %v", err)
	}
	if count != 3 {
		t.Errorf("Expected 3 purged readings, got %d", count)
	}

	// Verify only recent readings remain
	readings, err := db.QueryReadings(ctx, database, "disk_root", now.Add(-2*time.Hour))
	if err != nil {
		t.Fatalf("QueryReadings after purge: %v", err)
	}
	if len(readings) != 2 {
		t.Errorf("Expected 2 recent readings remaining, got %d", len(readings))
	}
}

func TestPurgeOldReadings_Empty(t *testing.T) {
	database := newTestDB(t)
	ctx := context.Background()

	count, err := db.PurgeOldReadings(ctx, database, 7*24*time.Hour)
	if err != nil {
		t.Fatalf("PurgeOldReadings on empty DB: %v", err)
	}
	if count != 0 {
		t.Errorf("Expected 0 purged on empty DB, got %d", count)
	}
}

func TestUpdateMetricStatus(t *testing.T) {
	database := newTestDB(t)
	ctx := context.Background()

	if err := db.UpsertMetric(ctx, database, "disk_root", "system", "gauge"); err != nil {
		t.Fatalf("UpsertMetric: %v", err)
	}

	if err := db.UpdateMetricStatus(ctx, database, "disk_root", 85.3, "warning"); err != nil {
		t.Fatalf("UpdateMetricStatus: %v", err)
	}

	metrics, err := db.ListMetrics(ctx, database)
	if err != nil {
		t.Fatalf("ListMetrics: %v", err)
	}
	if len(metrics) != 1 {
		t.Fatalf("Expected 1 metric, got %d", len(metrics))
	}
	m := metrics[0]
	if m.LastValue == nil {
		t.Fatal("Expected LastValue to be set, got nil")
	}
	if *m.LastValue != 85.3 {
		t.Errorf("Expected LastValue=85.3, got %v", *m.LastValue)
	}
	if m.LastStatus != "warning" {
		t.Errorf("Expected LastStatus='warning', got %q", m.LastStatus)
	}
}
