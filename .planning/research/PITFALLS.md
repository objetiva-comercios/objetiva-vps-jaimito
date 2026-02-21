# Pitfalls Research

**Domain:** Self-hosted VPS notification hub (Go + SQLite + Telegram Bot API + CLI)
**Researched:** 2026-02-20
**Confidence:** MEDIUM-HIGH (SQLite/Go findings HIGH via multiple verified sources; Telegram API findings MEDIUM via official docs + community; architectural patterns MEDIUM via post-mortems and real-world projects)

---

## Critical Pitfalls

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

**Phase to address:** MVP (Phase 1) — get connection pool config right before writing any business logic. Wrong defaults here cause subtle data loss that is hard to debug later.

---

### Pitfall 2: CGO Dependency Breaks Single-Binary Build Promise

**What goes wrong:**
The PRD promises a single statically-linked binary for easy deployment. `mattn/go-sqlite3` requires CGO (`CGO_ENABLED=1`), which means: the binary is not fully static, cross-compilation to `linux/arm64` from a `linux/amd64` host requires a cross-compiler (`gcc-aarch64-linux-gnu`), and Docker multi-stage builds fail silently when the builder image lacks `libsqlite3-dev`.

**Why it happens:**
Developers choose `mattn/go-sqlite3` because it is the most Googled Go SQLite library, without realising it embeds the SQLite C amalgamation via CGO. The binary compiles fine on the dev machine but fails in CI or on a fresh VPS because the linked C runtime differs.

**How to avoid:**
Choose `modernc.org/sqlite` (pure Go, CGO-free port of SQLite C code) from the start. It supports WAL mode, achieves comparable performance for the expected load (hundreds of messages/hour), and produces a truly static binary with `CGO_ENABLED=0 GOARCH=arm64 GOOS=linux go build`. If `mattn/go-sqlite3` is chosen anyway, document the cross-compilation toolchain requirement explicitly and add it to the Makefile — do not discover this during a 2am deployment.

**Warning signs:**
- `go build` works locally but fails in GitHub Actions or Docker with "cgo: C compiler not found."
- Binary size unexpectedly large (>10MB overhead from embedded C runtime).
- Colleague cannot build the project on a different OS without extra setup.

**Phase to address:** MVP (Phase 1) — select the driver once; migrating later requires changing all query code and retesting all SQLite behaviour.

---

### Pitfall 3: Telegram 429 Rate Limit Kills All Bot Notifications, Not Just Bursts

**What goes wrong:**
A burst of alerts (e.g., 5 cron jobs finish simultaneously, or a loop bug sends 30 messages in 2 seconds) triggers Telegram's rate limit. Telegram responds with HTTP 429 and a `retry_after` field. If the dispatcher does not respect `retry_after` and immediately retries, Telegram blacklists the bot IP for 30 seconds — **all** bot notifications fail during that window, including unrelated critical alerts.

**Why it happens:**
Developers implement retry logic with a fixed short delay (e.g., 1 second) without parsing `retry_after`. They test with single messages and never encounter the limit in development. In production, cron jobs complete at the same wall-clock second (common when scheduled at `:00`), generating a burst.

**How to avoid:**
- Parse `retry_after` from every 429 response and sleep exactly that duration plus jitter before re-queuing.
- As of Bot API 8.0 (November 2025), 429 responses include `adaptive_retry` (float, milliseconds) — honour this field.
- Implement a per-dispatcher token bucket at the application level: max 1 message/second per chat, max 20/minute for groups. Send before checking Telegram, not after.
- The SQLite queue's `next_retry_at` field is exactly the right place to store the Telegram-mandated retry time.
- Never retry synchronously inside the dispatcher function — always re-enqueue with `next_retry_at = now + retry_after`.

**Warning signs:**
- Log lines showing HTTP 429 more than once per minute.
- Critical alerts arriving 30+ seconds late during bursts.
- Retry count climbing on otherwise-healthy messages.

**Phase to address:** MVP dispatcher (Phase 1). The rate limit will be hit in the first week of real use if cron jobs run at round minutes.

---

### Pitfall 4: Dispatcher Goroutine Loops Without Bounded Retry = Poison Pill Messages

