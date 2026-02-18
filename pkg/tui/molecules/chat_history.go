package molecules

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/sipeed/picoclaw/pkg/tui/atoms"
)

// ChatHistoryModel displays a scrollable list of chat messages.
type ChatHistoryModel struct {
	viewport viewport.Model
	messages []string
	width    int
}

// NewChatHistory creates a ChatHistoryModel with the given dimensions.
func NewChatHistory(width, height int) ChatHistoryModel {
	vp := viewport.New(width, height)
	return ChatHistoryModel{viewport: vp, width: width}
}

// AppendMessage adds a formatted message and auto-scrolls to the bottom.
func (m *ChatHistoryModel) AppendMessage(sender, content string) {
	senderStyle := lipgloss.NewStyle().Foreground(atoms.ColorCyan).Bold(true)
	if sender == "user" || sender == "You" {
		senderStyle = lipgloss.NewStyle().Foreground(atoms.ColorGreen).Bold(true)
	}
	formatted := fmt.Sprintf("%s: %s", senderStyle.Render(sender), formatContent(content))
	m.messages = append(m.messages, formatted)
	m.viewport.SetContent(strings.Join(m.messages, "\n\n"))
	m.viewport.GotoBottom()
}

// Update forwards messages to the underlying viewport.
func (m ChatHistoryModel) Update(msg tea.Msg) (ChatHistoryModel, tea.Cmd) {
	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

// View renders the viewport.
func (m ChatHistoryModel) View() string {
	return m.viewport.View()
}

// formatContent applies styling for fenced code blocks and inline code.
func formatContent(content string) string {
	codeStyle := lipgloss.NewStyle().Foreground(atoms.ColorYellow)
	fenceStyle := lipgloss.NewStyle().Foreground(atoms.ColorGray)

	lines := strings.Split(content, "\n")
	var result []string
	inCodeBlock := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			inCodeBlock = !inCodeBlock
			result = append(result, fenceStyle.Render(line))
			continue
		}
		if inCodeBlock {
			result = append(result, codeStyle.Render(line))
			continue
		}
		result = append(result, formatInlineCode(line, codeStyle))
	}

	return strings.Join(result, "\n")
}

// formatInlineCode styles backtick-delimited spans within a single line.
func formatInlineCode(line string, codeStyle lipgloss.Style) string {
	var result strings.Builder
	inCode := false
	var codeBuf strings.Builder

	for i := 0; i < len(line); i++ {
		if line[i] == '`' {
			if inCode {
				result.WriteString(codeStyle.Render(codeBuf.String()))
				codeBuf.Reset()
				inCode = false
			} else {
				inCode = true
			}
		} else if inCode {
			codeBuf.WriteByte(line[i])
		} else {
			result.WriteByte(line[i])
		}
	}
	if inCode {
		result.WriteByte('`')
		result.WriteString(codeBuf.String())
	}
	return result.String()
}

// SetSize updates the viewport dimensions.
func (m *ChatHistoryModel) SetSize(w, h int) {
	m.width = w
	m.viewport.Width = w
	m.viewport.Height = h
}
