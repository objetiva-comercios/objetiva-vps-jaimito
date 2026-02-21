# Requirements: jaimito

**Defined:** 2026-02-21
**Core Value:** Every event that happens on the VPS gets reliably captured and delivered to Telegram — no notification is ever lost silently.

## v1 Requirements

Requirements for initial release (v0.1 MVP). Each maps to roadmap phases.

### HTTP API

- [ ] **API-01**: Service accepts HTTP POST to `/api/v1/notify` with JSON payload (title, body, channel, priority, tags, metadata)
- [ ] **API-02**: Service authenticates requests via Bearer token with `sk-` prefix API keys
- [ ] **API-03**: Service returns 202 with message ID on success, 400 on invalid payload, 401 on bad auth
- [ ] **API-04**: Service exposes `GET /api/v1/health` returning 200 when operational

### Persistence

- [ ] **PERS-01**: Service persists all messages to SQLite database in WAL mode
- [ ] **PERS-02**: Service tracks dispatch attempts in a dispatch_log table
- [ ] **PERS-03**: Service reclaims messages stuck in "dispatching" status on startup
- [ ] **PERS-04**: Service purges delivered messages after 30 days and failed messages after 90 days

### Telegram

- [ ] **TELE-01**: Service delivers messages to configured Telegram chat via Bot API
- [ ] **TELE-02**: Service formats messages with MarkdownV2 (priority emoji, bold title, code blocks, proper escaping)
- [ ] **TELE-03**: Service retries failed deliveries with exponential backoff, respecting Telegram 429 `retry_after`

### CLI

- [ ] **CLI-01**: `jaimito send "message"` sends a notification with optional -c channel, -p priority, --stdin flags
- [ ] **CLI-02**: `jaimito wrap -c channel -- /path/to/cmd` runs a command and notifies on failure with exit code and output
- [ ] **CLI-03**: `jaimito keys create/list/revoke` manages API keys without server restart

### Configuration

- [ ] **CONF-01**: Service reads configuration from YAML file at `/etc/jaimito/config.yaml`
- [ ] **CONF-02**: Service supports named channels (general, deploys, errors, cron, system, security, monitoring) with target routing
- [ ] **CONF-03**: Service ships with a systemd unit file for Linux deployment

## v2 Requirements

Deferred to future release. Tracked but not in current roadmap.

### Rate Control

- **RATE-01**: Service enforces rate limits per channel (configurable messages/minute)
- **RATE-02**: Service deduplicates messages with same dedupe_key within configurable window
- **RATE-03**: Service groups burst messages into digest within configurable group_window
- **RATE-04**: Service supports quiet hours with hold/downgrade/deliver actions

### Additional Dispatchers

- **DISP-01**: Service sends notifications via Email/SMTP
- **DISP-02**: Service sends notifications via configurable HTTP/cURL templates (Discord, Slack, ntfy, etc.)

### Query API

- **QAPI-01**: `GET /api/v1/messages` lists messages with filters (channel, priority, status, date range)
- **QAPI-02**: `GET /api/v1/messages/:id` shows message detail with dispatch log
- **QAPI-03**: `GET /api/v1/stats` shows delivery statistics per channel
- **QAPI-04**: `DELETE /api/v1/messages/:id` cancels a queued message

### Additional Ingestors

- **INGS-01**: File watcher monitors log files and fires notifications on pattern match
- **INGS-02**: `jaimito list` CLI shows recent messages and delivery status

## Out of Scope

Explicitly excluded. Documented to prevent scope creep.

| Feature | Reason |
|---------|--------|
| Web dashboard / notification UI | Anti-feature: 10x codebase, Telegram IS the history |
| Multi-user / team support | Single-user/single-VPS model; API keys per service suffice |
| Plugin system for custom dispatchers | Config-driven approach + HTTP generic dispatcher in v0.3 |
| Systemd watcher (D-Bus) | High complexity, defer to v2+ |
| Message templates / templating engine | Senders should format their own messages |
| Attachment storage (files/images) | Separate subsystem; use Telegram Bot API directly |
| SMTP email ingestor | Different product category; use mailrise |
| WebSocket real-time stream | Telegram IS the real-time stream |
| WhatsApp/PagerDuty/Matrix | Maintenance burden; recommend Apprise bridge |
| Dead man's switch / heartbeat | healthchecks.io exists; build only if users request it |
| Priority-based behavioral differences | Cosmetic in v0.1; behavioral in v0.2+ |
| 4096-char message truncation | Nice-to-have, can add incrementally |

## Traceability

Which phases cover which requirements. Updated during roadmap creation.

| Requirement | Phase | Status |
|-------------|-------|--------|
| API-01 | — | Pending |
| API-02 | — | Pending |
| API-03 | — | Pending |
| API-04 | — | Pending |
| PERS-01 | — | Pending |
| PERS-02 | — | Pending |
| PERS-03 | — | Pending |
| PERS-04 | — | Pending |
| TELE-01 | — | Pending |
| TELE-02 | — | Pending |
| TELE-03 | — | Pending |
| CLI-01 | — | Pending |
| CLI-02 | — | Pending |
| CLI-03 | — | Pending |
| CONF-01 | — | Pending |
| CONF-02 | — | Pending |
| CONF-03 | — | Pending |

**Coverage:**
- v1 requirements: 17 total
- Mapped to phases: 0
- Unmapped: 17 ⚠️

---
*Requirements defined: 2026-02-21*
*Last updated: 2026-02-21 after initial definition*
