package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/sipeed/picoclaw/pkg/tui/domain"
	"github.com/sipeed/picoclaw/pkg/tui/organisms"
	"github.com/sipeed/picoclaw/pkg/tui/pages"
)

// AppMode represents the current UI mode
type AppMode int

const (
	ModeNormal AppMode = iota
	ModeConfig
	ModeFocus
)

// Model is the root bubbletea model
type Model struct {
	mode   AppMode
	width  int
	height int
	ready  bool

	normalPage pages.NormalPageModel
	configPage pages.ConfigPageModel
	focusPage  pages.FocusPageModel

	// Modal overlays
	showHelp    bool
	modelPicker organisms.ModelPickerModel
	errorViewer organisms.ErrorViewerModel

	keys         KeyMap
	tracker      *domain.ActivityTracker
	errorTracker *domain.ErrorTracker

	// Callback to send user messages to the agent via bus
	onUserMessage func(content string)

	// Callback when model is selected from picker
	onModelSelected func(model, scope, channel string)

	// Deferred state — stored for propagation after pages are initialized
	configCallbacks *organisms.ConfigCallbacks
	qaTracker       *domain.QATracker
	workspacePath   string
	usagePath       string
}

// NewModel creates a new root Model. The tracker and errorTracker may be nil.
func NewModel(tracker *domain.ActivityTracker, errorTracker *domain.ErrorTracker) Model {
	modelPicker := organisms.NewModelPicker(nil, nil)
	var errorViewer organisms.ErrorViewerModel
	if errorTracker != nil {
		errorViewer = organisms.NewErrorViewer(errorTracker)
	}

	return Model{
		mode:         ModeNormal,
		keys:         Keys,
		tracker:      tracker,
		errorTracker: errorTracker,
		modelPicker:  modelPicker,
		errorViewer:  errorViewer,
	}
}

// SetOnUserMessage sets the callback invoked when the user sends a chat message.
func (m *Model) SetOnUserMessage(fn func(string)) {
	m.onUserMessage = fn
}

// SetOnModelSelected sets the callback invoked when a model is selected from the picker.
func (m *Model) SetOnModelSelected(fn func(model, scope, channel string)) {
	m.onModelSelected = fn
}

// SetConfigCallbacks stores config callbacks for propagation to the config page.
func (m *Model) SetConfigCallbacks(cb *organisms.ConfigCallbacks) {
	m.configCallbacks = cb
	if m.ready {
		m.configPage.SetConfigCallbacks(cb)
	}
}

// SetQATracker stores the QA tracker for propagation to the focus page.
func (m *Model) SetQATracker(qt *domain.QATracker) {
	m.qaTracker = qt
	if m.ready {
		m.focusPage.SetQATracker(qt)
	}
}

// SetModelPickerData configures the model picker with available models, channels, and aliases.
func (m *Model) SetModelPickerData(models, channels []string, currentModel string, aliases map[string]string) {
	m.modelPicker.SetData(models, channels, currentModel, aliases)
}

// SetWorkspacePath stores the workspace path for propagation to session panels.
func (m *Model) SetWorkspacePath(path string) {
	m.workspacePath = path
}

