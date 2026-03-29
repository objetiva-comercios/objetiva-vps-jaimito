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

- ✓ Interactive CLI setup wizard (`jaimito setup`) with bubbletea TUI — Validated in Phase 4-7: Setup Wizard
- ✓ Live validation of Telegram bot token and chat IDs against API — Validated in Phase 5: Validacion Telegram
- ✓ Auto-generation of API key and config YAML writing — Validated in Phase 6: Configuracion y Escritura
- ✓ Test notification to prove setup works before finishing — Validated in Phase 7: Verificacion e Integracion
- ✓ install.sh integration (replaces manual config.example.yaml copy) — Validated in Phase 7: Verificacion e Integracion
- ✓ REST API for metrics (GET /api/v1/metrics, GET /api/v1/metrics/{name}/readings, POST /api/v1/metrics) — Validated in Phase 10: REST API y CLI
- ✓ CLI `jaimito status` for metrics overview — Validated in Phase 10: REST API y CLI
- ✓ CLI `jaimito metric` for manual metric ingestion — Validated in Phase 10: REST API y CLI
- ✓ Automatic data retention with periodic purge of old readings — Validated in Phase 12: Cleanup y Polish
- ✓ config.example.yaml documents all v2.0 metrics options — Validated in Phase 12: Cleanup y Polish

## Current Milestone: v2.0 Métricas y Dashboard

**Goal:** Extender jaimito para recolectar métricas del sistema, almacenarlas, y mostrarlas en un dashboard web embedido — monitoreo liviano sin instalar Prometheus/Grafana.

**Target features:**
- Recolector interno de métricas (disco, RAM, CPU, Docker, custom) con ejecución por intervalos
- Definición de métricas en config.yaml con comandos, intervalos, umbrales y tipos
- Métricas predefinidas (disk_root, ram_used, cpu_load, docker_running, uptime_days)
- Dashboard web embedido (go:embed) con tabla de métricas + gráfico expandible
- Alertas automáticas a Telegram cuando una métrica cruza un umbral (warning/critical)
- CLI `jaimito metric` para ingesta manual de métricas
- CLI `jaimito status` para ver métricas actuales
- API REST para métricas (GET /api/v1/metrics, GET /api/v1/metrics/{name}/readings)
- Retención de 7 días con purga automática

### Out of Scope

- Email/SMTP dispatcher — deferred to future milestone
- HTTP generic/cURL dispatcher — deferred to future milestone
- File watcher ingestor — deferred to future milestone
- Systemd watcher — deferred to v2+
- Dashboard web — ~~anti-feature~~ → promoted to v2.0 milestone (métricas + dashboard)
- Message grouping/digest — deferred to future milestone
- Deduplication — deferred to future milestone
- Rate limiting — deferred to future milestone
- Quiet hours — deferred to future milestone
- Query API (list messages, stats) — partially addressed in v2.0 (metrics API)
- WhatsApp/PagerDuty/Matrix dispatchers — deferred to v2+

## Context

- Shipped v1.0 MVP with 2,090 LOC Go across 62 files
- v1.1 Setup Wizard shipped — interactive TUI config wizard with live Telegram validation
- Tech stack: Go 1.25, modernc.org/sqlite (CGO-free), chi v5, cobra, go-telegram/bot, bubbletea v2
- Runs on the same VPS it monitors (single machine deployment)
- Primary pain point solved: cron jobs no longer fail silently (`jaimito wrap`)
- Single binary, zero external dependencies, ~50MB memory footprint
- v2.0 adds: Tailwind CSS, Lucide icons, Alpine.js (~15KB), uPlot (~14KB) — all embedido via go:embed
- Dashboard sin auth (localhost only, acceso via Tailscale)
- Métricas como comandos shell ejecutados por jaimito con intervalos configurables

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

## Evolution

This document evolves at phase transitions and milestone boundaries.

**After each phase transition** (via `/gsd:transition`):
1. Requirements invalidated? → Move to Out of Scope with reason
2. Requirements validated? → Move to Validated with phase reference
3. New requirements emerged? → Add to Active
4. Decisions to log? → Add to Key Decisions
5. "What This Is" still accurate? → Update if drifted

**After each milestone** (via `/gsd:complete-milestone`):
1. Full review of all sections
2. Core Value check — still the right priority?
3. Audit Out of Scope — reasons still valid?
4. Update Context with current state

---
*Last updated: 2026-03-29 — Phase 12 complete: Purga automática de readings + config.example.yaml documentado para v2.0. Milestone v2.0 completo.*
