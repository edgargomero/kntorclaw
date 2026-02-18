package molecules

import (
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

const maxLogLines = 1000

// LogViewerModel displays a scrollable log with a ring buffer of lines.
type LogViewerModel struct {
	viewport viewport.Model
	lines    []string
}

// NewLogViewer creates a LogViewerModel with the given dimensions.
func NewLogViewer(width, height int) LogViewerModel {
	vp := viewport.New(width, height)
	return LogViewerModel{viewport: vp}
}

// AddLine appends a line, trimming to maxLogLines, and auto-scrolls.
func (m *LogViewerModel) AddLine(line string) {
	m.lines = append(m.lines, line)
	if len(m.lines) > maxLogLines {
		m.lines = m.lines[len(m.lines)-maxLogLines:]
	}
	m.viewport.SetContent(strings.Join(m.lines, "\n"))
	m.viewport.GotoBottom()
}

// Update forwards messages to the underlying viewport.
func (m LogViewerModel) Update(msg tea.Msg) (LogViewerModel, tea.Cmd) {
	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

// View renders the viewport.
func (m LogViewerModel) View() string {
	return m.viewport.View()
}

// SetSize updates the viewport dimensions.
func (m *LogViewerModel) SetSize(w, h int) {
	m.viewport.Width = w
	m.viewport.Height = h
}
