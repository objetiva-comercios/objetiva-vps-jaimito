// Package cleanup provides periodic purging of old messages and metric readings from the database.
package cleanup

import (
	"context"
	"database/sql"
	"log/slog"
	"time"

	dbpkg "github.com/chiguire/jaimito/internal/db"
)

// Start launches the cleanup scheduler. It runs an initial purge immediately,
// then repeats every interval. Stops cleanly when ctx is cancelled.
// interval is typically 24*time.Hour for production use.
// metricsRetention, if > 0, enables periodic purging of metric_readings older than that duration.
func Start(ctx context.Context, db *sql.DB, interval time.Duration, metricsRetention time.Duration) {
	// Run immediately on startup so cleanup happens even if service rarely restarts.
	// Per RESEARCH.md recommendation: startup-then-interval pattern.
	if err := purgeOldMessages(ctx, db); err != nil {
		slog.Error("initial cleanup failed", "error", err)
	}
	if metricsRetention > 0 {
		if err := purgeMetrics(ctx, db, metricsRetention); err != nil {
			slog.Error("initial metrics cleanup failed", "error", err)
		}
	}

	ticker := time.NewTicker(interval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if err := purgeOldMessages(ctx, db); err != nil {
					slog.Error("cleanup failed", "error", err)
				}
				if metricsRetention > 0 {
					if err := purgeMetrics(ctx, db, metricsRetention); err != nil {
						slog.Error("metrics cleanup failed", "error", err)
					}
				}
			case <-ctx.Done():
				return
			}
		}
	}()
}

// purgeOldMessages deletes old delivered and failed messages (and their dispatch logs).
// Delivered messages older than 30 days and failed messages older than 90 days are removed.
// dispatch_log entries are deleted first to satisfy the foreign key constraint.
// The two deletes are wrapped in a transaction for atomicity.
func purgeOldMessages(ctx context.Context, db *sql.DB) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck // rollback on error path is intentional

	// Delete dispatch_log entries first (FK constraint: dispatch_log.message_id -> messages.id).
	_, err = tx.ExecContext(ctx, `
		DELETE FROM dispatch_log WHERE message_id IN (
			SELECT id FROM messages
			WHERE (status = 'delivered' AND created_at < datetime('now', '-30 days'))
			   OR (status = 'failed' AND created_at < datetime('now', '-90 days'))
		)
	`)
	if err != nil {
		return err
	}

	// Delete the messages themselves.
	result, err := tx.ExecContext(ctx, `
		DELETE FROM messages
		WHERE (status = 'delivered' AND created_at < datetime('now', '-30 days'))
		   OR (status = 'failed' AND created_at < datetime('now', '-90 days'))
	`)
	if err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	deleted, _ := result.RowsAffected()
	if deleted > 0 {
		slog.Info("purged old messages", "count", deleted)
	}

	return nil
}

// purgeMetrics deletes metric_readings older than retention duration.
func purgeMetrics(ctx context.Context, db *sql.DB, retention time.Duration) error {
	deleted, err := dbpkg.PurgeOldReadings(ctx, db, retention)
	if err != nil {
		return err
	}
	if deleted > 0 {
		slog.Info("purged old metric readings", "count", deleted)
	}
	return nil
}
