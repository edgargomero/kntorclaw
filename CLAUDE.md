# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

PicoClaw is an ultra-lightweight personal AI assistant written in Go, designed to run on minimal hardware (<10MB RAM, <1s boot). It's a Go rewrite of the OpenClaw TypeScript project, targeting embedded devices (RISC-V, ARM, x86_64). Module path: `github.com/sipeed/picoclaw`.

## Build & Development Commands

```bash
make build          # Build for current platform → build/picoclaw-{platform}-{arch}
make build-all      # Cross-compile (linux/darwin/windows × amd64/arm64/riscv64)
make install        # Build + install to ~/.local/bin
make test           # go test ./...
make fmt            # go fmt ./...
make vet            # go vet ./...
make generate       # Embed workspace files (runs before build automatically)
make clean          # Remove build artifacts
make deps           # go get -u ./... && go mod tidy
```

GoReleaser is configured (`.goreleaser.yaml`) for releases with UPX compression.

## Architecture

Entry point: `cmd/picoclaw/main.go` — CLI with subcommands: `onboard`, `agent`, `gateway`, `status`, `migrate`, `auth`, `cron`, `skills`.

### Core Packages (`pkg/`)

| Package | Purpose |
|---------|---------|
| `agent` | Agent execution loop, context building, memory management |
| `providers` | LLM provider abstraction (Anthropic Claude SDK, OpenAI SDK, HTTP-based for OpenRouter/Gemini/Zhipu/Groq/vLLM, GitHub Copilot) |
| `tools` | Tool registry + implementations: filesystem (`read_file`, `write_file`, `edit_file`, `list_dir`, `append_file`), shell (`exec` with safety guards), web (`web_search`, `web_fetch`), hardware (`i2c`, `spi`), agent (`spawn`, `subagent`, `message`), scheduling (`cron`) |
| `channels` | Multi-platform messaging: Telegram, Discord, Slack, LINE, QQ, DingTalk, Feishu/Lark, WhatsApp, OneBot, MaixCam |
| `bus` | Message bus for decoupled inter-component communication |
| `session` | Conversation session and history management |
| `config` | JSON config loading with env var overrides (`PICOCLAW_*` via `caarlos0/env`) |
| `skills` | Skill system: loader + GitHub-based installer |
| `auth` | OAuth/token authentication (OpenAI, Anthropic) with PKCE |
| `cron` | Scheduled task execution (gronx-based) |
| `heartbeat` | Periodic tasks driven by `HEARTBEAT.md` |
| `voice` | Voice transcription via Groq Whisper |
| `devices` | Device event monitoring (USB on Linux) |
| `migrate` | OpenClaw → PicoClaw migration tooling |
| `state` | Atomic state persistence |

### Runtime Modes

- **Agent mode** (`picoclaw agent`): One-shot (`-m "message"`) or interactive CLI
- **Gateway mode** (`picoclaw gateway`): Long-running multi-channel bot — initializes channels, cron, heartbeat, device monitoring, and agent loop via message bus

### Workspace

Default workspace (`workspace/`) is embedded into the binary at build time via `go generate`. User workspace lives at `~/.picoclaw/workspace/` with `AGENT.md`, `IDENTITY.md`, `SOUL.md`, `USER.md`, `memory/`, and `skills/`.

### Configuration

Primary config: `~/.picoclaw/config.json` (see `config/config.example.json`). All values overridable via `PICOCLAW_*` env vars. Key sections: `agents.defaults`, `channels`, `providers`, `tools.web`, `heartbeat`, `devices`, `gateway`.

### Model Routing

The `ModelRouter` (`pkg/agent/router.go`) resolves which LLM model to use with a 3-layer priority:

1. **Session override** — runtime only, set via model picker (scope: session)
2. **Channel override** — persisted in `config.json` under `agents.models` (e.g. `"whatsapp": "claude-sonnet-4-5-20250929"`)
3. **Default model** — `agents.defaults.model` fallback

The `MultiProvider` (`pkg/providers/multi_provider.go`) auto-dispatches to the correct provider based on model name patterns (`claude*` → Anthropic, `gpt*` → OpenAI, etc.).

### Token Usage Persistence

The `ActivityTracker` (`pkg/tui/activity.go`) tracks per-session token usage in memory and persists historical totals to `~/.picoclaw/token_usage.json`. The TOKENS panel shows both session counts (In/Out) and cumulative totals (ΣIn/ΣOut).

### TUI Architecture (`pkg/tui/`)

Three layout modes switchable via keybindings:

| Mode | Toggle | Layout |
|------|--------|--------|
| Normal | default | Chat+Input (50%) \| Channels+Tokens+Sessions+Logs (50%) |
| Config | F8 | Sections list (25ch) \| Items list + footer |
| Focus | F9 | Files+Branches+Commits+QA (30%) \| Chat+Diff (70%) |

Key TUI files:

| File | Purpose |
|------|---------|
| `app.go` | App struct, Init(), lifecycle |
| `layout.go` | Panel layout, status bar, toast messages |
| `keybindings.go` | Global input capture, help screen (`?`), error viewer (Ctrl+E) |
| `model_picker.go` | Multi-channel model picker (Alt+M) with scope selection |
| `config_view.go` | F8 interactive config: sections/items navigation, delete confirmation |
| `config_editor.go` | Modal editors for providers, models, channels, aliases |
| `tokens.go` | TOKENS panel with session + historical totals |
| `sessions.go` | SESSIONS panel with live activity tracking |
| `channels_panel.go` | CHANNELS panel with online/offline status |
| `error_tracker.go` | Error aggregation with badge + persistent log |
| `activity.go` | ActivityTracker with in-memory + JSON persistence |

TUI keybindings (also available via `?` help screen):

- **Normal**: F1-F5 panels, Tab/Shift+Tab navigate, F8 config, F9 focus, Alt+M model picker, Ctrl+E errors, `?` help
- **Config**: Tab switch panel, Enter select, `d` delete (with y/n confirm), Esc back
- **Focus**: F1-F6 panels, Enter/Esc approve/reject QA checkpoints
- **Model Picker**: ←/→ switch channel, Tab cycle scope (session/channel/default), Enter apply

## Relationship to OpenClaw

The sibling repo at `../openclaw` is the TypeScript/Swift/Kotlin multi-platform version. PicoClaw reimplements the same agent concept as a single Go binary for constrained environments. The `pkg/migrate` package handles migration from OpenClaw configs/workspaces.

## Platform Notes

- Feishu/Lark channel has separate build tags for 32/64-bit (`feishu_*.go`)
- I2C/SPI tools are Linux-only
- USB device monitoring is Linux-only (`devices/sources/usb_linux.go`)
- Docker support via multi-stage Dockerfile (Alpine runtime)
