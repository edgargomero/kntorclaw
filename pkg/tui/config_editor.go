package tui

import (
	"fmt"
	"log"
	"reflect"
	"sort"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/sipeed/picoclaw/pkg/config"
)

// saveConfig persists the current config to disk.
func (a *App) saveConfig() {
	if a.configPath == "" {
		log.Println("[config] configPath not set, cannot save")
		return
	}
	if err := config.SaveConfig(a.configPath, a.config); err != nil {
		log.Printf("[config] Failed to save config: %v", err)
	}
}

// showConfigModal displays a modal overlay and returns a close function.
func (a *App) showConfigModal(modal tview.Primitive, focus tview.Primitive) func() {
	previousFocus := a.tviewApp.GetFocus()

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
		AddPage("modal", modal, true, true)

	a.modalOpen = true
	a.tviewApp.SetRoot(pages, true)
	a.tviewApp.SetFocus(focus)

	return func() {
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
}

// centerModal wraps a primitive in a centered flex layout.
func centerModal(inner tview.Primitive, width, height int) *tview.Flex {
	return tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(inner, height, 0, true).
			AddItem(nil, 0, 1, false),
			width, 0, true).
		AddItem(nil, 0, 1, false)
}

// getKnownModels returns a catalog of popular models for each configured provider.
func (a *App) getKnownModels() []string {
	cfg := a.config
	var models []string

	if cfg.Providers.Anthropic.APIKey != "" || cfg.Providers.Anthropic.AuthMethod != "" {
		models = append(models, "claude-sonnet-4-5-20250929", "claude-opus-4-6", "claude-haiku-4-5-20251001")
	}
	if cfg.Providers.OpenAI.APIKey != "" || cfg.Providers.OpenAI.AuthMethod != "" {
		models = append(models, "gpt-4o", "gpt-4o-mini", "gpt-4.1", "gpt-4.1-mini", "o3-mini")
	}
	if cfg.Providers.Gemini.APIKey != "" {
		models = append(models, "gemini-2.5-pro", "gemini-2.5-flash", "gemini-2.0-flash")
	}
	if cfg.Providers.Groq.APIKey != "" {
		models = append(models, "llama-3.3-70b-versatile", "llama-3.1-8b-instant", "mixtral-8x7b-32768")
	}
	if cfg.Providers.DeepSeek.APIKey != "" {
		models = append(models, "deepseek-chat", "deepseek-reasoner")
	}
	if cfg.Providers.Moonshot.APIKey != "" {
		models = append(models, "kimi-k2.5", "moonshot-v1-128k")
	}
	if cfg.Providers.Zhipu.APIKey != "" {
		models = append(models, "glm-4.7", "glm-4-plus", "glm-4-flash")
	}
	if cfg.Providers.Nvidia.APIKey != "" {
		models = append(models, "nvidia/llama-3.1-nemotron-70b-instruct")
	}
	if cfg.Providers.OpenRouter.APIKey != "" {
		models = append(models, "anthropic/claude-sonnet-4-5-20250929", "openai/gpt-4o", "google/gemini-2.5-pro", "meta-llama/llama-3.3-70b-instruct")
	}
	return models
}

// collectModelIDs returns a sorted list of known model IDs from aliases, current model, and provider catalogs.
func (a *App) collectModelIDs() []string {
	seen := make(map[string]bool)
	if a.modelRouter != nil {
		for _, modelID := range a.modelRouter.GetAliases() {
			seen[modelID] = true
		}
		model, _ := a.modelRouter.GetInfo("tui", "tui:local")
		seen[model] = true
		seen[a.modelRouter.DefaultModel()] = true
	}

	for _, m := range a.getKnownModels() {
		seen[m] = true
	}

	var models []string
	for m := range seen {
		if m != "" {
			models = append(models, m)
		}
	}
	sort.Strings(models)
	return models
}

