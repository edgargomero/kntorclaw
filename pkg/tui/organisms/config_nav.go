package organisms

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/sipeed/picoclaw/pkg/tui/atoms"
)

// Config navigation messages
type ConfigSectionSelectedMsg struct{ Section string }
type ConfigItemSelectedMsg struct{ Section, Key string }
type ConfigItemDeleteMsg struct{ Section, Key string }
type ConfigDeletedMsg struct{ Section, Key string }
type ConfigExitMsg struct{}

// ConfigItem represents a single item within a config section.
type ConfigItem struct {
	Key     string
	Value   string
	Summary string
	Fields  map[string]string // current values for editing (optional)
}

// ConfigNavModel is a 2-panel navigation for config sections and items.
type ConfigNavModel struct {
	sections     []string
	items        []ConfigItem
	sectionIdx   int
	itemIdx      int
	focusedPanel int // 0=sections, 1=items
	width        int
	height       int
	focused      bool
	callbacks    *ConfigCallbacks
}

// NewConfigNav creates a ConfigNavModel with the standard config sections.
func NewConfigNav() ConfigNavModel {
	return ConfigNavModel{
		sections: []string{"Providers", "Default Model", "Channel Models", "Aliases"},
	}
}

// SetItems replaces the items list and resets the item cursor.
func (m *ConfigNavModel) SetItems(items []ConfigItem) {
	m.items = items
	m.itemIdx = 0
}

// GetSelectedSection returns the currently highlighted section name.
func (m *ConfigNavModel) GetSelectedSection() string {
	if m.sectionIdx < len(m.sections) {
		return m.sections[m.sectionIdx]
	}
	return ""
}

// Update handles keyboard input for navigation.
func (m ConfigNavModel) Update(msg tea.Msg) (ConfigNavModel, tea.Cmd) {
	if !m.focused {
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.focusedPanel == 0 {
				if m.sectionIdx > 0 {
					m.sectionIdx--
				}
			} else {
				if m.itemIdx > 0 {
					m.itemIdx--
				}
			}
		case "down", "j":
			if m.focusedPanel == 0 {
				if m.sectionIdx < len(m.sections)-1 {
					m.sectionIdx++
				}
			} else {
				if m.itemIdx < len(m.items)-1 {
					m.itemIdx++
				}
			}
		case "tab", "shift+tab":
			m.focusedPanel = (m.focusedPanel + 1) % 2
		case "esc":
			if m.focusedPanel == 1 {
				m.focusedPanel = 0
			} else {
				// On sections panel, Esc should exit config mode — bubble up
				return m, func() tea.Msg { return ConfigExitMsg{} }
			}
		case "enter":
			if m.focusedPanel == 0 {
				section := m.sections[m.sectionIdx]
				return m, func() tea.Msg { return ConfigSectionSelectedMsg{Section: section} }
			} else if len(m.items) > 0 && m.itemIdx < len(m.items) {
				section := m.sections[m.sectionIdx]
				key := m.items[m.itemIdx].Key
				return m, func() tea.Msg { return ConfigItemSelectedMsg{Section: section, Key: key} }
			}
		case "d":
			if m.focusedPanel == 1 && len(m.items) > 0 && m.itemIdx < len(m.items) {
				section := m.sections[m.sectionIdx]
				key := m.items[m.itemIdx].Key
				return m, func() tea.Msg { return ConfigItemDeleteMsg{Section: section, Key: key} }
			}
		}
	}
	return m, nil
}

// View renders both the sections panel and the items panel side by side.
func (m ConfigNavModel) View() string {
	sectionsView := m.renderSections()
	itemsView := m.renderItems()
	return lipgloss.JoinHorizontal(lipgloss.Top, sectionsView, itemsView)
}

func (m ConfigNavModel) renderSections() string {
	var b strings.Builder
	selectedStyle := lipgloss.NewStyle().Foreground(atoms.ColorCyan).Bold(true)
	normalStyle := lipgloss.NewStyle().Foreground(atoms.ColorWhite)

	for i, s := range m.sections {
		if i == m.sectionIdx {
			b.WriteString(selectedStyle.Render("> "+s) + "\n")
		} else {
			b.WriteString(normalStyle.Render("  "+s) + "\n")
		}
	}
	return atoms.RenderPanel("SECTIONS", m.focused && m.focusedPanel == 0, 25, m.height, b.String())
}

func (m ConfigNavModel) renderItems() string {
	w := m.width - 27 // 25 for sections + 2 for border
	if w < 20 {
		w = 20
	}

	if len(m.items) == 0 {
		return atoms.RenderPanel("ITEMS", m.focused && m.focusedPanel == 1, w, m.height,
			lipgloss.NewStyle().Faint(true).Render("  Select a section and press Enter"))
	}

	var b strings.Builder
	selectedStyle := lipgloss.NewStyle().Foreground(atoms.ColorCyan).Bold(true)
	keyStyle := lipgloss.NewStyle().Foreground(atoms.ColorYellow)
	valStyle := lipgloss.NewStyle().Faint(true)

	for i, item := range m.items {
		if i == m.itemIdx {
			b.WriteString(selectedStyle.Render("> " + item.Key))
		} else {
			b.WriteString("  " + keyStyle.Render(item.Key))
		}
		if item.Summary != "" {
			b.WriteString(" " + valStyle.Render(item.Summary))
		}
		b.WriteString("\n")
	}

	b.WriteString("\n" + lipgloss.NewStyle().Faint(true).Render("Enter: edit | d: delete | Tab: switch panel"))
	return atoms.RenderPanel("ITEMS", m.focused && m.focusedPanel == 1, w, m.height, b.String())
}

// SetCallbacks sets the data loading and persistence callbacks.
func (m *ConfigNavModel) SetCallbacks(cb *ConfigCallbacks) { m.callbacks = cb }

// GetSelectedItem returns the currently highlighted item, or nil if none.
func (m *ConfigNavModel) GetSelectedItem() *ConfigItem {
	if m.itemIdx < len(m.items) {
		return &m.items[m.itemIdx]
	}
	return nil
}

// LoadSection loads items for the given section using callbacks.
func (m *ConfigNavModel) LoadSection(section string) {
	if m.callbacks == nil {
		m.SetItems(nil)
		return
	}
	items := m.callbacks.loadSection(section)
	if items != nil {
		m.SetItems(items)
	} else {
		m.SetItems(nil)
	}
	m.focusedPanel = 1
}

// SetSize updates the panel dimensions.
func (m *ConfigNavModel) SetSize(w, h int) { m.width = w; m.height = h }

// SetFocused sets the focused state for border styling.
func (m *ConfigNavModel) SetFocused(f bool) { m.focused = f }
