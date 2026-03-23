# Phase 3: CLI and Developer Experience - Context

**Gathered:** 2026-02-23
**Status:** Ready for planning

<domain>
## Phase Boundary

CLI commands that complete the user-facing surface: `jaimito send` for ad-hoc notifications, `jaimito wrap` for cron job monitoring, and `jaimito keys` for API key management. The server daemon (Phase 1+2) continues to run as before — CLI commands are clients that talk to the running server via HTTP API, except `keys` which operates on the database directly. No changes to the existing HTTP API, dispatcher, or Telegram formatting.

</domain>

<decisions>
## Implementation Decisions

### CLI framework
- Use cobra v1.10.2 for subcommand routing, flag parsing, and auto-generated help
- Root command with no subcommand starts the server (backward compatible with Phase 1+2)
- Persistent --config flag on root command (default: /etc/jaimito/config.yaml)

### Command structure
- `jaimito` (no args) → starts server daemon (current behavior, backward compatible)
- `jaimito send "body"` → sends notification via HTTP API
- `jaimito wrap -- cmd args...` → runs command, notifies on failure
- `jaimito keys create --name name` → creates new API key
- `jaimito keys list` → lists all active keys
- `jaimito keys revoke <id>` → revokes a key

### Authentication for send/wrap
- API key from JAIMITO_API_KEY environment variable (primary)
- --key flag as override
- No default — must be explicitly provided; error if neither set
- Server address resolved in order: --server flag → JAIMITO_SERVER env → config server.listen → default 127.0.0.1:8080

### Keys management
- Direct database access (not via HTTP API) — works even when server is stopped
- Reads database path from config file
- Key format: `sk-` + 32 random hex bytes (64 chars, 256 bits entropy)
- `create` prints raw key to stdout once (never stored in plaintext, only hash in DB)
- `revoke` sets revoked=1 (takes effect immediately — BearerAuth checks DB on every request)

### Claude's Discretion
- Exact cobra command wiring (Use, Short, Long, Example strings)
- Output format for keys list (table vs plain text)
- stderr vs stdout for error messages
- Exact truncation limit for wrap command output in notification body
- HTTP client timeout value

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

*Phase: 03-cli-developer-experience*
*Context gathered: 2026-02-23*
