# Architecture Research

**Domain:** Go metrics collector + embedded web dashboard added to existing notification hub
**Researched:** 2026-03-26
**Confidence:** HIGH (based on direct codebase inspection)

## Standard Architecture

### System Overview — v2.0 with Metrics Layer

```
┌─────────────────────────────────────────────────────────────────────┐
│                         CLI Layer (cmd/jaimito/)                     │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐  │
│  │  serve   │ │  send    │ │  wrap    │ │  keys    │ │  metric  │  │
│  │(modified)│ │(no chg)  │ │(no chg)  │ │(no chg)  │ │  (NEW)   │  │
│  └────┬─────┘ └──────────┘ └──────────┘ └──────────┘ └─────┬────┘  │
│       │ startup sequence                      status (NEW) ─┘       │
└───────┼─────────────────────────────────────────────────────────────┘
        │
┌───────▼─────────────────────────────────────────────────────────────┐
│                      Service Layer (internal/)                        │
│                                                                       │
│  ┌──────────────┐  ┌────────────────┐  ┌───────────────────────────┐ │
│  │  dispatcher  │  │    metrics/    │  │        api/               │ │
│  │  (no change) │  │  collector     │  │  (modified: + dashboard   │ │
│  │  polls 1s    │  │  (NEW)         │  │   routes + metrics API)   │ │
│  │  Telegram    │  │  runs shell    │  │                           │ │
│  │  delivery    │  │  cmds on timer │  │  GET  /                   │ │
│  └──────┬───────┘  │  evaluates     │  │  GET  /api/v1/health      │ │
│         │          │  thresholds    │  │  POST /api/v1/notify      │ │
│         │          └───────┬────────┘  │  GET  /api/v1/metrics     │ │
│         │                  │           │  GET  /api/v1/metrics/    │ │
│  ┌──────┴───────┐          │           │       {name}/readings     │ │
│  │   telegram/  │          │           │  POST /api/v1/metrics/    │ │
│  │   (no chg)   │          │           │       ingest (NEW)        │ │
│  └──────────────┘          │           └───────────────────────────┘ │
│                             │ alert path: db.EnqueueMessage()         │
│  ┌─────────────────────────▼──────────────────────────────────────┐  │
│  │               db/  (modified: + metrics tables)                 │  │
│  │  messages table  dispatch_log  api_keys  metrics  metric_reads  │  │
│  └────────────────────────────────────────────────────────────────┘  │
│                                                                       │
│  ┌──────────────────────────────────────────────────────────────┐    │
│  │              cleanup/ (modified: + metric_reads purge)       │    │
│  └──────────────────────────────────────────────────────────────┘    │
│                                                                       │
│  ┌──────────────────────────────────────────────────────────────┐    │
│  │              config/ (modified: + MetricDef struct)          │    │
│  └──────────────────────────────────────────────────────────────┘    │
│                                                                       │
│  ┌──────────────────────────────────────────────────────────────┐    │
│  │              web/  (NEW: go:embed dashboard static files)    │    │
│  └──────────────────────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────────────────┘
        │
┌───────▼──────────────────────────────────────────────────────────────┐
│                    SQLite (single file, WAL mode)                     │
│    /var/lib/jaimito/jaimito.db  — SetMaxOpenConns(1) enforced        │
└──────────────────────────────────────────────────────────────────────┘
```

### Component Responsibilities