// showDefaultModelPicker opens a list to change the default model, with a custom input field.
func (a *App) showDefaultModelPicker() {
	if a.modelRouter == nil {
		return
	}

	models := a.collectModelIDs()
	currentDefault := a.modelRouter.DefaultModel()

	applyModel := func(selected string, closeModal func()) {
		selected = strings.TrimSpace(selected)
		if selected == "" {
			return
		}
		a.modelRouter.SetDefaultModel(selected)
		a.config.Agents.Defaults.Model = selected
		a.saveConfig()
		a.renderConfig()
		log.Printf("[config] Default model changed to: %s", selected)
		closeModal()
	}

	list := tview.NewList()
	list.SetBorder(false)
	list.ShowSecondaryText(false)
	list.SetHighlightFullLine(true)
	list.SetSelectedBackgroundColor(tcell.ColorDarkBlue)

	currentIdx := 0
	for i, m := range models {
		label := m
		if m == currentDefault {
			label = fmt.Sprintf("[green]%s (current)[-]", m)
			currentIdx = i
		}
		list.AddItem(label, "", 0, nil)
	}
	if len(models) > 0 {
		list.SetCurrentItem(currentIdx)
	}

	customInput := tview.NewInputField().
		SetLabel("Custom: ").
		SetFieldWidth(40).
		SetFieldBackgroundColor(tcell.ColorDarkSlateGray)

	footer := tview.NewTextView().
		SetTextAlign(tview.AlignCenter).
		SetDynamicColors(true).
		SetText("[yellow]Enter[-] apply  [yellow]Esc[-] cancel  [yellow]Tab[-] custom input")

	container := tview.NewFlex().SetDirection(tview.FlexRow)
	container.SetBorder(true)
	container.SetTitle(" Select Default Model ")
	container.SetBorderColor(tcell.ColorBlue)

	if len(models) > 0 {
		container.AddItem(list, 0, 1, true)
	} else {
		hint := tview.NewTextView().
			SetTextAlign(tview.AlignCenter).
			SetDynamicColors(true).
			SetText("[gray]No models found. Configure provider API keys or type a custom model ID.[-]")
		container.AddItem(hint, 2, 0, false)
	}
	container.AddItem(customInput, 1, 0, false)
	container.AddItem(footer, 1, 0, false)

	modal := centerModal(container, 64, 20)
	closeModal := a.showConfigModal(modal, container)

	focusOnList := len(models) > 0

	list.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEscape:
			closeModal()
			return nil
		case tcell.KeyTab:
			a.tviewApp.SetFocus(customInput)
			focusOnList = false
			return nil
		case tcell.KeyEnter:
			idx := list.GetCurrentItem()
			if idx >= 0 && idx < len(models) {
				applyModel(models[idx], closeModal)
			}
			return nil
		}
		return event
	})

	customInput.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEscape:
			closeModal()
			return nil
		case tcell.KeyTab:
			if len(models) > 0 {
				a.tviewApp.SetFocus(list)
				focusOnList = true
			}
			return nil
		case tcell.KeyEnter:
			applyModel(customInput.GetText(), closeModal)
			return nil
		}
		return event
	})

	container.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			closeModal()
			return nil
		}
		return event
	})

	if focusOnList {
		a.tviewApp.SetFocus(list)
	} else {
		a.tviewApp.SetFocus(customInput)
	}
}

// showChannelModelEditor opens a two-step picker: first select channel, then select model.
func (a *App) showChannelModelEditor() {
	if a.modelRouter == nil {
		return
	}

	// Step 1: input channel name
	form := tview.NewForm()
	form.SetBorder(true)
	form.SetTitle(" Channel Model: Enter Channel Name ")
	form.SetBorderColor(tcell.ColorBlue)

	form.AddInputField("Channel", "", 30, nil, nil)

	modal := centerModal(form, 50, 7)
	closeModal := a.showConfigModal(modal, form)

	form.AddButton("Next", func() {
		channel := form.GetFormItemByLabel("Channel").(*tview.InputField).GetText()
		if channel == "" {
			return
		}
		closeModal()
		// Step 2: pick model from list
		a.showModelListForChannel(channel)
	})
	form.AddButton("Cancel", func() {
		closeModal()
	})

	form.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			closeModal()
			return nil
		}
		return event
	})
}

