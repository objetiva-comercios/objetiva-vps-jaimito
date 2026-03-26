# Stack Research

**Domain:** Self-hosted Go notification/alerting hub (VPS)
**Researched:** 2026-02-20
**Confidence:** HIGH (all versions verified against pkg.go.dev and official GitHub releases)

---

## v2.0 Additions: Métricas y Dashboard

*Added 2026-03-26 — new dependencies for metrics collection, storage, and embedded web dashboard.*

### Scope

This section covers ONLY what changes for v2.0. The existing stack (v1.0 + v1.1) remains unchanged. No new Go framework dependencies — the dashboard is embedded static files served by the existing chi router.

### New Go Dependencies

| Technology | Version | Purpose | Why Recommended |
|------------|---------|---------|-----------------|
| `github.com/shirou/gopsutil/v4` | v4.26.2 (published Feb 27, 2026) | CPU, RAM, disk, load, host, Docker metrics | **CGO-free** (all C structs ported to pure Go). Verified `pkg.go.dev`. Provides `cpu.Percent()`, `mem.VirtualMemory()`, `disk.Usage()`, `load.Avg()`, `host.Uptime()`, `docker.GetDockerStat()` — covers all 5 predefined metrics. Alternative is raw shell command parsing, but gopsutil eliminates parsing brittle output formats and handles cross-architecture differences (amd64/arm64). v4 is the current module path; v3 still exists but v4 is the maintained line. |

### Frontend Assets (Embedded via go:embed)

No npm, no Node.js, no build step in CI. All assets are either pre-compiled by the developer before committing, or fetched from CDN at development time and bundled.

| Asset | Version | Size (minified+gzip) | How to Embed | Why |
|-------|---------|---------------------|--------------|-----|
| Alpine.js | v3.14.x (latest v3) | ~7.1 KB gzipped | Download `cdn.min.js` → `web/static/js/alpine.min.js` | Lightweight reactive framework for the dashboard's table refresh and chart toggle. 7.1 KB gzipped is negligible in a binary. `x-data`, `x-show`, `@click`, `x-init` cover 100% of dashboard interactivity needs. No build toolchain required. |
| uPlot | v1.6.32 (Mar 14, 2025) | ~47.9 KB min (~15 KB gzip est.) | Download `dist/uPlot.iife.min.js` + `dist/uPlot.min.css` → `web/static/` | Time-series chart library at 1/5 the size of Chart.js (47.9 KB min vs 254 KB min). uPlot renders 3,600 data points at 60fps using 10% CPU vs Chart.js at 40% CPU. Purpose-built for time-series: x-axis is always time, y-axis is value — exactly the metrics readings pattern. No React/Vue dependency. IIFE build works directly in a `<script>` tag. |
| Tailwind CSS (pre-compiled) | v4.x (latest) | <10 KB (purged) | Run standalone CLI once to produce `web/static/css/tailwind.css` → commit compiled file | Tailwind v4 Play CDN is **development only** — not suitable for production (loads 1.2 MB of unused CSS). Standalone CLI (`tailwindcss-linux-x64`) produces purged CSS with only used classes. Typical dashboard output: <10 KB. No Node.js or npm needed on build/deploy machines — only the developer runs the CLI once. Commit `tailwind.css` to repo. |
| Lucide icons (inline SVG) | Latest (copy specific SVGs) | <1 KB per icon | Copy individual `.svg` files from `lucide-static` package → inline directly in HTML templates | For a dashboard with 10-15 icons, the CDN approach adds a full HTTP dependency and potential offline failure. Copy only the needed SVG markup inline into the HTML template — no external request, no JS library, no CSS icon font loading. Lucide SVGs are clean `<svg>` elements that accept Tailwind `class` attributes directly. |

### Build Integration: go:embed

No new Go libraries needed for embedding. The standard library `embed` package (Go 1.16+) handles everything.

**Recommended directory structure:**

```
internal/
  web/
    handler.go          # chi route registration for dashboard
web/
  static/
    css/
      tailwind.css      # pre-compiled by Tailwind standalone CLI
    js/
      alpine.min.js     # downloaded from CDN
      uplot.iife.min.js # downloaded from CDN/GitHub releases
      uplot.min.css     # downloaded from GitHub releases
  templates/
    dashboard.html      # single-page dashboard with inline Lucide SVGs
```

**Go embed declaration:**

```go
//go:embed web/static web/templates
var webFS embed.FS
```

**Chi route registration (integrates with existing router):**

