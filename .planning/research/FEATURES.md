# Feature Research

**Domain:** Lightweight self-hosted VPS metrics dashboard (jaimito v2.0 — subsequent milestone)
**Researched:** 2026-03-26
**Confidence:** HIGH (based on analysis of Beszel, Netdata, Glances, OpsDash, Uptime Kuma + UX pattern research)

> Note: This file was originally written for v1.0 (notification hub features). It has been replaced for v2.0 (metrics + dashboard). The v1.0 feature landscape is captured in PROJECT.md under Validated requirements.

---

## Context

This is NOT a standalone monitoring product. It's a metrics extension for jaimito, a Go binary that already handles notifications, Telegram dispatch, and SQLite storage. The target user already has jaimito running and wants metrics without installing Prometheus/Grafana/Netdata. The dashboard is accessed localhost-only (via Tailscale), no auth layer needed. Everything embeds into the same single binary.

Comparable tools analyzed: Beszel, Netdata, Glances, OpsDash, Uptime Kuma.

---

## Feature Landscape

### Table Stakes (Users Expect These)

Features users assume exist in a monitoring dashboard. Missing these = product feels incomplete.

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| CPU usage (current value + history) | Every monitoring tool shows this first; it's the mental model for "is the server struggling?" | LOW | `/proc/stat` polling at configurable interval; store as metric_readings in SQLite |
| RAM usage (current + history) | Second metric users check; VPS OOM kills are catastrophic and silent | LOW | `/proc/meminfo`; store used_bytes + total_bytes + percent; show percent in table |
| Disk usage per mount (current + history) | Primary VPS failure mode: disk fills up silently; all competitors show this prominently | LOW | `syscall.Statfs` for root mount minimum; percentage is the critical number |
| System uptime | Quick server health proxy; users verify "is this a fresh reboot?" on every check | LOW | `/proc/uptime`; trivial read; show as days in table |
| Metric table view (all metrics, current value, status) | Users scan a dense table to assess state at a glance before diving in | MEDIUM | HTML table with Alpine.js; columns: name, current value, unit, status (OK/WARN/CRIT), last updated |
| Expandable time-series chart per metric | Users drill into anomalies after alert or when value looks wrong; progressive disclosure | MEDIUM | uPlot (~45KB gzipped); table row click expands inline chart; show last N readings |
| Warning/critical threshold alerts to Telegram | Core value: jaimito already dispatches to Telegram; metrics without alerts are just vanity metrics | MEDIUM | Two severity levels; integrates directly with existing dispatcher.Send(); no new infrastructure |
| Automatic purge of old readings | Disk cannot grow unbounded; users expect managed retention without manual SQL | LOW | Background goroutine; `DELETE FROM metric_readings WHERE timestamp < now - retention_days`; run once per hour |
| `jaimito status` CLI command | SSH-first VPS operators check state from terminal before opening browser | LOW | Tabular output of latest value per metric via cobra subcommand; queries REST API or DB directly |
| Docker container running count | VPS users running Docker want immediate answer to "are my containers up?"; asked first after reboot | MEDIUM | `docker ps --format json` via exec; gracefully absent if Docker not installed |

### Differentiators (Competitive Advantage)

