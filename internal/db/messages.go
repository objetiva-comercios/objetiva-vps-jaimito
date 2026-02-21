package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
)

// Message represents a notification message record from the database.
type Message struct {
	ID        string
	Channel   string
	Priority  string
	Title     *string // nullable
	Body      string
	Tags      []string
	Metadata  map[string]any
	Status    string
	CreatedAt string
	UpdatedAt string
}

// EnqueueMessage inserts a new message with status 'queued'.
// tags and metadata are JSON-marshalled before storage.
// title is a pointer so nil becomes SQL NULL.
func EnqueueMessage(ctx context.Context, db *sql.DB, id string, channel string, priority string, title *string, body string, tags []string, metadata map[string]any) error {
	var tagsVal sql.NullString
	if tags != nil {
		b, err := json.Marshal(tags)
		if err != nil {
			return fmt.Errorf("marshal tags: %w", err)
		}
		tagsVal = sql.NullString{String: string(b), Valid: true}
	}

	var metaVal sql.NullString
	if metadata != nil {
		b, err := json.Marshal(metadata)
		if err != nil {
			return fmt.Errorf("marshal metadata: %w", err)
		}
		metaVal = sql.NullString{String: string(b), Valid: true}
	}

	_, err := db.ExecContext(ctx,
		`INSERT INTO messages (id, channel, priority, title, body, tags, metadata, status)
		 VALUES (?, ?, ?, ?, ?, ?, ?, 'queued')`,
		id, channel, priority, title, body, tagsVal, metaVal,
	)
	if err != nil {
		return fmt.Errorf("enqueue message: %w", err)
	}
	return nil
}

// GetNextQueued returns the oldest queued message, or nil if none exist.
// tags and metadata JSON strings are decoded back to their Go types.
func GetNextQueued(ctx context.Context, db *sql.DB) (*Message, error) {
	row := db.QueryRowContext(ctx,
		`SELECT id, channel, priority, title, body, tags, metadata, status, created_at, updated_at
		 FROM messages WHERE status = 'queued' ORDER BY created_at ASC LIMIT 1`,
	)

	var m Message
	var title sql.NullString
	var tagsStr sql.NullString
	var metaStr sql.NullString

	err := row.Scan(
		&m.ID, &m.Channel, &m.Priority, &title,
		&m.Body, &tagsStr, &metaStr, &m.Status,
		&m.CreatedAt, &m.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get next queued: %w", err)
	}

	if title.Valid {
		m.Title = &title.String
	}

	if tagsStr.Valid {
		if err := json.Unmarshal([]byte(tagsStr.String), &m.Tags); err != nil {
			return nil, fmt.Errorf("unmarshal tags: %w", err)
		}
	}

	if metaStr.Valid {
		if err := json.Unmarshal([]byte(metaStr.String), &m.Metadata); err != nil {
			return nil, fmt.Errorf("unmarshal metadata: %w", err)
		}
	}

	return &m, nil
}

// MarkDispatching transitions a message to the 'dispatching' status.
func MarkDispatching(ctx context.Context, db *sql.DB, id string) error {
	_, err := db.ExecContext(ctx,
		`UPDATE messages SET status = 'dispatching', updated_at = datetime('now') WHERE id = ?`,
		id,
	)
	if err != nil {
		return fmt.Errorf("mark dispatching: %w", err)
	}
	return nil
}

// MarkDelivered transitions a message to the 'delivered' status.
func MarkDelivered(ctx context.Context, db *sql.DB, id string) error {
	_, err := db.ExecContext(ctx,
		`UPDATE messages SET status = 'delivered', updated_at = datetime('now') WHERE id = ?`,
		id,
	)
	if err != nil {
		return fmt.Errorf("mark delivered: %w", err)
	}
	return nil
}

// MarkFailed transitions a message to the 'failed' status.
func MarkFailed(ctx context.Context, db *sql.DB, id string) error {
	_, err := db.ExecContext(ctx,
		`UPDATE messages SET status = 'failed', updated_at = datetime('now') WHERE id = ?`,
		id,
	)
	if err != nil {
		return fmt.Errorf("mark failed: %w", err)
	}
	return nil
}

// RecordAttempt inserts a dispatch_log entry for a delivery attempt.
// If sendErr is nil, status is "success"; otherwise "error" with the error message.
func RecordAttempt(ctx context.Context, db *sql.DB, messageID string, attempt int, sendErr error) error {
	status := "success"
	var errText sql.NullString
	if sendErr != nil {
		status = "error"
		errText = sql.NullString{String: sendErr.Error(), Valid: true}
	}

	_, err := db.ExecContext(ctx,
		`INSERT INTO dispatch_log (message_id, attempt, status, error) VALUES (?, ?, ?, ?)`,
		messageID, attempt, status, errText,
	)
	if err != nil {
		return fmt.Errorf("record attempt: %w", err)
	}
	return nil
}
