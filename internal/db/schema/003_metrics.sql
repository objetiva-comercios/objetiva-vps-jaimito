-- Migration 003: Add metrics tables for system monitoring.
-- Two tables: metrics (current state, upserted on startup) and
-- metric_readings (historical readings for dashboard/API).

CREATE TABLE IF NOT EXISTS metrics (
    name        TEXT PRIMARY KEY,
    category    TEXT NOT NULL DEFAULT 'custom',
    type        TEXT NOT NULL DEFAULT 'gauge',
    last_value  REAL,
    last_status TEXT NOT NULL DEFAULT 'ok',
    updated_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);

CREATE TABLE IF NOT EXISTS metric_readings (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    metric_name TEXT NOT NULL REFERENCES metrics(name),
    value       REAL NOT NULL,
    recorded_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);

CREATE INDEX IF NOT EXISTS idx_metric_readings_name_time
    ON metric_readings(metric_name, recorded_at);