| Component | Status | Responsibility | File |
|-----------|--------|----------------|------|
| `cmd/jaimito/serve.go` | MODIFIED | Startup sequence; add `collector.Start()` at step 11.5 | `cmd/jaimito/serve.go` |
| `internal/config/config.go` | MODIFIED | Add `MetricDef` struct, `Metrics []MetricDef` field | `internal/config/config.go` |
| `internal/db/db.go` | UNCHANGED | SQLite open, schema apply, reclaim stuck | `internal/db/db.go` |
| `internal/db/schema/003_metrics.sql` | NEW | Migration for `metrics` + `metric_reads` tables | `internal/db/schema/003_metrics.sql` |
| `internal/db/metrics.go` | NEW | CRUD: `InsertRead`, `UpsertMetric`, `ListMetrics`, `GetReadings` | `internal/db/metrics.go` |
| `internal/db/messages.go` | UNCHANGED | Message queue operations | `internal/db/messages.go` |
| `internal/metrics/collector.go` | NEW | Polling goroutine; shell exec; threshold eval; alert enqueue | `internal/metrics/collector.go` |
| `internal/api/server.go` | MODIFIED | Register dashboard route + metrics API routes | `internal/api/server.go` |
| `internal/api/handlers.go` | MODIFIED | Add `MetricsHandler`, `ReadingsHandler`, `IngestHandler` | `internal/api/handlers.go` |
| `internal/cleanup/scheduler.go` | MODIFIED | Add purge of `metric_reads` older than 7 days | `internal/cleanup/scheduler.go` |
| `internal/web/embed.go` | NEW | `//go:embed static` directive, exports `StaticFS embed.FS` | `internal/web/embed.go` |
| `internal/web/static/` | NEW | Dashboard HTML/JS/CSS assets | `internal/web/static/` |
| `cmd/jaimito/metric.go` | NEW | `jaimito metric` CLI subcommand (manual ingest) | `cmd/jaimito/metric.go` |
| `cmd/jaimito/status.go` | NEW | `jaimito status` CLI subcommand (current metrics) | `cmd/jaimito/status.go` |
| `internal/telegram/format.go` | UNCHANGED | Metric alerts reuse existing message formatting | `internal/telegram/format.go` |
| `internal/dispatcher/dispatcher.go` | UNCHANGED | Delivers queued messages including metric alerts | `internal/dispatcher/dispatcher.go` |

## Recommended Project Structure — v2.0 Additions

```
github.com/chiguire/jaimito/
├── cmd/jaimito/
│   ├── serve.go          # MODIFIED: add metrics.StartCollector() at step 11.5
│   ├── metric.go         # NEW: jaimito metric subcommand (manual ingest)
│   ├── status.go         # NEW: jaimito status subcommand (show current values)
│   ├── keys.go           # unchanged
│   ├── send.go           # unchanged
│   ├── wrap.go           # unchanged
│   ├── setup.go          # unchanged
│   └── setup/            # unchanged
├── internal/
│   ├── api/
│   │   ├── server.go     # MODIFIED: add dashboard + metrics routes
│   │   ├── handlers.go   # MODIFIED: add MetricsHandler, ReadingsHandler, IngestHandler
│   │   ├── middleware.go  # unchanged
│   │   └── response.go   # unchanged
│   ├── cleanup/
│   │   └── scheduler.go  # MODIFIED: add metric_reads 7-day purge
│   ├── config/
│   │   └── config.go     # MODIFIED: add MetricDef, Metrics field, validation
│   ├── db/
│   │   ├── db.go         # unchanged
│   │   ├── messages.go   # unchanged
│   │   ├── apikeys.go    # unchanged
│   │   ├── metrics.go    # NEW: InsertRead, UpsertMetric, ListMetrics, GetReadings
│   │   └── schema/
│   │       ├── 001_initial.sql         # unchanged
│   │       ├── 002_nullable_title.sql  # unchanged
│   │       └── 003_metrics.sql         # NEW: metrics + metric_reads tables
│   ├── dispatcher/
│   │   └── dispatcher.go # unchanged
│   ├── metrics/
│   │   └── collector.go  # NEW: polling goroutine + threshold evaluation + alert enqueue
│   ├── telegram/
│   │   ├── client.go     # unchanged
│   │   └── format.go     # unchanged
│   └── web/
│       ├── embed.go      # NEW: //go:embed directive + StaticFS export
│       └── static/
│           ├── index.html  # Dashboard shell
│           └── app.js      # Alpine.js reactive logic + uPlot chart
```

### Structure Rationale

- **`internal/metrics/`**: New package isolated from `internal/db/` — collector owns scheduling logic, `internal/db/metrics.go` owns persistence. Clean boundary enables independent testing.
- **`internal/web/`**: Separate from `internal/api/` — static assets embedded separately from routing logic. `embed.go` holds the `//go:embed` directive; `api/server.go` references `web.StaticFS`.
- **`internal/db/metrics.go`**: Same package `db` as `messages.go` and `apikeys.go` — consistent with existing pattern of one file per domain in the `db` package.
- **`internal/db/schema/003_metrics.sql`**: Follows existing numbered migration convention managed by `adlio/schema`. The library discovers migrations via glob pattern `schema/*.sql`.

## Architectural Patterns

### Pattern 1: Startup Sequence Extension

