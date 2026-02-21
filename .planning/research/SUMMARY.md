# Project Research Summary

**Project:** jaimito
**Domain:** Self-hosted VPS push notification hub (Go + SQLite + Telegram)
**Researched:** 2026-02-20
**Confidence:** HIGH

## Executive Summary

Jaimito is a self-hosted, single-binary notification hub that receives alerts via HTTP webhook and CLI, queues them durably in SQLite, and dispatches them to Telegram. The dominant pattern for this class of tool (ntfy, Gotify) is an HTTP ingest endpoint with bearer token auth, a persistent queue, and async dispatch. Jaimito's architectural distinction is that it is push-only (no subscribe model), which enables server-side retry — a gap that ntfy and Gotify leave to the client. The recommended stack is unambiguous: Go 1.26, `modernc.org/sqlite` (CGO-free), `go-chi/chi` for HTTP routing, `spf13/cobra` for CLI, and `go-telegram/bot` for dispatch. All library versions are verified current as of February 2026.

The killer feature is `jaimito wrap <cmd>`, a cron job wrapper that captures exit codes and output and notifies only on failure. No competitor bundles this in its core binary. Combined with Telegram-native MarkdownV2 formatting and in-process key management, jaimito has a clear differentiation story for the VPS operator audience. The MVP is achievable in a single development phase covering the full ingest-queue-dispatch loop plus the CLI, with observability and rate limiting following in a second phase.

The primary risks are all known and well-documented: SQLite write concurrency requires explicit `BEGIN IMMEDIATE` transactions or a single-writer goroutine pattern; Telegram's 429 rate limit requires honoring the `retry_after` field, not fixed-delay retries; and MarkdownV2 requires escaping 18 special characters in dynamic content. All three risks have clear prevention strategies that must be built into Phase 1, not retrofitted. A secondary risk is that `jaimito wrap` deployed to production cron jobs without deduplication in place can trigger notification storms — Phase 2 deduplication should land before any production cron integration.

## Key Findings

### Recommended Stack

The stack is 100% CGO-free Go, enabling true `CGO_ENABLED=0` cross-compilation to `linux/amd64` and `linux/arm64`. `modernc.org/sqlite` (a pure-Go transpilation of SQLite C) replaces the more commonly documented `mattn/go-sqlite3`, eliminating the CGO and GCC dependencies that would break the single-binary deployment promise. The performance trade-off (10-15% slower writes) is irrelevant at jaimito's expected volume.

**Core technologies:**
- **Go 1.26** — latest stable (released 2026-02-10); stdlib `net/http` ServeMux routing, `log/slog` structured logging, `go:embed` for migrations; single binary <10MB
- **`modernc.org/sqlite` v1.46.1** — CGO-free SQLite driver; WAL mode supported; critical for the single-binary deployment promise
- **`go-chi/chi/v5` v5.2.5** — stdlib-compatible HTTP router; middleware grouping enables per-route auth, rate limiting, and request logging cleanly
- **`spf13/cobra` v1.10.2** — CLI subcommands (`send`, `wrap`, `keys`, `status`); shell completion auto-generated
- **`go-telegram/bot` v1.19.0** — zero-dependency Telegram Bot API 9.4 wrapper; typed 429 errors with `retry_after` for backoff logic
- **`pressly/goose/v3` v3.26.0** — embedded SQL migrations at startup; transaction-safe (unlike golang-migrate); native `slog` integration
- **`google/uuid` v1.6.0** — UUID v7 (time-ordered) as primary key; sortable without a separate `created_at` index
- **`go.yaml.in/yaml/v3` v3.0.4** — canonical YAML library (gopkg.in/yaml.v3 is unmaintained; cobra already migrated)

**Do not use:** `mattn/go-sqlite3` (breaks cross-compile), `gin-gonic/gin` (50+ deps, custom context), `go-telegram-bot-api/v5` (stale, Dec 2021), `gorilla/mux` (archived), `GORM` (unnecessary for 2-table schema), `gopkg.in/yaml.v3` (unmaintained).

