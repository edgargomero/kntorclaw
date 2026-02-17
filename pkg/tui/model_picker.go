package tui

import (
	"fmt"
	"log"
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

// getActiveChannels returns channel names from all sources: config enabled channels,
// channelManager status, and already-configured channel models. "tui" is always first.
func (a *App) getActiveChannels() []string {
	seen := map[string]bool{"tui": true}

	// From config: channels with enabled: true
	if a.config != nil {
		if a.config.Channels.WhatsApp.Enabled {
			seen["whatsapp"] = true
		}
		if a.config.Channels.Telegram.Enabled {
			seen["telegram"] = true
		}
		if a.config.Channels.Discord.Enabled {
			seen["discord"] = true
		}
		if a.config.Channels.Feishu.Enabled {
			seen["feishu"] = true
		}
		if a.config.Channels.Slack.Enabled {
			seen["slack"] = true
		}
		if a.config.Channels.LINE.Enabled {
			seen["line"] = true
		}
		if a.config.Channels.QQ.Enabled {
			seen["qq"] = true
		}
		if a.config.Channels.DingTalk.Enabled {
			seen["dingtalk"] = true
		}
		if a.config.Channels.OneBot.Enabled {
			seen["onebot"] = true
		}
		if a.config.Channels.MaixCam.Enabled {
			seen["maixcam"] = true
		}
	}

	// From channelManager (runtime status)
	if a.channelManager != nil {
		for name := range a.channelManager.GetStatus() {
			seen[name] = true
		}
	}

	// From already-configured channel models in config
	if a.modelRouter != nil {
		for ch := range a.modelRouter.GetChannelModels() {
			seen[ch] = true
		}
	}

	channels := []string{"tui"}
	for name := range seen {
		if name != "tui" {
			channels = append(channels, name)
		}
	}
	sort.Strings(channels[1:])
	return channels
}

// showModelPicker opens a modal list to select a model.
func (a *App) showModelPicker() {
	debugLog("MODEL_PICKER: open, modelRouter=%v", a.modelRouter != nil)
	if a.modelRouter == nil {
		return
	}

	scope := scopeSession
	channels := a.getActiveChannels()
	channelIdx := 0 // start on "tui"
	debugLog("MODEL_PICKER: channels=%v", channels)
	log.Printf("[model-picker] Available channels: %v", channels)

	targetChannel := func() string { return channels[channelIdx] }
	targetSession := func() string {
		ch := targetChannel()
		if ch == "tui" {
			return "tui:local"
		}
		return ""
	}

	type entry struct {
		modelID string
		alias   string
	}

	// Build deduplicated list of models
	aliases := a.modelRouter.GetAliases()
	buildEntries := func() ([]entry, string, string) {
		currentModel, currentSource := a.modelRouter.GetInfo(targetChannel(), targetSession())

		seen := make(map[string]string) // model-id → alias
		for alias, modelID := range aliases {
			if existing, ok := seen[modelID]; ok {
				if len(alias) < len(existing) {
					seen[modelID] = alias
				}
			} else {
				seen[modelID] = alias
			}
		}
		if _, ok := seen[currentModel]; !ok {
			seen[currentModel] = ""
		}
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
		return entries, currentModel, currentSource
	}

	entries, currentModel, currentSource := buildEntries()

	// Build the list widget
	list := tview.NewList()
	list.SetBorder(true)
	list.SetBorderColor(tcell.ColorBlue)
	list.ShowSecondaryText(true)
	list.SetHighlightFullLine(true)
	list.SetSelectedBackgroundColor(tcell.ColorDarkBlue)

	populateList := func() {
		list.Clear()
		currentIndex := 0
		for i, e := range entries {
			label := formatModelLabel(e.modelID)
			secondary := fmt.Sprintf("  alias: %s", e.alias)
			if e.alias == "" {
				secondary = ""
			}
			if e.modelID == currentModel {
				label = fmt.Sprintf("[green]%s (current)[-]", formatModelLabel(e.modelID))
				currentIndex = i
			}
			list.AddItem(label, secondary, 0, nil)
		}
		list.SetCurrentItem(currentIndex)
	}

	updateTitle := func() {
		list.SetTitle(fmt.Sprintf(" Select Model [%s] [scope: %s] ", targetChannel(), scope))
	}

	scopeDesc := func() string {
		switch scope {
		case scopeSession:
			return "this session only (temporary)"
		case scopeChannel:
			return fmt.Sprintf("all %s sessions (persisted)", targetChannel())
		case scopeDefault:
			return "all channels (persisted)"
		}
		return ""
	}

	footer := tview.NewTextView().
		SetTextAlign(tview.AlignCenter).
		SetDynamicColors(true)

	updateFooter := func() {
		footer.Clear()
		fmt.Fprintf(footer, "[yellow]</>[-] channel: %s  |  [yellow]Tab[-] scope: %s  |  [yellow]Enter[-] apply  [yellow]Esc[-] cancel\n[gray]%s[-]", targetChannel(), scope, scopeDesc())
	}

	populateList()
	updateTitle()
	updateFooter()

	// Layout: list + footer (2 lines: controls + scope description)
	modalFlex := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(list, 0, 1, true).
		AddItem(footer, 2, 0, false)

	// Center the modal
	modal := tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(modalFlex, 16, 0, true).
			AddItem(nil, 0, 1, false),
			60, 0, true).
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

	switchChannel := func(delta int) {
		channelIdx = (channelIdx + delta + len(channels)) % len(channels)
		log.Printf("[model-picker] Switched to channel: %s (idx=%d)", targetChannel(), channelIdx)
		entries, currentModel, currentSource = buildEntries()
		_ = currentSource // used in footer via closure
		populateList()
		updateTitle()
		updateFooter()
	}

	list.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		debugLog("MODEL_PICKER key=%v rune=%c", event.Key(), event.Rune())
		switch event.Key() {
		case tcell.KeyEscape:
			debugLog("MODEL_PICKER: Esc → closePicker")
			closePicker()
			return nil
		case tcell.KeyLeft:
			debugLog("MODEL_PICKER: Left → switchChannel(-1)")
			switchChannel(-1)
			return nil
		case tcell.KeyRight:
			debugLog("MODEL_PICKER: Right → switchChannel(1)")
			switchChannel(1)
			return nil
		case tcell.KeyTab:
			scope = (scope + 1) % 3
			debugLog("MODEL_PICKER: Tab → scope=%s", scope)
			updateTitle()
			updateFooter()
			return nil
		case tcell.KeyEnter:
			idx := list.GetCurrentItem()
			debugLog("MODEL_PICKER: Enter idx=%d", idx)
			if idx >= 0 && idx < len(entries) {
				selected := entries[idx].modelID
				ch := targetChannel()
				sess := targetSession()
				var needSave bool
				debugLog("MODEL_PICKER: apply model=%s ch=%s scope=%s sess=%s", selected, ch, scope, sess)
				log.Printf("[model-picker] Applying model=%s channel=%s scope=%s session=%s", selected, ch, scope, sess)
				switch scope {
				case scopeSession:
					if sess != "" {
						a.modelRouter.SetSessionModel(sess, selected)
						log.Printf("[model-picker] Set session model: %s → %s", sess, selected)
					} else {
						// For non-TUI channels, apply as channel override
						a.modelRouter.SetChannelModel(ch, selected)
						a.config.Lock()
						if a.config.Agents.Models == nil {
							a.config.Agents.Models = make(map[string]string)
						}
						a.config.Agents.Models[ch] = selected
						a.config.Unlock()
						needSave = true
						log.Printf("[model-picker] Set channel model (from session scope): %s → %s", ch, selected)
					}
				case scopeChannel:
					a.modelRouter.SetChannelModel(ch, selected)
					a.config.Lock()
					if a.config.Agents.Models == nil {
						a.config.Agents.Models = make(map[string]string)
					}
					a.config.Agents.Models[ch] = selected
					a.config.Unlock()
					needSave = true
					log.Printf("[model-picker] Set channel model: %s → %s", ch, selected)
				case scopeDefault:
					a.modelRouter.SetDefaultModel(selected)
					a.config.Lock()
					a.config.Agents.Defaults.Model = selected
					a.config.Unlock()
					needSave = true
					log.Printf("[model-picker] Set default model: %s", selected)
				}
				closePicker()
				if needSave {
					a.asyncSaveAndRefresh()
				}
			} else {
				closePicker()
			}
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
