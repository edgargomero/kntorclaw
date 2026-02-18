package templates

import (
	"github.com/charmbracelet/lipgloss"
)

// NormalLayoutPanels holds rendered panel strings for normal mode
type NormalLayoutPanels struct {
	StatusBar string
	Chat      string
	Channels  string
	Tokens    string
	Sessions  string
	Logs      string
}

// RenderNormalLayout arranges panels in 50/50 split:
// Top: StatusBar (1 line) | Left: Chat+Input | Right: Channels, Tokens, Sessions, Logs stacked
func RenderNormalLayout(width, height int, panels NormalLayoutPanels) string {
	rightCol := lipgloss.JoinVertical(lipgloss.Left,
		panels.Channels,
		panels.Tokens,
		panels.Sessions,
		panels.Logs,
	)

	mainContent := lipgloss.JoinHorizontal(lipgloss.Top, panels.Chat, rightCol)

	if panels.StatusBar != "" {
		return lipgloss.JoinVertical(lipgloss.Left, panels.StatusBar, mainContent)
	}
	return mainContent
}
