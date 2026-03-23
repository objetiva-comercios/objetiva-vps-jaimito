# Phase 2: Core Pipeline - Research

**Researched:** 2026-02-21
**Domain:** Go HTTP API (chi), bearer auth, Telegram dispatcher, MarkdownV2 formatting, exponential backoff, seed key bootstrap
**Confidence:** HIGH (all library APIs verified against local module cache at v1.19.0 and pkg.go.dev)

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**Telegram message format:**
- Traffic light emoji per priority: low = 🟢, normal = 🟡, high = 🔴
- Compact layout: emoji + bold title on one line, body below
- When title is omitted, show emoji + body directly (no auto-generated title)
- Tags rendered as hashtags on a new line after body: #disk #backup
- Channel name is NOT shown in the message — the chat itself is the context
- All text escaped for MarkdownV2 as required by Telegram Bot API

**API payload defaults:**
- Minimal valid payload: `{"body": "text"}` — title, channel, priority all optional
- Omitted channel defaults to `general`
- Omitted priority defaults to `normal`
- Omitted title means no bold title line in Telegram (just emoji + body)
- Tags and metadata are always optional

**Retry & failure policy:**
- 5 retry attempts with exponential backoff
- Respect Telegram 429 `retry_after` header exactly
- After 5 exhausted retries, mark message as `failed` silently (logged, no further action)
- Cleanup purges failed messages after 90 days (already implemented in Phase 1)
- Dispatcher polls for new queued messages every 1 second
- Sequential dispatch — one message at a time, preserves order

**First API key bootstrap:**
- Seed API keys defined in `config.yaml` under a `seed_api_keys` section
- User provides the full `sk-...` key value in config
- Service hashes and inserts seed keys on startup if they don't already exist
- This bridges the gap until Phase 3 adds `jaimito keys create` CLI

**HTTP server configuration:**
- Listen address configurable via `server.listen` field in config.yaml
- Default listen address: Claude's Discretion (secure default)

### Claude's Discretion

- Message ID format (UUID v7, short ID, or other)
- Default listen address (likely localhost-only for security)
- Exact exponential backoff timing (initial delay, multiplier, jitter)
- Health endpoint response body structure
- MarkdownV2 escaping implementation details

### Deferred Ideas (OUT OF SCOPE)

None — discussion stayed within phase scope
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| API-01 | Service accepts HTTP POST to `/api/v1/notify` with JSON payload (title, body, channel, priority, tags, metadata) | chi v5 router with r.Post(); encoding/json decode; request struct with optional fields using pointers |
| API-02 | Service authenticates requests via Bearer token with `sk-` prefix API keys | Custom chi middleware extracting Authorization header; SHA-256 hash; crypto/subtle.ConstantTimeCompare against api_keys table |
| API-03 | Service returns 202 with message ID on success, 400 on invalid payload, 401 on bad auth | json.NewEncoder(w).Encode() response structs; w.WriteHeader() before Write; UUID v7 as message ID |
| API-04 | Service exposes `GET /api/v1/health` returning 200 when operational | chi r.Get(); simple JSON response with status field |
| TELE-01 | Service delivers messages to configured Telegram chat via Bot API | bot.SendMessage() with ChatID from config.Channels lookup by channel name |
| TELE-02 | Service formats messages with MarkdownV2 (priority emoji, bold title, code blocks, proper escaping) | bot.EscapeMarkdown(); *bold* with `*` delimiters; models.ParseModeMarkdown (= "MarkdownV2" string); build formatted string before sending |
| TELE-03 | Service retries failed deliveries with exponential backoff, respecting Telegram 429 `retry_after` | bot.IsTooManyRequestsError(); TooManyRequestsError.RetryAfter (int, seconds); time.Sleep; 5-attempt loop in dispatcher goroutine |
</phase_requirements>

---

## Summary

Phase 2 wires three independent subsystems onto the Phase 1 foundation: an HTTP API that ingests notifications (API-01 through API-04), a Telegram dispatcher that delivers them (TELE-01 through TELE-03), and a seed API key bootstrap mechanism. All three run inside the existing binary started by `cmd/jaimito/main.go`.

The HTTP layer uses `go-chi/chi/v5` (not yet in go.mod — must be added). Chi is the correct choice for this project per the preliminary STACK.md research: it provides middleware grouping (auth on `/api/v1/*`, open health check), is stdlib-compatible, and has no external dependencies of its own. The bearer auth middleware must be hand-rolled (chi v5 does NOT include a BearerToken middleware — verified against the complete `chi/v5/middleware` package listing). The auth middleware extracts the `Authorization: Bearer sk-...` header, SHA-256 hashes the token, and does a constant-time lookup against the `api_keys` table.

The Telegram dispatcher runs as a long-lived goroutine polling SQLite for `queued` messages every 1 second. It processes one message at a time (sequential, preserving order). For delivery, it uses `bot.SendMessage()` with `models.ParseModeMarkdown` (which equals the string `"MarkdownV2"` — the constant naming is counterintuitive but verified in the local module cache). MarkdownV2 escaping uses `bot.EscapeMarkdown()` from the `go-telegram/bot` package for user-supplied text, while formatting tokens (`*`, `\n`) are added raw. Rate limiting is handled by inspecting `bot.TooManyRequestsError.RetryAfter` (type `int`, seconds).

**Critical schema issue:** The Phase 1 schema has `title TEXT NOT NULL` but Phase 2 requires optional title. A migration (002_nullable_title.sql) must be added to alter this column.

**Primary recommendation:** Add chi v5 to go.mod; hand-roll a custom bearer auth middleware (no chi auth built-in); use `models.ParseModeMarkdown` (not `ParseModeMarkdownV2`) for MarkdownV2 formatting; migrate the schema to allow NULL title; use UUID v7 (`google/uuid.NewV7()`) as the message ID format.