**What:** `collector.Start()` is inserted into `serve.go` between `cleanup.Start()` (step 11) and the HTTP server start (step 12). It follows the same goroutine+context cancellation pattern as `dispatcher.Start()` and `cleanup.Start()`.

**When to use:** Any new long-running background service needs to slot into the startup sequence.

**Trade-offs:** All goroutines share the same `ctx` — SIGTERM/SIGINT cancels all cleanly. No additional orchestration needed.

```go
// serve.go — integration point
dispatcher.Start(ctx, database, tgBot, cfg)  // step 10 — unchanged
cleanup.Start(ctx, database, 24*time.Hour)   // step 11 — unchanged

// NEW step 11.5: start metrics collector
metrics.StartCollector(ctx, database, tgBot, cfg)

router := api.NewRouter(database, cfg)  // step 12 — unchanged
```

### Pattern 2: Router Extension (Dashboard + Metrics API)

**What:** `api/server.go`'s `NewRouter()` gets additional route groups. Existing routes are untouched. Dashboard is unauthenticated (localhost-only). Metrics read API is unauthenticated. Metrics write (ingest) reuses existing `BearerAuth` middleware.

**When to use:** Adding new URL namespaces to the existing chi router.

**Trade-offs:** Dashboard is intentionally unauthenticated per PROJECT.md decision ("Dashboard sin auth, acceso via Tailscale"). Safe because server binds to `127.0.0.1:8080` by default.

```go
func NewRouter(database *sql.DB, cfg *config.Config) http.Handler {
    r := chi.NewRouter()
    // ... existing middleware stack — unchanged ...

    // Existing routes — unchanged
    r.Get("/api/v1/health", HealthHandler)
    r.Group(func(r chi.Router) {
        r.Use(BearerAuth(database))
        r.Post("/api/v1/notify", NotifyHandler(database, cfg))
    })

    // NEW: Metrics read API — no auth required
    r.Get("/api/v1/metrics", MetricsHandler(database))
    r.Get("/api/v1/metrics/{name}/readings", ReadingsHandler(database))

    // NEW: Metrics write (manual ingest) — Bearer auth required
    r.Group(func(r chi.Router) {
        r.Use(BearerAuth(database))
        r.Post("/api/v1/metrics/ingest", IngestHandler(database, cfg))
    })

    // NEW: Dashboard — embedded static files served at root
    // Must be last (catch-all) so API routes take priority
    r.Handle("/*", http.FileServer(http.FS(web.StaticFS)))

    return r
}
```

### Pattern 3: Metrics Collector as Single Goroutine with Per-Metric Interval Tracking

**What:** `metrics.StartCollector()` launches one goroutine with a coarse-grained ticker (e.g. every 10 seconds). It maintains a `map[string]time.Time` of when each metric was last collected. When a metric is due (elapsed >= its configured interval), it spawns a short-lived goroutine for the shell exec only.

**When to use:** Per-metric intervals vary (disk_root: 5min, cpu_load: 30s, docker_running: 60s). A single ticker goroutine avoids spawning N separate goroutines with N separate tickers.

**Trade-offs:** Single goroutine + single `*sql.DB` write path simplifies SQLite concurrency. The shell exec goroutines write through the same serialized `*sql.DB` connection pool — no SQLITE_BUSY risk.

```go
func StartCollector(ctx context.Context, db *sql.DB, b *gobot.Bot, cfg *config.Config) {
    go func() {
        ticker := time.NewTicker(10 * time.Second)
        defer ticker.Stop()
        lastRun := make(map[string]time.Time)

        for {
            select {
            case <-ticker.C:
                for _, m := range cfg.Metrics {
                    if time.Since(lastRun[m.Name]) >= m.Interval {
                        lastRun[m.Name] = time.Now()
                        go collectOne(ctx, db, b, cfg, m) // non-blocking exec
                    }
                }
            case <-ctx.Done():
                return
            }
        }
    }()
}
```

`collectOne` runs the shell command, parses stdout, stores the reading, evaluates thresholds, and enqueues an alert message if needed. All DB operations go through the same `*sql.DB` pool.

### Pattern 4: Alert Flow via Existing Dispatcher (Zero New Delivery Infrastructure)

