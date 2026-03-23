# Design: `jaimito setup` — Interactive CLI Setup Wizard

**Date:** 2026-03-23
**Status:** Approved

## Problem

After installing jaimito, the user must manually edit `/etc/jaimito/config.yaml` with a Telegram bot token, chat IDs, and an API key. This requires knowing how to create a bot, how to get chat IDs, and how to generate secure keys — all before seeing jaimito work. New users bounce at this step.

## Solution

A `jaimito setup` subcommand that guides the user through configuration interactively, validates every input against the Telegram API in real time, generates credentials automatically, and sends a test notification to prove it works — all before the user has to read any documentation.

## Scope

- New cobra subcommand: `jaimito setup`
- Uses `charmbracelet/bubbletea` + `bubbles` + `lipgloss` for TUI
- Integrated into `install.sh` (replaces config.example.yaml copy)
- install.sh requires root (`sudo`) — wizard writes config directly

## Architecture

### File Structure

```
cmd/jaimito/
├── setup.go              # Cobra command definition, launches wizard
└── setup/
    ├── wizard.go          # Main bubbletea model (state machine orchestrator)
    ├── steps.go           # Step interface + all step implementations
    ├── validate.go        # Live validation (Telegram API calls, wrapped as tea.Cmd)
    ├── config.go          # YAML generation + file writing
    └── styles.go          # lipgloss theme (colors, boxes, layout)
```

### Dependencies

New:
- `github.com/charmbracelet/bubbletea` — TUI framework
- `github.com/charmbracelet/bubbles` — Components (textinput, spinner, list)
- `github.com/charmbracelet/lipgloss` — Styling

Existing (reused, not reimplemented):
- `internal/config` — `config.Validate()` for final config validation
- `internal/telegram` — `ValidateToken()`, `ValidateChats()` for live validation
- `internal/db` — `CreateKey()` for API key generation with crypto/rand
- `go-telegram/bot` — `bot.SendMessage()` for test notification

### State Machine

The wizard is a linear state machine. Each step collects one piece of data and validates it before advancing. The user can never be stuck — every validation failure offers retry.

```
Welcome → DetectConfig → BotToken → ChannelGeneral → ChannelsExtra
→ Server → Database → APIKey → Summary → WriteConfig → TestNotification → Done
```

Each step implements:

```go
type Step interface {
    Init() tea.Cmd
    Update(msg tea.Msg) (Step, tea.Cmd)
    View() string
    Result() any
}
```

The main wizard model holds:
- `steps []Step` — ordered list of steps
- `current int` — index of active step
- `data *SetupData` — accumulated configuration values

```go
type SetupData struct {
    BotToken    string
    BotUsername string
    BotName     string
    Channels    []ChannelData
    Listen      string
    DBPath      string
    APIKeyRaw   string
    APIKeyName  string
    ConfigPath  string
    ExistingCfg *config.Config // non-nil if editing existing config
}

type ChannelData struct {
    Name     string
    ChatID   int64
    ChatName string
    ChatType string
    Priority string
}
```

### Validation Strategy

All validations run as `tea.Cmd` (async) so the UI stays responsive with a spinner:

- **Bot token**: calls `telegram.ValidateToken(ctx, token)` — returns bot info or error
- **Chat ID**: calls `bot.GetChat()` via go-telegram/bot — returns chat name/type or error
- **Final config**: calls `config.Validate()` on the generated config struct before writing

On validation failure: show the error, keep the current step active, allow the user to edit their input and retry. No step limit on retries.

### API Key Generation

Uses `internal/db.CreateKey()` indirectly — the same `crypto/rand` 32-byte + hex + `sk-` prefix logic, but without DB insertion (the key goes into the YAML as a seed key). The wizard generates the key in-process:

```go
bytes := make([]byte, 32)
crypto/rand.Read(bytes)
key := "sk-" + hex.EncodeToString(bytes)
```

The raw key is displayed once and written to `seed_api_keys` in the config. On first server startup, `SeedKeys()` hashes and inserts it.

### Config Writing

Generates YAML using `gopkg.in/yaml.v3` marshal from a config struct (not string templates). This guarantees the output is valid YAML and consistent with what `config.Load()` expects.

Writes to `--config` flag path (default `/etc/jaimito/config.yaml`). Sets file permissions to `0600` (owner read/write only) since the file contains the bot token and API key.

Creates parent directory `/etc/jaimito/` if it doesn't exist.

### Test Notification

Sends directly via the bot API without starting the server:

1. Create bot instance (already validated in step 2)
2. Format a test message using `telegram.FormatMessage()`:
   - Channel: "general"
   - Priority: "normal"
   - Title: "jaimito setup"
   - Body: "Setup completado — las notificaciones funcionan correctamente"
3. Send via `bot.SendMessage()` to the general channel's chat_id
4. Report success or failure

