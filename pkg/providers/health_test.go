package providers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sipeed/picoclaw/pkg/config"
)

func TestHealthChecker_PingReachable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := &config.Config{}
	cfg.Providers.Zhipu.APIKey = "test-key"
	cfg.Providers.Zhipu.APIBase = server.URL

	endpoints := map[string]string{"zhipu": server.URL}
	status := checkProvider(context.Background(), cfg, "zhipu", endpoints["zhipu"])

	if !status.Reachable {
		t.Errorf("expected Reachable=true, got false; error: %s", status.Error)
	}
	if status.Latency == 0 {
		t.Error("expected non-zero latency")
	}
}

func TestHealthChecker_PingUnreachable(t *testing.T) {
	cfg := &config.Config{}
	// Use an endpoint that won't be reachable
	status := checkProvider(context.Background(), cfg, "zhipu", "http://127.0.0.1:1")

	if status.Reachable {
		t.Error("expected Reachable=false for unreachable server")
	}
	if status.Error == "" {
		t.Error("expected non-empty error for unreachable server")
	}
}

func TestHealthChecker_OnlyConfiguredProviders(t *testing.T) {
	cfg := &config.Config{}
	// No API keys configured
	providers := getConfiguredProviders(cfg)

	if len(providers) != 0 {
		t.Errorf("expected 0 configured providers, got %d: %v", len(providers), providers)
	}

	// Configure one provider
	cfg.Providers.Groq.APIKey = "test-key"
	providers = getConfiguredProviders(cfg)

	if len(providers) != 1 {
		t.Errorf("expected 1 configured provider, got %d", len(providers))
	}
	if _, ok := providers["groq"]; !ok {
		t.Error("expected groq in configured providers")
	}
}

func TestHealthChecker_Concurrent(t *testing.T) {
	var hitCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hitCount.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := &config.Config{}
	cfg.Providers.Groq.APIKey = "test-key"
	cfg.Providers.Groq.APIBase = server.URL
	cfg.Providers.Zhipu.APIKey = "test-key"
	cfg.Providers.Zhipu.APIBase = server.URL

	hc := NewHealthChecker(cfg, time.Hour) // long interval, we call CheckAll manually

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	hc.CheckAll(ctx)

	statuses := hc.GetStatuses()
	if len(statuses) < 2 {
		t.Errorf("expected at least 2 statuses, got %d", len(statuses))
	}

	// Both should have been pinged (at least 2 HEAD requests)
	if hitCount.Load() < 2 {
		t.Errorf("expected at least 2 ping requests, got %d", hitCount.Load())
	}
}

func TestHealthChecker_OnChangeCallback(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := &config.Config{}
	cfg.Providers.Groq.APIKey = "test-key"
	cfg.Providers.Groq.APIBase = server.URL

	hc := NewHealthChecker(cfg, time.Hour)

	called := make(chan struct{}, 1)
	hc.SetOnChange(func(statuses map[string]*ProviderStatus) {
		select {
		case called <- struct{}{}:
		default:
		}
	})

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	hc.CheckAll(ctx)

	select {
	case <-called:
		// OK
	case <-time.After(5 * time.Second):
		t.Error("onChange callback was not called")
	}
}

func TestGetProviderEndpoint(t *testing.T) {
	cfg := &config.Config{}

	tests := []struct {
		key      string
		expected string
	}{
		{"anthropic", "https://api.anthropic.com/v1"},
		{"openai", "https://api.openai.com/v1"},
		{"openrouter", "https://openrouter.ai/api/v1"},
		{"groq", "https://api.groq.com/openai/v1"},
		{"zhipu", "https://open.bigmodel.cn/api/paas/v4"},
		{"gemini", "https://generativelanguage.googleapis.com/v1beta"},
		{"nvidia", "https://integrate.api.nvidia.com/v1"},
		{"moonshot", "https://api.moonshot.cn/v1"},
		{"deepseek", "https://api.deepseek.com/v1"},
		{"ollama", "http://localhost:11434/v1"},
		{"shengsuanyun", "https://router.shengsuanyun.com/api/v1"},
		{"unknown", ""},
	}

	for _, tt := range tests {
		got := GetProviderEndpoint(cfg, tt.key)
		if got != tt.expected {
			t.Errorf("GetProviderEndpoint(%q) = %q, want %q", tt.key, got, tt.expected)
		}
	}

	// Test with custom API base
	cfg.Providers.Anthropic.APIBase = "https://custom.anthropic.com/v1"
	got := GetProviderEndpoint(cfg, "anthropic")
	if got != "https://custom.anthropic.com/v1" {
		t.Errorf("expected custom API base, got %q", got)
	}
}
