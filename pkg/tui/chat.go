package tui

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func (a *App) buildChatPanel() tview.Primitive {
	a.chatHistory = newPanel("CHAT [F1]")

	a.chatInput = tview.NewInputField().
		SetLabel("> ").
		SetFieldBackgroundColor(tcell.ColorDefault).
		SetLabelColor(tcell.ColorGreen)
	a.chatInput.SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEnter {
			text := a.chatInput.GetText()
			if text == "" {
				return
			}
			a.chatInput.SetText("")
			// Write directly since we're already in tview's event loop
			color := "green"
			fmt.Fprintf(a.chatHistory, "[%s]You:[white] %s\n\n", color, text)
			a.chatHistory.ScrollToEnd()
			if a.tuiChannel != nil {
				a.tuiChannel.sendMessage(text)
			}
		}
	})

	return a.chatHistory
}

func (a *App) appendChat(sender, content string) {
	a.tviewApp.QueueUpdateDraw(func() {
		color := "green"
		if sender == "Agent" {
			color = "cyan"
		}
		fmt.Fprintf(a.chatHistory, "[%s]%s:[white] %s\n\n", color, sender, content)
		a.chatHistory.ScrollToEnd()
	})
}
