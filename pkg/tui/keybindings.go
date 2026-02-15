package tui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func (a *App) setupKeybindings() {
	a.panels = []tview.Primitive{
		a.chatHistory,
		a.logsView,
		a.channelsTable,
		a.tokensTable,
		a.sessionsTable,
		a.configView,
	}
	a.focusIndex = 0

	a.tviewApp.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyF1:
			a.setFocus(0)
			return nil
		case tcell.KeyF2:
			a.setFocus(1)
			return nil
		case tcell.KeyF3:
			a.setFocus(2)
			return nil
		case tcell.KeyF4:
			a.setFocus(3)
			return nil
		case tcell.KeyF5:
			a.setFocus(4)
			return nil
		case tcell.KeyF6:
			a.setFocus(5)
			return nil
		case tcell.KeyTab:
			a.focusNext()
			return nil
		case tcell.KeyBacktab:
			a.focusPrev()
			return nil
		case tcell.KeyEsc:
			a.tviewApp.SetFocus(a.chatInput)
			return nil
		}

		// If typing regular characters and not focused on input, redirect to input
		if event.Key() == tcell.KeyRune {
			focused := a.tviewApp.GetFocus()
			if focused != a.chatInput {
				a.tviewApp.SetFocus(a.chatInput)
				// Let the key pass through to the input
			}
		}

		return event
	})
}

func (a *App) setFocus(index int) {
	if index >= 0 && index < len(a.panels) {
		a.focusIndex = index
		a.tviewApp.SetFocus(a.panels[index])
	}
}

func (a *App) focusNext() {
	a.focusIndex = (a.focusIndex + 1) % len(a.panels)
	a.tviewApp.SetFocus(a.panels[a.focusIndex])
}

func (a *App) focusPrev() {
	a.focusIndex--
	if a.focusIndex < 0 {
		a.focusIndex = len(a.panels) - 1
	}
	a.tviewApp.SetFocus(a.panels[a.focusIndex])
}