---

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `github.com/go-chi/chi/v5` | v5.2.5 | HTTP router and middleware grouping | Idiomatic, stdlib-compatible; auth middleware on `/api/v1/*` group; already selected in STACK.md research; requires Go 1.22 (already go 1.24) |
| `github.com/go-telegram/bot` | v1.19.0 (already in go.mod) | SendMessage, EscapeMarkdown, error handling | Already present; provides IsTooManyRequestsError, TooManyRequestsError.RetryAfter, EscapeMarkdown, bot.New reused from Phase 1 |
| `github.com/google/uuid` | v1.6.0 (already in go.mod as indirect) | UUID v7 message IDs | NewV7() is time-ordered (sortable by creation time); already indirectly present; promote to direct |
| `crypto/sha256` | stdlib | API key hashing | No external dependency; SHA-256 is sufficient for non-password tokens |
| `crypto/subtle` | stdlib | Timing-safe hash comparison | Prevents timing attacks on API key lookups |
| `encoding/json` | stdlib | JSON request/response encoding | Standard; no external package needed |
| `net/http` | stdlib | HTTP server | chi wraps it; `http.Server.Shutdown()` for graceful stop |

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `strings` | stdlib | Bearer token prefix extraction (`strings.TrimPrefix`, `strings.HasPrefix`) | Auth middleware |
| `time` | stdlib | Exponential backoff (`time.Sleep`) and dispatcher polling (`time.NewTicker`) | Dispatcher |
| `log/slog` | stdlib (Go 1.21+) | Structured logging of dispatch events, retry attempts, auth failures | Throughout |
| `context` | stdlib | Propagate cancellation from main shutdown signal to HTTP server and dispatcher | Throughout |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `go-chi/chi/v5` | `net/http` ServeMux (Go 1.22+) | ServeMux lacks middleware grouping — applying auth only to `/api/v1/*` would require manual wrapping; chi is cleaner and already selected |
| UUID v7 (`google/uuid`) | Short random string | UUID v7 is time-ordered, collision-resistant, RFC-standard; short IDs require custom generation |
| Custom auth middleware | `go-chi/oauth` or `go-chi/jwtauth` | jwtauth adds JWT overhead for a simple opaque token; oauth is overkill; custom middleware is 20 lines |
| Hand-rolled exponential backoff | `cenkalti/backoff/v4` | cenkalti/backoff is solid but adds a dependency; the retry logic here is simple enough (5 attempts, linear formula with jitter) that hand-rolling is appropriate |

**Installation (new packages only):**
```bash
go get github.com/go-chi/chi/v5@v5.2.5
go get github.com/google/uuid@v1.6.0   # promote from indirect to direct
```

---

## Architecture Patterns

### Recommended Project Structure (Phase 2 additions)

```
jaimito/
├── cmd/
│   └── jaimito/
│       └── main.go          # +HTTP server start/shutdown wired in; +seed key bootstrap
├── internal/
│   ├── api/
│   │   ├── server.go        # chi router setup, middleware stack, route registration
│   │   ├── handlers.go      # NotifyHandler, HealthHandler
│   │   ├── middleware.go    # BearerAuth middleware: extract token, hash, DB lookup
│   │   └── response.go      # WriteJSON helper, error response structs
│   ├── config/
│   │   └── config.go        # +ServerConfig{Listen string}, +SeedAPIKey struct, +SeedAPIKeys []SeedAPIKey
│   ├── db/
│   │   ├── db.go            # unchanged
│   │   ├── messages.go      # EnqueueMessage, GetNextQueued, MarkDelivered, MarkFailed, RecordAttempt
│   │   ├── apikeys.go       # LookupKeyHash, SeedKeys
│   │   └── schema/
│   │       ├── 001_initial.sql        # unchanged
│   │       └── 002_nullable_title.sql # ALTER TABLE messages: title -> TEXT (nullable)
│   ├── dispatcher/
│   │   └── dispatcher.go    # Start(ctx, db, bot, cfg); polling loop; retry logic
│   ├── telegram/
│   │   ├── client.go        # unchanged (ValidateToken, ValidateChats)
│   │   └── format.go        # FormatMessage(msg) -> string; emoji map; EscapeMarkdown usage
│   └── cleanup/
│       └── scheduler.go     # unchanged
├── configs/
│   └── config.example.yaml  # +server.listen, +seed_api_keys section
├── systemd/
│   └── jaimito.service      # unchanged
├── go.mod                   # +chi v5, uuid promoted to direct
└── go.sum
```

### Pattern 1: Chi Router Setup with Auth-Protected API Group

**What:** Single chi.NewRouter with global middleware (RequestID, Logger, Recoverer), then a subrouter group at `/api/v1` with BearerAuth middleware applied only to that group.

**When to use:** Always. The health endpoint (`GET /api/v1/health`) must be excluded from auth so monitoring can reach it. The notify endpoint requires auth. Use `r.Route()` to separate them.

```go
// Source: chi v5.2.5 docs verified 2026-02-21; internal/api/server.go
func NewRouter(db *sql.DB, bot *bot.Bot, cfg *config.Config) http.Handler {
    r := chi.NewRouter()

    // Global middleware (all routes)
    r.Use(middleware.RequestID)
    r.Use(middleware.RealIP)
    r.Use(middleware.Logger)
    r.Use(middleware.Recoverer)
    r.Use(middleware.Timeout(30 * time.Second))

    r.Get("/api/v1/health", HealthHandler)

    // Authenticated API group
    r.Group(func(r chi.Router) {
        r.Use(BearerAuth(db))  // Only applied to this group
        r.Post("/api/v1/notify", NotifyHandler(db, cfg))
    })

    return r
}
```

**Key point:** `r.Group()` creates a fresh middleware stack; `r.Route()` mounts a sub-router at a prefix. For this case, `r.Group()` is correct — the routes share the `/api/v1` prefix without needing a sub-path.

### Pattern 2: Bearer Auth Middleware (Hand-Rolled)

**What:** Extract `Authorization: Bearer sk-...` header, SHA-256 hash the token, constant-time compare against `api_keys` table. Return 401 if missing, malformed, or not found.

