package pages

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/sipeed/picoclaw/pkg/tui/atoms"
	"github.com/sipeed/picoclaw/pkg/tui/domain"
	"github.com/sipeed/picoclaw/pkg/tui/molecules"
	"github.com/sipeed/picoclaw/pkg/tui/organisms"
	"github.com/sipeed/picoclaw/pkg/tui/templates"
)

// ConfigPageModel composes the config navigation, editor, and confirm dialog
// into a full config mode page.
type ConfigPageModel struct {
	nav       organisms.ConfigNavModel
	editor    organisms.ConfigEditorModel
	confirm   molecules.ConfirmModel
	callbacks *organisms.ConfigCallbacks

	errorTracker *domain.ErrorTracker
	width        int
	height       int

	pendingDelete struct {
		section string
		key     string
	}
}

// SetErrorTracker sets the error tracker for status bar badge rendering.
func (m *ConfigPageModel) SetErrorTracker(et *domain.ErrorTracker) {
	m.errorTracker = et
}

// SetConfigCallbacks propagates data loading and persistence callbacks
// to the nav and editor sub-components.
func (m *ConfigPageModel) SetConfigCallbacks(cb *organisms.ConfigCallbacks) {
	m.callbacks = cb
	m.nav.SetCallbacks(cb)
	m.editor.SetCallbacks(cb)
}

// NewConfigPage creates a ConfigPageModel with the given dimensions.
func NewConfigPage(width, height int) ConfigPageModel {
	nav := organisms.NewConfigNav()
	nav.SetSize(width, height-2) // 1 for status bar, 1 for footer
	nav.SetFocused(true)
	return ConfigPageModel{
		nav:    nav,
		editor: organisms.NewConfigEditor(),
		width:  width,
		height: height,
	}
}

// Init returns nil; no initialization commands needed.
func (m ConfigPageModel) Init() tea.Cmd { return nil }

// Update processes messages, routing to editor/confirm when active, otherwise to nav.
func (m ConfigPageModel) Update(msg tea.Msg) (ConfigPageModel, tea.Cmd) {
	// Editor takes priority if visible
	if m.editor.Visible {
		var cmd tea.Cmd
		m.editor, cmd = m.editor.Update(msg)
		return m, cmd
	}

	// Confirm dialog takes priority
	if m.confirm.Visible {
		var cmd tea.Cmd
		m.confirm, cmd = m.confirm.Update(msg)
		return m, cmd
	}

	switch msg := msg.(type) {
	case organisms.ConfigSectionSelectedMsg:
		// Load items for the selected section via callbacks
		m.nav.LoadSection(msg.Section)
		return m, nil

	case organisms.ConfigItemSelectedMsg:
		// Find the selected item to pre-fill the editor with current values
		item := m.nav.GetSelectedItem()
		fields := make(map[string]string)
		if item != nil && item.Fields != nil {
			fields = item.Fields
		}
		// Open appropriate editor based on section
		switch msg.Section {
		case "Providers":
			m.editor.OpenProvider(msg.Key, fields)
		case "Default Model":
			currentModel := ""
			if item != nil {
				currentModel = item.Value
			}
			m.editor.OpenModel(msg.Key, currentModel)
		case "Channel Models":
			currentToken := ""
			if item != nil {
				currentToken = item.Value
			}
			m.editor.OpenChannel(msg.Key, currentToken)
		case "Aliases":
			currentValue := ""
			if item != nil {
				currentValue = item.Value
			}
			m.editor.OpenAlias(msg.Key, currentValue)
		default:
			currentValue := ""
			if item != nil {
				currentValue = item.Value
			}
			m.editor.OpenGeneric(msg.Section, msg.Key, "Value", currentValue)
		}
		return m, nil

	case organisms.ConfigAddItemMsg:
		// Open editor with empty values for a new item
		switch msg.Section {
		case "Providers":
			// Providers are predefined; open editor for a provider to configure its key
			m.editor.OpenProvider("new-provider", nil)
		case "Channel Models":
			m.editor.OpenChannel("", "")
		case "Aliases":
			m.editor.OpenAlias("", "")
		case "Default Model":
			m.editor.OpenModel("default", "")
		}
		return m, nil

	case organisms.ConfigItemDeleteMsg:
		m.pendingDelete.section = msg.Section
		m.pendingDelete.key = msg.Key
		m.confirm = molecules.NewConfirm("Delete " + msg.Key + " from " + msg.Section + "?")
		m.confirm.SetSize(m.width, m.height)
		m.confirm.Show()
		return m, nil

	case molecules.ConfirmResultMsg:
		if msg.Confirmed && m.pendingDelete.key != "" {
			section := m.pendingDelete.section
			key := m.pendingDelete.key
			m.pendingDelete.section = ""
			m.pendingDelete.key = ""
			// Call delete callback
			if m.callbacks != nil && m.callbacks.DeleteItem != nil {
				_ = m.callbacks.DeleteItem(section, key)
			}
			// Refresh items after deletion
			m.nav.LoadSection(section)
			return m, func() tea.Msg {
				return organisms.ConfigDeletedMsg{Section: section, Key: key}
			}
		}
		m.pendingDelete.section = ""
		m.pendingDelete.key = ""
		return m, nil

	case organisms.ConfigSavedMsg:
		// Refresh items after save to reflect changes
		section := m.nav.GetSelectedSection()
		if section != "" {
			m.nav.LoadSection(section)
		}
		return m, nil
	}

	// Forward to nav
	var cmd tea.Cmd
	m.nav, cmd = m.nav.Update(msg)
	return m, cmd
}

