# Pitfalls Research

**Domain:** Self-hosted VPS notification hub (Go + SQLite + Telegram Bot API + CLI)
**Researched:** 2026-02-20 (v1.0 MVP) | Updated: 2026-03-23 (v1.1 Setup Wizard) | Updated: 2026-03-26 (v2.0 Métricas y Dashboard)
**Confidence:** HIGH (SQLite/Go findings HIGH via multiple verified sources; Telegram API findings MEDIUM via official docs + community; bubbletea TUI pitfalls HIGH for stdin/permissions, MEDIUM for async patterns; v2.0 metrics pitfalls HIGH for os/exec/SQLite contention/go:embed, MEDIUM for alerting patterns)

---

## v2.0 Métricas y Dashboard Pitfalls

These pitfalls are specific to adding system metrics collection, threshold alerting, and an embedded web dashboard to the existing jaimito binary. They are the primary concern for the current milestone.

---

### Pitfall M1: os/exec with Shell (`sh -c`) Enables Command Injection from config.yaml

**What goes wrong:**
Metrics are defined in `config.yaml` as shell commands — e.g., `command: "df -h / | awk '{print $5}'"`. If the collector runs these via `exec.Command("sh", "-c", command)`, any YAML value that contains shell metacharacters (`;`, `&&`, `$(`, backticks) executes arbitrary shell code as the jaimito process user (root on most VPS deployments). An attacker who gains write access to `/etc/jaimito/config.yaml` achieves RCE.

**Why it happens:**
Developers reach for `sh -c` to support shell pipelines in metric commands (essential for `df | awk`-style extraction). The convenience of shell features makes the risk invisible until the config file is seen as an attack surface.

**How to avoid:**
Use `sh -c` intentionally but treat the config file as a security boundary:
- Enforce `0600` on `config.yaml` at startup (already done for v1.0). If the file is world-readable, refuse to start.
- Never interpolate user-supplied runtime data (e.g., metric names from API) into shell command strings.
- For the built-in predefined metrics (`disk_root`, `ram_used`, etc.), hardcode the commands in Go — do not read them from config. Only user-defined metrics come from config.
- Document in config schema that `command` values are executed as shell; advise VPS operators to treat the config file like a crontab.

**Warning signs:**
- `config.yaml` has permissions 0644 or 0664
- Metric command contains `$(...)` or backticks
- Config file is writable by the service user (jaimito) — the service should not be able to modify its own config

**Phase to address:** Phase 1 (metric collector scaffold) — enforce config file permission check on startup before any command is ever executed.

---

### Pitfall M2: os/exec Timeout Not Applied — Metric Collection Hangs the Collector Goroutine

**What goes wrong:**
A metric command like `docker stats --no-stream` or a custom `curl`-based check hangs due to network issues or a zombie Docker daemon. Without a context timeout, the `cmd.Run()` call blocks indefinitely. The ticker-based collector goroutine stalls. No metrics are collected for that metric, and if the collector runs commands sequentially, all subsequent metrics also stop. Over time, goroutines accumulate because each tick spawns a new `exec.Command` that also blocks.

**Why it happens:**
`exec.Command("sh", "-c", cmd)` with no context has no built-in timeout. `exec.CommandContext` is the correct form but developers forget to set it, especially since the command works fine locally. Docker commands in particular can block for 30+ seconds when the Docker daemon is unresponsive.

**How to avoid:**
Always use `exec.CommandContext` with a timeout capped well below the metric's collection interval:

```go
// Timeout = 80% of interval, max 30s
timeout := min(time.Duration(float64(interval)*0.8), 30*time.Second)
ctx, cancel := context.WithTimeout(parentCtx, timeout)
defer cancel()
cmd := exec.CommandContext(ctx, "sh", "-c", command)
out, err := cmd.Output()
```

Additionally, set `cmd.WaitDelay` (Go 1.20+) to bound the time waiting for pipe goroutines after the context deadline:

```go
cmd.WaitDelay = 5 * time.Second
```

**Warning signs:**
- `ps aux | grep sh` shows accumulating zombie shell processes
- Metric collection stops for one metric and never resumes
- Memory usage grows slowly over days without apparent cause
- `runtime.NumGoroutine()` increases monotonically

**Phase to address:** Phase 1 (metric collector) — every `exec.CommandContext` call must include a timeout. Verify with a `sleep 60` test command.

---

### Pitfall M3: Child Process Spawns Sub-children — Context Cancel Kills Parent but Not Children

**What goes wrong:**
A metric command like `docker ps --format ...` spawns sh which spawns docker. When the context deadline fires, Go's `exec.CommandContext` sends SIGKILL to the direct child (sh), but sh's child (docker) may still be running if it has already detached from sh's process group. The docker process runs until its own timeout. With a metric that runs every 30 seconds, dozens of orphaned docker processes accumulate.

**Why it happens:**
`exec.CommandContext` calls `cmd.Process.Kill()` on the direct process only. On Linux, child processes of the shell are in a separate process group unless you explicitly set `cmd.SysProcAttr` to create a new process group and send the kill signal to the entire group.

**How to avoid:**
Set a process group and kill the whole group on timeout:

```go
cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
// On timeout, kill entire group:
if ctx.Err() != nil {
    syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
}
```

For most predefined metrics (`df`, `free`, `uptime`), this is not a concern — they are single-process. Only commands that invoke multi-process chains (docker, systemctl) need this protection.

**Warning signs:**
- `ps aux --forest` shows orphaned docker/systemctl children owned by jaimito
- System load increases over days despite low notification/metric volume
- `docker stats` processes visible in process list long after their collection window

**Phase to address:** Phase 1 (metric collector) — implement process group kill for the Docker metric family. Test by interrupting a `docker stats --no-stream` mid-execution.

---

### Pitfall M4: Metric Writes Block Message Delivery — SQLite Single Writer Pool Conflict

**What goes wrong:**
jaimito's existing SQLite pool is configured as `SetMaxOpenConns(1)` — a single writer that serialises all operations. The v1.0 dispatcher and API ingestor were the only writers; their write patterns are short and fast. Adding a metric collector that writes every 30 seconds per metric (5 predefined + N user metrics) creates a write burst at each interval. If a collection interval fires while the dispatcher is in a slow Telegram API call with a pending status update, the dispatcher's write (`UPDATE messages SET status='dispatching'`) queues behind the metric inserts. Notification delivery latency increases.

**Why it happens:**
The `busy_timeout=5000` setting allows the write to wait up to 5 seconds, but the metrics write burst (10+ INSERTs in rapid succession) combined with the dispatcher's own transaction can cause queuing delays visible to the user as notification lag.

**How to avoid:**
Keep metric INSERTs small and fast — never hold a transaction open during command execution. The pattern must be:

1. Execute shell command (outside any transaction)
2. Record the result
3. Open transaction, INSERT reading, COMMIT immediately

Never do: open transaction → run shell command → INSERT → commit. The shell command could take seconds, holding the write lock throughout.

Additionally, batch metric INSERTs at the end of a collection round rather than one transaction per metric:

```go
// Collect all readings first
readings := collectAll(metrics)
// Then write in one transaction
tx.Begin()
for _, r := range readings {
    tx.Insert(r)
}
tx.Commit()
```

**Warning signs:**
- Notification delivery latency increases from <1s to 2-5s after metrics are added
- SQLite `busy_timeout` log warnings appearing in metric write paths
- `PRAGMA wal_checkpoint` takes longer than usual after metric collection intervals

**Phase to address:** Phase 1 (metric collector) AND Phase 2 (schema + storage) — enforce the collect-then-write pattern from the first implementation.

---

### Pitfall M5: go:embed Path Resolution — Files Must Be in or Below the Package Directory

**What goes wrong:**
The dashboard frontend files (HTML, JS, CSS) are intended to live at `web/dashboard/`. If the `//go:embed` directive is placed in a package that is not in the same directory or a parent of `web/`, the build fails with `pattern web/dashboard: no matching files found`. Developers try `//go:embed ../../web/dashboard` expecting relative paths to work — they don't. go:embed only allows paths relative to the file containing the directive, and only within the module.

**Why it happens:**
The natural desire is to put the embed directive in an `internal/dashboard` package alongside the HTTP handler, but that package may be at `internal/dashboard/handler.go` while assets are at `web/dashboard/`. The Go toolchain does not follow `..` traversal in embed patterns.

**How to avoid:**
Place a dedicated `embed.go` file in the same package at the root level, or restructure so the `web/` directory is a child of the package containing the directive. The cleanest pattern for jaimito:

```go
// In package main (cmd/jaimito/main.go or a sibling assets.go):
//go:embed web/dashboard
var DashboardFS embed.FS
```

Then pass `DashboardFS` down to the dashboard handler. Alternatively, create `internal/assets/assets.go` and place `web/` inside `internal/assets/web/`.

**Warning signs:**
- `go build` fails with "pattern X: no matching files found"
- Embed works locally but fails in CI where paths differ
- Developer uses `os.DirFS` in development and embed.FS in production — path mismatch causes 404s

**Phase to address:** Phase 3 (dashboard scaffold) — establish the embed file structure before writing a single line of HTML.

---

### Pitfall M6: go:embed Silently Excludes Files Starting with `.` or `_`

**What goes wrong:**
Tailwind pre-compiled output may be named `.output.css` or placed in `_dist/`. go:embed's glob patterns silently exclude files and directories whose names start with `.` or `_`. The build succeeds, the binary is built, the server starts — but requests for the CSS file return 404. The developer debugs HTTP routing for hours before discovering the file was never embedded.

**Why it happens:**
This is documented behavior in the `embed` package, but easy to miss. The exclusion applies to both direct file patterns and directory patterns.

