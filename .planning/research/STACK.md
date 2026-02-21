# Stack Research

**Domain:** Self-hosted Go notification/alerting hub (VPS)
**Researched:** 2026-02-20
**Confidence:** HIGH (all versions verified against pkg.go.dev and official GitHub releases)

---

## Recommended Stack

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
| `embed` (stdlib) | Go 1.16+ built-in | Embed SQL migration files | Embed `migrations/*.sql` into the binary with `//go:embed migrations/*.sql`. Eliminates runtime file path issues. No third-party tool needed. |
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

---

## Stack Patterns by Variant

**If running on arm64 VPS (e.g., Ampere, AWS Graviton):**
- Use `GOARCH=arm64 GOOS=linux CGO_ENABLED=0 go build`
- modernc.org/sqlite supports arm64 natively (pure Go)
- No additional toolchain changes needed

**If adding a web dashboard in v2:**
- Use `go:embed` to bundle a React/Vue SPA into the binary
- Chi already handles serving `http.FileServer` from an embedded FS
- Do NOT add a separate static file server dependency

**If Telegram rate limits become a problem:**
- go-telegram/bot surfaces `TooManyRequestsError` with `RetryAfter` field
- Implement exponential backoff in the dispatcher using this field directly
- SQLite `next_retry_at` column is pre-designed for this

**If multi-instance deployment is ever needed (v2+):**
- Switch from modernc.org/sqlite to `github.com/tursodatabase/libsql-client-go` (SQLite-compatible distributed DB)
- No application layer changes—same `database/sql` interface

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
- `go-chi/chi releases page` — v5.2.5 release notes confirmed (HIGH confidence)
- `spf13/cobra releases page` — v1.10.2 release notes, yaml.v3 migration confirmed (HIGH confidence)
- `alexedwards.net/blog/which-go-router-should-i-use` — chi vs stdlib analysis 2025 (MEDIUM confidence, respected Go author)
- `github.com/go-task/task/issues/2171` — gopkg.in/yaml.v3 unmaintained status (MEDIUM confidence, community issue thread)
- `sqlite.org/wal.html` — WAL mode concurrency characteristics (HIGH confidence, official SQLite docs)

---

*Stack research for: jaimito — VPS push notification hub in Go*
*Researched: 2026-02-20*