```go
// Inside existing chi router setup
staticFS, _ := fs.Sub(webFS, "web/static")
r.Handle("/dashboard/static/*", http.StripPrefix("/dashboard/static/", http.FileServer(http.FS(staticFS))))
r.Get("/dashboard", dashboardHandler)
r.Get("/dashboard/", dashboardHandler)
```

No auth required — dashboard serves on `127.0.0.1:8080`, Tailscale-only access per spec.

### New SQLite Schema (via goose migration)

No new Go libraries. Two new tables added via existing goose migration system:

- `metric_definitions` — config-driven metric definitions (name, command, interval, thresholds)
- `metric_readings` — time-series readings (metric_name, value, collected_at)
- Index on `(metric_name, collected_at)` for efficient 7-day range queries
- goose scheduled task or ticker in `main.go` handles purge (DELETE WHERE collected_at < NOW - 7 days)

### New REST API Endpoints (chi integration)

No new router dependency. Add to existing chi subrouter under `/api/v1/`:

```
GET  /api/v1/metrics                     — current reading per metric
GET  /api/v1/metrics/{name}/readings     — time-series readings (last N hours)
POST /api/v1/metrics/{name}              — manual ingestion (jaimito metric CLI)
```

Bearer auth middleware already applied to `/api/v1/*` group — metrics endpoints inherit it automatically.

---

### What NOT to Add

| Avoid | Why | Use Instead |
|-------|-----|-------------|
| Prometheus client (`prometheus/client_golang`) | 15+ transitive deps, 2 MB binary size increase, requires Prometheus scraper to consume — overkill for a self-contained dashboard. The spec explicitly says "sin instalar Prometheus/Grafana". | Direct SQLite storage + custom REST API |
| Chart.js | 254 KB minified vs uPlot's 47.9 KB. Built for general charts (pie, bar, radar) — the dashboard only needs time-series line charts. uPlot's API is lower-level but covers exactly this use case with 5x less code. | uPlot v1.6.32 |
| React / Vue / Svelte SPA | Build toolchain (npm, bundler, HMR server) adds complexity for a single-page read-only dashboard. Alpine.js handles the reactive state (selected metric, chart visibility, auto-refresh interval) with ~20 lines of `x-data`. | Alpine.js v3 |
| HTMX | HTMX is excellent for server-rendered interactivity, but the dashboard needs client-side chart rendering (uPlot draws on `<canvas>`) — HTMX cannot replace Alpine.js + uPlot for that. Adding both HTMX and Alpine.js is redundant. | Alpine.js for interactivity |
| Tailwind CDN Play script | Dev-only. Loads the entire Tailwind engine (1.2 MB) in the browser. Not for production embedded binaries. | Standalone CLI pre-compiled CSS committed to repo |
| Icon font CDN (Font Awesome, Lucide icon font) | External HTTP request at runtime; failure makes icons disappear; loads ALL icons (wasted KB). | Inline SVG from lucide-static |
| `github.com/shirou/gopsutil/v3` | v4 is the current maintained module path. v3 still exists but v4 adds platform-specific Ex functions and has more recent bug fixes. | `github.com/shirou/gopsutil/v4` |
| `github.com/docker/docker` SDK | 20 MB of transitive deps to get container stats. gopsutil v4 `docker` package covers `running containers count` via `/sys/fs/cgroup` — sufficient for the predefined `docker_running` metric. | `gopsutil/v4/docker` |

---

### Installation: v2.0 additions only

```bash
# New Go dependency
go get github.com/shirou/gopsutil/v4@v4.26.2

# Frontend: download and commit (run once, not in CI)
# Tailwind standalone CLI (Linux amd64)
curl -sLO https://github.com/tailwindlabs/tailwindcss/releases/latest/download/tailwindcss-linux-x64
chmod +x tailwindcss-linux-x64
./tailwindcss-linux-x64 -i web/src/input.css -o web/static/css/tailwind.css --minify

# Alpine.js
curl -sL https://cdn.jsdelivr.net/npm/alpinejs@3/dist/cdn.min.js -o web/static/js/alpine.min.js

# uPlot (check https://github.com/leeoniya/uPlot/releases for latest tag)
curl -sL https://github.com/leeoniya/uPlot/releases/download/1.6.32/uPlot-1.6.32.zip -o uplot.zip
# Extract dist/uPlot.iife.min.js and dist/uPlot.min.css → web/static/js/ and web/static/css/

# Verify
go mod tidy
```

---

### Version Compatibility: v2.0 additions

