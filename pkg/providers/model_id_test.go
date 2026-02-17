package providers

import "testing"

func TestParseModelID(t *testing.T) {
	tests := []struct {
		raw          string
		wantModel    string
		wantProvider string
	}{
		{"claude-sonnet-4-5-20250929@anthropic", "claude-sonnet-4-5-20250929", "anthropic"},
		{"z-ai/glm5@nvidia", "z-ai/glm5", "nvidia"},
		{"gpt-4o@openai", "gpt-4o", "openai"},
		{"claude-sonnet-4-5-20250929", "claude-sonnet-4-5-20250929", ""},
		{"z-ai/glm5", "z-ai/glm5", ""},
		{"", "", ""},
		{"@anthropic", "", "anthropic"},
		{"model@", "model", ""},
	}

	for _, tt := range tests {
		model, provider := ParseModelID(tt.raw)
		if model != tt.wantModel || provider != tt.wantProvider {
			t.Errorf("ParseModelID(%q) = (%q, %q), want (%q, %q)",
				tt.raw, model, provider, tt.wantModel, tt.wantProvider)
		}
	}
}

func TestFormatModelID(t *testing.T) {
	tests := []struct {
		model    string
		provider string
		want     string
	}{
		{"claude-sonnet-4-5-20250929", "anthropic", "claude-sonnet-4-5-20250929@anthropic"},
		{"z-ai/glm5", "nvidia", "z-ai/glm5@nvidia"},
		{"gpt-4o", "", "gpt-4o"},
		{"", "", ""},
	}

	for _, tt := range tests {
		got := FormatModelID(tt.model, tt.provider)
		if got != tt.want {
			t.Errorf("FormatModelID(%q, %q) = %q, want %q",
				tt.model, tt.provider, got, tt.want)
		}
	}
}

func TestParseFormatRoundTrip(t *testing.T) {
	inputs := []string{
		"claude-sonnet-4-5-20250929@anthropic",
		"z-ai/glm5@nvidia",
		"gpt-4o@openai",
	}
	for _, raw := range inputs {
		model, provider := ParseModelID(raw)
		got := FormatModelID(model, provider)
		if got != raw {
			t.Errorf("round-trip failed: %q -> FormatModelID(%q, %q) = %q",
				raw, model, provider, got)
		}
	}
}
