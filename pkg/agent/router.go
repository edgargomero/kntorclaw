package agent

import (
	"sync"
	"sync/atomic"
)

// ModelRouter resolves which LLM model to use based on a 3-layer priority:
//   1. Session override (runtime, set via /model command)
//   2. Channel override (from config agents.models map)
//   3. Default model (from config agents.defaults.model)
//
// Uses atomic.Value and sync.Map for lock-free reads, avoiding deadlocks
// between the tview event loop (writes) and agent goroutines (reads).
type ModelRouter struct {
	defaultModel  atomic.Value // stores string
	channelModels sync.Map     // string → string, config-persisted
	sessionModels sync.Map     // string → string, runtime-only
	aliases       sync.Map     // string → string, "opus" → "claude-opus-4-6"
}

// NewModelRouter creates a ModelRouter from config values.
func NewModelRouter(defaultModel string, channelModels, aliases map[string]string) *ModelRouter {
	r := &ModelRouter{}
	r.defaultModel.Store(defaultModel)
	for k, v := range channelModels {
		r.channelModels.Store(k, v)
	}
	for k, v := range aliases {
		r.aliases.Store(k, v)
	}
	return r
}

// Resolve returns the model to use, checking session → channel → default.
func (r *ModelRouter) Resolve(channel, sessionKey string) string {
	if sessionKey != "" {
		if m, ok := r.sessionModels.Load(sessionKey); ok {
			return m.(string)
		}
	}
	if channel != "" {
		if m, ok := r.channelModels.Load(channel); ok {
			return m.(string)
		}
	}
	return r.defaultModel.Load().(string)
}

// ResolveAlias expands an alias to a full model ID. If no alias matches,
// returns the input unchanged (treating it as a full model ID).
func (r *ModelRouter) ResolveAlias(input string) string {
	if m, ok := r.aliases.Load(input); ok {
		return m.(string)
	}
	return input
}

// SetSessionModel sets a per-session model override (runtime only).
func (r *ModelRouter) SetSessionModel(sessionKey, model string) {
	r.sessionModels.Store(sessionKey, model)
}

// ClearSessionModel removes a session model override.
func (r *ModelRouter) ClearSessionModel(sessionKey string) {
	r.sessionModels.Delete(sessionKey)
}

// SetChannelModel sets a per-channel model override.
func (r *ModelRouter) SetChannelModel(channel, model string) {
	r.channelModels.Store(channel, model)
}

// SetDefaultModel changes the global default model.
func (r *ModelRouter) SetDefaultModel(model string) {
	r.defaultModel.Store(model)
}

// GetInfo returns the resolved model and which layer it came from.
func (r *ModelRouter) GetInfo(channel, sessionKey string) (model, source string) {
	if sessionKey != "" {
		if m, ok := r.sessionModels.Load(sessionKey); ok {
			return m.(string), "session override"
		}
	}
	if channel != "" {
		if m, ok := r.channelModels.Load(channel); ok {
			return m.(string), "channel override"
		}
	}
	return r.defaultModel.Load().(string), "default"
}

// GetAliases returns a copy of the aliases map.
func (r *ModelRouter) GetAliases() map[string]string {
	cp := make(map[string]string)
	r.aliases.Range(func(key, value interface{}) bool {
		cp[key.(string)] = value.(string)
		return true
	})
	return cp
}

// DefaultModel returns the current default model.
func (r *ModelRouter) DefaultModel() string {
	return r.defaultModel.Load().(string)
}

// GetChannelModels returns a copy of the channel models map.
func (r *ModelRouter) GetChannelModels() map[string]string {
	cp := make(map[string]string)
	r.channelModels.Range(func(key, value interface{}) bool {
		cp[key.(string)] = value.(string)
		return true
	})
	return cp
}

// SetAlias adds or updates an alias mapping.
func (r *ModelRouter) SetAlias(alias, modelID string) {
	r.aliases.Store(alias, modelID)
}

// DeleteAlias removes an alias.
func (r *ModelRouter) DeleteAlias(alias string) {
	r.aliases.Delete(alias)
}

// DeleteChannelModel removes a channel model override.
func (r *ModelRouter) DeleteChannelModel(channel string) {
	r.channelModels.Delete(channel)
}
