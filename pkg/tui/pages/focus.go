package pages

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/sipeed/picoclaw/pkg/tui/atoms"
	"github.com/sipeed/picoclaw/pkg/tui/domain"
	"github.com/sipeed/picoclaw/pkg/tui/organisms"
	"github.com/sipeed/picoclaw/pkg/tui/templates"
)

const focusPanelCount = 6

// FocusPageModel composes focus-mode panels: FocusPanel (files/branches/commits/diff),
// QAPanel, and ChatPanel, arranged using the 30/70 focus layout.
type FocusPageModel struct {
	focus organisms.FocusPanelModel
	qa    organisms.QAPanelModel
	chat  organisms.ChatPanelModel

	errorTracker *domain.ErrorTracker
	qaTracker    *domain.QATracker
	focusedPanel int // 0=files, 1=branches, 2=commits, 3=chat, 4=diff, 5=qa
	width        int
	height       int
}

// SetErrorTracker sets the error tracker for status bar badge rendering.
func (m *FocusPageModel) SetErrorTracker(et *domain.ErrorTracker) {
	m.errorTracker = et
}

// SetQATracker sets the QA tracker for approve/reject handling.
func (m *FocusPageModel) SetQATracker(qt *domain.QATracker) {
	m.qaTracker = qt
}

// IsChatFocused returns true when the chat panel is currently focused.
func (m FocusPageModel) IsChatFocused() bool {
	return m.focusedPanel == 3
}

// LoadGitData triggers an immediate git data refresh (called on mode entry).
func (m *FocusPageModel) LoadGitData() {
	m.focus.EnsureLoaded()
	// Force reload even if already loaded (entering focus mode should refresh)
	m.focus.ForceRefresh()
}

// renderQA renders the QA panel, updating its size before rendering.
func (m FocusPageModel) renderQA(width, height int) string {
	m.qa.SetSize(width, height)
	return m.qa.View()
}

// NewFocusPage creates a FocusPageModel with the given dimensions.
func NewFocusPage(width, height int) FocusPageModel {
	leftW := width * 30 / 100
	rightW := width - leftW

	focus := organisms.NewFocusPanel()
	focus.SetSize(leftW, height)
	focus.SetFocused(true)
	focus.SetFocusedSub(0) // Start on files

	qa := organisms.NewQAPanel(nil)
	qa.SetSize(leftW, height/4)

	chat := organisms.NewChatPanel(rightW, height/2)
	chat.SetTitleHint("[F4]")

	return FocusPageModel{
		focus:  focus,
		qa:     qa,
		chat:   chat,
		width:  width,
		height: height,
	}
}

// Init returns the init command for sub-components.
func (m FocusPageModel) Init() tea.Cmd {
	return m.chat.Init()
}

// Update handles key messages for focus cycling and forwards to organisms.
func (m FocusPageModel) Update(msg tea.Msg) (FocusPageModel, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "f1":
			m.focusedPanel = 0
			m.syncFocus()
			return m, nil
		case "f2":
			m.focusedPanel = 1
			m.syncFocus()
			return m, nil
		case "f3":
			m.focusedPanel = 2
			m.syncFocus()
			return m, nil
		case "f4":
			m.focusedPanel = 3
			m.syncFocus()
			return m, nil
		case "f5":
			m.focusedPanel = 4
			m.syncFocus()
			return m, nil
		case "f6":
			m.focusedPanel = 5
			m.syncFocus()
			return m, nil
		case "tab":
			m.focusedPanel = (m.focusedPanel + 1) % focusPanelCount
			m.syncFocus()
			return m, nil
		case "shift+tab":
			m.focusedPanel = (m.focusedPanel - 1 + focusPanelCount) % focusPanelCount
			m.syncFocus()
			return m, nil
		case "esc":
			// If QA panel is focused and waiting approval, reject (handled by qa panel)
			// Otherwise, switch focus to chat input (matching tview behavior)
			if m.focusedPanel == 5 && m.qaTracker != nil && m.qaTracker.WaitingApproval {
				// Let QA panel handle reject
				break
			}
			m.focusedPanel = 3 // chat
			m.syncFocus()
			return m, nil
		}
	}

	// Handle QA approve/reject messages
	switch msg.(type) {
	case organisms.QAApproveMsg:
		if m.qaTracker != nil {
			m.qaTracker.Approve()
		}
		return m, nil
	case organisms.QARejectMsg:
		if m.qaTracker != nil {
			m.qaTracker.Reject()
		}
		return m, nil
	}

	// Ensure git data is loaded on first tick
	switch msg.(type) {
	case organisms.TickFocusMsg:
		m.focus.EnsureLoaded()
	}

	// Forward to all organisms
	var cmd tea.Cmd
	m.focus, cmd = m.focus.Update(msg)
	cmds = append(cmds, cmd)
	m.qa, cmd = m.qa.Update(msg)
	cmds = append(cmds, cmd)
	m.chat, cmd = m.chat.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

