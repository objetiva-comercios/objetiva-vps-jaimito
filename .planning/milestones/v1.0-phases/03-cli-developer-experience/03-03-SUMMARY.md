---
phase: 03-cli-developer-experience
plan: 03
subsystem: cli
tags: [cobra, cli, cron, subprocess, notifications, exec]

# Dependency graph
requires:
  - phase: 03-cli-developer-experience
    plan: 01
    provides: cobra scaffold, rootCmd, resolveServer, resolveAPIKey helpers
  - phase: 03-cli-developer-experience
    plan: 02
    provides: internal/client package with Client.Notify method
provides:
  - cmd/jaimito/wrap.go with wrap subcommand that runs subprocess and notifies on failure
affects: []

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "os/exec.CombinedOutput() captures both stdout and stderr for failure notification body"
    - "os.Exit(exitCode) called directly after notification — cobra error handling bypassed to preserve exact exit code"
    - "Notification is best-effort: failure to notify does not change exit behavior, only prints to stderr"
    - "Output truncated at 3500 bytes to fit within Telegram 4096-char message limit with metadata overhead"

key-files:
  created:
    - cmd/jaimito/wrap.go

key-decisions:
  - "os.Exit(exitCode) rather than returning error — cobra RunE error handling would exit with code 1, breaking cron transparency"
  - "Notification best-effort: if API key missing or send fails, wrap still exits with wrapped command's code (logged to stderr)"
  - "Title hardcoded to 'Command failed' — body contains command name and exit code for specificity"
  - "output truncation at maxOutputBytes=3500 leaves room for title, command, exit code prefix, and Telegram formatting overhead"

requirements-completed: [CLI-02]

# Metrics
duration: 1min
completed: 2026-02-23
---

# Phase 3 Plan 03: Wrap Subcommand Summary

**`jaimito wrap` cron job monitor using os/exec subprocess capture with best-effort failure notification and transparent exit code forwarding**

## Performance

- **Duration:** 1 min
- **Started:** 2026-02-23T21:21:47Z
- **Completed:** 2026-02-23T21:22:32Z
- **Tasks:** 1
- **Files modified:** 1

## Accomplishments
- Created `cmd/jaimito/wrap.go` implementing `jaimito wrap -- command [args...]` subcommand
- Subprocess runs via `exec.Command` with `CombinedOutput()` capturing stdout+stderr for failure notification body
- On success (exit 0): exits silently — no output, no notification
- On failure (non-zero exit): sends notification with title "Command failed", body with command name, exit code, and truncated output; then exits with the wrapped command's exact exit code

## Task Commits

Each task was committed atomically:

1. **Task 1: Create wrap subcommand with subprocess execution and failure notification** - `aa4ee34` (feat)

**Plan metadata:** (docs commit — see below)

## Files Created/Modified
- `cmd/jaimito/wrap.go` - wrap subcommand with subprocess execution, output capture, failure notification, and exit code forwarding

## Decisions Made
- `os.Exit(exitCode)` called directly after notification rather than returning an error — cobra's RunE error path would always exit with code 1, breaking transparent exit code forwarding required by cron jobs
- Notification is best-effort: if `resolveAPIKey` fails (no key configured), wrap prints a warning to stderr and exits with the original exit code without attempting notification
- Title hardcoded to `"Command failed"` — the body contains the specific command invocation and exit code, providing full context without over-engineering a configurable title
- `maxOutputBytes = 3500` chosen to leave margin for command name, exit code prefix, "Output:" header, truncation suffix, and Telegram MarkdownV2 formatting overhead within 4096-char limit
- Same `resolveServer`/`resolveAPIKey` helpers as `send` command — identical resolution priority (flag > env > config > default)

## Deviations from Plan

None — plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

Phase 3 is now complete. All three plans executed:
- 03-01: cobra scaffold with server, keys subcommands, and helper functions
- 03-02: HTTP client package and send subcommand
- 03-03: wrap subcommand for cron job failure monitoring

The jaimito binary is feature-complete: `jaimito` (server), `jaimito send`, `jaimito wrap`, `jaimito keys create/list` are all implemented and the project builds cleanly.

## Self-Check: PASSED

- `cmd/jaimito/wrap.go`: FOUND
- `03-03-SUMMARY.md`: FOUND
- Commit `aa4ee34`: FOUND in git history

---
*Phase: 03-cli-developer-experience*
*Completed: 2026-02-23*
