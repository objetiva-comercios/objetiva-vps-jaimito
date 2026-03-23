---
phase: 01-foundation
plan: 03
subsystem: infra
tags: [go, telegram, systemd, startup, signal, slog]

# Dependency graph
requires:
  - phase: 01-foundation/01-01
    provides: Go module, config package (config.Load, ChannelConfig types)
  - phase: 01-foundation/01-02
    provides: db.Open, db.ApplySchema, db.ReclaimStuck, cleanup.Start

provides:
  - Telegram bot token validation via getMe (internal/telegram.ValidateToken)
  - Telegram chat reachability validation via getChat (internal/telegram.ValidateChats)
  - Full startup sequence in cmd/jaimito/main.go (strict order: config->telegram->db->reclaim->cleanup)
  - Graceful shutdown via signal.NotifyContext (SIGTERM, SIGINT)
  - systemd unit file for Linux VPS deployment at /usr/local/bin/jaimito
  - -config flag with default /etc/jaimito/config.yaml

affects:
  - 02-dispatcher (reuses *bot.Bot instance from ValidateToken)
  - 03-cli (same binary, same startup sequence)

# Tech tracking
tech-stack:
  added:
    - os/signal + syscall (stdlib, signal context for graceful shutdown)
    - flag (stdlib, -config flag parsing)
  patterns:
    - Fail-fast startup: each step exits 1 with slog.Error on failure
    - signal.NotifyContext pattern for SIGTERM/SIGINT graceful shutdown
    - Telegram validation before DB open (auth errors surface early)
    - Startup-then-wait: all setup completes before <-ctx.Done()

key-files:
  created:
    - internal/telegram/client.go
    - systemd/jaimito.service
  modified:
    - cmd/jaimito/main.go

key-decisions:
  - "ChatID in GetChatParams is type `any` — int64 passed directly without conversion"
  - "ValidateToken wraps bot.New error: 'invalid telegram bot token: ...' for clear diagnostics"
  - "Chat deduplication uses map[int64]string (chatID -> first channel name) for error reporting"
  - "Cleanup.Start called before <-ctx.Done() in same goroutine — it launches its own goroutine"

patterns-established:
  - "Pattern: telegram.ValidateToken(ctx, token) -> *bot.Bot reused across validation and later phases"
  - "Pattern: main.go startup sequence comments numbered 1-12 matching PLAN.md strict order"
  - "Pattern: os.Exit(1) after slog.Error for fail-fast, never panic"

requirements-completed: [CONF-03]

# Metrics
duration: 2min
completed: 2026-02-21
---

# Phase 1 Plan 03: Telegram Validation, systemd Unit, and Full Startup Sequence Summary

**Deployable jaimito binary with Telegram token/chat validation via go-telegram/bot, strict 12-step startup sequence, graceful SIGTERM shutdown, and systemd unit for Linux VPS deployment**

## Performance

- **Duration:** 2 min
- **Started:** 2026-02-21T14:31:50Z
- **Completed:** 2026-02-21T14:33:36Z
- **Tasks:** 2
- **Files modified:** 3

## Accomplishments

- Telegram validation package: ValidateToken (getMe) and ValidateChats (getChat with dedup) with clear error messages naming unreachable channels
- main.go wired with strict 12-step startup sequence: logging -> config -> signal ctx -> telegram token -> telegram chats -> db open -> schema -> reclaim -> cleanup -> ready -> wait for shutdown
- systemd unit file with Type=simple, Restart=on-failure, journal logging, and ConditionPathExists guard
- Verified: missing config exits 1 with clear error; invalid token exits 1 at step 5 (fail-fast confirmed)

## Task Commits

Each task was committed atomically:

1. **Task 1: Create Telegram validation package and systemd unit file** - `26538fc` (feat)
2. **Task 2: Wire main.go with full startup sequence and graceful shutdown** - `1a4f84f` (feat)

**Plan metadata:** (this commit)

## Files Created/Modified

- `internal/telegram/client.go` - ValidateToken (calls bot.New/getMe), ValidateChats (dedup + getChat per unique chat_id)
- `systemd/jaimito.service` - systemd unit: Type=simple, Restart=on-failure, journal logging, root execution
- `cmd/jaimito/main.go` - Full 12-step startup sequence with signal.NotifyContext graceful shutdown

## Decisions Made

- `GetChatParams.ChatID` is type `any` in go-telegram/bot v1.19.0 — int64 passed directly without conversion
- Chat deduplication stores `map[int64]string` (chatID to first channel name) so error messages name the channel
- Cleanup goroutine launched before `<-ctx.Done()` — cleanup.Start internally spawns goroutine, main goroutine just waits

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required. The binary requires a valid Telegram bot token and accessible chat IDs in the config file to pass startup validation.

## Next Phase Readiness

- Phase 1 complete: all three plans executed, deployable binary ready
- The *bot.Bot instance from ValidateToken is available for Phase 2 dispatcher reuse
- systemd unit ready to install on any Linux VPS: `sudo cp systemd/jaimito.service /etc/systemd/system/ && sudo systemctl enable --now jaimito`
- No blockers for Phase 2 (HTTP API + Telegram dispatcher)

## Self-Check: PASSED
