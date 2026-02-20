# jaimito — VPS Push Notification Hub

## What This Is

jaimito is a lightweight, self-hosted notification hub that centralizes all alerts generated on a VPS (service events, cron job results, application errors, health checks) and dispatches them to Telegram via a single Go binary backed by SQLite. Services send messages through a webhook HTTP API or a CLI companion; jaimito queues, persists, and delivers them with automatic retries.

## Core Value

Every event that happens on the VPS gets reliably captured and delivered to Telegram — no notification is ever lost silently.

## Requirements

### Validated

(None yet — ship to validate)

### Active

- [ ] HTTP webhook endpoint (`POST /api/v1/notify`) with Bearer token auth
- [ ] CLI companion with `jaimito send` and `jaimito wrap` commands
- [ ] Telegram dispatcher with priority-based emoji formatting
- [ ] SQLite persistence with WAL mode for the message queue
- [ ] Channel-based message routing with predefined channels
- [ ] Priority system (critical/high/normal/low) with behavioral differences
- [ ] API key management via CLI (`jaimito keys create/list/revoke`)
- [ ] YAML configuration file (`/etc/jaimito/config.yaml`)
- [ ] Health check endpoint (`GET /api/v1/health`)
- [ ] Automatic retries with exponential backoff for failed deliveries
- [ ] `jaimito wrap` captures command output and sends notification on failure

### Out of Scope

- Email/SMTP dispatcher — deferred to v0.2
- HTTP generic/cURL dispatcher — deferred to v0.3
- File watcher ingestor — deferred to v0.4
- Systemd watcher — deferred to v2
- Dashboard web — deferred to v2
- Message grouping/digest — deferred to v0.3
- Deduplication — deferred to v0.3
- Rate limiting — deferred to v0.2
- Quiet hours — deferred to v0.2
- Query API (list messages, stats) — deferred to v0.4
- WhatsApp/PagerDuty/Matrix dispatchers — deferred to v2+

## Context

- Runs on the same VPS it monitors (single machine deployment)
- Intended for a personal VPS with multiple services and cron jobs
- Currently notifications are fragmented: each service has its own alerting logic (or none)
- Cron jobs fail silently — this is the primary pain point
- The PRD specifies a comprehensive v1.0 vision; this project starts with MVP (v0.1) scope
- Go chosen for: single binary distribution, low memory footprint (<50MB), native concurrency
- SQLite chosen for: zero external dependencies, trivial backup, sufficient throughput (hundreds of messages/hour)

## Constraints

- **Language**: Go — single binary, no runtime dependencies
- **Database**: SQLite with CGo — no external database servers
- **Deployment**: systemd unit on Linux (amd64 primary, arm64 optional)
- **Memory**: Target <50MB in normal operation
- **Config**: Single YAML file at `/etc/jaimito/config.yaml`
- **Network**: Listens on `127.0.0.1:8787` by default (behind reverse proxy for external access)
- **Auth**: Bearer tokens with `sk-` prefix, stored in SQLite

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| Go over Rust/Python | Single binary, low memory, mature ecosystem for HTTP+SQLite | — Pending |
| SQLite over Postgres/Redis | Zero dependencies, file-level backup, sufficient for VPS scale | — Pending |
| MVP scope (v0.1) first | Ship fast, validate the core loop (ingest → queue → deliver) before adding complexity | — Pending |
| CLI includes `wrap` in MVP | Cron monitoring is the primary pain point — `wrap` is the killer feature | — Pending |
| Same VPS deployment | Simplicity; separation can come later if jaimito proves valuable | — Pending |

---
*Last updated: 2026-02-20 after initialization*
