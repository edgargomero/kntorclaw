package tui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// Panel indices for normal mode
const (
	panelChatHistory = 0
	panelLogs        = 1
	panelChannels    = 2
	panelTokens      = 3
	panelSessions    = 4
)

func (a *App) setupKeybindings() {
	a.panels = []tview.Primitive{
		a.chatHistory,
		a.logsView,
		a.channelsTable,
		a.tokensTable,
		a.sessionsTable,
	}
	a.focusIndex = 0

	a.tviewApp.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		// Skip global keybindings when a modal is open
		if a.modalOpen {
			return event
		}

		// Mode toggles (always available)
		switch event.Key() {
		case tcell.KeyF8:
			if a.focusMode {
				a.toggleFocusMode()
			}
			a.toggleConfigMode()
			return nil
		case tcell.KeyF9:
			if a.configMode {
				a.toggleConfigMode()
			}
			a.toggleFocusMode()
			return nil
		}

		// Alt+M opens model picker (all modes)
		if event.Key() == tcell.KeyRune && event.Rune() == 'm' && event.Modifiers()&tcell.ModAlt != 0 {
			a.showModelPicker()
			return nil
		}

		// Config mode: delegate all keys to config handler
		if a.configMode {
			return a.handleConfigModeKeys(event)
		}

		// Normal / Focus mode keybindings
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
			a.setFocus(5) // QA panel in focus mode (no-op in normal mode)
			return nil
		case tcell.KeyEnter:
			// Approve checkpoint when QA panel is focused and waiting
			if a.focusMode && a.qaTracker != nil && a.qaTracker.WaitingApproval && a.focusIndex == 5 {
				a.qaTracker.Approve()
				a.tviewApp.QueueUpdateDraw(func() {
					a.refreshQAPanel()
				})
				return nil
			}
		case tcell.KeyTab:
			a.focusNext()
			return nil
		case tcell.KeyBacktab:
			a.focusPrev()
			return nil
		case tcell.KeyEsc:
			// Reject checkpoint when QA panel is focused and waiting
			if a.focusMode && a.qaTracker != nil && a.qaTracker.WaitingApproval && a.focusIndex == 5 {
				a.qaTracker.Reject()
				a.tviewApp.QueueUpdateDraw(func() {
					a.refreshQAPanel()
				})
				return nil
			}
			a.tviewApp.SetFocus(a.chatInput)
			return nil
		}

		// Redirect typing to chat input from read-only panels
		if event.Key() == tcell.KeyRune && event.Modifiers() == 0 {
			a.tviewApp.SetFocus(a.chatInput)
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