Features that set jaimito apart from Beszel/Netdata in this specific context.

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| Custom metrics via shell commands in config.yaml | Any metric imaginable without writing Go code: `ssl_days_remaining`, `pg_active_connections`, `queue_depth` — one config line per metric | MEDIUM | Each entry: `{name, command, interval, unit, thresholds}`; jaimito executes shell, parses float output, stores reading |
| Single binary, zero extra processes | Beszel requires hub + agent (two binaries, two systemd units); Netdata is ~100MB; jaimito adds metrics as goroutines inside existing binary | LOW | Already the architectural constraint; advantage is automatic given existing design |
| Threshold state machine (alert on crossing, not on every reading) | Alert fires once when crossing WARNING, once when crossing CRITICAL, once when recovering — not every 30 seconds | MEDIUM | Per-metric state: NORMAL/WARNING/CRITICAL; transition-only alerts; state persisted across restarts in SQLite |
| Pre-defined metrics out of the box with zero config | User gets `disk_root`, `ram_used`, `cpu_load`, `docker_running`, `uptime_days` immediately after install | LOW | Hard-coded defaults implemented as regular metric definitions; overridable in config.yaml |
| `jaimito metric push` CLI for external ingestion | External scripts push arbitrary values: `jaimito metric push --name ssl_expiry --value 42`; cron-friendly | LOW | Thin cobra subcommand + POST to internal REST endpoint; enables any external data source |
| Metrics API for programmatic access | Other tools, scripts, or dashboards can consume metrics: `GET /api/v1/metrics/{name}/readings?since=...` | LOW | Already planned in milestone scope; REST API is natural extension of existing chi router |
| Integrates with existing notification routing | Threshold alerts use existing channel routing and priority system — `critical` threshold fires as `critical` priority Telegram message automatically | LOW | No new wiring needed; call existing dispatcher with appropriate channel/priority |

### Anti-Features (Commonly Requested, Often Problematic)

| Feature | Why Requested | Why Problematic | Alternative |
|---------|---------------|-----------------|-------------|
| Multi-server monitoring from one dashboard | "Monitor all my VPS from one place" | Fundamental architecture change; jaimito is single-VPS by design; adds cross-node network auth, aggregation complexity, and distributed state | Run one jaimito per VPS; Telegram already aggregates alerts from multiple servers via different bot tokens or channels |
| Real-time push / WebSocket metrics | "I want live graph updates" | Polling every 30s is sufficient for VPS ops; WebSocket adds goroutine-per-connection, reconnect logic, backpressure handling — no operational benefit over periodic refresh | Alpine.js `setInterval` auto-refresh every 30s; feels live enough for ops use, zero WebSocket complexity |
| Dashboard authentication | "Shouldn't the dashboard have a login?" | Already decided: 127.0.0.1 bind + Tailscale is the auth layer; adding HTTP auth breaks the simplicity promise and creates credential rotation burden | Bind to loopback only; document Tailscale as the access layer; consistent with existing jaimito auth model |
| Alert on every reading above threshold | "Alert me whenever CPU > 80%" | Fires every polling interval while above threshold — every 30s during a spike = 120 alerts/hour; documented cause of alert fatigue leading to tool abandonment | Transition-only alerts: fire once when crossing threshold, optionally fire again on recovery |
| Metric aggregation / downsampling | "Keep a year of history at lower resolution" | Two-table schema (raw + aggregate), merge logic, background downsampling job — significant complexity for a VPS with 5-10 metrics | Fixed 7-day retention at full resolution; 7 days at 1-min intervals for 10 metrics ≈ ~1MB SQLite; add downsampling only if users request longer history |
| Drag-and-drop dashboard customization | "I want to rearrange panels like Grafana" | Requires stateful layout persistence, complex JS, full drag-and-drop library — breaks the lightweight contract | Fixed layout: table on top, expand-on-click chart below; opinionated but fast and simple |
| Per-metric Telegram bot / chat override | "I want disk alerts to go to a different Telegram chat" | Multi-destination routing is a dispatcher milestone concern; adds config complexity to every metric definition | Use jaimito's existing channel routing; alerts go to the channel configured for `metrics` or `alerts` |
| Process-level monitoring (top-N processes) | "Show me which process is eating CPU" | `/proc/[pid]/stat` polling for all PIDs, name resolution, sorting — separate subsystem with significant complexity | Use `jaimito wrap ps aux` as a custom metric command, or define a custom metric with `ps` command |
| Email / Slack alert channels | "I want disk alerts by email too" | Other dispatchers are deferred milestones for the whole jaimito project; grafting them onto threshold alerts couples two unrelated work streams | Threshold alerts go through the existing Telegram dispatcher; other channels follow when the dispatcher milestone ships |

