package tui

import (
	"fmt"
	"log"
	"sort"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// Config view section indices
const (
	configSectionProviders = iota
	configSectionModel
	configSectionChannels
	configSectionAliases
)

func (a *App) buildConfigViewPanels() {
	// Left: sections list
	a.configSections = tview.NewList()
	a.configSections.SetBorder(true)
	a.configSections.SetTitle(" Sections ")
	a.configSections.SetBorderColor(tcell.ColorBlue)
	a.configSections.ShowSecondaryText(false)
	a.configSections.SetHighlightFullLine(true)
	a.configSections.SetSelectedBackgroundColor(tcell.ColorDarkBlue)

	a.configSections.AddItem("Providers", "", 'p', nil)
	a.configSections.AddItem("Default Model", "", 'm', nil)
	a.configSections.AddItem("Channel Models", "", 'c', nil)
	a.configSections.AddItem("Aliases", "", 'a', nil)

	// Right: items list
	a.configItems = tview.NewList()
	a.configItems.SetBorder(true)
	a.configItems.ShowSecondaryText(true)
	a.configItems.SetHighlightFullLine(true)
	a.configItems.SetSelectedBackgroundColor(tcell.ColorDarkBlue)

	// Bottom: help text
	a.configInfo = tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter)
	a.configInfo.SetBackgroundColor(tcell.ColorDarkBlue)

	// When section selection changes, refresh items
	a.configSections.SetChangedFunc(func(index int, mainText, secondaryText string, shortcut rune) {
		a.refreshConfigItems(index)
	})
}

func (a *App) buildConfigLayout() *tview.Flex {
	rightColumn := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(a.configItems, 0, 1, false).
		AddItem(a.configInfo, 1, 0, false)

	content := tview.NewFlex().
		AddItem(a.configSections, 25, 0, true).
		AddItem(rightColumn, 0, 1, false)

	layout := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(a.statusBar, 1, 0, false).
		AddItem(content, 0, 1, true).
		AddItem(a.chatInput, 3, 0, false)

	return layout
}

func (a *App) toggleConfigMode() {
	if a.configMode {
		// Exit config mode → normal
		a.configMode = false
		a.panels = []tview.Primitive{
			a.chatHistory,
			a.logsView,
			a.channelsTable,
			a.tokensTable,
			a.sessionsTable,
		}
		a.focusIndex = 0
		a.updateStatusBar()
		a.tviewApp.SetRoot(a.normalLayout, true)
		a.tviewApp.SetFocus(a.chatInput)
	} else {
		// Enter config mode
		a.configMode = true
		a.refreshConfigItems(a.configSections.GetCurrentItem())
		a.configLayout = a.buildConfigLayout()
		a.panels = []tview.Primitive{
			a.configSections,
			a.configItems,
		}
		a.focusIndex = 0
		a.updateStatusBar()
		a.tviewApp.SetRoot(a.configLayout, true)
		a.tviewApp.SetFocus(a.configSections)
	}
}