| Package | Compatible With | Notes |
|---------|-----------------|-------|
| `gopsutil/v4@v4.26.2` | Go >= 1.18 | CGO_ENABLED=0 fully supported; pure Go; arm64 supported natively |
| Alpine.js v3 | All evergreen browsers | No IE11 support (irrelevant for local VPS dashboard) |
| uPlot v1.6.32 | All evergreen browsers | Uses Canvas API; works in all modern browsers; no polyfills needed |
| Tailwind CSS v4 pre-compiled | Static file — no runtime dep | Commit compiled CSS; no browser compatibility issue beyond Tailwind's own browser support matrix |

---

### Sources (v2.0 section)

- `pkg.go.dev/github.com/shirou/gopsutil/v4` — v4.26.2 published Feb 27, 2026; CGO-free confirmed in README (HIGH confidence)
- `github.com/leeoniya/uPlot` — v1.6.32, ~47.9 KB minified, performance benchmarks (HIGH confidence, official repo)
- `bundlephobia.com/package/alpinejs` — 7.1 KB gzipped confirmed (HIGH confidence)
- `tailwindcss.com/blog/standalone-cli` — standalone CLI without Node.js, official Tailwind announcement (HIGH confidence)
- `github.com/tailwindlabs/tailwindcss/discussions/15855` — v4 standalone CLI discussion, <10KB purged output (MEDIUM confidence, community-verified)
- `lucide.dev/guide/packages/lucide-static` — static SVG assets package for offline embedding (HIGH confidence, official docs)
- WebSearch: uPlot vs Chart.js bundle size comparison — 254 KB Chart.js vs 47.9 KB uPlot confirmed by multiple sources (HIGH confidence)
- WebSearch: Alpine.js vs React for lightweight dashboards (MEDIUM confidence, multiple community sources agree)
- `pkg.go.dev/embed` + `eli.thegreenplace.net` — go:embed with chi `http.FileServer(http.FS(...))` pattern (HIGH confidence)

---

## v1.1 Additions: TUI Setup Wizard

*Added 2026-03-23 — new dependencies for `jaimito setup` command only.*

### New Dependencies

| Technology | Version | Purpose | Why Recommended |
|------------|---------|---------|-----------------|
| `charm.land/bubbletea/v2` | v2.0.2 | TUI event loop and Model/Update/View framework | v2 is current stable (released Feb-Mar 2025/2026 via charm.land vanity domain). v1 (github.com path) is maintenance-only since Sep 2024. Use v2 for all new code — do not mix versions. |
| `charm.land/bubbles/v2` | v2.0.0 | Pre-built TUI components (textinput, spinner, list) | Official component library for bubbletea v2. Required v2 because types are incompatible with bubbletea v1. Provides textinput, spinner, and list — the three components needed by the wizard. |
| `charm.land/lipgloss/v2` | v2.0.2 | Terminal styling — colors, borders, padding, layout | `Style.Render()` still returns `string` in v2. Value type (immutable). Handles color downsampling automatically by terminal capability. Correct choice for the cyan/green/red/yellow theme in the spec. |
| `golang.org/x/term` | v0.41.0 | Detect interactive terminal before launching bubbletea | `term.IsTerminal(int(os.Stdin.Fd()))` — needed to abort gracefully when stdin is a pipe (curl \| bash). Official stdlib-adjacent package. Already transitively available via `golang.org/x/sys` in go.mod. |

### Installation

```bash
go get charm.land/bubbletea/v2@v2.0.2
go get charm.land/bubbles/v2@v2.0.0
go get charm.land/lipgloss/v2@v2.0.2
go get golang.org/x/term@v0.41.0
```

### Critical: v1 vs v2 — Always Use v2

The charmbracelet ecosystem has two generations with incompatible import paths:

| Generation | Import Path | Status |
|------------|------------|--------|
| v1 | `github.com/charmbracelet/bubbletea` | Maintenance-only (last: Sep 2024) |
| v2 | `charm.land/bubbletea/v2` | Current stable (v2.0.2, Mar 2026) |

Do NOT mix v1 and v2 — types are incompatible. bubbles v2 uses `tea.KeyPressMsg`; bubbletea v1 uses `tea.KeyMsg`. They will not compile together.

### v2 API Differences That Affect the Design Spec

1. **`Model.View()` returns `tea.View`** (not `string`). The spec's `Step.View() string` is an internal helper interface, not `tea.Model`. Steps return strings; the main wizard assembles them into `tea.View`. This is fully compatible.

