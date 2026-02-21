# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-02-20)

**Core value:** Every event that happens on the VPS gets reliably captured and delivered to Telegram — no notification is ever lost silently.
**Current focus:** Phase 2 — Core Pipeline

## Current Position

Phase: 2 of 3 (Core Pipeline)
Plan: 1 of 3 in current phase
Status: Plan complete
Last activity: 2026-02-21 — Plan 02-01 complete: config extension, schema migration 002, message queue CRUD, API key operations

Progress: [████░░░░░░] 44%

## Performance Metrics

**Velocity:**
- Total plans completed: 4
- Average duration: 4 min
- Total execution time: 0.3 hours

**By Phase:**

| Phase | Plans | Total | Avg/Plan |
|-------|-------|-------|----------|
| 01-foundation | 3/3 | 16 min | 5 min |
| 02-core-pipeline | 1/3 | 3 min | 3 min |

**Recent Trend:**
- Last 5 plans: 01-01 (6 min), 01-02 (8 min), 01-03 (2 min), 02-01 (3 min)
- Trend: Stable

*Updated after each plan completion*

## Accumulated Context

### Decisions

Decisions are logged in PROJECT.md Key Decisions table.
Recent decisions affecting current work:

- [Pre-phase]: Use `modernc.org/sqlite` (CGO-free) instead of `mattn/go-sqlite3` — required for single-binary cross-compile
- [Pre-phase]: Single-writer SQLite connection pool pattern — must be set in Phase 1, not retrofitted
- [Pre-phase]: Parse `retry_after` from Telegram 429 responses — required in Phase 2 dispatcher; fixed-delay retries will get the bot IP banned
- [01-01]: Module path github.com/chiguire/jaimito chosen as canonical module path
- [01-01]: modernc.org/sqlite v1.46.1 requires go 1.24+; go.mod upgraded from 1.22 to 1.24
- [01-01]: Config validation returns first error (fail-fast), not collected errors
- [01-02]: modernc.org/sqlite uses _pragma=NAME(VALUE) DSN format (confirmed working) — _journal_mode=WAL format silently ignored
- [01-02]: SetMaxOpenConns(1) is critical for SQLite single-writer model (prevents SQLITE_BUSY)
- [01-02]: Cleanup scheduler runs immediately on startup then every interval (startup-then-interval)
- [01-02]: dispatch_log must be deleted before messages in purge (FK constraint order)
- [01-03]: GetChatParams.ChatID is type `any` in go-telegram/bot v1.19.0 — int64 passed directly without conversion
- [01-03]: Chat deduplication stores map[int64]string (chatID -> channel name) for error messages naming unreachable channel
- [01-03]: cleanup.Start internally spawns goroutine — main goroutine just waits on <-ctx.Done()
- [02-01]: server.listen defaults to 127.0.0.1:8080 (localhost-only) for VPS security — not 0.0.0.0
- [02-01]: HashToken exported at package level — shared between SeedKeys and auth middleware to prevent divergence
- [02-01]: SeedKeys uses INSERT OR IGNORE — idempotent, safe on every startup
- [02-01]: SeedKeys logs key name at Info level only — raw key value never logged

### Pending Todos

None yet.

### Blockers/Concerns

- [Phase 2]: Verify whether `go-telegram/bot` v1.19.0 `TooManyRequestsError` exposes the Bot API 8.0 `adaptive_retry` field, or if raw response parsing is needed

## Session Continuity

Last session: 2026-02-21
Stopped at: Completed 02-01-PLAN.md — config extension, schema migration 002, message queue CRUD, API key DB operations
Resume file: None
