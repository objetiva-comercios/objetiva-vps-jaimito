# Phase 1: Foundation - Research

**Researched:** 2026-02-21
**Domain:** Go binary foundation — SQLite WAL persistence, YAML config, systemd service, startup validation
**Confidence:** HIGH

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**Config file design:**
- All credentials inline in `config.yaml` (no env var overrides)
- Channels defined as a flat list with required fields per channel (name, chat_id, priority)
- Example config ships with all 7 named channels pre-defined (general, deploys, errors, cron, system, security, monitoring)
- SQLite database path configurable via `db.path`, default `/var/lib/jaimito/jaimito.db`

**Channel routing model:**
- Multiple channels can share the same Telegram chat_id — channels are logical groupings
- 3 priority levels: low, normal, high (each maps to a different emoji prefix)
- Unknown channel names in API requests are rejected with 400 (strict validation)
- A channel named `general` is required — config validation fails without it

**Service integration:**
- Runs as root (no dedicated system user)
- systemd unit with `Restart=on-failure` — clean exits stay down, crashes restart
- Binary installed at `/usr/local/bin/jaimito`
- Logs to stdout only — journald captures via `journalctl -u jaimito`

**Startup strictness:**
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

### Deferred Ideas (OUT OF SCOPE)

None — discussion stayed within phase scope
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| CONF-01 | Service reads configuration from YAML file at `/etc/jaimito/config.yaml` | gopkg.in/yaml.v3 struct-tag unmarshaling; os.ReadFile + yaml.Unmarshal pattern |
| CONF-02 | Service supports named channels (general, deploys, errors, cron, system, security, monitoring) with target routing | Config struct with Channels []ChannelConfig; validated at startup; `general` required |
| CONF-03 | Service ships with a systemd unit file for Linux deployment | systemd unit file with Type=simple, Restart=on-failure, StandardOutput=journal |
| PERS-01 | Service persists all messages to SQLite database in WAL mode | modernc.org/sqlite v1.36.0 (CGO-free); `_pragma=journal_mode(WAL)` DSN; embed.FS schema via adlio/schema |
| PERS-02 | Service tracks dispatch attempts in a dispatch_log table | dispatch_log schema design; FK to messages; status enum columns |
| PERS-03 | Service reclaims messages stuck in "dispatching" status on startup | UPDATE messages SET status='queued' WHERE status='dispatching' on startup, before serving |
| PERS-04 | Service purges delivered messages after 30 days and failed messages after 90 days | time.Ticker goroutine scheduler; DELETE WHERE created_at < NOW()-interval; context-aware stop |
</phase_requirements>

---

## Summary

This phase establishes the complete runtime foundation for jaimito: a Go binary that reads YAML config, initializes a WAL-mode SQLite database with embedded schema, reclaims stuck messages on startup, schedules periodic cleanup, and runs as a systemd service. All downstream phases (HTTP API, Telegram dispatch, CLI) depend on the correctness established here.

The two most technically nuanced aspects are SQLite connection management and the startup validation sequence. For SQLite, `modernc.org/sqlite` (CGO-free) requires a specific DSN format (`_pragma=journal_mode(WAL)`) and a single-writer pool (`SetMaxOpenConns(1)`) — not the `_journal_mode` parameter names used by `mattn/go-sqlite3`. The startup sequence must follow a strict ordering: config parse → validate token (getMe) → validate chats (getChat) → open DB → apply schema → reclaim stuck messages → start cleanup scheduler → ready.

For config parsing, `gopkg.in/yaml.v3` with struct tags is the correct minimal-dependency choice (CONTEXT.md grants library discretion). For structured logging, Go 1.21+ `log/slog` is the standard library answer — no third-party dependency needed. Schema management with `adlio/schema` + `embed.FS` enables zero-external-tool migrations that live in the binary itself.

