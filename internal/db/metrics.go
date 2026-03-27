package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// MetricRow representa un registro de la tabla metrics.
type MetricRow struct {
	Name       string
	Category   string
	Type       string
	LastValue  *float64
	LastStatus string
	UpdatedAt  string
}

// ReadingRow representa un registro de metric_readings.
type ReadingRow struct {
	ID         int64
	MetricName string
	Value      float64
	RecordedAt string
}

// UpsertMetric inserta o actualiza una metrica en la tabla metrics.
// Es idempotente: llamar multiples veces con el mismo nombre actualiza category y type.
// La tabla metrics se puebla via upsert al arrancar jaimito.
func UpsertMetric(ctx context.Context, db *sql.DB, name, category, metricType string) error {
	_, err := db.ExecContext(ctx,
		`INSERT INTO metrics (name, category, type)
		 VALUES (?, ?, ?)
		 ON CONFLICT(name) DO UPDATE SET
		     category = excluded.category,
		     type = excluded.type,
		     updated_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now')`,
		name, category, metricType,
	)
	if err != nil {
		return fmt.Errorf("upsert metric %q: %w", name, err)
	}
	return nil
}

// InsertReading graba un valor float64 asociado a una metrica existente.
// Retorna error si metric_name no existe en la tabla metrics (FK constraint).
func InsertReading(ctx context.Context, db *sql.DB, metricName string, value float64) error {
	_, err := db.ExecContext(ctx,
		`INSERT INTO metric_readings (metric_name, value) VALUES (?, ?)`,
		metricName, value,
	)
	if err != nil {
		return fmt.Errorf("insert reading %q: %w", metricName, err)
	}
	return nil
}

// QueryReadings retorna readings de una metrica posteriores al parametro since,
// ordenados por recorded_at ASC. Retorna slice vacio (no nil) si no hay resultados.
func QueryReadings(ctx context.Context, db *sql.DB, metricName string, since time.Time) ([]ReadingRow, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT id, metric_name, value, recorded_at
		 FROM metric_readings
		 WHERE metric_name = ? AND recorded_at >= ?
		 ORDER BY recorded_at ASC`,
		metricName, since.UTC().Format(time.RFC3339),
	)
	if err != nil {
		return nil, fmt.Errorf("query readings %q: %w", metricName, err)
	}
	defer rows.Close()

	readings := []ReadingRow{}
	for rows.Next() {
		var r ReadingRow
		if err := rows.Scan(&r.ID, &r.MetricName, &r.Value, &r.RecordedAt); err != nil {
			return nil, fmt.Errorf("scan reading: %w", err)
		}
		readings = append(readings, r)
	}
	return readings, rows.Err()
}

// ListMetrics retorna todas las metricas definidas con sus campos, ordenadas por nombre ASC.
// Retorna slice vacio (no nil) si no hay metricas.
func ListMetrics(ctx context.Context, db *sql.DB) ([]MetricRow, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT name, category, type, last_value, last_status, updated_at
		 FROM metrics ORDER BY name ASC`,
	)
	if err != nil {
		return nil, fmt.Errorf("list metrics: %w", err)
	}
	defer rows.Close()

	metrics := []MetricRow{}
	for rows.Next() {
		var m MetricRow
		var lastValue sql.NullFloat64
		if err := rows.Scan(&m.Name, &m.Category, &m.Type, &lastValue, &m.LastStatus, &m.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan metric: %w", err)
		}
		if lastValue.Valid {
			m.LastValue = &lastValue.Float64
		}
		metrics = append(metrics, m)
	}
	return metrics, rows.Err()
}

// PurgeOldReadings elimina readings anteriores a la duracion especificada.
// Retorna el numero de readings eliminados.
func PurgeOldReadings(ctx context.Context, db *sql.DB, olderThan time.Duration) (int64, error) {
	cutoff := time.Now().UTC().Add(-olderThan)
	result, err := db.ExecContext(ctx,
		`DELETE FROM metric_readings WHERE recorded_at < ?`,
		cutoff.Format(time.RFC3339),
	)
	if err != nil {
		return 0, fmt.Errorf("purge old readings: %w", err)
	}
	return result.RowsAffected()
}

// UpdateMetricStatus actualiza last_value y last_status de una metrica existente.
func UpdateMetricStatus(ctx context.Context, db *sql.DB, name string, value float64, status string) error {
	_, err := db.ExecContext(ctx,
		`UPDATE metrics SET last_value = ?, last_status = ?,
		     updated_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now')
		 WHERE name = ?`,
		value, status, name,
	)
	if err != nil {
		return fmt.Errorf("update metric status %q: %w", name, err)
	}
	return nil
}

// MetricExists retorna true si la metrica existe en la tabla metrics.
func MetricExists(ctx context.Context, db *sql.DB, name string) (bool, error) {
	var count int
	err := db.QueryRowContext(ctx,
		`SELECT COUNT(1) FROM metrics WHERE name = ?`, name,
	).Scan(&count)
	return count > 0, err
}