// showModelListForChannel shows a model list picker and assigns the selected model to a channel.
func (a *App) showModelListForChannel(channel string) {
	models := a.collectModelIDs()
	if len(models) == 0 {
		return
	}

	list := tview.NewList()
	list.SetBorder(true)
	list.SetTitle(fmt.Sprintf(" Select Model for [yellow]%s[-] ", channel))
	list.SetBorderColor(tcell.ColorBlue)
	list.ShowSecondaryText(false)
	list.SetHighlightFullLine(true)
	list.SetSelectedBackgroundColor(tcell.ColorDarkBlue)

	for _, m := range models {
		list.AddItem(m, "", 0, nil)
	}

	modal := centerModal(list, 60, 16)
	closeModal := a.showConfigModal(modal, list)

	list.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEscape:
			closeModal()
			return nil
		case tcell.KeyEnter:
			idx := list.GetCurrentItem()
			if idx >= 0 && idx < len(models) {
				selected := models[idx]
				a.modelRouter.SetChannelModel(channel, selected)
				if a.config.Agents.Models == nil {
					a.config.Agents.Models = make(map[string]string)
				}
				a.config.Agents.Models[channel] = selected
				a.saveConfig()
				a.renderConfig()
				log.Printf("[config] Channel model set: %s → %s", channel, selected)
			}
			closeModal()
			if a.configMode {
				a.refreshConfigItems(a.configSections.GetCurrentItem())
			}
			return nil
		}
		return event
	})
}

// showAliasEditor opens a form for alias name, then a list to pick the model ID.
func (a *App) showAliasEditor() {
	if a.modelRouter == nil {
		return
	}

	// Step 1: input alias name
	form := tview.NewForm()
	form.SetBorder(true)
	form.SetTitle(" Add/Edit Alias: Enter Alias Name ")
	form.SetBorderColor(tcell.ColorBlue)

	form.AddInputField("Alias", "", 30, nil, nil)

	modal := centerModal(form, 50, 7)
	closeModal := a.showConfigModal(modal, form)

	form.AddButton("Next", func() {
		alias := form.GetFormItemByLabel("Alias").(*tview.InputField).GetText()
		if alias == "" {
			return
		}
		closeModal()
		// Step 2: pick model from list
		a.showModelListForAlias(alias)
	})
	form.AddButton("Cancel", func() {
		closeModal()
	})

	form.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			closeModal()
			return nil
		}
		return event
	})
}

// showModelListForAlias shows a model list picker and assigns it as an alias.
func (a *App) showModelListForAlias(alias string) {
	models := a.collectModelIDs()
	if len(models) == 0 {
		return
	}

	list := tview.NewList()
	list.SetBorder(true)
	list.SetTitle(fmt.Sprintf(" Select Model for alias [yellow]%s[-] ", alias))
	list.SetBorderColor(tcell.ColorBlue)
	list.ShowSecondaryText(false)
	list.SetHighlightFullLine(true)
	list.SetSelectedBackgroundColor(tcell.ColorDarkBlue)

	for _, m := range models {
		list.AddItem(m, "", 0, nil)
	}

	modal := centerModal(list, 60, 16)
	closeModal := a.showConfigModal(modal, list)

	list.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEscape:
			closeModal()
			return nil
		case tcell.KeyEnter:
			idx := list.GetCurrentItem()
			if idx >= 0 && idx < len(models) {
				selected := models[idx]
				a.modelRouter.SetAlias(alias, selected)
				if a.config.Agents.Aliases == nil {
					a.config.Agents.Aliases = make(map[string]string)
				}
				a.config.Agents.Aliases[alias] = selected
				a.saveConfig()
				a.renderConfig()
				log.Printf("[config] Alias set: %s → %s", alias, selected)
			}
			closeModal()
			if a.configMode {
				a.refreshConfigItems(a.configSections.GetCurrentItem())
			}
			return nil
		}
		return event
	})
}