### Expected Features

The feature research surveyed ntfy, Gotify, and Apprise and identified three categories.

**Must have (table stakes) — v0.1:**
- `POST /api/v1/notify` with Bearer token auth — the ingest gate; unauthenticated endpoint is not internet-safe
- `GET /api/v1/health` — required by UptimeKuma, Gatus, and other monitors
- SQLite queue with WAL mode — message durability across restarts; ntfy defaults to 12h cache, Gotify keeps until deleted
- Telegram dispatcher with priority-based emoji formatting — Telegram is the UI; no persistence needed in jaimito itself
- Automatic retry with exponential backoff — the gap ntfy and Gotify both leave open; jaimito's dispatch model is the fix
- Priority system: critical/high/normal/low with cosmetic differentiation — urgency signaling; behavioral differentiation deferred
- `jaimito send` CLI — shell integration without curl boilerplate
- `jaimito wrap <cmd>` — captures exit code and output; notifies on failure; the killer feature
- `jaimito keys create/list/revoke` — in-process key management without config edits or restarts
- YAML config at `/etc/jaimito/config.yaml` — version-controllable, automation-friendly
- Channel-based routing with named channels — organizes alert sources; channel defaults in config

**Should have (competitive) — v0.2-v0.3:**
- Rate limiting per channel — prevents notification storms from looping cron jobs
- Quiet hours (do-not-disturb window) — needed for daily-schedule operators
- Simple deduplication (same message within N minutes = suppress) — covers 80% of alert fatigue
- `jaimito list` CLI — query recent messages and delivery status
- Message queue drain on SIGTERM — flush pending notifications before exit

**Defer (v1.0+):**
- Dead man's switch / heartbeat monitoring — healthchecks.io exists; build only if explicitly requested
- Web dashboard — Telegram history plus `jaimito list` satisfies the use case; a dashboard is a product pivot
- Multi-dispatcher (Email, Slack, Discord) — Apprise covers this; recommend integration rather than rebuilding
- Plugin system — maintenance burden exceeds value at this scale

**Confirmed anti-features (deliberate omissions):** WebSocket stream, multi-user support, attachment storage, SMTP ingestor, message templating engine.

### Architecture Approach

The architecture is a simple 3-layer pipeline: **Ingestion** (HTTP webhook `POST /api/v1/notify` + `jaimito send` CLI) writes to the **Queue** (SQLite WAL-mode `messages` table), and the **Dispatcher** (background goroutine loop) reads from the queue and delivers to Telegram, updating status on success or incrementing `retry_count` and setting `next_retry_at` on failure. There is no external message broker; SQLite is the queue. This matches the architecture of similar single-binary tools (ntfy, Gotify) and is appropriate for the single-user VPS deployment target.

**Major components:**
1. **HTTP Server** (chi router) — accepts `POST /api/v1/notify`, `GET /api/v1/health`; auth middleware on `/api/v1/*`; writes 202 immediately and enqueues
2. **SQLite Queue** (modernc.org/sqlite + goose migrations) — `messages` table with `status`, `priority`, `channel`, `retry_count`, `next_retry_at`, `dedupe_key`; `api_keys` table; WAL mode; single-writer connection pool
3. **Dispatcher Loop** (goroutine) — polls queue for `status='queued' AND next_retry_at <= now`; dispatches to Telegram; updates status; reclaims stuck `dispatching` rows on startup
4. **Telegram Dispatcher** (go-telegram/bot) — sends with MarkdownV2; handles 429 with `retry_after`; detects permanent errors (403, 400 chat not found) and marks `failed` without retry
5. **CLI** (cobra) — `jaimito send` (HTTP call to local server or direct queue write), `jaimito wrap` (execute + capture output + send on failure), `jaimito keys` (CRUD on `api_keys` table), `jaimito status` (queue depth, dispatcher state)

### Critical Pitfalls

