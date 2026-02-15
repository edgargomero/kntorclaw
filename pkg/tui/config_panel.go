package tui

import (
	"fmt"

	"github.com/rivo/tview"
	"github.com/sipeed/picoclaw/pkg/config"
)

func (a *App) buildConfigPanel() *tview.TextView {
	a.configView = newPanel("CONFIG [F6]")
	a.renderConfig()
	return a.configView
}

func (a *App) renderConfig() {
	cfg := a.config
	a.configView.Clear()

	fmt.Fprintf(a.configView, "[yellow]Provider:[white] %s\n", detectProvider(cfg))
	fmt.Fprintf(a.configView, "[yellow]Model:[white] %s\n", cfg.Agents.Defaults.Model)
	fmt.Fprintf(a.configView, "[yellow]Workspace:[white] %s\n", cfg.WorkspacePath())
	fmt.Fprintf(a.configView, "[yellow]Max Tool Iterations:[white] %d\n", cfg.Agents.Defaults.MaxToolIterations)

	if cfg.Heartbeat.Enabled {
		fmt.Fprintf(a.configView, "[yellow]Heartbeat:[white] every %dm\n", cfg.Heartbeat.Interval)
	} else {
		fmt.Fprintf(a.configView, "[yellow]Heartbeat:[white] disabled\n")
	}

	if cfg.Devices.Enabled {
		fmt.Fprintf(a.configView, "[yellow]Devices:[white] enabled\n")
	}
}

func detectProvider(cfg *config.Config) string {
	switch {
	case cfg.Providers.Anthropic.APIKey != "" || cfg.Providers.Anthropic.AuthMethod != "":
		return "anthropic"
	case cfg.Providers.OpenAI.APIKey != "" || cfg.Providers.OpenAI.AuthMethod != "":
		return "openai"
	case cfg.Providers.OpenRouter.APIKey != "":
		return "openrouter"
	case cfg.Providers.Gemini.APIKey != "":
		return "gemini"
	case cfg.Providers.Zhipu.APIKey != "":
		return "zhipu"
	case cfg.Providers.Groq.APIKey != "":
		return "groq"
	case cfg.Providers.VLLM.APIBase != "":
		return "vllm"
	default:
		return "unknown"
	}
}