// syncFocus propagates page-level panel focus to sub-components.
// Maps page panels (0-5) to focus_panel sub-panels (0-3).
func (m *FocusPageModel) syncFocus() {
	// Map: 0=files→0, 1=branches→1, 2=commits→2, 3=chat, 4=diff→3, 5=qa
	isFocusPanelFocused := m.focusedPanel <= 2 || m.focusedPanel == 4
	m.focus.SetFocused(isFocusPanelFocused)
	m.chat.SetFocused(m.focusedPanel == 3)
	m.qa.SetFocused(m.focusedPanel == 5)

	// Sync sub-panel focus within FocusPanelModel
	switch m.focusedPanel {
	case 0:
		m.focus.SetFocusedSub(0) // files
	case 1:
		m.focus.SetFocusedSub(1) // branches
	case 2:
		m.focus.SetFocusedSub(2) // commits
	case 4:
		m.focus.SetFocusedSub(3) // diff
	}
}

// View renders the focus layout with all panels using individual sub-renderers.
func (m FocusPageModel) View() string {
	leftW := m.width * 30 / 100
	rightW := m.width - leftW
	contentH := m.height - 1 // minus status bar

	fileH := contentH / 4
	branchH := contentH / 4
	commitH := contentH / 4
	qaH := contentH - fileH - branchH - commitH

	chatH := contentH * 40 / 100
	diffH := contentH - chatH

	// Resize chat panel to match layout dimensions before rendering
	m.chat.SetSize(rightW, chatH)

	return templates.RenderFocusLayout(m.width, m.height, templates.FocusLayoutPanels{
		StatusBar: m.renderStatusBar(),
		Files:     m.focus.RenderFiles(leftW, fileH),
		Branches:  m.focus.RenderBranches(leftW, branchH),
		Commits:   m.focus.RenderCommits(leftW, commitH),
		QA:        m.renderQA(leftW, qaH),
		Chat:      m.chat.View(),
		Diff:      m.focus.RenderDiff(rightW, diffH),
	})
}

func (m FocusPageModel) renderStatusBar() string {
	modeStyle := lipgloss.NewStyle().Foreground(atoms.ColorCyan).Bold(true)
	hintStyle := lipgloss.NewStyle().Faint(true)

	parts := []string{modeStyle.Render("[FOCUS]")}

	if m.errorTracker != nil {
		if badge := atoms.RenderBadge(m.errorTracker.Count()); badge != "" {
			parts = append(parts, badge)
		}
	}

	parts = append(parts, hintStyle.Render("F1-F6:Panels Tab:Next j/k:Nav F9:Exit"))

	content := strings.Join(parts, " ")
	return atoms.StatusBarStyle.Width(m.width).Render(content)
}

// SetSize updates the page and all panel dimensions.
func (m *FocusPageModel) SetSize(w, h int) {
	m.width = w
	m.height = h
	contentH := h - 1
	leftW := w * 30 / 100
	rightW := w - leftW

	m.focus.SetSize(leftW, contentH)

	qaH := contentH - 3*(contentH/4)
	m.qa.SetSize(leftW, qaH)

	chatH := contentH * 40 / 100
	m.chat.SetSize(rightW, chatH)
}
