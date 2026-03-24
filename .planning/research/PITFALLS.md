# Pitfalls Research

**Domain:** Self-hosted VPS notification hub (Go + SQLite + Telegram Bot API + CLI)
**Researched:** 2026-02-20 (v1.0 MVP) | Updated: 2026-03-23 (v1.1 Setup Wizard)
**Confidence:** HIGH (SQLite/Go findings HIGH via multiple verified sources; Telegram API findings MEDIUM via official docs + community; bubbletea TUI pitfalls HIGH for stdin/permissions, MEDIUM for async patterns)

---

## v1.1 Setup Wizard Pitfalls (bubbletea TUI)

These pitfalls are specific to adding an interactive `jaimito setup` bubbletea TUI wizard to the existing cobra CLI. They are the primary concern for the current milestone.

---

### Pitfall W1: Bubbletea Hangs When Stdin is a Pipe (curl | bash)

**What goes wrong:**
When `install.sh` runs via `curl -sL ... | bash`, bash's stdin is occupied by the pipe carrying the script. Any program launched from that script that reads from stdin — including a bubbletea TUI — reads from the pipe (already exhausted or closed) instead of the keyboard. The TUI either hangs waiting for input that never comes or immediately receives EOF and exits without rendering anything.

**Why it happens:**
Bash inherits stdin from the calling shell. With `curl | bash`, stdin is the curl pipe. Child processes (`jaimito setup`) inherit that same stdin. Bubbletea opens stdin for keyboard input by default, so it gets the exhausted pipe, not the terminal.

**How to avoid:**
Redirect stdin from `/dev/tty` explicitly in `install.sh` when calling `jaimito setup`:

```bash
jaimito setup --config "$CONFIG_FILE" < /dev/tty
```

This forces jaimito's stdin to be the real terminal device, bypassing the pipe. Additionally, bubbletea should detect non-interactive stdin **before** launching the program:

```go
if !term.IsTerminal(int(os.Stdin.Fd())) {
    fmt.Fprintln(os.Stderr, "jaimito setup requiere una terminal interactiva.")
    os.Exit(1)
}
```

The `< /dev/tty` redirect in `install.sh` must come first — the `IsTerminal` check is a fallback for non-pipe non-TTY cases.

**Warning signs:**
- TUI renders nothing and exits immediately when run from install.sh
- Works fine when running `jaimito setup` directly in a terminal
- `echo $?` shows 0 (stdin read EOF gracefully) or program hangs indefinitely

**Phase to address:** Phase 1 (cobra command scaffolding) — the `< /dev/tty` redirect and the `IsTerminal` guard must be implemented and tested before any TUI step work begins.

---

### Pitfall W2: Bubbletea Output is Invisible or Unstyled When Stdout is Redirected

**What goes wrong:**
Even with the `< /dev/tty` fix for stdin, if anything in the calling script redirects stdout (e.g., `jaimito setup > logfile`), bubbletea renders the TUI to the redirected file instead of the terminal. The user sees nothing. Additionally, lipgloss loses color support because `termenv` detects stdout is not a terminal and strips all ANSI codes — the wizard renders as plain unstyled text.

**Why it happens:**
Bubbletea writes TUI output to stdout by default. `termenv` (which powers lipgloss) auto-detects terminal capabilities from the output file descriptor. A non-TTY stdout means no colors, no box-drawing characters. This is a documented known issue in bubbletea issue #860, with no automatic fix from the framework.

**How to avoid:**
Open `/dev/tty` explicitly for TUI output when stdout is not a TTY, and set the lipgloss renderer to match:

```go
var output *os.File
if !term.IsTerminal(int(os.Stdout.Fd())) {
    tty, err := os.OpenFile("/dev/tty", os.O_WRONLY, 0)
    if err == nil {
        output = tty
        defer tty.Close()
        lipgloss.SetDefaultRenderer(lipgloss.NewRenderer(tty))
    }
} else {
    output = os.Stdout
}
p := tea.NewProgram(model, tea.WithInput(ttyIn), tea.WithOutput(output))
```

For jaimito's case install.sh does not redirect stdout, so this is lower priority — but the pattern must be known before adding `tea.WithOutput()`.

**Warning signs:**
- Colors work in direct invocation but not inside install.sh when stdout is piped
- TUI box drawing characters appear as garbage in a log file
- lipgloss renders plain text despite a color terminal being connected via `/dev/tty`

**Phase to address:** Phase 1 — implement and test inside a simulated `curl | bash` flow before building wizard steps.

---

### Pitfall W3: Stale Async Validation Response Applied to Wrong Step State