func (a *App) refreshConfigItems(section int) {
	a.configItems.Clear()

	switch section {
	case configSectionProviders:
		a.configItems.SetTitle(" Providers ")
		a.configItems.SetBorderColor(tcell.ColorYellow)
		for _, p := range a.getProviderList() {
			status := "[red]not set[-]"
			if p.hasKey {
				status = "[green]configured[-]"
			}
			a.configItems.AddItem(p.name, "  "+status, 0, nil)
		}
		a.configInfo.Clear()
		fmt.Fprint(a.configInfo, " [yellow]Enter[-] edit API key  [yellow]d[-] clear key  [yellow]Tab[-] sections  [yellow]Esc[-] back  [yellow]F8[-] exit ")

	case configSectionModel:
		a.configItems.SetTitle(" Default Model ")
		a.configItems.SetBorderColor(tcell.ColorBlue)
		models := a.collectModelIDs()
		currentDefault := ""
		if a.modelRouter != nil {
			currentDefault = a.modelRouter.DefaultModel()
		}
		for _, m := range models {
			label := m
			secondary := ""
			if m == currentDefault {
				label = "[green]" + m + "[-]"
				secondary = "  [green](current default)[-]"
			}
			a.configItems.AddItem(label, secondary, 0, nil)
		}
		a.configInfo.Clear()
		fmt.Fprint(a.configInfo, " [yellow]Enter[-] set as default  [yellow]Tab[-] sections  [yellow]Esc[-] back  [yellow]F8[-] exit ")

	case configSectionChannels:
		a.configItems.SetTitle(" Channel Models ")
		a.configItems.SetBorderColor(tcell.ColorGreen)
		if a.modelRouter != nil {
			channelModels := a.modelRouter.GetChannelModels()
			keys := make([]string, 0, len(channelModels))
			for k := range channelModels {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, ch := range keys {
				a.configItems.AddItem("[green]"+ch+"[-]", "  → "+channelModels[ch], 0, nil)
			}
		}
		// Add "new" entry at the end
		a.configItems.AddItem("[yellow]+ Add new channel model[-]", "", 0, nil)
		a.configInfo.Clear()
		fmt.Fprint(a.configInfo, " [yellow]Enter[-] add/edit  [yellow]d[-] delete  [yellow]Tab[-] sections  [yellow]Esc[-] back  [yellow]F8[-] exit ")

	case configSectionAliases:
		a.configItems.SetTitle(" Aliases ")
		a.configItems.SetBorderColor(tcell.ColorDarkCyan)
		if a.modelRouter != nil {
			aliases := a.modelRouter.GetAliases()
			keys := make([]string, 0, len(aliases))
			for k := range aliases {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, alias := range keys {
				a.configItems.AddItem("[cyan]"+alias+"[-]", "  → "+aliases[alias], 0, nil)
			}
		}
		// Add "new" entry at the end
		a.configItems.AddItem("[yellow]+ Add new alias[-]", "", 0, nil)
		a.configInfo.Clear()
		fmt.Fprint(a.configInfo, " [yellow]Enter[-] add/edit  [yellow]d[-] delete  [yellow]Tab[-] sections  [yellow]Esc[-] back  [yellow]F8[-] exit ")
	}
}

func (a *App) handleConfigItemSelect() {
	section := a.configSections.GetCurrentItem()
	idx := a.configItems.GetCurrentItem()

	switch section {
	case configSectionProviders:
		providers := a.getProviderList()
		if idx >= 0 && idx < len(providers) {
			a.showAPIKeyInput(providers[idx].name)
		}

	case configSectionModel:
		models := a.collectModelIDs()
		if idx >= 0 && idx < len(models) && a.modelRouter != nil {
			selected := models[idx]
			a.modelRouter.SetDefaultModel(selected)
			a.config.Lock()
			a.config.Agents.Defaults.Model = selected
			a.config.Unlock()
			log.Printf("[config] Default model changed to: %s", selected)
			a.asyncSaveAndRefresh()
		}

	case configSectionChannels:
		a.showChannelModelEditor()

	case configSectionAliases:
		a.showAliasEditor()
	}
}

func (a *App) handleConfigItemDelete() {
	section := a.configSections.GetCurrentItem()
	idx := a.configItems.GetCurrentItem()
	if a.modelRouter == nil {
		return
	}

	// Determine what we're deleting for the confirmation message
	var itemName string
	switch section {
	case configSectionProviders:
		providers := a.getProviderList()
		if idx < 0 || idx >= len(providers) {
			return
		}
		itemName = providers[idx].name + " key"
	case configSectionChannels:
		channelModels := a.modelRouter.GetChannelModels()
		keys := make([]string, 0, len(channelModels))
		for k := range channelModels {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		if idx < 0 || idx >= len(keys) {
			return
		}
		itemName = "channel model: " + keys[idx]
	case configSectionAliases:
		aliases := a.modelRouter.GetAliases()
		keys := make([]string, 0, len(aliases))
		for k := range aliases {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		if idx < 0 || idx >= len(keys) {
			return
		}
		itemName = "alias: " + keys[idx]
	default:
		return
	}

	a.showDeleteConfirm(itemName, func() {
		a.executeConfigDelete(section, idx)
	})
}

func (a *App) showDeleteConfirm(itemName string, onConfirm func()) {
	label := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter)
	fmt.Fprintf(label, "\n[red]Delete %s?[-]\n\n[yellow]y[-] confirm  [yellow]n[-] cancel", itemName)
	label.SetBorder(true).SetTitle(" Confirm Delete ").SetBorderColor(tcell.ColorRed)

	modal := tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(label, 7, 0, true).
			AddItem(nil, 0, 1, false),
			40, 0, true).
		AddItem(nil, 0, 1, false)

	closeModal := a.showConfigModal(modal, label)

	label.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyRune {
			switch event.Rune() {
			case 'y', 'Y':
				closeModal()
				onConfirm()
				return nil
			case 'n', 'N':
				closeModal()
				return nil
			}
		}
		if event.Key() == tcell.KeyEscape {
			closeModal()
			return nil
		}
		return event
	})
}

