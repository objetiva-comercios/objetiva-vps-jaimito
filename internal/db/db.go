// Package db provides SQLite database initialization and management for jaimito.
package db

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"os"
	"path/filepath"

	"github.com/adlio/schema"
	_ "modernc.org/sqlite"
)

//go:embed schema/*.sql
var schemaFS embed.FS

// Open opens (or creates) a SQLite database at path with WAL mode enabled.
// It creates the parent directory if it does not exist.
// The returned *sql.DB is configured with a single-writer connection pool
// to prevent SQLITE_BUSY errors under concurrent access.
func Open(path string) (*sql.DB, error) {
	// Auto-create parent directory — zero manual setup required.
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, fmt.Errorf("create db directory: %w", err)
	}

	// IMPORTANT: modernc.org/sqlite uses _pragma=NAME(VALUE) format,
	// NOT the _journal_mode=WAL format used by mattn/go-sqlite3.
	dsn := "file:" + path +
		"?_pragma=journal_mode(WAL)" +
		"&_pragma=busy_timeout(5000)" +
		"&_pragma=foreign_keys(1)" +
		"&_pragma=synchronous(NORMAL)"

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	// CRITICAL: SQLite supports only one concurrent writer.
	// SetMaxOpenConns(1) serializes all operations and prevents SQLITE_BUSY.
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0) // connections live for app lifetime

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}

	return db, nil
}

// ApplySchema applies embedded SQL migrations to the database.
// It is idempotent — safe to call on every startup.
// adlio/schema tracks which migrations have been applied.
func ApplySchema(db *sql.DB) error {
	migrations, err := schema.FSMigrations(schemaFS, "schema/*.sql")
	if err != nil {
		return fmt.Errorf("load schema migrations: %w", err)
	}

	migrator := schema.NewMigrator(schema.WithDialect(schema.SQLite))
	if err := migrator.Apply(db, migrations); err != nil {
		return fmt.Errorf("apply schema: %w", err)
	}

	return nil
}

// ReclaimStuck resets messages stuck in "dispatching" status back to "queued".
// This handles crash recovery — the dispatcher may have crashed mid-flight.
// Returns the number of messages reclaimed.
func ReclaimStuck(ctx context.Context, db *sql.DB) (int64, error) {
	result, err := db.ExecContext(ctx,
		`UPDATE messages SET status = 'queued', updated_at = datetime('now')
		 WHERE status = 'dispatching'`,
	)
	if err != nil {
		return 0, fmt.Errorf("reclaim stuck messages: %w", err)
	}

	n, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("rows affected: %w", err)
	}

	return n, nil
}