**What:** When a metric crosses a threshold, `collectOne` calls `db.EnqueueMessage()` with channel="monitoring", priority derived from threshold level (warning→high, critical→critical), and a descriptive body. The existing dispatcher picks it up within 1 second and delivers to Telegram via the unmodified path.

**When to use:** Reuse existing delivery infrastructure — no new Telegram client, no new retry logic, no new message status machine.

**Trade-offs:** Alert latency up to 1 second (dispatcher poll interval). Acceptable for VPS monitoring. Zero changes to `internal/telegram/` or `internal/dispatcher/`.

```
metric threshold crossed
    → db.EnqueueMessage(channel="monitoring", priority="high", body="disk_root: 92% (threshold: 90%)")
    → dispatcher.dispatchNext() picks it up within 1s
    → telegram.FormatMessage() formats it
    → bot.SendMessage() delivers to Telegram
```

### Pattern 5: go:embed for Dashboard Static Files

**What:** A dedicated `internal/web/embed.go` file holds `//go:embed static` and exports `var StaticFS embed.FS`. `api/server.go` imports the `web` package and serves `web.StaticFS` via `http.FileServer(http.FS(web.StaticFS))`.

**When to use:** Any static file tree that must be compiled into the binary at build time.

**Trade-offs:** Files are read-only and goroutine-safe after init. Tailwind CSS (CDN or locally vendored), uPlot (~14KB), Alpine.js (~15KB), and Lucide icons are all served from this embedded FS without network access at runtime.

```go
// internal/web/embed.go
package web

import "embed"

//go:embed static
var StaticFS embed.FS
```

The `//go:embed static` directive embeds the entire `static/` directory. `http.FileServer(http.FS(StaticFS))` serves `static/index.html` at `GET /`, and all assets at their relative paths.

## Data Flow

### Flow 1: Metric Collection → Storage → Alert

```
cfg.Metrics[i].Interval elapsed
    ↓
collector goroutine: os/exec runs shell command (e.g. "df / | awk ...")
    ↓
parse stdout → float64 value
    ↓
db.InsertRead(name, value, ts)      ← INSERT INTO metric_reads
db.UpsertMetric(name, value, ts)    ← INSERT OR REPLACE INTO metrics (current state)
    ↓ (both writes serialized by SetMaxOpenConns(1))
evaluate: value >= threshold.critical or >= threshold.warning?
    │
    ├─ NO: done for this metric
    │
    └─ YES: db.EnqueueMessage(
               channel="monitoring",
               priority="high"/"critical",
               body="disk_root: 92% [threshold: 90%]"
           )
               ↓
           dispatcher.dispatchNext() picks it up within 1s
               ↓
           telegram.FormatMessage() → bot.SendMessage()
               ↓
           Telegram notification delivered
```

### Flow 2: Dashboard API Read

```
Browser GET /api/v1/metrics
    ↓
chi router → MetricsHandler
    ↓
db.ListMetrics() → SELECT name, last_value, last_collected_at, status FROM metrics
    ↓
JSON response: [{name, last_value, last_collected_at, status}, ...]
    ↓
Alpine.js renders metrics table

Browser GET /api/v1/metrics/{name}/readings?hours=24
    ↓
chi router → ReadingsHandler (chi.URLParam "name")
    ↓
db.GetReadings(name, since) → SELECT value, collected_at FROM metric_reads
                               WHERE metric_name=? AND collected_at > ?
                               ORDER BY collected_at ASC LIMIT 1000
    ↓
JSON response: [{ts, value}, ...]
    ↓
uPlot renders time-series chart on row expand
```

### Flow 3: Manual CLI Metric Ingest

```
jaimito metric --name disk_root --value 72.5
    ↓
cmd/jaimito/metric.go: POST /api/v1/metrics/ingest with Bearer token
    ↓
IngestHandler: parse JSON, call db.InsertRead + db.UpsertMetric
    ↓
threshold evaluation (same logic as collector)
    ↓
optional: db.EnqueueMessage if threshold crossed
```

### Flow 4: Startup Sequence (serve.go annotated)

```
1.  slog setup
2.  config.Load()                    ← reads MetricsConfig from yaml (NEW field)
3.  signal context
4.  telegram.ValidateToken()
5.  telegram.ValidateChats()
6.  db.Open()
7.  db.ApplySchema()                 ← applies 003_metrics.sql (NEW migration)
8.  db.ReclaimStuck()
9.  db.SeedKeys()
10. dispatcher.Start()               ← unchanged
11. cleanup.Start()                  ← unchanged (but internally modified for metric_reads)
[NEW] metrics.StartCollector()       ← inserted at step 11.5
12. api.NewRouter() + http.Server    ← router modified: new routes + dashboard
13. log ready
14. wait for ctx.Done()
15. server.Shutdown()
```