### install.sh Integration

Replace the current config.example.yaml copy block with:

```bash
if [ ! -f "$CONFIG_FILE" ]; then
    info "Iniciando setup interactivo..."
    jaimito setup --config "$CONFIG_FILE"
else
    ok "Config existente preservada en ${CONFIG_FILE}"
fi
```

Since install.sh runs as root, the wizard has full permissions to write anywhere.

Add root check at the top of install.sh:

```bash
if [ "$EUID" -ne 0 ]; then
    error "Este instalador requiere root. Ejecutar con: sudo bash install.sh"
fi
```

## Visual Theme

### Colors (lipgloss)

| Role | Color | Hex | Usage |
|------|-------|-----|-------|
| Primary | Cyan | #00BFFF | Titles, borders, prompts, step counter |
| Success | Green | #00FF7F | Validation OK, checkmarks |
| Error | Red | #FF6B6B | Validation failures, warnings |
| Highlight | Yellow | #FFD700 | API key display, important data |
| Muted | Gray | #666666 | Hints, secondary text, instructions |

### Layout

Every step renders inside a consistent frame:

```
  Step N/8 — Step Title

  [content area — instructions, prompts, results]

  [hint in gray]
```

Spinner: dots style (⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏) in cyan during async validations.

Boxes: lipgloss border for summary table and API key display.

## Step-by-Step UX Detail

### Step 1: Welcome + Detect Config

Shows banner and purpose. If config exists at the target path, offers three options via bubbles list selector:
- **Editar configuración existente** — loads current values as defaults in every subsequent step
- **Empezar de cero** — ignores existing config
- **Cancelar** — exits

If no config exists, skips straight to step 2.

### Step 2: Bot Token

Shows inline instructions for creating a bot via BotFather (5 numbered steps). Text input for the token. Validates with `ValidateToken()` + spinner. On success, displays bot username and display name. On failure, shows error and stays on the same prompt for retry.

If editing existing config, pre-fills the current token (masked except last 4 chars).

### Step 3: General Channel

Shows inline instructions for obtaining chat_id via the getUpdates API trick (4 numbered steps, with the URL templated using the token just validated). Text input for chat_id. Validates with `bot.GetChat()` + spinner. On success, shows chat title and type. Then offers priority selection (low/normal/high) via list selector, default normal.

### Step 4: Extra Channels

Multi-select list of predefined channels: cron, errors, deploys, system, security, monitoring. Each has a one-line description. User toggles with space, confirms with Enter. Can select none.

For each selected channel: prompts for chat_id (with the general channel's chat_id as the default — just press Enter if same group), validates, then asks for priority. Default priorities: cron=low, errors=high, deploys=normal, system=normal, security=high, monitoring=normal.

### Step 5: Server

Text input for listen address, pre-filled with `127.0.0.1:8080`. Basic format validation (host:port pattern). No network validation — just syntax.

### Step 6: Database

Text input for database path, pre-filled with `/var/lib/jaimito/jaimito.db`. Verifies parent directory exists or can be created.

### Step 7: API Key

No user input — auto-generates the key and displays it in a highlighted box. Shows a warning that it cannot be recovered. Prompts "¿La copiaste? (s/n)" and only advances on "s".

If editing existing config, shows the existing seed key name and asks if the user wants to generate a new one or keep the existing.

### Step 8: Summary

Renders a bordered table with all configured values:
- Bot username
- Listen address
- Database path
- API key (truncated: `sk-a1b2...f6`)
- Channel table (name → chat name, priority emoji)

Three options: Guardar / Volver a revisar / Cancelar.

"Volver a revisar" restarts from step 2 with all current values pre-filled.

### Step 9: Write Config + Test + Done

Writes config YAML, shows path and permissions. Offers test notification (y/n). If yes, sends via bot API directly, reports result. Shows final "setup completo" banner with useful commands (systemctl, send, wrap with crontab example).

## Edge Cases

- **Ctrl+C at any point**: bubbletea handles SIGINT gracefully, exits cleanly
- **Terminal too narrow**: lipgloss gracefully degrades, wrapping text
- **Non-interactive terminal** (piped stdin): detect with `term.IsTerminal()`, print error suggesting interactive mode
- **Bot not added to chat**: getChat fails — error message explains "Asegurate de que el bot esté agregado al grupo"
- **Same chat_id for multiple channels**: valid and common — the wizard handles this by showing "Mismo chat que general" and skipping re-validation
- **Existing config with invalid values**: when editing, loads values but re-validates everything during the wizard flow

## Non-Goals

- No web UI — this is terminal-only
- No editing individual config fields (use `nano` for that) — the wizard is for initial setup or full reconfiguration
- No channel deletion from existing config — start fresh if needed
- No automatic BotFather interaction — the user creates the bot manually