```go
// Source: stdlib crypto/sha256, crypto/subtle — verified 2026-02-21
// internal/api/middleware.go
func BearerAuth(db *sql.DB) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            authHeader := r.Header.Get("Authorization")
            if !strings.HasPrefix(authHeader, "Bearer ") {
                writeError(w, http.StatusUnauthorized, "missing or invalid Authorization header")
                return
            }

            token := strings.TrimPrefix(authHeader, "Bearer ")
            if !strings.HasPrefix(token, "sk-") {
                writeError(w, http.StatusUnauthorized, "invalid token format")
                return
            }

            // Hash the token for lookup (never store raw tokens)
            hash := sha256.Sum256([]byte(token))
            hashHex := hex.EncodeToString(hash[:])

            keyID, err := db.LookupKeyHash(r.Context(), hashHex)
            if err != nil || keyID == "" {
                writeError(w, http.StatusUnauthorized, "invalid token")
                return
            }

            // Update last_used_at (fire-and-forget, non-blocking)
            go db.UpdateLastUsed(r.Context(), keyID)

            next.ServeHTTP(w, r)
        })
    }
}
```

**Security note:** Using `hex.EncodeToString(sha256.Sum256(...))` produces a fixed-length string; comparing strings of equal length makes `strings.Compare` timing-safe for equal-length inputs. For extra safety, use `subtle.ConstantTimeCompare([]byte(computed), []byte(stored))`.

### Pattern 3: Notify Handler — Enqueue and Return 202

**What:** Decode JSON body into a request struct with optional fields (pointers for nullable). Validate required field `body`. Insert into messages table. Return 202 with message ID.

```go
// Source: stdlib encoding/json — verified 2026-02-21; internal/api/handlers.go
type NotifyRequest struct {
    Title    *string          `json:"title"`              // optional; nil = no title line
    Body     string           `json:"body"`               // required
    Channel  string           `json:"channel"`            // optional, default "general"
    Priority string           `json:"priority"`           // optional, default "normal"
    Tags     []string         `json:"tags,omitempty"`
    Metadata map[string]any   `json:"metadata,omitempty"`
}

type NotifyResponse struct {
    ID string `json:"id"`
}

func NotifyHandler(db *sql.DB, cfg *config.Config) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        // Limit body size (prevent abuse)
        r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1MB

        var req NotifyRequest
        if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
            writeError(w, http.StatusBadRequest, "invalid JSON body")
            return
        }

        // Apply defaults
        if req.Channel == "" {
            req.Channel = "general"
        }
        if req.Priority == "" {
            req.Priority = "normal"
        }

        // Validate
        if req.Body == "" {
            writeError(w, http.StatusBadRequest, "body is required")
            return
        }
        if !cfg.ChannelExists(req.Channel) {
            writeError(w, http.StatusBadRequest, fmt.Sprintf("unknown channel: %q", req.Channel))
            return
        }
        if !validPriority(req.Priority) {
            writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid priority: %q", req.Priority))
            return
        }

        // Generate UUID v7 message ID
        id, err := uuid.NewV7()
        if err != nil {
            writeError(w, http.StatusInternalServerError, "failed to generate message ID")
            return
        }

        if err := db.EnqueueMessage(r.Context(), id.String(), req); err != nil {
            writeError(w, http.StatusInternalServerError, "failed to enqueue message")
            return
        }

        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusAccepted)  // 202
        json.NewEncoder(w).Encode(NotifyResponse{ID: id.String()})
    }
}
```

**Critical order:** `w.WriteHeader(202)` MUST come after `w.Header().Set(...)` and BEFORE `w.Write()`/`Encode()`. If you call `Encode` first, Go auto-sends 200.

### Pattern 4: Health Handler

**What:** Simple GET handler returning 200 JSON with service status. No auth required.

```go
// internal/api/handlers.go
type HealthResponse struct {
    Status  string `json:"status"`
    Service string `json:"service"`
}

func HealthHandler(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")
    // 200 is implicit when WriteHeader is not called before Write
    json.NewEncoder(w).Encode(HealthResponse{
        Status:  "ok",
        Service: "jaimito",
    })
}
```

### Pattern 5: HTTP Server with Graceful Shutdown

**What:** Start `http.Server` in a goroutine; on signal cancellation, call `server.Shutdown()` with a timeout context. This is separate from the existing signal context — shutdown gets its own deadline.

```go
// Source: chi _examples/graceful/main.go — verified 2026-02-21; cmd/jaimito/main.go
server := &http.Server{
    Addr:         cfg.Server.Listen,
    Handler:      api.NewRouter(database, tgBot, cfg),
    ReadTimeout:  15 * time.Second,
    WriteTimeout: 15 * time.Second,
    IdleTimeout:  60 * time.Second,
}

go func() {
    if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
        slog.Error("HTTP server error", "error", err)
        os.Exit(1)
    }
}()

slog.Info("jaimito started", "addr", cfg.Server.Listen, "channels", len(cfg.Channels))
<-ctx.Done()

// Graceful shutdown with 30s timeout
shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()
if err := server.Shutdown(shutdownCtx); err != nil {
    slog.Error("HTTP server shutdown error", "error", err)
}
```

**Key:** Use `errors.Is(err, http.ErrServerClosed)` — `ListenAndServe` returns this non-nil error on clean Shutdown; it is not a real error.

### Pattern 6: Telegram Dispatcher (Poll → Dispatch → Retry Loop)

**What:** A goroutine that polls SQLite every 1 second for the next `queued` message, marks it `dispatching`, sends to Telegram, and records the result. Sequential — one message at a time.

