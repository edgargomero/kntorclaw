package tui

import (
	"fmt"
	"sort"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// modelPickerScope determines where the selected model applies.
type modelPickerScope int

const (
	scopeSession modelPickerScope = iota
	scopeChannel
	scopeDefault
)

func (s modelPickerScope) String() string {
	switch s {
	case scopeSession:
		return "session"
	case scopeChannel:
		return "channel"
	case scopeDefault:
		return "default"
	}
	return "session"
}

// showModelPicker opens a modal list to select a model.
func (a *App) showModelPicker() {
	if a.modelRouter == nil {
		return
	}

	scope := scopeSession

	// Collect models from aliases + current model
	aliases := a.modelRouter.GetAliases()
	currentModel, currentSource := a.modelRouter.GetInfo("tui", "tui:local")

	type entry struct {
		modelID string
		alias   string
	}

	// Build deduplicated list of models
	seen := make(map[string]string) // model-id → alias
	for alias, modelID := range aliases {
		if existing, ok := seen[modelID]; ok {
			// Keep shorter alias
			if len(alias) < len(existing) {
				seen[modelID] = alias
			}
		} else {
			seen[modelID] = alias
		}
	}
	// Ensure current model is in the list
	if _, ok := seen[currentModel]; !ok {
		seen[currentModel] = ""
	}
	// Add known models from configured providers
	for _, m := range a.getKnownModels() {
		if _, ok := seen[m]; !ok {
			seen[m] = ""
		}
	}

	var entries []entry
	for modelID, alias := range seen {
		entries = append(entries, entry{modelID: modelID, alias: alias})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].modelID < entries[j].modelID
	})

	// Build the list widget
	list := tview.NewList()
	list.SetBorder(true)
	list.SetBorderColor(tcell.ColorBlue)
	list.ShowSecondaryText(true)
	list.SetHighlightFullLine(true)
	list.SetSelectedBackgroundColor(tcell.ColorDarkBlue)

	updateTitle := func() {
		list.SetTitle(fmt.Sprintf(" Select Model [scope: %s] ", scope))
	}
	updateTitle()

	currentIndex := 0
	for i, e := range entries {
		label := e.modelID
		secondary := fmt.Sprintf("  alias: %s", e.alias)
		if e.alias == "" {
			secondary = ""
		}
		if e.modelID == currentModel {
			label = fmt.Sprintf("[green]%s (current)[-]", e.modelID)
			currentIndex = i
		}
		list.AddItem(label, secondary, 0, nil)
	}
	list.SetCurrentItem(currentIndex)

	// Footer info
	footer := tview.NewTextView().
		SetTextAlign(tview.AlignCenter).
		SetDynamicColors(true)
	fmt.Fprintf(footer, "[yellow]Current:[-] %s [gray](%s)[-]  |  [yellow]Enter[-] apply  [yellow]Esc[-] cancel  [yellow]Tab[-] scope: %s", currentModel, currentSource, scope)

	// Layout: list + footer
	modalFlex := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(list, 0, 1, true).
		AddItem(footer, 1, 0, false)

	// Center the modal
	modal := tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(modalFlex, 16, 0, true).
			AddItem(nil, 0, 1, false),
			50, 0, true).
		AddItem(nil, 0, 1, false)

	// Save previous root to restore on close
	previousRoot := a.tviewApp.GetFocus()
	a.modalOpen = true

	closePicker := func() {
		a.modalOpen = false
		if a.configMode {
			a.tviewApp.SetRoot(a.configLayout, true)
		} else if a.focusMode {
			a.tviewApp.SetRoot(a.focusLayout, true)
		} else {
			a.tviewApp.SetRoot(a.normalLayout, true)
		}
		a.tviewApp.SetFocus(previousRoot)
	}

	list.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEscape:
			closePicker()
			return nil
		case tcell.KeyTab:
			// Cycle scope
			scope = (scope + 1) % 3
			updateTitle()
			footer.Clear()
			fmt.Fprintf(footer, "[yellow]Current:[-] %s [gray](%s)[-]  |  [yellow]Enter[-] apply  [yellow]Esc[-] cancel  [yellow]Tab[-] scope: %s", currentModel, currentSource, scope)
			return nil
		case tcell.KeyEnter:
			idx := list.GetCurrentItem()
			if idx >= 0 && idx < len(entries) {
				selected := entries[idx].modelID
				switch scope {
				case scopeSession:
					a.modelRouter.SetSessionModel("tui:local", selected)
				case scopeChannel:
					a.modelRouter.SetChannelModel("tui", selected)
					if a.config.Agents.Models == nil {
						a.config.Agents.Models = make(map[string]string)
					}
					a.config.Agents.Models["tui"] = selected
					a.saveConfig()
				case scopeDefault:
					a.modelRouter.SetDefaultModel(selected)
					a.config.Agents.Defaults.Model = selected
					a.saveConfig()
				}
				a.renderConfig()
			}
			closePicker()
			return nil
		}
		return event
	})

	// Show the modal by setting it as root with a pages overlay
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
		AddPage("picker", modal, true, true)

	a.tviewApp.SetRoot(pages, true)
	a.tviewApp.SetFocus(list)
}
