package organisms

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/sipeed/picoclaw/pkg/tui/atoms"
)

// ModelSelectedMsg is emitted when the user picks a model.
type ModelSelectedMsg struct {
	Model   string
	Channel string
	Scope   string // "session", "channel", "default"
}

// ModelScope determines where the selected model applies.
type ModelScope int

const (
	ScopeSession ModelScope = iota
	ScopeChannel
	ScopeDefault
)

func (s ModelScope) String() string {
	switch s {
	case ScopeSession:
		return "session"
	case ScopeChannel:
		return "channel"
	case ScopeDefault:
		return "default"
	default:
		return "session"
	}
}

// ModelPickerModel is a modal overlay for selecting LLM models.
type ModelPickerModel struct {
	Visible      bool
	models       []string          // available model names
	channels     []string          // active channel names
	aliases      map[string]string // alias -> full model name
	reverseAlias map[string]string // full model name -> alias
	currentModel string            // currently active model

	cursor     int        // cursor position in model list
	channelIdx int        // current channel index
	scope      ModelScope // "session", "channel", "default"

	customInput textinput.Model // for typing custom model names
	focusCustom bool            // whether custom input has focus

	width  int
	height int
}

// NewModelPicker creates a ModelPickerModel with the given models and channels.
func NewModelPicker(models, channels []string) ModelPickerModel {
	ti := textinput.New()
	ti.Placeholder = "Type custom model name..."
	ti.CharLimit = 128
	ti.Width = 44

	return ModelPickerModel{
		models:       models,
		channels:     channels,
		aliases:      make(map[string]string),
		reverseAlias: make(map[string]string),
		customInput:  ti,
	}
}

// SetModels replaces the list of available models.
func (m *ModelPickerModel) SetModels(models []string) { m.models = models }

// SetChannels replaces the list of available channels.
func (m *ModelPickerModel) SetChannels(channels []string) { m.channels = channels }

// SetData sets all picker data at once.
func (m *ModelPickerModel) SetData(models []string, channels []string, currentModel string, aliases map[string]string) {
	m.models = models
	m.channels = channels
	m.currentModel = currentModel
	m.aliases = aliases
	m.reverseAlias = make(map[string]string)
	for alias, full := range aliases {
		m.reverseAlias[full] = alias
	}
}

// Show makes the picker visible, resets cursor, and unfocuses custom input.
func (m *ModelPickerModel) Show() {
	m.Visible = true
	m.cursor = 0
	m.focusCustom = false
	m.customInput.Blur()
	m.customInput.SetValue("")

	// Position cursor at the current model if found
	for i, model := range m.models {
		if model == m.currentModel {
			m.cursor = i
			break
		}
	}
}

// Hide closes the picker.
func (m *ModelPickerModel) Hide() {
	m.Visible = false
	m.focusCustom = false
	m.customInput.Blur()
}

// Update handles key events for the model picker.
func (m ModelPickerModel) Update(msg tea.Msg) (ModelPickerModel, tea.Cmd) {
	if !m.Visible {
		return m, nil
	}

	// When custom input is focused, route most keys there
	if m.focusCustom {
		return m.updateCustomInput(msg)
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if len(m.models) > 0 {
				m.cursor--
				if m.cursor < 0 {
					m.cursor = len(m.models) - 1
				}
			}
		case "down", "j":
			if len(m.models) > 0 {
				m.cursor = (m.cursor + 1) % len(m.models)
			}
		case "left", "h":
			if len(m.channels) > 1 {
				m.channelIdx = (m.channelIdx - 1 + len(m.channels)) % len(m.channels)
			}
		case "right", "l":
			if len(m.channels) > 1 {
				m.channelIdx = (m.channelIdx + 1) % len(m.channels)
			}
		case "tab":
			m.scope = (m.scope + 1) % 3
		case "/", "i":
			m.focusCustom = true
			m.customInput.Focus()
			return m, textinput.Blink
		case "enter":
			return m.selectCurrent()
		case "esc":
			m.Visible = false
			return m, nil
		}
	}
	return m, nil
}

// updateCustomInput handles input while the custom text field is focused.
func (m ModelPickerModel) updateCustomInput(msg tea.Msg) (ModelPickerModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			// Return focus to the list
			m.focusCustom = false
			m.customInput.Blur()
			return m, nil
		case "enter":
			value := strings.TrimSpace(m.customInput.Value())
			if value != "" {
				// Resolve alias if applicable
				if full, ok := m.aliases[value]; ok {
					value = full
				}
				channel := m.currentChannel()
				m.Visible = false
				m.focusCustom = false
				m.customInput.Blur()
				return m, func() tea.Msg {
					return ModelSelectedMsg{
						Model:   value,
						Channel: channel,
						Scope:   m.scope.String(),
					}
				}
			}
			return m, nil
		}
	}

	// Forward to textinput
	var cmd tea.Cmd
	m.customInput, cmd = m.customInput.Update(msg)
	return m, cmd
}

// selectCurrent emits a ModelSelectedMsg for the currently highlighted model.
func (m ModelPickerModel) selectCurrent() (ModelPickerModel, tea.Cmd) {
	if len(m.models) == 0 || m.cursor >= len(m.models) {
		return m, nil
	}
	selected := m.models[m.cursor]
	channel := m.currentChannel()
	m.Visible = false
	return m, func() tea.Msg {
		return ModelSelectedMsg{
			Model:   selected,
			Channel: channel,
			Scope:   m.scope.String(),
		}
	}
}