```go
// Source: stdlib time, go-telegram/bot v1.19.0 — verified 2026-02-21
// internal/dispatcher/dispatcher.go
func Start(ctx context.Context, db *sql.DB, b *bot.Bot, cfg *config.Config) {
    ticker := time.NewTicker(1 * time.Second)
    go func() {
        defer ticker.Stop()
        for {
            select {
            case <-ticker.C:
                if err := dispatchNext(ctx, db, b, cfg); err != nil {
                    slog.Error("dispatcher error", "error", err)
                }
            case <-ctx.Done():
                return
            }
        }
    }()
}

func dispatchNext(ctx context.Context, db *sql.DB, b *bot.Bot, cfg *config.Config) error {
    msg, err := db.GetNextQueued(ctx)
    if err != nil {
        return fmt.Errorf("get next queued: %w", err)
    }
    if msg == nil {
        return nil // nothing to dispatch
    }

    return dispatch(ctx, db, b, cfg, msg)
}
```

### Pattern 7: Retry Logic with 429 Handling

**What:** Attempt delivery up to 5 times. On `TooManyRequestsError`, sleep exactly `RetryAfter` seconds before next attempt. On other errors, use exponential backoff. On exhausted retries, mark `failed`.

```go
// Source: go-telegram/bot v1.19.0 errors.go verified 2026-02-21 (local module cache)
// TooManyRequestsError struct: { Message string; RetryAfter int }
// IsTooManyRequestsError(err) bool
func dispatch(ctx context.Context, db *sql.DB, b *bot.Bot, cfg *config.Config, msg *db.Message) error {
    const maxAttempts = 5
    baseDelay := 2 * time.Second

    for attempt := 1; attempt <= maxAttempts; attempt++ {
        if err := markDispatching(ctx, db, msg.ID); err != nil {
            return err
        }

        text := telegram.FormatMessage(msg)
        chatID := cfg.ChatIDForChannel(msg.Channel)

        _, err := b.SendMessage(ctx, &bot.SendMessageParams{
            ChatID:    chatID,
            Text:      text,
            ParseMode: models.ParseModeMarkdown, // = "MarkdownV2" string value!
        })

        db.RecordAttempt(ctx, msg.ID, attempt, err)

        if err == nil {
            db.MarkDelivered(ctx, msg.ID)
            slog.Info("message delivered", "id", msg.ID, "attempt", attempt)
            return nil
        }

        if bot.IsTooManyRequestsError(err) {
            tooMany := err.(*bot.TooManyRequestsError)
            waitDur := time.Duration(tooMany.RetryAfter) * time.Second
            slog.Warn("rate limited by Telegram", "retry_after_sec", tooMany.RetryAfter, "attempt", attempt)
            select {
            case <-time.After(waitDur):
            case <-ctx.Done():
                return ctx.Err()
            }
            continue
        }

        // Exponential backoff for other errors
        if attempt < maxAttempts {
            delay := baseDelay * time.Duration(1<<(attempt-1)) // 2s, 4s, 8s, 16s
            slog.Warn("telegram send failed, retrying", "attempt", attempt, "delay", delay, "error", err)
            select {
            case <-time.After(delay):
            case <-ctx.Done():
                return ctx.Err()
            }
        }
    }

    db.MarkFailed(ctx, msg.ID)
    slog.Error("message delivery failed after max attempts", "id", msg.ID, "attempts", maxAttempts)
    return nil
}
```

**Critical:** `models.ParseModeMarkdown` equals the string `"MarkdownV2"` (verified in `/home/sanchez/go/pkg/mod/github.com/go-telegram/bot@v1.19.0/models/parse_mode.go`). Do NOT use `models.ParseModeMarkdownV1` (= `"Markdown"`, the old V1 format).

### Pattern 8: MarkdownV2 Message Formatting

**What:** Build the formatted message string using `bot.EscapeMarkdown()` for user content, and literal MarkdownV2 syntax for formatting tokens. The `shouldBeEscaped` set in the library is `_*[]()~\`>#+-=|{}.!` (18 characters).

```go
// Source: go-telegram/bot v1.19.0 common.go verified 2026-02-21 (local module cache)
// EscapeMarkdown escapes: _*[]()~`>#+-=|{}.!
// internal/telegram/format.go

var priorityEmoji = map[string]string{
    "low":    "🟢",
    "normal": "🟡",
    "high":   "🔴",
}

func FormatMessage(msg *db.Message) string {
    var sb strings.Builder
    emoji := priorityEmoji[msg.Priority]
    if emoji == "" {
        emoji = "🟡" // fallback
    }

    if msg.Title != nil && *msg.Title != "" {
        // Emoji + bold title on first line: emoji *escaped_title*
        sb.WriteString(emoji)
        sb.WriteString(" *")
        sb.WriteString(bot.EscapeMarkdown(*msg.Title))
        sb.WriteString("*\n")
        // Body below
        sb.WriteString(bot.EscapeMarkdown(msg.Body))
    } else {
        // No title: emoji + body directly
        sb.WriteString(emoji)
        sb.WriteString(" ")
        sb.WriteString(bot.EscapeMarkdown(msg.Body))
    }

    // Tags as hashtags on new line
    if len(msg.Tags) > 0 {
        sb.WriteString("\n")
        for i, tag := range msg.Tags {
            if i > 0 {
                sb.WriteString(" ")
            }
            sb.WriteString("#")
            // Tags: escape special chars but # itself is in shouldBeEscaped set
            // Write # unescaped (it's a valid hashtag prefix), escape the tag content
            sb.WriteString(bot.EscapeMarkdown(tag))
        }
    }

    return sb.String()
}
```

**Warning:** The `#` character IS in `shouldBeEscaped` (`bot.EscapeMarkdown("#disk")` → `\#disk`). For hashtags, write `#` then `bot.EscapeMarkdown(tag)` (the tag content without `#`). Do NOT pass `"#disk"` through EscapeMarkdown — the `#` will be escaped as `\#disk` and Telegram will not render it as a hashtag. Plain `#` in MarkdownV2 is valid when not escaped.

### Pattern 9: Seed API Key Bootstrap

**What:** On startup, for each key in `config.seed_api_keys`, SHA-256 hash the `sk-...` value, check if the hash exists in `api_keys`, insert if absent. Idempotent — safe to run every startup.

