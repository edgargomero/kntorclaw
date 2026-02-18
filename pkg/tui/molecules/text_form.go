package molecules

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/sipeed/picoclaw/pkg/tui/atoms"
)

// FormSubmitMsg is emitted when the user presses Enter in a text form.
type FormSubmitMsg struct{ Values map[string]string }

// FormCancelMsg is emitted when the user presses Esc in a text form.
type FormCancelMsg struct{}

// FormField describes a single field in a text form.
type FormField struct {
	Label       string
	Placeholder string
	Value       string
	EchoMode    textinput.EchoMode
}

// TextFormModel is a multi-field form with Tab navigation.
type TextFormModel struct {
	title   string
	fields  []FormField
	inputs  []textinput.Model
	focused int
	Visible bool
}

// NewTextForm creates a TextFormModel with the given title and fields.
func NewTextForm(title string, fields []FormField) TextFormModel {
	inputs := make([]textinput.Model, len(fields))
	for i, f := range fields {
		ti := textinput.New()
		ti.Placeholder = f.Placeholder
		ti.SetValue(f.Value)
		ti.EchoMode = f.EchoMode
		ti.Width = 40
		if i == 0 {
			ti.Focus()
		}
		inputs[i] = ti
	}
	return TextFormModel{
		title:  title,
		fields: fields,
		inputs: inputs,
	}
}

// Update handles Tab/Shift+Tab navigation, Enter submit, and Esc cancel.
func (m TextFormModel) Update(msg tea.Msg) (TextFormModel, tea.Cmd) {
	if !m.Visible {
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "tab", "down":
			m.inputs[m.focused].Blur()
			m.focused = (m.focused + 1) % len(m.inputs)
			return m, m.inputs[m.focused].Focus()
		case "shift+tab", "up":
			m.inputs[m.focused].Blur()
			m.focused = (m.focused - 1 + len(m.inputs)) % len(m.inputs)
			return m, m.inputs[m.focused].Focus()
		case "enter":
			values := make(map[string]string)
			for i, f := range m.fields {
				values[f.Label] = m.inputs[i].Value()
			}
			m.Visible = false
			return m, func() tea.Msg { return FormSubmitMsg{Values: values} }
		case "esc":
			m.Visible = false
			return m, func() tea.Msg { return FormCancelMsg{} }
		}
	}

	var cmd tea.Cmd
	m.inputs[m.focused], cmd = m.inputs[m.focused].Update(msg)
	return m, cmd
}

// View renders the form with labels, inputs, and hints.
func (m TextFormModel) View() string {
	if !m.Visible {
		return ""
	}

	var b strings.Builder
	titleStyle := lipgloss.NewStyle().Foreground(atoms.ColorYellow).Bold(true)
	labelStyle := lipgloss.NewStyle().Foreground(atoms.ColorCyan)

	b.WriteString(titleStyle.Render(m.title) + "\n\n")
	for i, f := range m.fields {
		b.WriteString(labelStyle.Render(f.Label) + "\n")
		b.WriteString(m.inputs[i].View() + "\n\n")
	}
	b.WriteString("Enter: submit | Esc: cancel")

	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(atoms.ColorCyan).
		Padding(1, 2)

	return style.Render(b.String())
}

// Show makes the form visible and focuses the first input.
func (m *TextFormModel) Show() {
	m.Visible = true
	if len(m.inputs) > 0 {
		m.inputs[0].Focus()
	}
}
