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

## Relationship to OpenClaw

The sibling repo at `../openclaw` is the TypeScript/Swift/Kotlin multi-platform version. PicoClaw reimplements the same agent concept as a single Go binary for constrained environments. The `pkg/migrate` package handles migration from OpenClaw configs/workspaces.

## Platform Notes

- Feishu/Lark channel has separate build tags for 32/64-bit (`feishu_*.go`)
- I2C/SPI tools are Linux-only
- USB device monitoring is Linux-only (`devices/sources/usb_linux.go`)
- Docker support via multi-stage Dockerfile (Alpine runtime)