// SetUsagePath stores the token usage path for propagation to the tokens panel.
func (m *Model) SetUsagePath(path string) {
	m.usagePath = path
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		tea.EnterAltScreen,
		tickTokens(),
		tickChannels(),
		tickSessions(),
		tickFocus(),
	)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if !m.ready {
			// First size message: initialize pages
			m.normalPage = pages.NewNormalPage(m.width, m.height, m.tracker)
			m.configPage = pages.NewConfigPage(m.width, m.height)
			m.focusPage = pages.NewFocusPage(m.width, m.height)
			// Propagate error tracker to pages for status bar badge
			if m.errorTracker != nil {
				m.normalPage.SetErrorTracker(m.errorTracker)
				m.configPage.SetErrorTracker(m.errorTracker)
				m.focusPage.SetErrorTracker(m.errorTracker)
			}
			// Propagate deferred state to pages
			if m.configCallbacks != nil {
				m.configPage.SetConfigCallbacks(m.configCallbacks)
			}
			if m.qaTracker != nil {
				m.focusPage.SetQATracker(m.qaTracker)
			}
			if m.workspacePath != "" {
				m.normalPage.SetWorkspacePath(m.workspacePath)
			}
			if m.usagePath != "" {
				m.normalPage.SetUsagePath(m.usagePath)
			}
			m.ready = true
			return m, m.normalPage.Init()
		}
		// Resize all pages
		m.normalPage.SetSize(m.width, m.height)
		m.configPage.SetSize(m.width, m.height)
		m.focusPage.SetSize(m.width, m.height)
		return m, nil

	case tea.KeyMsg:
		// If model picker is visible, route keys there first
		if m.modelPicker.Visible {
			var cmd tea.Cmd
			m.modelPicker, cmd = m.modelPicker.Update(msg)
			return m, cmd
		}
		// If error viewer is visible, route keys there first
		if m.errorViewer.Visible {
			var cmd tea.Cmd
			m.errorViewer, cmd = m.errorViewer.Update(msg)
			return m, cmd
		}

		// If config page has an active editor/confirm overlay, route all keys there
		if m.mode == ModeConfig && m.ready && m.configPage.HasActiveOverlay() {
			var cmd tea.Cmd
			m.configPage, cmd = m.configPage.Update(msg)
			return m, cmd
		}

		// Global keys (work in all modes)
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "f8":
			if m.mode == ModeConfig {
				m.mode = ModeNormal
			} else {
				m.mode = ModeConfig
				// Auto-load the first section when entering config mode
				if m.ready {
					section := m.configPage.GetSelectedSection()
					if section != "" {
						m.configPage.LoadSelectedSection()
					}
				}
			}
			return m, nil
		case "f9":
			if m.mode == ModeFocus {
				m.mode = ModeNormal
			} else {
				m.mode = ModeFocus
				// Trigger immediate git data load when entering focus mode
				m.focusPage.LoadGitData()
			}
			return m, nil
		case "?":
			// Only open help when NOT in a text-input panel (chat focused)
			if m.mode == ModeFocus && m.focusPage.IsChatFocused() {
				break // fall through to page handler so ? goes to chat input
			}
			if m.mode == ModeNormal && m.normalPage.IsChatFocused() {
				break // fall through to page handler so ? goes to chat input
			}
			m.showHelp = !m.showHelp
			return m, nil
		case "alt+m":
			if m.modelPicker.Visible {
				m.modelPicker.Hide()
			} else {
				m.modelPicker.Show()
			}
			return m, nil
		case "ctrl+e":
			if m.errorViewer.Visible {
				m.errorViewer.Hide()
			} else {
				m.errorViewer.Show()
			}
			return m, nil
		}

	case organisms.ChatSendMsg:
		// User sent a message from the chat input — forward to agent via bus
		if m.onUserMessage != nil {
			go m.onUserMessage(msg.Content)
		}
		return m, nil

	case organisms.ConfigExitMsg:
		// Esc from sections panel in config mode exits back to normal
		m.mode = ModeNormal
		return m, nil

	case organisms.ModelSelectedMsg:
		if m.onModelSelected != nil {
			go m.onModelSelected(msg.Model, msg.Scope, msg.Channel)
		}
		return m, nil

	case tickTokensMsg:
		cmds = append(cmds, tickTokens())
		// Convert to organism tick message
		orgMsg := organisms.TickTokensMsg{}
		return m.forwardToActivePage(orgMsg, cmds)
	case tickChannelsMsg:
		cmds = append(cmds, tickChannels())
		orgMsg := organisms.TickChannelsMsg{}
		return m.forwardToActivePage(orgMsg, cmds)
	case tickSessionsMsg:
		cmds = append(cmds, tickSessions())
		orgMsg := organisms.TickSessionsMsg{}
		return m.forwardToActivePage(orgMsg, cmds)
	case tickFocusMsg:
		cmds = append(cmds, tickFocus())
		orgMsg := organisms.TickFocusMsg{}
		return m.forwardToActivePage(orgMsg, cmds)
	}

	// Forward all other messages to the active page
	return m.forwardToActivePage(msg, cmds)
}

// forwardToActivePage sends a message to whichever page is currently active.
func (m Model) forwardToActivePage(msg tea.Msg, cmds []tea.Cmd) (tea.Model, tea.Cmd) {
	if !m.ready {
		return m, tea.Batch(cmds...)
	}

	var cmd tea.Cmd
	switch m.mode {
	case ModeNormal:
		m.normalPage, cmd = m.normalPage.Update(msg)
	case ModeConfig:
		m.configPage, cmd = m.configPage.Update(msg)
	case ModeFocus:
		m.focusPage, cmd = m.focusPage.Update(msg)
	}
	cmds = append(cmds, cmd)
	return m, tea.Batch(cmds...)
}