1. **SQLite DEFERRED transactions cause SQLITE_BUSY under concurrent writes** — the dispatcher and HTTP ingestor writing simultaneously will race. Prevention: set `MaxOpenConns(1)` on the write connection pool (single-writer pattern) and use `BEGIN IMMEDIATE` for any write transaction. Set `busy_timeout=5000` in the DSN as a floor, but do not rely on it as the only guard. Must be correct in Phase 1; wrong defaults cause subtle data loss that is hard to reproduce later.

2. **Telegram 429 rate limit blacklists the bot IP, not just the burst** — if the dispatcher retries immediately on 429, Telegram bans the bot IP for 30 seconds and all notifications fail. Prevention: parse `retry_after` from every 429 response (and `adaptive_retry` from Bot API 8.0+); store `now + retry_after` in `next_retry_at`; never retry synchronously. Implement a per-dispatcher token bucket (1 msg/sec per chat, 20/min for groups) proactively. Will be triggered in the first week of real use if any cron jobs run at round minutes.

3. **Telegram MarkdownV2 requires escaping 18 special characters** — unescaped `.`, `-`, `(`, `)`, `!`, `_`, and backticks cause a 400 Bad Request and the message is silently lost (not retried). Production alert bodies always contain these characters. Prevention: implement `EscapeMarkdownV2(s string) string` at the same time as the dispatcher, not as a follow-up. Alternative: use `parse_mode: HTML` (only 4 characters to escape).

4. **Dispatcher goroutine can leave messages stuck in `dispatching` status** — a crash mid-dispatch leaves rows that are never reclaimed. Prevention: add `dispatching_since` timestamp column; on startup, reclaim any `dispatching` rows older than `2 * dispatch_timeout`; enforce `max_retries` hard limit and transition to `failed` status with the error reason preserved.

5. **Plain-text secrets in YAML config committed to git** — Telegram bot token in `config.yaml` committed and pushed is permanently in git history. Prevention: add `config.yaml` and `*.db` to `.gitignore` before the first commit; support `JAIMITO_TELEGRAM_BOT_TOKEN` env var override; enforce `0600` file permissions at startup.

## Implications for Roadmap

Based on combined research, the ingest-queue-dispatch pipeline has clear internal dependencies that dictate a natural build order. The HTTP server and SQLite layer must be working before the dispatcher has anything to consume. The CLI `send` command shares the delivery pipeline with `wrap`, so they come together. Observability and rate limiting are meaningful only once the core loop is proven stable.

### Phase 1: Foundation and Core Pipeline

**Rationale:** The entire product value depends on a working ingest-queue-dispatch loop. Auth must ship with the ingest endpoint (not retrofitted), and the SQLite connection pool must be configured correctly from day one. All six critical pitfalls are Phase 1 concerns; retrofitting any of them is expensive.

**Delivers:** A deployable service that receives HTTP webhook notifications, queues them durably, and dispatches to Telegram with retry. Covers the primary use case: silent cron failure alerting.

**Addresses (from FEATURES.md):** HTTP webhook endpoint, Bearer token auth, SQLite WAL queue, Telegram dispatcher, automatic retry with exponential backoff, priority system (cosmetic), health check endpoint, YAML config.

**Avoids (from PITFALLS.md):** SQLite DEFERRED transaction race (single-writer pattern from day one), CGO binary breakage (modernc.org/sqlite from the start), Telegram 429 mishandling (parse retry_after in initial dispatcher), MarkdownV2 escaping (implement EscapeMarkdownV2 with the dispatcher), request body size limit (MaxBytesReader in handler setup), stuck dispatching rows (startup reclaim logic), secrets in git (gitignore and env var override before first commit).

### Phase 2: CLI and Developer Experience

**Rationale:** Once the server pipeline is stable, the CLI completes the user-facing surface. `jaimito send` and `jaimito wrap` share the delivery pipeline already built in Phase 1. Key management via CLI requires the `api_keys` table (Phase 1 dependency). `wrap` is the differentiating feature and should land before any other enhancement.