**Primary recommendation:** Use `modernc.org/sqlite` v1.36.0 with `_pragma` DSN parameters, `SetMaxOpenConns(1)` single-writer pool, `gopkg.in/yaml.v3` for config, `log/slog` for logging, `adlio/schema` for embedded schema migrations, and `github.com/go-telegram/bot` v1.19.0 for startup token/chat validation only.

---

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| modernc.org/sqlite | v1.36.0 | SQLite driver (CGO-free) | Required for single-binary cross-compile; no C toolchain needed; decided in STATE.md |
| gopkg.in/yaml.v3 | v3 (stable) | YAML config parsing | Battle-tested; struct-tag API; no key lowercasing bugs like Viper; minimal deps |
| log/slog | Go 1.21 stdlib | Structured logging | Standard library since Go 1.21; JSON or text handlers; no dependency |
| github.com/go-telegram/bot | v1.19.0 | Telegram API calls (startup validation only) | Supports getMe/getChat; Bot API 9.4; well-maintained; will be used in Phase 2 |
| adlio/schema | latest | Embedded schema migrations | Zero external tools; embed.FS native; SQLite dialect; minimal dependency |

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| database/sql | stdlib | SQL connection pooling abstraction | Always; wraps modernc.org/sqlite driver |
| embed | stdlib | Embed SQL migration files into binary | Always; embed schema/*.sql files at build time |
| os/signal | stdlib | SIGTERM/SIGINT graceful shutdown | Main goroutine cleanup; signal.NotifyContext pattern |
| context | stdlib | Cancellation propagation to goroutines | Cleanup scheduler and DB operations |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| gopkg.in/yaml.v3 | spf13/viper | Viper forcibly lowercases keys; 313% larger binary; overkill for single-file config |
| gopkg.in/yaml.v3 | knadh/koanf | Koanf is excellent but adds complexity unnecessary for a single static YAML file |
| adlio/schema | pressly/goose | Goose is more feature-rich (down migrations) but adds complexity; adlio is simpler for initial schema |
| adlio/schema | golang-migrate | golang-migrate is heavier; better for complex migration histories |
| log/slog | uber-go/zap | Zap is faster at high throughput; slog is sufficient for this service's log volume; zero dependency |
| modernc.org/sqlite | mattn/go-sqlite3 | mattn requires CGO — breaks single-binary cross-compile requirement |

**Installation:**

```bash
go get modernc.org/sqlite@v1.36.0
go get gopkg.in/yaml.v3
go get github.com/go-telegram/bot@v1.19.0
go get github.com/adlio/schema
```

---

## Architecture Patterns

### Recommended Project Structure

```
jaimito/
├── cmd/
│   └── jaimito/
│       └── main.go          # Entry point: parse flags, load config, wire dependencies, run
├── internal/
│   ├── config/
│   │   ├── config.go        # Config struct, Load(), Validate()
│   │   └── config_test.go
│   ├── db/
│   │   ├── db.go            # Open(), schema apply, reclaim, connection pool setup
│   │   └── schema/
│   │       └── 001_initial.sql  # CREATE TABLE messages, dispatch_log, api_keys
│   ├── cleanup/
│   │   └── scheduler.go     # Start(ctx, db, interval), purge logic
│   └── telegram/
│       └── client.go        # ValidateToken(ctx), ValidateChat(ctx, chatID)
├── systemd/
│   └── jaimito.service      # systemd unit file
├── go.mod
└── go.sum
```

**Key principle:** `internal/` prevents accidental import by Phase 3 CLI — all packages are private to the binary. `cmd/jaimito/main.go` wires everything together. No `pkg/` directory needed; nothing is intended for external reuse.

### Pattern 1: Startup Sequence (Ordered, Fail-Fast)

**What:** Strict ordering of initialization steps — each step must succeed before proceeding. Any failure exits 1 with a clear message.

**When to use:** Always. The CONTEXT.md requires fail-fast on config errors, invalid tokens, and unreachable chats.

**Order:**
1. Parse YAML config → validate struct (channel list, `general` required, priority values)
2. Call Telegram `getMe` → validate bot token
3. For each unique `chat_id` in channels, call `getChat` → validate reachability
4. Open SQLite connection with WAL pragma
5. Apply embedded schema migrations (idempotent)
6. Reclaim stuck `dispatching` → `queued` messages (log count at INFO)
7. Start cleanup scheduler goroutine
8. (Phase 2 adds: start HTTP server)

```go
// Source: pattern derived from go-telegram/bot pkg.go.dev docs
func main() {
    cfg, err := config.Load("/etc/jaimito/config.yaml")
    if err != nil {
        slog.Error("failed to load config", "error", err)
        os.Exit(1)
    }

    tg, err := bot.New(cfg.Telegram.Token)
    if err != nil {
        slog.Error("invalid telegram token", "error", err)
        os.Exit(1)
    }

    ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
    defer stop()

    if err := validateChats(ctx, tg, cfg.Channels); err != nil {
        slog.Error("telegram chat validation failed", "error", err)
        os.Exit(1)
    }

    db, err := db.Open(cfg.Database.Path)
    if err != nil {
        slog.Error("failed to open database", "error", err)
        os.Exit(1)
    }
    defer db.Close()

    reclaimed, err := db.ReclaimStuck(ctx)
    if err != nil {
        slog.Error("startup reclaim failed", "error", err)
        os.Exit(1)
    }
    if reclaimed > 0 {
        slog.Info("reclaimed stuck messages", "count", reclaimed)
    }

    cleanup.Start(ctx, db)

    slog.Info("jaimito started", "channels", len(cfg.Channels))
    <-ctx.Done()
    slog.Info("shutting down")
}
```

### Pattern 2: SQLite Connection Pool Setup (Single Writer)

**What:** Configure `database/sql` pool with `SetMaxOpenConns(1)` to serialize all writes — SQLite only supports one writer at a time.

**When to use:** Always with SQLite. Without this, concurrent writes produce `SQLITE_BUSY` errors even with WAL mode.

```go
// Source: pkg.go.dev/modernc.org/sqlite (verified DSN format)
func Open(path string) (*sql.DB, error) {
    dsn := "file:" + path +
        "?_pragma=journal_mode(WAL)" +
        "&_pragma=busy_timeout(5000)" +
        "&_pragma=foreign_keys(1)" +
        "&_pragma=synchronous(NORMAL)"

    db, err := sql.Open("sqlite", dsn)
    if err != nil {
        return nil, fmt.Errorf("open sqlite: %w", err)
    }

    // CRITICAL: SQLite supports one writer — serialize all operations
    db.SetMaxOpenConns(1)
    db.SetMaxIdleConns(1)
    db.SetConnMaxLifetime(0) // connections live for app lifetime

    return db, nil
}
```

**CRITICAL DSN note:** `modernc.org/sqlite` uses `_pragma=journal_mode(WAL)` format (parentheses, not equals). The `mattn/go-sqlite3` format `_journal_mode=WAL` does NOT work with modernc.

### Pattern 3: Embedded Schema Migration

**What:** SQL schema files embedded into the binary via `//go:embed`. Applied at startup using `adlio/schema`. Idempotent — safe to run on every startup.

**When to use:** Always. Zero manual setup required per the CONTEXT.md decision.

```go
// Source: github.com/adlio/schema README, verified
//go:embed schema/*.sql
var schemaFS embed.FS

func applySchema(db *sql.DB) error {
    migrations, err := schema.FSMigrations(schemaFS, "schema/*.sql")
    if err != nil {
        return fmt.Errorf("load migrations: %w", err)
    }
    migrator := schema.NewMigrator(schema.WithDialect(schema.SQLite))
    return migrator.Apply(db, migrations)
}
```

### Pattern 4: Cleanup Scheduler

**What:** Goroutine with `time.Ticker` that runs periodic purge SQL. Uses context cancellation for clean shutdown.

**When to use:** Always. Runs on a 24-hour interval (or configurable). Stopped via ctx.Done().

```go
// Source: standard Go pattern verified against multiple 2025 sources
func Start(ctx context.Context, db *sql.DB) {
    ticker := time.NewTicker(24 * time.Hour)
    go func() {
        defer ticker.Stop()
        for {
            select {
            case <-ticker.C:
                if err := purgeOldMessages(ctx, db); err != nil {
                    slog.Error("cleanup failed", "error", err)
                }
            case <-ctx.Done():
                return
            }
        }
    }()
}
```

### Pattern 5: YAML Config Parsing

**What:** `os.ReadFile` + `yaml.Unmarshal` into a typed struct. Validation happens on the returned struct before any further initialization.

```go
// Source: gopkg.in/yaml.v3 pkg.go.dev docs
type Config struct {
    Telegram TelegramConfig  `yaml:"telegram"`
    Database DatabaseConfig  `yaml:"db"`
    Channels []ChannelConfig `yaml:"channels"`
}

type TelegramConfig struct {
    Token string `yaml:"token"`
}

type DatabaseConfig struct {
    Path string `yaml:"path"`
}

type ChannelConfig struct {
    Name   string `yaml:"name"`
    ChatID int64  `yaml:"chat_id"`
    // Priority stored as string, validated against known values
    Priority string `yaml:"priority"`
}

func Load(path string) (*Config, error) {
    data, err := os.ReadFile(path)
    if err != nil {
        return nil, fmt.Errorf("read config %s: %w", path, err)
    }
    var cfg Config
    if err := yaml.Unmarshal(data, &cfg); err != nil {
        return nil, fmt.Errorf("parse config: %w", err)
    }
    if err := cfg.Validate(); err != nil {
        return nil, err
    }
    return &cfg, nil
}
```

### Pattern 6: Startup Reclaim (Stuck Messages)

**What:** On startup, any message stuck in `dispatching` status is reset to `queued`. This handles crash recovery — the dispatcher may have crashed mid-flight.

```go
// Source: derived from PERS-03 requirement + standard SQL pattern
func (db *DB) ReclaimStuck(ctx context.Context) (int64, error) {
    result, err := db.ExecContext(ctx,
        `UPDATE messages SET status = 'queued', updated_at = CURRENT_TIMESTAMP
         WHERE status = 'dispatching'`,
    )
    if err != nil {
        return 0, fmt.Errorf("reclaim stuck: %w", err)
    }
    return result.RowsAffected()
}
```

### Anti-Patterns to Avoid

- **Using `_journal_mode=WAL` DSN parameter:** This is `mattn/go-sqlite3` syntax. With `modernc.org/sqlite`, use `_pragma=journal_mode(WAL)` (verified against pkg.go.dev docs).
- **SetMaxOpenConns > 1 with SQLite:** Causes `SQLITE_BUSY` errors under concurrent writes. Always use `SetMaxOpenConns(1)`.
- **Forgetting to close `sql.Rows`:** In WAL mode, unclosed result sets block the WAL checkpoint, causing unbounded disk growth (documented real case: 4.1KB → 42MB WAL file).
- **Calling `os.Exit(1)` after deferred cleanup:** Use `log.Fatal` only before defers are set up, or use the shutdown pattern above.
- **Skipping busy_timeout pragma:** Without it, concurrent readers will immediately fail instead of waiting, producing spurious errors.
- **Validating chats before token:** `getChat` will fail with auth error if token is wrong. Always `getMe` first.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| YAML parsing | Custom parser / string splitting | `gopkg.in/yaml.v3` | YAML edge cases (multiline, anchors, types) are non-trivial |
| Schema versioning | Track applied migrations manually | `adlio/schema` | Already handles schema_version table, idempotency, ordering |
| SQLite connection DSN | Custom connection hook | `_pragma=` DSN params | DSN approach is cleaner; hook needed only for dynamic config |
| Telegram token validation | Parse JWT manually | `bot.New()` + `GetMe()` | `go-telegram/bot` does auth check on `New()` with 5s timeout by default |
| Graceful shutdown | `os.Signal` channel manually | `signal.NotifyContext` | Cleaner API; handles edge cases; Go stdlib since 1.16 |
| Periodic cleanup | Custom timer/cron | `time.Ticker` + goroutine | Standard Go pattern; no dependency needed |

**Key insight:** The hardest part of this phase is not any single library — it's the ordering and interaction between startup steps. Get the sequence right, and each component is independently straightforward.

---

## Common Pitfalls

### Pitfall 1: Wrong DSN Parameter Names for modernc.org/sqlite

**What goes wrong:** Using `mattn/go-sqlite3`-style DSN like `?_journal_mode=WAL&_busy_timeout=5000` with `modernc.org/sqlite` — silently ignored, WAL mode not enabled.

**Why it happens:** Developers copy examples from the much more common `mattn/go-sqlite3` docs.

**How to avoid:** Use `modernc.org/sqlite` DSN format: `?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)`. Parameters use `_pragma=NAME(VALUE)` syntax (parentheses, not `=`).

**Warning signs:** `PRAGMA journal_mode` returns `delete` after opening DB; no WAL file (`.db-wal`) created.

### Pitfall 2: Unclosed sql.Rows Blocking WAL Checkpoint

**What goes wrong:** Any call to `db.QueryContext()` that doesn't close the returned `*sql.Rows` keeps a read transaction open. In WAL mode, this prevents the checkpoint from running, causing the WAL file to grow indefinitely.

**Why it happens:** Forgetting `defer rows.Close()` or early returns before close in error paths.

**How to avoid:** Always use `defer rows.Close()` immediately after checking `err` from `QueryContext`. Use `sqlx` or helper functions that enforce closing, or use `QueryRowContext` when only one row is expected.

**Warning signs:** WAL file (`.db-wal`) grows to 10-100x the main DB file size; disk space alerts.

### Pitfall 3: SetMaxOpenConns Missing Causes SQLITE_BUSY

**What goes wrong:** With multiple goroutines (cleanup scheduler + future HTTP handlers), concurrent writes produce `database is locked (SQLITE_BUSY)` errors even with WAL mode and `busy_timeout` set.

**Why it happens:** `database/sql` defaults to unlimited open connections; multiple concurrent write transactions conflict at the SQLite level.

**How to avoid:** Set `db.SetMaxOpenConns(1)` immediately after `sql.Open()`. This serializes all DB operations through a single connection.

**Warning signs:** Intermittent `SQLITE_BUSY` errors; errors become more frequent under load.

### Pitfall 4: Startup Validation Order Matters

**What goes wrong:** Calling `getChat` before verifying the token works → confusing "unauthorized" error on what appears to be a chat validation step.

**Why it happens:** Trying to validate all config in one pass without considering dependency order.

**How to avoid:** Strict order: token validation (`getMe`) always before chat validation (`getChat`). If `getMe` fails, exit before attempting `getChat`.

**Warning signs:** `getChat` returns 401 Unauthorized even though chat_id looks correct.

### Pitfall 5: Schema Applied After Reclaim

**What goes wrong:** Trying to run `UPDATE messages SET status='queued' WHERE status='dispatching'` before the schema exists → SQL error on first-ever startup.

**Why it happens:** Reclaim runs before schema migration.

**How to avoid:** Apply schema migrations (step 5) strictly before reclaim (step 6). `adlio/schema` is idempotent — safe on both fresh installs and upgrades.

**Warning signs:** Error on first startup: `no such table: messages`.

### Pitfall 6: slog Default Handler Writes JSON to stdout Not stderr

**What goes wrong:** Default `slog.Default()` handler writes text to stdout; journald captures both but log parsers may be confused.

**Why it happens:** slog default handler is `TextHandler` to `os.Stderr` — actually correct. But if you create a `TextHandler` pointing to `os.Stdout`, levels may mix with application output.

**How to avoid:** Use `slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))` explicitly. Since systemd captures stdout only (per CONTEXT.md decision), write all logs to stdout.

---

## Code Examples

Verified patterns from official sources:

### SQLite Open with WAL Mode (modernc.org/sqlite)

```go
// Source: pkg.go.dev/modernc.org/sqlite - verified DSN format 2026-02-21
import (
    "database/sql"
    _ "modernc.org/sqlite"
)

func Open(path string) (*sql.DB, error) {
    // IMPORTANT: modernc.org/sqlite uses _pragma=NAME(VALUE), NOT _journal_mode=WAL
    dsn := "file:" + path +
        "?_pragma=journal_mode(WAL)" +
        "&_pragma=busy_timeout(5000)" +
        "&_pragma=foreign_keys(1)" +
        "&_pragma=synchronous(NORMAL)"

    db, err := sql.Open("sqlite", dsn)
    if err != nil {
        return nil, err
    }

    // Single writer: SQLite only allows one concurrent write transaction
    db.SetMaxOpenConns(1)
    db.SetMaxIdleConns(1)

    return db, nil
}
```

### Telegram Startup Validation

```go
// Source: pkg.go.dev/github.com/go-telegram/bot v1.19.0 - verified 2026-02-21
import (
    "context"
    "github.com/go-telegram/bot"
)

func ValidateToken(ctx context.Context, token string) (*bot.Bot, error) {
    // bot.New() automatically calls getMe with 5s timeout
    b, err := bot.New(token)
    if err != nil {
        return nil, fmt.Errorf("invalid telegram token: %w", err)
    }
    return b, nil
}

func ValidateChats(ctx context.Context, b *bot.Bot, channels []ChannelConfig) error {
    seen := map[int64]bool{}
    for _, ch := range channels {
        if seen[ch.ChatID] {
            continue // deduplicate - multiple channels can share chat_id
        }
        seen[ch.ChatID] = true
        _, err := b.GetChat(ctx, &bot.GetChatParams{ChatID: ch.ChatID})
        if err != nil {
            return fmt.Errorf("channel %q: chat_id %d unreachable: %w", ch.Name, ch.ChatID, err)
        }
    }
    return nil
}
```

### Embedded Schema with adlio/schema

```go
// Source: github.com/adlio/schema README - verified 2026-02-21
import (
    "embed"
    "github.com/adlio/schema"
)

//go:embed schema/*.sql
var schemaFS embed.FS

func ApplySchema(db *sql.DB) error {
    migrations, err := schema.FSMigrations(schemaFS, "schema/*.sql")
    if err != nil {
        return fmt.Errorf("load schema: %w", err)
    }
    migrator := schema.NewMigrator(schema.WithDialect(schema.SQLite))
    if err := migrator.Apply(db, migrations); err != nil {
        return fmt.Errorf("apply schema: %w", err)
    }
    return nil
}
```

### Graceful Shutdown with signal.NotifyContext

```go
// Source: standard library os/signal, Go 1.16+ pattern - verified 2026-02-21
import (
    "context"
    "os/signal"
    "syscall"
)

func main() {
    ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
    defer stop()

    // ... start goroutines that accept ctx ...

    <-ctx.Done()
    slog.Info("received shutdown signal")
    // deferred db.Close() runs here
}
```

### Proposed SQLite Schema (001_initial.sql)

```sql
-- Source: derived from REQUIREMENTS.md PERS-01, PERS-02 + standard patterns
CREATE TABLE IF NOT EXISTS messages (
    id          TEXT PRIMARY KEY,         -- UUID v4
    channel     TEXT NOT NULL,
    priority    TEXT NOT NULL DEFAULT 'normal', -- low|normal|high
    title       TEXT NOT NULL,
    body        TEXT NOT NULL,
    tags        TEXT,                     -- JSON array
    metadata    TEXT,                     -- JSON object
    status      TEXT NOT NULL DEFAULT 'queued', -- queued|dispatching|delivered|failed
    created_at  TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at  TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_messages_status ON messages(status);
CREATE INDEX IF NOT EXISTS idx_messages_created_at ON messages(created_at);

CREATE TABLE IF NOT EXISTS dispatch_log (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    message_id  TEXT NOT NULL REFERENCES messages(id),
    attempt     INTEGER NOT NULL DEFAULT 1,
    status      TEXT NOT NULL,           -- success|failed|retrying
    error       TEXT,                    -- NULL on success
    attempted_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_dispatch_log_message_id ON dispatch_log(message_id);

CREATE TABLE IF NOT EXISTS api_keys (
    id          TEXT PRIMARY KEY,        -- UUID v4
    key_hash    TEXT NOT NULL UNIQUE,    -- SHA-256 of sk-... token
    name        TEXT NOT NULL,
    created_at  TEXT NOT NULL DEFAULT (datetime('now')),
    last_used_at TEXT,
    revoked     INTEGER NOT NULL DEFAULT 0  -- boolean
);
```

### Cleanup Purge SQL

```sql
-- PERS-04: delivered > 30 days, failed > 90 days
DELETE FROM messages
WHERE (status = 'delivered' AND created_at < datetime('now', '-30 days'))
   OR (status = 'failed' AND created_at < datetime('now', '-90 days'));
```

### systemd Unit File (jaimito.service)

```ini
[Unit]
Description=jaimito VPS notification hub
After=network.target
ConditionPathExists=/usr/local/bin/jaimito

[Service]
Type=simple
ExecStart=/usr/local/bin/jaimito
Restart=on-failure
RestartSec=5s

# Logs to stdout; journald captures automatically
StandardOutput=journal
StandardError=journal
SyslogIdentifier=jaimito

# CONTEXT.md: runs as root, no dedicated user

[Install]
WantedBy=multi-user.target
```

### slog Setup (stdout to journald)

```go
// Source: log/slog stdlib Go 1.21 - verified pattern
import "log/slog"

func initLogging() {
    handler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
        Level: slog.LevelInfo,
    })
    slog.SetDefault(slog.New(handler))
}
```

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `mattn/go-sqlite3` (CGO) | `modernc.org/sqlite` (pure Go) | ~2020, stable by 2022 | Single binary without C toolchain; required for this project |
| Custom logging packages (logrus, zap) | `log/slog` stdlib | Go 1.21 (2023) | No dependency; structured JSON/text; sufficient for most services |
| External migration tools (flyway, liquibase) | `embed.FS` + `adlio/schema` or `pressly/goose` | Go 1.16+ (2021) | Migrations embedded in binary; zero external tooling required |
| `spf13/viper` for config | `gopkg.in/yaml.v3` or `knadh/koanf` | 2022-2023 | Viper lowercases keys (bug), larger binary; direct yaml.v3 is simpler |
| Manual signal handling | `signal.NotifyContext` | Go 1.16 (2021) | Cleaner API; context-aware cancellation propagation |

**Deprecated/outdated:**
- `mattn/go-sqlite3`: Requires CGO — incompatible with CGO-free single-binary builds required by this project
- `spf13/viper`: Forcibly lowercases YAML keys (spec violation); known bugs; use direct yaml.v3 for single-file YAML
- `log/logrus`: Replaced by stdlib slog for new projects; no functional advantage for this use case

---

## Open Questions

1. **modernc.org/sqlite version pinning**
   - What we know: STATE.md references v1.46.1 but pkg.go.dev shows v1.36.0 as the latest stable version. The DeepWiki page referenced SQLite 3.51.2. Version numbering appears inconsistent across sources.
   - What's unclear: The exact latest stable version. STATE.md says v1.46.1 (published Feb 18, 2026 per pkg.go.dev), which is newer than v1.36.0 (which appears to be an older release). pkg.go.dev fetch showed v1.46.1.
   - Recommendation: Run `go get modernc.org/sqlite@latest` and let Go resolve the current latest. Pin whatever version resolves in go.mod. The DSN parameter format (`_pragma=NAME(VALUE)`) is stable regardless of minor version.

2. **Cleanup scheduler interval**
   - What we know: CONTEXT.md grants discretion on "cleanup scheduler implementation (interval, goroutine strategy)".
   - What's unclear: Whether 24-hour interval is appropriate, or if a startup-then-daily pattern would be better (run once on startup + every 24h after).
   - Recommendation: Run cleanup once at startup (after reclaim, before serving), then every 24 hours via `time.Ticker`. This ensures cleanup happens even if the service rarely restarts.

3. **getChat ChatID type**
   - What we know: `bot.GetChatParams{ChatID: ...}` — the type of ChatID in the params struct may be `int64`, `string`, or `interface{}` depending on the library version.
   - What's unclear: Exact Go type for ChatID in `bot.GetChatParams` at v1.19.0.
   - Recommendation: Check `pkg.go.dev/github.com/go-telegram/bot` at implementation time. Telegram chat IDs for private groups are negative int64; channels use `@username` strings. Support both via `interface{}` or the library's own type.

4. **`api_keys` table in Phase 1**
   - What we know: PERS-01 requires the full schema with `api_keys` table. But API key operations (CLI-03) are Phase 3.
   - What's unclear: Should Phase 1 create the table but leave it empty? Or skip until Phase 3?
   - Recommendation: Create the `api_keys` table in the Phase 1 schema. Creating it later would require a new migration. Zero risk to create it empty now.

---

## Sources

### Primary (HIGH confidence)

- `pkg.go.dev/modernc.org/sqlite` — DSN parameter format `_pragma=NAME(VALUE)`, supported parameters, v1.46.1 version confirmed
- `pkg.go.dev/github.com/go-telegram/bot` — v1.19.0, GetMe/GetChat APIs, WithSkipGetMe option, Bot API 9.4 support
- `pkg.go.dev/gopkg.in/yaml.v3` — struct tags, Unmarshal API, stable API guarantee
- `pkg.go.dev/log/slog` — Go 1.21 stdlib, TextHandler/JSONHandler, LevelInfo default
- `github.com/adlio/schema` — FSMigrations API, WithDialect(schema.SQLite), embed.FS support
- `pkg.go.dev/database/sql` — SetMaxOpenConns, SetMaxIdleConns patterns

### Secondary (MEDIUM confidence)

- `turso.tech/blog/something-you-probably-want-to-know-about-if-youre-using-sqlite-in-golang` — Verified WAL checkpoint blocking pitfall; rows.Close() requirement; real disk growth numbers (4.1KB vs 42MB WAL)
- `deepwiki.com/modernc-org/sqlite` — DSN format confirmation; pragma execution order (busy_timeout first)
- `theitsolutions.io/blog/modernc.org-sqlite-with-go` — Connection hook pattern; WAL setup; separate read/write pool pattern
- Multiple 2025 sources on `signal.NotifyContext` for graceful shutdown pattern

### Tertiary (LOW confidence)

- WebSearch results on "single writer SQLite pattern" — Consistent across 5+ sources but not verified against single canonical source. Claim: `SetMaxOpenConns(1)` is the standard pattern. Very likely correct given SQLite's documented single-writer model, but no official Go+SQLite guide explicitly endorses this exact parameter.

---

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — All libraries verified against pkg.go.dev current docs (Feb 2026)
- DSN parameters: HIGH — Verified directly against `pkg.go.dev/modernc.org/sqlite` Driver.Open() docs
- Architecture: HIGH — Standard Go patterns, verified against official docs
- Schema design: MEDIUM — Derived from REQUIREMENTS.md; not validated against runtime behavior
- Pitfalls: HIGH for WAL/rows.Close (verified with real numbers); HIGH for DSN format (verified); MEDIUM for SetMaxOpenConns (consistent but no single canonical source)

**Research date:** 2026-02-21
**Valid until:** 2026-04-21 (stable ecosystem — Go stdlib, modernc.org/sqlite, yaml.v3 change slowly)
