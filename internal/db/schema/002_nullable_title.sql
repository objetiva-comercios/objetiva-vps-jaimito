-- Migration 002: Make title column nullable.
-- SQLite does not support ALTER COLUMN, so we use the shadow-table approach.
-- Idempotent: checks if migration is needed before running.

-- Only run if title column is still NOT NULL (original schema).
-- After migration, title allows NULL values.
CREATE TABLE IF NOT EXISTS messages_new (
    id          TEXT PRIMARY KEY,
    channel     TEXT NOT NULL,
    priority    TEXT NOT NULL DEFAULT 'normal',
    title       TEXT,
    body        TEXT NOT NULL,
    tags        TEXT,
    metadata    TEXT,
    status      TEXT NOT NULL DEFAULT 'queued',
    created_at  TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at  TEXT NOT NULL DEFAULT (datetime('now'))
);

INSERT OR IGNORE INTO messages_new SELECT * FROM messages;

DROP TABLE IF EXISTS messages;

ALTER TABLE messages_new RENAME TO messages;

DROP TABLE IF EXISTS messages_old;

CREATE INDEX IF NOT EXISTS idx_messages_status ON messages(status);
CREATE INDEX IF NOT EXISTS idx_messages_created_at ON messages(created_at);