**How to avoid:**
- Name all embedded assets without leading `.` or `_`: use `tailwind.css`, `dist/`, `assets/` — never `_dist/` or `.build/`.
- Verify embedded contents at build time: `go list -json -mod=mod ./... | jq '.[].EmbedFiles'` lists what is actually embedded.
- Add a build-time test that opens the embedded FS and asserts the critical files exist:

```go
func TestEmbeddedAssets(t *testing.T) {
    _, err := dashboardFS.Open("web/dashboard/index.html")
    require.NoError(t, err)
    _, err = dashboardFS.Open("web/dashboard/tailwind.css")
    require.NoError(t, err)
}
```

**Warning signs:**
- CSS/JS files return 404 in production binary but work in `go run` dev mode
- File sizes show 0 bytes for assets that should exist
- `http.FileServer(http.FS(...))` returns "file not found" for known-good paths

**Phase to address:** Phase 3 (dashboard scaffold) — add the embedded assets test before building any dashboard UI.

---

### Pitfall M7: Dashboard SPA Routing Breaks — chi FileServer Returns 404 for Non-Root Paths

**What goes wrong:**
The dashboard is served as a single-page app from an embedded FS. Direct navigation to `/dashboard/metrics/disk_root` (a client-side route) causes chi's `http.FileServer` to look for a file named `metrics/disk_root` in the embedded FS. It doesn't exist. The server returns 404 instead of serving `index.html` and letting the client-side router handle it.

**Why it happens:**
`http.FileServer` is a file server, not an SPA server. It serves files that exist; unknown paths return 404. SPA routing requires the server to return `index.html` for any path that doesn't match a known static file.

**How to avoid:**
Implement a custom SPA handler that checks if the requested file exists in the embedded FS, and falls back to `index.html` if it doesn't:

```go
func spaHandler(fs embed.FS, rootDir string) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        path := strings.TrimPrefix(r.URL.Path, "/")
        f, err := fs.Open(filepath.Join(rootDir, path))
        if err != nil {
            // File not found — serve index.html for SPA routing
            indexPath := filepath.Join(rootDir, "index.html")
            http.ServeFileFS(w, r, fs, indexPath)
            return
        }
        f.Close()
        http.ServeFileFS(w, r, fs, filepath.Join(rootDir, path))
    }
}
```

Note: For jaimito's dashboard, if all navigation is handled by Alpine.js without changing the URL path (hash-based routing or single-page with no routing), this pitfall does not apply. Design the dashboard to avoid URL-based SPA routing if possible.

**Warning signs:**
- Direct URL navigation to any dashboard subpath returns 404
- Browser back/forward buttons break after any navigation
- Refreshing the page on any non-root URL returns 404

**Phase to address:** Phase 3 (dashboard scaffold) — decide on routing strategy (hash vs path) before building navigation.

---

### Pitfall M8: Tailwind Play CDN Cannot Be Embedded — It Requires Network at Runtime

**What goes wrong:**
The Tailwind Play CDN (`<script src="https://cdn.tailwindcss.com">`) compiles CSS in the browser at runtime by scanning the DOM for class names. This requires a live network connection to Tailwind's CDN. On a VPS that is offline or behind a strict firewall, the dashboard renders unstyled. Additionally, the Play CDN bundle is ~300KB and generates styles on every page load — defeating the purpose of embedding assets into the binary for offline capability.

**Why it happens:**
Play CDN is marketed as the zero-setup way to use Tailwind, and it works perfectly in development. Developers embed the HTML with the CDN script tag and embed all other JS/CSS files, not realising Tailwind is the one dependency that cannot be embedded this way.

**How to avoid:**
Pre-compile Tailwind CSS using the Tailwind CLI standalone binary as a build step before `go build`:

```bash
# In Makefile or build script:
tailwindcss -i web/dashboard/src/input.css -o web/dashboard/tailwind.css --minify
go build ./...
```

The output `tailwind.css` is a static file that can be embedded with `//go:embed`. The compiled CSS for jaimito's dashboard will be 5-15KB, not 300KB. The Tailwind CLI standalone binary does not require Node.js.

Alternatively, write the dashboard CSS without Tailwind and use a minimal hand-written CSS file — for a simple metrics table and chart, ~2KB of custom CSS is sufficient.

**Warning signs:**
- Dashboard renders as unstyled HTML when tested with network disabled
- CSS file in embedded FS is empty or missing
- Build logs show no Tailwind compilation step

**Phase to address:** Phase 3 (dashboard scaffold) — establish the Tailwind build step in the Makefile before writing any dashboard HTML. Never use Play CDN in the final implementation.

---

### Pitfall M9: Alert Storm — Threshold Crossed on Every Collection Fires Unlimited Notifications

**What goes wrong:**
Disk usage is at 91% (threshold: 90%). The collector runs every 60 seconds. Every collection reads 91% and fires a Telegram alert. The user receives 1 alert per minute indefinitely until disk drops below 90%. At 60 alerts/hour, this is notification fatigue at best; at worst, it triggers jaimito's own Telegram rate limiting (Pitfall 3) and real notifications (from `jaimito wrap`) are blocked.

**Why it happens:**
Naive threshold implementation: `if value > threshold { sendAlert() }`. Every collection where the condition is true fires a new alert.

**How to avoid:**
Implement alert state tracking with cooldown in SQLite:

1. Store alert state per metric: `alert_state` table with columns `(metric_name, level, fired_at, resolved_at)`.
2. Only fire an alert when transitioning from `ok` to `warning` or `warning` to `critical` — not on every collection.
3. Implement a cooldown: once an alert fires, suppress re-alerts for the same metric at the same level for at least `cooldown_minutes` (configurable, default 60 minutes).
4. Fire a resolution notification when the metric returns below threshold.

```
State transitions that trigger notifications:
  ok → warning     : fire warning alert
  ok → critical    : fire critical alert
  warning → critical : fire escalation alert
  critical → ok    : fire resolution alert
  warning → ok     : fire resolution alert
  Same level (repeated): suppressed
```

**Warning signs:**
- Multiple identical alert notifications in Telegram within minutes
- Telegram showing "too many requests" errors after a metric crosses threshold
- `SELECT COUNT(*) FROM messages WHERE channel='alerts' AND created_at > datetime('now', '-1 hour')` returns >5

**Phase to address:** Phase 4 (threshold alerting) — implement state machine from the first alert, not as a patch after storms occur.

---

### Pitfall M10: Alert Flapping — Metric Oscillating Around Threshold Causes Rapid On/Off Alerts

**What goes wrong:**
CPU load averages 89.5-90.5% due to normal variation. Every other collection crosses the 90% threshold. The state machine correctly fires an alert (89→91), then a resolution (91→89), then another alert (89→91), cycling every few minutes. The user receives alternating "alert" and "resolved" notifications continuously.

**Why it happens:**
A single threshold with no hysteresis has no tolerance for natural variance. The alert state transitions correctly by the state machine logic, but the frequency is intolerable.

**How to avoid:**
Implement hysteresis with separate alert and recovery thresholds:

```yaml
metrics:
  - name: cpu_load
    threshold:
      warning: 90
      warning_recovery: 85   # Must drop to 85% before alert clears
      critical: 95
      critical_recovery: 90
```

Alert fires at 90%, but only clears when value drops to 85%. This creates a 5% deadband that absorbs natural oscillation.

Additionally, require N consecutive threshold crossings before firing (e.g., `alert_for: 3` means value must be above threshold for 3 consecutive collections before alerting). This absorbs single-point spikes.

**Warning signs:**
- User reports receiving alternating alert/resolved notifications in pairs
- Alert fired timestamp and resolved timestamp are less than 2 collection intervals apart
- `SELECT * FROM alert_state WHERE metric_name='cpu_load' ORDER BY fired_at DESC LIMIT 10` shows alternating fired/resolved rows

**Phase to address:** Phase 4 (threshold alerting) — implement recovery thresholds and `alert_for` count in the alerting design, not as a retrospective fix.

---

### Pitfall M11: Metric Collector Goroutine Leaks When Context Is Cancelled During Command Execution

**What goes wrong:**
jaimito receives SIGTERM (systemd stop). The main context is cancelled. The metric collector's ticker loop checks `ctx.Done()` and exits cleanly — but only if no command is currently executing. If a collection was mid-flight (e.g., `docker stats` running), the goroutine is blocked in `cmd.Wait()` and does not check `ctx.Done()`. Shutdown blocks until the command completes or times out. If `WaitDelay` was not set, it blocks forever.

