package atoms

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

var badgeStyle = lipgloss.NewStyle().
	Foreground(ColorWhite).
	Background(ColorRed).
	Padding(0, 1).
	Bold(true)

// RenderBadge returns a styled error count badge
func RenderBadge(count int) string {
	if count == 0 {
		return ""
	}
	return badgeStyle.Render(fmt.Sprintf("%d errors", count))
}
