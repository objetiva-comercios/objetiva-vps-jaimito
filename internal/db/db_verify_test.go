package db

import (
	"context"
	"os"
	"testing"
)

func TestOpenAndApplySchema(t *testing.T) {
	tmpfile, err := os.CreateTemp("", "jaimito-verify-*.db")
	if err != nil {
		t.Fatal(err)
	}
	tmpfile.Close()
	os.Remove(tmpfile.Name())
	t.Cleanup(func() {
		os.Remove(tmpfile.Name())
		os.Remove(tmpfile.Name() + "-wal")
		os.Remove(tmpfile.Name() + "-shm")
	})

	db, err := Open(tmpfile.Name())
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	if err := ApplySchema(db); err != nil {
		t.Fatalf("ApplySchema failed: %v", err)
	}

	var journalMode string
	if err := db.QueryRow("PRAGMA journal_mode").Scan(&journalMode); err != nil {
		t.Fatalf("PRAGMA journal_mode failed: %v", err)
	}
	if journalMode != "wal" {
		t.Errorf("Expected WAL mode, got: %s", journalMode)
	}
	t.Logf("WAL mode: OK (journal_mode = %s)", journalMode)

	tables := []string{"messages", "dispatch_log", "api_keys"}
	for _, table := range tables {
		var name string
		err := db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&name)
		if err != nil {
			t.Errorf("Table %s not found: %v", table, err)
		} else {
			t.Logf("Table: %s OK", name)
		}
	}

	ctx := context.Background()
	n, err := ReclaimStuck(ctx, db)
	if err != nil {
		t.Fatalf("ReclaimStuck failed: %v", err)
	}
	t.Logf("ReclaimStuck: %d messages reclaimed (empty DB, expected 0)", n)
	if n != 0 {
		t.Errorf("Expected 0 reclaimed messages on empty DB, got %d", n)
	}
}
