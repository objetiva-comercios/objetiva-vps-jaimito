---
phase: 03-cli-developer-experience
plan: 02
subsystem: cli
tags: [cobra, cli, http-client, send, notifications]

# Dependency graph
requires:
  - phase: 03-cli-developer-experience
    plan: 01
    provides: cobra scaffold, rootCmd, resolveServer, resolveAPIKey helpers
  - phase: 02-core-pipeline
    provides: internal/api/handlers.go NotifyRequest, POST /api/v1/notify endpoint
provides:
  - internal/client/client.go with Client.Notify method for HTTP POST to /api/v1/notify
  - cmd/jaimito/send.go with full flag support for send subcommand
affects:
  - 03-03-wrap (reuses internal/client package for HTTP notifications)

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "client package is standalone — does not import internal/api, avoids circular dependency"
    - "NotifyRequest pointer title field: only set when flag explicitly provided"
    - "context.Background() passed to Notify — no deadline on user-facing CLI send"

key-files:
  created:
    - internal/client/client.go
    - cmd/jaimito/send.go

key-decisions:
  - "internal/client does not import internal/api — client-side struct mirrors server struct to avoid circular dependency"
  - "New() takes host:port and prepends http:// — matches how config stores server.listen"
  - "Channel and priority sent as empty strings when not specified — server applies general/normal defaults"
  - "Title is pointer field: only set in request when --title flag is explicitly provided"
  - "On success, only message ID printed to stdout — pipe-friendly output"

requirements-completed: [CLI-01]

# Metrics
duration: 1min
completed: 2026-02-23
---

# Phase 3 Plan 02: HTTP Client and Send Command Summary

**Lightweight HTTP client package and `jaimito send` subcommand delivering notifications to the jaimito API with full flag support for channel, priority, title, tags, and stdin**

## Performance

- **Duration:** 1 min
- **Started:** 2026-02-23T21:18:40Z
- **Completed:** 2026-02-23T21:19:47Z
- **Tasks:** 2
- **Files modified:** 2

## Accomplishments
- Created `internal/client/client.go` with `Client`, `New()`, `NotifyRequest`, and `Notify()` method — standalone package, no circular dependency with `internal/api`
- Created `cmd/jaimito/send.go` implementing the full send subcommand with body-from-arg, --stdin, -c channel, -p priority, -t title, --tags flags

## Task Commits

Each task was committed atomically:

1. **Task 1: Create HTTP client package for jaimito API** - `5f9e315` (feat)
2. **Task 2: Create send subcommand** - `9dcd548` (feat)

**Plan metadata:** (docs commit — see below)

## Files Created/Modified
- `internal/client/client.go` - Client struct, New() constructor, NotifyRequest, Notify() method with Bearer auth, 10s timeout, context-aware
- `cmd/jaimito/send.go` - sendCmd with full flag set, runSend resolving body/auth/server, creating client.Client and calling Notify

## Decisions Made
- `internal/client` does not import `internal/api` — client-side `NotifyRequest` mirrors server struct independently to avoid circular dependency
- `New()` takes host:port and prepends `http://` — consistent with how `config.Server.Listen` stores the address
- Channel and priority sent as empty strings when flags omitted; the server applies `general`/`normal` defaults before validation
- `Title` is a pointer field (`*string`) set only when `--title` flag is explicitly provided — matches the optional semantics in `api.NotifyRequest`
- On success, only the message ID is printed to stdout — ensures pipe-friendliness (`jaimito send "msg" | xargs ...`)

## Deviations from Plan

None — plan executed exactly as written.

## Self-Check: PASSED

All created files exist and all task commits verified in git history.

---
*Phase: 03-cli-developer-experience*
*Completed: 2026-02-23*
