package constants

import "testing"

func TestIsInternalChannel(t *testing.T) {
	tests := []struct {
		channel string
		want    bool
	}{
		{"cli", true},
		{"system", true},
		{"subagent", true},
		{"telegram", false},
		{"discord", false},
		{"slack", false},
		{"whatsapp", false},
		{"tui", false},
		{"", false},
	}

	for _, tt := range tests {
		got := IsInternalChannel(tt.channel)
		if got != tt.want {
			t.Errorf("IsInternalChannel(%q) = %v, want %v", tt.channel, got, tt.want)
		}
	}
}

func TestInternalChannels_Map(t *testing.T) {
	expected := map[string]bool{
		"cli":      true,
		"system":   true,
		"subagent": true,
	}

	if len(InternalChannels) != len(expected) {
		t.Errorf("InternalChannels has %d entries, want %d", len(InternalChannels), len(expected))
	}

	for k, v := range expected {
		if InternalChannels[k] != v {
			t.Errorf("InternalChannels[%q] = %v, want %v", k, InternalChannels[k], v)
		}
	}
}