```go
// internal/db/apikeys.go
func SeedKeys(ctx context.Context, db *sql.DB, keys []config.SeedAPIKey) error {
    for _, key := range keys {
        if !strings.HasPrefix(key.Key, "sk-") {
            return fmt.Errorf("seed key %q: must start with sk-", key.Name)
        }
        hash := sha256.Sum256([]byte(key.Key))
        hashHex := hex.EncodeToString(hash[:])

        // INSERT OR IGNORE handles idempotency
        _, err := db.ExecContext(ctx, `
            INSERT OR IGNORE INTO api_keys (id, key_hash, name)
            VALUES (?, ?, ?)`,
            uuid.New().String(), hashHex, key.Name,
        )
        if err != nil {
            return fmt.Errorf("seed key %q: %w", key.Name, err)
        }
    }
    return nil
}
```

**Config struct additions:**

```go
// internal/config/config.go additions
type ServerConfig struct {
    Listen string `yaml:"listen"` // default: "127.0.0.1:8080"
}

type SeedAPIKey struct {
    Name string `yaml:"name"`
    Key  string `yaml:"key"` // full sk-... value
}

// Add to Config:
type Config struct {
    Telegram    TelegramConfig  `yaml:"telegram"`
    Database    DatabaseConfig  `yaml:"database"`
    Server      ServerConfig    `yaml:"server"`
    Channels    []ChannelConfig `yaml:"channels"`
    SeedAPIKeys []SeedAPIKey    `yaml:"seed_api_keys"`
}
```

### Pattern 10: Schema Migration — Nullable Title

**What:** Phase 1 schema has `title TEXT NOT NULL`. Phase 2 requires `title` to be optional. SQLite does not support `ALTER COLUMN`, so a migration copies the table, drops, and renames.

```sql
-- internal/db/schema/002_nullable_title.sql
-- SQLite cannot ALTER COLUMN constraints; use shadow table approach.
-- This makes title nullable to support messages without a title line.

ALTER TABLE messages RENAME TO messages_old;

CREATE TABLE messages (
    id          TEXT PRIMARY KEY,
    channel     TEXT NOT NULL,
    priority    TEXT NOT NULL DEFAULT 'normal',
    title       TEXT,                           -- now nullable
    body        TEXT NOT NULL,
    tags        TEXT,
    metadata    TEXT,
    status      TEXT NOT NULL DEFAULT 'queued',
    created_at  TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at  TEXT NOT NULL DEFAULT (datetime('now'))
);

INSERT INTO messages SELECT * FROM messages_old;
DROP TABLE messages_old;

CREATE INDEX IF NOT EXISTS idx_messages_status ON messages(status);
CREATE INDEX IF NOT EXISTS idx_messages_created_at ON messages(created_at);
```

**Alternative:** Insert empty string `""` as title and treat `""` as "no title" in formatter. Avoids migration but requires sentinel value logic. The migration approach is cleaner.

### Anti-Patterns to Avoid

- **Using `models.ParseModeMarkdownV1`:** The constant `ParseModeMarkdownV1 = "Markdown"` is the OLD v1 format. Use `models.ParseModeMarkdown` (= `"MarkdownV2"`). This naming is counterintuitive but verified in the local module cache.
- **Calling `w.Write()` before `w.WriteHeader(202)`:** Causes Go to auto-send 200 first. Set headers, call WriteHeader(202), then write body.
- **Escaping `#` before hashtags:** `bot.EscapeMarkdown("#tag")` produces `\#tag` — not a hashtag. Write `#` literally, escape only the tag content: `"#" + bot.EscapeMarkdown(tag)`.
- **Comparing raw tokens:** Never store or compare raw `sk-...` values. Always hash first.
- **Blocking the dispatcher on DB lookup:** Use `db.GetNextQueued` with a short context timeout, not the global shutdown context, so a slow DB doesn't block shutdown.
- **Ignoring `http.ErrServerClosed`:** When `server.Shutdown()` is called, `ListenAndServe()` returns `http.ErrServerClosed`. This must be ignored — it is not an error.
- **Using `r.Route("/api/v1", ...)` for auth group:** `r.Route()` creates a sub-router mounted at a path prefix (needed for URL parameter extraction). For middleware grouping without a new prefix, use `r.Group()`. Both work here but `r.Group()` is semantically correct when all routes are in the same `/api/v1` namespace.
- **Starting dispatcher before HTTP server is ready:** Dispatcher and HTTP server can start in any order since they both read from the same DB. But log "started" only after both are running.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| HTTP routing with middleware groups | Custom `net/http` mux | `go-chi/chi/v5` | Middleware group isolation (`r.Group()`), stdlib-compatible, 0 deps |
| MarkdownV2 character escaping | Custom escape function | `bot.EscapeMarkdown()` | Verified against Telegram's 18-char set (`_*[]()~\`>#+-=|{}.!`); in local cache |
| 429 retry_after extraction | Parse raw HTTP response | `bot.IsTooManyRequestsError()` + `TooManyRequestsError.RetryAfter` | Library already parses the Telegram response; RetryAfter is `int` (seconds) |
| UUID generation | Random hex string | `uuid.NewV7()` | Time-ordered, collision-resistant, RFC 9562 standard |
| Timing-safe token comparison | `==` string comparison | `subtle.ConstantTimeCompare` | Prevents timing attacks; equal-length SHA-256 hashes make this effective |
| JSON encoding | `fmt.Sprintf` or manual | `json.NewEncoder(w).Encode()` | Correct escaping, Content-Type enforcement, streaming |

**Key insight:** The most error-prone parts of this phase are (1) the `ParseMode` constant naming trap, (2) the MarkdownV2 escaping rules for formatting tokens (`*`, `\n`) vs user content (EscapeMarkdown), and (3) the `http.ErrServerClosed` non-error from graceful shutdown. Get these three right and the rest is mechanical.

---

## Common Pitfalls

