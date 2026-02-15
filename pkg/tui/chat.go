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

			// Intercept /model command before sending to bus
			if a.handleModelCommand(text) {
				a.chatInput.SetText("", true)
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

// handleModelCommand intercepts /model commands typed in the chat input.
// Returns true if the text was a /model command (and was handled).
func (a *App) handleModelCommand(text string) bool {
	if !strings.HasPrefix(text, "/model") {
		return false
	}

	if a.modelRouter == nil {
		a.appendChat("System", "Model router not initialized.")
		return true
	}

	args := strings.TrimSpace(strings.TrimPrefix(text, "/model"))

	// "/model" or "/model status" → show current model info
	if args == "" || args == "status" {
		sessionKey := "tui:local"
		model, source := a.modelRouter.GetInfo("tui", sessionKey)
		msg := fmt.Sprintf("Current model: %s\nSource: %s", model, source)
		aliases := a.modelRouter.GetAliases()
		if len(aliases) > 0 {
			msg += "\n\nAliases:"
			for alias, target := range aliases {
				msg += fmt.Sprintf("\n  %s → %s", alias, target)
			}
		}
		a.appendChat("System", msg)
		return true
	}

	// "/model <name>" → set session model
	resolved := a.modelRouter.ResolveAlias(args)
	sessionKey := "tui:local"
	a.modelRouter.SetSessionModel(sessionKey, resolved)
	a.appendChat("System", fmt.Sprintf("Model changed to: %s (session override)", resolved))
	a.renderConfig()
	return true
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
