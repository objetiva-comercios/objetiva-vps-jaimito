# Phase 1: Foundation - Context

**Gathered:** 2026-02-21
**Status:** Ready for planning

<domain>
## Phase Boundary

A deployable Go binary that reads YAML config from `/etc/jaimito/config.yaml`, initializes a WAL-mode SQLite database with the correct schema (`messages`, `dispatch_log`, `api_keys`), reclaims stuck messages on startup, schedules old message purging, and installs as a systemd service. No HTTP API, no Telegram dispatch, no CLI — those are later phases.

</domain>

<decisions>
## Implementation Decisions

### Config file design
- All credentials inline in `config.yaml` (no env var overrides)
- Channels defined as a flat list with required fields per channel (name, chat_id, priority)
- Example config ships with all 7 named channels pre-defined (general, deploys, errors, cron, system, security, monitoring)
- SQLite database path configurable via `db.path`, default `/var/lib/jaimito/jaimito.db`

### Channel routing model
- Multiple channels can share the same Telegram chat_id — channels are logical groupings
- 3 priority levels: low, normal, high (each maps to a different emoji prefix)
- Unknown channel names in API requests are rejected with 400 (strict validation)
- A channel named `general` is required — config validation fails without it

### Service integration
- Runs as root (no dedicated system user)
- systemd unit with `Restart=on-failure` — clean exits stay down, crashes restart
- Binary installed at `/usr/local/bin/jaimito`
- Logs to stdout only — journald captures via `journalctl -u jaimito`

### Startup strictness
- Missing or malformed config: fail fast, exit 1 with clear error message
- Validates Telegram bot token at startup via `getMe` API call — fails if invalid
- Validates every configured chat_id at startup via `getChat` — fails if any unreachable
- Auto-creates SQLite DB file and schema if not present (zero manual setup)

### Claude's Discretion
- Go module structure and package layout
- Exact SQLite schema DDL (columns, indexes, types)
- Log format and verbosity levels
- Config parsing library choice
- Error message wording
- Cleanup scheduler implementation (interval, goroutine strategy)

</decisions>

<specifics>
## Specific Ideas

No specific requirements — open to standard approaches

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 01-foundation*
*Context gathered: 2026-02-21*
