---
phase: 02-core-pipeline
plan: 01
subsystem: database
tags: [sqlite, config, api-keys, sha256, messages, migration]

# Dependency graph
requires:
  - phase: 01-foundation
    provides: "db.go with Open/ApplySchema/ReclaimStuck, schema 001_initial.sql, config.go with Load/Validate"

provides:
  - "ServerConfig and SeedAPIKey types added to Config"
  - "ChannelExists and ChatIDForChannel helper methods on *Config"
  - "Schema migration 002 making title nullable via shadow-table pattern"
  - "Message struct with full EnqueueMessage/GetNextQueued/MarkDispatching/MarkDelivered/MarkFailed/RecordAttempt"
  - "HashToken (SHA-256), LookupKeyHash, SeedKeys, UpdateLastUsed for API key management"

affects:
  - 02-core-pipeline/02-02 (HTTP API uses EnqueueMessage, LookupKeyHash, HashToken)
  - 02-core-pipeline/02-03 (dispatcher uses GetNextQueued, MarkDispatching, MarkDelivered, MarkFailed, RecordAttempt)

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "All DB functions take *sql.DB parameter (not methods on a struct) — Phase 1 pattern continued"
    - "Nullable pointer types (*string) for optional columns — nil becomes SQL NULL via database/sql"
    - "JSON marshal/unmarshal for tags ([]string) and metadata (map[string]any) stored as TEXT"
    - "INSERT OR IGNORE for idempotent seed operations — safe to run on every startup"
    - "HashToken as single source of truth — used by both SeedKeys and auth middleware"

key-files:
  created:
    - internal/db/schema/002_nullable_title.sql
    - internal/db/messages.go
    - internal/db/apikeys.go
  modified:
    - internal/config/config.go
    - internal/config/config_test.go

key-decisions:
  - "server.listen defaults to 127.0.0.1:8080 (localhost-only) for VPS security — not 0.0.0.0"
  - "HashToken exported as shared helper between SeedKeys and auth middleware to avoid divergence"
  - "SeedKeys uses INSERT OR IGNORE — idempotent and safe on every startup without explicit exists check"
  - "SeedKeys logs key name at Info level on seed — never logs raw key value"

patterns-established:
  - "Shadow-table migration pattern for SQLite ALTER COLUMN workaround"
  - "sql.NullString for all nullable TEXT columns in scan/exec"
  - "JSON encoding for structured types ([]string, map[string]any) in TEXT columns"

requirements-completed: [API-01, API-02, TELE-01]

# Metrics
duration: 3min
completed: 2026-02-21
---

# Phase 2 Plan 01: Data Layer Extension Summary

**Config extended with ServerConfig/SeedAPIKey/channel-helpers; schema 002 makes title nullable via shadow-table; full message queue CRUD and SHA-256 API key operations added to db package**

## Performance

- **Duration:** ~3 min
- **Started:** 2026-02-21T19:00:09Z
- **Completed:** 2026-02-21T19:02:26Z
- **Tasks:** 3
- **Files modified:** 5

## Accomplishments
- Config package extended with ServerConfig (default listen 127.0.0.1:8080), SeedAPIKey type, seed_api_keys validation, and ChannelExists/ChatIDForChannel helpers
- Schema migration 002 makes title nullable using SQLite shadow-table approach (rename, recreate, copy, drop)
- Full message lifecycle DB layer: EnqueueMessage, GetNextQueued, MarkDispatching, MarkDelivered, MarkFailed, RecordAttempt
- API key DB layer: HashToken (SHA-256 hex), LookupKeyHash (revocation-aware), SeedKeys (idempotent INSERT OR IGNORE), UpdateLastUsed

## Task Commits

Each task was committed atomically:

1. **Task 1: Extend config with ServerConfig, SeedAPIKey, and channel helpers** - `3ff3f35` (feat)
2. **Task 2: Add schema migration 002 and message DB operations** - `285f017` (feat)
3. **Task 3: Add API key DB operations with shared hash helper** - `2415e66` (feat)

**Plan metadata:** (docs commit follows)

## Files Created/Modified
- `internal/config/config.go` - Added ServerConfig, SeedAPIKey, Server/SeedAPIKeys fields, ChannelExists, ChatIDForChannel, seed key validation
- `internal/config/config_test.go` - Added 7 new tests covering all new config functionality
- `internal/db/schema/002_nullable_title.sql` - Shadow-table migration making title TEXT nullable
- `internal/db/messages.go` - Message struct and 6 DB functions for full message queue lifecycle
- `internal/db/apikeys.go` - HashToken, LookupKeyHash, SeedKeys, UpdateLastUsed for API key management

## Decisions Made
- `server.listen` defaults to `127.0.0.1:8080` — localhost-only binding for VPS security; explicit override required to expose publicly
- `HashToken` exported at package level — both SeedKeys and the auth middleware use the same hash function to prevent subtle divergence bugs
- `SeedKeys` uses `INSERT OR IGNORE` — no separate existence check needed, always safe to call on startup
- Logging in `SeedKeys` uses `slog.Info` with key name only — raw key value never logged (security hygiene)

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- All DB operations needed by the HTTP API (EnqueueMessage, LookupKeyHash, SeedKeys) are complete
- All DB operations needed by the dispatcher (GetNextQueued, MarkDispatching, MarkDelivered, MarkFailed, RecordAttempt) are complete
- Config provides ChannelExists and ChatIDForChannel for request routing in the HTTP layer
- Schema migration 002 ready — adlio/schema will apply it automatically at next startup

---
*Phase: 02-core-pipeline*
*Completed: 2026-02-21*
