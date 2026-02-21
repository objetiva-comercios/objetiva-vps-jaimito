---
phase: 02-core-pipeline
plan: "02"
subsystem: api
tags: [chi, http, bearer-auth, uuid, middleware, go]

# Dependency graph
requires:
  - phase: 02-01
    provides: db.HashToken, db.LookupKeyHash, db.UpdateLastUsed, db.EnqueueMessage, config.ChannelExists

provides:
  - Chi v5 HTTP router with global middleware stack (RequestID, RealIP, Logger, Recoverer, Timeout)
  - BearerAuth middleware extracting sk- tokens, SHA-256 hashing, DB lookup
  - NotifyHandler: POST /api/v1/notify — validates, defaults, enqueues, returns 202 + UUID v7 ID
  - HealthHandler: GET /api/v1/health — returns 200 JSON unauthenticated
  - WriteJSON/WriteError response helpers enforcing correct header ordering

affects:
  - 02-03 (dispatcher reads messages enqueued via this API)
  - main.go integration (must wire NewRouter with db and cfg)

# Tech tracking
tech-stack:
  added:
    - github.com/go-chi/chi/v5 v5.2.5 (HTTP router and middleware)
    - github.com/google/uuid v1.6.0 (promoted from indirect to direct; UUID v7 message IDs)
  patterns:
    - WriteHeader before json.Encode to prevent spurious 200 responses
    - Fire-and-forget UpdateLastUsed with context.Background() (non-critical, request context may cancel)
    - r.Group() for scoped middleware application without affecting health route
    - http.MaxBytesReader for body size limiting

key-files:
  created:
    - internal/api/server.go
    - internal/api/middleware.go
    - internal/api/handlers.go
    - internal/api/response.go
  modified:
    - go.mod
    - go.sum

key-decisions:
  - "handlers.go implemented alongside server.go in Task 1 commit scope — server.go references handler symbols, so both must compile together; handlers committed separately in Task 2"
  - "BearerAuth uses db.HashToken (same function as SeedKeys) — no hash mismatch possible between seeding and auth"
  - "NotifyHandler defaults channel=general and priority=normal before validation, per CONTEXT.md locked decisions"
  - "NewRouter takes no *bot.Bot — API layer only enqueues to DB; dispatcher is decoupled and reads independently"

patterns-established:
  - "WriteJSON: always set Content-Type then WriteHeader then Encode — never reverse order"
  - "BearerAuth pattern: HasPrefix(Bearer ) -> TrimPrefix -> HasPrefix(sk-) -> HashToken -> LookupKeyHash -> fire-and-forget UpdateLastUsed"
  - "Unauthenticated routes registered before r.Group() block to avoid middleware inheritance"

requirements-completed: [API-01, API-02, API-03, API-04]

# Metrics
duration: 2min
completed: 2026-02-21
---

# Phase 2 Plan 02: HTTP API Layer Summary

**Chi v5 router with BearerAuth middleware (sk- tokens, SHA-256 hashing), NotifyHandler (POST 202 + UUID v7), and HealthHandler (GET 200) — complete internal/api package wiring the ingest surface to the message queue**

## Performance

- **Duration:** 2 min
- **Started:** 2026-02-21T19:04:39Z
- **Completed:** 2026-02-21T19:07:04Z
- **Tasks:** 2
- **Files modified:** 6

## Accomplishments

- Chi v5 router with five global middleware (RequestID, RealIP, Logger, Recoverer, 30s Timeout)
- BearerAuth middleware validates sk- prefixed Bearer tokens via SHA-256 hash lookup in DB
- NotifyHandler validates body/channel/priority, enqueues message with UUID v7 ID, responds 202 Accepted
- HealthHandler responds 200 JSON without requiring authentication
- WriteJSON/WriteError helpers enforce correct header-before-encode ordering

## Task Commits

Each task was committed atomically:

1. **Task 1: Add chi v5 dependency and create router with auth middleware** - `c00a3b0` (feat)
2. **Task 2: Implement NotifyHandler and HealthHandler** - `e0575eb` (feat)

**Plan metadata:** _(docs commit follows)_

## Files Created/Modified

- `internal/api/response.go` - WriteJSON, WriteError, ErrorResponse helpers
- `internal/api/middleware.go` - BearerAuth middleware with db.HashToken + db.LookupKeyHash
- `internal/api/server.go` - NewRouter: chi router, global middleware, health + notify routes
- `internal/api/handlers.go` - NotifyHandler and HealthHandler implementations
- `go.mod` - chi v5.2.5 and uuid v1.6.0 promoted to direct dependencies
- `go.sum` - updated checksums

## Decisions Made

- handlers.go was created during Task 1's compilation step (server.go references handler symbols that must be present to compile), then committed separately in Task 2 as planned
- NewRouter signature takes `*sql.DB` and `*config.Config` only — no Telegram bot dependency, enforcing clean separation between ingest (API) and delivery (dispatcher)
- Channel and priority defaults (general, normal) applied before validation so partial payloads are valid

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- internal/api package complete and compiles cleanly (go build + go vet pass)
- Ready for Phase 02-03: Telegram dispatcher that reads from the message queue populated by NotifyHandler
- main.go integration needed: wire NewRouter(db, cfg) and start HTTP server on cfg.Server.Listen

---
*Phase: 02-core-pipeline*
*Completed: 2026-02-21*

## Self-Check: PASSED

- internal/api/server.go: FOUND
- internal/api/middleware.go: FOUND
- internal/api/handlers.go: FOUND
- internal/api/response.go: FOUND
- Commit c00a3b0 (Task 1): FOUND
- Commit e0575eb (Task 2): FOUND