**What goes wrong:**
A message with a malformed `targets_override` or an invalid chat_id gets enqueued. The dispatcher retries it repeatedly with exponential backoff. After `max_retries` is reached, the message status is set to `failed` — but if the retry logic has a bug (off-by-one on `retry_count`, or the update query fails), the message stays in `dispatching` or `queued` forever, consuming dispatcher cycles on every poll iteration.

**Why it happens:**
The happy path is easy to test. The failure path — "what happens when the dispatcher crashes mid-update?" — is not. A crashed or killed dispatcher leaves rows with `status = 'dispatching'` that are never reclaimed.

**How to avoid:**
- Add a `dispatched_by` column with a process UUID and a `dispatching_since` timestamp. On startup, reclaim any rows stuck in `dispatching` for more than `2 * dispatch_timeout`.
- Implement a hard maximum on `retry_count` (e.g., 10) that marks the message `failed` and stops dispatching, regardless of why it failed.
- Log and move exhausted messages to a `dead_letter` status — do not delete them; they contain the failure reason.
- Write a startup check: `SELECT COUNT(*) FROM messages WHERE status='dispatching'` — alert if non-zero after grace period.

**Warning signs:**
- Same message ID appearing in dispatcher logs on every poll cycle.
- `retry_count` exceeding the configured maximum without the status changing.
- Memory or CPU creeping up over hours (dispatcher polling a growing set of stuck messages).

**Phase to address:** MVP (Phase 1) — implement reclaim logic before going to production. This will happen on the first systemd restart.

---

### Pitfall 5: No Request Body Size Limit on Webhook Allows Memory Exhaustion

**What goes wrong:**
Go's `net/http` reads request bodies on demand without a built-in size limit. A misconfigured client, a bug in a monitoring script, or a malicious actor sends a 50MB payload to `POST /api/v1/notify`. The server buffers the entire body before parsing JSON, exhausting the 50MB memory budget of the entire process.

**Why it happens:**
`json.NewDecoder(r.Body).Decode(&payload)` reads until EOF. There is no default limit in Go's HTTP server. This is not documented as a gotcha because it is a correct design choice for a general-purpose HTTP library, but it is a concrete pitfall for constrained services.

**How to avoid:**
Wrap every request body with `http.MaxBytesReader(w, r.Body, 64*1024)` (64KB is generous for any notification payload). Return 413 on overflow. Also set `ReadTimeout` and `WriteTimeout` on the HTTP server to prevent slow-read attacks:
```go
server := &http.Server{
    ReadTimeout:  10 * time.Second,
    WriteTimeout: 10 * time.Second,
    IdleTimeout:  60 * time.Second,
}
```

**Warning signs:**
- Memory usage spikes under load testing.
- Slow clients causing connection accumulation (visible via `ss -s`).
- Process OOM-killed by the kernel without obvious cause.

**Phase to address:** MVP (Phase 1) — three lines of code at handler setup, zero cost.

---

### Pitfall 6: Plain-Text Secrets in YAML Config Committed to Git

**What goes wrong:**
`config.yaml` contains `bot_token: "123456:ABC-DEF"` and `smtp_pass: "hunter2"`. A developer runs `git init` in the project directory, does `git add .`, and pushes. The Telegram bot token is now in git history permanently. Rotating the token requires revoking it via BotFather, updating the config, and restarting the service.

**Why it happens:**
Self-hosted projects feel "safe" because they are personal. The config file is the easiest place to put credentials during initial setup, and it is never reviewed for security.

**How to avoid:**
- Ship `config.yaml.example` with placeholder values in git. Add `config.yaml` and `*.db` to `.gitignore` before the first commit.
- Support environment variable overrides for all secrets: `JAIMITO_TELEGRAM_BOT_TOKEN` overrides `dispatchers.telegram.bot_token`. This allows the systemd unit to inject secrets via `EnvironmentFile=/etc/jaimito/secrets.env` (file mode 600, not in git).
- File permissions on `config.yaml`: enforce `0600` at startup and warn/exit if looser.

