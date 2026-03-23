# Phase 3: CLI and Developer Experience - Research

**Researched:** 2026-02-23
**Confidence:** HIGH

## Key Findings

### cobra v1.10.2 (CLI framework)

Already in STACK.md as recommended library. Module path: `github.com/spf13/cobra`.

**Pattern for default-to-serve:**
```go
var rootCmd = &cobra.Command{
    Use:  "jaimito",
    RunE: runServe, // bare `jaimito` = server mode
}
```
Subcommands (send, wrap, keys) added via `rootCmd.AddCommand()`. Persistent flags on root are inherited by all subcommands.

**`--` separator for wrap:** cobra natively handles `--` as end-of-flags. After `--`, all remaining tokens become positional args. `jaimito wrap -c cron -- backup.sh -v` gives channel="cron", args=["backup.sh", "-v"]. No special handling needed.

### HTTP client for send/wrap

Standard `net/http` client with JSON encoding. Mirrors the existing `api.NotifyRequest` struct. Bearer token in `Authorization` header. Endpoint: `POST {server}/api/v1/notify`.

Server URL construction: config `server.listen` is `host:port` format (e.g., "127.0.0.1:8080"). CLI prepends `http://` to construct full URL.

### API key generation

Use `crypto/rand` for secure random bytes. Format: `sk-` + `hex.EncodeToString(32 random bytes)` = 67 chars total. Same format as seed keys in config.

SHA-256 hash via existing `db.HashToken()` — single source of truth.

### Subprocess execution for wrap

`exec.Command(args[0], args[1:]...)` with `cmd.CombinedOutput()` captures both stdout and stderr. Exit code extracted via `(*exec.ExitError).ExitCode()`.

Truncation: limit captured output to ~3500 bytes in notification body to fit within Telegram's 4096 UTF-8 character message limit after formatting overhead.

### Existing integration points

| CLI Command | Integrates With | How |
|-------------|----------------|-----|
| send | HTTP API (POST /api/v1/notify) | HTTP client with Bearer auth |
| wrap | HTTP API (POST /api/v1/notify) | Same HTTP client, sends on failure |
| keys create | internal/db (api_keys table) | Direct SQLite insert via new CreateKey function |
| keys list | internal/db (api_keys table) | Direct SQLite query via new ListKeys function |
| keys revoke | internal/db (api_keys table) | Direct SQLite update via new RevokeKey function |

### New DB functions needed

| Function | SQL | Returns |
|----------|-----|---------|
| `CreateKey(ctx, db, name)` | INSERT INTO api_keys (id, key_hash, name) | raw key string |
| `ListKeys(ctx, db)` | SELECT id, name, created_at, last_used_at FROM api_keys WHERE revoked=0 | []ApiKey |
| `RevokeKey(ctx, db, id)` | UPDATE api_keys SET revoked=1 WHERE id=? AND revoked=0 | error (checks RowsAffected) |

## No Unknowns

All libraries and patterns are well-established. No API exploration or prototype needed.

---

*Phase: 03-cli-developer-experience*
*Researched: 2026-02-23*
