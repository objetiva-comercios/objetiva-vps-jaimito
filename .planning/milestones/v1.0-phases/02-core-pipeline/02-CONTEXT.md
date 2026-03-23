# Phase 2: Core Pipeline - Context

**Gathered:** 2026-02-21
**Status:** Ready for planning

<domain>
## Phase Boundary

A running service that accepts authenticated HTTP POST notifications, queues them durably in SQLite, and delivers them to the configured Telegram chat with correct formatting and automatic retry. Covers requirements API-01 through API-04 and TELE-01 through TELE-03. CLI key management and CLI send commands are Phase 3.

</domain>

<decisions>
## Implementation Decisions

### Telegram message format
- Traffic light emoji per priority: low = 🟢, normal = 🟡, high = 🔴
- Compact layout: emoji + bold title on one line, body below
- When title is omitted, show emoji + body directly (no auto-generated title)
- Tags rendered as hashtags on a new line after body: #disk #backup
- Channel name is NOT shown in the message — the chat itself is the context
- All text escaped for MarkdownV2 as required by Telegram Bot API

### API payload defaults
- Minimal valid payload: `{"body": "text"}` — title, channel, priority all optional
- Omitted channel defaults to `general`
- Omitted priority defaults to `normal`
- Omitted title means no bold title line in Telegram (just emoji + body)
- Tags and metadata are always optional

### Retry & failure policy
- 5 retry attempts with exponential backoff
- Respect Telegram 429 `retry_after` header exactly
- After 5 exhausted retries, mark message as `failed` silently (logged, no further action)
- Cleanup purges failed messages after 90 days (already implemented in Phase 1)
- Dispatcher polls for new queued messages every 1 second
- Sequential dispatch — one message at a time, preserves order

### First API key bootstrap
- Seed API keys defined in `config.yaml` under a `seed_api_keys` section
- User provides the full `sk-...` key value in config
- Service hashes and inserts seed keys on startup if they don't already exist
- This bridges the gap until Phase 3 adds `jaimito keys create` CLI

### HTTP server configuration
- Listen address configurable via `server.listen` field in config.yaml
- Default listen address: Claude's Discretion (secure default)

### Claude's Discretion
- Message ID format (UUID v7, short ID, or other)
- Default listen address (likely localhost-only for security)
- Exact exponential backoff timing (initial delay, multiplier, jitter)
- Health endpoint response body structure
- MarkdownV2 escaping implementation details

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

*Phase: 02-core-pipeline*
*Context gathered: 2026-02-21*