// showProviderKeyEditor opens a list of providers to select, then an input for the API key.
func (a *App) showProviderKeyEditor() {
	type providerEntry struct {
		name   string
		hasKey bool
	}

	providers := a.getProviderList()

	list := tview.NewList()
	list.SetBorder(true)
	list.SetTitle(" Select Provider to Edit API Key ")
	list.SetBorderColor(tcell.ColorYellow)
	list.ShowSecondaryText(true)
	list.SetHighlightFullLine(true)
	list.SetSelectedBackgroundColor(tcell.ColorDarkBlue)

	for _, p := range providers {
		status := "[red]not set[-]"
		if p.hasKey {
			status = "[green]configured[-]"
		}
		list.AddItem(p.name, "  "+status, 0, nil)
	}

	modal := centerModal(list, 50, 20)
	closeModal := a.showConfigModal(modal, list)

	list.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEscape:
			closeModal()
			return nil
		case tcell.KeyEnter:
			idx := list.GetCurrentItem()
			if idx >= 0 && idx < len(providers) {
				selected := providers[idx].name
				closeModal()
				a.showAPIKeyInput(selected)
			}
			return nil
		}
		return event
	})
}

// providerEntry holds provider display info.
type providerEntry struct {
	name   string
	hasKey bool
}

// getProviderList returns all providers with their configuration status.
func (a *App) getProviderList() []providerEntry {
	cfg := a.config
	return []providerEntry{
		{"anthropic", cfg.Providers.Anthropic.APIKey != ""},
		{"openai", cfg.Providers.OpenAI.APIKey != ""},
		{"openrouter", cfg.Providers.OpenRouter.APIKey != ""},
		{"gemini", cfg.Providers.Gemini.APIKey != ""},
		{"groq", cfg.Providers.Groq.APIKey != ""},
		{"zhipu", cfg.Providers.Zhipu.APIKey != ""},
		{"deepseek", cfg.Providers.DeepSeek.APIKey != ""},
		{"nvidia", cfg.Providers.Nvidia.APIKey != ""},
		{"moonshot", cfg.Providers.Moonshot.APIKey != ""},
		{"shengsuanyun", cfg.Providers.ShengSuanYun.APIKey != ""},
		{"vllm", cfg.Providers.VLLM.APIBase != ""},
		{"github_copilot", cfg.Providers.GitHubCopilot.APIKey != ""},
	}
}

// showAPIKeyInput opens a form to input an API key for a specific provider.
func (a *App) showAPIKeyInput(providerName string) {
	form := tview.NewForm()
	form.SetBorder(true)
	form.SetTitle(fmt.Sprintf(" API Key for [yellow]%s[-] ", providerName))
	form.SetBorderColor(tcell.ColorYellow)

	// Show current masked key
	currentKey := a.getProviderKey(providerName)
	hint := ""
	if currentKey != "" {
		if len(currentKey) > 8 {
			hint = currentKey[:4] + strings.Repeat("*", len(currentKey)-8) + currentKey[len(currentKey)-4:]
		} else {
			hint = strings.Repeat("*", len(currentKey))
		}
	}

	fieldLabel := "API Key"
	if providerName == "vllm" {
		fieldLabel = "API Base URL"
	}

	if hint != "" {
		form.AddTextView("Current", hint, 50, 1, false, false)
	}
	form.AddInputField(fieldLabel, "", 50, nil, nil)

	modal := centerModal(form, 60, 10)
	closeModal := a.showConfigModal(modal, form)

	form.AddButton("Save", func() {
		newKey := form.GetFormItemByLabel(fieldLabel).(*tview.InputField).GetText()
		if newKey == "" {
			return
		}
		a.setProviderKey(providerName, newKey)
		a.saveConfig()
		a.renderConfig()
		closeModal()
		if a.configMode {
			a.refreshConfigItems(a.configSections.GetCurrentItem())
		}
		log.Printf("[config] Provider key updated: %s", providerName)
	})

	form.AddButton("Cancel", func() {
		closeModal()
	})

	form.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			closeModal()
			return nil
		}
		return event
	})
}