**Warning signs:**
- `git log --all --oneline -- config.yaml` shows the file was ever tracked.
- Config file readable by other users (`ls -la /etc/jaimito/config.yaml` shows world-readable).

**Phase to address:** Before first commit (pre-MVP). This is a one-time setup that is trivially done early and catastrophically expensive to fix after a public push.

---

## Moderate Pitfalls

### Pitfall 7: WAL File Grows Unboundedly Without Checkpointing

**What goes wrong:**
SQLite WAL mode works by appending writes to a `-wal` file and checkpointing back to the main database file. If the database is under constant write load (e.g., every incoming notification writes to the WAL), and a long-running read transaction is open (e.g., the dispatcher holds a read while sending to Telegram which takes 5 seconds), SQLite cannot checkpoint. The WAL file grows to hundreds of megabytes.

**How to avoid:**
- Keep read transactions short. The dispatcher should read a batch, close the transaction, dispatch, then open a new transaction to update statuses — not hold the read open across the network call.
- Set `PRAGMA wal_autocheckpoint = 1000` (checkpoint every 1000 pages).
- Periodically run `PRAGMA wal_checkpoint(PASSIVE)` from a background goroutine (e.g., every 5 minutes).
- Monitor WAL file size: `du -sh /var/lib/jaimito/jaimito.db-wal` — alert if > 50MB.

**Warning signs:**
- The `-wal` file is present and growing continuously.
- Database reads slowing down over time (WAL file must be scanned on every read).
- Disk usage increasing faster than the message volume would explain.

**Phase to address:** Phase 2 (after MVP is stable) — add WAL monitoring to the health endpoint.

---

### Pitfall 8: Notification Flooding When the Monitored Service Has a Loop Bug

**What goes wrong:**
A cron job has a bug and runs in a tight loop, or a service crashes and restarts 60 times per minute. Each restart or failure sends a notification via jaimito. The result is 60+ Telegram messages per minute, which triggers Telegram rate limiting (pitfall 3), fills the queue, and makes the Telegram channel useless due to notification fatigue.

**How to avoid:**
- Implement deduplication windows per `dedupe_key` (already planned in PRD) — enforce this at the ingestor, not just at the queue level.
- Add a per-channel rate limit with a configurable `burst` and `rate` (e.g., 10 messages/minute for the `cron` channel). Messages exceeding the rate should be held and grouped, not queued individually.
- The `group_window` feature (already in PRD) is specifically the mitigation — implement it before deploying to a production VPS.
- For `jaimito wrap`: if the wrapped command exits non-zero more than N times in M seconds (detectable via a lockfile with timestamp), suppress subsequent notifications and send one "N failures in M seconds, suppressing further alerts" message.

**Warning signs:**
- Queue depth growing faster than the dispatcher can drain it.
- Same `body` or `dedupe_key` appearing repeatedly in `messages` table.
- Telegram chat flooded with identical messages.

**Phase to address:** Phase 2 (deduplication + rate limiting). Do not deploy `jaimito wrap` to production cron jobs without deduplication working.

---

### Pitfall 9: CLI Tool API Key Stored in Shell History or Process List

**What goes wrong:**
Users invoke `jaimito send -k sk-abc123 "message"` with the API key as a command-line argument. The key appears in `~/.bash_history`, in `ps aux` output, and in systemd's `ExecStart=` if the unit calls the CLI directly.

**How to avoid:**
- Do not accept the API key as a positional argument or short flag. Use a dedicated config file for the CLI (e.g., `~/.config/jaimito/client.yaml` with mode 600), or read from environment variable `JAIMITO_API_KEY`.
- The `jaimito` CLI should look for credentials in this priority: env var → config file → interactive prompt (never CLI flag).
- Document this prominently. Users will reach for the flag first.

**Warning signs:**
- `history | grep sk-` reveals API keys.
- `cat /proc/$(pgrep jaimito)/cmdline` shows the key inline.

**Phase to address:** MVP CLI (Phase 1) — design the credential interface before users establish bad habits.

---

### Pitfall 10: Self-Monitoring Bootstrap Problem — jaimito Cannot Alert If It Is Down