---

## Feature Dependencies

```
[SQLite metrics schema]
    └──required by──> [Metric Collector goroutine]
    └──required by──> [REST API for metrics]
    └──required by──> [Alert state machine]
    └──required by──> [Automatic purge]
    └──requires──>    [Existing SQLite connection pool (already built)]

[Metric Collector goroutine]
    └──required by──> [Predefined metrics (disk/RAM/CPU/uptime/docker)]
    └──required by──> [Custom metrics from config.yaml]
    └──required by──> [Alert state machine]
    └──requires──>    [SQLite metrics schema]
    └──requires──>    [config.yaml metrics section parsing]

[REST API: GET /api/v1/metrics]
    └──required by──> [Dashboard web UI table]
    └──required by──> [`jaimito status` CLI]
    └──requires──>    [SQLite metrics schema]

[REST API: GET /api/v1/metrics/{name}/readings]
    └──required by──> [Dashboard uPlot chart]
    └──required by──> [External metric consumers]
    └──requires──>    [SQLite metrics schema]

[REST API: POST /api/v1/metrics/{name}/readings]
    └──required by──> [`jaimito metric push` CLI]
    └──requires──>    [SQLite metrics schema]

[Dashboard web UI]
    └──requires──>    [REST API: GET /api/v1/metrics]
    └──requires──>    [REST API: GET /api/v1/metrics/{name}/readings]
    └──requires──>    [go:embed static assets (HTML/CSS/JS)]

[Alert state machine]
    └──requires──>    [Metric Collector goroutine]
    └──requires──>    [Existing Telegram dispatcher (already built)]
    └──enhances──>    [Threshold config in config.yaml]

[Automatic purge]
    └──requires──>    [SQLite metrics schema]
    └──can run in──>  [Metric Collector goroutine] (same background loop)
```

### Dependency Notes

- **Schema first, everything else second.** The metric_definitions and metric_readings table design affects every other component. Index strategy (metric_name + timestamp composite) must be correct from the start — retrofitting indexes on a populated table is painful.
- **REST API before UI.** The dashboard is a static HTML file that fetches from the API. API must be stable and tested independently before building the frontend assets.
- **Alert state machine is decoupled from Telegram.** The state machine (NORMAL/WARNING/CRITICAL per metric name) can be fully implemented and unit-tested by calling `dispatcher.Send()` with a mock. No Telegram dependency during development.
- **Custom metrics are identical to predefined metrics at runtime.** Predefined metrics are implemented as hard-coded metric definitions fed through the same collector path as custom metrics. One implementation, two configuration sources. Build the custom metric path, predefined metrics become trivial defaults.
- **Docker monitoring is isolatable.** It's implemented as a single custom metric internally (`docker_running` via `docker ps`). If Docker is absent, the collector logs a warning and skips. No conditional compilation needed.

---

## MVP Definition

This is milestone v2.0. "MVP" = minimum to make the milestone complete and useful.

### Launch With (v2.0)

- [x] SQLite schema: metric_definitions + metric_readings tables — everything depends on this, must be first
- [x] Collector goroutine with predefined metrics (disk_root, ram_used, cpu_load, uptime_days) — immediate visible value
- [x] Custom metrics via config.yaml shell commands — the differentiator; enables ssl_expiry, pg_connections, queue depth
- [x] REST API: GET /api/v1/metrics + GET /api/v1/metrics/{name}/readings — feeds UI and CLI
- [x] REST API: POST /api/v1/metrics/{name}/readings — enables `jaimito metric push`
- [x] Dashboard web UI: metric table + expandable uPlot time-series chart (go:embed) — the visible deliverable
- [x] Warning/critical threshold alerts via existing Telegram dispatcher (transition-only, no alert storm) — closes the monitoring loop
- [x] `jaimito status` CLI — terminal-first VPS operator workflow
- [x] `jaimito metric push` CLI — external script integration
- [x] Automatic 7-day retention purge — operational hygiene, prevents disk growth