### Pitfall 1: Wrong ParseMode Constant (CRITICAL)

**What goes wrong:** Using `models.ParseModeMarkdownV1` (thinking it means "V1 = old") or constructing the raw string `"MarkdownV2"` instead of using the library constant.

**Why it happens:** The constant naming is backwards: `ParseModeMarkdownV1 = "Markdown"` (old v1 format), `ParseModeMarkdown = "MarkdownV2"` (current v2 format). Sending `"Markdown"` causes Telegram to use the v1 parser, which will fail on text escaped for v2.

**How to avoid:** Always use `models.ParseModeMarkdown`. Verified in `/home/sanchez/go/pkg/mod/github.com/go-telegram/bot@v1.19.0/models/parse_mode.go`.

**Warning signs:** Telegram API returns a 400 error mentioning "can't parse entities" or special characters cause message send failures.

### Pitfall 2: Hashtag Escaping Breaks # Symbol

**What goes wrong:** Passing `"#tagname"` through `bot.EscapeMarkdown()` produces `"\#tagname"` — the `#` is in the escape set and becomes `\#`, which Telegram does not render as a hashtag.

**Why it happens:** The `shouldBeEscaped` string in `common.go` includes `#`. EscapeMarkdown escapes all 18 special characters unconditionally.

**How to avoid:** Build hashtags as `"#" + bot.EscapeMarkdown(tagname)`. The bare `#` in MarkdownV2 is valid (it's a formatting character, not a hashtag trigger — Telegram renders bare `#word` as text, not a hashtag in MarkdownV2 mode). Use `EscapeMarkdown` only on the tag name itself.

**Warning signs:** Tags appear in Telegram as `\#tagname` (with a visible backslash) instead of being rendered as hashtag links.

### Pitfall 3: WriteHeader Called After Write

**What goes wrong:** Calling `json.NewEncoder(w).Encode(...)` before `w.WriteHeader(202)` causes Go to auto-flush a 200 response before your 202 is set. The 202 call is then silently ignored.

**Why it happens:** Any write to the response body triggers implicit `WriteHeader(200)` in Go's `http.ResponseWriter`.

**How to avoid:** Always follow this order: (1) `w.Header().Set(...)`, (2) `w.WriteHeader(statusCode)`, (3) `w.Write(...)` / `Encode(...)`. Only for 200 responses can you skip `WriteHeader()`.

**Warning signs:** API returns 200 instead of 202 on successful message creation.

### Pitfall 4: http.ErrServerClosed Treated as Fatal

**What goes wrong:** The goroutine running `server.ListenAndServe()` gets a non-nil error when `Shutdown()` is called. If you `log.Fatal(err)` or `os.Exit(1)` on this error, graceful shutdown fails.

**Why it happens:** `ListenAndServe` always returns non-nil — either a startup error (bad address) or `http.ErrServerClosed` on clean shutdown. These must be distinguished.

**How to avoid:** Use `errors.Is(err, http.ErrServerClosed)` to ignore the shutdown signal. Only fatal on a real startup error.

```go
if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
    slog.Error("HTTP server failed", "error", err)
    os.Exit(1)
}
```

### Pitfall 5: Schema Migration Needed for Nullable Title

**What goes wrong:** Phase 1 schema has `title TEXT NOT NULL`. Phase 2 context requires optional title (`"Omitted title means no bold title line"`). Inserting a message without a title fails with `NOT NULL constraint failed: messages.title`.

**Why it happens:** The schema was designed before Phase 2 requirements were finalized. Phase 1 research assumed title would be required.

**How to avoid:** Add `002_nullable_title.sql` migration (adlio/schema applies it idempotently). The migration uses SQLite's shadow-table pattern (rename, recreate, copy, drop) since SQLite doesn't support `ALTER COLUMN`.

**Warning signs:** `EnqueueMessage()` fails with `NOT NULL constraint failed: messages.title` when title is nil.

### Pitfall 6: Dispatcher Context Cancellation During Sleep

**What goes wrong:** During a retry backoff sleep (`time.Sleep`), if the service receives SIGTERM, the goroutine won't respond until the sleep expires. Long 429 `retry_after` values (Telegram can return 60+ seconds) cause slow shutdown.

**Why it happens:** `time.Sleep` is not cancellation-aware.

**How to avoid:** Use `select { case <-time.After(dur): case <-ctx.Done(): return ctx.Err() }` instead of `time.Sleep`. This allows immediate exit on shutdown signal.

### Pitfall 7: Seed Key Hashing Must Match Auth Middleware

**What goes wrong:** Seed key bootstrap hashes keys with one algorithm/encoding; auth middleware hashes tokens with another. Lookups always fail.

**Why it happens:** Inconsistency in hash encoding (raw bytes vs hex string) or algorithm (SHA-256 vs SHA-512).

**How to avoid:** Use a single shared `hashToken(token string) string` helper in `internal/db/apikeys.go` that both the seed bootstrap and the auth middleware import. Hash = `hex.EncodeToString(sha256.Sum256([]byte(token)))`.

---

## Code Examples

Verified patterns from official sources and local module cache:

### TooManyRequestsError (from local cache)

```go
// Source: /home/sanchez/go/pkg/mod/github.com/go-telegram/bot@v1.19.0/errors.go
type TooManyRequestsError struct {
    Message    string
    RetryAfter int  // seconds
}

func IsTooManyRequestsError(err error) bool {
    _, ok := err.(*TooManyRequestsError)
    return ok
}

// Usage:
if bot.IsTooManyRequestsError(err) {
    tooMany := err.(*bot.TooManyRequestsError)
    wait := time.Duration(tooMany.RetryAfter) * time.Second
    select {
    case <-time.After(wait):
    case <-ctx.Done():
        return ctx.Err()
    }
}
```

### ParseMode Constants (from local cache)

```go
// Source: /home/sanchez/go/pkg/mod/github.com/go-telegram/bot@v1.19.0/models/parse_mode.go
// VERIFIED LOCALLY — counterintuitive naming:
const (
    ParseModeMarkdownV1 ParseMode = "Markdown"   // OLD v1 format — DO NOT USE
    ParseModeMarkdown   ParseMode = "MarkdownV2" // CURRENT v2 format — USE THIS
    ParseModeHTML       ParseMode = "HTML"
)

// Correct usage:
b.SendMessage(ctx, &bot.SendMessageParams{
    ChatID:    chatID,
    Text:      formattedText,
    ParseMode: models.ParseModeMarkdown, // = "MarkdownV2" string
})
```

### EscapeMarkdown Characters (from local cache)

```go
// Source: /home/sanchez/go/pkg/mod/github.com/go-telegram/bot@v1.19.0/common.go
// shouldBeEscaped = "_*[]()~`>#+-=|{}.!" (18 chars + backslash handling)

// These characters are escaped by bot.EscapeMarkdown():
// _ * [ ] ( ) ~ ` > # + - = | { } . !

// Bold formatting: surround escaped text with literal * (not escaped)
boldTitle := "*" + bot.EscapeMarkdown(userTitle) + "*"

// Hashtags: write # literal, escape only the tag content
hashtag := "#" + bot.EscapeMarkdown(tagName)  // NOT bot.EscapeMarkdown("#" + tagName)
```

### SendMessageParams (from local cache)

```go
// Source: /home/sanchez/go/pkg/mod/github.com/go-telegram/bot@v1.19.0/methods_params.go
type SendMessageParams struct {
    BusinessConnectionID  string                    `json:"business_connection_id,omitempty"`
    ChatID                any                       `json:"chat_id"`           // int64 or string (@username)
    MessageThreadID       int                       `json:"message_thread_id,omitempty"`
    Text                  string                    `json:"text"`
    ParseMode             models.ParseMode          `json:"parse_mode,omitempty"`
    Entities              []models.MessageEntity    `json:"entities,omitempty"`
    LinkPreviewOptions    *models.LinkPreviewOptions `json:"link_preview_options,omitempty"`
    DisableNotification   bool                      `json:"disable_notification,omitempty"`
    ProtectContent        bool                      `json:"protect_content,omitempty"`
    // ... other fields omitted for brevity
}
```

### UUID v7 Message ID

```go
// Source: pkg.go.dev/github.com/google/uuid v1.6.0 — verified 2026-02-21
// NewV7() returns time-ordered UUID; format: xxxxxxxx-xxxx-7xxx-xxxx-xxxxxxxxxxxx

id, err := uuid.NewV7()
if err != nil {
    return fmt.Errorf("generate message ID: %w", err)
}
messageID := id.String()  // standard UUID format: "01234567-89ab-7cde-f012-3456789abcde"
```

### Chi Graceful Shutdown (from official chi example)

```go
// Source: github.com/go-chi/chi/blob/master/_examples/graceful/main.go — verified 2026-02-21
server := &http.Server{Addr: cfg.Server.Listen, Handler: router}

go func() {
    if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
        slog.Error("HTTP server failed", "error", err)
        os.Exit(1)
    }
}()

