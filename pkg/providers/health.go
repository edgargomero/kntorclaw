package providers

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/sipeed/picoclaw/pkg/config"
)

// ProviderStatus holds the health check result for a single provider.
type ProviderStatus struct {
	Name        string
	Reachable   bool
	LLMReady    bool
	Latency     time.Duration
	Error       string
	LastChecked time.Time
}

// HealthChecker periodically validates provider connectivity and LLM availability.
type HealthChecker struct {
	cfg      *config.Config
	mu       sync.RWMutex
	statuses map[string]*ProviderStatus
	interval time.Duration
	onChange func(map[string]*ProviderStatus)
}

// NewHealthChecker creates a HealthChecker that checks all configured providers
// at the given interval.
func NewHealthChecker(cfg *config.Config, interval time.Duration) *HealthChecker {
	return &HealthChecker{
		cfg:      cfg,
		statuses: make(map[string]*ProviderStatus),
		interval: interval,
	}
}

// SetOnChange registers a callback invoked after each check cycle with a snapshot
// of all provider statuses.
func (hc *HealthChecker) SetOnChange(fn func(map[string]*ProviderStatus)) {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	hc.onChange = fn
}

// Start runs the health check loop until ctx is cancelled. It performs an
// immediate check, then repeats every interval.
func (hc *HealthChecker) Start(ctx context.Context) {
	hc.CheckAll(ctx)

	ticker := time.NewTicker(hc.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			hc.CheckAll(ctx)
		}
	}
}

// CheckAll runs ping + LLM probe for every configured provider in parallel.
func (hc *HealthChecker) CheckAll(ctx context.Context) {
	endpoints := getConfiguredProviders(hc.cfg)
	if len(endpoints) == 0 {
		return
	}

	var wg sync.WaitGroup
	results := make(chan *ProviderStatus, len(endpoints))

	for name, endpoint := range endpoints {
		wg.Add(1)
		go func(name, endpoint string) {
			defer wg.Done()
			results <- checkProvider(ctx, hc.cfg, name, endpoint)
		}(name, endpoint)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	newStatuses := make(map[string]*ProviderStatus)
	for s := range results {
		newStatuses[s.Name] = s
	}

	hc.mu.Lock()
	hc.statuses = newStatuses
	onChange := hc.onChange
	hc.mu.Unlock()

	if onChange != nil {
		onChange(hc.GetStatuses())
	}
}

// GetStatuses returns a thread-safe snapshot of all provider statuses.
func (hc *HealthChecker) GetStatuses() map[string]*ProviderStatus {
	hc.mu.RLock()
	defer hc.mu.RUnlock()

	snapshot := make(map[string]*ProviderStatus, len(hc.statuses))
	for k, v := range hc.statuses {
		cp := *v
		snapshot[k] = &cp
	}
	return snapshot
}

// checkProvider performs Phase 1 (ping) and Phase 2 (LLM probe) for one provider.
func checkProvider(ctx context.Context, cfg *config.Config, name, endpoint string) *ProviderStatus {
	status := &ProviderStatus{
		Name:        name,
		LastChecked: time.Now(),
	}

	// Phase 1: HTTP ping
	start := time.Now()
	pingCtx, pingCancel := context.WithTimeout(ctx, 5*time.Second)
	defer pingCancel()

	req, err := http.NewRequestWithContext(pingCtx, http.MethodHead, endpoint+"/models", nil)
	if err != nil {
		status.Error = "ping: " + err.Error()
		return status
	}

	apiKey := getAPIKeyForProviderName(cfg, name)
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		status.Error = "ping: " + err.Error()
		status.Latency = time.Since(start)
		return status
	}
	resp.Body.Close()
	status.Latency = time.Since(start)
	status.Reachable = true

	// Phase 2: LLM probe (only if ping succeeded)
	probeCtx, probeCancel := context.WithTimeout(ctx, 10*time.Second)
	defer probeCancel()

	provider, err := CreateProviderForModel(cfg, getDefaultModelForProvider(name))
	if err != nil {
		status.Error = "probe: " + err.Error()
		return status
	}

	_, err = provider.Chat(probeCtx, []Message{{Role: "user", Content: "hi"}}, nil, getDefaultModelForProvider(name), map[string]interface{}{"max_tokens": 1})
	if err != nil {
		status.Error = "probe: " + err.Error()
		return status
	}

	status.LLMReady = true
	return status
}

// GetProviderEndpoint returns the API base URL for a given provider key,
// using config overrides or sensible defaults.
func GetProviderEndpoint(cfg *config.Config, providerKey string) string {
	switch providerKey {
	case "anthropic", "claude":
		if cfg.Providers.Anthropic.APIBase != "" {
			return cfg.Providers.Anthropic.APIBase
		}
		return "https://api.anthropic.com/v1"
	case "anthropic-cc":
		return "https://api.anthropic.com/v1"
	case "openai", "gpt":
		if cfg.Providers.OpenAI.APIBase != "" {
			return cfg.Providers.OpenAI.APIBase
		}
		return "https://api.openai.com/v1"
	case "openrouter":
		if cfg.Providers.OpenRouter.APIBase != "" {
			return cfg.Providers.OpenRouter.APIBase
		}
		return "https://openrouter.ai/api/v1"
	case "groq":
		if cfg.Providers.Groq.APIBase != "" {
			return cfg.Providers.Groq.APIBase
		}
		return "https://api.groq.com/openai/v1"
	case "zhipu", "glm":
		if cfg.Providers.Zhipu.APIBase != "" {
			return cfg.Providers.Zhipu.APIBase
		}
		return "https://open.bigmodel.cn/api/paas/v4"
	case "gemini", "google":
		if cfg.Providers.Gemini.APIBase != "" {
			return cfg.Providers.Gemini.APIBase
		}
		return "https://generativelanguage.googleapis.com/v1beta"
	case "nvidia":
		if cfg.Providers.Nvidia.APIBase != "" {
			return cfg.Providers.Nvidia.APIBase
		}
		return "https://integrate.api.nvidia.com/v1"
	case "moonshot":
		if cfg.Providers.Moonshot.APIBase != "" {
			return cfg.Providers.Moonshot.APIBase
		}
		return "https://api.moonshot.cn/v1"
	case "deepseek":
		if cfg.Providers.DeepSeek.APIBase != "" {
			return cfg.Providers.DeepSeek.APIBase
		}
		return "https://api.deepseek.com/v1"
	case "ollama":
		if cfg.Providers.Ollama.APIBase != "" {
			return cfg.Providers.Ollama.APIBase
		}
		return "http://localhost:11434/v1"
	case "vllm":
		return cfg.Providers.VLLM.APIBase
	case "shengsuanyun":
		if cfg.Providers.ShengSuanYun.APIBase != "" {
			return cfg.Providers.ShengSuanYun.APIBase
		}
		return "https://router.shengsuanyun.com/api/v1"
	default:
		return ""
	}
}

