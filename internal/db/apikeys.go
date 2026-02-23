package db

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"log/slog"

	"github.com/chiguire/jaimito/internal/config"
	"github.com/google/uuid"
)

// ApiKey represents an API key record for listing purposes.
type ApiKey struct {
	ID         string
	Name       string
	CreatedAt  string
	LastUsedAt *string // nullable
}

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

// CreateKey generates a new API key with the given name, stores its hash in the database,
// and returns the raw key. The raw key is shown exactly once — only the hash is persisted.
func CreateKey(ctx context.Context, database *sql.DB, name string) (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate key: %w", err)
	}
	key := "sk-" + hex.EncodeToString(raw)
	hash := HashToken(key)
	id := uuid.New().String()

	_, err := database.ExecContext(ctx,
		`INSERT INTO api_keys (id, key_hash, name) VALUES (?, ?, ?)`,
		id, hash, name,
	)
	if err != nil {
		return "", fmt.Errorf("create key: %w", err)
	}
	return key, nil
}

// ListKeys returns all non-revoked API keys ordered by creation date.
func ListKeys(ctx context.Context, database *sql.DB) ([]ApiKey, error) {
	rows, err := database.QueryContext(ctx,
		`SELECT id, name, created_at, last_used_at FROM api_keys WHERE revoked = 0 ORDER BY created_at ASC`,
	)
	if err != nil {
		return nil, fmt.Errorf("list keys: %w", err)
	}
	defer rows.Close()

	var keys []ApiKey
	for rows.Next() {
		var k ApiKey
		var lastUsed sql.NullString
		if err := rows.Scan(&k.ID, &k.Name, &k.CreatedAt, &lastUsed); err != nil {
			return nil, fmt.Errorf("scan key: %w", err)
		}
		if lastUsed.Valid {
			k.LastUsedAt = &lastUsed.String
		}
		keys = append(keys, k)
	}
	return keys, rows.Err()
}

// RevokeKey marks an API key as revoked. Returns an error if the key ID was not found
// or was already revoked.
func RevokeKey(ctx context.Context, database *sql.DB, id string) error {
	result, err := database.ExecContext(ctx,
		`UPDATE api_keys SET revoked = 1 WHERE id = ? AND revoked = 0`,
		id,
	)
	if err != nil {
		return fmt.Errorf("revoke key: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("revoke key rows affected: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("key not found or already revoked: %s", id)
	}
	return nil
}