## SQLite Concurrency Strategy

This is the critical integration constraint. The existing codebase uses `SetMaxOpenConns(1)` to serialize all database operations through a single connection. This strategy is correct and must not change for v2.0.

### Current Configuration (db.Open — unchanged)

```go
db.SetMaxOpenConns(1)       // serialize all ops through one connection
db.SetMaxIdleConns(1)
db.SetConnMaxLifetime(0)    // connections live for app lifetime
// DSN includes: busy_timeout(5000) — 5s wait before SQLITE_BUSY
```

### New Write Paths Added in v2.0

Three new writers join the existing pool:

| Writer | Frequency | Operation |
|--------|-----------|-----------|
| `collector` goroutine | Per metric interval (30s–5min) | `INSERT INTO metric_reads` + `INSERT OR REPLACE INTO metrics` |
| `collector` goroutine (alerts) | Occasional (threshold crossings) | `INSERT INTO messages` (EnqueueMessage) |
| `jaimito metric` CLI | Manual, infrequent | Same as collector writes |

### Why SetMaxOpenConns(1) Remains Correct

All components (dispatcher, collector, HTTP handlers, cleanup) share the same `*sql.DB` pointer passed from `serve.go`. Go's `database/sql` pool queues concurrent calls and serializes them automatically. At VPS scale (10–50 metrics, 1–10 messages/hour), contention is negligible:

- Metric reads are tiny (single float64 + timestamp INSERT: ~1ms)
- Dispatcher reads one message row per second
- Dashboard API reads are fast SELECT queries
- Cleanup runs hourly DELETE batches

No second connection, no WAL read-only separation, no mutex — the existing single-pool pattern is sufficient.

## New Database Schema (003_metrics.sql)

```sql
-- Table: current state of each metric (one row per metric name)
CREATE TABLE IF NOT EXISTS metrics (
    name              TEXT PRIMARY KEY,
    last_value        REAL,
    last_collected_at TEXT,
    status            TEXT NOT NULL DEFAULT 'unknown',
    created_at        TEXT NOT NULL DEFAULT (datetime('now'))
);
-- status values: 'ok', 'warning', 'critical', 'unknown'

-- Table: time-series readings for history (7-day retention via cleanup)
CREATE TABLE IF NOT EXISTS metric_reads (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    metric_name   TEXT NOT NULL REFERENCES metrics(name),
    value         REAL NOT NULL,
    collected_at  TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_metric_reads_name_ts
    ON metric_reads(metric_name, collected_at);
```

Note: Two-table design separates current state (metrics) from history (metric_reads). `metrics` table is cheap to query for the dashboard summary view. `metric_reads` holds the time-series data for charts.

## New Config Schema Addition

```go
// Added to internal/config/config.go

type MetricDef struct {
    Name     string        `yaml:"name"`
    Command  string        `yaml:"command"`          // shell command returning a single float
    Interval time.Duration `yaml:"interval"`         // e.g. 30s, 5m
    Unit     string        `yaml:"unit,omitempty"`   // display unit: "%", "GB", "days"
    Warning  *float64      `yaml:"warning,omitempty"`
    Critical *float64      `yaml:"critical,omitempty"`
    Channel  string        `yaml:"channel,omitempty"` // default: "monitoring"
}

// Added to Config struct
// Metrics []MetricDef `yaml:"metrics"`
```

Config example addition (configs/config.example.yaml):
```yaml
metrics:
  - name: disk_root
    command: "df / | awk 'NR==2 {gsub(\"%\",\"\",$5); print $5}'"
    interval: 5m
    unit: "%"
    warning: 80
    critical: 90
    channel: monitoring
  - name: ram_used
    command: "free | awk '/^Mem:/ {printf \"%.0f\", $3/$2*100}'"
    interval: 1m
    unit: "%"
    warning: 85
    critical: 95
```

## Integration Points Summary

### Files Modified

