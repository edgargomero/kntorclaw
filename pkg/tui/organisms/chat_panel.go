package organisms

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/sipeed/picoclaw/pkg/tui/atoms"
	"github.com/sipeed/picoclaw/pkg/tui/molecules"
)

// ChatSendMsg is emitted when user sends a message from the chat panel.
type ChatSendMsg struct{ Content string }

// ChatResponseMsg pushes an assistant response into the chat.
type ChatResponseMsg struct {
	Sender  string
	Content string
}

// ChatPanelModel composes ChatInputModel and ChatHistoryModel into a single
// chat panel with a scrollable history area and an input area separated by a
// horizontal rule.
type ChatPanelModel struct {
	history   molecules.ChatHistoryModel
	input     molecules.ChatInputModel
	width     int
	height    int
	focused   bool
	titleHint string // panel title hint e.g. "[F4]"; defaults to "[F1]"
}

// NewChatPanel creates a ChatPanelModel with the given dimensions.
func NewChatPanel(width, height int) ChatPanelModel {
	inputHeight := 5 // 3 lines + border
	historyHeight := height - inputHeight - 2
	if historyHeight < 1 {
		historyHeight = 1
	}
	return ChatPanelModel{
		history: molecules.NewChatHistory(width-2, historyHeight),
		input:   molecules.NewChatInput(),
		width:   width,
		height:  height,
		focused: true,
	}
}

// Init returns the init command for the input textarea.
func (m ChatPanelModel) Init() tea.Cmd {
	return m.input.Init()
}

// Update handles incoming messages, forwarding to sub-components as needed.
func (m ChatPanelModel) Update(msg tea.Msg) (ChatPanelModel, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case molecules.SendMsg:
		// Re-emit as ChatSendMsg for parent handling
		m.history.AppendMessage("You", msg.Content)
		return m, func() tea.Msg { return ChatSendMsg{Content: msg.Content} }

	case ChatResponseMsg:
		m.history.AppendMessage(msg.Sender, msg.Content)
		return m, nil
	}

	// Forward to input if focused
	if m.focused {
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		cmds = append(cmds, cmd)
	}

	// Forward to history for scrolling
	var cmd tea.Cmd
	m.history, cmd = m.history.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

// View renders the chat panel with history, separator, and input.
func (m ChatPanelModel) View() string {
	historyView := m.history.View()
	inputView := m.input.View()

	innerW := m.width - 2
	if innerW < 0 {
		innerW = 0
	}

	separator := lipgloss.NewStyle().
		Foreground(atoms.ColorGray).
		Width(innerW).
		Render(strings.Repeat("─", innerW))

	content := lipgloss.JoinVertical(lipgloss.Left,
		historyView,
		separator,
		inputView,
	)

	title := "CHAT [F1]"
	if m.titleHint != "" {
		title = "CHAT " + m.titleHint
	}
	return atoms.RenderPanel(title, m.focused, m.width, m.height, content)
}

// SetSize updates the panel and sub-component dimensions.
func (m *ChatPanelModel) SetSize(w, h int) {
	m.width = w
	m.height = h
	inputHeight := 5
	historyHeight := h - inputHeight - 4 // account for borders, separator
	if historyHeight < 1 {
		historyHeight = 1
	}
	m.history.SetSize(w-2, historyHeight)
	m.input.SetWidth(w - 2)
}

// SetTitleHint overrides the default "[F1]" hint shown in the panel title.
func (m *ChatPanelModel) SetTitleHint(hint string) {
	m.titleHint = hint
}

// SetFocused sets the focus state of the panel and its input.
func (m *ChatPanelModel) SetFocused(f bool) {
	m.focused = f
	if f {
		m.input.Focus()
	} else {
		m.input.Blur()
	}
}
