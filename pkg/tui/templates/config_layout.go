package templates

import (
	"github.com/charmbracelet/lipgloss"
)

// ConfigLayoutPanels holds rendered panel strings for config mode
type ConfigLayoutPanels struct {
	StatusBar string
	Sections  string // This is the full nav view (sections + items side by side)
	Items     string // Unused — kept for API compat; nav renders items internally
	Footer    string
}

// RenderConfigLayout arranges: StatusBar on top, Nav content in middle, Footer at bottom
func RenderConfigLayout(width, height int, panels ConfigLayoutPanels) string {
	footerStyle := lipgloss.NewStyle().
		Width(width).
		Align(lipgloss.Center).
		Faint(true)

	parts := []string{}
	if panels.StatusBar != "" {
		parts = append(parts, panels.StatusBar)
	}
	parts = append(parts, panels.Sections)
	if panels.Footer != "" {
		parts = append(parts, footerStyle.Render(panels.Footer))
	}

	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}
