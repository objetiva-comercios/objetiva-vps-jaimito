---
phase: 02-core-pipeline
plan: 03
subsystem: dispatcher
tags: [telegram, markdownv2, retry, backoff, polling, sqlite]

# Dependency graph
requires:
  - phase: 02-01
    provides: "db.Message type, GetNextQueued, MarkDispatching, MarkDelivered, MarkFailed, RecordAttempt, config.ChatIDForChannel"
  - phase: 01-03
    provides: "bot.Bot instance, go-telegram/bot library, telegram package structure"
provides:
  - "telegram.FormatMessage — MarkdownV2 message formatting with emoji prefix, bold title, escaped body, hashtag tags"
  - "dispatcher.Start — 1-second polling loop delivering messages to Telegram with retry and 429 handling"
affects: [02-04, main-wiring, integration-tests]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Traffic light emoji: low=🟢 normal=🟡 high=🔴 per channel priority"
    - "MarkdownV2 escaping: bot.EscapeMarkdown() for all user text; # prefix written literally before escaped tag content"
    - "Exponential backoff: baseDelay * 2^(attempt-1) = 2s, 4s, 8s, 16s for attempts 1-4"
    - "429 handling: type-assert to *bot.TooManyRequestsError, wait exact RetryAfter seconds"
    - "Context-cancellable waits: select { case <-time.After(...): case <-ctx.Done(): return ctx.Err() }"
    - "Silent failure after exhaustion: MarkFailed + slog.Error, return nil (no error propagation)"

key-files:
  created:
    - internal/telegram/format.go
    - internal/dispatcher/dispatcher.go
  modified: []

key-decisions:
  - "Tags use literal # prefix not passed through EscapeMarkdown — # is in shouldBeEscaped set, would produce \\#"
  - "models.ParseModeMarkdown = MarkdownV2 (not ParseModeMarkdownV1 = Markdown old V1 format)"
  - "After exhausted retries dispatcher returns nil, not error — failure is logged and silenced per CONTEXT.md"
  - "dispatcher alias gobot for go-telegram/bot to avoid collision with internal telegram package name"

patterns-established:
  - "Format: emoji + space + bold-title + newline + body (with title); emoji + space + body (without title)"
  - "Dispatch loop: MarkDispatching -> FormatMessage -> SendMessage -> RecordAttempt -> MarkDelivered/retry/MarkFailed"
  - "Goroutine lifecycle: ticker inside goroutine, defer ticker.Stop(), ctx.Done() for clean exit"

requirements-completed: [TELE-01, TELE-02, TELE-03]

# Metrics
duration: 2min
completed: 2026-02-21
---

# Phase 2 Plan 3: Telegram Formatter and Dispatcher Summary

**MarkdownV2 message formatter and 1-second polling dispatcher with 5-attempt exponential backoff and exact 429 retry_after handling**

## Performance

- **Duration:** 2 min
- **Started:** 2026-02-21T19:04:52Z
- **Completed:** 2026-02-21T19:06:03Z
- **Tasks:** 2
- **Files modified:** 2

## Accomplishments

- FormatMessage produces MarkdownV2-compliant output: traffic-light emoji, optional bold title, escaped body, hashtag tags
- Dispatcher polls every 1 second, delivers messages sequentially to preserve ordering
- 429 TooManyRequestsError handled with exact retry_after wait, all other errors use exponential backoff (2s/4s/8s/16s)
- All context waits cancellable for clean shutdown; silent failure after 5 exhausted attempts per CONTEXT.md

## Task Commits

Each task was committed atomically:

1. **Task 1: Create MarkdownV2 message formatter** - `7c98059` (feat)
2. **Task 2: Create dispatcher with polling loop, retry, and 429 handling** - `db98967` (feat)

**Plan metadata:** (docs commit — see below)

## Files Created/Modified

- `internal/telegram/format.go` - FormatMessage: MarkdownV2 formatting with emoji, bold title, escaped body, hashtag tags
- `internal/dispatcher/dispatcher.go` - Start, dispatchNext, dispatch: polling loop with retry and 429 handling

## Decisions Made

- Tags use literal `#` prefix written directly, then `bot.EscapeMarkdown(tag)` for the tag text only — passing `"#tag"` through EscapeMarkdown would produce `\#tag` because `#` is in MarkdownV2's escape set.
- `models.ParseModeMarkdown` is `"MarkdownV2"` (the modern format); `ParseModeMarkdownV1` is `"Markdown"` (old format). Plan comment clarified which to use.
- After 5 exhausted retries the dispatcher returns `nil` (not an error) — the failure is logged at Error level and silenced, per CONTEXT.md locked decision.
- The dispatcher package imports `go-telegram/bot` as `gobot` to avoid a name collision with the internal `telegram` package in the same import list.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Formatter and dispatcher are complete and verified with `go build` and `go vet`
- Ready to wire into `main.go` alongside the HTTP server from plan 02-02
- Blocker from STATE.md resolved: `go-telegram/bot` v1.19.0 `TooManyRequestsError.RetryAfter` is a plain `int` field — no adaptive_retry field present, plain int seconds is sufficient

---
*Phase: 02-core-pipeline*
*Completed: 2026-02-21*