**What goes wrong:**
jaimito runs on the same VPS it monitors. If jaimito itself crashes or the service unit fails to start, there is no notification. The PRD mentions "meta-notification" (jaimito notifying about its own errors), but this is logically impossible when the process is dead.

**How to avoid:**
- Use systemd's `Restart=on-failure` and `RestartSec=10` to automatically restart jaimito on crash. This covers most failure modes.
- Use systemd's watchdog: set `WatchdogSec=30` in the unit file and call `daemon.SdNotify(false, "WATCHDOG=1")` from the main dispatch loop. If the loop hangs (e.g., SQLite deadlock), systemd restarts the process.
- For true external monitoring, use a lightweight external health check (e.g., UptimeRobot or a cron job from a second VPS pinging `GET /api/v1/health`). Document this as a known limitation — jaimito is a hub, not an external watchdog.
- The health endpoint `GET /api/v1/health` should check SQLite connectivity and return 503 if the database is inaccessible.

**Warning signs:**
- `systemctl status jaimito` shows `failed` or `activating (auto-restart)` with no alerts received.
- Queue depth stops draining but no error is visible from the outside.

**Phase to address:** MVP deployment (Phase 1) — configure systemd unit with watchdog before declaring the service "running."

---

### Pitfall 11: Telegram MarkdownV2 Escaping Breaks Message Rendering

**What goes wrong:**
The PRD specifies `parse_mode: MarkdownV2`. MarkdownV2 requires escaping 18 special characters (`.`, `!`, `-`, `(`, `)`, etc.) with a backslash. Alert messages naturally contain these characters (e.g., "Error in step 3 - timeout after 30.5s (exit code 1)"). Unescaped characters cause the entire message to fail with a 400 Bad Request from the Telegram API — the message is lost, not retried.

**Why it happens:**
Developers test with clean messages. Production messages contain punctuation from log output, stack traces, and shell command results.

**How to avoid:**
- Write a dedicated `EscapeMarkdownV2(s string) string` function that escapes all 18 characters before embedding dynamic content in templates.
- Alternatively, use `parse_mode: HTML` — the escaping rules are simpler (only `<`, `>`, `&`, `"`) and HTML is well-understood.
- Test the Telegram dispatcher with pathological inputs: backticks, brackets, dots, hyphens, parentheses, and underscores.
- If the Telegram API returns 400 on a send attempt, log the raw message body for debugging before re-queuing (otherwise the bug is invisible).

**Warning signs:**
- 400 errors from Telegram API on messages that contain special characters.
- Messages failing on first attempt but succeeding when sent as plain text.
- Dispatch log shows `success=false`, `response_code=400` without an obvious network reason.

**Phase to address:** MVP dispatcher (Phase 1) — implement escaping at the same time as the Telegram dispatcher, not as a follow-up.

---

## Technical Debt Patterns

| Shortcut | Immediate Benefit | Long-term Cost | When Acceptable |
|----------|-------------------|----------------|-----------------|
| Skip `BEGIN IMMEDIATE` — use default DEFERRED transactions | Less code, faster to write | Intermittent SQLITE_BUSY under concurrent load; hard to reproduce | Never — 5-minute fix now vs. hours of debugging later |
| Use `mattn/go-sqlite3` (CGO) instead of `modernc.org/sqlite` | More documentation and examples | Cross-compilation requires toolchain setup; binary not purely static | Acceptable if ARM deployment is not planned and CI already has CGO |
| Store secrets directly in `config.yaml` | Faster initial setup | Accidental git commit exposure; manual rotation required | Never — environment variable fallback takes 30 minutes |
| No `MaxBytesReader` on webhook handler | Fewer lines of handler code | OOM from large payloads; DoS vector on public endpoints | Never — 1 line of code |
| Synchronous Telegram dispatch in HTTP handler | Simpler code, no queue needed for MVP spike | Webhook times out (30s) if Telegram is slow; no retry on failure; blocks handler goroutine | Never in production — the queue is the entire point of the project |
| Poll queue every 100ms unconditionally | Simple loop, no complexity | SQLite busy reads 10x/second even with empty queue; wasted CPU | Acceptable during MVP; add `time.Sleep` backoff when queue is empty |
| Use `log.Printf` instead of structured logging | Zero setup | Cannot filter or query logs by field (message_id, channel, dispatcher) | MVP only — migrate to `log/slog` before v1.0 |

