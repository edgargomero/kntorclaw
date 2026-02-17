package tui

import (
	"fmt"
	"log"
	"reflect"
	"sort"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/sipeed/picoclaw/pkg/auth"
	"github.com/sipeed/picoclaw/pkg/config"
	"github.com/sipeed/picoclaw/pkg/providers"
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

// asyncSaveAndRefresh saves config and refreshes the UI synchronously.
// Must be called from the tview event loop (InputCapture handlers, button callbacks, etc.).
// Previous async version caused deadlocks: the save goroutine held config.RLock while
// a new config edit on the event loop tried config.Lock, freezing the UI.
func (a *App) asyncSaveAndRefresh() {
	a.saveConfig()
	a.renderConfig()
	if a.configMode {
		a.refreshConfigItems(a.configSections.GetCurrentItem())
	}
	a.showToast("[green]Config saved[-]")
}

// showConfigModal displays a modal overlay and returns a close function.
func (a *App) showConfigModal(modal tview.Primitive, focus tview.Primitive) func() {
	previousFocus := a.tviewApp.GetFocus()
	debugLog("MODAL OPEN: focus=%T previousFocus=%T configMode=%v", focus, previousFocus, a.configMode)

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
	debugLog("MODAL OPEN done: modalOpen=%v actualFocus=%T", a.modalOpen, a.tviewApp.GetFocus())

	return func() {
		debugLog("MODAL CLOSE: restoring previousFocus=%T configMode=%v", previousFocus, a.configMode)
		a.modalOpen = false
		if a.configMode {
			a.tviewApp.SetRoot(a.configLayout, true)
		} else if a.focusMode {
			a.tviewApp.SetRoot(a.focusLayout, true)
		} else {
			a.tviewApp.SetRoot(a.normalLayout, true)
		}
		a.tviewApp.SetFocus(previousFocus)
		debugLog("MODAL CLOSE done: modalOpen=%v actualFocus=%T", a.modalOpen, a.tviewApp.GetFocus())
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
		models = append(models,
			"claude-sonnet-4-5-20250929@anthropic",
			"claude-opus-4-6@anthropic",
			"claude-haiku-4-5-20251001@anthropic",
		)
	}
	if cfg.Providers.Anthropic.AuthMethod != "" {
		models = append(models,
			"claude-sonnet-4-5-20250929@anthropic-cc",
			"claude-opus-4-6@anthropic-cc",
			"claude-haiku-4-5-20251001@anthropic-cc",
		)
	}
	if cfg.Providers.OpenAI.APIKey != "" || cfg.Providers.OpenAI.AuthMethod != "" {
		models = append(models,
			"gpt-4o@openai", "gpt-4o-mini@openai",
			"gpt-4.1@openai", "gpt-4.1-mini@openai", "o3-mini@openai",
		)
	}
	if cfg.Providers.Gemini.APIKey != "" {
		models = append(models,
			"gemini-2.5-pro@gemini", "gemini-2.5-flash@gemini", "gemini-2.0-flash@gemini",
		)
	}
	if cfg.Providers.Groq.APIKey != "" {
		models = append(models,
			"llama-3.3-70b-versatile@groq", "llama-3.1-8b-instant@groq", "mixtral-8x7b-32768@groq",
		)
	}
	if cfg.Providers.DeepSeek.APIKey != "" {
		models = append(models, "deepseek-chat@deepseek", "deepseek-reasoner@deepseek")
	}
	if cfg.Providers.Moonshot.APIKey != "" {
		models = append(models, "kimi-k2.5@moonshot", "moonshot-v1-128k@moonshot")
	}
	if cfg.Providers.Zhipu.APIKey != "" {
		models = append(models, "glm-4.7@zhipu", "glm-4-plus@zhipu", "glm-4-flash@zhipu")
	}
	if cfg.Providers.Nvidia.APIKey != "" {
		models = append(models,
			"nvidia/llama-3.1-nemotron-70b-instruct@nvidia", "z-ai/glm5@nvidia",
		)
	}
	if cfg.Providers.OpenRouter.APIKey != "" {
		models = append(models,
			"anthropic/claude-sonnet-4-5-20250929@openrouter",
			"openai/gpt-4o@openrouter",
			"google/gemini-2.5-pro@openrouter",
			"meta-llama/llama-3.3-70b-instruct@openrouter",
		)
	}
	return models
}

// collectModelIDs returns a sorted list of known model IDs from aliases, current model, and provider catalogs.
func (a *App) collectModelIDs() []string {
	debugLog("collectModelIDs: start")
	seen := make(map[string]bool)
	if a.modelRouter != nil {
		debugLog("collectModelIDs: GetAliases...")
		for _, modelID := range a.modelRouter.GetAliases() {
			seen[modelID] = true
		}
		debugLog("collectModelIDs: GetInfo...")
		model, _ := a.modelRouter.GetInfo("tui", "tui:local")
		seen[model] = true
		debugLog("collectModelIDs: DefaultModel...")
		seen[a.modelRouter.DefaultModel()] = true
		debugLog("collectModelIDs: getKnownModels...")
	}

	for _, m := range a.getKnownModels() {
		seen[m] = true
	}
	debugLog("collectModelIDs: done, count=%d", len(seen))

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
		a.config.Lock()
		a.config.Agents.Defaults.Model = selected
		a.config.Unlock()
		log.Printf("[config] Default model changed to: %s", selected)
		closeModal()
		a.asyncSaveAndRefresh()
	}

	list := tview.NewList()
	list.SetBorder(false)
	list.ShowSecondaryText(false)
	list.SetHighlightFullLine(true)
	list.SetSelectedBackgroundColor(tcell.ColorDarkBlue)

	currentIdx := 0
	for i, m := range models {
		label := formatModelLabel(m)
		if m == currentDefault {
			label = fmt.Sprintf("[green]%s (current)[-]", formatModelLabel(m))
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
			text := strings.TrimSpace(customInput.GetText())
			if text == "" {
				return nil
			}
			_, prov := providers.ParseModelID(text)
			if prov != "" {
				// Already has @provider, apply directly
				applyModel(text, closeModal)
			} else {
				// No @provider — show provider picker
				closeModal()
				a.showProviderPickerForCustomModel(text, func(modelWithProvider string) {
					a.modelRouter.SetDefaultModel(modelWithProvider)
					a.config.Lock()
					a.config.Agents.Defaults.Model = modelWithProvider
					a.config.Unlock()
					log.Printf("[config] Default model changed to: %s", modelWithProvider)
					a.asyncSaveAndRefresh()
				})
			}
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

// showChannelModelEditor opens a two-step picker: first select channel from list, then select model.
func (a *App) showChannelModelEditor() {
	if a.modelRouter == nil {
		return
	}

	// Step 1: pick channel from list of available channels
	channels := a.getActiveChannels()

	list := tview.NewList()
	list.SetBorder(true)
	list.SetTitle(" Select Channel ")
	list.SetBorderColor(tcell.ColorGreen)
	list.ShowSecondaryText(true)
	list.SetHighlightFullLine(true)
	list.SetSelectedBackgroundColor(tcell.ColorDarkBlue)

	channelModels := a.modelRouter.GetChannelModels()
	for _, ch := range channels {
		secondary := "  [gray]no model set[-]"
		if m, ok := channelModels[ch]; ok {
			secondary = fmt.Sprintf("  [green]%s[-]", m)
		}
		list.AddItem(ch, secondary, 0, nil)
	}

	modal := centerModal(list, 50, 16)
	closeModal := a.showConfigModal(modal, list)

	list.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEscape:
			closeModal()
			return nil
		case tcell.KeyEnter:
			idx := list.GetCurrentItem()
			if idx >= 0 && idx < len(channels) {
				selected := channels[idx]
				closeModal()
				a.showModelListForChannel(selected)
			}
			return nil
		}
		return event
	})
}

