# jaimito — VPS Push Notification Hub

## What This Is

jaimito is a lightweight, self-hosted notification hub that centralizes all alerts generated on a VPS (service events, cron job results, application errors, health checks) and dispatches them to Telegram via a single Go binary backed by SQLite. Services send messages through a webhook HTTP API or a CLI companion (`jaimito send`, `jaimito wrap`); jaimito queues, persists, and delivers them with automatic retries.

## Core Value

Every event that happens on the VPS gets reliably captured and delivered to Telegram — no notification is ever lost silently.

## Requirements

### Validated

- ✓ HTTP webhook endpoint (`POST /api/v1/notify`) with Bearer token auth — v1.0
- ✓ CLI companion with `jaimito send` and `jaimito wrap` commands — v1.0
- ✓ Telegram dispatcher with priority-based emoji formatting — v1.0
- ✓ SQLite persistence with WAL mode for the message queue — v1.0
- ✓ Channel-based message routing with predefined channels — v1.0
- ✓ Priority system (critical/high/normal/low) with emoji differentiation — v1.0
- ✓ API key management via CLI (`jaimito keys create/list/revoke`) — v1.0
- ✓ YAML configuration file (`/etc/jaimito/config.yaml`) — v1.0
- ✓ Health check endpoint (`GET /api/v1/health`) — v1.0
- ✓ Automatic retries with exponential backoff for failed deliveries — v1.0
- ✓ `jaimito wrap` captures command output and sends notification on failure — v1.0

### Active

- [ ] Interactive CLI setup wizard (`jaimito setup`) with bubbletea TUI
- [ ] Live validation of Telegram bot token and chat IDs against API
- [ ] Auto-generation of API key and config YAML writing
- [ ] Test notification to prove setup works before finishing
- [ ] install.sh integration (replaces manual config.example.yaml copy)

## Current Milestone: v1.1 Setup Wizard

**Goal:** Eliminate the manual config.yaml editing barrier with an interactive CLI wizard that guides users through setup, validates everything live, and sends a test notification.

**Target features:**
- `jaimito setup` cobra subcommand with bubbletea TUI
- Step-by-step wizard: bot token → channels → server → db → API key → summary
- Live Telegram API validation (bot token, chat IDs)
- Auto-generated API key with `db.GenerateRawKey()`
- Config YAML generation and writing with proper permissions
- Test notification via bot API
- install.sh integration with `/dev/tty` redirect for `curl | bash`

### Out of Scope

- Email/SMTP dispatcher — deferred to future milestone
- HTTP generic/cURL dispatcher — deferred to future milestone
- File watcher ingestor — deferred to future milestone
- Systemd watcher — deferred to v2+
- Dashboard web — anti-feature: Telegram IS the history
- Message grouping/digest — deferred to future milestone
- Deduplication — deferred to future milestone
- Rate limiting — deferred to future milestone
- Quiet hours — deferred to future milestone
- Query API (list messages, stats) — deferred to future milestone
- WhatsApp/PagerDuty/Matrix dispatchers — deferred to v2+

## Context

- Shipped v1.0 MVP with 2,090 LOC Go across 62 files
- Tech stack: Go 1.24, modernc.org/sqlite (CGO-free), chi v5, cobra, go-telegram/bot
- Runs on the same VPS it monitors (single machine deployment)
- Primary pain point solved: cron jobs no longer fail silently (`jaimito wrap`)
- Single binary, zero external dependencies, ~50MB memory footprint
- 16 unit tests passing

## Constraints

- **Language**: Go — single binary, no runtime dependencies
- **Database**: SQLite via modernc.org/sqlite (CGO-free) — no external database servers
- **Deployment**: systemd unit on Linux (amd64 primary, arm64 optional)
- **Memory**: Target <50MB in normal operation
- **Config**: Single YAML file at `/etc/jaimito/config.yaml`
- **Network**: Listens on `127.0.0.1:8080` by default (behind reverse proxy for external access)
- **Auth**: Bearer tokens with `sk-` prefix, SHA-256 hashed, stored in SQLite

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| Go over Rust/Python | Single binary, low memory, mature ecosystem for HTTP+SQLite | ✓ Good — 2,090 LOC, clean build, fast iteration |
| SQLite over Postgres/Redis | Zero dependencies, file-level backup, sufficient for VPS scale | ✓ Good — WAL mode, single-writer pattern works |
| MVP scope (v0.1→v1.0) first | Ship fast, validate the core loop (ingest → queue → deliver) | ✓ Good — 5 days to full MVP |
| CLI includes `wrap` in MVP | Cron monitoring is the primary pain point — `wrap` is the killer feature | ✓ Good — killer feature delivered |
| Same VPS deployment | Simplicity; separation can come later if jaimito proves valuable | ✓ Good — single binary, trivial deploy |
| modernc.org/sqlite (CGO-free) | Single-binary cross-compile without CGO dependency chain | ✓ Good — no build complications |
| cobra for CLI | Industry standard, subcommand support, persistent flags | ✓ Good — clean CLI architecture |
| chi v5 for HTTP | Lightweight, stdlib-compatible, good middleware ecosystem | ✓ Good — clean routing, middleware composition |
| Single-writer SQLite pool | SetMaxOpenConns(1) prevents SQLITE_BUSY in WAL mode | ✓ Good — no concurrency issues |
| API/dispatcher separation | API enqueues to DB, dispatcher reads independently | ✓ Good — clean boundary, testable |

---
*Last updated: 2026-03-23 after v1.1 milestone start*
