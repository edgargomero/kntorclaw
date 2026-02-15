package tui

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func (a *App) buildChatPanel() tview.Primitive {
	a.chatHistory = newPanel("CHAT [F1]")
	a.chatHistory.SetChangedFunc(func() {
		a.chatHistory.ScrollToEnd()
	})

	a.chatInput = tview.NewTextArea().
		SetPlaceholder("Escribe un mensaje...").
		SetWordWrap(true)
	a.chatInput.SetBorder(true).SetTitle(" > ")
	a.chatInput.SetBorderColor(tcell.ColorGreen)

	// Enter sends message, Shift+Enter inserts newline
	a.chatInput.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEnter {
			// Shift+Enter → insert newline (let it pass through)
			if event.Modifiers()&tcell.ModShift != 0 {
				return event
			}
			// Plain Enter → send message
			text := a.chatInput.GetText()
			text = strings.TrimSpace(text)
			if text == "" {
				return nil
			}
			a.chatInput.SetText("", true)
			a.appendChatMessage("You", text)
			if a.tuiChannel != nil {
				a.tuiChannel.sendMessage(text)
			}
			return nil
		}
		return event
	})

	return a.chatHistory
}

func (a *App) appendChat(sender, content string) {
	a.tviewApp.QueueUpdateDraw(func() {
		a.appendChatMessage(sender, content)
	})
}

func (a *App) appendChatMessage(sender, content string) {
	color := "green"
	if sender == "Agent" {
		color = "cyan"
	}

	// Separator line
	fmt.Fprintf(a.chatHistory, "[gray]───────────────────────────────────[-]\n")

	// Sender header
	fmt.Fprintf(a.chatHistory, "[%s::b]> %s[-::-]\n", color, sender)

	// Format content with basic code block highlighting
	formatted := formatContent(content)
	fmt.Fprintf(a.chatHistory, "%s\n", formatted)
}

func formatContent(content string) string {
	lines := strings.Split(content, "\n")
	var result strings.Builder
	inCodeBlock := false

	for _, line := range lines {
		if strings.HasPrefix(line, "```") {
			inCodeBlock = !inCodeBlock
			if inCodeBlock {
				result.WriteString("[darkgray]" + tview.Escape(line) + "[-]\n")
			} else {
				result.WriteString("[darkgray]" + tview.Escape(line) + "[-]\n")
			}
			continue
		}
		if inCodeBlock {
			result.WriteString("[yellow]" + tview.Escape(line) + "[-]\n")
		} else {
			result.WriteString("[white]" + tview.Escape(line) + "[-]\n")
		}
	}

	// Remove trailing newline
	s := result.String()
	if strings.HasSuffix(s, "\n") {
		s = s[:len(s)-1]
	}
	return s
}