// showModelListForChannel shows a model list picker and assigns the selected model to a channel.
func (a *App) showModelListForChannel(channel string) {
	log.Printf("[config] showModelListForChannel called for: %s", channel)
	models := a.collectModelIDs()
	if len(models) == 0 {
		log.Printf("[config] No models found, returning")
		return
	}
	log.Printf("[config] Models available: %d", len(models))

	list := tview.NewList()
	list.SetBorder(true)
	list.SetTitle(fmt.Sprintf(" Select Model for [yellow]%s[-] ", channel))
	list.SetBorderColor(tcell.ColorBlue)
	list.ShowSecondaryText(false)
	list.SetHighlightFullLine(true)
	list.SetSelectedBackgroundColor(tcell.ColorDarkBlue)

	for _, m := range models {
		list.AddItem(formatModelLabel(m), "", 0, nil)
	}

	modal := centerModal(list, 60, 16)
	closeModal := a.showConfigModal(modal, list)
	log.Printf("[config] Modal opened for channel: %s, modalOpen=%v", channel, a.modalOpen)

	list.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		log.Printf("[config] Model list key: %v rune=%c", event.Key(), event.Rune())
		switch event.Key() {
		case tcell.KeyEscape:
			log.Printf("[config] Model list: Esc pressed, closing")
			closeModal()
			return nil
		case tcell.KeyEnter:
			idx := list.GetCurrentItem()
			log.Printf("[config] Model list: Enter pressed, idx=%d", idx)
			if idx >= 0 && idx < len(models) {
				selected := models[idx]
				log.Printf("[config] Applying: %s → %s", channel, selected)
				a.modelRouter.SetChannelModel(channel, selected)
				a.config.Lock()
				if a.config.Agents.Models == nil {
					a.config.Agents.Models = make(map[string]string)
				}
				a.config.Agents.Models[channel] = selected
				a.config.Unlock()
				log.Printf("[config] Channel model set: %s → %s", channel, selected)
			}
			log.Printf("[config] Closing modal...")
			closeModal()
			log.Printf("[config] Modal closed, saving...")
			a.asyncSaveAndRefresh()
			log.Printf("[config] asyncSaveAndRefresh called")
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
		list.AddItem(formatModelLabel(m), "", 0, nil)
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
				a.config.Lock()
				if a.config.Agents.Aliases == nil {
					a.config.Agents.Aliases = make(map[string]string)
				}
				a.config.Agents.Aliases[alias] = selected
				a.config.Unlock()
				log.Printf("[config] Alias set: %s → %s", alias, selected)
			}
			closeModal()
			a.asyncSaveAndRefresh()
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
		{"anthropic-cc", cfg.Providers.Anthropic.AuthMethod != ""},
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

// showAuthProviderInfo shows a config modal for auth-based providers (anthropic-cc).
// Displays current credential status and allows pasting a setup-token from Claude Code.
func (a *App) showAuthProviderInfo(providerName string) {
	cred, _ := auth.GetCredential("anthropic")

	container := tview.NewFlex().SetDirection(tview.FlexRow)
	container.SetBorder(true)
	container.SetTitle(fmt.Sprintf(" [yellow]%s[-] — Claude Code Setup Token ", providerName))
	container.SetBorderColor(tcell.ColorBlue)

	// Status section
	status := tview.NewTextView().
		SetDynamicColors(true).
		SetWordWrap(true)

	if cred != nil && cred.AccessToken != "" {
		maskedToken := ""
		if len(cred.AccessToken) > 12 {
			maskedToken = cred.AccessToken[:6] + "..." + cred.AccessToken[len(cred.AccessToken)-4:]
		} else {
			maskedToken = "***"
		}
		expiresInfo := "n/a"
		if !cred.ExpiresAt.IsZero() {
			expiresInfo = cred.ExpiresAt.Format("2006-01-02 15:04")
		}
		fmt.Fprintf(status, " [green]● Connected[-]  method:[yellow]%s[-]  token:[gray]%s[-]  expires:%s", cred.AuthMethod, maskedToken, expiresInfo)
	} else {
		fmt.Fprint(status, " [red]● Not configured[-]")
	}

	// Instructions
	instructions := tview.NewTextView().
		SetDynamicColors(true).
		SetWordWrap(true)
	fmt.Fprint(instructions, " Run in Claude Code CLI:  [yellow]claude setup-token[-]    then paste the token below:")

	// Token input
	tokenInput := tview.NewInputField().
		SetLabel(" Token: ").
		SetFieldWidth(60).
		SetFieldBackgroundColor(tcell.ColorDarkSlateGray).
		SetPlaceholder("sk-ant-oat01-...")

	// Footer
	footer := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter).
		SetText("[yellow]Enter[-] save token  [yellow]Esc[-] cancel")

	container.AddItem(status, 1, 0, false)
	container.AddItem(instructions, 2, 0, false)
	container.AddItem(tokenInput, 1, 0, true)
	container.AddItem(footer, 1, 0, false)

	modal := centerModal(container, 72, 9)
	closeModal := a.showConfigModal(modal, tokenInput)

	tokenInput.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEscape:
			closeModal()
			return nil
		case tcell.KeyEnter:
			token := strings.TrimSpace(tokenInput.GetText())
			if token == "" {
				return nil
			}
			// Validate prefix
			if !strings.HasPrefix(token, "sk-ant-oat01-") {
				status.Clear()
				fmt.Fprint(status, " [red]Invalid token[-] — must start with [yellow]sk-ant-oat01-[-]")
				return nil
			}
			if len(token) < 80 {
				status.Clear()
				fmt.Fprint(status, " [red]Invalid token[-] — too short (min 80 chars)")
				return nil
			}
			// Save credential
			newCred := &auth.AuthCredential{
				AccessToken: token,
				Provider:    "anthropic",
				AuthMethod:  "setup-token",
			}
			if err := auth.SetCredential("anthropic", newCred); err != nil {
				status.Clear()
				fmt.Fprintf(status, " [red]Error saving[-]: %v", err)
				return nil
			}
			// Update config auth_method
			a.config.Lock()
			a.config.Providers.Anthropic.AuthMethod = "setup-token"
			a.config.Unlock()
			log.Printf("[config] anthropic-cc: setup-token saved successfully")
			closeModal()
			a.asyncSaveAndRefresh()
			a.showToast("[green]anthropic-cc configured[-]")
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
		log.Printf("[config] Provider key updated: %s", providerName)
		closeModal()
		a.asyncSaveAndRefresh()
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
	a.config.Lock()
	defer a.config.Unlock()
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
					a.config.Lock()
					delete(a.config.Agents.Models, e.key)
					a.config.Unlock()
					log.Printf("[config] Deleted channel model: %s", e.key)
				case "alias":
					a.modelRouter.DeleteAlias(e.key)
					a.config.Lock()
					delete(a.config.Agents.Aliases, e.key)
					a.config.Unlock()
					log.Printf("[config] Deleted alias: %s", e.key)
				}
			}
			closeModal()
			a.asyncSaveAndRefresh()
			return nil
		}
		return event
	})
}

