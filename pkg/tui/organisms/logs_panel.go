package organisms

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/sipeed/picoclaw/pkg/tui/atoms"
	"github.com/sipeed/picoclaw/pkg/tui/molecules"
)

// LogsPanelModel wraps a LogViewerModel in a styled panel, receiving log lines
// via LogLineMsg and rendering them in a scrollable viewport.
type LogsPanelModel struct {
	viewer  molecules.LogViewerModel
	width   int
	height  int
	focused bool
}

// NewLogsPanel creates a LogsPanelModel with initial dimensions.
func NewLogsPanel(width, height int) LogsPanelModel {
	return LogsPanelModel{
		viewer: molecules.NewLogViewer(width, height),
		width:  width,
		height: height,
	}
}

// SetSize updates the panel and inner viewer dimensions.
func (m *LogsPanelModel) SetSize(w, h int) {
	m.width = w
	m.height = h

	innerW, innerH := atoms.InnerSize(w, h)
	m.viewer.SetSize(innerW, innerH)
}

// SetFocused sets the focused state for border styling.
func (m *LogsPanelModel) SetFocused(focused bool) {
	m.focused = focused
}

// Update handles LogLineMsg to append lines and forwards viewport messages.
func (m LogsPanelModel) Update(msg tea.Msg) (LogsPanelModel, tea.Cmd) {
	switch msg := msg.(type) {
	case LogLineMsg:
		line := strings.TrimRight(msg.Line, "\n")
		if line != "" {
			m.viewer.AddLine(line)
		}
		return m, nil
	}

	var cmd tea.Cmd
	m.viewer, cmd = m.viewer.Update(msg)
	return m, cmd
}

// View renders the log viewer inside a styled panel border.
func (m LogsPanelModel) View() string {
	return atoms.RenderPanel("LOGS [F2]", m.focused, m.width, m.height, m.viewer.View())
}

// LogLineMsg carries a single log line into the TUI.
type LogLineMsg struct {
	Line string
}

// LogWriter implements io.Writer by sending each write as a LogLineMsg
// through a tea.Program. It can be used as the output for Go's log package
// or any other io.Writer consumer.
type LogWriter struct {
	program *tea.Program
}

// NewLogWriter creates a LogWriter that sends lines to the given tea.Program.
// The program may be nil initially and set later via SetProgram.
func NewLogWriter(p *tea.Program) *LogWriter {
	return &LogWriter{program: p}
}

// SetProgram sets or replaces the tea.Program used to send messages.
func (w *LogWriter) SetProgram(p *tea.Program) {
	w.program = p
}

// Write implements io.Writer. Each call sends a LogLineMsg to the program.
func (w *LogWriter) Write(p []byte) (n int, err error) {
	if w.program != nil {
		text := strings.TrimRight(string(p), "\n")
		if text != "" {
			w.program.Send(LogLineMsg{Line: text})
		}
	}
	return len(p), nil
}