func (m Model) View() string {
	if !m.ready {
		return "Initializing..."
	}

	var base string
	switch m.mode {
	case ModeConfig:
		base = m.configPage.View()
	case ModeFocus:
		base = m.focusPage.View()
	default:
		base = m.normalPage.View()
	}

	// Help screen overlay
	if m.showHelp {
		return m.renderHelpOverlay(base)
	}

	// Render modal overlays on top of the base view
	if m.modelPicker.Visible {
		return m.modelPicker.View(m.width, m.height)
	}
	if m.errorViewer.Visible {
		return m.errorViewer.View(m.width, m.height)
	}

	return base
}

func (m Model) renderHelpOverlay(base string) string {
	headerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#00ffff")).Bold(true)
	keyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#ffffff"))
	descStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#808080"))
	sepStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#808080"))

	var b strings.Builder
	b.WriteString(headerStyle.Render("Keybindings") + "\n")
	b.WriteString(sepStyle.Render(strings.Repeat("\u2500", 35)) + "\n\n")

	b.WriteString(headerStyle.Render("Normal Mode:") + "\n")
	normalBindings := [][2]string{
		{"F1-F5", "Focus panel"},
		{"Tab", "Next panel"},
		{"Shift+Tab", "Previous panel"},
		{"F8", "Config mode"},
		{"F9", "Focus mode"},
		{"Alt+M", "Model picker"},
		{"Ctrl+E", "Error viewer"},
		{"?", "This help"},
		{"Ctrl+C", "Quit"},
	}
	for _, kv := range normalBindings {
		b.WriteString("  " + keyStyle.Render(fmt.Sprintf("%-12s", kv[0])) + descStyle.Render(kv[1]) + "\n")
	}

	b.WriteString("\n" + headerStyle.Render("Config Mode:") + "\n")
	configBindings := [][2]string{
		{"Tab", "Switch panel"},
		{"Enter", "Select/Edit"},
		{"d", "Delete"},
		{"Esc", "Back"},
		{"F8", "Exit config"},
	}
	for _, kv := range configBindings {
		b.WriteString("  " + keyStyle.Render(fmt.Sprintf("%-12s", kv[0])) + descStyle.Render(kv[1]) + "\n")
	}

	b.WriteString("\n" + headerStyle.Render("Focus Mode:") + "\n")
	focusBindings := [][2]string{
		{"F1-F6", "Focus panel"},
		{"Enter", "Approve QA"},
		{"Esc", "Reject QA"},
		{"F9", "Exit focus"},
	}
	for _, kv := range focusBindings {
		b.WriteString("  " + keyStyle.Render(fmt.Sprintf("%-12s", kv[0])) + descStyle.Render(kv[1]) + "\n")
	}

	b.WriteString("\n" + descStyle.Render("Press ? to close"))

	modalW := 45
	modalH := 30
	if modalH > m.height-4 {
		modalH = m.height - 4
	}

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#00ffff")).
		Padding(1, 2).
		Width(modalW).
		Height(modalH)

	rendered := boxStyle.Render(b.String())

	// Center on screen
	lines := strings.Split(rendered, "\n")
	contentH := len(lines)
	contentW := 0
	for _, l := range lines {
		w := lipgloss.Width(l)
		if w > contentW {
			contentW = w
		}
	}

	padTop := (m.height - contentH) / 2
	padLeft := (m.width - contentW) / 2
	if padTop < 0 {
		padTop = 0
	}
	if padLeft < 0 {
		padLeft = 0
	}

	var out strings.Builder
	for i := 0; i < padTop; i++ {
		out.WriteString("\n")
	}
	prefix := strings.Repeat(" ", padLeft)
	for _, l := range lines {
		out.WriteString(prefix + l + "\n")
	}
	return out.String()
}

// Tick commands
func tickTokens() tea.Cmd {
	return tea.Tick(5*time.Second, func(t time.Time) tea.Msg {
		return tickTokensMsg(t)
	})
}

func tickChannels() tea.Cmd {
	return tea.Tick(5*time.Second, func(t time.Time) tea.Msg {
		return tickChannelsMsg(t)
	})
}

func tickSessions() tea.Cmd {
	return tea.Tick(3*time.Second, func(t time.Time) tea.Msg {
		return tickSessionsMsg(t)
	})
}

func tickFocus() tea.Cmd {
	return tea.Tick(3*time.Second, func(t time.Time) tea.Msg {
		return tickFocusMsg(t)
	})
}