<-ctx.Done() // wait for SIGTERM/SIGINT

shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()
server.Shutdown(shutdownCtx)
```

### Startup Sequence Additions for Phase 2 (main.go)

```go
// Additions to existing 12-step startup sequence in cmd/jaimito/main.go:

// After step 9 (ReclaimStuck) and before existing step 10 (cleanup.Start):

// 10. Seed API keys from config — idempotent bootstrap
if err := db.SeedKeys(ctx, database, cfg.SeedAPIKeys); err != nil {
    slog.Error("failed to seed API keys", "error", err)
    os.Exit(1)
}

// 11. Start Telegram dispatcher
dispatcher.Start(ctx, database, tgBot, cfg)

// 12. Build and start HTTP server
router := api.NewRouter(database, tgBot, cfg)
server := &http.Server{
    Addr:         cfg.Server.Listen,
    Handler:      router,
    ReadTimeout:  15 * time.Second,
    WriteTimeout: 15 * time.Second,
}
go func() {
    if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
        slog.Error("HTTP server failed", "error", err)
        os.Exit(1)
    }
}()

slog.Info("jaimito started",
    "addr", cfg.Server.Listen,
    "channels", len(cfg.Channels),
    "db", cfg.Database.Path,
)

<-ctx.Done()
slog.Info("shutting down")

shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()
server.Shutdown(shutdownCtx)
```

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `gorilla/mux` | `go-chi/chi/v5` | gorilla archived ~2022 | chi is the community standard for stdlib-compatible routing |
| Raw `net/http` with manual mux | chi `r.Group()` + `r.Use()` | chi v5 (Go 1.22+) | Middleware grouping makes auth-protected subroutes clean |
| `time.Sleep` in retry loops | `select { case <-time.After(); case <-ctx.Done(): }` | Standard Go practice since Go 1.7 | Context-aware sleep allows graceful shutdown during backoff |
| `Markdown` parse mode (v1) | `MarkdownV2` / `models.ParseModeMarkdown` | Telegram Bot API introduced V2 in 2019 | V2 requires explicit escaping but supports more formatting options |
| `mattn/go-sqlite3` (CGO) | `modernc.org/sqlite` (already established) | Already decided Phase 1 | Unchanged |

**Deprecated/outdated:**
- `models.ParseModeMarkdownV1` (`"Markdown"`): The old format; use `models.ParseModeMarkdown` (`"MarkdownV2"`) for all new messages
- `gorilla/mux`: Archived, unsupported; replaced by chi
- `time.Sleep` for retry delays: Not context-cancellable; replace with `select` + `time.After`

---

## Open Questions

1. **`adaptive_retry` field in TooManyRequestsError**
   - What we know: STATE.md notes "Verify whether `go-telegram/bot` v1.19.0 `TooManyRequestsError` exposes the Bot API 8.0 `adaptive_retry` field". The local module cache at v1.19.0 shows `TooManyRequestsError` has only `Message string` and `RetryAfter int`. No `AdaptiveRetry` or `adaptive_retry` field exists in v1.19.0.
   - What's unclear: Whether a future version will add it.
   - Recommendation: Use `RetryAfter int` (seconds) as-is. **This blocker from STATE.md is resolved: NO adaptive_retry field exists in v1.19.0.**

2. **Default listen address**
   - What we know: CONTEXT.md grants Claude's discretion for the default. The phase description says "likely localhost-only for security".
   - Recommendation: Default to `127.0.0.1:8080` — localhost only, non-privileged port. Systemd can expose it externally if needed via reverse proxy. Do NOT default to `0.0.0.0:8080` for a VPS service with no external users.

3. **Exponential backoff parameters**
   - What we know: CONTEXT.md grants discretion for "initial delay, multiplier, jitter".
   - Recommendation: `baseDelay = 2s`, double per non-429 attempt (2s, 4s, 8s, 16s for attempts 1-4). For 429 errors, use exact `retry_after` seconds. No jitter needed since sequential dispatch means only one outstanding request at a time.

4. **tags column format in SQLite**
   - What we know: The schema has `tags TEXT`. For JSON arrays, this means storing `["tag1","tag2"]` as a JSON string.
   - Recommendation: Store as `json.Marshal([]string{...})` and unmarshal on read. SQLite has JSON functions but encoding/decoding in Go is sufficient.

5. **metadata column format in SQLite**
   - What we know: The schema has `metadata TEXT`. API accepts `map[string]any`.
   - Recommendation: Store as `json.Marshal(map[string]any{...})`. No need for SQLite JSON functions at this phase.

6. **Config validation: server.listen**
   - What we know: The `server.listen` field is new in Phase 2. Config.Validate() needs to handle it.
   - Recommendation: If empty, set default `127.0.0.1:8080` in the Load() function (same pattern as db.path defaulting). No validation beyond non-empty after default.

---

## Sources

### Primary (HIGH confidence)

- Local module cache `/home/sanchez/go/pkg/mod/github.com/go-telegram/bot@v1.19.0/models/parse_mode.go` — ParseMode constants confirmed: `ParseModeMarkdown = "MarkdownV2"`, `ParseModeMarkdownV1 = "Markdown"`
- Local module cache `/home/sanchez/go/pkg/mod/github.com/go-telegram/bot@v1.19.0/errors.go` — `TooManyRequestsError{Message string, RetryAfter int}`, `IsTooManyRequestsError()` — no AdaptiveRetry field
- Local module cache `/home/sanchez/go/pkg/mod/github.com/go-telegram/bot@v1.19.0/common.go` — `EscapeMarkdown()`, `EscapeMarkdownUnescaped()`, `shouldBeEscaped = "_*[]()~\`>#+-=|{}.!"`
- Local module cache `/home/sanchez/go/pkg/mod/github.com/go-telegram/bot@v1.19.0/methods_params.go` — `SendMessageParams` struct with `ChatID any`, `ParseMode models.ParseMode`
- `pkg.go.dev/github.com/go-chi/chi/v5/middleware` — Complete middleware listing: BasicAuth exists, BearerToken does NOT exist (confirmed absence)
- `pkg.go.dev/github.com/go-chi/chi/v5` — Router interface, `r.Group()`, `r.Route()`, `r.Use()`, subrouter patterns
- `github.com/go-chi/chi/blob/master/_examples/graceful/main.go` — Graceful shutdown: signal.NotifyContext + server.Shutdown + 30s timeout
- `pkg.go.dev/github.com/google/uuid` — `NewV7()` returns time-ordered UUID v7 in standard format

### Secondary (MEDIUM confidence)

- WebSearch + WebFetch on chi v5 middleware — Confirmed no BearerToken built-in; BasicAuth is the only auth middleware; consistent with local pkg.go.dev verification
- WebSearch on Telegram Bot API 429 handling — `retry_after` is `int` (seconds); response JSON: `{"parameters": {"retry_after": N}}`; aligns with library implementation
- WebFetch alexedwards.net `how-to-properly-parse-a-json-request-body` — `http.MaxBytesReader` + `json.NewDecoder` + specific error type handling pattern
- WebFetch `core.telegram.org/bots/api` — MarkdownV2 requires escaping 18 special characters: `_*[]()~\`>#+-=|{}.!`

### Tertiary (LOW confidence)

- WebSearch on Telegram "adaptive_retry" field — Reports mention Bot API 7.8 (June 2025) adding `scope` field; "adaptive rate limits" referenced as future; no `adaptive_retry` field found. OVERRIDDEN by HIGH-confidence direct source inspection of local v1.19.0 module.

---

## Metadata

**Confidence breakdown:**
- Standard stack (chi, uuid, go-telegram/bot): HIGH — pkg.go.dev and local module cache verified
- ParseMode constant naming: HIGH — verified directly in local module cache at v1.19.0
- TooManyRequestsError fields: HIGH — verified directly in local module cache at v1.19.0
- EscapeMarkdown behavior: HIGH — verified directly in local module cache at v1.19.0
- chi middleware listing: HIGH — complete list fetched from pkg.go.dev; no BearerToken exists
- Architecture patterns: HIGH — standard Go patterns; chi example verified from official repo
- Schema migration approach (002): MEDIUM — SQLite shadow-table migration pattern is well-established; adlio/schema ordering is correct; not verified against adlio docs specifically
- Exponential backoff parameters: LOW — chosen as reasonable defaults; no authoritative source for these specific values

**STATE.md blocker resolved:** "Verify whether `go-telegram/bot` v1.19.0 `TooManyRequestsError` exposes the Bot API 8.0 `adaptive_retry` field" — **RESOLVED: NO. The struct has only `Message string` and `RetryAfter int`. Use `RetryAfter int` (seconds) directly.**

**Research date:** 2026-02-21
**Valid until:** 2026-04-21 (chi v5 and go-telegram/bot are stable; core patterns unlikely to change)
