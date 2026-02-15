package providers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sipeed/picoclaw/pkg/config"
)

func newTestConfig() *config.Config {
	return &config.Config{
		Agents: config.AgentsConfig{
			Defaults: config.AgentDefaults{
				Model: "claude-sonnet-4-5-20250929",
			},
		},
	}
}

func TestResolveProviderKey_OpenRouterPrefixes(t *testing.T) {
	cfg := newTestConfig()
	cfg.Providers.OpenRouter.APIKey = "or-key"
	mp := NewMultiProvider(cfg)

	tests := []struct {
		model string
		want  string
	}{
		{"anthropic/claude-sonnet-4-5-20250929", "openrouter"},
		{"openai/gpt-4o", "openrouter"},
		{"meta-llama/llama-3-70b", "openrouter"},
		{"google/gemini-pro", "openrouter"},
		{"deepseek/deepseek-chat", "openrouter"},
		{"openrouter/auto", "openrouter"},
	}

	for _, tt := range tests {
		got := mp.resolveProviderKey(tt.model)
		if got != tt.want {
			t.Errorf("resolveProviderKey(%q) = %q, want %q", tt.model, got, tt.want)
		}
	}
}

func TestResolveProviderKey_DirectModels(t *testing.T) {
	cfg := newTestConfig()
	cfg.Providers.Anthropic.APIKey = "ant-key"
	cfg.Providers.OpenAI.APIKey = "oai-key"
	cfg.Providers.Gemini.APIKey = "gem-key"
	cfg.Providers.Zhipu.APIKey = "zhipu-key"
	cfg.Providers.Groq.APIKey = "groq-key"
	mp := NewMultiProvider(cfg)

	tests := []struct {
		model string
		want  string
	}{
		{"claude-sonnet-4-5-20250929", "anthropic"},
		{"claude-3-haiku", "anthropic"},
		{"gpt-4o", "openai"},
		{"gpt-3.5-turbo", "openai"},
		{"gemini-pro", "gemini"},
		{"glm-4", "zhipu"},
	}

	for _, tt := range tests {
		got := mp.resolveProviderKey(tt.model)
		if got != tt.want {
			t.Errorf("resolveProviderKey(%q) = %q, want %q", tt.model, got, tt.want)
		}
	}
}

func TestResolveProviderKey_ExplicitProvider(t *testing.T) {
	cfg := newTestConfig()
	cfg.Agents.Defaults.Provider = "groq"
	cfg.Providers.Groq.APIKey = "groq-key"
	mp := NewMultiProvider(cfg)

	// Model without routing prefix should use the explicit provider
	got := mp.resolveProviderKey("llama-3-70b")
	if got != "groq" {
		t.Errorf("resolveProviderKey with explicit provider = %q, want %q", got, "groq")
	}

	// Model WITH routing prefix should override the explicit provider
	got = mp.resolveProviderKey("anthropic/claude-sonnet-4-5-20250929")
	if got != "openrouter" {
		t.Errorf("resolveProviderKey with prefix should override explicit provider = %q, want %q", got, "openrouter")
	}
}

func TestResolveProviderKey_FallbackOpenRouter(t *testing.T) {
	cfg := newTestConfig()
	cfg.Providers.OpenRouter.APIKey = "or-key"
	mp := NewMultiProvider(cfg)

	got := mp.resolveProviderKey("some-unknown-model")
	if got != "openrouter" {
		t.Errorf("resolveProviderKey(unknown model) = %q, want %q (fallback)", got, "openrouter")
	}
}

func TestResolveProviderKey_FallbackVLLM(t *testing.T) {
	cfg := newTestConfig()
	cfg.Providers.VLLM.APIBase = "http://localhost:8000/v1"
	mp := NewMultiProvider(cfg)

	got := mp.resolveProviderKey("some-local-model")
	if got != "vllm" {
		t.Errorf("resolveProviderKey(unknown, vllm available) = %q, want %q", got, "vllm")
	}
}

func TestHasRoutingPrefix(t *testing.T) {
	tests := []struct {
		model string
		want  bool
	}{
		{"anthropic/claude-sonnet-4-5-20250929", true},
		{"openai/gpt-4o", true},
		{"meta-llama/llama-3", true},
		{"google/gemini-pro", true},
		{"claude-sonnet-4-5-20250929", false},
		{"gpt-4o", false},
		{"my-custom-model", false},
	}

	for _, tt := range tests {
		got := hasRoutingPrefix(tt.model)
		if got != tt.want {
			t.Errorf("hasRoutingPrefix(%q) = %v, want %v", tt.model, got, tt.want)
		}
	}
}