**What goes wrong:**
The user types a bot token and triggers validation (a `tea.Cmd` that calls the Telegram API). Before the response arrives, the user presses Escape to go back and retypes the token. When the first validation response finally arrives, `Update()` applies it to the current model state — which now belongs to a different input. Result: the validation result from the old token is displayed as if it belongs to the new one. This can falsely show a bad token as valid or a good token as invalid.

**Why it happens:**
Bubbletea commands run as fire-and-forget goroutines. There is no built-in mechanism to cancel an in-flight command or correlate a response to the request that triggered it. Commands execute concurrently and return messages in non-deterministic order.

**How to avoid:**
Track a validation sequence number in the model. Each time a new validation starts, increment the counter. The validation `tea.Cmd` captures the expected counter value in its closure. In `Update()`, discard any validation response whose counter does not match the current model counter:

```go
type validationResultMsg struct {
    seq  int    // sequence number when this validation was launched
    err  error
    info BotInfo
}

// In Update():
case validationResultMsg:
    if msg.seq != m.validationSeq {
        return m, nil // stale response — discard
    }
    // apply result
```

**Warning signs:**
- Validation shows "valid" for a token the user already cleared
- Error from a previous token appears after entering a new one
- Spinner continues showing "validating" state longer than the actual network latency

**Phase to address:** Phase 2 (BotToken and ChannelGeneral validation steps) — implement the sequence counter pattern from the first step that does async validation, before any subsequent step is built.

---

### Pitfall W4: No Context Timeout in tea.Cmd Goroutines — Slow Network Hangs Wizard

**What goes wrong:**
A `tea.Cmd` that calls the Telegram API has no timeout. On a VPS with a misconfigured firewall or a temporarily slow Telegram endpoint, the HTTP call blocks indefinitely. The spinner keeps spinning, the user cannot advance or exit cleanly (Ctrl+C exits bubbletea but the goroutine leaks until the OS reclaims it — which may never happen if the server has a 30+ minute connection timeout).

**Why it happens:**
Bubbletea intentionally leaks command goroutines on shutdown (documented design decision to prevent hang on exit). `tea.Cmd` functions receive no context from the framework. Without an explicit timeout, HTTP calls run to completion regardless of whether the program has exited. This is especially bad for validation steps that run multiple times per session.

**How to avoid:**
Create a context with timeout **inside** every validation `tea.Cmd`:

```go
func validateTokenCmd(token string, seq int) tea.Cmd {
    return func() tea.Msg {
        ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
        defer cancel()
        bot, info, err := telegram.ValidateTokenWithInfo(ctx, token)
        return validationResultMsg{seq: seq, bot: bot, info: info, err: err}
    }
}
```

Do NOT attempt to pass a context from the cobra command through the model — bubbletea provides no mechanism for this. Use `context.WithTimeout` inside the Cmd closure.

**Warning signs:**
- Spinner hangs for >15 seconds during validation
- After Ctrl+C, `ps aux | grep jaimito` shows the process still running
- Wizard becomes unresponsive only on specific VPS network configurations

**Phase to address:** Phase 2 (all validation commands) — every `tea.Cmd` that calls external APIs must include a 15-second `context.WithTimeout`.

---

### Pitfall W5: Panic in a tea.Cmd Goroutine Leaves Terminal in Raw Mode

**What goes wrong:**
If a nil pointer dereference or other panic occurs inside a `tea.Cmd` function (e.g., accessing a nil `*bot.Bot` instance from a failed validation), bubbletea's panic recovery does not catch it. The terminal remains in raw mode: no echo, no line buffering, control characters appear as `^C` instead of triggering signals. The user's shell appears completely broken until they run `reset`.

**Why it happens:**
Bubbletea wraps `Update()` and `View()` in panic recovery, but `tea.Cmd` functions run in separate goroutines that are not wrapped. A panic in those goroutines propagates to the Go runtime and kills the process, bypassing the deferred terminal cleanup. This is a known architectural limitation.

**How to avoid:**
Wrap all `tea.Cmd` implementations in a deferred panic recovery that returns a safe error message:

```go
return func() tea.Msg {
    defer func() {
        if r := recover(); r != nil {
            // return error msg instead of crashing
        }
    }()
    // ...validation logic
}
```

Additionally, nil-check all values before use: the `*bot.Bot` returned from `ValidateToken` may be nil on error. Never dereference without checking.

**Warning signs:**
- Terminal becomes unresponsive after wizard exits during certain edge cases
- No error message shown before exit (process just dies)
- Reproducible when providing an invalid token that causes `bot.New()` to return nil

**Phase to address:** Phase 2 (validation commands) and Phase 3 (config writing) — any `tea.Cmd` that touches external state or can produce nil values needs panic recovery.

---

### Pitfall W6: os.WriteFile Does Not Guarantee 0600 Permissions on Existing Files

