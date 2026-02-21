# Feature Research

**Domain:** Self-hosted VPS notification hub
**Researched:** 2026-02-20
**Confidence:** HIGH (ntfy/Gotify official docs + multiple verified sources)

---

## Feature Landscape

### Table Stakes (Users Expect These)

Features users assume exist. Missing these = product feels incomplete or untrustworthy.

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| HTTP webhook endpoint (POST) | Every notification tutorial starts with `curl -d "msg" url` — if curl can't send, it's broken | LOW | ntfy, Gotify both require this; de-facto standard for self-hosted |
| Bearer token / API key auth | Unauthenticated endpoints can't be exposed to the internet safely | LOW | ntfy uses `Authorization: Bearer`, Gotify uses `X-Gotify-Key`; both are expected |
| Message persistence (SQLite queue) | Users expect to see what was sent; no persistence = notifications silently lost on restart | MEDIUM | ntfy defaults to 12h cache; Gotify keeps history until manually deleted |
| Priority levels | Different events need different urgency signals; flat priority = alert fatigue | LOW | ntfy has 5 levels (min/low/default/high/max); Gotify has numeric 1-10; 4 levels is the minimum |
| Automatic retry with backoff | If Telegram is down or rate-limits, the message must eventually deliver | MEDIUM | Without this, failed dispatches are silently dropped; ntfy doesn't have server-side retry (client responsibility); jaimito's queue fills this gap |
| CLI `send` command | Ops and cron jobs live in the shell; an HTTP-only interface requires `curl` boilerplate in every script | LOW | ntfy has `ntfy publish`; Gotify has a CLI tool; this is the ops ergonomics baseline |
| Health check endpoint | Used by monitoring systems (UptimeKuma, Gatus) to verify the notifier itself is alive | LOW | `GET /health` returning 200 is the minimum; ntfy exposes `/v1/health` |
| YAML/file-based configuration | Self-hosted software that requires database-only config is painful to automate and version-control | LOW | ntfy uses `server.yml`; Gotify uses env vars + config file; both expected |
| Delivery status visibility | Users need to know if a notification succeeded or is retrying — silent failure is the worst outcome | MEDIUM | Gotify shows messages in WebUI; ntfy shows in app/web; jaimito needs at minimum a log/CLI query |
| Structured message format | `title` + `body` is the minimum; without title, all messages look alike in Telegram | LOW | ntfy supports title header; Gotify supports title field; Telegram requires this distinction |

### Differentiators (Competitive Advantage)

Features that set jaimito apart. Not required for launch, but high value for the target audience.

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| `jaimito wrap <cmd>` — cron job wrapper | Captures exit code + stdout/stderr, notifies only on failure with full output. Solves the #1 VPS pain point: silent cron failures | MEDIUM | ntfy has `ntfy-run` but it's separate; no competitor bundles this in the core binary; this is jaimito's killer feature |
| Dead man's switch / heartbeat monitoring | cron sends a ping every run; if no ping within grace window, jaimito alerts. Solves "cron silently stopped running" | HIGH | healthchecks.io does this as its entire purpose; ntfy supports via scheduled message trick (deliver delayed, cancel if healthy); jaimito could bake this in natively |
| Channel-based routing with named channels | Named channels (e.g., `cron`, `app-errors`, `deployments`) allow filtering and different formatting per channel rather than flat topic namespaces | LOW | ntfy topics are flat strings; Gotify apps are pre-registered; named channels with semantic meaning and default configs is a cleaner UX |
| Telegram-optimized formatting | Priority-to-emoji mapping, monospace code blocks for command output, Telegram MarkdownV2 — built specifically for Telegram rather than generic text | LOW | Generic notifiers (Apprise, ntfy) send plaintext or basic markdown; Telegram-specific formatting is a first-class citizen in jaimito |
| `jaimito wrap` — output truncation to Telegram limits | Telegram message limit is 4096 chars; wrap must truncate and indicate truncation without crashing | LOW | Competitors send raw output and hit Telegram limits silently |
| Single-binary deployment (systemd unit) | No Docker, no Python runtime, no Ruby gems — `apt install` or `curl | sh`, drop in systemd unit, done | LOW | Gotify and ntfy require Docker or building from source for many users; single Go binary is a meaningful differentiator for VPS operators who avoid Docker |
| Priority-based behavioral differences | Critical = immediate delivery, skip rate limit; Low = batch/defer; not just a cosmetic label | MEDIUM | ntfy maps priority to Android vibration but server behavior is identical; Gotify priorities are purely cosmetic; jaimito can make priority actually affect delivery behavior |
| Key management via CLI (`jaimito keys create/list/revoke`) | API keys managed in-process without editing config files or restarting the server | LOW | ntfy requires server restart or CLI; Gotify requires web UI; in-process key rotation is friendlier for ops |
| Message queue drain on shutdown | Graceful shutdown attempts to flush pending notifications before exit | LOW | Most competitors drop in-flight messages on SIGTERM; this is a reliability differentiator |