**Why it happens:**
`os/exec`'s `CommandContext` sends a kill signal when the context is cancelled, but if the kill signal doesn't terminate the process (e.g., it's caught), `cmd.Wait()` still blocks. The goroutine running `cmd.Wait()` does not select on `ctx.Done()` — it's a blocking call.

**How to avoid:**
Set `cmd.WaitDelay` on every `exec.CommandContext` call. `WaitDelay` bounds the time `cmd.Wait()` will wait for pipe-copying goroutines after the context is cancelled:

```go
cmd := exec.CommandContext(ctx, "sh", "-c", command)
cmd.WaitDelay = 3 * time.Second  // Force-close pipes 3s after context cancellation
out, err := cmd.Output()
```

The metric collector's main loop must use `select` with both ticker and `ctx.Done()`:

```go
ticker := time.NewTicker(interval)
defer ticker.Stop()
for {
    select {
    case <-ctx.Done():
        return
    case <-ticker.C:
        go collectMetric(ctx, metric)  // context propagated; collector respects cancellation
    }
}
```

**Warning signs:**
- `systemctl stop jaimito` takes >5 seconds to complete
- `journalctl -u jaimito` shows "Timeout: killing process" from systemd
- Process still visible in `ps aux` after `systemctl stop`

**Phase to address:** Phase 1 (metric collector) — implement shutdown gracefully from the first goroutine. Test with `systemctl stop` in the phase acceptance criteria.

---

### Pitfall M12: Metric Retention Purge Deletes Too Aggressively — Misses Timezone Edge Cases

**What goes wrong:**
The 7-day purge query uses `datetime('now', '-7 days')` in SQLite. SQLite's `now` returns UTC. If the VPS is configured with a local timezone (e.g., UTC-5), the purge may delete readings that are only 6.x days old in local time. More critically, if the purge runs at startup and the VPS clock was wrong at insert time, readings may have `collected_at` values in the future or far past — purge never removes them (future) or removes them immediately (clock skew).

**Why it happens:**
SQLite `datetime('now')` is always UTC. If timestamps are stored as local time strings (e.g., Go's `time.Now().String()` without UTC conversion), the comparison is between UTC now and local-time strings — the subtraction is wrong.

**How to avoid:**
Store all `collected_at` timestamps as UTC using `time.Now().UTC()` and insert as RFC3339 strings or Unix timestamps. Use `strftime('%s', 'now')` (Unix epoch) for reliable timezone-independent comparisons:

```sql
DELETE FROM metric_readings WHERE collected_at < strftime('%s', 'now') - (7 * 86400);
```

Or, if using RFC3339 strings, always use `datetime('now')` (UTC) for comparisons and ensure all inserts use `time.Now().UTC().Format(time.RFC3339)`.

**Warning signs:**
- Metric readings disappearing earlier than 7 days (clock skew or timezone mismatch)
- Old readings never purged despite being weeks old (future-dated inserts from clock skew)
- `SELECT MIN(collected_at), MAX(collected_at) FROM metric_readings` returns unexpected range

**Phase to address:** Phase 2 (schema + storage) — establish UTC-only convention in schema design and enforce in all inserts.

---

### Pitfall M13: Dashboard Served Without Content-Type Headers — Browser Refuses to Execute JS/CSS

**What goes wrong:**
`http.FileServer(http.FS(embeddedFS))` sets content-type headers automatically based on file extension. However, if the embedded FS is served with a custom handler that calls `w.Write(content)` without setting `Content-Type`, or if the MIME type is not registered for `.css`/`.js`, the browser receives `Content-Type: application/octet-stream`. Chrome and Firefox refuse to apply stylesheets or execute scripts with the wrong content type. The dashboard loads as an unstyled page with no JavaScript.

**Why it happens:**
Developers writing a custom static file handler forget that `http.ServeContent` or `http.ServeFileFS` auto-detects MIME types, but `w.Write()` alone does not. The `net/http` package's MIME detection relies on `mime.TypeByExtension` which uses the OS's MIME database — on minimal VPS Linux images, `.js` may not be registered.

**How to avoid:**
Use `http.FileServer(http.FS(...))` or `http.ServeFileFS` for all static assets — these handle content-type automatically. If writing a custom handler, set content-type explicitly:

```go
ext := filepath.Ext(r.URL.Path)
ct := mime.TypeByExtension(ext)
if ct == "" {
    // Fallback for minimal OS MIME databases
    switch ext {
    case ".js":   ct = "application/javascript"
    case ".css":  ct = "text/css; charset=utf-8"
    case ".html": ct = "text/html; charset=utf-8"
    }
}
w.Header().Set("Content-Type", ct)
```

**Warning signs:**
- Dashboard HTML loads but has no styling or interactivity
- Browser DevTools Network tab shows `.css` files with `Content-Type: application/octet-stream`
- Browser console shows "Refused to apply style from ... as it has MIME type text/plain"

**Phase to address:** Phase 3 (dashboard scaffold) — verify content-type headers for all asset types in the phase acceptance test.

---

### Pitfall M14: uPlot / Alpine.js Bundled from CDN — Binary No Longer Works Offline

**What goes wrong:**
`index.html` references `<script src="https://cdn.jsdelivr.net/npm/uplot@1.6.31/dist/uPlot.iife.min.js">`. The CDN is fast and the file is reliably available. But jaimito's core value proposition is running on a VPS as a self-contained binary. If the VPS loses internet access (firewall rule change, ISP outage), the dashboard becomes non-functional. Worse, if the CDN removes the specific version or the file hash changes, the dashboard silently breaks even with internet access.

**Why it happens:**
CDN links are convenient for initial development and testing. Developers mean to embed the files "later" but the debt accumulates.

**How to avoid:**
Download and vendor the JS libraries into `web/dashboard/vendor/` at project setup time, then embed them:

```
web/dashboard/vendor/uplot.min.js    (~40KB, from uPlot releases)
web/dashboard/vendor/alpine.min.js   (~15KB, from Alpine.js releases)
```

Reference them as relative paths in HTML:

```html
<script src="/dashboard/vendor/uplot.min.js"></script>
<script src="/dashboard/vendor/alpine.min.js"></script>
```

Pin the exact version in a `web/dashboard/vendor/VERSIONS` file for auditability. Total overhead: ~55KB embedded in binary.

**Warning signs:**
- Dashboard requires network to load any JS or CSS
- Disabling network on VPS causes blank/broken dashboard
- `<script src="https://...">` tags visible in `index.html`

**Phase to address:** Phase 3 (dashboard scaffold) — vendor all frontend dependencies before writing the first component. Never merge HTML that references external CDN URLs.

---

### Pitfall M15: High-Frequency Metric Collection Starves the VPS of CPU/IO

**What goes wrong:**
A user defines 10 custom metrics, each with a 5-second interval. Every 5 seconds, jaimito spawns 10 shell processes. Each shell forks the target command. On a 1-core VPS, this is 2 shell + 2 command processes running simultaneously, plus jaimito's existing goroutines. The VPS load average climbs. The services being monitored (nginx, postgres) become slower. jaimito's monitoring causes the very degradation it's meant to detect.

**Why it happens:**
Each metric's interval feels reasonable in isolation. The aggregate load is only visible when many metrics run simultaneously. A naively parallel collector runs all metrics at once when intervals align.

**How to avoid:**
- Set a minimum collection interval of 30 seconds in config validation — reject intervals below this.
- Run metric collections sequentially (one at a time, not concurrently) by default. Only allow parallel collection if explicitly configured.
- Add a configurable `max_concurrent_collections` limit (default: 1).
- Stagger initial collection times: metric N starts at `N * (interval / num_metrics)` offset to spread the load.

**Warning signs:**
- VPS load average spikes every N seconds where N is the GCD of all collection intervals
- `top` shows repeated short-lived `sh` and command processes
- Metrics for system load show artificially high values (jaimito is inflating the metric it measures)

**Phase to address:** Phase 1 (metric collector) — implement sequential-by-default collection and minimum interval validation before adding any custom metric support.

---

### Pitfall M16: adlio/schema Migration Ordering — New Metrics Tables Must Not Break Existing Data

**What goes wrong:**
jaimito uses `adlio/schema` with numbered SQL migration files (`001_initial.sql`, `002_nullable_title.sql`). The next migration for v2.0 must be `003_metrics.sql`. If a developer accidentally names it `002_metrics.sql` (confusing the naming convention) or introduces a migration that references the `messages` table with a constraint jaimito already has data for, the migration fails on existing deployments. The service fails to start. The migration is irreversible.

**Why it happens:**
Migration numbering is easy to get wrong when multiple developers work in parallel (not an issue for solo projects, but worth noting). More commonly, the new migration adds a NOT NULL column to `metric_readings` with no DEFAULT, then the 7-day purge runs and tries to read that column from rows inserted before the migration — which have NULL in that column.

**How to avoid:**
- Always use the next sequential number: currently `003`.
- Never add NOT NULL columns without a DEFAULT to tables that may already exist (they won't — `metric_readings` is new in v2.0, but `api_keys` and `messages` are not).
- Test the migration on a copy of a production database (not just an empty dev database).
- The `003_metrics.sql` migration must only CREATE TABLE — no ALTER TABLE on existing tables unless absolutely required.

**Warning signs:**
- Service fails to start with "schema migration failed" after upgrade
- `adlio/schema` migration tracker shows migration partially applied
- `sqlite3 jaimito.db .tables` shows metrics tables missing after upgrade

**Phase to address:** Phase 2 (schema + storage) — write `003_metrics.sql`, test on a DB that already has v1.x data (messages + api_keys populated).

---

### Pitfall M17: Dashboard Accessible on Public Interface — Sensitive VPS Data Exposed

**What goes wrong:**
jaimito's HTTP server binds to `127.0.0.1:8080` by default. When a reverse proxy (nginx/caddy) exposes jaimito's API to the internet for the webhook endpoint, the admin may configure the proxy to forward all paths — including `/dashboard`. Disk usage, memory, uptime, Docker container names, and custom metric data are now publicly accessible without authentication.

**Why it happens:**
The v1.0 server served only `/api/v1/notify` (authenticated) and `/api/v1/health` (unauthenticated). Adding `/dashboard` (unauthenticated by design — "localhost only via Tailscale") creates a new unauthenticated surface that existing reverse proxy configs may inadvertently expose.

**How to avoid:**
- Serve the dashboard on a separate port (e.g., `127.0.0.1:8081`) distinct from the API port (`127.0.0.1:8080`). Reverse proxy only exposes port 8080. Dashboard is only reachable locally or via Tailscale by connecting to port 8081.
- Alternatively, add a chi middleware that checks `r.RemoteAddr` for loopback — reject non-loopback requests to `/dashboard` with 403.
- Document in `DEPLOY.md`: "The dashboard port (8081) must NOT be exposed in your reverse proxy configuration."

**Warning signs:**
- `curl https://yourdomain.com/dashboard` returns the dashboard HTML (it should 404 or 403)
- Nginx/Caddy config uses `proxy_pass http://127.0.0.1:8080/` (all paths proxied)
- Dashboard port exposed in `firewall-cmd --list-ports` or `ufw status`

**Phase to address:** Phase 3 (dashboard scaffold) — implement separate port or loopback-only middleware on the first day of dashboard work, before any data is served.

---

## v1.1 Setup Wizard Pitfalls (bubbletea TUI)

These pitfalls addressed the interactive `jaimito setup` bubbletea TUI wizard. Kept for reference during ongoing development.

---

### Pitfall W1: Bubbletea Hangs When Stdin is a Pipe (curl | bash)

**What goes wrong:**
When `install.sh` runs via `curl -sL ... | bash`, bash's stdin is occupied by the pipe carrying the script. Any program launched from that script that reads from stdin — including a bubbletea TUI — reads from the pipe (already exhausted or closed) instead of the keyboard. The TUI either hangs waiting for input that never comes or immediately receives EOF and exits without rendering anything.

**Why it happens:**
Bash inherits stdin from the calling shell. With `curl | bash`, stdin is the curl pipe. Child processes (`jaimito setup`) inherit that same stdin. Bubbletea opens stdin for keyboard input by default, so it gets the exhausted pipe, not the terminal.

**How to avoid:**
Redirect stdin from `/dev/tty` explicitly in `install.sh` when calling `jaimito setup`:

```bash
jaimito setup --config "$CONFIG_FILE" < /dev/tty
```

This forces jaimito's stdin to be the real terminal device, bypassing the pipe. Additionally, bubbletea should detect non-interactive stdin **before** launching the program:

```go
if !term.IsTerminal(int(os.Stdin.Fd())) {
    fmt.Fprintln(os.Stderr, "jaimito setup requiere una terminal interactiva.")
    os.Exit(1)
}
```

The `< /dev/tty` redirect in `install.sh` must come first — the `IsTerminal` check is a fallback for non-pipe non-TTY cases.

**Warning signs:**
- TUI renders nothing and exits immediately when run from install.sh
- Works fine when running `jaimito setup` directly in a terminal
- `echo $?` shows 0 (stdin read EOF gracefully) or program hangs indefinitely

**Phase to address:** Phase 1 (cobra command scaffolding) — the `< /dev/tty` redirect and the `IsTerminal` guard must be implemented and tested before any TUI step work begins.

---

### Pitfall W2: Bubbletea Output is Invisible or Unstyled When Stdout is Redirected

**What goes wrong:**
Even with the `< /dev/tty` fix for stdin, if anything in the calling script redirects stdout (e.g., `jaimito setup > logfile`), bubbletea renders the TUI to the redirected file instead of the terminal. The user sees nothing. Additionally, lipgloss loses color support because `termenv` detects stdout is not a terminal and strips all ANSI codes — the wizard renders as plain unstyled text.

**Why it happens:**
Bubbletea writes TUI output to stdout by default. `termenv` (which powers lipgloss) auto-detects terminal capabilities from the output file descriptor. A non-TTY stdout means no colors, no box-drawing characters. This is a documented known issue in bubbletea issue #860, with no automatic fix from the framework.

**How to avoid:**
Open `/dev/tty` explicitly for TUI output when stdout is not a TTY, and set the lipgloss renderer to match:

```go
var output *os.File
if !term.IsTerminal(int(os.Stdout.Fd())) {
    tty, err := os.OpenFile("/dev/tty", os.O_WRONLY, 0)
    if err == nil {
        output = tty
        defer tty.Close()
        lipgloss.SetDefaultRenderer(lipgloss.NewRenderer(tty))
    }
} else {
    output = os.Stdout
}
p := tea.NewProgram(model, tea.WithInput(ttyIn), tea.WithOutput(output))
```

For jaimito's case install.sh does not redirect stdout, so this is lower priority — but the pattern must be known before adding `tea.WithOutput()`.

**Warning signs:**
- Colors work in direct invocation but not inside install.sh when stdout is piped
- TUI box drawing characters appear as garbage in a log file
- lipgloss renders plain text despite a color terminal being connected via `/dev/tty`

**Phase to address:** Phase 1 — implement and test inside a simulated `curl | bash` flow before building wizard steps.

---

### Pitfall W3: Stale Async Validation Response Applied to Wrong Step State

**What goes wrong:**
The user types a bot token and triggers validation (a `tea.Cmd` that calls the Telegram API). Before the response arrives, the user presses Escape to go back and retypes the token. When the first validation response finally arrives, `Update()` applies it to the current model state — which now belongs to a different input. Result: the validation result from the old token is displayed as if it belongs to the new one. This can falsely show a bad token as valid or a good token as invalid.

**Why it happens:**
Bubbletea commands run as fire-and-forget goroutines. There is no built-in mechanism to cancel an in-flight command or correlate a response to the request that triggered it. Commands execute concurrently and return messages in non-deterministic order.

**How to avoid:**
Track a validation sequence number in the model. Each time a new validation starts, increment the counter. The validation `tea.Cmd` captures the expected counter value in its closure. In `Update()`, discard any validation response whose counter does not match the current model counter:

```go
type validationResultMsg struct {
    seq  int    // sequence number when this validation was launched
    err  error
    info BotInfo
}

// In Update():
case validationResultMsg:
    if msg.seq != m.validationSeq {
        return m, nil // stale response — discard
    }
    // apply result
```

**Warning signs:**
- Validation shows "valid" for a token the user already cleared
- Error from a previous token appears after entering a new one
- Spinner continues showing "validating" state longer than the actual network latency

**Phase to address:** Phase 2 (BotToken and ChannelGeneral validation steps) — implement the sequence counter pattern from the first step that does async validation, before any subsequent step is built.

---

### Pitfall W4: No Context Timeout in tea.Cmd Goroutines — Slow Network Hangs Wizard

**What goes wrong:**
A `tea.Cmd` that calls the Telegram API has no timeout. On a VPS with a misconfigured firewall or a temporarily slow Telegram endpoint, the HTTP call blocks indefinitely. The spinner keeps spinning, the user cannot advance or exit cleanly (Ctrl+C exits bubbletea but the goroutine leaks until the OS reclaims it — which may never happen if the server has a 30+ minute connection timeout).

**Why it happens:**
Bubbletea intentionally leaks command goroutines on shutdown (documented design decision to prevent hang on exit). `tea.Cmd` functions receive no context from the framework. Without an explicit timeout, HTTP calls run to completion regardless of whether the program has exited. This is especially bad for validation steps that run multiple times per session.

**How to avoid:**
Create a context with timeout **inside** every validation `tea.Cmd`:

```go
func validateTokenCmd(token string, seq int) tea.Cmd {
    return func() tea.Msg {
        ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
        defer cancel()
        bot, info, err := telegram.ValidateTokenWithInfo(ctx, token)
        return validationResultMsg{seq: seq, bot: bot, info: info, err: err}
    }
}
```

Do NOT attempt to pass a context from the cobra command through the model — bubbletea provides no mechanism for this. Use `context.WithTimeout` inside the Cmd closure.

**Warning signs:**
- Spinner hangs for >15 seconds during validation
- After Ctrl+C, `ps aux | grep jaimito` shows the process still running
- Wizard becomes unresponsive only on specific VPS network configurations

**Phase to address:** Phase 2 (all validation commands) — every `tea.Cmd` that calls external APIs must include a 15-second `context.WithTimeout`.

---

### Pitfall W5: Panic in a tea.Cmd Goroutine Leaves Terminal in Raw Mode

**What goes wrong:**
If a nil pointer dereference or other panic occurs inside a `tea.Cmd` function (e.g., accessing a nil `*bot.Bot` instance from a failed validation), bubbletea's panic recovery does not catch it. The terminal remains in raw mode: no echo, no line buffering, control characters appear as `^C` instead of triggering signals. The user's shell appears completely broken until they run `reset`.

**Why it happens:**
Bubbletea wraps `Update()` and `View()` in panic recovery, but `tea.Cmd` functions run in separate goroutines that are not wrapped. A panic in those goroutines propagates to the Go runtime and kills the process, bypassing the deferred terminal cleanup. This is a known architectural limitation.

**How to avoid:**
Wrap all `tea.Cmd` implementations in a deferred panic recovery that returns a safe error message:

```go
return func() tea.Msg {
    defer func() {
        if r := recover(); r != nil {
            // return error msg instead of crashing
        }
    }()
    // ...validation logic
}
```

Additionally, nil-check all values before use: the `*bot.Bot` returned from `ValidateToken` may be nil on error. Never dereference without checking.

**Warning signs:**
- Terminal becomes unresponsive after wizard exits during certain edge cases
- No error message shown before exit (process just dies)
- Reproducible when providing an invalid token that causes `bot.New()` to return nil

**Phase to address:** Phase 2 (validation commands) and Phase 3 (config writing) — any `tea.Cmd` that touches external state or can produce nil values needs panic recovery.

---

### Pitfall W6: os.WriteFile Does Not Guarantee 0600 Permissions on Existing Files

**What goes wrong:**
The config file is written with `os.WriteFile(path, data, 0600)`. However, Go's documentation states: "If the file does not exist, WriteFile creates it with permissions perm (before umask); otherwise WriteFile truncates it before writing, **without changing permissions**." If the file previously existed with permissions 0644 (e.g., copied from `config.example.yaml` by a previous install.sh), it remains 0644 after the wizard writes the bot token and API key into it.

Additionally, even for new files, the umask modifies the requested permissions. With the default root umask of `0022`, `0600 & ~0022 = 0600` (fine). But with a custom umask, the result diverges.

**Why it happens:**
Developers test with a fresh install (no prior config file) and get 0600. The case of "editing existing config" creates a world-readable secrets file that silently passes all local tests.

**How to avoid:**
Always call `os.Chmod` explicitly after writing, regardless of whether the file is new or existing:

```go
if err := os.WriteFile(path, data, 0600); err != nil {
    return err
}
return os.Chmod(path, 0600)
```

For directory creation (`/etc/jaimito/`), use `os.MkdirAll(dir, 0755)` followed by `os.Chmod(dir, 0755)` to guarantee permissions unaffected by umask.

**Warning signs:**
- `ls -la /etc/jaimito/config.yaml` shows `-rw-r--r--` (0644) after running setup on an existing config
- Bot token readable by any local user on the VPS
- gosec linter flags G306 on the WriteFile call without the accompanying Chmod

**Phase to address:** Phase 3 (config writing) — the `os.Chmod` call must be explicit and tested with a pre-existing 0644 file.

---

### Pitfall W7: Cobra PersistentPreRun Tries to Load Config Before It Exists

**What goes wrong:**
The existing `root.go` cobra command may call `config.Load(cfgPath)` in a `PersistentPreRun` or early in the `RunE` of subcommands. When `jaimito setup` runs before any config exists, this produces a "config file not found" error from the root command before the setup wizard even launches.

**Why it happens:**
The existing cobra subcommands (`serve`, `send`, `wrap`) all need a valid config. If a `PersistentPreRun` was added to validate config for all subcommands, `jaimito setup` inherits that hook and fails before the wizard can create the config.

**How to avoid:**
The `setup` command must skip all config-loading hooks. Either:
1. Do not put config loading in `PersistentPreRun` (current codebase does not appear to do this — verify before assuming it's safe)
2. Add a guard in the hook: if cobra's active command is `setup`, skip config loading

Reviewing `root.go` confirms config loading is only done inside individual command `RunE` functions, not in a `PersistentPreRun`. Verify this does not change when adding the setup command.

**Warning signs:**
- Running `jaimito setup` on a fresh system prints "config not found" before the wizard appears
- `jaimito setup` exits with code 1 immediately without displaying the welcome screen

**Phase to address:** Phase 1 (cobra command scaffolding) — verify cobra hooks do not interfere with setup before building any TUI.

---

### Pitfall W8: bot.New() Does Not Validate the Token — getMe Must Be Called Explicitly

**What goes wrong:**
`telegram.ValidateToken()` calls `bot.New(token)` and returns the bot instance as proof of validation. However, `go-telegram/bot`'s `bot.New()` only parses the token format and creates the client struct — it does NOT make a network call to confirm the token is valid. Validation passes with any syntactically correct but invalid token. The wizard advances to the next step showing "token valid", but `SendMessage` later fails.

**Why it happens:**
The existing `ValidateToken` function was written for startup validation where the server subsequently makes API calls. For the wizard, the validation step must be the explicit network check.

**How to avoid:**
The new `telegram.ValidateTokenWithInfo(ctx, token)` function designed for the wizard must call `bot.GetMe(ctx)` (or equivalent) after `bot.New()`:

```go
func ValidateTokenWithInfo(ctx context.Context, token string) (*bot.Bot, BotInfo, error) {
    b, err := bot.New(token)
    if err != nil {
        return nil, BotInfo{}, fmt.Errorf("invalid token format: %w", err)
    }
    me, err := b.GetMe(ctx)
    if err != nil {
        return nil, BotInfo{}, fmt.Errorf("token rejected by Telegram: %w", err)
    }
    return b, BotInfo{Username: me.Username, DisplayName: me.FirstName}, nil
}
```

**Warning signs:**
- Wizard accepts a token like `1234567890:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA` (valid format, invalid token)
- Step 1 shows "token valid" but test notification in the final step fails
- Validation passes with no network activity visible in `tcpdump`

**Phase to address:** Phase 2 (BotToken step) — write and test `ValidateTokenWithInfo` explicitly, not just `bot.New()`.

---

## v1.0 MVP Pitfalls (original)

These pitfalls addressed the core notification hub. Kept for reference during ongoing development.

---

### Pitfall 1: SQLite DEFERRED Transactions Ignore busy_timeout on Write Upgrades

**What goes wrong:**
You configure `PRAGMA busy_timeout = 5000` expecting SQLite to wait up to 5 seconds when the database is locked. But when a read transaction (BEGIN DEFERRED) needs to upgrade to a write, SQLite returns SQLITE_BUSY **immediately** — the timeout is not honoured. The dispatcher loop and the HTTP ingestor both try to write concurrently, causing one to fail silently or log a spurious error.

**Why it happens:**
Go's `database/sql` opens transactions as BEGIN DEFERRED by default. When two goroutines both start reading and then try to write (e.g., the dispatcher updating `status` while the ingestor inserts a new row), the one that arrives second cannot upgrade and fails at once. Developers set `busy_timeout` expecting it to fix all locking issues, but it only applies to situations where the lock is held by an external process, not to in-process upgrade conflicts.

**How to avoid:**
- Use `BEGIN IMMEDIATE` for any transaction that will write, not BEGIN DEFERRED. In Go: `db.BeginTx(ctx, &sql.TxOptions{})` does not help — use a raw `EXEC("BEGIN IMMEDIATE")` or a dedicated write connection.
- **Simpler for this project:** limit the write connection pool to `MaxOpenConns(1)` with a single writer goroutine and use read replicas for readers. This serialises all writes at the application level and eliminates the upgrade race entirely.
- Always set `PRAGMA busy_timeout = 5000` as a floor, but do not rely on it as the only concurrency guard.

**Warning signs:**
- Log lines containing `SQLITE_BUSY` or `database is locked` appearing intermittently under load.
- Dispatcher missing messages that are clearly in the queue.
- Tests pass in isolation but fail when run in parallel.

**Phase to address:** MVP (Phase 1) — get connection pool config right before writing any business logic.

---

### Pitfall 2: CGO Dependency Breaks Single-Binary Build Promise

**What goes wrong:**
The PRD promises a single statically-linked binary for easy deployment. `mattn/go-sqlite3` requires CGO (`CGO_ENABLED=1`), which means: the binary is not fully static, cross-compilation to `linux/arm64` from a `linux/amd64` host requires a cross-compiler, and Docker multi-stage builds fail silently when the builder image lacks `libsqlite3-dev`.

**Why it happens:**
Developers choose `mattn/go-sqlite3` because it is the most Googled Go SQLite library, without realising it embeds the SQLite C amalgamation via CGO.

**How to avoid:**
Choose `modernc.org/sqlite` (pure Go, CGO-free port) from the start. It supports WAL mode, achieves comparable performance for the expected load, and produces a truly static binary with `CGO_ENABLED=0`. (Already resolved in this project — documented for context.)

**Warning signs:**
- `go build` works locally but fails in CI with "cgo: C compiler not found."
- Binary size unexpectedly large from embedded C runtime.

**Phase to address:** MVP (Phase 1) — select the driver once; migrating later requires changing all query code.

---

### Pitfall 3: Telegram 429 Rate Limit Kills All Bot Notifications, Not Just Bursts

**What goes wrong:**
A burst of alerts triggers Telegram's rate limit. Telegram responds with HTTP 429 and a `retry_after` field. If the dispatcher does not respect `retry_after` and immediately retries, Telegram blacklists the bot IP for 30 seconds — **all** bot notifications fail during that window.

**How to avoid:**
- Parse `retry_after` from every 429 response and sleep exactly that duration plus jitter before re-queuing.
- As of Bot API 8.0 (November 2025), 429 responses include `adaptive_retry` — honour this field.
- Implement a per-dispatcher token bucket: max 1 message/second per chat, max 20/minute for groups.

**Phase to address:** MVP dispatcher (Phase 1). The rate limit will be hit in the first week of real use if cron jobs run at round minutes.

---

### Pitfall 4: Dispatcher Goroutine Loops Without Bounded Retry = Poison Pill Messages

**What goes wrong:**
A message with a malformed `targets_override` or an invalid chat_id gets enqueued. The dispatcher retries it repeatedly. If the retry logic has a bug, the message stays in `dispatching` forever, consuming dispatcher cycles on every poll iteration.

**How to avoid:**
- Add a hard maximum on `retry_count` (e.g., 10) that marks the message `failed` and stops dispatching.
- Log and move exhausted messages to a `dead_letter` status.
- Add a startup reclaim for rows stuck in `dispatching` for more than `2 * dispatch_timeout`.

**Phase to address:** MVP (Phase 1) — implement reclaim logic before going to production.

---

### Pitfall 5: No Request Body Size Limit on Webhook Allows Memory Exhaustion

**What goes wrong:**
Go's `net/http` reads request bodies on demand without a built-in size limit. A misconfigured client or malicious actor sends a 50MB payload to `POST /api/v1/notify`. The server buffers the entire body before parsing JSON, exhausting the 50MB memory budget.

**How to avoid:**
Wrap every request body with `http.MaxBytesReader(w, r.Body, 64*1024)` (64KB). Return 413 on overflow. Set `ReadTimeout` and `WriteTimeout` on the HTTP server.

**Phase to address:** MVP (Phase 1) — three lines of code at handler setup.

---

### Pitfall 6: Plain-Text Secrets in YAML Config Committed to Git

**What goes wrong:**
`config.yaml` contains `bot_token: "..."` and API keys. A developer does `git add .` and pushes. The Telegram bot token is now in git history permanently.

**How to avoid:**
- Ship `config.yaml.example` with placeholder values. Add `config.yaml` and `*.db` to `.gitignore` before the first commit.
- Enforce `0600` on `config.yaml` at startup.

**Phase to address:** Before first commit (pre-MVP).

---

### Pitfall 7: WAL File Grows Unboundedly Without Checkpointing

**What goes wrong:**
If the database is under constant write load and a long-running read transaction is open, SQLite cannot checkpoint. The WAL file grows to hundreds of megabytes.

**How to avoid:**
- Keep read transactions short — close before making network calls.
- Set `PRAGMA wal_autocheckpoint = 1000`.
- Periodically run `PRAGMA wal_checkpoint(PASSIVE)` from a background goroutine.

**Phase to address:** Phase 2 (after MVP is stable).

---

### Pitfall 8: Notification Flooding When the Monitored Service Has a Loop Bug

**What goes wrong:**
A cron job has a bug and runs in a tight loop, sending 60+ notifications per minute, triggering Telegram rate limiting and notification fatigue.

**How to avoid:**
- Implement deduplication windows per `dedupe_key`.
- Add per-channel rate limiting.
- For `jaimito wrap`: suppress repeated failures.

**Phase to address:** Phase 2 (deduplication + rate limiting).

---

### Pitfall 9: CLI Tool API Key Stored in Shell History or Process List

**What goes wrong:**
Users invoke `jaimito send -k sk-abc123 "message"` with the API key as a CLI argument. The key appears in `~/.bash_history` and `ps aux`.

**How to avoid:**
Read API key from `JAIMITO_API_KEY` env var or a config file, not a CLI flag. (Already implemented — documented for context.)

**Phase to address:** MVP CLI (Phase 1).

---

### Pitfall 10: Self-Monitoring Bootstrap Problem

**What goes wrong:**
jaimito runs on the same VPS it monitors. If jaimito itself crashes, there is no notification.

**How to avoid:**
Use systemd `Restart=on-failure` and `WatchdogSec=30`. Document as known limitation — external monitoring (UptimeRobot, second VPS) is the real solution.

**Phase to address:** MVP deployment (Phase 1).

---

### Pitfall 11: Telegram MarkdownV2 Escaping Breaks Message Rendering

**What goes wrong:**
MarkdownV2 requires escaping 18 special characters. Alert messages naturally contain these characters. Unescaped characters cause the entire message to fail with 400 Bad Request — the message is lost.

**How to avoid:**
Implement `EscapeMarkdownV2(s string) string` or switch to `parse_mode: HTML`. Test with pathological inputs containing `.`, `-`, `_`, `(`, `)`, `!`.

**Phase to address:** MVP dispatcher (Phase 1).

---

## Technical Debt Patterns

| Shortcut | Immediate Benefit | Long-term Cost | When Acceptable |
|----------|-------------------|----------------|-----------------|
| Skip `BEGIN IMMEDIATE` — use default DEFERRED transactions | Less code | Intermittent SQLITE_BUSY under concurrent load | Never |
| Use `mattn/go-sqlite3` (CGO) | More documentation | Cross-compilation requires toolchain; non-static binary | Acceptable if ARM not planned and CI already has CGO |
| Store secrets directly in `config.yaml` | Faster initial setup | Accidental git commit exposure | Never |
| No `MaxBytesReader` on webhook handler | Fewer lines | OOM from large payloads | Never |
| Hardcode spinner style in wizard | Simpler initial code | Harder to theme later | Acceptable for v1.1 |
| Use `SetupData` as shared pointer mutated directly by steps | Simpler than message-passing | Steps are not independently testable | Acceptable if teatest coverage added |
| Skip context propagation to bot instance reuse across steps | Less plumbing | Bot instance can go stale if token changes mid-flow | Never — implement token refresh on token change |
| Skip atomic write (write-then-rename) for config | Simpler code | Power failure mid-write leaves corrupted config | Acceptable for v1.1 root-level writes (low risk) |
| Single 15s timeout for all Telegram API calls | Less configuration | `getUpdates` can be slower than `getMe` | Acceptable |
| Use Tailwind Play CDN instead of pre-compiled CSS | Zero build step | Requires internet at runtime; 300KB runtime overhead; offline dashboard broken | Never for production binary |
| Run metric collection without per-command timeout | Simpler code | Goroutine accumulation; hung collectors; shutdown timeout | Never |
| Alert on every threshold crossing without state tracking | Trivial to implement | Alert storms; Telegram rate limit triggered; notification fatigue | Never |
| Reference frontend libraries from CDN in embedded HTML | Easy initial development | Binary requires internet; version drift; CDN outages break dashboard | Never — vendor all JS/CSS |
| Collect metrics in parallel goroutines without concurrency limit | Faster collection | CPU/IO spikes; self-inflating system load metrics | Acceptable only if max_concurrent explicitly capped |
| Place embed directive in a package that does not own the web/ dir | Follows package conventions | Build fails with "no matching files found" — must restructure | Never |

---

## Integration Gotchas

| Integration | Common Mistake | Correct Approach |
|-------------|----------------|------------------|
| Telegram Bot API | Using `parse_mode: MarkdownV2` without escaping all 18 special characters | Implement `EscapeMarkdownV2()` or switch to HTML |
| Telegram Bot API | Ignoring `retry_after` in 429 responses | Parse `retry_after`; store `next_retry_at` in SQLite |
| Telegram Bot API | Not handling "bot was blocked" (403) or "chat not found" (400) — retrying forever | Detect permanent errors by code; mark `failed` immediately |
| Telegram Bot API (wizard) | `bot.New()` does NOT call getMe — token appears valid but is rejected | Call `bot.GetMe(ctx)` explicitly after `bot.New()` |
| `go-telegram/bot` + `GetChat` | Chat_id where bot hasn't been added returns 400, not timeout | Wrap with message: "Asegurate de que el bot esté en el grupo" |
| `gopkg.in/yaml.v3` marshal | Struct with unexported fields silently omits them | Verify all `config.Config` fields have yaml struct tags |
| `bubbles/textinput` + `Focus()` | Forgetting to call `Focus()` means input never captures keystrokes | Call `ti.Focus()` in `Init()` or on step transition |
| cobra `PersistentPreRun` | Root hook loading config runs before `jaimito setup` (config doesn't exist yet) | Guard: skip config loading if active command is `setup` |
| SQLite (WAL) | Opening database file with multiple processes simultaneously | Use WAL mode; `_busy_timeout=5000` in DSN; single process owns the file |
| SQLite (WAL) | Not setting `_foreign_keys=on` in DSN | Add `?_foreign_keys=on&_journal_mode=WAL&_busy_timeout=5000` to DSN |
| net/http webhook | Not returning 202 immediately and processing synchronously | Enqueue to SQLite, return 202, dispatch asynchronously |
| `os/exec` + metric commands | Using `exec.Command("sh", "-c", cmd)` without context timeout | Always use `exec.CommandContext` with capped timeout |
| `os/exec` + Docker commands | Killing parent sh leaves child docker processes running | Set `cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}` and kill process group |
| `go:embed` | Placing `//go:embed` directive in package that does not own the path | Move embed directive to a package at or above the embedded path |
| `go:embed` | Files named with leading `.` or `_` silently excluded | Name embedded files without leading dots or underscores |
| `http.FileServer` + SPA | Unknown client-side routes return 404 instead of index.html | Implement SPA fallback handler; or use hash-based routing to avoid this entirely |
| adlio/schema migrations | Adding v2.0 migration with wrong number (skipping or reusing a number) | Always use next sequential number; test migration on a DB with existing v1.x data |

---

## Performance Traps

| Trap | Symptoms | Prevention | When It Breaks |
|------|----------|------------|----------------|
| Full table scan on `messages` for dispatch polling | Dispatcher slows as queue grows | Index `(status, next_retry_at)` at schema creation | Noticeable at ~10,000 rows |
| WAL file not checkpointed | Reads slow; `-wal` file grows to hundreds of MB | Periodic `PRAGMA wal_checkpoint(PASSIVE)`; keep reads short | Within days of continuous write load |
| Dispatcher holds read transaction across Telegram HTTP call | Blocks WAL checkpoint; latency spikes | Read batch → close → dispatch → open new transaction to update | Whenever Telegram is slow |
| Blocking Telegram API call in wizard `Update()` directly | TUI freezes during validation | Only call external APIs inside `tea.Cmd` | Immediately — first validation |
| Multiple simultaneous validation requests from rapid typing | N API calls fired per keystroke | Debounce: only fire validation cmd on Enter, not each keystroke | As soon as user types fast |
| Re-rendering entire wizard with expensive lipgloss operations | High CPU during spinner animation | Keep `View()` lightweight; cache rendered strings for static content | At ~10 animation ticks/second |
| Metric collector holds SQLite transaction open during shell command execution | Message delivery latency spikes at every collection interval | Collect readings outside any transaction; write in a single batch transaction after all commands complete | At any collection interval |
| High-frequency metric collection (intervals <30s) spawning many shell processes | VPS load average inflated; monitored processes show degraded performance | Enforce minimum 30s interval; run collections sequentially by default | With 5+ metrics at 10s intervals on a 1-core VPS |
| `metric_readings` table not indexed by `(metric_name, collected_at)` | Dashboard API slow when fetching historical readings | Add index at schema creation | Noticeable at ~50,000 rows (7 days × 5 metrics × 5min intervals = 10,080 rows; custom metrics can push past 50k quickly) |
| Full table scan for retention purge without index on `collected_at` | Purge job slow; holds write lock longer than necessary | Index `collected_at` or use the `(metric_name, collected_at)` composite index | At ~50,000 rows |

---

## Security Mistakes

| Mistake | Risk | Prevention |
|---------|------|------------|
| API key comparison with `==` (non-constant time) | Timing oracle enables key enumeration | Use `hmac.Equal([]byte(provided), []byte(expected))` |
| Config written as 0644 (world-readable) | Bot token and API key readable by all users | `os.Chmod(path, 0600)` after write, regardless of previous permissions |
| Parent directory `/etc/jaimito/` created with 0777 | Other users can replace or read config | Create with `0755` and verify with `os.Chmod` |
| Bot token logged via `slog` during validation | Token appears in system logs | Never log the token value; log only "token validated" or "validation failed" |
| Bot token in process environment visible in `/proc/PID/environ` | Leaked in crash dumps | Load from file at startup; clear from env after reading |
| Webhook endpoint on public interface without IP restriction | Abuse from public internet | Bind to `127.0.0.1` by default; document reverse proxy requirement |
| Config file with 0644 permissions allows anyone to read metric commands | Local user can see all custom monitoring logic and potentially modify it | Enforce 0600 on config at startup; refuse to load if permissions are too permissive |
| Dashboard endpoint exposed through reverse proxy | VPS disk usage, memory, uptime, container names visible publicly | Serve dashboard on separate port not exposed in reverse proxy config |
| Metric command values containing user-controlled data interpolated into shell string | Command injection via config | Never interpolate dynamic data into metric command strings; treat config as static trusted input |

---

## UX Pitfalls

| Pitfall | User Impact | Better Approach |
|---------|-------------|-----------------|
| Showing full bot token in pre-fill for "edit existing config" | Token exposed if screen recorded | Mask as `****...xxxx` (last 4 chars visible only) |
| Not confirming API key was copied before advancing | User loses credentials, must re-run setup | Block advancement until user types "s" — already in spec, must not be simplified away |
| Silently advancing to next step immediately after validation success | User misses bot name / chat title confirmation | Show success state for 1-2 seconds or require Enter to advance |
| No way to cancel during async validation | User stuck watching spinner on slow network | Bubbletea handles Ctrl+C globally — ensure spinner step doesn't block event loop |
| Step "Volver a revisar" forces re-validation of all steps | Frustrating for changing one field | Jump-to-step selector is in spec — implement it, do not simplify away |
| Error messages truncated by terminal width | User cannot read full Telegram API error | Use `lipgloss.NewStyle().Width(termWidth - 4).Render(err.Error())` to wrap |
| `jaimito wrap` sends notification on every success | Cron noise; notification fatigue | Default to notify-on-failure only; add `--always` flag |
| Long truncated `body` from piped commands overwhelms Telegram | Telegram 4096-char limit; truncated context | Truncate at 3800 chars and append "… [truncated, N total chars]" |
| Dashboard shows stale metrics with no "last updated" timestamp | User cannot tell if metrics are current or collector is broken | Show `collected_at` timestamp on each metric row; highlight if >2× interval has elapsed since last reading |
| Alert notification has no link to dashboard | User receives "disk at 92%" with no context | Include current value, threshold, and "check dashboard at http://..." in alert message |
| `jaimito status` CLI shows all-time max/min instead of current value | Misleading at a glance | Default to showing most recent reading; use `--history` flag for time series |

---

## "Looks Done But Isn't" Checklist

### Metrics & Dashboard (v2.0)
- [ ] **Command timeout:** Every metric command is wrapped in `exec.CommandContext` — verify with a `sleep 60` test command that collection times out, does not hang
- [ ] **Process group kill:** Docker and multi-process commands leave no orphaned children — check with `ps aux --forest` after a forced timeout
- [ ] **Write-lock separation:** Metric inserts do not overlap with active dispatcher transactions — add timing logs to verify write latency during collection intervals
- [ ] **Embedded asset completeness:** `TestEmbeddedAssets` verifies index.html, tailwind.css, uplot.min.js, alpine.min.js are all present in the embedded FS
- [ ] **Offline operation:** Dashboard fully functional with `iptables -P OUTPUT DROP` (no internet) — all assets load from embedded FS
- [ ] **Alert state machine:** Two consecutive collections above threshold fire exactly ONE alert, not two — confirm in integration test
- [ ] **Alert flapping:** Metric oscillating between 89% and 91% (threshold 90%) with no recovery threshold fires at most 1 alert per hour
- [ ] **Migration on existing DB:** `003_metrics.sql` applied to a DB with existing messages and api_keys — service starts without error, existing data intact
- [ ] **Dashboard port isolation:** `curl http://localhost:8080/dashboard` returns 404 (API port); `curl http://localhost:8081/dashboard` returns 200 (dashboard port)
- [ ] **Content-Type headers:** Browser DevTools confirms `.js` → `application/javascript`, `.css` → `text/css` for all embedded assets
- [ ] **Retention purge:** After running purge with timezone set to UTC-5, no readings younger than 7 actual UTC days are deleted
- [ ] **Graceful shutdown:** `systemctl stop jaimito` completes within 10 seconds — no "Timeout: killing process" in journalctl
- [ ] **Sequential collection:** With 5 metrics defined, `ps aux` never shows more than 1 `sh` process spawned by jaimito at a time (default sequential mode)

### Setup Wizard (v1.1)
- [ ] **stdin detection:** Verify `< /dev/tty` redirect in install.sh is before `jaimito setup` call — check `bash -x install.sh` output
- [ ] **Terminal detection:** `term.IsTerminal(int(os.Stdin.Fd()))` runs before `tea.NewProgram()`, not inside `Init()`
- [ ] **Permissions:** `ls -la /etc/jaimito/config.yaml` after setup on a pre-existing 0644 file must show `-rw-------` (0600)
- [ ] **Telegram validation:** Providing a valid-format but invalid token must fail — not silently pass because `bot.New()` succeeded
- [ ] **Config round-trip:** Generated YAML passes through `config.Load()` before writing — not just marshalled
- [ ] **Sequence counter:** Fire two rapid validations — confirm only the last result is applied
- [ ] **Ctrl+C during validation:** Press Ctrl+C while spinner is running — confirm terminal echo is restored
- [ ] **Existing config:** Run `jaimito setup` when config already exists — must offer the three-option menu, not overwrite silently
- [ ] **install.sh test:** Run `curl -sL [url] | bash` — confirm TUI appears (not hangs, not crashes)
- [ ] **Non-root execution:** Run setup as non-root attempting to write to `/etc/jaimito/` — must fail clearly, not partially write

### Core Notification Hub (v1.0)
- [ ] **SQLite WAL mode:** Verify `PRAGMA journal_mode` returns `wal` after opening.
- [ ] **Telegram rate limit:** Confirm dispatcher reads `retry_after` from 429 responses and stores in `next_retry_at`.
- [ ] **Retry exhaustion:** Messages with `retry_count >= max_retries` transition to `failed`.
- [ ] **Stuck dispatcher reclaim:** Kill dispatcher mid-dispatch; verify message is reclaimed on restart.
- [ ] **Signal handling:** `SIGTERM` causes clean shutdown; no WAL corruption.
- [ ] **Webhook security:** Request with no `Authorization` header returns 401.
- [ ] **Body size limit:** 10MB POST returns 413 without crashing.
- [ ] **CLI exit codes:** `jaimito send` with server stopped exits with code 1.
- [ ] **Config permissions:** Service refuses to start if `config.yaml` has mode 0644.
- [ ] **MarkdownV2 escaping:** Message with `_`, `.`, `-`, `(`, `)`, `!` arrives in Telegram without 400 error.

---

## Recovery Strategies

| Pitfall | Recovery Cost | Recovery Steps |
|---------|---------------|----------------|
| Terminal left in raw mode after wizard panic | LOW | User runs `reset` in shell; add note to README troubleshooting |
| Config written with wrong permissions | LOW | `chmod 0600 /etc/jaimito/config.yaml`; add to troubleshooting docs |
| Bot token exposed in system logs | MEDIUM | Rotate bot token via BotFather; revoke old; update config |
| Stale validation shown to user | LOW | User retries from same step — no data corruption, cosmetic only |
| Config written as partial YAML (crash mid-write) | MEDIUM | Delete corrupted file; re-run `jaimito setup` |
| curl \| bash install hangs on TUI | MEDIUM | Kill process; run `bash install.sh` directly; document limitation |
| Secrets committed to git | HIGH | Rotate bot token; rotate API key; `git filter-repo` to purge; force-push; notify forks |
| SQLite WAL corruption from hard kill | MEDIUM | Stop service; `sqlite3 jaimito.db "PRAGMA integrity_check"`; restore from backup |
| Stuck messages in `dispatching` status | LOW | `UPDATE messages SET status='queued', retry_count=0 WHERE status='dispatching'`; restart |
| Telegram bot banned (repeated 429 abuse) | MEDIUM | Wait 1-24h for unban; implement rate limiting; test with `getMe` |
| Alert storm — unlimited identical alerts sent | MEDIUM | Deploy with state machine fix; backfill `alert_state` table; user likely muted Telegram chat already |
| Goroutine leak from hung metric commands | MEDIUM | `kill -9` jaimito; redeploy with `WaitDelay` fix; check for orphaned sh processes with `pkill -f 'sh -c'` |
| Dashboard assets 404 due to embed exclusion | LOW | Add `TestEmbeddedAssets` test; fix file naming; redeploy |
| Migration failed on existing DB | HIGH | Do NOT attempt to re-run migration manually; restore from backup; fix migration SQL; redeploy |

---

## Pitfall-to-Phase Mapping

| Pitfall | Prevention Phase | Verification |
|---------|------------------|--------------|
| Shell command injection via config | v2.0 Phase 1: Metric collector | Config file permission enforced on startup; dynamic data never interpolated into commands |
| os/exec no timeout — hanging collector | v2.0 Phase 1: Metric collector | `sleep 60` command times out; no goroutine accumulation after 5 collections |
| Child process orphans (Docker) | v2.0 Phase 1: Metric collector | `ps aux --forest` shows no orphaned children after forced timeout |
| Metric writes block message delivery | v2.0 Phase 1 + Phase 2 | Notification latency unchanged after adding metrics; collect-then-write pattern enforced |
| go:embed path resolution | v2.0 Phase 3: Dashboard scaffold | `go build` succeeds; embedded asset test passes |
| go:embed silent exclusion of dot/underscore files | v2.0 Phase 3: Dashboard scaffold | `TestEmbeddedAssets` asserts all critical files present |
| SPA routing 404 | v2.0 Phase 3: Dashboard scaffold | Direct URL navigation to all dashboard routes returns 200 (or hash routing chosen to avoid issue) |
| Tailwind Play CDN embedded | v2.0 Phase 3: Dashboard scaffold | Dashboard loads fully with network disabled |
| Alert storm (no state machine) | v2.0 Phase 4: Threshold alerting | Integration test: 5 consecutive threshold crossings fire exactly 1 alert |
| Alert flapping (no hysteresis) | v2.0 Phase 4: Threshold alerting | Oscillating metric fires at most 1 alert/hour |
| Collector goroutine leak on shutdown | v2.0 Phase 1: Metric collector | `systemctl stop jaimito` completes in <10s |
| Metric timestamp timezone mismatch | v2.0 Phase 2: Schema + storage | All `collected_at` stored as UTC; purge test with TZ=America/New_York |
| Wrong content-type headers for JS/CSS | v2.0 Phase 3: Dashboard scaffold | Browser DevTools network tab shows correct MIME types |
| Frontend JS/CSS from CDN (not embedded) | v2.0 Phase 3: Dashboard scaffold | Binary passes offline test; no external script tags in index.html |
| VPS load inflation from metric collection | v2.0 Phase 1: Metric collector | Sequential-by-default collection; minimum 30s interval enforced in config validation |
| Migration breaks existing data | v2.0 Phase 2: Schema + storage | Migration tested on DB with existing messages and api_keys |
| Dashboard exposed through reverse proxy | v2.0 Phase 3: Dashboard scaffold | Dashboard on separate port; DEPLOY.md updated with warning |
| Stdin pipe hang (curl\|bash) | v1.1 Phase 1: Cobra command scaffold | `echo "" \| ./jaimito setup` must print error and exit 1 |
| Bubbletea output invisible/unstyled | v1.1 Phase 1: Cobra command scaffold | Test inside subshell with redirected stdout |
| Stale async validation response | v1.1 Phase 2: Bot token + channel validation | Fire two rapid validations; only last result applies |
| Context/goroutine leak on Ctrl+C | v1.1 Phase 2: All validation tea.Cmd functions | After Ctrl+C, no lingering process; timeout after max 15s |
| Panic in tea.Cmd leaves raw terminal | v1.1 Phase 2: Validation commands | Inject nil bot; confirm graceful error, not crash |
| Config written with wrong permissions | v1.1 Phase 3: Config writing | `ls -la` after write confirms 0600; test with pre-existing 0644 file |
| Umask overrides intended permissions | v1.1 Phase 3: Config writing | Test with `umask 0` and `umask 0077` before running setup |
| Cobra PersistentPreRun loads missing config | v1.1 Phase 1: Cobra command scaffold | Run `jaimito setup` on fresh system; no "config not found" error |
| bot.New() lazy init misidentified as validation | v1.1 Phase 2: Telegram validate commands | Invalid token must return error, not succeed |
| SQLite DEFERRED transaction SQLITE_BUSY | v1.0 Phase 1: Database layer | Concurrent integration test: 2 goroutines write simultaneously |
| CGO cross-compilation | v1.0 Phase 1: Build setup | `make build-arm64` succeeds on amd64 host without extra toolchain |
| Telegram 429 rate limit | v1.0 Phase 1: Telegram dispatcher | Send 35 messages in 1 second; verify backoff behaviour |
| Dispatcher poison pill / stuck messages | v1.0 Phase 1: Queue + dispatcher | Kill process mid-dispatch; restart; verify delivery or clean failure |
| Webhook body size / timeout | v1.0 Phase 1: HTTP server setup | 10MB POST returns 413 without crash |
| Plain-text secrets in git | Pre-v1.0 Phase 1: Project setup | `git log --all -- config.yaml` returns empty |
| MarkdownV2 escaping | v1.0 Phase 1: Telegram dispatcher | Integration test with special-character payload |

---

## Sources

**Metrics & Dashboard (v2.0):**
- [Understanding command injection vulnerabilities in Go — Snyk](https://snyk.io/blog/understanding-go-command-injection-vulnerabilities/) — HIGH confidence (official security research)
- [os/exec: CommandContext does not respect context timeout — golang/go issue #57129](https://github.com/golang/go/issues/57129) — HIGH confidence (official Go tracker)
- [os/exec: CommandContext with multiple subprocesses not canceled — golang/go issue #22485](https://github.com/golang/go/issues/22485) — HIGH confidence (official Go tracker)
- [os/exec resource leak on exec failure — golang/go issue #69284](https://github.com/golang/go/issues/69284) — HIGH confidence (official Go tracker, 2024)
- [SQLite concurrent writes and "database is locked" errors — tenthousandmeters.com](https://tenthousandmeters.com/blog/sqlite-concurrent-writes-and-database-is-locked-errors/) — HIGH confidence
- [The Write Stuff: Concurrent Write Transactions in SQLite — oldmoe.blog 2024](https://oldmoe.blog/2024/07/08/the-write-stuff-concurrent-write-transactions-in-sqlite/) — HIGH confidence
- [embed package — pkg.go.dev](https://pkg.go.dev/embed) — HIGH confidence (official)
- [How to Use //go:embed — blog.carlana.net](https://blog.carlana.net/post/2021/how-to-use-go-embed/) — MEDIUM confidence (practitioner, 2021, still accurate)
- [Embedded File Systems: Using embed.FS in Production — DEV Community](https://dev.to/rezmoss/embedded-file-systems-using-embedfs-in-production-89-2fpa) — MEDIUM confidence
- [Fileserver example for an SPA — go-chi/chi issue #611](https://github.com/go-chi/chi/issues/611) — MEDIUM confidence (community pattern, widely referenced)
- [Play CDN not for production — Tailwind CSS official docs](https://tailwindcss.com/docs/installation/play-cdn) — HIGH confidence (official)
- [Stop Using Tailwind CDN — DEV Community](https://dev.to/mr_nova/stop-using-tailwind-cdn-build-only-the-css-you-actually-use-django-php-go-1h38) — MEDIUM confidence
- [Recovery thresholds for alerts — Grafana Labs 2024](https://grafana.com/whats-new/2024-01-06-recovery-thresholds-for-alerts/) — HIGH confidence (official Grafana)
- [Reduce alert flapping — Datadog docs](https://docs.datadoghq.com/monitors/guide/reduce-alert-flapping/) — HIGH confidence (official Datadog)
- [Go Concurrency Mastery: Preventing Goroutine Leaks — DEV Community](https://dev.to/serifcolakel/go-concurrency-mastery-preventing-goroutine-leaks-with-context-timeout-cancellation-best-1lg0) — MEDIUM confidence
- [proposal: vet: report goroutine leak using time.Ticker — golang/go issue #68483](https://github.com/golang/go/issues/68483) — HIGH confidence (official Go tracker)
- [Killing a process and all of its descendants in Go — sigmoid.at](https://sigmoid.at/post/2023/08/kill_process_descendants_golang/) — MEDIUM confidence (practitioner)

**Setup Wizard (v1.1):**
- [bubbletea issue #860: stdout not a terminal workaround](https://github.com/charmbracelet/bubbletea/issues/860) — MEDIUM confidence (open issue, community workaround documented)
- [DeepWiki: Concurrency and Goroutines in bubbletea](https://deepwiki.com/charmbracelet/bubbletea/5.1-concurrency-and-goroutines) — MEDIUM confidence
- [bubbletea PR #1372: fix deadlock on context cancellation](https://github.com/charmbracelet/bubbletea/pull/1372) — HIGH confidence (merged fix)
- [Commands in Bubble Tea (official Charm blog)](https://charm.land/blog/commands-in-bubbletea/) — HIGH confidence (official)
- [golang/go issue #56173: os.WriteFile is not atomic](https://github.com/golang/go/issues/56173) — HIGH confidence (official Go tracker)
- [golang/go issue #35835: umask and os.WriteFile permissions](https://github.com/golang/go/issues/35835) — HIGH confidence (official Go tracker)

**Core Hub (v1.0):**
- [SQLite Concurrent Writes — tenthousandmeters.com](https://tenthousandmeters.com/blog/sqlite-concurrent-writes-and-database-is-locked-errors/) — HIGH confidence
- [Go + SQLite Best Practices — Jake Gold](https://jacob.gold/posts/go-sqlite-best-practices/) — HIGH confidence
- [Telegram Bot API — Rate Limits documentation](https://core.telegram.org/bots/faq) — HIGH confidence (official)
- [Go net/http timeouts — Cloudflare Engineering](https://blog.cloudflare.com/the-complete-guide-to-golang-net-http-timeouts/) — HIGH confidence
- [Telegram Bot API Errors list](https://github.com/TelegramBotAPI/errors) — MEDIUM confidence (community-maintained)

---
*Pitfalls research for: jaimito — VPS Push Notification Hub (Go + SQLite + Telegram Bot API + CLI + bubbletea TUI wizard + metrics collector + embedded web dashboard)*
*Researched: 2026-02-20 (v1.0 MVP) | Updated: 2026-03-23 (v1.1 Setup Wizard) | Updated: 2026-03-26 (v2.0 Métricas y Dashboard)*