**What goes wrong:**
The config file is written with `os.WriteFile(path, data, 0600)`. However, Go's documentation states: "If the file does not exist, WriteFile creates it with permissions perm (before umask); otherwise WriteFile truncates it before writing, **without changing permissions**." If the file previously existed with permissions 0644 (e.g., copied from `config.example.yaml` by a previous install.sh), it remains 0644 after the wizard writes the bot token and API key into it.

Additionally, even for new files, the umask modifies the requested permissions. With the default root umask of `0022`, `0600 & ~0022 = 0600` (fine). But with a custom umask, the result diverges.

**Why it happens:**
Developers test with a fresh install (no prior config file) and get 0600. The case of "editing existing config" creates a world-readable secrets file that silently passes all local tests.

**How to avoid:**
Always call `os.Chmod` explicitly after writing, regardless of whether the file is new or existing:

```go
if err := os.WriteFile(path, data, 0600); err != nil {
    return err
}
return os.Chmod(path, 0600)
```

For directory creation (`/etc/jaimito/`), use `os.MkdirAll(dir, 0755)` followed by `os.Chmod(dir, 0755)` to guarantee permissions unaffected by umask.

**Warning signs:**
- `ls -la /etc/jaimito/config.yaml` shows `-rw-r--r--` (0644) after running setup on an existing config
- Bot token readable by any local user on the VPS
- gosec linter flags G306 on the WriteFile call without the accompanying Chmod

**Phase to address:** Phase 3 (config writing) — the `os.Chmod` call must be explicit and tested with a pre-existing 0644 file.

---

### Pitfall W7: Cobra PersistentPreRun Tries to Load Config Before It Exists

**What goes wrong:**
The existing `root.go` cobra command may call `config.Load(cfgPath)` in a `PersistentPreRun` or early in the `RunE` of subcommands. When `jaimito setup` runs before any config exists, this produces a "config file not found" error from the root command before the setup wizard even launches.

**Why it happens:**
The existing cobra subcommands (`serve`, `send`, `wrap`) all need a valid config. If a `PersistentPreRun` was added to validate config for all subcommands, `jaimito setup` inherits that hook and fails before the wizard can create the config.