### Anti-Features (Commonly Requested, Often Problematic)

Features that seem good but create complexity that exceeds the value for jaimito's scope.

| Feature | Why Requested | Why Problematic | Alternative |
|---------|---------------|-----------------|-------------|
| Web dashboard / notification history UI | "I want to see what was sent" | Requires frontend build system, CORS, session auth, JS bundle — easily 10x the codebase; goes against single-binary principle | CLI `jaimito list` command for recent messages; Telegram itself is the history |
| Multi-channel dispatchers (Email, Slack, Discord, PagerDuty) | "I want notifications on Slack too" | Apprise already solves this for 110+ services; building our own multi-channel dispatch duplicates Apprise and balloons maintenance burden | Document Apprise integration as the extension pattern; keep jaimito Telegram-only |
| Real-time WebSocket stream | "I want to receive notifications in my terminal live" | ntfy and Gotify both have this; requires persistent connections, additional complexity; the VPS-local use case is Telegram delivery, not live streaming | Telegram IS the real-time stream; the CLI can poll for recent messages if needed |
| Multi-user / team support | "My team shares this server" | User management (registration, roles, permissions) is a separate product surface; adds auth complexity, session management, and shared state | Single-user / single-VPS model is a feature, not a bug; API keys per service are sufficient |
| Plugin system for custom dispatchers | "I want to write my own output target" | Plugin architectures (Gotify's model) require ABI stability, versioning, and documentation; increases maintenance burden for small projects | Configuration-driven approach: forward to Apprise webhook, or use HTTP generic dispatcher in v0.3 |
| Message templates / templating engine | "I want to customize how GitHub webhook payloads look" | ntfy's Go template support for webhook payloads is complex and rarely used correctly; it requires the server to understand payload schemas | Senders should format their own messages; jaimito is the transport, not the formatter |
| Attachment storage (files/images) | "I want to send screenshots with alerts" | File storage on a VPS means managing disk quotas, TTL cleanup, MIME detection, and URL serving — a separate subsystem | Send Telegram file attachments directly via Telegram Bot API if needed; jaimito handles text alerts |
| SMTP email ingestor (receive-by-email) | "I want to forward system emails to Telegram" | MTA integration, DKIM/SPF, spam filtering — a different product category | Use a dedicated mail-to-Telegram bridge (mailrise, etc.) |
| Alert deduplication / grouping (complex rules) | "Don't spam me with 50 identical errors" | Production-grade dedup (by fingerprint, labels, time window) requires Alertmanager-level complexity | Simple rate limiting per channel (deferred to v0.2) covers 80% of the use case; route noisy services to a low-priority channel |

---

## Feature Dependencies

```
[HTTP webhook endpoint]
    └──requires──> [Bearer token auth]
    └──requires──> [SQLite queue]
                       └──requires──> [Retry with backoff]
                                          └──requires──> [Telegram dispatcher]

[CLI `jaimito send`]
    └──requires──> [HTTP webhook endpoint]  (or direct SQLite write)

[CLI `jaimito wrap`]
    └──requires──> [CLI `jaimito send`]
    └──enhances──> [Priority system]  (failure = high, success = optional low)

[Health check endpoint]
    └──requires──> [HTTP webhook endpoint]  (same HTTP server)

[Key management CLI]
    └──requires──> [Bearer token auth]
    └──requires──> [SQLite queue]  (keys stored in same DB)

[Dead man's switch / heartbeat]
    └──requires──> [SQLite queue]  (heartbeat state persisted)
    └──requires──> [Retry with backoff]  (alert on missed heartbeat)
    └──enhances──> [CLI `jaimito wrap`]  (wrap can send heartbeat ping)

[Priority-based behavioral differences]
    └──requires──> [Priority system]
    └──requires──> [SQLite queue]  (defer low-priority messages)
    └──enhances──> [Retry with backoff]  (critical skips rate limit)

[Channel-based routing]
    └──requires──> [SQLite queue]  (channel stored on message)
    └──requires──> [YAML configuration]  (channel defaults defined in config)
```

### Dependency Notes

- **HTTP webhook requires auth before going public**: The endpoint must not be exposed unauthenticated. Auth is not optional even for MVP.
- **`wrap` requires `send`**: `wrap` is a thin orchestration layer over the send path; they share the delivery pipeline.
- **Dead man's switch requires persistent state**: Unlike the pure webhook path, heartbeat monitoring needs to track last-seen times and fire alerts from background goroutines. This is a meaningful jump in architecture complexity — defer to v1.x.
- **Priority behavioral differences require queue**: Making priority affect scheduling (not just formatting) requires the queue to support priority lanes. Implement cosmetically first, behaviorally later.
- **Channel routing requires config**: Channels without config defaults are just strings. The value is in channel-level defaults (e.g., `cron` channel always adds `wrap` context formatting).

---

## MVP Definition

### Launch With (v0.1)

Minimum viable product — what's needed to solve the primary pain point (silent cron failures) reliably.

- [x] HTTP `POST /api/v1/notify` with Bearer token auth — **the ingestion gate**
- [x] `jaimito send` CLI — **shell integration without curl boilerplate**
- [x] `jaimito wrap <cmd>` — **killer feature; solves silent cron failures**
- [x] Telegram dispatcher with priority-based emoji formatting — **the output; Telegram IS the UI**
- [x] SQLite queue with WAL mode — **no message loss on restart**
- [x] Automatic retry with exponential backoff — **reliability baseline**
- [x] Priority system (critical/high/normal/low) with cosmetic differentiation — **urgency signaling**
- [x] `GET /api/v1/health` — **so monitoring can watch the notifier**
- [x] `jaimito keys create/list/revoke` — **key management without config edits**
- [x] YAML config at `/etc/jaimito/config.yaml` — **version-controllable setup**
- [x] Channel-based routing (named channels with config defaults) — **organize noisy VPS alerts**

### Add After Validation (v0.2 — v0.3)

Features to add once the core ingest-queue-deliver loop is stable.

- [ ] Rate limiting per channel — add when users report notification storms from cron failures
- [ ] Quiet hours (do not disturb window) — add when daily-schedule users need sleep protection
- [ ] Simple deduplication (same message within N minutes = suppress) — add when alert fatigue is reported
- [ ] `jaimito list` CLI (recent messages, delivery status) — add when "what happened?" becomes a common question
- [ ] HTTP generic dispatcher — add when users need non-Telegram targets without Apprise overhead
- [ ] Message digests / batching for low-priority channels — add when low-priority noise is a complaint

### Future Consideration (v1.0+)

Features to defer until core is proven valuable.

- [ ] Dead man's switch / heartbeat monitoring — defer; healthchecks.io exists and integrates with jaimito's webhook; build only if users specifically request native heartbeat
- [ ] Email/SMTP dispatcher — defer; Apprise covers this; add only if Apprise integration is too heavy
- [ ] Web dashboard — defer; Telegram history + `jaimito list` satisfies the use case; a dashboard is a product pivot
- [ ] File watcher ingestor (watch log files, fire on pattern match) — defer; loggifly + ntfy-style approach exists; out of scope for a notification hub
- [ ] Multi-dispatcher (WhatsApp, Slack, Matrix) — defer; adds maintenance surface; recommend Apprise bridge

---

## Feature Prioritization Matrix

| Feature | User Value | Implementation Cost | Priority |
|---------|------------|---------------------|----------|
| HTTP webhook endpoint | HIGH | LOW | P1 |
| Bearer token auth | HIGH | LOW | P1 |
| SQLite queue + WAL | HIGH | MEDIUM | P1 |
| Telegram dispatcher | HIGH | LOW | P1 |
| `jaimito wrap` | HIGH | MEDIUM | P1 |
| `jaimito send` CLI | HIGH | LOW | P1 |
| Retry with exponential backoff | HIGH | MEDIUM | P1 |
| Priority system (cosmetic) | MEDIUM | LOW | P1 |
| Health check endpoint | MEDIUM | LOW | P1 |
| Key management CLI | MEDIUM | LOW | P1 |
| YAML config | MEDIUM | LOW | P1 |
| Channel-based routing | MEDIUM | LOW | P1 |
| Rate limiting per channel | MEDIUM | LOW | P2 |
| Quiet hours | MEDIUM | LOW | P2 |
| `jaimito list` / query API | MEDIUM | LOW | P2 |
| Simple deduplication | LOW | MEDIUM | P2 |
| Message digests / batching | LOW | MEDIUM | P2 |
| HTTP generic dispatcher | LOW | MEDIUM | P2 |
| Dead man's switch | MEDIUM | HIGH | P3 |
| Web dashboard | LOW | HIGH | P3 |
| File watcher ingestor | LOW | HIGH | P3 |
| Multi-dispatcher (non-Telegram) | LOW | HIGH | P3 |
| Plugin system | LOW | HIGH | P3 |

**Priority key:**
- P1: Must have for launch (v0.1)
- P2: Should have, add when possible (v0.2–v0.3)
- P3: Nice to have, future consideration (v1.0+)

---

## Competitor Feature Analysis

| Feature | ntfy | Gotify | Apprise | jaimito (plan) |
|---------|------|--------|---------|----------------|
| HTTP webhook ingest | Yes (PUT/POST) | Yes (REST API) | Yes (API) | Yes (POST /api/v1/notify) |
| CLI send | Yes (`ntfy publish`) | Yes (CLI tool) | Yes (`apprise`) | Yes (`jaimito send`) |
| Cron wrapper / `wrap` | `ntfy-run` (separate binary) | No | No | Yes (built-in, `jaimito wrap`) |
| Priority levels | 5 (min to max) | 1-10 numeric | No native priority | 4 levels with behavioral differences |
| Message persistence | SQLite, configurable TTL (default 12h) | SQLite, manual delete | No persistence | SQLite WAL, configurable TTL |
| Retry on delivery failure | No (server pushes to clients; retry is client-side) | No (WebSocket push) | No built-in retry | Yes (exponential backoff, configurable max attempts) |
| Bearer token auth | Yes (+ ACL per topic) | Yes (app tokens) | Yes (HTTP Basic) | Yes (API keys with `sk-` prefix) |
| Multiple output channels | No (clients subscribe) | No (clients subscribe) | Yes (110+ services) | No (Telegram only in MVP) |
| Web UI | Yes (web app) | Yes (web app) | Yes (API only) | No (deliberate anti-feature) |
| Dead man's switch | Via scheduled message trick | No | No | No (v1.0+) |
| Rate limiting | Yes (visitor token bucket) | No | No | Deferred to v0.2 |
| Mobile apps | Android + iOS + Web Push | Android (Gotify app) | No | No (via Telegram app) |
| Single binary deployment | Yes | Yes | No (Python) | Yes (Go) |
| Memory footprint | ~30MB | ~20MB | Varies | Target <50MB |
| YAML/file config | Yes (`server.yml`) | Yes + env vars | Yes (config files) | Yes (`/etc/jaimito/config.yaml`) |
| Structured channels/topics | Yes (topic strings) | Yes (apps) | No | Yes (named channels with defaults) |
| Telegram-native formatting | No (generic text) | No (generic text) | Basic | Yes (MarkdownV2, emoji priority, code blocks) |
| Key management via CLI | Partial (requires restart for YAML tokens) | Via web UI | No | Yes (`jaimito keys create/list/revoke`) |

### Key Observations from Competitor Analysis

1. **Retry is the gap.** ntfy and Gotify push to connected clients in real time — they don't retry failed deliveries because delivery model is different (subscribe-based, not dispatch-based). jaimito's queue-and-dispatch model with retries is architecturally distinct and more reliable for a fire-and-forget VPS notification pattern.

2. **`wrap` is differentiated.** ntfy has `ntfy-run` as a third-party add-on, not bundled in the main binary. No other competitor treats cron job wrapping as a first-class feature. This is jaimito's clearest competitive differentiation.

3. **Telegram formatting is a gap.** Every competitor sends plain text or generic markdown. Telegram's MarkdownV2 syntax enables meaningful rich formatting that none of the general-purpose tools optimize for.

4. **Single binary is table stakes for VPS operators.** Python-based tools (Apprise, healthchecks.io) require runtime setup. ntfy and Gotify are Go binaries — this is why the Go choice aligns with user expectations.

5. **No competitor does priority-as-behavior.** All competitors treat priority as a cosmetic label or client-side hint. Making priority affect server-side behavior (skip rate limits for critical, defer low-priority) would be genuinely novel.

---

## Sources

- ntfy official documentation: https://docs.ntfy.sh/ (HIGH confidence — official source)
- ntfy publishing features: https://docs.ntfy.sh/publish/ (HIGH confidence — official source)
- ntfy configuration reference: https://docs.ntfy.sh/config/ (HIGH confidence — official source)
- ntfy examples: https://docs.ntfy.sh/examples/ (HIGH confidence — official source)
- Gotify GitHub: https://github.com/gotify/server (HIGH confidence — official source)
- Gotify vs ntfy comparison: https://blog.vezpi.com/en/post/notification-system-gotify-vs-ntfy/ (MEDIUM confidence — verified against official docs)
- Apprise GitHub: https://github.com/caronc/apprise (HIGH confidence — official source)
- 4 reasons to use Apprise: https://www.xda-developers.com/reasons-use-apprise-instead-of-ntfy-gotify/ (MEDIUM confidence — editorial)
- Shoutrrr (Go library): https://github.com/containrrr/shoutrrr (HIGH confidence — official source)
- Healthchecks.io (cron monitoring): https://healthchecks.io/ (HIGH confidence — official source)
- Self-hosted notifications pain points: https://thomaswildetech.com/blog/2026/01/05/the-holy-grail-of-self-hosted-notifications/ (MEDIUM confidence — practitioner blog, Jan 2026)
- ntfy dead man's switch pattern: https://docs.ntfy.sh/publish/#scheduled-delivery (HIGH confidence — official docs)
- ntfy user/ACL management: https://docs.ntfy.sh/config/#access-control (HIGH confidence — official docs)

---
*Feature research for: self-hosted VPS notification hub (jaimito)*
*Researched: 2026-02-20*