// View renders the config page with nav panels and any active overlays.
func (m ConfigPageModel) View() string {
	footer := "F8: exit config | Tab: switch panel | Enter: edit | a: add | d: delete"

	base := templates.RenderConfigLayout(m.width, m.height, templates.ConfigLayoutPanels{
		StatusBar: m.renderStatusBar(),
		Sections:  m.nav.View(),
		Footer:    footer,
	})

	// Overlay editor centered on screen
	if m.editor.Visible {
		overlay := m.editor.View()
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, overlay,
			lipgloss.WithWhitespaceChars(" "),
			lipgloss.WithWhitespaceForeground(lipgloss.Color("#000000")),
		)
	}
	if m.confirm.Visible {
		return m.confirm.View()
	}

	return base
}

func (m ConfigPageModel) renderStatusBar() string {
	modeStyle := lipgloss.NewStyle().Foreground(atoms.ColorCyan).Bold(true)
	hintStyle := lipgloss.NewStyle().Faint(true)

	parts := []string{modeStyle.Render("[CONFIG]")}

	if m.errorTracker != nil {
		if badge := atoms.RenderBadge(m.errorTracker.Count()); badge != "" {
			parts = append(parts, badge)
		}
	}

	parts = append(parts, hintStyle.Render("Tab:Switch Enter:Edit a:Add d:Delete Esc:Back F8:Exit"))

	content := strings.Join(parts, " ")
	return atoms.StatusBarStyle.Width(m.width).Render(content)
}

// HasActiveOverlay returns true if the editor or confirm dialog is visible.
func (m *ConfigPageModel) HasActiveOverlay() bool {
	return m.editor.Visible || m.confirm.Visible
}

// GetSelectedSection returns the currently highlighted section name.
func (m *ConfigPageModel) GetSelectedSection() string {
	return m.nav.GetSelectedSection()
}

// LoadSelectedSection loads items for the currently highlighted section.
func (m *ConfigPageModel) LoadSelectedSection() {
	section := m.nav.GetSelectedSection()
	if section != "" {
		m.nav.LoadSection(section)
	}
}

// SetSize updates the page dimensions and propagates to sub-components.
func (m *ConfigPageModel) SetSize(w, h int) {
	m.width = w
	m.height = h
	m.nav.SetSize(w, h-2) // 1 for status bar, 1 for footer
}