| File | What Changes |
|------|-------------|
| `cmd/jaimito/serve.go` | Add `metrics.StartCollector(ctx, database, tgBot, cfg)` at step 11.5 |
| `internal/config/config.go` | Add `MetricDef` struct, `Metrics []MetricDef` field, validation |
| `internal/api/server.go` | Add 3 metrics routes + dashboard static file catch-all route |
| `internal/api/handlers.go` | Add `MetricsHandler`, `ReadingsHandler`, `IngestHandler` |
| `internal/cleanup/scheduler.go` | Add `DELETE FROM metric_reads WHERE collected_at < datetime('now', '-7 days')` |
| `configs/config.example.yaml` | Add `metrics:` section with predefined examples |

### Files Created

| File | Purpose |
|------|---------|
| `internal/db/schema/003_metrics.sql` | Schema migration for metrics tables |
| `internal/db/metrics.go` | `InsertRead`, `UpsertMetric`, `ListMetrics`, `GetReadings` |
| `internal/metrics/collector.go` | Polling goroutine, shell exec, threshold eval, alert enqueue |
| `internal/web/embed.go` | `//go:embed static` + `var StaticFS embed.FS` |
| `internal/web/static/index.html` | Dashboard HTML shell |
| `internal/web/static/app.js` | Alpine.js reactive logic + uPlot integration |
| `cmd/jaimito/metric.go` | `jaimito metric` CLI subcommand |
| `cmd/jaimito/status.go` | `jaimito status` CLI subcommand |

### Files Unchanged

| File | Reason |
|------|--------|
| `internal/dispatcher/dispatcher.go` | Alert delivery is transparent — metric alerts are just messages |
| `internal/telegram/format.go` | `FormatMessage()` formats metric alerts like any other message |
| `internal/telegram/client.go` | Same Telegram bot client |
| `internal/db/db.go` | Open/schema/reclaim logic untouched |
| `internal/db/messages.go` | Message queue operations untouched |
| `internal/db/apikeys.go` | Auth logic untouched |
| `internal/api/middleware.go` | `BearerAuth` reused unchanged for new authenticated routes |
| `cmd/jaimito/send.go`, `wrap.go`, `keys.go`, `setup.go` | Unaffected |

## Suggested Build Order (phase dependencies)

Build phases in this dependency order so each compiles and tests independently:

```
Phase 1: Config extension
    internal/config/config.go
    — Add MetricDef struct, Metrics []MetricDef, validation rules
    — No new imports, no new packages
    — DEPENDENCY: nothing; must be built first (all later phases depend on it)

Phase 2: Database layer
    internal/db/schema/003_metrics.sql
    internal/db/metrics.go
    — InsertRead, UpsertMetric, ListMetrics, GetReadings
    — DEPENDENCY: Phase 1 (config types used for threshold lookup in IngestHandler)

Phase 3: Metrics collector
    internal/metrics/collector.go
    — Shell exec goroutine, threshold evaluation, db.EnqueueMessage alerts
    — DEPENDENCY: Phase 1 (MetricDef), Phase 2 (db.metrics functions)

Phase 4: serve.go integration
    cmd/jaimito/serve.go
    — Add metrics.StartCollector() at step 11.5
    — DEPENDENCY: Phase 3 complete

Phase 5: API + Dashboard
    internal/api/handlers.go  — add metric handlers
    internal/api/server.go    — register routes
    internal/web/             — embed static files + dashboard HTML/JS
    — DEPENDENCY: Phase 2 (db.metrics functions); independent of Phase 3-4

Phase 6: Cleanup extension
    internal/cleanup/scheduler.go
    — Add metric_reads 7-day purge
    — DEPENDENCY: Phase 2 (003_metrics.sql must exist)

Phase 7: CLI subcommands
    cmd/jaimito/metric.go
    cmd/jaimito/status.go
    — DEPENDENCY: Phase 2 + Phase 5 HTTP ingest endpoint (or direct db write)
```

Phases 3-4 and Phases 5-6 are independent and can be built in parallel worktrees.
Phase 7 is independent of Phases 3-6 but benefits from both being complete.

## Anti-Patterns

### Anti-Pattern 1: Second SQLite Connection for Dashboard Reads

**What people do:** Open a second `*sql.DB` instance for the dashboard/metrics API read path, reasoning that "reads don't need to block on writes."

