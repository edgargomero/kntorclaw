package organisms

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/sipeed/picoclaw/pkg/tui/atoms"
)

// ConfigSummaryModel displays a read-only summary of config counts.
type ConfigSummaryModel struct {
	providers int
	models    int
	channels  int
	aliases   int
	width     int
	height    int
	focused   bool
}

// NewConfigSummary creates a ConfigSummaryModel with zero counts.
func NewConfigSummary() ConfigSummaryModel {
	return ConfigSummaryModel{}
}

// SetCounts updates the displayed config counts.
func (m *ConfigSummaryModel) SetCounts(providers, models, channels, aliases int) {
	m.providers = providers
	m.models = models
	m.channels = channels
	m.aliases = aliases
}

// Update is a no-op; the summary is driven by SetCounts.
func (m ConfigSummaryModel) Update(_ tea.Msg) (ConfigSummaryModel, tea.Cmd) {
	return m, nil
}

// View renders the config summary panel.
func (m ConfigSummaryModel) View() string {
	labelStyle := lipgloss.NewStyle().Foreground(atoms.ColorCyan)
	countStyle := lipgloss.NewStyle().Foreground(atoms.ColorYellow).Bold(true)

	var b strings.Builder
	b.WriteString(labelStyle.Render("Providers: ") + countStyle.Render(fmt.Sprintf("%d", m.providers)) + "\n")
	b.WriteString(labelStyle.Render("Models:    ") + countStyle.Render(fmt.Sprintf("%d", m.models)) + "\n")
	b.WriteString(labelStyle.Render("Channels:  ") + countStyle.Render(fmt.Sprintf("%d", m.channels)) + "\n")
	b.WriteString(labelStyle.Render("Aliases:   ") + countStyle.Render(fmt.Sprintf("%d", m.aliases)) + "\n")

	return atoms.RenderPanel("CONFIG", m.focused, m.width, m.height, b.String())
}

// SetSize updates the panel dimensions.
func (m *ConfigSummaryModel) SetSize(w, h int) { m.width = w; m.height = h }

// SetFocused sets the focused state for border styling.
func (m *ConfigSummaryModel) SetFocused(f bool) { m.focused = f }
