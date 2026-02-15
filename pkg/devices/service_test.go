package devices

import "testing"

func TestParseLastChannel(t *testing.T) {
	tests := []struct {
		input        string
		wantPlatform string
		wantUserID   string
	}{
		{"telegram:user123", "telegram", "user123"},
		{"discord:guild:channel", "discord", "guild:channel"},
		{"whatsapp:+1234567890", "whatsapp", "+1234567890"},
		{"tui:local", "tui", "local"},

		// Invalid cases
		{"", "", ""},
		{"nocolon", "", ""},
		{":noplatform", "", ""},
		{"nouser:", "", ""},
	}

	for _, tt := range tests {
		platform, userID := parseLastChannel(tt.input)
		if platform != tt.wantPlatform || userID != tt.wantUserID {
			t.Errorf("parseLastChannel(%q) = (%q, %q), want (%q, %q)",
				tt.input, platform, userID, tt.wantPlatform, tt.wantUserID)
		}
	}
}

func TestNewService_Disabled(t *testing.T) {
	s := NewService(Config{Enabled: false}, nil)
	if s.enabled {
		t.Error("Service should be disabled when config Enabled is false")
	}
	if len(s.sources) != 0 {
		t.Errorf("disabled service should have 0 sources, got %d", len(s.sources))
	}
}
