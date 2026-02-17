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
	debugLog("REFRESH section=%d", section)
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
		debugLog("REFRESH model: calling collectModelIDs")
		models := a.collectModelIDs()
		debugLog("REFRESH model: got %d models", len(models))
		currentDefault := ""
		if a.modelRouter != nil {
			debugLog("REFRESH model: getting DefaultModel")
			currentDefault = a.modelRouter.DefaultModel()
			debugLog("REFRESH model: default=%s", currentDefault)
		}
		for _, m := range models {
			label := formatModelLabel(m)
			secondary := ""
			if m == currentDefault {
				label = "[green]" + formatModelLabel(m) + "[-]"
				secondary = "  [green](current default)[-]"
			}
			a.configItems.AddItem(label, secondary, 0, nil)
		}
		a.configInfo.Clear()
		fmt.Fprint(a.configInfo, " [yellow]Enter[-] set as default  [yellow]Tab[-] sections  [yellow]Esc[-] back  [yellow]F8[-] exit ")

	case configSectionChannels:
		a.configItems.SetTitle(" Channel Models ")
		a.configItems.SetBorderColor(tcell.ColorGreen)
		// Show ALL active channels, with current model or "default" indicator
		channels := a.getActiveChannels()
		channelModels := map[string]string{}
		if a.modelRouter != nil {
			channelModels = a.modelRouter.GetChannelModels()
		}
		defaultModel := ""
		if a.modelRouter != nil {
			defaultModel = a.modelRouter.DefaultModel()
		}
		for _, ch := range channels {
			if m, ok := channelModels[ch]; ok {
				a.configItems.AddItem("[green]"+ch+"[-]", "  → [green]"+formatModelLabel(m)+"[-]", 0, nil)
			} else {
				a.configItems.AddItem(ch, "  → [gray]"+formatModelLabel(defaultModel)+" (default)[-]", 0, nil)
			}
		}
		a.configInfo.Clear()
		fmt.Fprint(a.configInfo, " [yellow]Enter[-] set model  [yellow]d[-] clear override  [yellow]Tab[-] sections  [yellow]Esc[-] back  [yellow]F8[-] exit ")

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
				a.configItems.AddItem("[cyan]"+alias+"[-]", "  → "+formatModelLabel(aliases[alias]), 0, nil)
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
	debugLog("SELECT section=%d idx=%d", section, idx)

	switch section {
	case configSectionProviders:
		providers := a.getProviderList()
		debugLog("SELECT providers: count=%d idx=%d", len(providers), idx)
		if idx >= 0 && idx < len(providers) {
			selected := providers[idx].name
			debugLog("SELECT opening editor for: %s", selected)
			if selected == "anthropic-cc" {
				a.showAuthProviderInfo(selected)
			} else {
				a.showAPIKeyInput(selected)
			}
		}

	case configSectionModel:
		models := a.collectModelIDs()
		debugLog("SELECT model: count=%d idx=%d", len(models), idx)
		if idx >= 0 && idx < len(models) && a.modelRouter != nil {
			selected := models[idx]
			debugLog("SELECT [1] about to SetDefaultModel(%s)", selected)
			func() {
				defer func() {
					if r := recover(); r != nil {
						debugLog("SELECT PANIC in SetDefaultModel: %v", r)
					}
				}()
				a.modelRouter.SetDefaultModel(selected)
			}()
			debugLog("SELECT [2] SetDefaultModel done")
			func() {
				defer func() {
					if r := recover(); r != nil {
						debugLog("SELECT PANIC in config.Lock: %v", r)
					}
				}()
				a.config.Lock()
				a.config.Agents.Defaults.Model = selected
				a.config.Unlock()
			}()
			debugLog("SELECT [3] config updated, saving...")
			a.asyncSaveAndRefresh()
			debugLog("SELECT [4] save done")
		}

	case configSectionChannels:
		debugLog("SELECT channels: modelRouter=%v", a.modelRouter != nil)
		if a.modelRouter == nil {
			return
		}
		// All active channels are listed; select any to set/change its model
		channels := a.getActiveChannels()
		debugLog("SELECT channels: count=%d idx=%d", len(channels), idx)
		if idx >= 0 && idx < len(channels) {
			debugLog("SELECT channels: %s → showModelListForChannel", channels[idx])
			a.showModelListForChannel(channels[idx])
		}

	case configSectionAliases:
		debugLog("SELECT aliases: modelRouter=%v", a.modelRouter != nil)
		if a.modelRouter == nil {
			return
		}
		// If selecting an existing alias, go straight to model picker
		aliases := a.modelRouter.GetAliases()
		aliasKeys := make([]string, 0, len(aliases))
		for k := range aliases {
			aliasKeys = append(aliasKeys, k)
		}
		sort.Strings(aliasKeys)
		debugLog("SELECT aliases: existing=%d keys=%v idx=%d", len(aliasKeys), aliasKeys, idx)
		if idx >= 0 && idx < len(aliasKeys) {
			debugLog("SELECT aliases: existing alias %s → showModelListForAlias", aliasKeys[idx])
			a.showModelListForAlias(aliasKeys[idx])
		} else {
			debugLog("SELECT aliases: new → showAliasEditor")
			a.showAliasEditor()
		}
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
		channels := a.getActiveChannels()
		if idx < 0 || idx >= len(channels) {
			return
		}
		ch := channels[idx]
		// Only allow delete if channel has a model override
		if _, ok := a.modelRouter.GetChannelModels()[ch]; !ok {
			return // no override to delete
		}
		itemName = "model override: " + ch
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
		channels := a.getActiveChannels()
		if idx >= 0 && idx < len(channels) {
			ch := channels[idx]
			a.modelRouter.DeleteChannelModel(ch)
			a.config.Lock()
			delete(a.config.Agents.Models, ch)
			a.config.Unlock()
			debugLog("DELETE channel model: %s", ch)
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
	debugLog("CONFIG key=%v rune=%c focusIdx=%d", event.Key(), event.Rune(), a.focusIndex)
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
			debugLog("CONFIG Enter: focusIdx 0→1, focusing configItems (count=%d)", a.configItems.GetItemCount())
			a.focusIndex = 1
			a.tviewApp.SetFocus(a.configItems)
			debugLog("CONFIG Enter: done, focus=%T", a.tviewApp.GetFocus())
		} else {
			// Items → handle selection
			debugLog("CONFIG Enter: focusIdx=1, calling handleConfigItemSelect")
			a.handleConfigItemSelect()
			debugLog("CONFIG Enter: handleConfigItemSelect returned")
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