---

## Integration Gotchas

| Integration | Common Mistake | Correct Approach |
|-------------|----------------|------------------|
| Telegram Bot API | Using `parse_mode: MarkdownV2` without escaping all 18 special characters in dynamic content | Implement `EscapeMarkdownV2()` or switch to `parse_mode: HTML` |
| Telegram Bot API | Ignoring `retry_after` in 429 responses; retrying immediately | Parse `retry_after` (and `adaptive_retry` in API 8.0+); store computed `next_retry_at` in SQLite |
| Telegram Bot API | Not handling "bot was blocked by the user" (403) or "chat not found" (400) — retrying forever | Detect permanent errors by error code; mark message `failed` immediately without retry |
| Telegram Bot API | Sending to a group chat before the bot is added as a member — silent failure | On startup, call `getChat` to validate the configured `chat_id` is reachable |
| SQLite (WAL) | Opening database file with multiple processes simultaneously — corruption risk with non-WAL | Use WAL mode; set `_busy_timeout=5000` in DSN; only one process should own the file |
| SQLite (WAL) | Not setting `_foreign_keys=on` in DSN — FK constraints silently ignored | Add `?_foreign_keys=on&_journal_mode=WAL&_busy_timeout=5000` to the DSN string |
| net/http webhook | Not returning 202 immediately and processing synchronously | Enqueue to SQLite, return 202, dispatch asynchronously |
| SMTP dispatcher | Using `smtp_port: 465` (implicit SSL) vs `587` (STARTTLS) interchangeably — one silently fails | Test both; document which Gmail/Fastmail/etc. requires which |

---

## Performance Traps

| Trap | Symptoms | Prevention | When It Breaks |
|------|----------|------------|----------------|
| Full table scan on `messages` for dispatch polling | Dispatcher slows as queue grows (even with <1000 rows) | Index `(status, next_retry_at)` — add at schema creation time | Noticeable at ~10,000 rows; critical at 100,000+ |
| WAL file not checkpointed | Reads slow; `-wal` file grows to hundreds of MB | Periodic `PRAGMA wal_checkpoint(PASSIVE)` + keep reads short | Within days of continuous operation under write load |
| Dispatcher holds read transaction across Telegram HTTP call | Blocks WAL checkpoint; latency spikes | Read batch → close transaction → dispatch → open new transaction to update | Whenever Telegram is slow (>1s response), which happens regularly |
| No `PRAGMA cache_size` configuration | SQLite uses 2MB default page cache; reads hit disk unnecessarily | Set `PRAGMA cache_size = -8192` (8MB) in DSN; negligible on a VPS | At all times — free performance left on the table |
| Unbounded `SELECT *` on messages history endpoint | Full table scan on large history tables; API timeout | Add `LIMIT` and cursor-based pagination from the start | At ~50MB database size |

---

## Security Mistakes

| Mistake | Risk | Prevention |
|---------|------|------------|
| API key comparison with `==` (non-constant time) | Timing oracle enables key enumeration | Use `hmac.Equal([]byte(provided), []byte(expected))` for all key comparisons |
| Bot token in process environment (`TELEGRAM_BOT_TOKEN=...`) visible in `/proc/PID/environ` | Token readable by root or by the process itself; leaked in crash dumps | Store in file readable only by the service user; load at startup; clear from env after reading |
| Webhook endpoint on public interface without IP restriction | Abuse from public internet; API key brute-force | Bind to `127.0.0.1` by default; document reverse proxy requirement; add rate limiting at server level |
| No request ID or correlation in logs | Cannot trace a webhook call through ingestor → queue → dispatcher when debugging | Generate a `request_id` (UUID) at ingest and propagate it through all log lines |
| `config.yaml` world-readable (`0644`) | SMTP password, Telegram token readable by any local user | Enforce `0600` at startup: `os.Chmod(configPath, 0600)` or exit with clear error |

---

## UX Pitfalls