### Add After Validation (v2.x)

- [ ] Docker container monitoring — useful but optional; requires Docker socket; add after core metrics proven stable
- [ ] Configurable retention window in config.yaml — currently hard-coded 7 days; add when a user requests longer history
- [ ] Alert recovery notifications ("CPU back to normal") — valuable once basic alerts are shipping; adds one more state transition to the machine

### Future Consideration (v3+)

- [ ] Metric aggregation / downsampling for long-term retention — add only if SQLite size becomes a real concern at observed usage
- [ ] Additional alert channels (Slack, email) — follows the broader dispatcher milestone roadmap
- [ ] Multi-VPS aggregation view — architectural change; out of scope for this milestone

---

## Feature Prioritization Matrix

| Feature | User Value | Implementation Cost | Priority |
|---------|------------|---------------------|----------|
| SQLite metrics schema | HIGH | LOW | P1 |
| Predefined system metrics (disk/RAM/CPU/uptime) | HIGH | LOW | P1 |
| Custom metrics via shell commands + config.yaml | HIGH | MEDIUM | P1 |
| Dashboard table view | HIGH | MEDIUM | P1 |
| Expandable uPlot chart | HIGH | MEDIUM | P1 |
| Threshold alerts (warning + critical) to Telegram | HIGH | MEDIUM | P1 |
| Alert state machine (transition-only, no storm) | HIGH | MEDIUM | P1 |
| REST API GET /api/v1/metrics | MEDIUM | LOW | P1 |
| REST API GET /api/v1/metrics/{name}/readings | MEDIUM | LOW | P1 |
| REST API POST /api/v1/metrics/{name}/readings | MEDIUM | LOW | P1 |
| `jaimito status` CLI | MEDIUM | LOW | P1 |
| `jaimito metric push` CLI | MEDIUM | LOW | P1 |
| Automatic 7-day retention purge | MEDIUM | LOW | P1 |
| Docker container monitoring | MEDIUM | MEDIUM | P2 |
| Configurable retention window | LOW | LOW | P2 |
| Alert recovery notifications | MEDIUM | LOW | P2 |
| Metric downsampling / aggregation | LOW | HIGH | P3 |
| Multi-VPS aggregation | LOW | HIGH | P3 |

**Priority key:**
- P1: Must have for v2.0 launch
- P2: Should have, add in v2.x when possible
- P3: Nice to have, future consideration

---

## Competitor Feature Analysis

| Feature | Beszel | Netdata | Glances | OpsDash | jaimito v2.0 |
|---------|--------|---------|---------|---------|--------------|
| CPU/RAM/Disk monitoring | Yes | Yes | Yes | Yes | Yes |
| Docker container monitoring | Yes (deep — per-container stats) | Yes (deep) | Yes | Yes | Yes (basic — running count) |
| Custom metrics | REST API push only | Plugin system (complex) | No | No | Shell commands in config.yaml |
| Threshold alerting | Yes (per metric) | Yes (complex YAML rules) | No | Yes | Yes (warning/critical, transition-only) |
| Telegram native alerts | No (webhook generic) | Via community integrations | No | No | Yes (native, pre-existing dispatcher) |
| Dashboard UI | Yes (PocketBase web) | Yes (heavy JS) | TUI / basic web | Yes | Yes (embedded ~60KB JS total) |
| Multi-server | Yes (hub + agent model) | Yes | No | Yes | No (by design) |
| Single binary | No (hub + agent = 2 binaries) | No | Yes | No | Yes |
| Memory footprint | 23MB hub + 6MB agent = 29MB | ~100MB | ~30MB | Unknown | <50MB target (all-in) |
| External DB required | No (PocketBase = embedded) | Optional (InfluxDB) | No | No | No (SQLite already embedded) |
| Config format | YAML + web UI | Complex YAML | Config file | Config file | Extends existing config.yaml |
| CLI metric ingestion | No | No | No | No | Yes (`jaimito metric push`) |
| Integrates with notification hub | No | No | No | No | Yes (same binary, same dispatcher) |
| Install complexity | Hub + agent setup, Docker recommended | Package install or Docker | pip install or Docker | Single binary | Already installed (add to existing) |

