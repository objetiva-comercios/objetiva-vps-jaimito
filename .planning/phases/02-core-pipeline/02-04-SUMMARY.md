---
phase: 02-core-pipeline
plan: "04"
subsystem: infra
tags: [go, http, graceful-shutdown, startup-wiring, seed-keys, dispatcher]

# Dependency graph
requires:
  - phase: 02-core-pipeline/02-01
    provides: db.SeedKeys, ServerConfig.Listen, SeedAPIKey type
  - phase: 02-core-pipeline/02-02
    provides: api.NewRouter, BearerAuth, NotifyHandler
  - phase: 02-core-pipeline/02-03
    provides: dispatcher.Start, polling loop with retry/429 handling
provides:
  - Full Phase 2 startup wiring in cmd/jaimito/main.go (15 steps)
  - HTTP server started on cfg.Server.Listen with 15s read/write timeouts
  - Graceful HTTP shutdown with 30s context on SIGTERM/SIGINT
  - Seed API key bootstrap on every startup (idempotent)
  - Dispatcher goroutine started before HTTP server accepts connections
  - Updated example config with server.listen and seed_api_keys sections
affects: [03-deployment, ops, systemd-service]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "15-step startup sequence: config -> validation -> DB -> seed -> background workers -> HTTP server -> signal wait -> graceful shutdown"
    - "errors.Is(err, http.ErrServerClosed) for clean server stop detection"
    - "context.WithTimeout(context.Background(), 30*time.Second) for graceful shutdown — new Background context so parent cancellation doesn't abort shutdown"

key-files:
  created: []
  modified:
    - cmd/jaimito/main.go
    - configs/config.example.yaml

key-decisions:
  - "Dispatcher and cleanup start before HTTP server accepts connections — ensures delivery goroutines are live before first API call lands"
  - "HTTP shutdown uses new context.Background() with 30s timeout — parent ctx is already cancelled at shutdown time, so inheriting it would cause immediate abort"
  - "errors.Is(err, http.ErrServerClosed) — ListenAndServe returns this non-error on clean Shutdown(); must NOT be treated as failure"

patterns-established:
  - "Startup-then-shutdown pattern: start all background goroutines, then block on <-ctx.Done(), then explicitly shut down only components that need explicit shutdown (HTTP server)"
  - "Database closes via defer — no explicit shutdown call needed"

requirements-completed: [API-01, API-02, API-03, API-04, TELE-01, TELE-02, TELE-03]

# Metrics
duration: 2min
completed: 2026-02-21
---

# Phase 2 Plan 04: Startup Wiring Summary

**Full Phase 2 startup sequence wired into main.go: seed keys + dispatcher + HTTP server with graceful 30s shutdown, plus updated example config**

## Performance

- **Duration:** 2 min
- **Started:** 2026-02-21T19:09:59Z
- **Completed:** 2026-02-21T19:11:59Z
- **Tasks:** 2
- **Files modified:** 2

## Accomplishments
- Extended main.go from 12 to 15 startup steps, wiring all Phase 2 components
- HTTP server starts on cfg.Server.Listen with 15s read/write/60s idle timeouts
- Graceful shutdown: HTTP server gets explicit 30s Shutdown context; dispatcher and cleanup stop via ctx cancellation; DB closes via defer
- Example config updated with server.listen (127.0.0.1:8080) and seed_api_keys sections with generation instructions

## Task Commits

Each task was committed atomically:

1. **Task 1: Wire Phase 2 components into main.go startup sequence** - `4e0bfdb` (feat)
2. **Task 2: Update example config with server and seed_api_keys sections** - `05afb42` (chore)

**Plan metadata:** `(pending docs commit)`

## Files Created/Modified
- `cmd/jaimito/main.go` - Extended startup sequence: seed keys (step 10), dispatcher.Start (step 11), cleanup.Start (step 12), HTTP server with goroutine (step 13), ready log (step 14), graceful shutdown (step 15)
- `configs/config.example.yaml` - Added server section (after database) and seed_api_keys section (at end) with generation comment

## Decisions Made
- Dispatcher and cleanup start before HTTP server so delivery goroutines are live before any API call can land
- HTTP shutdown uses `context.WithTimeout(context.Background(), 30*time.Second)` — cannot inherit `ctx` because it is already cancelled when shutdown runs
- `errors.Is(err, http.ErrServerClosed)` mandatory — `ListenAndServe` returns this on clean `Shutdown()`, must not trigger `os.Exit(1)`

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- Phase 2 complete: all components built and wired. Binary compiles, startup sequence is correct.
- Phase 3 (deployment) can now write the systemd service, install scripts, and packaging.
- Seed key in example config (`sk-REPLACE_ME_WITH_A_REAL_KEY`) must be replaced before production deployment.

## Self-Check: PASSED

- cmd/jaimito/main.go: FOUND
- configs/config.example.yaml: FOUND
- 02-04-SUMMARY.md: FOUND
- commit 4e0bfdb: FOUND
- commit 05afb42: FOUND

---
*Phase: 02-core-pipeline*
*Completed: 2026-02-21*