| Pitfall | User Impact | Better Approach |
|---------|-------------|-----------------|
| `jaimito wrap` sends notification on every success, not only on failure | Cron noise; notification fatigue; Telegram flooded | Default to notify-on-failure only; add `--always` flag for explicit success notifications |
| Long truncated `body` from piped commands overwhelms Telegram message | Telegram has 4096-char message limit; truncated messages lose context | Truncate at 3800 chars and append "… [truncated, N total chars]"; or send as file for large outputs |
| No colour or formatting in CLI error messages | User cannot distinguish "message queued" from "connection refused" | Use `os.Stderr` for errors; exit code 1 on failure; exit code 0 on success — scripts depend on this |
| `jaimito send` exits with code 0 even when the server is unreachable | Cron job treats notification failure as success; silent failure | Exit 1 with a clear error message when the webhook returns non-2xx or connection is refused |
| Config validation only at dispatch time, not at startup | Misconfigured bot_token discovered when the first alert fires, not when starting the service | Validate all dispatcher configs (call Telegram `getMe`, test SMTP auth) at startup with `--check-config` flag |

---

## "Looks Done But Isn't" Checklist

- [ ] **SQLite WAL mode:** Verify `PRAGMA journal_mode` returns `wal` after opening — not just that you set it in the DSN.
- [ ] **Telegram rate limit:** Confirm the dispatcher reads `retry_after` from 429 responses and stores it in `next_retry_at`, not just sleeps a fixed duration.
- [ ] **Retry exhaustion:** Confirm messages with `retry_count >= max_retries` transition to `failed`, not stuck in `queued` or `dispatching`.
- [ ] **Stuck dispatcher reclaim:** Kill the dispatcher mid-dispatch and verify the message is reclaimed on restart, not left in `dispatching` forever.
- [ ] **Signal handling:** Send `SIGTERM` to the process and verify: current dispatch completes, SQLite connection closes cleanly, no WAL corruption.
- [ ] **Webhook security:** Verify that a request with no `Authorization` header returns 401, not 200 or 500.
- [ ] **Body size limit:** Send a 10MB POST to `/api/v1/notify` and verify the server returns 413 without crashing or consuming excess memory.
- [ ] **CLI exit codes:** Run `jaimito send` with the server stopped and verify the exit code is 1 (check with `echo $?`).
- [ ] **Config permissions:** Verify that the service refuses to start (or logs a prominent warning) if `config.yaml` has mode 0644.
- [ ] **MarkdownV2 escaping:** Send a message body containing `_`, `.`, `-`, `(`, `)`, `!` and verify it arrives in Telegram without a 400 error.

---

## Recovery Strategies

| Pitfall | Recovery Cost | Recovery Steps |
|---------|---------------|----------------|
| Secrets committed to git | HIGH | Rotate bot token via BotFather; rotate SMTP password; `git filter-repo` to purge history; force-push; notify all forks/clones |
| SQLite WAL corruption from hard kill | MEDIUM | Stop service; run `sqlite3 jaimito.db "PRAGMA integrity_check"`; if corrupt, restore from last backup (trivial if daily `cp jaimito.db jaimito.db.bak`) |
| Stuck messages in `dispatching` status | LOW | `UPDATE messages SET status='queued', retry_count=0 WHERE status='dispatching'`; restart service |
| Telegram bot banned (repeated 429 abuse) | MEDIUM | Wait 1-24 hours for IP unban; implement proper rate limiting before re-enabling; test with `getMe` |
| Queue flooded with poison pill messages | LOW | `UPDATE messages SET status='failed' WHERE retry_count >= 10`; investigate root cause from `error` column |
| CGO binary not running on target Linux (glibc mismatch) | MEDIUM | Rebuild with `modernc.org/sqlite` (CGO-free); or build with `CGO_ENABLED=1` on matching target glibc |

---

## Pitfall-to-Phase Mapping

