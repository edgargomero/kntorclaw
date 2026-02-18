package tui

import "github.com/charmbracelet/bubbles/key"

// KeyMap defines all application keybindings
type KeyMap struct {
	Panel1    key.Binding
	Panel2    key.Binding
	Panel3    key.Binding
	Panel4    key.Binding
	Panel5    key.Binding
	Panel6    key.Binding
	Config    key.Binding
	Focus     key.Binding
	NextPanel key.Binding
	PrevPanel key.Binding
	ModelPick key.Binding
	Errors    key.Binding
	Help      key.Binding
	Enter     key.Binding
	Escape    key.Binding
	Delete    key.Binding
	Quit      key.Binding
}

var Keys = KeyMap{
	Panel1:    key.NewBinding(key.WithKeys("f1"), key.WithHelp("F1", "panel 1")),
	Panel2:    key.NewBinding(key.WithKeys("f2"), key.WithHelp("F2", "panel 2")),
	Panel3:    key.NewBinding(key.WithKeys("f3"), key.WithHelp("F3", "panel 3")),
	Panel4:    key.NewBinding(key.WithKeys("f4"), key.WithHelp("F4", "panel 4")),
	Panel5:    key.NewBinding(key.WithKeys("f5"), key.WithHelp("F5", "panel 5")),
	Panel6:    key.NewBinding(key.WithKeys("f6"), key.WithHelp("F6", "panel 6")),
	Config:    key.NewBinding(key.WithKeys("f8"), key.WithHelp("F8", "config mode")),
	Focus:     key.NewBinding(key.WithKeys("f9"), key.WithHelp("F9", "focus mode")),
	NextPanel: key.NewBinding(key.WithKeys("tab"), key.WithHelp("Tab", "next panel")),
	PrevPanel: key.NewBinding(key.WithKeys("shift+tab"), key.WithHelp("Shift+Tab", "prev panel")),
	ModelPick: key.NewBinding(key.WithKeys("alt+m"), key.WithHelp("Alt+M", "model picker")),
	Errors:    key.NewBinding(key.WithKeys("ctrl+e"), key.WithHelp("Ctrl+E", "error viewer")),
	Help:      key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
	Enter:     key.NewBinding(key.WithKeys("enter"), key.WithHelp("Enter", "confirm")),
	Escape:    key.NewBinding(key.WithKeys("esc"), key.WithHelp("Esc", "back/close")),
	Delete:    key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "delete")),
	Quit:      key.NewBinding(key.WithKeys("ctrl+c"), key.WithHelp("Ctrl+C", "quit")),
}

// ShortHelp returns key bindings for the short help view
func (k KeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Help, k.Quit}
}

// FullHelp returns key bindings for the full help view
func (k KeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Panel1, k.Panel2, k.Panel3, k.Panel4, k.Panel5, k.Panel6},
		{k.Config, k.Focus, k.ModelPick, k.Errors},
		{k.NextPanel, k.PrevPanel, k.Enter, k.Escape},
		{k.Help, k.Quit},
	}
}
