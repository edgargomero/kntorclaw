package organisms

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/sipeed/picoclaw/pkg/tui/atoms"
	"github.com/sipeed/picoclaw/pkg/tui/domain"
)

// TickFocusMsg triggers a refresh of git data in the focus panel.
type TickFocusMsg struct{}

// FocusPanelModel displays git status, branches, commits, and a diff viewport.
type FocusPanelModel struct {
	files    []domain.FileEntry
	branches []domain.BranchEntry
	commits  []domain.CommitEntry
	diff     viewport.Model

	fileCursor   int
	branchCursor int
	commitCursor int

	focusedSub int // 0=files, 1=branches, 2=commits, 3=diff
	width      int
	height     int
	focused    bool
	gitError   string // non-empty if git commands fail (e.g., not a git repo)
	loaded     bool   // true after first git data refresh
}

// NewFocusPanel creates a FocusPanelModel.
func NewFocusPanel() FocusPanelModel {
	return FocusPanelModel{
		diff: viewport.New(40, 10),
	}
}

// Update handles keyboard navigation and tick refreshes.
func (m FocusPanelModel) Update(msg tea.Msg) (FocusPanelModel, tea.Cmd) {
	switch msg := msg.(type) {
	case TickFocusMsg:
		m.refreshGitData()
		return m, nil
	case tea.KeyMsg:
		if !m.focused {
			return m, nil
		}
		switch msg.String() {
		case "up", "k":
			m.moveCursor(-1)
		case "down", "j":
			m.moveCursor(1)
		case "enter":
			// If on files or commits sub-panel, update the diff view
			if m.focusedSub == 0 || m.focusedSub == 2 {
				m.updateDiff()
			}
		}
	}

	if m.focusedSub == 3 {
		var cmd tea.Cmd
		m.diff, cmd = m.diff.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m *FocusPanelModel) refreshGitData() {
	// Check if we're in a git repository first
	if err := domain.GitCmd("rev-parse", "--git-dir").Run(); err != nil {
		m.gitError = "Not a git repository"
		m.files = nil
		m.branches = nil
		m.commits = nil
		m.diff.SetContent("")
		return
	}
	m.gitError = ""
	m.loaded = true

	m.files = domain.GitStatus()
	m.branches = domain.GitBranches()
	m.commits = domain.GitLog(20)

	// Clamp cursors after refresh
	m.fileCursor = clampVal(m.fileCursor, 0, max(0, len(m.files)-1))
	m.branchCursor = clampVal(m.branchCursor, 0, max(0, len(m.branches)-1))
	m.commitCursor = clampVal(m.commitCursor, 0, max(0, len(m.commits)-1))

	m.updateDiff()
}

// EnsureLoaded triggers an initial git data refresh if not yet loaded.
func (m *FocusPanelModel) EnsureLoaded() {
	if !m.loaded {
		m.refreshGitData()
	}
}

// ForceRefresh forces a git data refresh regardless of loaded state.
func (m *FocusPanelModel) ForceRefresh() {
	m.refreshGitData()
}

func (m *FocusPanelModel) updateDiff() {
	// When commits panel is focused, show commit diff
	if m.focusedSub == 2 && len(m.commits) > 0 && m.commitCursor < len(m.commits) {
		diffText := domain.GitShowCommit(m.commits[m.commitCursor].Hash)
		m.diff.SetContent(m.formatDiff(diffText))
		m.diff.GotoTop()
	} else if len(m.files) > 0 && m.fileCursor < len(m.files) {
		// Default: show file diff (for files panel or initial load)
		diffText := domain.GitDiff(m.files[m.fileCursor].Path)
		m.diff.SetContent(m.formatDiff(diffText))
		m.diff.GotoTop()
	} else if len(m.commits) > 0 && m.commitCursor < len(m.commits) {
		// Fallback: show commit diff when no files
		diffText := domain.GitShowCommit(m.commits[m.commitCursor].Hash)
		m.diff.SetContent(m.formatDiff(diffText))
		m.diff.GotoTop()
	} else {
		m.diff.SetContent("")
	}
}

func (m *FocusPanelModel) moveCursor(delta int) {
	switch m.focusedSub {
	case 0:
		m.fileCursor = clampVal(m.fileCursor+delta, 0, max(0, len(m.files)-1))
		m.updateDiff()
	case 1:
		m.branchCursor = clampVal(m.branchCursor+delta, 0, max(0, len(m.branches)-1))
	case 2:
		m.commitCursor = clampVal(m.commitCursor+delta, 0, max(0, len(m.commits)-1))
		// Update diff when navigating commits
		m.updateDiff()
	case 3:
		// Diff viewport handles its own scrolling
	}
}

func (m FocusPanelModel) formatDiff(diff string) string {
	addStyle := lipgloss.NewStyle().Foreground(atoms.ColorGreen)
	delStyle := lipgloss.NewStyle().Foreground(atoms.ColorRed)
	headerStyle := lipgloss.NewStyle().Foreground(atoms.ColorCyan).Bold(true)

	var b strings.Builder
	for _, line := range strings.Split(diff, "\n") {
		switch {
		case strings.HasPrefix(line, "+"):
			b.WriteString(addStyle.Render(line))
		case strings.HasPrefix(line, "-"):
			b.WriteString(delStyle.Render(line))
		case strings.HasPrefix(line, "@@"):
			b.WriteString(headerStyle.Render(line))
		default:
			b.WriteString(line)
		}
		b.WriteString("\n")
	}
	return b.String()
}

// truncLine truncates a string to maxW visible characters, adding ellipsis if needed.
func truncLine(s string, maxW int) string {
	if maxW <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= maxW {
		return s
	}
	runes := []rune(s)
	for i := len(runes); i > 0; i-- {
		candidate := string(runes[:i]) + "…"
		if lipgloss.Width(candidate) <= maxW {
			return candidate
		}
	}
	return "…"
}

// SetFocusedSub sets which sub-panel (0=files, 1=branches, 2=commits, 3=diff) is active.
func (m *FocusPanelModel) SetFocusedSub(sub int) {
	if sub >= 0 && sub <= 3 {
		m.focusedSub = sub
	}
}

// GetFocusedSub returns the currently focused sub-panel index.
func (m FocusPanelModel) GetFocusedSub() int {
	return m.focusedSub
}

// RenderFiles renders only the files list sub-panel.
func (m FocusPanelModel) RenderFiles(width, height int) string {
	if m.gitError != "" {
		return atoms.RenderPanel("FILES [F1]", m.focusedSub == 0 && m.focused, width, height,
			lipgloss.NewStyle().Faint(true).Render("  "+m.gitError))
	}
	return m.renderFileList("FILES [F1]", m.files, m.fileCursor, m.focusedSub == 0, width, height)
}

// RenderBranches renders only the branches list sub-panel.
func (m FocusPanelModel) RenderBranches(width, height int) string {
	if m.gitError != "" {
		return atoms.RenderPanel("BRANCHES [F2]", m.focusedSub == 1 && m.focused, width, height,
			lipgloss.NewStyle().Faint(true).Render("  "+m.gitError))
	}
	return m.renderBranchList("BRANCHES [F2]", m.branches, m.branchCursor, m.focusedSub == 1, width, height)
}

// RenderCommits renders only the commits list sub-panel.
func (m FocusPanelModel) RenderCommits(width, height int) string {
	if m.gitError != "" {
		return atoms.RenderPanel("COMMITS [F3]", m.focusedSub == 2 && m.focused, width, height,
			lipgloss.NewStyle().Faint(true).Render("  "+m.gitError))
	}
	return m.renderCommitList("COMMITS [F3]", m.commits, m.commitCursor, m.focusedSub == 2, width, height)
}

// RenderDiff renders only the diff viewport sub-panel.
func (m FocusPanelModel) RenderDiff(width, height int) string {
	// Resize diff viewport to match requested dimensions
	innerW, innerH := atoms.InnerSize(width, height)
	m.diff.Width = innerW
	m.diff.Height = innerH
	if m.gitError != "" {
		return atoms.RenderPanel("DIFF [F5]", m.focusedSub == 3 && m.focused, width, height,
			lipgloss.NewStyle().Faint(true).Render("  "+m.gitError))
	}
	return atoms.RenderPanel("DIFF [F5]", m.focusedSub == 3 && m.focused, width, height, m.diff.View())
}

// View renders the focus panel with files, branches, commits, and diff sub-panels.
func (m FocusPanelModel) View() string {
	listHeight := m.height / 6
	if listHeight < 3 {
		listHeight = 3
	}

	filesView := m.renderFileList("FILES [F1]", m.files, m.fileCursor, m.focusedSub == 0, m.width, listHeight)
	branchesView := m.renderBranchList("BRANCHES [F2]", m.branches, m.branchCursor, m.focusedSub == 1, m.width, listHeight)
	commitsView := m.renderCommitList("COMMITS [F3]", m.commits, m.commitCursor, m.focusedSub == 2, m.width, listHeight)

	diffHeight := m.height - 3*listHeight
	if diffHeight < 3 {
		diffHeight = 3
	}
	diffView := atoms.RenderPanel("DIFF [F5]", m.focusedSub == 3 && m.focused, m.width, diffHeight, m.diff.View())

	return lipgloss.JoinVertical(lipgloss.Left, filesView, branchesView, commitsView, diffView)
}

func (m FocusPanelModel) renderFileList(title string, items []domain.FileEntry, cursor int, subFocused bool, w, h int) string {
	maxLineW := w - 4 // inner width minus prefix
	var b strings.Builder
	visibleLines := h - 2
	if visibleLines < 1 {
		visibleLines = 1
	}
	// Scroll offset: keep cursor visible
	offset := 0
	if cursor >= visibleLines {
		offset = cursor - visibleLines + 1
	}
	for i := offset; i < len(items) && i-offset < visibleLines; i++ {
		item := items[i]
		display := fmt.Sprintf("[%s] %s", item.Status, item.Path)
		display = truncLine(display, maxLineW-2) // -2 for prefix
		prefix := "  "
		if i == cursor {
			prefix = "> "
			display = lipgloss.NewStyle().Foreground(atoms.ColorCyan).Render(display)
		} else {
			// Color by status (matching tview: M=yellow, A=green, D=red, ?=gray)
			var color lipgloss.Color
			switch {
			case strings.Contains(item.Status, "M"):
				color = atoms.ColorYellow
			case strings.Contains(item.Status, "A"):
				color = atoms.ColorGreen
			case strings.Contains(item.Status, "D"):
				color = atoms.ColorRed
			case strings.Contains(item.Status, "?"):
				color = atoms.ColorGray
			default:
				color = atoms.ColorWhite
			}
			display = lipgloss.NewStyle().Foreground(color).Render(display)
		}
		b.WriteString(prefix + display + "\n")
	}
	if len(items) == 0 {
		b.WriteString(lipgloss.NewStyle().Faint(true).Render("  (no changes)"))
	}
	return atoms.RenderPanel(title, subFocused && m.focused, w, h, b.String())
}

func (m FocusPanelModel) renderBranchList(title string, items []domain.BranchEntry, cursor int, subFocused bool, w, h int) string {
	maxLineW := w - 4
	var b strings.Builder
	visibleLines := h - 2
	if visibleLines < 1 {
		visibleLines = 1
	}
	offset := 0
	if cursor >= visibleLines {
		offset = cursor - visibleLines + 1
	}
	for i := offset; i < len(items) && i-offset < visibleLines; i++ {
		item := items[i]
		display := item.Name
		if item.IsCurrent {
			display = "* " + display
		}
		display = truncLine(display, maxLineW-2)
		prefix := "  "
		if i == cursor {
			prefix = "> "
			display = lipgloss.NewStyle().Foreground(atoms.ColorCyan).Render(display)
		} else if item.IsCurrent {
			// Highlight current branch in green (matching tview behavior)
			display = lipgloss.NewStyle().Foreground(atoms.ColorGreen).Render(display)
		}
		b.WriteString(prefix + display + "\n")
	}
	if len(items) == 0 {
		b.WriteString(lipgloss.NewStyle().Faint(true).Render("  (no branches)"))
	}
	return atoms.RenderPanel(title, subFocused && m.focused, w, h, b.String())
}

func (m FocusPanelModel) renderCommitList(title string, items []domain.CommitEntry, cursor int, subFocused bool, w, h int) string {
	maxLineW := w - 4
	var b strings.Builder
	visibleLines := h - 2
	if visibleLines < 1 {
		visibleLines = 1
	}
	offset := 0
	if cursor >= visibleLines {
		offset = cursor - visibleLines + 1
	}
	for i := offset; i < len(items) && i-offset < visibleLines; i++ {
		item := items[i]
		display := fmt.Sprintf("%s %s", item.Hash, item.Message)
		display = truncLine(display, maxLineW-2)
		prefix := "  "
		if i == cursor {
			prefix = "> "
			display = lipgloss.NewStyle().Foreground(atoms.ColorCyan).Render(display)
		}
		b.WriteString(prefix + display + "\n")
	}
	if len(items) == 0 {
		b.WriteString(lipgloss.NewStyle().Faint(true).Render("  (no commits)"))
	}
	return atoms.RenderPanel(title, subFocused && m.focused, w, h, b.String())
}

// SetSize updates panel dimensions and the diff viewport.
func (m *FocusPanelModel) SetSize(w, h int) {
	m.width = w
	m.height = h
	m.diff.Width = max(w-4, 0)
	listHeight := h / 6
	if listHeight < 3 {
		listHeight = 3
	}
	m.diff.Height = max(h-3*listHeight-4, 1)
}

// SetFocused sets whether this panel is focused.
func (m *FocusPanelModel) SetFocused(f bool) { m.focused = f }

func clampVal(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
