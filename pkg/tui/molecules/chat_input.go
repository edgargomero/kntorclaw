package molecules

import (
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
)

// SendMsg is emitted when the user presses Enter with non-empty input.
type SendMsg struct{ Content string }

// ChatInputModel wraps a bubbles textarea for chat message composition.
type ChatInputModel struct {
	textarea textarea.Model
	focused  bool
}

// NewChatInput creates a ChatInputModel with sensible defaults.
func NewChatInput() ChatInputModel {
	ta := textarea.New()
	ta.Placeholder = "Type a message... (Enter to send, Shift+Enter for newline)"
	ta.CharLimit = 4000
	ta.SetHeight(3)
	ta.ShowLineNumbers = false
	ta.Focus()
	return ChatInputModel{textarea: ta, focused: true}
}

// Init returns the textarea blink command.
func (m ChatInputModel) Init() tea.Cmd { return textarea.Blink }

// Update handles key events; Enter sends the message, other keys are forwarded.
func (m ChatInputModel) Update(msg tea.Msg) (ChatInputModel, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			content := m.textarea.Value()
			if content != "" {
				m.textarea.Reset()
				return m, func() tea.Msg { return SendMsg{Content: content} }
			}
			return m, nil
		}
	}
	m.textarea, cmd = m.textarea.Update(msg)
	return m, cmd
}

// View renders the textarea.
func (m ChatInputModel) View() string {
	return m.textarea.View()
}

// Focus activates the textarea cursor.
func (m *ChatInputModel) Focus() tea.Cmd { m.focused = true; return m.textarea.Focus() }

// Blur deactivates the textarea cursor.
func (m *ChatInputModel) Blur() { m.focused = false; m.textarea.Blur() }

// SetWidth sets the textarea width.
func (m *ChatInputModel) SetWidth(w int) { m.textarea.SetWidth(w) }

// Focused returns whether the input is focused.
func (m ChatInputModel) Focused() bool { return m.focused }