**Key insight:** No comparable tool combines single-binary + native Telegram alerting + custom shell-command metrics + zero new dependencies + existing notification pipeline integration. jaimito v2.0's differentiator is not metrics collection per se (all tools do this) but the seamless integration of metrics into an existing operational notification workflow.

---

## Expected User Behaviors

Based on analysis of homelab monitoring tool usage patterns:

1. **Glance, not monitor.** VPS operators open the dashboard occasionally — before deploys, after receiving an alert, or when the server feels slow. They do not stare at it continuously. Dashboard load time and information density matter more than real-time updates. 30-second auto-refresh is sufficient; sub-second is unnecessary.

2. **Alert-then-investigate flow.** Primary user journey: Telegram alert fires → user opens dashboard → clicks on the alerted metric → examines the time-series chart to understand context. The expand-on-click chart pattern directly serves this flow.

3. **CLI first for quick checks.** SSH-first operators will run `jaimito status` before opening a browser. Terminal output must be scannable (aligned columns, clear units, status indicators).

4. **Config transparency builds trust.** Users trust the system more when they can see exactly what commands are running (config.yaml) rather than opaque internal collectors. Showing the command in the UI or CLI helps users debug custom metrics.

5. **Alert fatigue is an adoption killer.** Tools that fire alerts every polling interval above a threshold get disabled within days. Two-level thresholds (warning vs critical) and transition-only firing are operational requirements, not UX niceties. This is documented across Netdata, Icinga, and Datadog post-mortems on alert fatigue.

6. **Custom metrics are the power user unlock.** The first question after seeing predefined metrics work is "can I add my own?" SSL expiry days, PostgreSQL connection count, queue depth, backup age — every VPS operator has 2-3 custom values they care about. This feature determines long-term retention.

---

## Sources

- [Beszel GitHub — feature set, architecture, resource usage](https://github.com/henrygd/beszel) — HIGH confidence (official source)
- [Beszel official docs](https://beszel.dev/guide/what-is-beszel) — HIGH confidence
- [XDA Developers: Beszel review (homelab user behavior)](https://www.xda-developers.com/beszel-feature/) — MEDIUM confidence
- [Netdata vs Glances comparison](https://nestnepal.com/blog/guide-resource-monitoring-htop-glances-netdata/) — MEDIUM confidence
- [Alert Fatigue in Monitoring — Icinga](https://icinga.com/blog/alert-fatigue-monitoring/) — MEDIUM confidence
- [VPS Monitoring Guide 2026 — threshold recommendations](https://simpleobservability.com/blog/vps-monitoring-guide) — MEDIUM confidence
- [uPlot GitHub — 45KB, Canvas 2D, time series](https://github.com/leeoniya/uPlot) — HIGH confidence (official source)
- [Dashboard UX — Smashing Magazine 2025](https://www.smashingmagazine.com/2025/09/ux-strategies-real-time-dashboards/) — MEDIUM confidence
- [Dashboard layout patterns](https://www.datawirefra.me/blog/dashboard-layout-patterns) — MEDIUM confidence
- [Monitoring alerting best practices 2026](https://oneuptime.com/blog/post/2026-02-20-monitoring-alerting-best-practices/view) — MEDIUM confidence
- [Beszel Docker monitoring — DeepWiki](https://deepwiki.com/henrygd/beszel/3.2-docker-container-monitoring) — MEDIUM confidence

---
*Feature research for: Lightweight VPS metrics dashboard (jaimito v2.0 — Metricas y Dashboard milestone)*
*Researched: 2026-03-26*
