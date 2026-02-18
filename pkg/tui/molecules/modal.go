package molecules

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/sipeed/picoclaw/pkg/tui/atoms"
)

// CloseModalMsg is emitted when the user presses Esc inside a modal.
type CloseModalMsg struct{}

// ModalModel provides a centered overlay with title and content.
type ModalModel struct {
	Title   string
	Content string
	Width   int
	Height  int
	Visible bool
}

// NewModal creates a ModalModel with the given title and dimensions.
func NewModal(title string, width, height int) ModalModel {
	return ModalModel{
		Title:  title,
		Width:  width,
		Height: height,
	}
}

// Update handles Esc to close the modal.
func (m ModalModel) Update(msg tea.Msg) (ModalModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "esc" {
			m.Visible = false
			return m, func() tea.Msg { return CloseModalMsg{} }
		}
	}
	return m, nil
}

// View renders the modal centered on the given screen dimensions.
func (m ModalModel) View(screenWidth, screenHeight int) string {
	if !m.Visible {
		return ""
	}

	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(atoms.ColorCyan).
		Padding(1, 2).
		Width(m.Width).
		Height(m.Height)

	titleStyle := lipgloss.NewStyle().
		Foreground(atoms.ColorYellow).
		Bold(true).
		Align(lipgloss.Center).
		Width(m.Width - 4)

	modal := titleStyle.Render(m.Title) + "\n\n" + m.Content
	rendered := style.Render(modal)

	return placeCenter(screenWidth, screenHeight, rendered)
}

func placeCenter(width, height int, content string) string {
	lines := strings.Split(content, "\n")
	contentHeight := len(lines)
	contentWidth := 0
	for _, l := range lines {
		if len(l) > contentWidth {
			contentWidth = len(l)
		}
	}

	padTop := (height - contentHeight) / 2
	padLeft := (width - contentWidth) / 2
	if padTop < 0 {
		padTop = 0
	}
	if padLeft < 0 {
		padLeft = 0
	}

	var b strings.Builder
	for i := 0; i < padTop; i++ {
		b.WriteString("\n")
	}
	prefix := strings.Repeat(" ", padLeft)
	for _, l := range lines {
		b.WriteString(prefix + l + "\n")
	}
	return b.String()
}