2. **Key messages**: Use `tea.KeyPressMsg` instead of `tea.KeyMsg` in all `Update()` switches.

3. **`textinput.New()`**: Uses option functions in v2. Verify exact API with pkg.go.dev before coding — constructors changed from struct initialization to functional options.

4. **`spinner.New()`**: Takes `Option` functions: `spinner.New(spinner.WithSpinner(spinner.Dot), spinner.WithStyle(...))`.

### What NOT to Add

| Avoid | Why | Use Instead |
|-------|-----|-------------|
| `github.com/charmbracelet/bubbletea` (v1) | Maintenance-only; type-incompatible with bubbles v2 and lipgloss v2 | `charm.land/bubbletea/v2` |
| `github.com/charmbracelet/huh` | Higher-level form abstraction loses flexibility for non-linear navigation (jump-to-step) required by the spec | Raw bubbletea v2 + bubbles components |
| `github.com/AlecAivazis/survey` | Blocking stdin reads; incompatible with bubbletea's event loop | `charm.land/bubbles/v2/textinput` |
| `github.com/mattn/go-isatty` | Already an indirect dep via modernc.org/sqlite; direct dependency not needed; x/term is the official package | `golang.org/x/term` |

### Integration Points in Existing Codebase

**cobra entry** (`cmd/jaimito/setup.go`): Check `term.IsTerminal()` before `tea.NewProgram()`. If non-interactive, print error and `os.Exit(1)`.

**Validation** (`internal/telegram`): Add `ValidateTokenWithInfo()` returning bot username + display name. Wrap all validation calls as `tea.Cmd` — never block in `Update()`.

**DB key generation** (`internal/db`): Extract `GenerateRawKey() string` from `CreateKey()`. Pure function, no DB dependency. Wizard calls it directly; `CreateKey()` delegates internally.

**Config writing** (`cmd/jaimito/setup/config.go`): Marshal config struct with `gopkg.in/yaml.v3` (already in go.mod). Write to `--config` path with `os.WriteFile(..., 0600)`.

### Version Compatibility

| Package | Version | Notes |
|---------|---------|-------|
| `charm.land/bubbletea/v2` | v2.0.2 | Requires bubbles v2.0.0+ and lipgloss v2 for type compatibility |
| `charm.land/bubbles/v2` | v2.0.0 | Go 1.21+ required; Go 1.24 fully compatible |
| `charm.land/lipgloss/v2` | v2.0.2 | Color type changed from `string` to `image/color.Color` in v2; use `lipgloss.Color("#00BFFF")` which still works |
| `golang.org/x/term` | v0.41.0 | `golang.org/x/sys` already in go.mod (indirect dep); x/term builds on it, no conflict |

### Sources (TUI section)

- Go module proxy (`proxy.golang.org`, `charm.land`) — version timestamps verified 2026-03-23 — HIGH confidence
- `pkg.go.dev/charm.land/bubbletea/v2` — Model interface, `tea.View` return type confirmed — HIGH confidence
- `pkg.go.dev/charm.land/bubbles/v2/textinput` — v2 textinput API — HIGH confidence
- `pkg.go.dev/charm.land/bubbles/v2/spinner` — v2 spinner API — HIGH confidence
- `pkg.go.dev/charm.land/lipgloss/v2` — `Style.Render()` returns `string` confirmed — HIGH confidence
- `github.com/charmbracelet/bubbletea/discussions/1374` — v2 rationale, new-project recommendation — MEDIUM confidence
- `github.com/charmbracelet/bubbles/blob/main/UPGRADE_GUIDE_V2.md` — breaking changes v1→v2 — HIGH confidence

---

## Recommended Stack (v1.0 — Existing)

### Core Technologies

| Technology | Version | Purpose | Why Recommended |
|------------|---------|---------|-----------------|
| Go | 1.26 (released 2026-02-10) | Language runtime | Latest stable. `net/http` ServeMux 1.22+ routing, `log/slog` stdlib structured logging, `go:embed` for config embedding. Single binary output, <10MB compiled. CGO complications from modernc sqlite are handled cleanly in 1.22+. |
| `modernc.org/sqlite` | v1.46.1 (published 2026-02-18) | SQLite database driver | **CGO-free**, enabling true cross-compilation (`CGO_ENABLED=0`). Pure Go transpilation of SQLite C source. Slight performance penalty vs `mattn/go-sqlite3` (~10-15% slower writes) but irrelevant at jaimito's scale (hundreds of messages/hour). Eliminates GCC requirement and Docker cross-compile complexity. WAL mode supported. |
| `github.com/go-chi/chi/v5` | v5.2.5 (published 2026-02-05) | HTTP router and middleware | Idiomatic, stdlib-compatible (uses `net/http` handlers directly). Middleware grouping is why chi beats plain `net/http` for this project: auth middleware on `/api/v1/*` group, rate-limit middleware per-route group, all with clean subrouter composition. Requires min Go 1.22 (already met). |
| `github.com/spf13/cobra` | v1.10.2 (published 2025-12-03) | CLI framework | De-facto standard for Go CLI. Handles `jaimito send`, `jaimito wrap`, `jaimito keys`, `jaimito status` as subcommands with flags, usage docs, and shell completion generation automatically. No viable contender at this maturity level. |

