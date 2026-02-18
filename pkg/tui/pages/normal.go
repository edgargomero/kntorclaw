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

const normalPanelCount = 5

// NormalPageModel assembles chat + logs + channels + tokens + sessions
// with focus cycling via F1-F5 and Tab/Shift+Tab.
// Panel indices match tview: 0=chat(F1), 1=logs(F2), 2=channels(F3), 3=tokens(F4), 4=sessions(F5).
type NormalPageModel struct {
	chat     organisms.ChatPanelModel
	channels organisms.ChannelsPanelModel
	tokens   organisms.TokensPanelModel
	sessions organisms.SessionsPanelModel
	logs     organisms.LogsPanelModel

	errorTracker *domain.ErrorTracker
	focusedPanel int // 0=chat, 1=logs, 2=channels, 3=tokens, 4=sessions
	width        int
	height       int
}

// SetErrorTracker sets the error tracker for status bar badge rendering.
func (m *NormalPageModel) SetErrorTracker(et *domain.ErrorTracker) {
	m.errorTracker = et
}

// SetWorkspacePath propagates the workspace path to the sessions panel for disk merge.
func (m *NormalPageModel) SetWorkspacePath(path string) {
	m.sessions.SetWorkspacePath(path)
}

// SetUsagePath propagates the token usage path to the tokens panel for historical totals.
func (m *NormalPageModel) SetUsagePath(path string) {
	m.tokens.SetUsagePath(path)
}

// NewNormalPage creates a NormalPageModel with the given dimensions.
func NewNormalPage(width, height int, tracker *domain.ActivityTracker) NormalPageModel {
	leftW := width / 2
	rightW := width - leftW
	contentH := height - 1 // status bar takes 1 row
	panelH := contentH / 4

	m := NormalPageModel{
		chat:     organisms.NewChatPanel(leftW, contentH),
		channels: organisms.NewChannelsPanel(),
		tokens:   organisms.NewTokensPanel(tracker),
		sessions: organisms.NewSessionsPanel(tracker),
		logs:     organisms.NewLogsPanel(rightW, contentH-3*panelH),
		width:    width,
		height:   height,
	}
	// Set sizes for panels that don't take dimensions in constructor
	m.channels.SetSize(rightW, panelH)
	m.tokens.SetSize(rightW, panelH)
	m.sessions.SetSize(rightW, panelH)
	m.updateFocus()
	return m
}

// Init returns the init command for sub-components.
func (m NormalPageModel) Init() tea.Cmd {
	return m.chat.Init()
}

// Update handles key messages for focus cycling and forwards to all organisms.
func (m NormalPageModel) Update(msg tea.Msg) (NormalPageModel, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "f1":
			m.focusedPanel = 0 // Chat
			m.updateFocus()
			return m, nil
		case "f2":
			m.focusedPanel = 1 // Logs
			m.updateFocus()
			return m, nil
		case "f3":
			m.focusedPanel = 2 // Channels
			m.updateFocus()
			return m, nil
		case "f4":
			m.focusedPanel = 3 // Tokens
			m.updateFocus()
			return m, nil
		case "f5":
			m.focusedPanel = 4 // Sessions
			m.updateFocus()
			return m, nil
		case "tab":
			m.focusedPanel = (m.focusedPanel + 1) % normalPanelCount
			m.updateFocus()
			return m, nil
		case "shift+tab":
			m.focusedPanel = (m.focusedPanel - 1 + normalPanelCount) % normalPanelCount
			m.updateFocus()
			return m, nil
		case "esc":
			// Return focus to chat input (matches tview behavior)
			m.focusedPanel = 0
			m.updateFocus()
			return m, nil
		}
	}

	// Forward to all organisms
	var cmd tea.Cmd
	m.chat, cmd = m.chat.Update(msg)
	cmds = append(cmds, cmd)
	m.channels, cmd = m.channels.Update(msg)
	cmds = append(cmds, cmd)
	m.tokens, cmd = m.tokens.Update(msg)
	cmds = append(cmds, cmd)
	m.sessions, cmd = m.sessions.Update(msg)
	cmds = append(cmds, cmd)
	m.logs, cmd = m.logs.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

// View renders the normal layout with all panels.
func (m NormalPageModel) View() string {
	return templates.RenderNormalLayout(m.width, m.height, templates.NormalLayoutPanels{
		StatusBar: m.renderStatusBar(),
		Chat:      m.chat.View(),
		Channels:  m.channels.View(),
		Tokens:    m.tokens.View(),
		Sessions:  m.sessions.View(),
		Logs:      m.logs.View(),
	})
}

func (m NormalPageModel) renderStatusBar() string {
	keyStyle := lipgloss.NewStyle().Foreground(atoms.ColorYellow)
	labelStyle := lipgloss.NewStyle().Foreground(atoms.ColorWhite)
	hintStyle := lipgloss.NewStyle().Faint(true)

	var parts []string

	if m.errorTracker != nil {
		if badge := atoms.RenderBadge(m.errorTracker.Count()); badge != "" {
			parts = append(parts, badge)
		}
	}

	// F1-F5 panel hints matching tview status bar
	parts = append(parts,
		keyStyle.Render("F1")+labelStyle.Render("-Chat"),
		keyStyle.Render("F2")+labelStyle.Render("-Logs"),
		keyStyle.Render("F3")+labelStyle.Render("-Channels"),
		keyStyle.Render("F4")+labelStyle.Render("-Tokens"),
		keyStyle.Render("F5")+labelStyle.Render("-Sessions"),
	)

	parts = append(parts,
		hintStyle.Render("F8 config"),
		hintStyle.Render("F9 focus"),
		hintStyle.Render("Tab/Shift+Tab navigate"),
		hintStyle.Render("Ctrl+C quit"),
	)

	content := strings.Join(parts, "  ")
	return atoms.StatusBarStyle.Width(m.width).Render(content)
}

// SetSize updates the page and all panel dimensions.
func (m *NormalPageModel) SetSize(w, h int) {
	m.width = w
	m.height = h
	contentH := h - 1 // status bar takes 1 row
	leftW := w / 2
	rightW := w - leftW
	panelH := contentH / 4

	m.chat.SetSize(leftW, contentH)
	m.channels.SetSize(rightW, panelH)
	m.tokens.SetSize(rightW, panelH)
	m.sessions.SetSize(rightW, panelH)
	m.logs.SetSize(rightW, contentH-3*panelH)
}

// IsChatFocused returns true when the chat panel is currently focused.
func (m NormalPageModel) IsChatFocused() bool {
	return m.focusedPanel == 0
}

func (m *NormalPageModel) updateFocus() {
	m.chat.SetFocused(m.focusedPanel == 0)
	m.logs.SetFocused(m.focusedPanel == 1)
	m.channels.SetFocused(m.focusedPanel == 2)
	m.tokens.SetFocused(m.focusedPanel == 3)
	m.sessions.SetFocused(m.focusedPanel == 4)
}