// formatModelLabel formats a model ID for display in pickers.
// "claude-sonnet-4-5-20250929@anthropic" → "[anthropic] claude-sonnet-4-5-20250929"
// "gpt-4o" → "gpt-4o"
func formatModelLabel(modelID string) string {
	model, prov := providers.ParseModelID(modelID)
	if prov != "" {
		return fmt.Sprintf("[yellow]%s[-] %s", prov, model)
	}
	return model
}

// showProviderPickerForCustomModel opens a provider picker after the user types a custom model without @provider.
// Once a provider is selected, it calls the apply callback with the full model@provider string.
func (a *App) showProviderPickerForCustomModel(model string, apply func(string)) {
	providerList := a.getProviderList()

	// Only show configured providers
	var configured []providerEntry
	for _, p := range providerList {
		if p.hasKey {
			configured = append(configured, p)
		}
	}

	if len(configured) == 0 {
		// No providers configured, just use the model as-is
		apply(model)
		return
	}

	list := tview.NewList()
	list.SetBorder(true)
	list.SetTitle(fmt.Sprintf(" Select Provider for [yellow]%s[-] ", model))
	list.SetBorderColor(tcell.ColorYellow)
	list.ShowSecondaryText(false)
	list.SetHighlightFullLine(true)
	list.SetSelectedBackgroundColor(tcell.ColorDarkBlue)

	for _, p := range configured {
		list.AddItem(p.name, "", 0, nil)
	}

	modal := centerModal(list, 40, 12)
	closeModal := a.showConfigModal(modal, list)

	list.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEscape:
			closeModal()
			return nil
		case tcell.KeyEnter:
			idx := list.GetCurrentItem()
			if idx >= 0 && idx < len(configured) {
				selected := providers.FormatModelID(model, configured[idx].name)
				closeModal()
				apply(selected)
			}
			return nil
		}
		return event
	})
}
