package providers

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/sipeed/picoclaw/pkg/config"
)

// MultiProvider implements LLMProvider and dispatches each Chat() call
// to the correct underlying provider based on the model name.
// It caches providers to avoid recreating them on every request.
type MultiProvider struct {
	cfg   *config.Config
	mu    sync.RWMutex
	cache map[string]LLMProvider // cacheKey → provider
}

// NewMultiProvider creates a new MultiProvider that dynamically routes
// requests to the correct provider based on model name.
func NewMultiProvider(cfg *config.Config) *MultiProvider {
	return &MultiProvider{
		cfg:   cfg,
		cache: make(map[string]LLMProvider),
	}
}

// Chat dispatches the request to the correct provider for the given model.
func (mp *MultiProvider) Chat(ctx context.Context, messages []Message, tools []ToolDefinition, model string, options map[string]interface{}) (*LLMResponse, error) {
	provider, err := mp.getProvider(model)
	if err != nil {
		return nil, fmt.Errorf("resolving provider for model %q: %w", model, err)
	}
	return provider.Chat(ctx, messages, tools, model, options)
}

// GetDefaultModel returns the default model from config.
func (mp *MultiProvider) GetDefaultModel() string {
	return mp.cfg.Agents.Defaults.Model
}

// getProvider returns a cached provider for the given model, creating one if needed.
func (mp *MultiProvider) getProvider(model string) (LLMProvider, error) {
	key := mp.cacheKey(model)

	mp.mu.RLock()
	if p, ok := mp.cache[key]; ok {
		mp.mu.RUnlock()
		return p, nil
	}
	mp.mu.RUnlock()

	mp.mu.Lock()
	defer mp.mu.Unlock()

	// Double-check after acquiring write lock
	if p, ok := mp.cache[key]; ok {
		return p, nil
	}

	p, err := CreateProviderForModel(mp.cfg, model)
	if err != nil {
		return nil, err
	}

	mp.cache[key] = p
	return p, nil
}

// cacheKey builds a cache key from the model name that incorporates
// the relevant API key, so that cache auto-invalidates when keys change in config.
func (mp *MultiProvider) cacheKey(model string) string {
	providerKey := mp.resolveProviderKey(model)
	apiKey := mp.getAPIKeyForProvider(providerKey)
	return providerKey + ":" + apiKey
}

// resolveProviderKey determines which provider key to use for a given model name.
func (mp *MultiProvider) resolveProviderKey(model string) string {
	cfg := mp.cfg
	providerName := strings.ToLower(cfg.Agents.Defaults.Provider)
	lowerModel := strings.ToLower(model)

	// If an explicit provider is set AND the model doesn't have a routing prefix,
	// honor the explicit provider setting.
	if providerName != "" && !hasRoutingPrefix(model) {
		return providerName
	}

	switch {
	case strings.Contains(lowerModel, "kimi") || strings.Contains(lowerModel, "moonshot") || strings.HasPrefix(model, "moonshot/"):
		if cfg.Providers.Moonshot.APIKey != "" {
			return "moonshot"
		}

	case strings.HasPrefix(model, "openrouter/") || strings.HasPrefix(model, "anthropic/") ||
		strings.HasPrefix(model, "openai/") || strings.HasPrefix(model, "meta-llama/") ||
		strings.HasPrefix(model, "deepseek/") || strings.HasPrefix(model, "google/"):
		return "openrouter"

	case strings.Contains(lowerModel, "claude"):
		if cfg.Providers.Anthropic.APIKey != "" || cfg.Providers.Anthropic.AuthMethod != "" {
			return "anthropic"
		}

	case strings.Contains(lowerModel, "gpt"):
		if cfg.Providers.OpenAI.APIKey != "" || cfg.Providers.OpenAI.AuthMethod != "" {
			return "openai"
		}

	case strings.Contains(lowerModel, "gemini"):
		if cfg.Providers.Gemini.APIKey != "" {
			return "gemini"
		}

	case strings.Contains(lowerModel, "glm") || strings.Contains(lowerModel, "zhipu") || strings.Contains(lowerModel, "zai"):
		if cfg.Providers.Zhipu.APIKey != "" {
			return "zhipu"
		}

	case strings.Contains(lowerModel, "groq") || strings.HasPrefix(model, "groq/"):
		if cfg.Providers.Groq.APIKey != "" {
			return "groq"
		}

	case strings.Contains(lowerModel, "nvidia") || strings.HasPrefix(model, "nvidia/"):
		if cfg.Providers.Nvidia.APIKey != "" {
			return "nvidia"
		}
	}

	// Fallbacks
	if cfg.Providers.VLLM.APIBase != "" {
		return "vllm"
	}
	if cfg.Providers.OpenRouter.APIKey != "" {
		return "openrouter"
	}

	return "unknown"
}

// hasRoutingPrefix returns true if the model name has a provider-routing prefix
// like "anthropic/", "openai/", etc.
func hasRoutingPrefix(model string) bool {
	prefixes := []string{"anthropic/", "openai/", "openrouter/", "meta-llama/", "deepseek/", "google/", "moonshot/", "nvidia/", "groq/"}
	for _, p := range prefixes {
		if strings.HasPrefix(model, p) {
			return true
		}
	}
	return false
}

// getAPIKeyForProvider returns the current API key for a provider key,
// used to invalidate cache when keys change.
func (mp *MultiProvider) getAPIKeyForProvider(providerKey string) string {
	cfg := mp.cfg
	switch providerKey {
	case "openrouter":
		return cfg.Providers.OpenRouter.APIKey
	case "anthropic", "claude":
		return cfg.Providers.Anthropic.APIKey + cfg.Providers.Anthropic.AuthMethod
	case "openai", "gpt":
		return cfg.Providers.OpenAI.APIKey + cfg.Providers.OpenAI.AuthMethod
	case "gemini", "google":
		return cfg.Providers.Gemini.APIKey
	case "zhipu", "glm":
		return cfg.Providers.Zhipu.APIKey
	case "groq":
		return cfg.Providers.Groq.APIKey
	case "moonshot":
		return cfg.Providers.Moonshot.APIKey
	case "nvidia":
		return cfg.Providers.Nvidia.APIKey
	case "deepseek":
		return cfg.Providers.DeepSeek.APIKey
	case "vllm":
		return cfg.Providers.VLLM.APIBase
	case "shengsuanyun":
		return cfg.Providers.ShengSuanYun.APIKey
	default:
		return ""
	}
}
