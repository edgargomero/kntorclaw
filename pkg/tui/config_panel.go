package tui

import (
	"fmt"
	"sort"

	"github.com/rivo/tview"
	"github.com/sipeed/picoclaw/pkg/config"
)

func (a *App) buildConfigPanel() *tview.TextView {
	a.configView = newPanel("CONFIG [F8]")
	a.renderConfig()
	return a.configView
}

func (a *App) renderConfig() {
	cfg := a.config
	a.configView.Clear()

	fmt.Fprintf(a.configView, "[yellow]Provider:[white] %s\n", detectProvider(cfg))
	if a.modelRouter != nil {
		model, source := a.modelRouter.GetInfo("tui", "tui:local")
		fmt.Fprintf(a.configView, "[yellow]Model:[white] %s [gray](%s)[-]\n", model, source)
	} else {
		fmt.Fprintf(a.configView, "[yellow]Model:[white] %s\n", cfg.Agents.Defaults.Model)
	}
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

	fmt.Fprintf(a.configView, "\n[yellow::b][[]F8[-::-] open config editor\n")

	// Channel Models
	fmt.Fprintf(a.configView, "\n[blue::b]Channel Models[-::-]\n")
	if a.modelRouter != nil {
		channelModels := a.modelRouter.GetChannelModels()
		if len(channelModels) > 0 {
			keys := make([]string, 0, len(channelModels))
			for k := range channelModels {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, ch := range keys {
				fmt.Fprintf(a.configView, "  [green]%s[white] → %s\n", ch, channelModels[ch])
			}
		} else {
			fmt.Fprintf(a.configView, "  [gray](none)[-]\n")
		}
	}

	// Aliases
	fmt.Fprintf(a.configView, "\n[blue::b]Aliases[-::-]\n")
	if a.modelRouter != nil {
		aliases := a.modelRouter.GetAliases()
		if len(aliases) > 0 {
			keys := make([]string, 0, len(aliases))
			for k := range aliases {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, alias := range keys {
				fmt.Fprintf(a.configView, "  [green]%s[white] → %s\n", alias, aliases[alias])
			}
		} else {
			fmt.Fprintf(a.configView, "  [gray](none)[-]\n")
		}
	}

	// Providers
	fmt.Fprintf(a.configView, "\n[blue::b]Providers[-::-]\n")
	for _, p := range a.getProviderList() {
		if p.hasKey {
			fmt.Fprintf(a.configView, "  [green]%-15s[white] configured\n", p.name)
		} else {
			fmt.Fprintf(a.configView, "  [gray]%-15s[red] not set[-]\n", p.name)
		}
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
