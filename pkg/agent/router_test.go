package agent

import (
	"testing"
)

func TestModelRouter_Resolve_Default(t *testing.T) {
	r := NewModelRouter("default-model", nil, nil)
	got := r.Resolve("", "")
	if got != "default-model" {
		t.Errorf("expected 'default-model', got '%s'", got)
	}
}

func TestModelRouter_Resolve_ChannelOverride(t *testing.T) {
	r := NewModelRouter("default-model", map[string]string{
		"telegram": "haiku",
	}, nil)

	got := r.Resolve("telegram", "")
	if got != "haiku" {
		t.Errorf("expected 'haiku', got '%s'", got)
	}

	// Unknown channel falls back to default
	got = r.Resolve("discord", "")
	if got != "default-model" {
		t.Errorf("expected 'default-model', got '%s'", got)
	}
}

func TestModelRouter_Resolve_SessionOverride(t *testing.T) {
	r := NewModelRouter("default-model", map[string]string{
		"telegram": "haiku",
	}, nil)

	r.SetSessionModel("session-1", "opus")

	// Session override beats channel
	got := r.Resolve("telegram", "session-1")
	if got != "opus" {
		t.Errorf("expected 'opus', got '%s'", got)
	}

	// Different session falls through to channel
	got = r.Resolve("telegram", "session-2")
	if got != "haiku" {
		t.Errorf("expected 'haiku', got '%s'", got)
	}
}

func TestModelRouter_ClearSessionModel(t *testing.T) {
	r := NewModelRouter("default-model", nil, nil)
	r.SetSessionModel("s1", "opus")

	got := r.Resolve("", "s1")
	if got != "opus" {
		t.Errorf("expected 'opus', got '%s'", got)
	}

	r.ClearSessionModel("s1")

	got = r.Resolve("", "s1")
	if got != "default-model" {
		t.Errorf("expected 'default-model' after clear, got '%s'", got)
	}
}

func TestModelRouter_ResolveAlias(t *testing.T) {
	r := NewModelRouter("default", nil, map[string]string{
		"opus":   "claude-opus-4-6",
		"sonnet": "claude-sonnet-4-5-20250929",
	})

	if got := r.ResolveAlias("opus"); got != "claude-opus-4-6" {
		t.Errorf("expected 'claude-opus-4-6', got '%s'", got)
	}

	// Unknown alias returns input unchanged
	if got := r.ResolveAlias("gpt-4o"); got != "gpt-4o" {
		t.Errorf("expected 'gpt-4o' passthrough, got '%s'", got)
	}
}

func TestModelRouter_GetInfo(t *testing.T) {
	r := NewModelRouter("default-model", map[string]string{
		"tui": "tui-model",
	}, nil)

	model, source := r.GetInfo("", "")
	if model != "default-model" || source != "default" {
		t.Errorf("expected default/default, got %s/%s", model, source)
	}

	model, source = r.GetInfo("tui", "")
	if model != "tui-model" || source != "channel override" {
		t.Errorf("expected tui-model/channel override, got %s/%s", model, source)
	}

	r.SetSessionModel("s1", "session-model")
	model, source = r.GetInfo("tui", "s1")
	if model != "session-model" || source != "session override" {
		t.Errorf("expected session-model/session override, got %s/%s", model, source)
	}
}

func TestModelRouter_SetDefaultModel(t *testing.T) {
	r := NewModelRouter("old", nil, nil)

	r.SetDefaultModel("new")

	if got := r.Resolve("", ""); got != "new" {
		t.Errorf("expected 'new', got '%s'", got)
	}
	if got := r.DefaultModel(); got != "new" {
		t.Errorf("expected 'new', got '%s'", got)
	}
}

func TestModelRouter_SetChannelModel(t *testing.T) {
	r := NewModelRouter("default", nil, nil)

	r.SetChannelModel("slack", "slack-model")

	if got := r.Resolve("slack", ""); got != "slack-model" {
		t.Errorf("expected 'slack-model', got '%s'", got)
	}
}

func TestModelRouter_GetAliases(t *testing.T) {
	aliases := map[string]string{"a": "model-a", "b": "model-b"}
	r := NewModelRouter("default", nil, aliases)

	got := r.GetAliases()
	if len(got) != 2 {
		t.Errorf("expected 2 aliases, got %d", len(got))
	}

	// Verify it's a copy (modifying returned map doesn't affect router)
	got["c"] = "model-c"
	if len(r.GetAliases()) != 2 {
		t.Error("GetAliases should return a copy")
	}
}