### Supporting Libraries

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `github.com/go-telegram/bot` | v1.19.0 (published 2026-02-18) | Telegram dispatcher | Zero-dependency Telegram Bot API wrapper supporting API 9.4. Use for the Telegram dispatcher: call `SendMessage` with MarkdownV2, skip `bot.Start()` entirely (no polling needed—jaimito is send-only). Handles 429 TooManyRequests with typed errors you can catch for retry logic. |
| `github.com/pressly/goose/v3` | v3.26.0 (published 2025-10-03) | Database migrations | Embedded SQL migrations run at startup (`goose.Up`). Keeps schema evolution versioned, rollback-safe, and auditable. Works cleanly with `modernc.org/sqlite`. Native `slog` integration via `WithSlog` option. Alternative: `golang-migrate` does not wrap individual statements in transactions—skip it. |
| `github.com/google/uuid` | v1.6.0 (published 2024-01-23) | UUID v7 generation | Official Google UUID library with `NewV7()`. UUID v7 is time-ordered (millisecond precision) making it sortable as a primary key in SQLite without a separate `created_at` index for ordering. RFC 9562 recommended: "implementations SHOULD utilize UUID version 7 over v1 and v6". |
| `github.com/stretchr/testify` | v1.11.1 (published 2025-08-27) | Test assertions and mocks | `assert` for non-fatal test failures, `require` for fatal ones, `mock` for dispatcher interface mocking. Industry standard. Keeps test code readable without rolling custom assertion helpers. |
| `go.yaml.in/yaml/v3` | v3.0.4 (published 2025-06-29) | YAML config parsing | `gopkg.in/yaml.v3` is unmaintained (the canonical upstream moved to `go.yaml.in`). cobra's own v1.10.2 already migrated to `go.yaml.in/yaml/v3`. Use this import path from the start to avoid a migration later. Same API surface as yaml.v3. |

### Development Tools

| Tool | Purpose | Notes |
|------|---------|-------|
| `golangci-lint` | v2.10.1 (published 2026-02-17) | Static analysis and linting | v2 config format uses `linters.default` (replaces enable-all/disable-all). Run in CI. Key linters: `errcheck`, `staticcheck`, `govet`, `revive`. Use `golangci-lint migrate` to generate v2 config from scratch. |
| `log/slog` (stdlib) | Go 1.21+ built-in | Structured logging | Use `slog.JSONHandler` in production (systemd captures stdout; JSON parses in Loki/Grafana). `slog.TextHandler` for dev. Zero external dependency. Goose v3.26 also integrates with slog via `WithSlog`. |
| `embed` (stdlib) | Go 1.16+ built-in | Embed SQL migration files + dashboard assets | Embed `migrations/*.sql` and `web/` directory into the binary with `//go:embed`. Eliminates runtime file path issues. No third-party tool needed. |
| GoReleaser | v2.x (latest, check goreleaser.com) | Release binary builds | Produces `linux/amd64` and `linux/arm64` binaries, `.deb`/`.rpm` packages, and systemd unit in a single config. With `CGO_ENABLED=0` (modernc sqlite), no Docker cross-compile complexity needed. |

---

## Installation

```bash
# Initialize module
go mod init github.com/youruser/jaimito

# Core runtime dependencies
go get github.com/go-chi/chi/v5@v5.2.5
go get github.com/spf13/cobra@v1.10.2
go get modernc.org/sqlite@v1.46.1
go get github.com/go-telegram/bot@v1.19.0
go get github.com/pressly/goose/v3@v3.26.0
go get github.com/google/uuid@v1.6.0
go get go.yaml.in/yaml/v3@v3.0.4

# Test dependencies
go get github.com/stretchr/testify@v1.11.1

# v1.1: TUI wizard
go get charm.land/bubbletea/v2@v2.0.2
go get charm.land/bubbles/v2@v2.0.0
go get charm.land/lipgloss/v2@v2.0.2
go get golang.org/x/term@v0.41.0

# v2.0: Metrics and dashboard
go get github.com/shirou/gopsutil/v4@v4.26.2

# Verify
go mod tidy
```