**How to avoid:**
The `setup` command must skip all config-loading hooks. Either:
1. Do not put config loading in `PersistentPreRun` (current codebase does not appear to do this — verify before assuming it's safe)
2. Add a guard in the hook: if cobra's active command is `setup`, skip config loading

Reviewing `root.go` confirms config loading is only done inside individual command `RunE` functions, not in a `PersistentPreRun`. Verify this does not change when adding the setup command.

**Warning signs:**
- Running `jaimito setup` on a fresh system prints "config not found" before the wizard appears
- `jaimito setup` exits with code 1 immediately without displaying the welcome screen

**Phase to address:** Phase 1 (cobra command scaffolding) — verify cobra hooks do not interfere with setup before building any TUI.

---

### Pitfall W8: bot.New() Does Not Validate the Token — getMe Must Be Called Explicitly

**What goes wrong:**
`telegram.ValidateToken()` calls `bot.New(token)` and returns the bot instance as proof of validation. However, `go-telegram/bot`'s `bot.New()` only parses the token format and creates the client struct — it does NOT make a network call to confirm the token is valid. Validation passes with any syntactically correct but invalid token. The wizard advances to the next step showing "token valid", but `SendMessage` later fails.

**Why it happens:**
The existing `ValidateToken` function was written for startup validation where the server subsequently makes API calls. For the wizard, the validation step must be the explicit network check.

**How to avoid:**
The new `telegram.ValidateTokenWithInfo(ctx, token)` function designed for the wizard must call `bot.GetMe(ctx)` (or equivalent) after `bot.New()`:

```go
func ValidateTokenWithInfo(ctx context.Context, token string) (*bot.Bot, BotInfo, error) {
    b, err := bot.New(token)
    if err != nil {
        return nil, BotInfo{}, fmt.Errorf("invalid token format: %w", err)
    }
    me, err := b.GetMe(ctx)
    if err != nil {
        return nil, BotInfo{}, fmt.Errorf("token rejected by Telegram: %w", err)
    }
    return b, BotInfo{Username: me.Username, DisplayName: me.FirstName}, nil
}
```

**Warning signs:**
- Wizard accepts a token like `1234567890:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA` (valid format, invalid token)
- Step 1 shows "token valid" but test notification in the final step fails
- Validation passes with no network activity visible in `tcpdump`

**Phase to address:** Phase 2 (BotToken step) — write and test `ValidateTokenWithInfo` explicitly, not just `bot.New()`.

---

## v1.0 MVP Pitfalls (original)

These pitfalls addressed the core notification hub. Kept for reference during ongoing development.

---

### Pitfall 1: SQLite DEFERRED Transactions Ignore busy_timeout on Write Upgrades

**What goes wrong:**
You configure `PRAGMA busy_timeout = 5000` expecting SQLite to wait up to 5 seconds when the database is locked. But when a read transaction (BEGIN DEFERRED) needs to upgrade to a write, SQLite returns SQLITE_BUSY **immediately** — the timeout is not honoured. The dispatcher loop and the HTTP ingestor both try to write concurrently, causing one to fail silently or log a spurious error.

**Why it happens:**
Go's `database/sql` opens transactions as BEGIN DEFERRED by default. When two goroutines both start reading and then try to write (e.g., the dispatcher updating `status` while the ingestor inserts a new row), the one that arrives second cannot upgrade and fails at once. Developers set `busy_timeout` expecting it to fix all locking issues, but it only applies to situations where the lock is held by an external process, not to in-process upgrade conflicts.

**How to avoid:**
- Use `BEGIN IMMEDIATE` for any transaction that will write, not BEGIN DEFERRED. In Go: `db.BeginTx(ctx, &sql.TxOptions{})` does not help — use a raw `EXEC("BEGIN IMMEDIATE")` or a dedicated write connection.
- **Simpler for this project:** limit the write connection pool to `MaxOpenConns(1)` with a single writer goroutine and use read replicas for readers. This serialises all writes at the application level and eliminates the upgrade race entirely.
- Always set `PRAGMA busy_timeout = 5000` as a floor, but do not rely on it as the only concurrency guard.

**Warning signs:**
- Log lines containing `SQLITE_BUSY` or `database is locked` appearing intermittently under load.
- Dispatcher missing messages that are clearly in the queue.
- Tests pass in isolation but fail when run in parallel.

**Phase to address:** MVP (Phase 1) — get connection pool config right before writing any business logic.

---

### Pitfall 2: CGO Dependency Breaks Single-Binary Build Promise

**What goes wrong:**
The PRD promises a single statically-linked binary for easy deployment. `mattn/go-sqlite3` requires CGO (`CGO_ENABLED=1`), which means: the binary is not fully static, cross-compilation to `linux/arm64` from a `linux/amd64` host requires a cross-compiler, and Docker multi-stage builds fail silently when the builder image lacks `libsqlite3-dev`.

**Why it happens:**
Developers choose `mattn/go-sqlite3` because it is the most Googled Go SQLite library, without realising it embeds the SQLite C amalgamation via CGO.

**How to avoid:**
Choose `modernc.org/sqlite` (pure Go, CGO-free port) from the start. It supports WAL mode, achieves comparable performance for the expected load, and produces a truly static binary with `CGO_ENABLED=0`. (Already resolved in this project — documented for context.)

**Warning signs:**
- `go build` works locally but fails in CI with "cgo: C compiler not found."
- Binary size unexpectedly large from embedded C runtime.

**Phase to address:** MVP (Phase 1) — select the driver once; migrating later requires changing all query code.

---

### Pitfall 3: Telegram 429 Rate Limit Kills All Bot Notifications, Not Just Bursts

**What goes wrong:**
A burst of alerts triggers Telegram's rate limit. Telegram responds with HTTP 429 and a `retry_after` field. If the dispatcher does not respect `retry_after` and immediately retries, Telegram blacklists the bot IP for 30 seconds — **all** bot notifications fail during that window.

**How to avoid:**
- Parse `retry_after` from every 429 response and sleep exactly that duration plus jitter before re-queuing.
- As of Bot API 8.0 (November 2025), 429 responses include `adaptive_retry` — honour this field.
- Implement a per-dispatcher token bucket: max 1 message/second per chat, max 20/minute for groups.

**Phase to address:** MVP dispatcher (Phase 1). The rate limit will be hit in the first week of real use if cron jobs run at round minutes.

---

### Pitfall 4: Dispatcher Goroutine Loops Without Bounded Retry = Poison Pill Messages

**What goes wrong:**
A message with a malformed `targets_override` or an invalid chat_id gets enqueued. The dispatcher retries it repeatedly. If the retry logic has a bug, the message stays in `dispatching` forever, consuming dispatcher cycles on every poll iteration.

**How to avoid:**
- Add a hard maximum on `retry_count` (e.g., 10) that marks the message `failed` and stops dispatching.
- Log and move exhausted messages to a `dead_letter` status.
- Add a startup reclaim for rows stuck in `dispatching` for more than `2 * dispatch_timeout`.

**Phase to address:** MVP (Phase 1) — implement reclaim logic before going to production.

---

### Pitfall 5: No Request Body Size Limit on Webhook Allows Memory Exhaustion

**What goes wrong:**
Go's `net/http` reads request bodies on demand without a built-in size limit. A misconfigured client or malicious actor sends a 50MB payload to `POST /api/v1/notify`. The server buffers the entire body before parsing JSON, exhausting the 50MB memory budget.

**How to avoid:**
Wrap every request body with `http.MaxBytesReader(w, r.Body, 64*1024)` (64KB). Return 413 on overflow. Set `ReadTimeout` and `WriteTimeout` on the HTTP server.

**Phase to address:** MVP (Phase 1) — three lines of code at handler setup.

---

### Pitfall 6: Plain-Text Secrets in YAML Config Committed to Git

**What goes wrong:**
`config.yaml` contains `bot_token: "..."` and API keys. A developer does `git add .` and pushes. The Telegram bot token is now in git history permanently.

**How to avoid:**
- Ship `config.yaml.example` with placeholder values. Add `config.yaml` and `*.db` to `.gitignore` before the first commit.
- Enforce `0600` on `config.yaml` at startup.

**Phase to address:** Before first commit (pre-MVP).

---

### Pitfall 7: WAL File Grows Unboundedly Without Checkpointing

**What goes wrong:**
If the database is under constant write load and a long-running read transaction is open, SQLite cannot checkpoint. The WAL file grows to hundreds of megabytes.

**How to avoid:**
- Keep read transactions short — close before making network calls.
- Set `PRAGMA wal_autocheckpoint = 1000`.
- Periodically run `PRAGMA wal_checkpoint(PASSIVE)` from a background goroutine.

**Phase to address:** Phase 2 (after MVP is stable).

---

### Pitfall 8: Notification Flooding When the Monitored Service Has a Loop Bug

**What goes wrong:**
A cron job has a bug and runs in a tight loop, sending 60+ notifications per minute, triggering Telegram rate limiting and notification fatigue.

**How to avoid:**
- Implement deduplication windows per `dedupe_key`.
- Add per-channel rate limiting.
- For `jaimito wrap`: suppress repeated failures.

**Phase to address:** Phase 2 (deduplication + rate limiting).

---

### Pitfall 9: CLI Tool API Key Stored in Shell History or Process List

**What goes wrong:**
Users invoke `jaimito send -k sk-abc123 "message"` with the API key as a CLI argument. The key appears in `~/.bash_history` and `ps aux`.

**How to avoid:**
Read API key from `JAIMITO_API_KEY` env var or a config file, not a CLI flag. (Already implemented — documented for context.)

**Phase to address:** MVP CLI (Phase 1).

---

### Pitfall 10: Self-Monitoring Bootstrap Problem

**What goes wrong:**
jaimito runs on the same VPS it monitors. If jaimito itself crashes, there is no notification.

**How to avoid:**
Use systemd `Restart=on-failure` and `WatchdogSec=30`. Document as known limitation — external monitoring (UptimeRobot, second VPS) is the real solution.

**Phase to address:** MVP deployment (Phase 1).

---

### Pitfall 11: Telegram MarkdownV2 Escaping Breaks Message Rendering

**What goes wrong:**
MarkdownV2 requires escaping 18 special characters. Alert messages naturally contain these characters. Unescaped characters cause the entire message to fail with 400 Bad Request — the message is lost.

**How to avoid:**
Implement `EscapeMarkdownV2(s string) string` or switch to `parse_mode: HTML`. Test with pathological inputs containing `.`, `-`, `_`, `(`, `)`, `!`.

**Phase to address:** MVP dispatcher (Phase 1).

---

## Technical Debt Patterns

| Shortcut | Immediate Benefit | Long-term Cost | When Acceptable |
|----------|-------------------|----------------|-----------------|
| Skip `BEGIN IMMEDIATE` — use default DEFERRED transactions | Less code | Intermittent SQLITE_BUSY under concurrent load | Never |
| Use `mattn/go-sqlite3` (CGO) | More documentation | Cross-compilation requires toolchain; non-static binary | Acceptable if ARM not planned and CI already has CGO |
| Store secrets directly in `config.yaml` | Faster initial setup | Accidental git commit exposure | Never |
| No `MaxBytesReader` on webhook handler | Fewer lines | OOM from large payloads | Never |
| Hardcode spinner style in wizard | Simpler initial code | Harder to theme later | Acceptable for v1.1 |
| Use `SetupData` as shared pointer mutated directly by steps | Simpler than message-passing | Steps are not independently testable | Acceptable if teatest coverage added |
| Skip context propagation to bot instance reuse across steps | Less plumbing | Bot instance can go stale if token changes mid-flow | Never — implement token refresh on token change |
| Skip atomic write (write-then-rename) for config | Simpler code | Power failure mid-write leaves corrupted config | Acceptable for v1.1 root-level writes (low risk) |
| Single 15s timeout for all Telegram API calls | Less configuration | `getUpdates` can be slower than `getMe` | Acceptable |

---

## Integration Gotchas

| Integration | Common Mistake | Correct Approach |
|-------------|----------------|------------------|
| Telegram Bot API | Using `parse_mode: MarkdownV2` without escaping all 18 special characters | Implement `EscapeMarkdownV2()` or switch to HTML |
| Telegram Bot API | Ignoring `retry_after` in 429 responses | Parse `retry_after`; store `next_retry_at` in SQLite |
| Telegram Bot API | Not handling "bot was blocked" (403) or "chat not found" (400) — retrying forever | Detect permanent errors by code; mark `failed` immediately |
| Telegram Bot API (wizard) | `bot.New()` does NOT call getMe — token appears valid but is rejected | Call `bot.GetMe(ctx)` explicitly after `bot.New()` |
| `go-telegram/bot` + `GetChat` | Chat_id where bot hasn't been added returns 400, not timeout | Wrap with message: "Asegurate de que el bot esté en el grupo" |
| `gopkg.in/yaml.v3` marshal | Struct with unexported fields silently omits them | Verify all `config.Config` fields have yaml struct tags |
| `bubbles/textinput` + `Focus()` | Forgetting to call `Focus()` means input never captures keystrokes | Call `ti.Focus()` in `Init()` or on step transition |
| cobra `PersistentPreRun` | Root hook loading config runs before `jaimito setup` (config doesn't exist yet) | Guard: skip config loading if active command is `setup` |
| SQLite (WAL) | Opening database file with multiple processes simultaneously | Use WAL mode; `_busy_timeout=5000` in DSN; single process owns the file |
| SQLite (WAL) | Not setting `_foreign_keys=on` in DSN | Add `?_foreign_keys=on&_journal_mode=WAL&_busy_timeout=5000` to DSN |
| net/http webhook | Not returning 202 immediately and processing synchronously | Enqueue to SQLite, return 202, dispatch asynchronously |

---

## Performance Traps

| Trap | Symptoms | Prevention | When It Breaks |
|------|----------|------------|----------------|
| Full table scan on `messages` for dispatch polling | Dispatcher slows as queue grows | Index `(status, next_retry_at)` at schema creation | Noticeable at ~10,000 rows |
| WAL file not checkpointed | Reads slow; `-wal` file grows to hundreds of MB | Periodic `PRAGMA wal_checkpoint(PASSIVE)`; keep reads short | Within days of continuous write load |
| Dispatcher holds read transaction across Telegram HTTP call | Blocks WAL checkpoint; latency spikes | Read batch → close → dispatch → open new transaction to update | Whenever Telegram is slow |
| Blocking Telegram API call in wizard `Update()` directly | TUI freezes during validation | Only call external APIs inside `tea.Cmd` | Immediately — first validation |
| Multiple simultaneous validation requests from rapid typing | N API calls fired per keystroke | Debounce: only fire validation cmd on Enter, not each keystroke | As soon as user types fast |
| Re-rendering entire wizard with expensive lipgloss operations | High CPU during spinner animation | Keep `View()` lightweight; cache rendered strings for static content | At ~10 animation ticks/second |

---

## Security Mistakes

| Mistake | Risk | Prevention |
|---------|------|------------|
| API key comparison with `==` (non-constant time) | Timing oracle enables key enumeration | Use `hmac.Equal([]byte(provided), []byte(expected))` |
| Config written as 0644 (world-readable) | Bot token and API key readable by all users | `os.Chmod(path, 0600)` after write, regardless of previous permissions |
| Parent directory `/etc/jaimito/` created with 0777 | Other users can replace or read config | Create with `0755` and verify with `os.Chmod` |
| Bot token logged via `slog` during validation | Token appears in system logs | Never log the token value; log only "token validated" or "validation failed" |
| Bot token in process environment visible in `/proc/PID/environ` | Leaked in crash dumps | Load from file at startup; clear from env after reading |
| Webhook endpoint on public interface without IP restriction | Abuse from public internet | Bind to `127.0.0.1` by default; document reverse proxy requirement |

---

## UX Pitfalls

| Pitfall | User Impact | Better Approach |
|---------|-------------|-----------------|
| Showing full bot token in pre-fill for "edit existing config" | Token exposed if screen recorded | Mask as `****...xxxx` (last 4 chars visible only) |
| Not confirming API key was copied before advancing | User loses credentials, must re-run setup | Block advancement until user types "s" — already in spec, must not be simplified away |
| Silently advancing to next step immediately after validation success | User misses bot name / chat title confirmation | Show success state for 1-2 seconds or require Enter to advance |
| No way to cancel during async validation | User stuck watching spinner on slow network | Bubbletea handles Ctrl+C globally — ensure spinner step doesn't block event loop |
| Step "Volver a revisar" forces re-validation of all steps | Frustrating for changing one field | Jump-to-step selector is in spec — implement it, do not simplify away |
| Error messages truncated by terminal width | User cannot read full Telegram API error | Use `lipgloss.NewStyle().Width(termWidth - 4).Render(err.Error())` to wrap |
| `jaimito wrap` sends notification on every success | Cron noise; notification fatigue | Default to notify-on-failure only; add `--always` flag |
| Long truncated `body` from piped commands overwhelms Telegram | Telegram 4096-char limit; truncated context | Truncate at 3800 chars and append "… [truncated, N total chars]" |

---

## "Looks Done But Isn't" Checklist

### Setup Wizard (v1.1)
- [ ] **stdin detection:** Verify `< /dev/tty` redirect in install.sh is before `jaimito setup` call — check `bash -x install.sh` output
- [ ] **Terminal detection:** `term.IsTerminal(int(os.Stdin.Fd()))` runs before `tea.NewProgram()`, not inside `Init()`
- [ ] **Permissions:** `ls -la /etc/jaimito/config.yaml` after setup on a pre-existing 0644 file must show `-rw-------` (0600)
- [ ] **Telegram validation:** Providing a valid-format but invalid token must fail — not silently pass because `bot.New()` succeeded
- [ ] **Config round-trip:** Generated YAML passes through `config.Load()` before writing — not just marshalled
- [ ] **Sequence counter:** Fire two rapid validations — confirm only the last result is applied
- [ ] **Ctrl+C during validation:** Press Ctrl+C while spinner is running — confirm terminal echo is restored
- [ ] **Existing config:** Run `jaimito setup` when config already exists — must offer the three-option menu, not overwrite silently
- [ ] **install.sh test:** Run `curl -sL [url] | bash` — confirm TUI appears (not hangs, not crashes)
- [ ] **Non-root execution:** Run setup as non-root attempting to write to `/etc/jaimito/` — must fail clearly, not partially write

### Core Notification Hub (v1.0)
- [ ] **SQLite WAL mode:** Verify `PRAGMA journal_mode` returns `wal` after opening.
- [ ] **Telegram rate limit:** Confirm dispatcher reads `retry_after` from 429 responses and stores in `next_retry_at`.
- [ ] **Retry exhaustion:** Messages with `retry_count >= max_retries` transition to `failed`.
- [ ] **Stuck dispatcher reclaim:** Kill dispatcher mid-dispatch; verify message is reclaimed on restart.
- [ ] **Signal handling:** `SIGTERM` causes clean shutdown; no WAL corruption.
- [ ] **Webhook security:** Request with no `Authorization` header returns 401.
- [ ] **Body size limit:** 10MB POST returns 413 without crashing.
- [ ] **CLI exit codes:** `jaimito send` with server stopped exits with code 1.
- [ ] **Config permissions:** Service refuses to start if `config.yaml` has mode 0644.
- [ ] **MarkdownV2 escaping:** Message with `_`, `.`, `-`, `(`, `)`, `!` arrives in Telegram without 400 error.

---

## Recovery Strategies

| Pitfall | Recovery Cost | Recovery Steps |
|---------|---------------|----------------|
| Terminal left in raw mode after wizard panic | LOW | User runs `reset` in shell; add note to README troubleshooting |
| Config written with wrong permissions | LOW | `chmod 0600 /etc/jaimito/config.yaml`; add to troubleshooting docs |
| Bot token exposed in system logs | MEDIUM | Rotate bot token via BotFather; revoke old; update config |
| Stale validation shown to user | LOW | User retries from same step — no data corruption, cosmetic only |
| Config written as partial YAML (crash mid-write) | MEDIUM | Delete corrupted file; re-run `jaimito setup` |
| curl \| bash install hangs on TUI | MEDIUM | Kill process; run `bash install.sh` directly; document limitation |
| Secrets committed to git | HIGH | Rotate bot token; rotate API key; `git filter-repo` to purge; force-push; notify forks |
| SQLite WAL corruption from hard kill | MEDIUM | Stop service; `sqlite3 jaimito.db "PRAGMA integrity_check"`; restore from backup |
| Stuck messages in `dispatching` status | LOW | `UPDATE messages SET status='queued', retry_count=0 WHERE status='dispatching'`; restart |
| Telegram bot banned (repeated 429 abuse) | MEDIUM | Wait 1-24h for unban; implement rate limiting; test with `getMe` |

---

## Pitfall-to-Phase Mapping

| Pitfall | Prevention Phase | Verification |
|---------|------------------|--------------|
| Stdin pipe hang (curl\|bash) | v1.1 Phase 1: Cobra command scaffold | `echo "" \| ./jaimito setup` must print error and exit 1 |
| Bubbletea output invisible/unstyled | v1.1 Phase 1: Cobra command scaffold | Test inside subshell with redirected stdout |
| Stale async validation response | v1.1 Phase 2: Bot token + channel validation | Fire two rapid validations; only last result applies |
| Context/goroutine leak on Ctrl+C | v1.1 Phase 2: All validation tea.Cmd functions | After Ctrl+C, no lingering process; timeout after max 15s |
| Panic in tea.Cmd leaves raw terminal | v1.1 Phase 2: Validation commands | Inject nil bot; confirm graceful error, not crash |
| Config written with wrong permissions | v1.1 Phase 3: Config writing | `ls -la` after write confirms 0600; test with pre-existing 0644 file |
| Umask overrides intended permissions | v1.1 Phase 3: Config writing | Test with `umask 0` and `umask 0077` before running setup |
| Cobra PersistentPreRun loads missing config | v1.1 Phase 1: Cobra command scaffold | Run `jaimito setup` on fresh system; no "config not found" error |
| bot.New() lazy init misidentified as validation | v1.1 Phase 2: Telegram validate commands | Invalid token must return error, not succeed |
| SQLite DEFERRED transaction SQLITE_BUSY | v1.0 Phase 1: Database layer | Concurrent integration test: 2 goroutines write simultaneously |
| CGO cross-compilation | v1.0 Phase 1: Build setup | `make build-arm64` succeeds on amd64 host without extra toolchain |
| Telegram 429 rate limit | v1.0 Phase 1: Telegram dispatcher | Send 35 messages in 1 second; verify backoff behaviour |
| Dispatcher poison pill / stuck messages | v1.0 Phase 1: Queue + dispatcher | Kill process mid-dispatch; restart; verify delivery or clean failure |
| Webhook body size / timeout | v1.0 Phase 1: HTTP server setup | 10MB POST returns 413 without crash |
| Plain-text secrets in git | Pre-v1.0 Phase 1: Project setup | `git log --all -- config.yaml` returns empty |
| MarkdownV2 escaping | v1.0 Phase 1: Telegram dispatcher | Integration test with special-character payload |

---

## Sources

**Setup Wizard (v1.1):**
- [bubbletea issue #860: stdout not a terminal workaround](https://github.com/charmbracelet/bubbletea/issues/860) — MEDIUM confidence (open issue, community workaround documented)
- [DeepWiki: Concurrency and Goroutines in bubbletea](https://deepwiki.com/charmbracelet/bubbletea/5.1-concurrency-and-goroutines) — MEDIUM confidence
- [bubbletea PR #1372: fix deadlock on context cancellation](https://github.com/charmbracelet/bubbletea/pull/1372) — HIGH confidence (merged fix)
- [Tips for building Bubble Tea programs](https://leg100.github.io/en/posts/building-bubbletea-programs/) — MEDIUM confidence (independent practitioner)
- [Loss of input in Charm's Bubbletea](https://dr-knz.net/bubbletea-control-inversion.html) — MEDIUM confidence (independent technical analysis)
- [Commands in Bubble Tea (official Charm blog)](https://charm.land/blog/commands-in-bubbletea/) — HIGH confidence (official)
- [golang/go issue #56173: os.WriteFile is not atomic](https://github.com/golang/go/issues/56173) — HIGH confidence (official Go tracker)
- [golang/go issue #35835: umask and os.WriteFile permissions](https://github.com/golang/go/issues/35835) — HIGH confidence (official Go tracker)
- [linuxvox.com: Why Bash Script Input Prompts Fail via cURL](https://linuxvox.com/blog/execute-bash-script-remotely-via-curl/) — HIGH confidence (well-known bash behavior)

**Core Hub (v1.0):**
- [SQLite Concurrent Writes — tenthousandmeters.com](https://tenthousandmeters.com/blog/sqlite-concurrent-writes-and-database-is-locked-errors/) — HIGH confidence
- [Go + SQLite Best Practices — Jake Gold](https://jacob.gold/posts/go-sqlite-best-practices/) — HIGH confidence
- [Telegram Bot API — Rate Limits documentation](https://core.telegram.org/bots/faq) — HIGH confidence (official)
- [Go net/http timeouts — Cloudflare Engineering](https://blog.cloudflare.com/the-complete-guide-to-golang-net-http-timeouts/) — HIGH confidence
- [Telegram Bot API Errors list](https://github.com/TelegramBotAPI/errors) — MEDIUM confidence (community-maintained)

---
*Pitfalls research for: jaimito — VPS Push Notification Hub (Go + SQLite + Telegram Bot API + CLI + bubbletea TUI wizard)*
*Researched: 2026-02-20 (v1.0 MVP) | Updated: 2026-03-23 (v1.1 Setup Wizard)*