**Delivers:** A complete CLI (`send`, `wrap`, `keys`, `status`); operator can use jaimito from shell scripts and cron without curl boilerplate; `wrap` solves the silent cron failure use case natively.

**Addresses (from FEATURES.md):** `jaimito send`, `jaimito wrap <cmd>` (killer feature), `jaimito keys create/list/revoke`, channel-based routing with named channels.

**Avoids (from PITFALLS.md):** CLI API key in shell history (credential priority: env var -> config file -> prompt, never CLI flag), `wrap` notify-on-every-success noise (default to failure-only; --always flag for success), Telegram 4096-char message limit from `wrap` output (truncate at 3800 chars with truncation indicator), CLI exit code 0 on server unreachable (must exit 1 on non-2xx or connection refused).

**Note:** Do not deploy `jaimito wrap` to production cron jobs before Phase 3 deduplication is in place. A looping cron job will flood Telegram and trigger rate limiting.

### Phase 3: Reliability and Rate Control

**Rationale:** The features in Phase 1 and 2 are sufficient to hit notification storms from looping cron jobs or repeated failures. Deduplication and rate limiting are the difference between jaimito being useful and being silenced by alert fatigue. The WAL file monitoring belongs here as an operational stability concern.

**Delivers:** Rate limiting per channel, simple deduplication by `dedupe_key`, quiet hours, `jaimito list` for delivery status queries, WAL checkpoint monitoring in health endpoint, graceful shutdown with queue drain on SIGTERM.

**Addresses (from FEATURES.md):** Rate limiting per channel, simple deduplication (same message within N minutes = suppress), quiet hours, `jaimito list` / delivery status visibility, message queue drain on shutdown.

**Avoids (from PITFALLS.md):** Notification flooding from loop bugs (per-channel rate limit + deduplication window), WAL file unbounded growth (periodic PRAGMA wal_checkpoint(PASSIVE), WAL size in health check), `wrap` failure storm suppression (N failures in M seconds = one summary message).

### Phase Ordering Rationale

- The HTTP server must exist before the CLI `send` command has an endpoint to call; the queue must exist before the dispatcher has rows to consume. The pipeline layers are strictly ordered by data flow.
- Auth must ship with the ingest endpoint in Phase 1. A public HTTP endpoint without auth is not acceptable even during development on a VPS.
- `jaimito wrap` (Phase 2) intentionally precedes rate limiting (Phase 3) in implementation order but should not be deployed to production cron jobs until Phase 3 is complete. The implementation is simple; the operational safety requires deduplication.
- Observability features (`jaimito list`, WAL monitoring) land in Phase 3 because they require a stable production baseline to be meaningful. Adding monitoring before the monitored system is correct produces noisy data.

### Research Flags

Phases likely needing deeper research during planning:

- **Phase 1 (SQLite connection pool):** The single-writer pattern with `modernc.org/sqlite` has nuances around DSN options and connection pool configuration (`_journal_mode`, `_busy_timeout`, `_foreign_keys`, `_synchronous`). Verify the exact DSN string and connection pool settings against the modernc.org/sqlite documentation before writing the database layer.
- **Phase 1 (Telegram dispatcher):** Bot API 8.0 (November 2025) added the `adaptive_retry` field to 429 responses. Verify `go-telegram/bot` v1.19.0 surfaces this field, or implement raw response parsing for the 429 case.

Phases with standard patterns (skip research-phase):

