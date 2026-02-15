package utils

import "testing"

func TestTruncate(t *testing.T) {
	tests := []struct {
		input  string
		maxLen int
		want   string
	}{
		// No truncation needed
		{"hello", 10, "hello"},
		{"hello", 5, "hello"},
		{"", 5, ""},

		// Truncation with ellipsis
		{"hello world", 8, "hello..."},
		{"abcdefghij", 7, "abcd..."},

		// Edge cases: maxLen <= 3 (no room for "...")
		{"abcdef", 3, "abc"},
		{"abcdef", 2, "ab"},
		{"abcdef", 1, "a"},
		{"abcdef", 0, ""},

		// Unicode: multi-byte characters
		{"こんにちは世界", 5, "こん..."},
		{"こんにちは", 5, "こんにちは"},
		{"🦞🦀🐙🐟", 3, "🦞🦀🐙"},
		{"🦞🦀🐙🐟🐠", 4, "🦞..."},
	}

	for _, tt := range tests {
		got := Truncate(tt.input, tt.maxLen)
		if got != tt.want {
			t.Errorf("Truncate(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.want)
		}
	}
}