---

## Alternatives Considered

| Recommended | Alternative | When to Use Alternative |
|-------------|-------------|-------------------------|
| `modernc.org/sqlite` | `mattn/go-sqlite3` | Only if benchmarks reveal a genuine bottleneck at production load, OR if CGO cross-compile complexity is acceptable (requires GCC on build host). mattn is 10-15% faster on writes but the difference is immaterial at jaimito's expected volume. |
| `go-chi/chi/v5` | `net/http` stdlib ServeMux | For projects where middleware grouping is not needed. Go 1.22+ ServeMux handles method+path routing adequately for 3-5 routes. chi becomes necessary once you add per-group auth, rate-limit, and request logging middleware—jaimito needs this from day one. |
| `go-chi/chi/v5` | `gin-gonic/gin` | Gin adds unnecessary weight (~50 deps) and uses its own context type (breaks stdlib handler compatibility). Not worth the complexity for a <10 route API. |
| `github.com/go-telegram/bot` | Raw `net/http` POST to Telegram API | Acceptable for the absolute MVP (30 lines of code), but the library adds typed errors for 429/400/401, structured params, MarkdownV2 escaping helpers—worth the zero-dependency cost. |
| `github.com/go-telegram/bot` | `github.com/go-telegram-bot-api/telegram-bot-api/v5` | v5.5.1 was published in December 2021 and is not actively maintained. go-telegram/bot tracks Bot API 9.4 (2026). Avoid the older library for new projects. |
| `go.yaml.in/yaml/v3` | `gopkg.in/yaml.v3` | Never. `gopkg.in/yaml.v3` is unmaintained. `go.yaml.in/yaml/v3` is the upstream continuation with identical API. |
| `pressly/goose/v3` | `golang-migrate` | golang-migrate does not wrap individual statements in transactions—a partial migration leaves the database inconsistent. Goose wraps each migration file in a transaction. For an embedded use case (no separate CLI needed), goose is more ergonomic. |
| `log/slog` (stdlib) | `uber-go/zap` or `rs/zerolog` | Only if benchmarks show slog overhead is a problem, which at jaimito's scale (<50MB target, low notification volume) it will not be. Avoid external logging dependencies when stdlib suffices. |
| `charm.land/bubbletea/v2` | `github.com/charmbracelet/huh` | huh provides a higher-level form abstraction that works well for linear wizards. Avoid here because the spec requires non-linear navigation (jump-to-step from summary). Raw bubbletea gives full control. |
| uPlot (v2.0 dashboard) | Chart.js | Chart.js is better if you need pie charts, radar charts, stacked bar charts. For time-series only, uPlot is 5x smaller and 4x lower CPU. |
| Alpine.js (v2.0 dashboard) | React/Vue/Svelte | Use a SPA framework only if the dashboard requires complex client-side routing or deeply nested component trees. A single-page metrics dashboard doesn't warrant the build complexity. |
| Pre-compiled Tailwind CSS (v2.0) | Tailwind CDN Play script | Use Play CDN only in development/prototyping. Never in production embedded binaries. |

---

## What NOT to Use