- **Phase 2 (CLI with cobra):** `spf13/cobra` is extremely well-documented; subcommand + flag patterns are standard. The credential lookup priority (env var -> config file -> prompt) is a known pattern. No additional research needed.
- **Phase 3 (rate limiting):** A simple token bucket per channel using `time.Ticker` or `golang.org/x/time/rate` is a well-documented Go pattern. No novel research required.

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| Stack | HIGH | All library versions verified against pkg.go.dev as of 2026-02-20; Go 1.26 confirmed released 2026-02-10 |
| Features | HIGH | Competitor analysis against ntfy official docs, Gotify GitHub, and Apprise; feature gaps verified against multiple practitioner sources |
| Architecture | HIGH | Architecture is defined in the PRD (3-layer pipeline); validated against established patterns from ntfy, Gotify, and goqite reference implementation |
| Pitfalls | MEDIUM-HIGH | SQLite and Go findings HIGH (multiple verified technical sources); Telegram API 8.0 `adaptive_retry` field MEDIUM (official docs confirmed via web search, not Context7) |

**Overall confidence:** HIGH

### Gaps to Address

- **Telegram `adaptive_retry` field (Bot API 8.0):** Confirmed present in the API but `go-telegram/bot` v1.19.0 typed error surface for `adaptive_retry` has not been verified. During Phase 1 dispatcher implementation, check whether `TooManyRequestsError` exposes this field or whether raw response parsing is needed.
- **modernc.org/sqlite DSN options:** The exact DSN parameter names for WAL mode, foreign keys, and busy timeout with `modernc.org/sqlite` v1.46.1 may differ from `mattn/go-sqlite3`. Verify DSN format (`file:path?_journal_mode=WAL&_busy_timeout=5000&_foreign_keys=on`) against the modernc.org/sqlite documentation before writing the database initialization code.
- **Priority behavioral differences:** Research confirmed that priority-as-behavior (not just cosmetic label) would be a differentiating feature. The MVP ships priority cosmetically. Whether to implement behavioral differentiation in Phase 1 or defer to Phase 3 should be resolved during requirements definition.

## Sources

### Primary (HIGH confidence)
- `pkg.go.dev/modernc.org/sqlite` — v1.46.1 verified; CGO-free; WAL support confirmed
- `pkg.go.dev/github.com/go-telegram/bot` — v1.19.0; Bot API 9.4; typed 429 errors confirmed
- `pkg.go.dev/github.com/go-chi/chi/v5` — v5.2.5; Go 1.22+ minimum; middleware grouping verified
- `pkg.go.dev/github.com/spf13/cobra` — v1.10.2; yaml.v3 migration to go.yaml.in confirmed
- `pkg.go.dev/github.com/pressly/goose/v3` — v3.26.0; WithSlog added; transaction-safe migrations
- `go.dev/doc/devel/release` — Go 1.26 released 2026-02-10 confirmed
- `docs.ntfy.sh` — ntfy feature set, publish API, config reference (official documentation)
- `github.com/gotify/server` — Gotify feature set (official repository)
- `sqlite.org/wal.html` — WAL mode concurrency characteristics (official SQLite docs)
- `core.telegram.org/bots/faq` — Telegram rate limit behavior (official Telegram docs)
- `blog.cloudflare.com` — Go net/http timeouts reference (authoritative Go networking)
- `tenthousandmeters.com` — SQLite concurrent writes and SQLITE_BUSY (technical deep-dive with benchmarks)
- `threedots.tech` — Durable background execution with Go and SQLite (established Go architecture blog)

### Secondary (MEDIUM confidence)
- `alexedwards.net/blog/which-go-router-should-i-use` — chi vs stdlib analysis 2025
- `blog.vezpi.com/en/post/notification-system-gotify-vs-ntfy/` — competitor comparison
- `core.telegram.org/bots/api` — Bot API 8.0 adaptive_retry field (web search confirmation)
- `github.com/maragudk/goqite` — SQLite-backed queue reference implementation
- `jacobgold.co/posts/go-sqlite-best-practices` — Go + SQLite best practices
- `thomaswildetech.com/blog/2026/01/05/the-holy-grail-of-self-hosted-notifications/` — practitioner VPS notification pain points (Jan 2026)
- `github.com/TelegramBotAPI/errors` — community-maintained Telegram error codes list

---
*Research completed: 2026-02-21*
*Ready for roadmap: yes*