// getProviderKey returns the current API key (or API base for vllm) for a provider.
func (a *App) getProviderKey(name string) string {
	p := a.getProviderConfig(name)
	if p == nil {
		return ""
	}
	if name == "vllm" {
		return p.APIBase
	}
	return p.APIKey
}

// setProviderKey sets the API key (or API base for vllm) for a provider.
func (a *App) setProviderKey(name, key string) {
	p := a.getProviderConfig(name)
	if p == nil {
		return
	}
	if name == "vllm" {
		p.APIBase = key
	} else {
		p.APIKey = key
	}
}

// getProviderConfig returns a pointer to the ProviderConfig for a given provider name.
func (a *App) getProviderConfig(name string) *config.ProviderConfig {
	v := reflect.ValueOf(&a.config.Providers).Elem()
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		tag := t.Field(i).Tag.Get("json")
		if tag == name {
			return v.Field(i).Addr().Interface().(*config.ProviderConfig)
		}
	}
	return nil
}

// showConfigDeletePicker opens a list of channel models and aliases to delete.
func (a *App) showConfigDeletePicker() {
	if a.modelRouter == nil {
		return
	}

	channelModels := a.modelRouter.GetChannelModels()
	aliases := a.modelRouter.GetAliases()

	if len(channelModels) == 0 && len(aliases) == 0 {
		return
	}

	type deleteEntry struct {
		kind string // "channel" or "alias"
		key  string
		val  string
	}

	var entries []deleteEntry

	chKeys := make([]string, 0, len(channelModels))
	for k := range channelModels {
		chKeys = append(chKeys, k)
	}
	sort.Strings(chKeys)
	for _, k := range chKeys {
		entries = append(entries, deleteEntry{kind: "channel", key: k, val: channelModels[k]})
	}

	aliasKeys := make([]string, 0, len(aliases))
	for k := range aliases {
		aliasKeys = append(aliasKeys, k)
	}
	sort.Strings(aliasKeys)
	for _, k := range aliasKeys {
		entries = append(entries, deleteEntry{kind: "alias", key: k, val: aliases[k]})
	}

	list := tview.NewList()
	list.SetBorder(true)
	list.SetTitle(" Delete Entry (Enter to delete, Esc to cancel) ")
	list.SetBorderColor(tcell.ColorRed)
	list.ShowSecondaryText(true)
	list.SetHighlightFullLine(true)
	list.SetSelectedBackgroundColor(tcell.ColorDarkRed)

	for _, e := range entries {
		label := fmt.Sprintf("[%s] %s", e.kind, e.key)
		secondary := fmt.Sprintf("  → %s", e.val)
		list.AddItem(label, secondary, 0, nil)
	}

	modal := centerModal(list, 60, 16)
	closeModal := a.showConfigModal(modal, list)

	list.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEscape:
			closeModal()
			return nil
		case tcell.KeyEnter:
			idx := list.GetCurrentItem()
			if idx >= 0 && idx < len(entries) {
				e := entries[idx]
				switch e.kind {
				case "channel":
					a.modelRouter.DeleteChannelModel(e.key)
					delete(a.config.Agents.Models, e.key)
					log.Printf("[config] Deleted channel model: %s", e.key)
				case "alias":
					a.modelRouter.DeleteAlias(e.key)
					delete(a.config.Agents.Aliases, e.key)
					log.Printf("[config] Deleted alias: %s", e.key)
				}
				a.saveConfig()
				a.renderConfig()
			}
			closeModal()
			return nil
		}
		return event
	})
}