// currentChannel returns the name of the currently selected channel.
func (m ModelPickerModel) currentChannel() string {
	if len(m.channels) > 0 && m.channelIdx < len(m.channels) {
		return m.channels[m.channelIdx]
	}
	return ""
}

// View renders the model picker as a centered modal overlay.
func (m ModelPickerModel) View(screenWidth, screenHeight int) string {
	if !m.Visible {
		return ""
	}

	titleStyle := lipgloss.NewStyle().Foreground(atoms.ColorYellow).Bold(true)
	selectedStyle := lipgloss.NewStyle().Foreground(atoms.ColorCyan).Bold(true)
	currentStyle := lipgloss.NewStyle().Foreground(atoms.ColorGreen).Bold(true)
	aliasStyle := lipgloss.NewStyle().Foreground(atoms.ColorGray)
	dimStyle := lipgloss.NewStyle().Faint(true)
	scopeStyle := lipgloss.NewStyle().Foreground(atoms.ColorGreen)

	var b strings.Builder

	// Title with channel indicator
	channelName := "default"
	if len(m.channels) > 0 && m.channelIdx < len(m.channels) {
		channelName = m.channels[m.channelIdx]
	}

	header := "Model Picker"
	if len(m.channels) > 1 {
		header = fmt.Sprintf("Model Picker — %s", channelName)
	} else if len(m.channels) == 1 {
		header = fmt.Sprintf("Model Picker — %s", channelName)
	}
	b.WriteString(titleStyle.Render(header) + "\n")

	if len(m.channels) > 1 {
		b.WriteString(dimStyle.Render("  ←/→ switch channel") + "\n")
	}

	// Scope line
	b.WriteString(dimStyle.Render("Scope: ") + scopeStyle.Render("["+m.scope.String()+"]"))
	b.WriteString("  " + dimStyle.Render("(Tab to change)") + "\n\n")

	// Model list
	maxVisible := screenHeight - 14
	if maxVisible < 5 {
		maxVisible = 5
	}

	startIdx := 0
	if len(m.models) > maxVisible {
		// Scroll to keep cursor visible
		if m.cursor >= maxVisible {
			startIdx = m.cursor - maxVisible + 1
		}
		if startIdx+maxVisible > len(m.models) {
			startIdx = len(m.models) - maxVisible
		}
		if startIdx < 0 {
			startIdx = 0
		}
	}

	endIdx := startIdx + maxVisible
	if endIdx > len(m.models) {
		endIdx = len(m.models)
	}

	if startIdx > 0 {
		b.WriteString(dimStyle.Render("  ↑ more") + "\n")
	}

	for i := startIdx; i < endIdx; i++ {
		model := m.models[i]
		isCurrent := model == m.currentModel
		isCursor := i == m.cursor

		// Build the model display string
		display := model
		if alias, ok := m.reverseAlias[model]; ok {
			display = model + " " + aliasStyle.Render("("+alias+")")
		}

		switch {
		case isCursor && isCurrent:
			b.WriteString(currentStyle.Render("→ "+display) + "\n")
		case isCursor:
			b.WriteString(selectedStyle.Render("> "+display) + "\n")
		case isCurrent:
			b.WriteString(currentStyle.Render("  "+display) + "\n")
		default:
			b.WriteString("  " + display + "\n")
		}
	}

	if endIdx < len(m.models) {
		b.WriteString(dimStyle.Render("  ↓ more") + "\n")
	}

	// Custom input section
	b.WriteString("\n")
	if m.focusCustom {
		b.WriteString(selectedStyle.Render("Custom: ") + m.customInput.View() + "\n")
	} else {
		b.WriteString(dimStyle.Render("Press / or i for custom model input") + "\n")
	}

	// Footer
	b.WriteString("\n" + dimStyle.Render("Enter: apply | Esc: cancel"))

	// Wrap in modal box
	modalWidth := 58
	contentLines := strings.Count(b.String(), "\n") + 1
	modalHeight := contentLines + 4 // padding
	if modalHeight > screenHeight-4 {
		modalHeight = screenHeight - 4
	}

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(atoms.ColorCyan).
		Padding(1, 2).
		Width(modalWidth).
		Height(modalHeight)

	rendered := boxStyle.Render(b.String())

	// Center on screen
	return placeOverlay(screenWidth, screenHeight, rendered)
}

// placeOverlay centers the given content on a screen of the specified dimensions.
func placeOverlay(screenW, screenH int, content string) string {
	lines := strings.Split(content, "\n")
	contentH := len(lines)
	contentW := 0
	for _, l := range lines {
		w := lipgloss.Width(l)
		if w > contentW {
			contentW = w
		}
	}

	padTop := (screenH - contentH) / 2
	padLeft := (screenW - contentW) / 2
	if padTop < 0 {
		padTop = 0
	}
	if padLeft < 0 {
		padLeft = 0
	}

	var out strings.Builder
	for i := 0; i < padTop; i++ {
		out.WriteString("\n")
	}
	prefix := strings.Repeat(" ", padLeft)
	for _, l := range lines {
		out.WriteString(prefix + l + "\n")
	}
	return out.String()
}
