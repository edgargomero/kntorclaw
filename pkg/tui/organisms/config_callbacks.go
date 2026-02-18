package organisms

// ConfigCallbacks provides data loading and persistence hooks for config UI.
// All fields are optional — nil callbacks are safely skipped.
type ConfigCallbacks struct {
	// Data loaders
	GetProviders     func() []ConfigItem
	GetModels        func() []ConfigItem
	GetChannelModels func() []ConfigItem
	GetAliases       func() []ConfigItem
	GetKnownModels   func() []string // provider-aware model catalog

	// Actions
	SaveProvider     func(name string, fields map[string]string) error
	SaveModel        func(name string) error
	SaveChannelModel func(channel, model string) error
	SaveAlias        func(alias, model string) error
	DeleteItem       func(section, key string) error
}

// loadSection calls the appropriate data loader for the given section name.
// Returns nil if no callback is registered for the section.
func (cb *ConfigCallbacks) loadSection(section string) []ConfigItem {
	if cb == nil {
		return nil
	}
	switch section {
	case "Providers":
		if cb.GetProviders != nil {
			return cb.GetProviders()
		}
	case "Default Model":
		if cb.GetModels != nil {
			return cb.GetModels()
		}
	case "Channel Models":
		if cb.GetChannelModels != nil {
			return cb.GetChannelModels()
		}
	case "Aliases":
		if cb.GetAliases != nil {
			return cb.GetAliases()
		}
	}
	return nil
}
