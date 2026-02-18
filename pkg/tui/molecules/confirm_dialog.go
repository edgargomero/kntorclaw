package molecules

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/sipeed/picoclaw/pkg/tui/atoms"
)

// ConfirmResultMsg is emitted when the user answers a confirmation dialog.
type ConfirmResultMsg struct{ Confirmed bool }

// ConfirmModel presents a yes/no confirmation dialog.
type ConfirmModel struct {
	Question string
	Visible  bool
	width    int
	height   int
}

// NewConfirm creates a ConfirmModel with the given question.
func NewConfirm(question string) ConfirmModel {
	return ConfirmModel{Question: question}
}

// Update handles y/n/Esc key presses.
func (m ConfirmModel) Update(msg tea.Msg) (ConfirmModel, tea.Cmd) {
	if !m.Visible {
		return m, nil
	}
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "y", "Y":
			m.Visible = false
			return m, func() tea.Msg { return ConfirmResultMsg{Confirmed: true} }
		case "n", "N", "esc":
			m.Visible = false
			return m, func() tea.Msg { return ConfirmResultMsg{Confirmed: false} }
		}
	}
	return m, nil
}

// View renders the confirmation dialog centered on screen.
func (m ConfirmModel) View() string {
	if !m.Visible {
		return ""
	}
	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(atoms.ColorYellow).
		Padding(1, 2)

	content := m.Question + "\n\n[y]es / [n]o"
	rendered := style.Render(content)

	if m.width > 0 && m.height > 0 {
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, rendered)
	}
	return rendered
}

// Show makes the dialog visible.
func (m *ConfirmModel) Show() { m.Visible = true }

// SetSize sets the screen dimensions for centering.
func (m *ConfirmModel) SetSize(w, h int) { m.width = w; m.height = h }
