package db

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"log/slog"

	"github.com/chiguire/jaimito/internal/config"
	"github.com/google/uuid"
)

// HashToken computes the SHA-256 hex digest of token.
// This is the single source of truth for token hashing shared between
// seed bootstrap (SeedKeys) and the auth middleware (LookupKeyHash).
func HashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}

// LookupKeyHash returns the key ID for the given key_hash, or "" if not found or revoked.
// sql.ErrNoRows (no match) is treated as not-found, not an error.
func LookupKeyHash(ctx context.Context, db *sql.DB, keyHash string) (string, error) {
	var id string
	err := db.QueryRowContext(ctx,
		`SELECT id FROM api_keys WHERE key_hash = ? AND revoked = 0`,
		keyHash,
	).Scan(&id)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("lookup key hash: %w", err)
	}
	return id, nil
}

// SeedKeys inserts the provided API keys into the database if they do not already exist.
// INSERT OR IGNORE makes this idempotent — safe to run on every startup.
// Each key must have a non-empty name and a key starting with "sk-".
func SeedKeys(ctx context.Context, db *sql.DB, keys []config.SeedAPIKey) error {
	for _, k := range keys {
		if k.Name == "" {
			return fmt.Errorf("seed key: name must not be empty")
		}
		if len(k.Key) < 3 || k.Key[:3] != "sk-" {
			return fmt.Errorf("seed key %q: key must start with \"sk-\"", k.Name)
		}

		hash := HashToken(k.Key)
		id := uuid.New().String()

		_, err := db.ExecContext(ctx,
			`INSERT OR IGNORE INTO api_keys (id, key_hash, name) VALUES (?, ?, ?)`,
			id, hash, k.Name,
		)
		if err != nil {
			return fmt.Errorf("seed key %q: %w", k.Name, err)
		}

		slog.Info("seeded api key", "name", k.Name)
	}
	return nil
}

// UpdateLastUsed records the current timestamp for the given key ID.
// Intended to be called fire-and-forget from the auth middleware (non-critical path).
func UpdateLastUsed(ctx context.Context, db *sql.DB, keyID string) error {
	_, err := db.ExecContext(ctx,
		`UPDATE api_keys SET last_used_at = datetime('now') WHERE id = ?`,
		keyID,
	)
	if err != nil {
		return fmt.Errorf("update last used: %w", err)
	}
	return nil
}
