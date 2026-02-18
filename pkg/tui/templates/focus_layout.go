package templates

import (
	"github.com/charmbracelet/lipgloss"
)

// FocusLayoutPanels holds rendered panel strings for focus mode
type FocusLayoutPanels struct {
	StatusBar string
	Files     string
	Branches  string
	Commits   string
	QA        string
	Chat      string
	Diff      string
}

// RenderFocusLayout arranges panels in 30/70 split:
// Top: StatusBar (1 line) | Left (30%): Files, Branches, Commits, QA stacked | Right (70%): Chat, Diff stacked
func RenderFocusLayout(width, height int, panels FocusLayoutPanels) string {
	leftCol := lipgloss.JoinVertical(lipgloss.Left,
		panels.Files,
		panels.Branches,
		panels.Commits,
		panels.QA,
	)

	rightCol := lipgloss.JoinVertical(lipgloss.Left,
		panels.Chat,
		panels.Diff,
	)

	mainContent := lipgloss.JoinHorizontal(lipgloss.Top, leftCol, rightCol)

	if panels.StatusBar != "" {
		return lipgloss.JoinVertical(lipgloss.Left, panels.StatusBar, mainContent)
	}
	return mainContent
}
