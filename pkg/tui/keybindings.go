package tui

import (
	"fmt"

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

		// ? opens help screen (all modes)
		if event.Key() == tcell.KeyRune && event.Rune() == '?' {
			a.showHelpScreen()
			return nil
		}

		// Alt+M opens model picker (all modes)
		if event.Key() == tcell.KeyRune && event.Rune() == 'm' && event.Modifiers()&tcell.ModAlt != 0 {
			a.showModelPicker()
			return nil
		}

		// Ctrl+E opens error viewer
		if event.Key() == tcell.KeyCtrlE {
			a.showErrorViewer()
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

func (a *App) showErrorViewer() {
	if a.errorTracker == nil {
		return
	}

	entries := a.errorTracker.Recent(50)
	a.errorTracker.Clear()
	a.updateStatusBar()

	tv := tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true).
		SetWordWrap(true)
	tv.SetBorder(true).SetTitle(" Errors (Ctrl+E) ").SetBorderColor(tcell.ColorRed)

	if len(entries) == 0 {
		fmt.Fprintln(tv, "[green]No recent errors[-]")
	} else {
		for i := len(entries) - 1; i >= 0; i-- {
			e := entries[i]
			color := "yellow"
			if e.Level == "ERROR" || e.Level == "FATAL" {
				color = "red"
			}
			comp := ""
			if e.Component != "" {
				comp = fmt.Sprintf(" [gray](%s)[-]", e.Component)
			}
			fmt.Fprintf(tv, "[%s][%s][-] %s%s %s\n", color, e.Level, color, comp, e.Message)
		}
	}

	previousFocus := a.tviewApp.GetFocus()
	a.modalOpen = true

	closeViewer := func() {
		a.modalOpen = false
		if a.configMode {
			a.tviewApp.SetRoot(a.configLayout, true)
		} else if a.focusMode {
			a.tviewApp.SetRoot(a.focusLayout, true)
		} else {
			a.tviewApp.SetRoot(a.normalLayout, true)
		}
		a.tviewApp.SetFocus(previousFocus)
	}

	tv.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape || event.Key() == tcell.KeyCtrlE {
			closeViewer()
			return nil
		}
		return event
	})

	modal := tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(tv, 20, 0, true).
			AddItem(nil, 0, 1, false),
			60, 0, true).
		AddItem(nil, 0, 1, false)

	pages := tview.NewPages().
		AddPage("background", func() tview.Primitive {
			if a.configMode {
				return a.configLayout
			}
			if a.focusMode {
				return a.focusLayout
			}
			return a.normalLayout
		}(), true, true).
		AddPage("errors", modal, true, true)

	a.tviewApp.SetRoot(pages, true)
	a.tviewApp.SetFocus(tv)
}

func (a *App) showHelpScreen() {
	tv := tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true).
		SetWordWrap(true)
	tv.SetBorder(true).SetTitle(" Help (?) ").SetBorderColor(tcell.ColorBlue)

	help := `[yellow]== Normal Mode ==[white]
  F1-F5      Focus panel (Chat, Logs, Channels, Tokens, Sessions)
  Tab        Next panel
  Shift+Tab  Previous panel
  F8         Enter config mode
  F9         Enter focus mode
  Esc        Return to chat input
  Alt+M      Model picker
  Ctrl+E     Error viewer
  ?          This help screen
  Ctrl+C     Quit

[yellow]== Config Mode (F8) ==[white]
  Tab        Switch between Sections / Items
  Enter      Select section or edit item
  d          Delete selected item (with confirmation)
  Esc        Back (Items -> Sections -> Exit)
  F8         Exit config mode

[yellow]== Focus Mode (F9) ==[white]
  F1-F6      Focus panel (Files, Branches, Commits, Chat, Diff, QA)
  Enter      Approve QA checkpoint (when QA focused)
  Esc        Reject QA checkpoint / Return to input
  F9         Exit focus mode

[yellow]== Model Picker (Alt+M) ==[white]
  Up/Down    Navigate models
  Left/Right Switch channel (tui, whatsapp, etc.)
  Tab        Cycle scope (session / channel / default)
  Enter      Apply selected model
  Esc        Cancel

[gray]Press Esc or ? to close this screen[-]`

	fmt.Fprint(tv, help)

	previousFocus := a.tviewApp.GetFocus()
	a.modalOpen = true

	closeHelp := func() {
		a.modalOpen = false
		if a.configMode {
			a.tviewApp.SetRoot(a.configLayout, true)
		} else if a.focusMode {
			a.tviewApp.SetRoot(a.focusLayout, true)
		} else {
			a.tviewApp.SetRoot(a.normalLayout, true)
		}
		a.tviewApp.SetFocus(previousFocus)
	}

	tv.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			closeHelp()
			return nil
		}
		if event.Key() == tcell.KeyRune && event.Rune() == '?' {
			closeHelp()
			return nil
		}
		return event
	})

	modal := tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(tv, 30, 0, true).
			AddItem(nil, 0, 1, false),
			60, 0, true).
		AddItem(nil, 0, 1, false)

	pages := tview.NewPages().
		AddPage("background", func() tview.Primitive {
			if a.configMode {
				return a.configLayout
			}
			if a.focusMode {
				return a.focusLayout
			}
			return a.normalLayout
		}(), true, true).
		AddPage("help", modal, true, true)

	a.tviewApp.SetRoot(pages, true)
	a.tviewApp.SetFocus(tv)
}