**Why it's wrong:** Even with WAL mode, multiple connections complicate the write serialization model the codebase relies on. The existing `SetMaxOpenConns(1)` was chosen deliberately to avoid SQLITE_BUSY. At VPS scale, metric API reads complete in under 1ms — no observable latency from serialization.

**Do this instead:** Pass the same `*sql.DB` pointer to all components. The Go `database/sql` pool serializes naturally at no perceptible cost.

### Anti-Pattern 2: One Goroutine Per Metric

**What people do:** Launch `go collectMetric(ctx, m)` for each `MetricDef` at startup — N goroutines with N separate `time.NewTicker` instances.

**Why it's wrong:** N concurrent goroutines all try to write to SQLite simultaneously. With `SetMaxOpenConns(1)` they serialize anyway but generate unnecessary goroutine overhead and make shutdown ordering harder.

**Do this instead:** Single goroutine with a 10-second coarse-grained ticker. Check `time.Since(lastRun[name]) >= m.Interval` per metric. Spawn a short-lived goroutine only for the actual shell exec (I/O bound), keeping DB writes sequential.

### Anti-Pattern 3: New Delivery Path for Metric Alerts

**What people do:** Add a `metric_alerts` table, a second dispatcher goroutine, and new Telegram formatting for metric alerts — reasoning that alerts are different from messages.

**Why it's wrong:** `db.EnqueueMessage()` + the existing dispatcher handles this perfectly. Metric alerts are messages: they have a body, priority, and channel. Adding a parallel delivery path doubles the complexity of the dispatcher system.

**Do this instead:** `db.EnqueueMessage(channel="monitoring", priority="high", body="disk_root: 92%")`. The dispatcher delivers it within 1 second. The existing `telegram.FormatMessage()` formats it correctly.

### Anti-Pattern 4: Auth on Dashboard Routes

**What people do:** Add `BearerAuth` middleware to `GET /` dashboard routes "for security."

**Why it's wrong:** PROJECT.md explicitly specifies "Dashboard sin auth (localhost only, acceso via Tailscale)." The `127.0.0.1:8080` bind address already restricts access. Adding auth creates friction for the single-user VPS monitoring use case.

**Do this instead:** Serve dashboard at `GET /*` with no auth. Document that dashboard access requires either localhost or Tailscale VPN access.

### Anti-Pattern 5: Storing Raw Shell Commands Output Without Parsing

**What people do:** Store the full stdout of shell commands as text in `metric_reads.value`.

**Why it's wrong:** The `metric_reads.value` column must be `REAL` (float64) for uPlot to chart it and for threshold comparison (`value >= threshold.warning`). Text storage requires runtime parsing at read time.

**Do this instead:** Shell commands must output a single float-parseable line. Document this contract in `MetricDef.Command`. The collector calls `strconv.ParseFloat(strings.TrimSpace(stdout), 64)` and stores the result.

## Scaling Considerations

| Scale | Architecture Adjustments |
|-------|--------------------------|
| Single VPS, up to ~50 metrics | Current design — zero changes needed |
| Multi-VPS monitoring | Network ingest endpoint + per-source namespacing required. Out of scope for v2.0. |
| High-frequency metrics (<5s intervals) | In-memory ring buffer + batch INSERT instead of per-reading INSERT. Not needed at VPS scale. |
| Large history (>30 days) | Increase metric_reads retention or add downsampling. 7-day default is intentional. |

## Sources

- Direct codebase inspection: `cmd/jaimito/serve.go`, `internal/api/server.go`, `internal/db/db.go`, `internal/db/messages.go`, `internal/dispatcher/dispatcher.go`, `internal/cleanup/scheduler.go`, `internal/config/config.go`, `internal/db/schema/*.sql`, `go.mod`
- Go standard library `embed` package: https://pkg.go.dev/embed (HIGH confidence — stdlib)
- adlio/schema migration pattern: confirmed by existing `schema/*.sql` file structure and `ApplySchema()` in `internal/db/db.go`
- modernc.org/sqlite single-writer strategy: confirmed by `SetMaxOpenConns(1)` comment in `internal/db/db.go`: "CRITICAL: SQLite supports only one concurrent writer"
- PROJECT.md: v2.0 milestone requirements, technology choices (Alpine.js ~15KB, uPlot ~14KB, Tailwind CSS)

---
*Architecture research for: jaimito v2.0 — metrics collector + embedded dashboard integration*
*Researched: 2026-03-26*
