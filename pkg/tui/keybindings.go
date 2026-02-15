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
		// F9 toggles focus mode
		if event.Key() == tcell.KeyF9 {
			a.toggleFocusMode()
			return nil
		}

		// Ctrl+M opens model picker
		if event.Key() == tcell.KeyRune && event.Rune() == 'm' && event.Modifiers()&tcell.ModAlt != 0 {
			a.showModelPicker()
			return nil
		}

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
			if a.focusMode {
				a.setFocus(5) // QA panel in focus mode
			} else {
				a.setFocus(5) // Config in normal mode
			}
			return nil
		case tcell.KeyEnter:
			// Approve checkpoint when QA panel is focused and waiting
			if a.focusMode && a.qaTracker != nil && a.qaTracker.WaitingApproval {
				focused := a.tviewApp.GetFocus()
				if focused == a.focusQA {
					a.qaTracker.Approve()
					a.tviewApp.QueueUpdateDraw(func() {
						a.refreshQAPanel()
					})
					return nil
				}
			}
		case tcell.KeyTab:
			a.focusNext()
			return nil
		case tcell.KeyBacktab:
			a.focusPrev()
			return nil
		case tcell.KeyEsc:
			// Reject checkpoint when QA panel is focused and waiting
			if a.focusMode && a.qaTracker != nil && a.qaTracker.WaitingApproval {
				focused := a.tviewApp.GetFocus()
				if focused == a.focusQA {
					a.qaTracker.Reject()
					a.tviewApp.QueueUpdateDraw(func() {
						a.refreshQAPanel()
					})
					return nil
				}
			}
			a.tviewApp.SetFocus(a.chatInput)
			return nil
		}

		// If typing regular characters and not focused on input or a focus-mode list, redirect to input
		if event.Key() == tcell.KeyRune && event.Modifiers() == 0 {
			focused := a.tviewApp.GetFocus()
			if focused != a.chatInput &&
				focused != a.focusFiles &&
				focused != a.focusBranches &&
				focused != a.focusCommits &&
				focused != a.focusQA {
				a.tviewApp.SetFocus(a.chatInput)
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
