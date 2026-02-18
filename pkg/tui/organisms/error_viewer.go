package organisms

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/sipeed/picoclaw/pkg/tui/atoms"
	"github.com/sipeed/picoclaw/pkg/tui/domain"
)

// ErrorViewerModel is a modal showing recent errors from ErrorTracker.
type ErrorViewerModel struct {
	tracker *domain.ErrorTracker
	Visible bool
	scroll  int
	width   int
	height  int
}

// NewErrorViewer creates an ErrorViewerModel bound to the given ErrorTracker.
func NewErrorViewer(tracker *domain.ErrorTracker) ErrorViewerModel {
	return ErrorViewerModel{tracker: tracker}
}

// Show makes the error viewer visible, resets scroll, and clears the error count badge.
func (m *ErrorViewerModel) Show() {
	m.Visible = true
	m.scroll = 0
	if m.tracker != nil {
		m.tracker.Clear()
	}
}

// Hide closes the error viewer.
func (m *ErrorViewerModel) Hide() { m.Visible = false }

// Update handles key events for the error viewer.
func (m ErrorViewerModel) Update(msg tea.Msg) (ErrorViewerModel, tea.Cmd) {
	if !m.Visible {
		return m, nil
	}
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "ctrl+e":
			m.Visible = false
		case "up", "k":
			if m.scroll > 0 {
				m.scroll--
			}
		case "down", "j":
			m.scroll++
		}
	}
	return m, nil
}

// View renders the error viewer as a centered modal overlay.
func (m ErrorViewerModel) View(screenWidth, screenHeight int) string {
	if !m.Visible {
		return ""
	}

	titleStyle := lipgloss.NewStyle().Foreground(atoms.ColorRed).Bold(true)
	timeStyle := lipgloss.NewStyle().Faint(true)
	errStyle := lipgloss.NewStyle().Foreground(atoms.ColorRed)

	var b strings.Builder
	b.WriteString(titleStyle.Render("Error Log") + "\n\n")

	// Get recent errors from tracker
	errors := m.tracker.Recent(50)
	if len(errors) == 0 {
		b.WriteString(lipgloss.NewStyle().Faint(true).Render("No errors recorded"))
	} else {
		start := m.scroll
		if start >= len(errors) {
			start = len(errors) - 1
		}
		end := start + screenHeight - 10
		if end > len(errors) {
			end = len(errors)
		}

		for i := start; i < end; i++ {
			e := errors[i]
			b.WriteString(fmt.Sprintf("%s %s\n",
				timeStyle.Render(e.Timestamp),
				errStyle.Render(e.Message),
			))
		}
	}

	b.WriteString("\n" + lipgloss.NewStyle().Faint(true).Render("Esc/Ctrl+E: close | up/down: scroll"))

	modalW := screenWidth * 2 / 3
	if modalW < 40 {
		modalW = 40
	}
	modalH := screenHeight * 2 / 3
	if modalH < 10 {
		modalH = 10
	}

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(atoms.ColorRed).
		Padding(1, 2).
		Width(modalW).
		Height(modalH)

	rendered := boxStyle.Render(b.String())
	return placeOverlay(screenWidth, screenHeight, rendered)
}