func TestCacheKey_InvalidatesOnKeyChange(t *testing.T) {
	cfg := newTestConfig()
	cfg.Providers.Anthropic.APIKey = "key-v1"
	mp := NewMultiProvider(cfg)

	key1 := mp.cacheKey("claude-sonnet-4-5-20250929")

	// Simulate user changing API key in TUI
	cfg.Providers.Anthropic.APIKey = "key-v2"
	key2 := mp.cacheKey("claude-sonnet-4-5-20250929")

	if key1 == key2 {
		t.Error("cacheKey should change when API key changes")
	}
}

func TestMultiProvider_GetDefaultModel(t *testing.T) {
	cfg := newTestConfig()
	cfg.Agents.Defaults.Model = "my-model"
	mp := NewMultiProvider(cfg)

	if got := mp.GetDefaultModel(); got != "my-model" {
		t.Errorf("GetDefaultModel() = %q, want %q", got, "my-model")
	}
}

func TestMultiProvider_ChatDispatchesToCorrectProvider(t *testing.T) {
	// Create two mock servers to verify dispatch
	anthropicServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{"message": map[string]interface{}{"content": "from-anthropic"}, "finish_reason": "stop"},
			},
		})
	}))
	defer anthropicServer.Close()

	openrouterServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{"message": map[string]interface{}{"content": "from-openrouter"}, "finish_reason": "stop"},
			},
		})
	}))
	defer openrouterServer.Close()

	cfg := newTestConfig()
	cfg.Providers.Anthropic.APIKey = "ant-key"
	cfg.Providers.Anthropic.APIBase = anthropicServer.URL
	cfg.Providers.OpenRouter.APIKey = "or-key"
	cfg.Providers.OpenRouter.APIBase = openrouterServer.URL

	mp := NewMultiProvider(cfg)
	ctx := context.Background()
	msgs := []Message{{Role: "user", Content: "hi"}}

	// Model with openrouter prefix → should go to openrouter
	resp, err := mp.Chat(ctx, msgs, nil, "anthropic/claude-sonnet-4-5-20250929", nil)
	if err != nil {
		t.Fatalf("Chat(openrouter model) error: %v", err)
	}
	if resp.Content != "from-openrouter" {
		t.Errorf("Chat(openrouter model) content = %q, want %q", resp.Content, "from-openrouter")
	}

	// Direct claude model → should go to anthropic (HTTP provider since we set APIBase, not SDK)
	resp, err = mp.Chat(ctx, msgs, nil, "claude-sonnet-4-5-20250929", nil)
	if err != nil {
		t.Fatalf("Chat(anthropic model) error: %v", err)
	}
	if resp.Content != "from-anthropic" {
		t.Errorf("Chat(anthropic model) content = %q, want %q", resp.Content, "from-anthropic")
	}
}

func TestMultiProvider_CachesProviders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{"message": map[string]interface{}{"content": "ok"}, "finish_reason": "stop"},
			},
		})
	}))
	defer server.Close()

	cfg := newTestConfig()
	cfg.Providers.OpenRouter.APIKey = "or-key"
	cfg.Providers.OpenRouter.APIBase = server.URL

	mp := NewMultiProvider(cfg)
	ctx := context.Background()
	msgs := []Message{{Role: "user", Content: "hi"}}

	// First call creates the provider
	_, err := mp.Chat(ctx, msgs, nil, "anthropic/claude-sonnet-4-5-20250929", nil)
	if err != nil {
		t.Fatalf("first Chat() error: %v", err)
	}

	if len(mp.cache) != 1 {
		t.Errorf("cache size after first call = %d, want 1", len(mp.cache))
	}

	// Second call with same model should reuse cached provider
	_, err = mp.Chat(ctx, msgs, nil, "anthropic/claude-sonnet-4-5-20250929", nil)
	if err != nil {
		t.Fatalf("second Chat() error: %v", err)
	}

	if len(mp.cache) != 1 {
		t.Errorf("cache size after second call = %d, want 1 (should reuse)", len(mp.cache))
	}

	// Different model on same provider should still be 1 cache entry (same provider key + key)
	_, err = mp.Chat(ctx, msgs, nil, "openai/gpt-4o", nil)
	if err != nil {
		t.Fatalf("third Chat() error: %v", err)
	}

	// Same provider (openrouter) so still 1 entry
	if len(mp.cache) != 1 {
		t.Errorf("cache size after different model same provider = %d, want 1", len(mp.cache))
	}
}