func (a *App) executeConfigDelete(section, idx int) {
	switch section {
	case configSectionProviders:
		providers := a.getProviderList()
		if idx >= 0 && idx < len(providers) {
			a.setProviderKey(providers[idx].name, "")
			log.Printf("[config] Provider key cleared: %s", providers[idx].name)
			a.asyncSaveAndRefresh()
		}

	case configSectionChannels:
		channelModels := a.modelRouter.GetChannelModels()
		keys := make([]string, 0, len(channelModels))
		for k := range channelModels {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		if idx >= 0 && idx < len(keys) {
			key := keys[idx]
			a.modelRouter.DeleteChannelModel(key)
			a.config.Lock()
			delete(a.config.Agents.Models, key)
			a.config.Unlock()
			log.Printf("[config] Deleted channel model: %s", key)
			a.asyncSaveAndRefresh()
		}

	case configSectionAliases:
		aliases := a.modelRouter.GetAliases()
		keys := make([]string, 0, len(aliases))
		for k := range aliases {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		if idx >= 0 && idx < len(keys) {
			key := keys[idx]
			a.modelRouter.DeleteAlias(key)
			a.config.Lock()
			delete(a.config.Agents.Aliases, key)
			a.config.Unlock()
			log.Printf("[config] Deleted alias: %s", key)
			a.asyncSaveAndRefresh()
		}
	}
}

// handleConfigModeKeys processes all keybindings when in config mode.
func (a *App) handleConfigModeKeys(event *tcell.EventKey) *tcell.EventKey {
	switch event.Key() {
	case tcell.KeyEscape:
		if a.focusIndex == 1 {
			// Items → back to sections
			a.focusIndex = 0
			a.tviewApp.SetFocus(a.configSections)
		} else {
			// Sections → exit config mode
			a.toggleConfigMode()
		}
		return nil

	case tcell.KeyEnter:
		if a.focusIndex == 0 {
			// Sections → jump to items
			a.focusIndex = 1
			a.tviewApp.SetFocus(a.configItems)
		} else {
			// Items → handle selection
			a.handleConfigItemSelect()
		}
		return nil

	case tcell.KeyTab, tcell.KeyBacktab:
		if a.focusIndex == 0 {
			a.focusIndex = 1
			a.tviewApp.SetFocus(a.configItems)
		} else {
			a.focusIndex = 0
			a.tviewApp.SetFocus(a.configSections)
		}
		return nil
	}

	// 'd' for delete when on items
	if event.Key() == tcell.KeyRune && event.Rune() == 'd' && a.focusIndex == 1 {
		a.handleConfigItemDelete()
		return nil
	}

	// Let arrow keys and other keys through to the list widgets
	return event
}