| Pitfall | Prevention Phase | Verification |
|---------|------------------|--------------|
| SQLite DEFERRED transaction SQLITE_BUSY | Phase 1 (MVP — database layer) | Concurrent integration test: 2 goroutines write simultaneously; no SQLITE_BUSY in logs |
| CGO cross-compilation | Phase 1 (MVP — build setup) | `make build-arm64` succeeds on an amd64 host without extra toolchain |
| Telegram 429 rate limit | Phase 1 (MVP — Telegram dispatcher) | Send 35 messages in 1 second; verify queue drains correctly with proper backoff |
| Dispatcher poison pill / stuck messages | Phase 1 (MVP — queue + dispatcher) | Kill process mid-dispatch; restart; verify message eventually delivers or fails cleanly |
| Webhook body size / timeout | Phase 1 (MVP — HTTP server setup) | Send 10MB POST; verify 413 response and stable memory |
| Plain-text secrets in git | Pre-Phase 1 (project setup) | `git log --all -- config.yaml` returns empty; `.gitignore` covers `*.db` and `config.yaml` |
| WAL file unbounded growth | Phase 2 (stability + observability) | Run 10,000 writes; verify `-wal` file is checkpointed and stable in size |
| Notification flooding / deduplication | Phase 2 (deduplication + rate limiting) | Send 100 identical messages with same `dedupe_key`; verify only 1 delivered |
| CLI API key in shell history | Phase 1 (MVP — CLI) | `history | grep sk-` returns empty after normal CLI usage |
| Self-monitoring bootstrap problem | Phase 1 (MVP — deployment) | `systemctl kill jaimito`; verify systemd restarts it within `RestartSec` |
| MarkdownV2 escaping | Phase 1 (MVP — Telegram dispatcher) | Integration test with special-character payload; verify no 400 from Telegram |
| Webhook secrets / timing attacks | Phase 1 (MVP — HTTP server) | Verify 401 on bad key; verify response time is constant regardless of key validity |

---

## Sources

- [SQLite Concurrent Writes and "database is locked" errors](https://tenthousandmeters.com/blog/sqlite-concurrent-writes-and-database-is-locked-errors/) — HIGH confidence (technical deep-dive with benchmarks)
- [Go + SQLite Best Practices — Jake Gold](https://jacob.gold/posts/go-sqlite-best-practices/) — HIGH confidence (official-adjacent, widely cited in Go community)
- [SQLite in Practice (1): WAL busy_timeout for workers](https://docsaid.org/en/blog/sqlite-wal-busy-timeout-for-workers/) — MEDIUM confidence (WebSearch verified)
- [Durable Background Execution with Go and SQLite — Three Dots Labs](https://threedots.tech/post/sqlite-durable-execution/) — HIGH confidence (established Go architecture blog)
- [Telegram Bot API — Rate Limits documentation](https://core.telegram.org/bots/faq) — HIGH confidence (official Telegram docs)
- [Telegram Bot API 8.0 — adaptive_retry field](https://core.telegram.org/bots/api) — MEDIUM confidence (confirmed via WebSearch; Bot API 8.0 released November 2025)
- [Go net/http timeouts — Cloudflare Engineering](https://blog.cloudflare.com/the-complete-guide-to-golang-net-http-timeouts/) — HIGH confidence (authoritative Go networking reference)
- [Binary compiled with CGO_ENABLED=0 — mattn/go-sqlite3 issue #855](https://github.com/mattn/go-sqlite3/issues/855) — HIGH confidence (official GitHub issue thread)
- [Graceful Shutdown in Go — VictoriaMetrics Blog](https://victoriametrics.com/blog/go-graceful-shutdown/) — MEDIUM confidence (production-focused Go engineering blog)
- [HMAC webhook security — prismatic.io](https://prismatic.io/blog/how-secure-webhook-endpoints-hmac/) — MEDIUM confidence (WebSearch, consistent with general security practice)
- [Telegram Bot API Errors list](https://github.com/TelegramBotAPI/errors) — MEDIUM confidence (community-maintained, aligns with official error codes)
- [goqite — SQLite-backed queue for Go](https://github.com/maragudk/goqite) — MEDIUM confidence (reference implementation showing correct patterns)

---
*Pitfalls research for: jaimito — VPS Push Notification Hub (Go + SQLite + Telegram Bot API + CLI)*
*Researched: 2026-02-20*