| Avoid | Why | Use Instead |
|-------|-----|-------------|
| `gopkg.in/yaml.v3` | Unmaintained—upstream moved to `go.yaml.in`. `cobra` v1.10.2 already migrated away. Using old path creates supply-chain drift. | `go.yaml.in/yaml/v3` (identical API) |
| `mattn/go-sqlite3` | Requires CGO and GCC. Breaks `CGO_ENABLED=0` cross-compilation. Binary requires platform-matching libc. Complicates Docker-based releases. | `modernc.org/sqlite` |
| `gin-gonic/gin` | 50+ indirect dependencies; custom context type breaks stdlib handler compatibility; too heavy for a <10 route internal API | `go-chi/chi/v5` |
| `go-telegram-bot-api/v5` | Last release December 2021 (v5.5.1); missing 4+ years of Telegram Bot API updates (now at 9.4); active issue backlog with no merges | `github.com/go-telegram/bot` |
| `nikoksr/notify` | Attractive multi-channel abstraction, but adds significant dependency weight and hides Telegram API internals. Jaimito *is* the notification abstraction layer—using notify creates an abstraction on top of an abstraction. Build dispatchers directly. | Direct dispatcher implementations per channel |
| `GORM` | ORM unnecessary for a 2-table schema; hides SQL behavior; GORM's auto-migration is not safe for production schema changes. | Direct `database/sql` queries + goose for migrations |
| `gorilla/mux` | Unmaintained (archived). chi superseded it as the community standard. | `go-chi/chi/v5` |
| External message queue (Redis, RabbitMQ) | Violates the zero-external-dependencies constraint. SQLite WAL mode handles the queue durably at jaimito's scale. | SQLite `messages` table as the queue |
| `github.com/charmbracelet/bubbletea` (v1 path) | Maintenance-only since Sep 2024; type-incompatible with bubbles v2 and lipgloss v2 | `charm.land/bubbletea/v2` |
| `prometheus/client_golang` | 15+ transitive deps, ~2 MB binary increase, requires external Prometheus scraper. Contradicts "no Prometheus/Grafana" spec constraint. | Direct SQLite storage + custom `/api/v1/metrics` endpoint |
| Chart.js | 254 KB minified (vs uPlot 47.9 KB). General-purpose library overkill for time-series-only dashboard. | uPlot v1.6.32 |
| Tailwind CDN Play script in production | Dev-only. Ships 1.2 MB of unused CSS. Requires browser JS engine to compile CSS at runtime. | Tailwind standalone CLI pre-compiled CSS |
| `github.com/docker/docker` SDK | ~20 MB of transitive dependencies for container stats. gopsutil/v4's docker package covers the `docker_running` metric via cgroups. | `gopsutil/v4/docker` |
| `github.com/shirou/gopsutil/v3` | v3 is superseded. v4 is the current module path with ongoing maintenance (v4.26.2 Feb 2026). | `github.com/shirou/gopsutil/v4` |

---

## Stack Patterns by Variant

**If running on arm64 VPS (e.g., Ampere, AWS Graviton):**
- Use `GOARCH=arm64 GOOS=linux CGO_ENABLED=0 go build`
- modernc.org/sqlite supports arm64 natively (pure Go)
- gopsutil/v4 supports arm64 natively (pure Go)
- No additional toolchain changes needed

**If adding a web dashboard in v2 (now active):**
- Use `go:embed` to bundle `web/` directory into the binary
- Chi already handles serving `http.FileServer(http.FS(...))` from an embedded FS
- Pre-compile Tailwind CSS once with standalone CLI, commit to repo
- Do NOT add a separate static file server dependency

**If Telegram rate limits become a problem:**
- go-telegram/bot surfaces `TooManyRequestsError` with `RetryAfter` field
- Implement exponential backoff in the dispatcher using this field directly
- SQLite `next_retry_at` column is pre-designed for this

**If multi-instance deployment is ever needed (v2+):**
- Switch from modernc.org/sqlite to `github.com/tursodatabase/libsql-client-go` (SQLite-compatible distributed DB)
- No application layer changes—same `database/sql` interface

**If wizard needs to run in a non-standard terminal (e.g., tmux, screen):**
- bubbletea v2 handles terminal detection via `TERM` environment; no special handling needed
- `/dev/tty` redirect in install.sh handles the curl | bash case at the shell level

**If metrics collector needs to run custom shell commands (v2.0):**
- Use `os/exec` stdlib — no external library. `exec.CommandContext(ctx, "sh", "-c", command)` with a timeout context
- gopsutil handles the 5 predefined metrics (disk, RAM, CPU, Docker, uptime)
- Custom user-defined metrics in config.yaml use raw shell commands via exec

---

## Version Compatibility

| Package | Compatible With | Notes |
|---------|-----------------|-------|
| `go-chi/chi/v5@v5.2.5` | Go >= 1.22 | v5.2.5 bumped minimum Go to 1.22; Go 1.26 fully compatible |
| `modernc.org/sqlite@v1.46.1` | Go >= 1.21 | CGO_ENABLED=0; works with Go 1.26 |
| `pressly/goose/v3@v3.26.0` | Go >= 1.21 | `WithSlog` added in 3.26.0; compatible with `log/slog` stdlib |
| `github.com/go-telegram/bot@v1.19.0` | Go >= 1.18 | Zero external dependencies; Go 1.26 compatible |
| `github.com/spf13/cobra@v1.10.2` | Go >= 1.21 | Migrated to `go.yaml.in/yaml/v3` internally in this version |
| `go.yaml.in/yaml/v3@v3.0.4` | Go >= 1.18 | v4 tagged but no migration guide published yet; stick with v3 |
| `github.com/google/uuid@v1.6.0` | Go >= 1.17 | `NewV7()` added in v1.6.0 (Jan 2024) |
| `charm.land/bubbletea/v2@v2.0.2` | Go >= 1.21 | Requires bubbles v2 and lipgloss v2 for type-safe composition |
| `charm.land/bubbles/v2@v2.0.0` | bubbletea v2 only | Type-incompatible with bubbletea v1 |
| `charm.land/lipgloss/v2@v2.0.2` | bubbletea v1 or v2 | Styling only; `Render()` returns string; no bubbletea type dependency |
| `golang.org/x/term@v0.41.0` | All Go versions | Uses `golang.org/x/sys` internally; already transitive dep in go.mod |
| `gopsutil/v4@v4.26.2` | Go >= 1.18 | CGO_ENABLED=0; arm64 + amd64 supported; Linux only for docker package |