// getConfiguredProviders returns a map of provider name -> API base for all
// providers that have an API key (or API base for local providers) configured.
func getConfiguredProviders(cfg *config.Config) map[string]string {
	providers := make(map[string]string)

	if cfg.Providers.Anthropic.APIKey != "" || cfg.Providers.Anthropic.AuthMethod != "" {
		providers["anthropic"] = GetProviderEndpoint(cfg, "anthropic")
	}
	if cfg.Providers.Anthropic.AuthMethod != "" {
		providers["anthropic-cc"] = GetProviderEndpoint(cfg, "anthropic-cc")
	}
	if cfg.Providers.OpenAI.APIKey != "" || cfg.Providers.OpenAI.AuthMethod != "" {
		providers["openai"] = GetProviderEndpoint(cfg, "openai")
	}
	if cfg.Providers.OpenRouter.APIKey != "" {
		providers["openrouter"] = GetProviderEndpoint(cfg, "openrouter")
	}
	if cfg.Providers.Groq.APIKey != "" {
		providers["groq"] = GetProviderEndpoint(cfg, "groq")
	}
	if cfg.Providers.Zhipu.APIKey != "" {
		providers["zhipu"] = GetProviderEndpoint(cfg, "zhipu")
	}
	if cfg.Providers.Gemini.APIKey != "" {
		providers["gemini"] = GetProviderEndpoint(cfg, "gemini")
	}
	if cfg.Providers.Nvidia.APIKey != "" {
		providers["nvidia"] = GetProviderEndpoint(cfg, "nvidia")
	}
	if cfg.Providers.Moonshot.APIKey != "" {
		providers["moonshot"] = GetProviderEndpoint(cfg, "moonshot")
	}
	if cfg.Providers.DeepSeek.APIKey != "" {
		providers["deepseek"] = GetProviderEndpoint(cfg, "deepseek")
	}
	if cfg.Providers.Ollama.APIKey != "" || cfg.Providers.Ollama.APIBase != "" {
		providers["ollama"] = GetProviderEndpoint(cfg, "ollama")
	}
	if cfg.Providers.VLLM.APIBase != "" {
		providers["vllm"] = GetProviderEndpoint(cfg, "vllm")
	}
	if cfg.Providers.ShengSuanYun.APIKey != "" {
		providers["shengsuanyun"] = GetProviderEndpoint(cfg, "shengsuanyun")
	}

	return providers
}

// getAPIKeyForProviderName returns the API key for a provider name.
func getAPIKeyForProviderName(cfg *config.Config, name string) string {
	switch name {
	case "anthropic":
		return cfg.Providers.Anthropic.APIKey
	case "anthropic-cc":
		return "" // uses auth store, not API key
	case "openai":
		return cfg.Providers.OpenAI.APIKey
	case "openrouter":
		return cfg.Providers.OpenRouter.APIKey
	case "groq":
		return cfg.Providers.Groq.APIKey
	case "zhipu":
		return cfg.Providers.Zhipu.APIKey
	case "gemini":
		return cfg.Providers.Gemini.APIKey
	case "nvidia":
		return cfg.Providers.Nvidia.APIKey
	case "moonshot":
		return cfg.Providers.Moonshot.APIKey
	case "deepseek":
		return cfg.Providers.DeepSeek.APIKey
	case "ollama":
		return cfg.Providers.Ollama.APIKey
	case "vllm":
		return cfg.Providers.VLLM.APIKey
	case "shengsuanyun":
		return cfg.Providers.ShengSuanYun.APIKey
	default:
		return ""
	}
}

// getDefaultModelForProvider returns a representative model name for a provider,
// used by the LLM probe phase.
func getDefaultModelForProvider(name string) string {
	switch name {
	case "anthropic":
		return "claude-sonnet-4-5-20250929"
	case "anthropic-cc":
		return "claude-sonnet-4-5-20250929@anthropic-cc"
	case "openai":
		return "gpt-4o-mini"
	case "openrouter":
		return "openrouter/auto"
	case "groq":
		return "groq/llama-3.1-8b-instant"
	case "zhipu":
		return "glm-4-flash"
	case "gemini":
		return "gemini-1.5-flash"
	case "nvidia":
		return "nvidia/llama-3.1-nemotron-70b-instruct"
	case "moonshot":
		return "moonshot/moonshot-v1-8k"
	case "deepseek":
		return "deepseek-chat"
	case "ollama":
		return "ollama/llama3"
	case "vllm":
		return "default"
	case "shengsuanyun":
		return "shengsuanyun/default"
	default:
		return "default"
	}
}
