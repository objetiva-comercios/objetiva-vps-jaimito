---
phase: 03-cli-developer-experience
plan: 01
subsystem: cli
tags: [cobra, cli, api-keys, sqlite, crypto]

# Dependency graph
requires:
  - phase: 02-core-pipeline
    provides: db.HashToken, db.Open, db.ApplySchema, db.SeedKeys, internal/config, all daemon components
provides:
  - cobra CLI scaffold with rootCmd, persistent --config flag, resolveServer/resolveAPIKey helpers
  - runServe: daemon logic extracted from main() into cobra RunE handler
  - db.CreateKey, db.ListKeys, db.RevokeKey, db.ApiKey for programmatic key management
  - jaimito keys create/list/revoke subcommands with direct DB access
affects:
  - 03-02-send (uses resolveServer, resolveAPIKey helpers from root.go)
  - 03-03-wrap (uses cobra scaffold, rootCmd.AddCommand pattern)

# Tech tracking
tech-stack:
  added:
    - github.com/spf13/cobra v1.10.2 (CLI framework)
    - github.com/spf13/pflag v1.0.9 (cobra dependency, persistent flags)
    - github.com/inconshreveable/mousetrap v1.1.0 (cobra dependency, Windows)
  patterns:
    - cobra RunE pattern: RunE returns error, main prints to stderr and exits 1
    - SilenceUsage+SilenceErrors on rootCmd: clean error output without usage spam
    - openDB helper: load config -> open DB -> apply schema, used by all keys subcommands
    - resolveServer/resolveAPIKey: flag -> env -> config/default priority chain

key-files:
  created:
    - cmd/jaimito/root.go
    - cmd/jaimito/serve.go
    - cmd/jaimito/keys.go
  modified:
    - cmd/jaimito/main.go (reduced to 12-line entrypoint)
    - internal/db/apikeys.go (added ApiKey, CreateKey, ListKeys, RevokeKey)
    - go.mod (cobra + pflag + mousetrap added)

key-decisions:
  - "SilenceUsage+SilenceErrors on rootCmd: prevents cobra from printing usage on RunE errors, main handles error display"
  - "Bare jaimito with no subcommand starts server via rootCmd.RunE = runServe (backward compatible)"
  - "keys subcommands use direct DB access (not HTTP) — keys management bypasses auth middleware by design"
  - "openDB helper applies schema on every keys command invocation — idempotent, ensures CLI works on fresh DB"
  - "CreateKey uses crypto/rand 32 bytes -> hex -> sk- prefix — 256 bits entropy"

patterns-established:
  - "cobra command files: one file per command group (keys.go, send.go, wrap.go), all package main"
  - "RunE error handling: return fmt.Errorf, never os.Exit in subcommands — main.go handles os.Exit"
  - "openDB pattern: config.Load -> db.Open -> db.ApplySchema in one helper, defer Close at call site"

requirements-completed: [CLI-03]

# Metrics
duration: 2min
completed: 2026-02-23
---

# Phase 3 Plan 01: CLI Scaffold and Keys Management Summary

**Cobra CLI scaffold with jaimito keys create/list/revoke subcommands backed by direct SQLite access and crypto/rand key generation**

## Performance

- **Duration:** 2 min
- **Started:** 2026-02-23T21:14:02Z
- **Completed:** 2026-02-23T21:16:13Z
- **Tasks:** 3
- **Files modified:** 6

## Accomplishments
- Added cobra v1.10.2 and extracted server daemon into runServe — bare jaimito still starts the server
- Added ApiKey struct, CreateKey (crypto/rand sk- keys), ListKeys, RevokeKey to internal/db/apikeys.go
- Implemented jaimito keys create/list/revoke with tabwriter output and full openDB helper chain

## Task Commits

Each task was committed atomically:

1. **Task 1: Add cobra scaffold (root.go, serve.go, main.go)** - `54f9e3b` (feat)
2. **Task 2: Add DB key management functions** - `5f7c2d0` (feat)
3. **Task 3: Create keys subcommand** - `3c9dce6` (feat)

**Plan metadata:** (docs commit — see below)

## Files Created/Modified
- `cmd/jaimito/root.go` - rootCmd with persistent --config flag, resolveServer/resolveAPIKey helpers
- `cmd/jaimito/serve.go` - runServe: all daemon logic extracted from main.go as cobra RunE handler
- `cmd/jaimito/keys.go` - keysCmd with create/list/revoke subcommands and openDB helper
- `cmd/jaimito/main.go` - reduced to 12-line entrypoint: rootCmd.Execute() + error printing
- `internal/db/apikeys.go` - added ApiKey struct, CreateKey, ListKeys, RevokeKey, crypto/rand import
- `go.mod` / `go.sum` - cobra v1.10.2 added

## Decisions Made
- SilenceUsage+SilenceErrors on rootCmd: cobra won't print usage on RunE errors; main.go owns error display
- Bare `jaimito` (no subcommand) starts the server via rootCmd.RunE = runServe, preserving backward compat
- keys subcommands use direct DB access rather than HTTP — key management intentionally bypasses auth middleware
- openDB applies schema on each invocation — idempotent, ensures CLI works on fresh or existing database
- crypto/rand 32 bytes -> hex -> sk- prefix gives 256 bits of entropy per key

## Deviations from Plan

None — plan executed exactly as written.

## Issues Encountered
- First `go get` ran from wrong shell working directory (env cwd mismatch), cobra was not added to go.mod. Re-ran with absolute path reference. No code impact.

## User Setup Required
None — no external service configuration required.

## Next Phase Readiness
- Cobra scaffold complete: rootCmd, keysCmd registered, resolveServer/resolveAPIKey helpers available
- Plans 03-02 (send) and 03-03 (wrap) can add their commands via rootCmd.AddCommand in their own files
- All verification steps pass: go build, go vet, --help output correct for all commands

## Self-Check: PASSED

All created files exist and all task commits verified in git history.

---
*Phase: 03-cli-developer-experience*
*Completed: 2026-02-23*
