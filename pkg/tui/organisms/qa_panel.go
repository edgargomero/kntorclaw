package organisms

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/sipeed/picoclaw/pkg/tui/atoms"
	"github.com/sipeed/picoclaw/pkg/tui/domain"
)

// QAApproveMsg is sent when user approves a QA checkpoint.
type QAApproveMsg struct{}

// QARejectMsg is sent when user rejects a QA checkpoint.
type QARejectMsg struct{}

// QAPanelModel renders QATracker stages and checkpoints with pass/fail/pending icons.
type QAPanelModel struct {
	tracker *domain.QATracker
	width   int
	height  int
	focused bool
}

// NewQAPanel creates a QAPanelModel bound to the given QATracker.
func NewQAPanel(tracker *domain.QATracker) QAPanelModel {
	return QAPanelModel{tracker: tracker}
}

// Update handles Enter to approve and Esc to reject when focused.
func (m QAPanelModel) Update(msg tea.Msg) (QAPanelModel, tea.Cmd) {
	if !m.focused {
		return m, nil
	}
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			return m, func() tea.Msg { return QAApproveMsg{} }
		case "esc":
			return m, func() tea.Msg { return QARejectMsg{} }
		}
	}
	return m, nil
}

// View renders the QA pipeline stages with status icons and a progress bar.
func (m QAPanelModel) View() string {
	if m.tracker == nil {
		return atoms.RenderPanel("QA [F6]", m.focused, m.width, m.height, lipgloss.NewStyle().Faint(true).Render("No QA data"))
	}

	feature, stages, waitingApproval := m.tracker.GetStagesSnapshot()
	if len(stages) == 0 {
		return atoms.RenderPanel("QA [F6]", m.focused, m.width, m.height, lipgloss.NewStyle().Faint(true).Render("No stages"))
	}

	passStyle := lipgloss.NewStyle().Foreground(atoms.ColorGreen)
	failStyle := lipgloss.NewStyle().Foreground(atoms.ColorRed)
	pendingStyle := lipgloss.NewStyle().Foreground(atoms.ColorYellow)
	runningStyle := lipgloss.NewStyle().Foreground(atoms.ColorCyan)
	titleStyle := lipgloss.NewStyle().Foreground(atoms.ColorCyan).Bold(true)

	var b strings.Builder

	// Feature header
	if feature != "" {
		b.WriteString(titleStyle.Render(feature) + "\n\n")
	}

	for _, stage := range stages {
		icon := m.stageIcon(stage.Status, pendingStyle, runningStyle, passStyle, failStyle)
		line := fmt.Sprintf("%s %s", icon, stage.Name)
		if stage.Summary != "" {
			line += " " + lipgloss.NewStyle().Faint(true).Render("("+stage.Summary+")")
		}
		b.WriteString(line + "\n")

		// Render individual test results if present
		for _, r := range stage.Results {
			var rIcon string
			switch r.Status {
			case "pass":
				rIcon = passStyle.Render("  ✓")
			case "fail":
				rIcon = failStyle.Render("  ✗")
			case "skip":
				rIcon = pendingStyle.Render("  ○")
			default:
				rIcon = pendingStyle.Render("  ○")
			}
			testName := r.Test
			if r.Package != "" {
				testName = r.Package + "/" + r.Test
			}
			b.WriteString(fmt.Sprintf("%s %s\n", rIcon, testName))
		}
	}

	// Waiting approval hint
	if waitingApproval {
		b.WriteString("\n" + lipgloss.NewStyle().Foreground(atoms.ColorYellow).Bold(true).Render(
			"[Enter] Approve  [Esc] Reject"))
	}

	// Progress bar
	total, done := m.countProgress(stages)
	if total > 0 {
		pct := done * 100 / total
		barWidth := m.width - 12
		if barWidth < 10 {
			barWidth = 10
		}
		filled := barWidth * pct / 100
		if filled > barWidth {
			filled = barWidth
		}
		bar := passStyle.Render(strings.Repeat("█", filled)) +
			lipgloss.NewStyle().Faint(true).Render(strings.Repeat("░", barWidth-filled))
		b.WriteString(fmt.Sprintf("\n%s %d%%", bar, pct))
	}

	return atoms.RenderPanel("QA [F6]", m.focused, m.width, m.height, b.String())
}

func (m QAPanelModel) stageIcon(status domain.StageStatus, pending, running, pass, fail lipgloss.Style) string {
	switch status {
	case domain.StagePassed:
		return pass.Render("✓")
	case domain.StageFailed:
		return fail.Render("✗")
	case domain.StageRunning:
		return running.Render("◎")
	case domain.StageBlocked:
		return fail.Render("⊘")
	default:
		return pending.Render("○")
	}
}

func (m QAPanelModel) countProgress(stages []*domain.CheckpointStage) (total, done int) {
	for _, s := range stages {
		total++
		if s.Status == domain.StagePassed || s.Status == domain.StageFailed {
			done++
		}
	}
	return
}

// SetSize updates the panel dimensions.
func (m *QAPanelModel) SetSize(w, h int) { m.width = w; m.height = h }

// SetFocused sets whether this panel is focused.
func (m *QAPanelModel) SetFocused(f bool) { m.focused = f }
