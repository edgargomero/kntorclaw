package agent

import (
	"sync"
)

// ModelRouter resolves which LLM model to use based on a 3-layer priority:
//   1. Session override (runtime, set via /model command)
//   2. Channel override (from config agents.models map)
//   3. Default model (from config agents.defaults.model)
type ModelRouter struct {
	mu            sync.RWMutex
	defaultModel  string
	channelModels map[string]string // config-persisted, e.g. "telegram" → "claude-haiku-4-5-20251001"
	sessionModels map[string]string // runtime-only, lost on restart
	aliases       map[string]string // "opus" → "claude-opus-4-6"
}

// NewModelRouter creates a ModelRouter from config values.
func NewModelRouter(defaultModel string, channelModels, aliases map[string]string) *ModelRouter {
	if channelModels == nil {
		channelModels = make(map[string]string)
	}
	if aliases == nil {
		aliases = make(map[string]string)
	}
	return &ModelRouter{
		defaultModel:  defaultModel,
		channelModels: channelModels,
		sessionModels: make(map[string]string),
		aliases:       aliases,
	}
}

// Resolve returns the model to use, checking session → channel → default.
func (r *ModelRouter) Resolve(channel, sessionKey string) string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if sessionKey != "" {
		if m, ok := r.sessionModels[sessionKey]; ok {
			return m
		}
	}
	if channel != "" {
		if m, ok := r.channelModels[channel]; ok {
			return m
		}
	}
	return r.defaultModel
}

// ResolveAlias expands an alias to a full model ID. If no alias matches,
// returns the input unchanged (treating it as a full model ID).
func (r *ModelRouter) ResolveAlias(input string) string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if m, ok := r.aliases[input]; ok {
		return m
	}
	return input
}

// SetSessionModel sets a per-session model override (runtime only).
func (r *ModelRouter) SetSessionModel(sessionKey, model string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sessionModels[sessionKey] = model
}

// ClearSessionModel removes a session model override.
func (r *ModelRouter) ClearSessionModel(sessionKey string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.sessionModels, sessionKey)
}

// SetChannelModel sets a per-channel model override.
func (r *ModelRouter) SetChannelModel(channel, model string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.channelModels[channel] = model
}

// SetDefaultModel changes the global default model.
func (r *ModelRouter) SetDefaultModel(model string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.defaultModel = model
}

// GetInfo returns the resolved model and which layer it came from.
func (r *ModelRouter) GetInfo(channel, sessionKey string) (model, source string) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if sessionKey != "" {
		if m, ok := r.sessionModels[sessionKey]; ok {
			return m, "session override"
		}
	}
	if channel != "" {
		if m, ok := r.channelModels[channel]; ok {
			return m, "channel override"
		}
	}
	return r.defaultModel, "default"
}

// GetAliases returns a copy of the aliases map.
func (r *ModelRouter) GetAliases() map[string]string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	cp := make(map[string]string, len(r.aliases))
	for k, v := range r.aliases {
		cp[k] = v
	}
	return cp
}

// DefaultModel returns the current default model.
func (r *ModelRouter) DefaultModel() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.defaultModel
}

// GetChannelModels returns a copy of the channel models map.
func (r *ModelRouter) GetChannelModels() map[string]string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	cp := make(map[string]string, len(r.channelModels))
	for k, v := range r.channelModels {
		cp[k] = v
	}
	return cp
}

// SetAlias adds or updates an alias mapping.
func (r *ModelRouter) SetAlias(alias, modelID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.aliases[alias] = modelID
}

// DeleteAlias removes an alias.
func (r *ModelRouter) DeleteAlias(alias string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.aliases, alias)
}

// DeleteChannelModel removes a channel model override.
func (r *ModelRouter) DeleteChannelModel(channel string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.channelModels, channel)
}
