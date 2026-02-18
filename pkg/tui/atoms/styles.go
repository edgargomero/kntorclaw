package atoms

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Colors
var (
	ColorGreen    = lipgloss.Color("#00ff00")
	ColorCyan     = lipgloss.Color("#00ffff")
	ColorYellow   = lipgloss.Color("#ffff00")
	ColorRed      = lipgloss.Color("#ff0000")
	ColorDarkCyan = lipgloss.Color("#008b8b")
	ColorWhite    = lipgloss.Color("#ffffff")
	ColorGray     = lipgloss.Color("#808080")
)

var HeaderStyle = lipgloss.NewStyle().
	Foreground(ColorYellow).
	Bold(true)

var StatusBarStyle = lipgloss.NewStyle().
	Background(lipgloss.Color("#333333")).
	Foreground(ColorWhite).
	Padding(0, 1)

// PanelStyle returns a lipgloss.Style for a bordered panel.
// width and height are OUTER dimensions (border included).
// NOTE: This does NOT render the title. Use RenderPanel for titled panels.
func PanelStyle(title string, focused bool, width, height int) lipgloss.Style {
	borderColor := ColorGray
	if focused {
		borderColor = ColorCyan
	}

	innerW := width - 2
	innerH := height - 2
	if innerW < 0 {
		innerW = 0
	}
	if innerH < 0 {
		innerH = 0
	}

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Width(innerW).
		Height(innerH)
}

// RenderPanel renders content inside a bordered panel with a title in the top border.
// width and height are OUTER dimensions (border included).
func RenderPanel(title string, focused bool, width, height int, content string) string {
	borderColor := ColorGray
	if focused {
		borderColor = ColorCyan
	}

	innerW := width - 2
	innerH := height - 2
	if innerW < 0 {
		innerW = 0
	}
	if innerH < 0 {
		innerH = 0
	}

	// Build the panel with border
	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Width(innerW).
		Height(innerH)

	rendered := style.Render(content)

	// Inject title into the top border line by replacing the dashes after ╭
	// We avoid building the entire line from separately-styled segments which
	// can cause ANSI escape sequence width mismatches. Instead, we simply
	// splice the title into the existing border line's rune sequence between
	// the left corner and the right corner, preserving the original ANSI
	// styling of the border line.
	if title != "" {
		lines := strings.Split(rendered, "\n")
		if len(lines) > 0 {
			origLine := lines[0]
			origVisualW := lipgloss.Width(origLine)

			// Build a styled title string
			titleStyle := lipgloss.NewStyle().Foreground(borderColor).Bold(true)
			titleText := titleStyle.Render(" " + title + " ")
			titleVisualW := lipgloss.Width(titleText)

			// Build the top border: ╭ + styled title + dashes + ╮
			// all in a single foreground style so there's no ANSI fragmentation
			bdrStyle := lipgloss.NewStyle().Foreground(borderColor)

			dashCount := origVisualW - 2 - titleVisualW // -2 for corners
			if dashCount < 0 {
				dashCount = 0
			}

			// Build as: border-colored ╭ + title (bold border-colored) + dashes + ╮
			// Use a single Render call for the corner+dashes portion to minimise
			// ANSI segment count.
			lines[0] = bdrStyle.Render("╭") +
				titleText +
				bdrStyle.Render(strings.Repeat("─", dashCount)+"╮")

			// Verify the visual width matches; if not, pad or trim dashes
			newVisualW := lipgloss.Width(lines[0])
			if newVisualW < origVisualW {
				// Pad with extra dashes before the closing corner
				extra := origVisualW - newVisualW
				lines[0] = bdrStyle.Render("╭") +
					titleText +
					bdrStyle.Render(strings.Repeat("─", dashCount+extra)+"╮")
			} else if newVisualW > origVisualW && dashCount > 0 {
				// Trim dashes
				excess := newVisualW - origVisualW
				trimmed := dashCount - excess
				if trimmed < 0 {
					trimmed = 0
				}
				lines[0] = bdrStyle.Render("╭") +
					titleText +
					bdrStyle.Render(strings.Repeat("─", trimmed)+"╮")
			}

			rendered = strings.Join(lines, "\n")
		}
	}

	return rendered
}

// InnerSize calculates the content area dimensions for a panel with the given outer size.
func InnerSize(outerW, outerH int) (int, int) {
	w := outerW - 2
	h := outerH - 2
	if w < 0 {
		w = 0
	}
	if h < 0 {
		h = 0
	}
	return w, h
}
