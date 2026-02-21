# Roadmap: jaimito

## Overview

jaimito ships in three phases that follow the natural data flow of the product: first build the durable foundation (config, SQLite queue, systemd), then wire up the full ingest-queue-dispatch pipeline (HTTP API, Telegram, retry), then surface the CLI that makes the tool actually usable from cron and shell scripts. Every v1 requirement maps to exactly one phase and every phase delivers a coherent, verifiable capability.

## Phases

**Phase Numbering:**
- Integer phases (1, 2, 3): Planned milestone work
- Decimal phases (2.1, 2.2): Urgent insertions (marked with INSERTED)

Decimal phases appear between their surrounding integers in numeric order.

- [x] **Phase 1: Foundation** - Project scaffold, YAML config, SQLite persistence layer, and systemd unit
- [x] **Phase 2: Core Pipeline** - HTTP ingest endpoint with auth, Telegram dispatcher with retry — the product is alive
- [ ] **Phase 3: CLI and Developer Experience** - `jaimito send`, `jaimito wrap`, and `jaimito keys` completing the full user-facing surface

## Phase Details

### Phase 1: Foundation
**Goal**: A deployable Go binary that reads config, initializes a WAL-mode SQLite database with the correct schema, and installs as a systemd service
**Depends on**: Nothing (first phase)
**Requirements**: CONF-01, CONF-02, CONF-03, PERS-01, PERS-02, PERS-03, PERS-04
**Success Criteria** (what must be TRUE):
  1. Running `jaimito` reads `/etc/jaimito/config.yaml` and starts without error, logging the configured channels and Telegram target
  2. The SQLite database file exists after startup with `messages`, `dispatch_log`, and `api_keys` tables present and WAL mode enabled
  3. On startup after a crash, any rows stuck in `dispatching` status are reclaimed to `queued` automatically (logged at INFO level)
  4. Delivered messages older than 30 days and failed messages older than 90 days are purged on the scheduled cleanup run
  5. The systemd unit file installs and `systemctl start jaimito` brings up the process; `systemctl status jaimito` shows active
**Plans:** 3/3 plans complete

Plans:
- [x] 01-01-PLAN.md — Go module scaffold and YAML config package
- [x] 01-02-PLAN.md — SQLite persistence layer with schema, reclaim, and cleanup
- [x] 01-03-PLAN.md — Telegram validation, main.go wiring, and systemd unit

### Phase 2: Core Pipeline
**Goal**: A running service that accepts authenticated HTTP notifications, queues them durably, and delivers them to Telegram with correct formatting and automatic retry
**Depends on**: Phase 1
**Requirements**: API-01, API-02, API-03, API-04, TELE-01, TELE-02, TELE-03
**Success Criteria** (what must be TRUE):
  1. `curl -X POST /api/v1/notify` with a valid Bearer token and JSON body returns 202 with a message ID; the same request with a bad token returns 401; a malformed payload returns 400
  2. The submitted message appears in the configured Telegram chat within seconds, formatted with the priority emoji, bold title, and properly escaped MarkdownV2
  3. `GET /api/v1/health` returns 200 JSON when the service is operational
  4. If Telegram is unreachable, the dispatcher retries with exponential backoff; a 429 response causes the dispatcher to wait exactly `retry_after` seconds before the next attempt
  5. After Telegram recovers, all queued messages that were pending during the outage are delivered in order
**Plans:** 4/4 plans complete

Plans:
- [x] 02-01-PLAN.md — Data layer: config extensions, schema migration, DB operations
- [x] 02-02-PLAN.md — HTTP API: chi router, bearer auth, notify and health endpoints
- [x] 02-03-PLAN.md — Telegram formatter and dispatcher with retry logic
- [x] 02-04-PLAN.md — Main.go wiring: seed keys, dispatcher, HTTP server, graceful shutdown

### Phase 3: CLI and Developer Experience
**Goal**: Operators can send notifications, wrap cron jobs, and manage API keys entirely from the command line without curl boilerplate
**Depends on**: Phase 2
**Requirements**: CLI-01, CLI-02, CLI-03
**Success Criteria** (what must be TRUE):
  1. `jaimito send "Backup complete"` delivers a notification to Telegram; `-c cron -p high` flags route it to the correct channel at the correct priority
  2. `jaimito wrap -- /path/to/backup.sh` runs the script silently on success and sends a failure notification with exit code and captured output if the script exits non-zero
  3. `jaimito keys create --name backup-service` creates a new `sk-` prefixed API key and prints it once; `jaimito keys list` shows it; `jaimito keys revoke <id>` removes it without requiring a server restart
**Plans**: TBD

## Progress

**Execution Order:**
Phases execute in numeric order: 1 → 2 → 3

| Phase | Plans Complete | Status | Completed |
|-------|----------------|--------|-----------|
| 1. Foundation | 3/3 | Complete    | 2026-02-21 |
| 2. Core Pipeline | 4/4 | Complete   | 2026-02-21 |
| 3. CLI and Developer Experience | 0/TBD | Not started | - |
