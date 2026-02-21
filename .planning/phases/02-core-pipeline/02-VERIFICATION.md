---
phase: 02-core-pipeline
verified: 2026-02-21T20:00:00Z
status: passed
score: 12/12 must-haves verified
re_verification: false
---

# Phase 2: Core Pipeline Verification Report

**Phase Goal:** A running service that accepts authenticated HTTP notifications, queues them durably, and delivers them to Telegram with correct formatting and automatic retry
**Verified:** 2026-02-21T20:00:00Z
**Status:** passed
**Re-verification:** No — initial verification

---

## Goal Achievement

### Observable Truths

| #  | Truth                                                                                          | Status     | Evidence                                                                   |
|----|------------------------------------------------------------------------------------------------|------------|----------------------------------------------------------------------------|
| 1  | Config accepts server.listen, seed_api_keys, and exposes ChannelExists/ChatIDForChannel        | VERIFIED   | `config.go:17-18,39-47,140-157`; default 127.0.0.1:8080 set at line 75   |
| 2  | Schema migration makes title nullable without data loss                                        | VERIFIED   | `002_nullable_title.sql`: shadow-table rename/recreate/copy/drop + indexes |
| 3  | Messages can be enqueued, fetched by queued status, and transitioned through all states        | VERIFIED   | `messages.go`: EnqueueMessage/GetNextQueued/MarkDispatching/MarkDelivered/MarkFailed/RecordAttempt all present and substantive |
| 4  | API key hashes can be inserted, looked up, and last_used_at updated                            | VERIFIED   | `apikeys.go`: HashToken(SHA-256), LookupKeyHash(revocation-aware), SeedKeys(INSERT OR IGNORE), UpdateLastUsed |
| 5  | POST /api/v1/notify with valid Bearer token and JSON body returns 202 with message ID          | VERIFIED   | `handlers.go:88`: WriteJSON(w, http.StatusAccepted, NotifyResponse{ID: id.String()}) |
| 6  | POST /api/v1/notify with bad token returns 401                                                 | VERIFIED   | `middleware.go:20,27,33`: three 401 paths — missing Bearer, non-sk- prefix, DB lookup failure |
| 7  | POST /api/v1/notify with malformed payload returns 400                                         | VERIFIED   | `handlers.go:49,63,67,71`: 400 for bad JSON, empty body, unknown channel, invalid priority |
| 8  | GET /api/v1/health returns 200 JSON without requiring auth                                     | VERIFIED   | `server.go:26`: r.Get("/api/v1/health", HealthHandler) registered before Group; `handlers.go:94`: StatusOK |
| 9  | Messages are formatted with priority emoji, optional bold title, escaped body, and hashtag tags | VERIFIED   | `format.go:11-61`: priorityEmoji map, bold title branch, EscapeMarkdown on body, literal # + escaped tag content |
| 10 | Dispatcher polls every 1 second and delivers messages sequentially                             | VERIFIED   | `dispatcher.go:29`: time.NewTicker(1 * time.Second); sequential dispatch in dispatchNext |
| 11 | 429 responses cause exact retry_after wait; other errors use exponential backoff               | VERIFIED   | `dispatcher.go:105-118`: IsTooManyRequestsError + tooMany.RetryAfter * time.Second; line 123: baseDelay * 1<<uint(attempt-1) |
| 12 | After 5 failed attempts, message is marked failed silently                                     | VERIFIED   | `dispatcher.go:139-143`: MarkFailed + slog.Error + return nil after loop |

**Score:** 12/12 truths verified

---

### Required Artifacts

