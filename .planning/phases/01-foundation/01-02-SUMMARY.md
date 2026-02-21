---
phase: 01-foundation
plan: 02
subsystem: database
tags: [sqlite, modernc, wal, schema, migration, cleanup, adlio]

# Dependency graph
requires:
  - phase: 01-foundation/01-01
    provides: Go module initialized with all dependencies (modernc.org/sqlite, adlio/schema)

provides:
  - SQLite Open() with WAL mode, single-writer pool (SetMaxOpenConns(1))
  - ApplySchema() via embedded SQL migrations using adlio/schema + embed.FS
  - ReclaimStuck() crash recovery (dispatching -> queued reset)
  - Cleanup scheduler with initial purge and periodic 24h interval
  - Schema: messages, dispatch_log, api_keys tables with indexes

affects:
  - 01-foundation/01-03 (main.go wiring: db.Open, ApplySchema, ReclaimStuck, cleanup.Start)
  - 02-dispatcher (reads/writes messages table, reads dispatch_log)
  - 03-cli (reads api_keys table)

# Tech tracking
tech-stack:
  added:
    - modernc.org/sqlite v1.46.1 (CGO-free SQLite driver)
    - github.com/adlio/schema v1.3.9 (embedded schema migrations)
    - database/sql (stdlib connection pool abstraction)
    - embed (stdlib, schema/*.sql embedded into binary)
  patterns:
    - Single-writer SQLite pool (SetMaxOpenConns(1)) to prevent SQLITE_BUSY
    - modernc.org/sqlite DSN format (_pragma=NAME(VALUE), NOT _journal_mode=WAL)
    - embed.FS schema migrations with adlio/schema (idempotent, runs every startup)
    - Startup-then-interval cleanup scheduler pattern (purge on start + every 24h)
    - Transaction-wrapped purge with FK-correct delete order (dispatch_log before messages)

key-files:
  created:
    - internal/db/db.go
    - internal/db/schema/001_initial.sql
    - internal/cleanup/scheduler.go
    - internal/db/db_verify_test.go
  modified: []

key-decisions:
  - "modernc.org/sqlite DSN uses _pragma=NAME(VALUE) format — NOT _journal_mode=WAL (mattn/go-sqlite3 syntax)"
  - "SetMaxOpenConns(1) is mandatory for SQLite — prevents SQLITE_BUSY under concurrent goroutines"
  - "Cleanup runs immediately on startup then every interval (startup-then-interval, not wait-then-first)"
  - "dispatch_log deleted before messages in purge transaction (FK constraint order)"
  - "ReclaimStuck is a standalone function (not method) taking *sql.DB as parameter"

patterns-established:
  - "Pattern: SQLite Open with WAL mode uses file:path?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)&_pragma=synchronous(NORMAL)"
  - "Pattern: db.SetMaxOpenConns(1) + SetMaxIdleConns(1) + SetConnMaxLifetime(0) for single-writer SQLite"
  - "Pattern: //go:embed schema/*.sql var schemaFS embed.FS for zero-external-tool migrations"
  - "Pattern: cleanup.Start(ctx, db, interval) — caller controls interval, context cancels goroutine"

requirements-completed: [PERS-01, PERS-02, PERS-03, PERS-04]

# Metrics
duration: 8min
completed: 2026-02-21
---

# Phase 1 Plan 02: SQLite Persistence Layer Summary

**WAL-mode SQLite with embedded schema migrations (adlio/schema), single-writer pool, crash-recovery ReclaimStuck, and context-aware periodic cleanup scheduler**

## Performance

- **Duration:** 8 min
- **Started:** 2026-02-21T14:19:55Z
- **Completed:** 2026-02-21T14:28:00Z
- **Tasks:** 2
- **Files modified:** 4

## Accomplishments

- SQLite database package with Open() (WAL mode, single-writer pool), ApplySchema() (embedded migrations), and ReclaimStuck() (crash recovery)
- Schema creates messages, dispatch_log, and api_keys tables with appropriate indexes
- Cleanup scheduler purges delivered messages (>30 days) and failed messages (>90 days), using FK-correct delete order in a transaction
- All packages verified with go build, go vet, and functional test confirming WAL mode and table creation

## Task Commits

Each task was committed atomically:

1. **Task 1: Create SQLite database package with WAL mode and embedded schema** - `ae08a09` (feat)
2. **Task 2: Create cleanup scheduler for periodic message purging** - `65b4709` (feat)

**Plan metadata:** (this commit)

## Files Created/Modified

- `internal/db/db.go` - Open(), ApplySchema(), ReclaimStuck() functions; SQLite WAL setup
- `internal/db/schema/001_initial.sql` - DDL for messages, dispatch_log, api_keys tables + indexes
- `internal/cleanup/scheduler.go` - Start() with initial purge and time.Ticker goroutine
- `internal/db/db_verify_test.go` - Functional test verifying WAL mode, table creation, ReclaimStuck

## Decisions Made

- Used modernc.org/sqlite `_pragma=NAME(VALUE)` DSN format — mattn/go-sqlite3 format silently ignored
- SetMaxOpenConns(1) is mandatory for SQLite single-writer model
- Cleanup runs immediately on startup (startup-then-interval pattern, per RESEARCH.md recommendation)
- dispatch_log entries deleted before messages in purge to satisfy FK constraint

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Installed Go 1.22 compiler before project could be built**
- **Found during:** Pre-execution setup (prerequisite for Plan 02 tasks)
- **Issue:** Go compiler not installed on system; no Go toolchain available
- **Fix:** Installed golang-go via apt-get; verified with `go version go1.22.2`
- **Files modified:** System packages only (no project files)
- **Verification:** `go build ./cmd/jaimito/` succeeds after installation
- **Committed in:** N/A (system-level change)

**2. [Rule 3 - Blocking] Executed Plan 01 prerequisites before Plan 02**
- **Found during:** Pre-execution, when no go.mod or source files existed
- **Issue:** Plan 02 (db and cleanup packages) requires go.mod and module initialization from Plan 01
- **Fix:** Executed Plan 01 tasks (go mod init, dependency installation, config package) as prerequisite
- **Files modified:** go.mod, go.sum, cmd/jaimito/main.go, internal/config/config.go, configs/config.example.yaml
- **Verification:** `go build ./cmd/jaimito/` and `go build ./internal/config/...` succeed
- **Committed in:** 604675e (Plan 01 prerequisite commit)

---

**Total deviations:** 2 auto-fixed (both Rule 3 - Blocking prerequisites)
**Impact on plan:** Both required for basic executability. No scope creep — Plan 01 artifacts are exactly what Plan 01 would have produced.

## Issues Encountered

- Go 1.22 available via apt but `golang-go` meta-package was needed (not `golang-1.22-go` specifically)
- First `go get` ran under go1.24.0 toolchain (downloaded automatically), resulting in empty go.sum until re-running from project directory

## User Setup Required

None - no external service configuration required for the persistence layer.

## Next Phase Readiness

- db.Open(), db.ApplySchema(), db.ReclaimStuck(), cleanup.Start() all ready for wiring in main.go (Plan 03)
- Schema is established and idempotent — safe for Plan 03 to build on top of
- No blockers for Plan 03 (config + service wiring + systemd unit)

## Self-Check: PASSED

- FOUND: internal/db/db.go
- FOUND: internal/db/schema/001_initial.sql
- FOUND: internal/cleanup/scheduler.go
- FOUND: internal/db/db_verify_test.go
- FOUND commit: ae08a09 (Task 1)
- FOUND commit: 65b4709 (Task 2)

---
*Phase: 01-foundation*
*Completed: 2026-02-21*