---

## Sources

- `pkg.go.dev/github.com/go-chi/chi/v5` — v5.2.5 published Feb 5, 2026 (HIGH confidence, verified)
- `pkg.go.dev/modernc.org/sqlite` — v1.46.1 published Feb 18, 2026 (HIGH confidence, verified)
- `pkg.go.dev/github.com/spf13/cobra` — v1.10.2 published Dec 3, 2025 (HIGH confidence, verified)
- `pkg.go.dev/github.com/go-telegram/bot` — v1.19.0 published Feb 18, 2026, Bot API 9.4 (HIGH confidence, verified)
- `pkg.go.dev/github.com/pressly/goose/v3` — v3.26.0 published Oct 3, 2025 (HIGH confidence, verified)
- `pkg.go.dev/github.com/google/uuid` — v1.6.0 published Jan 23, 2024 (HIGH confidence, verified)
- `pkg.go.dev/github.com/stretchr/testify` — v1.11.1 published Aug 27, 2025 (HIGH confidence, verified)
- `pkg.go.dev/go.yaml.in/yaml/v3` — v3.0.4 published Jun 29, 2025 (HIGH confidence, verified)
- `go.dev/doc/devel/release` — Go 1.26 released Feb 10, 2026 (HIGH confidence, official Go site)
- `github.com/golangci/golangci-lint/releases` — v2.10.1 released Feb 17, 2026 (HIGH confidence, verified)
- Go module proxy `proxy.golang.org` + `charm.land` — bubbletea v2.0.2, bubbles v2.0.0, lipgloss v2.0.2, x/term v0.41.0 timestamps verified 2026-03-23 (HIGH confidence)
- `pkg.go.dev/charm.land/bubbletea/v2` — Model interface, `tea.View` return type (HIGH confidence)
- `pkg.go.dev/charm.land/bubbles/v2/textinput` — v2 textinput API (HIGH confidence)
- `pkg.go.dev/charm.land/bubbles/v2/spinner` — v2 spinner API (HIGH confidence)
- `pkg.go.dev/charm.land/lipgloss/v2` — `Style.Render()` returns `string` confirmed (HIGH confidence)
- `github.com/charmbracelet/bubbletea/discussions/1374` — v2 rationale and recommendation (MEDIUM confidence)
- `github.com/charmbracelet/bubbles/blob/main/UPGRADE_GUIDE_V2.md` — v1→v2 breaking changes (HIGH confidence)
- `alexedwards.net/blog/which-go-router-should-i-use` — chi vs stdlib analysis 2025 (MEDIUM confidence, respected Go author)
- `sqlite.org/wal.html` — WAL mode concurrency characteristics (HIGH confidence, official SQLite docs)
- `pkg.go.dev/github.com/shirou/gopsutil/v4` — v4.26.2 published Feb 27, 2026; CGO-free confirmed (HIGH confidence)
- `github.com/leeoniya/uPlot` — v1.6.32, ~47.9 KB minified, benchmark data (HIGH confidence, official repo)
- `bundlephobia.com/package/alpinejs` — 7.1 KB gzipped (HIGH confidence)
- `tailwindcss.com/blog/standalone-cli` — standalone CLI official announcement (HIGH confidence)
- `lucide.dev/guide/packages/lucide-static` — static SVG assets, offline embedding (HIGH confidence, official docs)

---

*Stack research for: jaimito — VPS push notification hub in Go*
*Researched: 2026-02-20 (v1.0 base stack) | Updated: 2026-03-23 (v1.1 TUI additions) | Updated: 2026-03-26 (v2.0 metrics + dashboard)*
