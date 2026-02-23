# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-02-20)

**Core value:** Every event that happens on the VPS gets reliably captured and delivered to Telegram — no notification is ever lost silently.
**Current focus:** Phase 3 — executing (CLI and Developer Experience)

## Current Position

Phase: 3 of 3 (CLI and Developer Experience) — IN PROGRESS
Plan: 2 of 3 in current phase
Status: Plans ready for execution
Last activity: 2026-02-23 — 03-02 complete: HTTP client package, send subcommand

Progress: [█████████░] 93%

## Performance Metrics

**Velocity:**
- Total plans completed: 8
- Average duration: 3 min
- Total execution time: 0.4 hours

**By Phase:**

| Phase | Plans | Total | Avg/Plan |
|-------|-------|-------|----------|
| 01-foundation | 3/3 | 16 min | 5 min |
| 02-core-pipeline | 4/4 | 7 min | 2 min |
| 03-cli-developer-experience | 2/3 | 3 min | 1.5 min |

**Recent Trend:**
- Last 5 plans: 02-01 (3 min), 02-03 (2 min), 02-04 (2 min), 03-01 (2 min), 03-02 (1 min)
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
- [02-02]: NewRouter takes no *bot.Bot — API layer enqueues to DB only; dispatcher reads independently (clean separation)
- [02-02]: BearerAuth uses db.HashToken (same function as SeedKeys) — no hash mismatch possible
- [02-02]: channel and priority defaults (general, normal) applied before validation so partial payloads are valid
- [02-03]: Tags use literal # prefix written directly, then bot.EscapeMarkdown(tag) — passing "#tag" through EscapeMarkdown would produce \#tag
- [02-03]: models.ParseModeMarkdown = "MarkdownV2" (use this); ParseModeMarkdownV1 = "Markdown" (old V1, do not use)
- [02-03]: After 5 exhausted retries dispatcher returns nil (not error) — failure logged at Error level, silenced per CONTEXT.md
- [02-03]: go-telegram/bot v1.19.0 TooManyRequestsError.RetryAfter is plain int (seconds) — no adaptive_retry field
- [02-04]: Dispatcher and cleanup start before HTTP server — delivery goroutines live before first API call lands
- [02-04]: HTTP shutdown uses context.WithTimeout(context.Background(), 30s) — parent ctx already cancelled at shutdown time
- [02-04]: errors.Is(err, http.ErrServerClosed) required — ListenAndServe returns this non-error on clean Shutdown()
- [03-01]: SilenceUsage+SilenceErrors on rootCmd — cobra won't print usage on RunE errors; main.go owns error display
- [03-01]: Bare jaimito (no subcommand) starts server via rootCmd.RunE = runServe — backward compatible
- [03-01]: keys subcommands use direct DB access — key management bypasses auth middleware by design
- [03-01]: openDB applies schema on each invocation — idempotent, CLI works on fresh or existing DB
- [03-01]: CreateKey uses crypto/rand 32 bytes -> hex -> sk- prefix — 256 bits entropy
- [03-02]: internal/client does not import internal/api — client-side NotifyRequest mirrors server struct to avoid circular dependency
- [03-02]: New() takes host:port and prepends http:// — consistent with config.Server.Listen format
- [03-02]: Title is pointer field only set when --title flag explicitly provided; channel/priority sent empty (server applies defaults)

### Pending Todos

None yet.

### Blockers/Concerns

None.

## Session Continuity

Last session: 2026-02-23
Stopped at: Completed 03-02-PLAN.md — HTTP client package, send subcommand with full flag support
Resume file: None