| Artifact                                  | Provides                                                                 | Exists | Substantive | Wired  | Status     |
|-------------------------------------------|--------------------------------------------------------------------------|--------|-------------|--------|------------|
| `internal/config/config.go`               | ServerConfig, SeedAPIKey, ChannelExists, ChatIDForChannel                | yes    | yes (158 lines, all functions present) | yes (imported by api, db, dispatcher, main) | VERIFIED |
| `internal/db/schema/002_nullable_title.sql` | Shadow-table migration making title TEXT nullable                       | yes    | yes (contains ALTER TABLE RENAME, shadow CREATE, INSERT SELECT, DROP, indexes) | yes (applied by db.ApplySchema on startup) | VERIFIED |
| `internal/db/messages.go`                 | EnqueueMessage, GetNextQueued, MarkDispatching, MarkDelivered, MarkFailed, RecordAttempt | yes | yes (156 lines, all 6 functions substantive) | yes (used by handlers.go and dispatcher.go) | VERIFIED |
| `internal/db/apikeys.go`                  | HashToken, LookupKeyHash, SeedKeys, UpdateLastUsed                       | yes    | yes (79 lines, all 4 functions substantive) | yes (middleware.go uses HashToken+LookupKeyHash; main.go uses SeedKeys) | VERIFIED |
| `internal/api/server.go`                  | Chi router with middleware stack and route registration                   | yes    | yes (NewRouter with 5 global middleware, unauthenticated health, authenticated group) | yes (main.go:98 api.NewRouter) | VERIFIED |
| `internal/api/middleware.go`              | BearerAuth middleware with sk- token validation and DB lookup            | yes    | yes (complete auth chain: Bearer extract -> sk- check -> HashToken -> LookupKeyHash -> UpdateLastUsed) | yes (server.go:30 r.Use(BearerAuth(database))) | VERIFIED |
| `internal/api/handlers.go`               | NotifyHandler (POST 202 + UUID v7), HealthHandler (GET 200)              | yes    | yes (validation, defaults, enqueue, 202 response; health 200 response) | yes (server.go:26,31 route registration) | VERIFIED |
| `internal/api/response.go`               | WriteJSON, WriteError, ErrorResponse                                     | yes    | yes (WriteHeader before Encode enforced; ErrorResponse struct) | yes (used by middleware.go and handlers.go) | VERIFIED |
| `internal/telegram/format.go`            | FormatMessage producing MarkdownV2 text                                  | yes    | yes (emoji map, title branch, body escape, tag hashtag handling with literal #) | yes (dispatcher.go:71 telegram.FormatMessage) | VERIFIED |
| `internal/dispatcher/dispatcher.go`      | Polling loop with retry and 429 handling                                 | yes    | yes (Start goroutine, 1s ticker, maxAttempts=5, backoff, 429 handling, MarkFailed on exhaustion) | yes (main.go:92 dispatcher.Start) | VERIFIED |
| `cmd/jaimito/main.go`                    | Full startup wiring: seed keys, dispatcher, HTTP server, graceful shutdown | yes  | yes (15 startup steps, errors.Is(ErrServerClosed), 30s shutdown context) | yes (all components wired together) | VERIFIED |
| `configs/config.example.yaml`            | Updated example config with server.listen and seed_api_keys              | yes    | yes (server section after database; seed_api_keys at end with generation comment) | yes (documentation artifact) | VERIFIED |

---

### Key Link Verification

| From                              | To                               | Via                                              | Status  | Evidence                              |
|-----------------------------------|----------------------------------|--------------------------------------------------|---------|---------------------------------------|
| `internal/db/apikeys.go`          | `internal/config/config.go`      | SeedKeys accepts []config.SeedAPIKey             | WIRED   | `apikeys.go:43` func signature        |
| `internal/db/messages.go`         | `002_nullable_title.sql`         | EnqueueMessage uses nullable title column         | WIRED   | `messages.go:47` INSERT INTO messages |
| `internal/api/middleware.go`      | `internal/db/apikeys.go`         | BearerAuth calls db.HashToken and db.LookupKeyHash | WIRED | `middleware.go:30-31`                 |
| `internal/api/handlers.go`        | `internal/db/messages.go`        | NotifyHandler calls db.EnqueueMessage            | WIRED   | `handlers.go:83`                      |
| `internal/api/handlers.go`        | `internal/config/config.go`      | NotifyHandler calls cfg.ChannelExists            | WIRED   | `handlers.go:66`                      |
| `internal/dispatcher/dispatcher.go` | `internal/telegram/format.go`  | dispatch calls telegram.FormatMessage            | WIRED   | `dispatcher.go:71`                    |
| `internal/dispatcher/dispatcher.go` | `internal/db/messages.go`      | dispatch calls GetNextQueued, MarkDispatching, MarkDelivered, MarkFailed, RecordAttempt | WIRED | `dispatcher.go:48,66,77,91,97,139` |
| `internal/dispatcher/dispatcher.go` | `bot.SendMessage`              | sends via b.SendMessage with ParseModeMarkdown   | WIRED   | `dispatcher.go:84-88`                 |
| `cmd/jaimito/main.go`             | `internal/api/server.go`         | main creates router via api.NewRouter            | WIRED   | `main.go:98`                          |
| `cmd/jaimito/main.go`             | `internal/dispatcher/dispatcher.go` | main calls dispatcher.Start                   | WIRED   | `main.go:92`                          |
| `cmd/jaimito/main.go`             | `internal/db/apikeys.go`         | main calls db.SeedKeys on startup               | WIRED   | `main.go:84`                          |

All 11 key links: WIRED.

---

### Requirements Coverage

| Requirement | Source Plan(s) | Description                                                                                    | Status    | Evidence                                                     |
|-------------|---------------|------------------------------------------------------------------------------------------------|-----------|--------------------------------------------------------------|
| API-01      | 02-01, 02-02, 02-04 | Service accepts HTTP POST to `/api/v1/notify` with JSON payload (title, body, channel, priority, tags, metadata) | SATISFIED | `handlers.go:15-22` NotifyRequest struct; `server.go:31` POST route |
| API-02      | 02-01, 02-02, 02-04 | Service authenticates requests via Bearer token with `sk-` prefix API keys                    | SATISFIED | `middleware.go:15-44` BearerAuth complete implementation     |
| API-03      | 02-02, 02-04        | Service returns 202 with message ID on success, 400 on invalid payload, 401 on bad auth       | SATISFIED | `handlers.go:49,63,67,71,88`; `middleware.go:20,27,33`      |
| API-04      | 02-02, 02-04        | Service exposes GET `/api/v1/health` returning 200 when operational                           | SATISFIED | `server.go:26`; `handlers.go:93-95`                         |
| TELE-01     | 02-01, 02-03, 02-04 | Service delivers messages to configured Telegram chat via Bot API                             | SATISFIED | `dispatcher.go:84-88` b.SendMessage with chatID from cfg    |
| TELE-02     | 02-03, 02-04        | Service formats messages with MarkdownV2 (priority emoji, bold title, code blocks, proper escaping) | SATISFIED | `format.go:24-61` FormatMessage with EscapeMarkdown and ParseModeMarkdown |
| TELE-03     | 02-03, 02-04        | Service retries failed deliveries with exponential backoff, respecting Telegram 429 `retry_after` | SATISFIED | `dispatcher.go:104-135` 429 handling + exponential backoff; `dispatcher.go:18-20` maxAttempts=5, baseDelay=2s |

All 7 requirements: SATISFIED.

No orphaned requirements found — all phase-2 requirement IDs in REQUIREMENTS.md (API-01 through API-04, TELE-01 through TELE-03) are accounted for across plans 02-01 through 02-04.

---

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| `internal/config/config_test.go` | 135-136 | "placeholder" in comment | Info | Test comment explaining that the example config token is a placeholder; test patches it — expected and correct |

No blocker or warning anti-patterns. The single info-level match is a test-internal comment about fixture setup, not an implementation stub.

---

### Human Verification Required

#### 1. End-to-End Delivery to Real Telegram

**Test:** Configure a real bot token and chat ID, send a POST to /api/v1/notify with a valid sk- API key, observe the Telegram message.
**Expected:** Message appears in the Telegram chat within 1-2 seconds, formatted with correct emoji, bold title (if provided), escaped special characters, and hashtag tags.
**Why human:** Requires a live Telegram Bot API credential and real chat ID. Cannot verify actual delivery or MarkdownV2 rendering correctness programmatically.

#### 2. ParseModeMarkdown vs ParseModeMarkdownV1 Rendering

**Test:** Send a message with MarkdownV2 special characters (e.g. body containing `hello_world (test)`) and confirm it renders correctly in Telegram rather than showing escape backslashes.
**Expected:** Characters are rendered as formatted text, not raw escape sequences.
**Why human:** The `models.ParseModeMarkdown` constant is documented as "MarkdownV2" in the library, but visual confirmation of correct rendering requires the Telegram client.

#### 3. Graceful Shutdown Under Load

**Test:** Start the service with an active dispatcher processing messages, send SIGTERM, verify no message is duplicated or lost.
**Expected:** HTTP server drains in-flight requests (30s window), dispatcher stops after current message completes, database closes cleanly.
**Why human:** Cannot verify shutdown sequencing behavior without a live process under actual load.

---

### Build and Test Results

- `go build ./...`: PASS (zero errors)
- `go vet ./...`: PASS (zero warnings)
- `go test ./internal/config/ -v -count=1`: PASS (15/15 tests)
- All commit hashes from summaries verified in git log: 3ff3f35, 285f017, 2415e66, c00a3b0, e0575eb, 7c98059, db98967, 4e0bfdb, 05afb42

---

### Summary

Phase 2 goal is fully achieved. All 12 observable truths are verified against the actual codebase (not SUMMARY claims). Every artifact exists, is substantive (not a stub), and is wired into the runtime. All 11 key links trace cleanly from source to destination. All 7 requirement IDs are satisfied with direct code evidence.

Critical implementation correctness verified:
- BearerAuth uses `db.HashToken` (same function as `SeedKeys`) — no hash divergence possible
- `WriteHeader` is called before `json.Encode` in `WriteJSON` — no spurious 200 responses
- Dispatcher uses `models.ParseModeMarkdown` ("MarkdownV2"), not `ParseModeMarkdownV1` ("Markdown")
- Graceful shutdown uses `context.WithTimeout(context.Background(), 30*time.Second)` — not inherited from already-cancelled parent context
- `errors.Is(err, http.ErrServerClosed)` guards the ListenAndServe goroutine correctly
- Tags use literal `#` prefix, not passed through `EscapeMarkdown` (which would produce `\#`)
- Exponential backoff formula `baseDelay * 1<<uint(attempt-1)` produces 2s/4s/8s/16s for attempts 1-4

Three items flagged for human verification (visual Telegram rendering, live delivery, shutdown under load) — all automated checks pass.

---

_Verified: 2026-02-21T20:00:00Z_
_Verifier: Claude (gsd-verifier)_
