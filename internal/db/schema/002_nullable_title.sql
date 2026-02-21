-- Migration 002: Make title column nullable.
-- SQLite does not support ALTER COLUMN, so we use the shadow-table approach.

ALTER TABLE messages RENAME TO messages_old;

CREATE TABLE messages (
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

INSERT INTO messages SELECT * FROM messages_old;

DROP TABLE messages_old;

CREATE INDEX IF NOT EXISTS idx_messages_status ON messages(status);
CREATE INDEX IF NOT EXISTS idx_messages_created_at ON messages(created_at);
