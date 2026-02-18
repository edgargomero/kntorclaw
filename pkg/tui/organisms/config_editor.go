package organisms

import (
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/sipeed/picoclaw/pkg/tui/molecules"
)

// ConfigSavedMsg indicates config was saved with the given values.
type ConfigSavedMsg struct {
	Section string
	Key     string
	Values  map[string]string
}

// EditorType identifies which kind of config editor is open.
type EditorType int

const (
	EditorNone EditorType = iota
	EditorProvider
	EditorModel
	EditorChannel
	EditorAlias
	EditorGeneric
)

// ConfigEditorModel wraps a TextFormModel for editing different config types.
type ConfigEditorModel struct {
	form       molecules.TextFormModel
	editorType EditorType
	section    string
	key        string
	Visible    bool
	callbacks  *ConfigCallbacks
}

// NewConfigEditor creates a ConfigEditorModel with no active editor.
func NewConfigEditor() ConfigEditorModel {
	return ConfigEditorModel{}
}

// SetCallbacks sets the persistence callbacks.
func (m *ConfigEditorModel) SetCallbacks(cb *ConfigCallbacks) { m.callbacks = cb }

// OpenProvider opens the provider editor with fields from the config item.
// If fields is nil, all values default to empty.
func (m *ConfigEditorModel) OpenProvider(key string, fields map[string]string) {
	m.editorType = EditorProvider
	m.section = "Providers"
	m.key = key
	if fields == nil {
		fields = make(map[string]string)
	}
	// Fields come from callbacks with keys "api_key" and "api_base"
	apiKey := fields["api_key"]
	if apiKey == "" {
		apiKey = fields["API Key"]
	}
	apiBase := fields["api_base"]
	if apiBase == "" {
		apiBase = fields["API Base"]
	}
	m.form = molecules.NewTextForm("Edit Provider: "+key, []molecules.FormField{
		{Label: "api_key", Placeholder: "Enter API key...", Value: apiKey, EchoMode: textinput.EchoPassword},
		{Label: "api_base", Placeholder: "Enter API base URL...", Value: apiBase},
	})
	m.form.Show()
	m.Visible = true
}

// OpenModel opens the model name editor.
func (m *ConfigEditorModel) OpenModel(key string, currentModel string) {
	m.editorType = EditorModel
	m.section = "Models"
	m.key = key
	m.form = molecules.NewTextForm("Edit Model: "+key, []molecules.FormField{
		{Label: "Model Name", Placeholder: "e.g. claude-sonnet-4-5-20250929", Value: currentModel},
	})
	m.form.Show()
	m.Visible = true
}

// OpenChannel opens the channel model editor.
func (m *ConfigEditorModel) OpenChannel(key string, currentModel string) {
	m.editorType = EditorChannel
	m.section = "Channel Models"
	m.key = key
	title := "Edit Channel Model"
	if key != "" {
		title += ": " + key
	} else {
		title = "Add Channel Model"
	}
	fields := []molecules.FormField{}
	if key == "" {
		fields = append(fields, molecules.FormField{Label: "Channel", Placeholder: "e.g. telegram, discord, whatsapp"})
	}
	fields = append(fields, molecules.FormField{Label: "Model", Placeholder: "e.g. claude-sonnet-4-5-20250929@anthropic", Value: currentModel})
	m.form = molecules.NewTextForm(title, fields)
	m.form.Show()
	m.Visible = true
}

// OpenAlias opens the alias model editor.
func (m *ConfigEditorModel) OpenAlias(key string, currentValue string) {
	m.editorType = EditorAlias
	m.section = "Aliases"
	m.key = key
	title := "Edit Alias"
	if key != "" {
		title += ": " + key
	} else {
		title = "Add Alias"
	}
	fields := []molecules.FormField{}
	if key == "" {
		fields = append(fields, molecules.FormField{Label: "Alias", Placeholder: "e.g. sonnet, opus, haiku"})
	}
	fields = append(fields, molecules.FormField{Label: "Model", Placeholder: "e.g. claude-sonnet-4-5-20250929@anthropic", Value: currentValue})
	m.form = molecules.NewTextForm(title, fields)
	m.form.Show()
	m.Visible = true
}

// OpenGeneric opens a generic key-value editor for any section.
func (m *ConfigEditorModel) OpenGeneric(section, key, label, value string) {
	m.editorType = EditorGeneric
	m.section = section
	m.key = key
	m.form = molecules.NewTextForm("Edit "+section+": "+key, []molecules.FormField{
		{Label: label, Placeholder: "Enter value...", Value: value},
	})
	m.form.Show()
	m.Visible = true
}

// Update processes form submit/cancel messages and forwards input to the form.
func (m ConfigEditorModel) Update(msg tea.Msg) (ConfigEditorModel, tea.Cmd) {
	if !m.Visible {
		return m, nil
	}

	switch msg := msg.(type) {
	case molecules.FormSubmitMsg:
		m.Visible = false
		section := m.section
		key := m.key
		values := msg.Values
		// Call the appropriate save callback
		m.invokeSave(key, values)
		return m, func() tea.Msg {
			return ConfigSavedMsg{Section: section, Key: key, Values: values}
		}
	case molecules.FormCancelMsg:
		m.Visible = false
		return m, nil
	}

	var cmd tea.Cmd
	m.form, cmd = m.form.Update(msg)
	return m, cmd
}

// invokeSave calls the appropriate save callback based on editor type.
func (m *ConfigEditorModel) invokeSave(key string, values map[string]string) {
	if m.callbacks == nil {
		return
	}
	switch m.editorType {
	case EditorProvider:
		if m.callbacks.SaveProvider != nil {
			_ = m.callbacks.SaveProvider(key, values)
		}
	case EditorModel:
		if m.callbacks.SaveModel != nil {
			_ = m.callbacks.SaveModel(values["Model Name"])
		}
	case EditorChannel:
		if m.callbacks.SaveChannelModel != nil {
			// If key is empty, get it from the Channel form field
			ch := key
			if ch == "" {
				ch = values["Channel"]
			}
			_ = m.callbacks.SaveChannelModel(ch, values["Model"])
		}
	case EditorAlias:
		if m.callbacks.SaveAlias != nil {
			// If key is empty, get it from the Alias form field
			alias := key
			if alias == "" {
				alias = values["Alias"]
			}
			_ = m.callbacks.SaveAlias(alias, values["Model"])
		}
	}
}

// View renders the form editor overlay.
func (m ConfigEditorModel) View() string {
	if !m.Visible {
		return ""
	}
	return m.form.View()
}
