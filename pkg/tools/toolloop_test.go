package tools

import (
	"context"
	"strings"
	"testing"

	"github.com/sipeed/picoclaw/pkg/providers"
)

// sequentialProvider returns scripted LLM responses in order.
// Subsequent calls beyond the script return a safe fallback.
type sequentialProvider struct {
	responses []*providers.LLMResponse
	idx       int
}

func (p *sequentialProvider) Chat(_ context.Context, _ []providers.Message, _ []providers.ToolDefinition, _ string, _ map[string]interface{}) (*providers.LLMResponse, error) {
	if p.idx < len(p.responses) {
		resp := p.responses[p.idx]
		p.idx++
		return resp, nil
	}
	return &providers.LLMResponse{Content: "fallback-answer"}, nil
}

func (p *sequentialProvider) GetDefaultModel() string { return "seq-model" }

// noopTool is a minimal Tool for testing tool-call tracking.
type noopTool struct{ toolName string }

func (t *noopTool) Name() string                                          { return t.toolName }
func (t *noopTool) Description() string                                   { return "noop" }
func (t *noopTool) Parameters() map[string]interface{}                    { return map[string]interface{}{"type": "object", "properties": map[string]interface{}{}} }
func (t *noopTool) Execute(_ context.Context, _ map[string]interface{}) *ToolResult {
	return SilentResult("noop-done")
}

// makeToolCallResponse builds an LLMResponse that requests a single tool call.
func makeToolCallResponse(toolName string) *providers.LLMResponse {
	return &providers.LLMResponse{
		ToolCalls: []providers.ToolCall{
			{
				ID:        "tc-1",
				Name:      toolName,
				Arguments: map[string]interface{}{},
				Function:  &providers.FunctionCall{Name: toolName, Arguments: "{}"},
			},
		},
	}
}

// ── ToolLoopResult field tests ────────────────────────────────────────────────

func TestToolLoopResult_HasNewFields(t *testing.T) {
	r := &ToolLoopResult{
		Content:      "ok",
		Iterations:   3,
		InputTokens:  1000,
		OutputTokens: 250,
		ToolsUsed:    []string{"read_file", "exec"},
	}

	if r.InputTokens != 1000 {
		t.Errorf("InputTokens: want 1000, got %d", r.InputTokens)
	}
	if r.OutputTokens != 250 {
		t.Errorf("OutputTokens: want 250, got %d", r.OutputTokens)
	}
	if len(r.ToolsUsed) != 2 {
		t.Errorf("ToolsUsed length: want 2, got %d", len(r.ToolsUsed))
	}
}

// ── Token tracking ────────────────────────────────────────────────────────────

func TestRunToolLoop_TokenTracking(t *testing.T) {
	provider := &sequentialProvider{
		responses: []*providers.LLMResponse{
			{
				Content: "answer",
				Usage:   &providers.UsageInfo{PromptTokens: 120, CompletionTokens: 40},
			},
		},
	}

	messages := []providers.Message{
		{Role: "system", Content: "system"},
		{Role: "user", Content: "task"},
	}

	result, err := RunToolLoop(context.Background(), ToolLoopConfig{
		Provider:      provider,
		Model:         "m",
		MaxIterations: 5,
	}, messages, "cli", "direct")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.InputTokens != 120 {
		t.Errorf("InputTokens: want 120, got %d", result.InputTokens)
	}
	if result.OutputTokens != 40 {
		t.Errorf("OutputTokens: want 40, got %d", result.OutputTokens)
	}
}

func TestRunToolLoop_TokensAccumulateAcrossIterations(t *testing.T) {
	// Two iterations: first returns a tool call, second returns final answer.
	// Tokens from both iterations must be accumulated.
	registry := NewToolRegistry()
	registry.Register(&noopTool{toolName: "noop"})

	provider := &sequentialProvider{
		responses: []*providers.LLMResponse{
			// iteration 1: tool call
			{
				ToolCalls: []providers.ToolCall{
					{ID: "tc1", Name: "noop", Arguments: map[string]interface{}{},
						Function: &providers.FunctionCall{Name: "noop", Arguments: "{}"}},
				},
				Usage: &providers.UsageInfo{PromptTokens: 100, CompletionTokens: 10},
			},
			// iteration 2: final answer
			{
				Content: "done",
				Usage:   &providers.UsageInfo{PromptTokens: 200, CompletionTokens: 30},
			},
		},
	}

	messages := []providers.Message{
		{Role: "system", Content: "system"},
		{Role: "user", Content: "task"},
	}

	result, err := RunToolLoop(context.Background(), ToolLoopConfig{
		Provider:      provider,
		Model:         "m",
		Tools:         registry,
		MaxIterations: 5,
	}, messages, "cli", "direct")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// 100+200 = 300 prompt tokens, 10+30 = 40 completion tokens
	if result.InputTokens != 300 {
		t.Errorf("accumulated InputTokens: want 300, got %d", result.InputTokens)
	}
	if result.OutputTokens != 40 {
		t.Errorf("accumulated OutputTokens: want 40, got %d", result.OutputTokens)
	}
}

// ── ToolsUsed tracking ────────────────────────────────────────────────────────

func TestRunToolLoop_ToolsUsedTracked(t *testing.T) {
	registry := NewToolRegistry()
	registry.Register(&noopTool{toolName: "my_tool"})

	provider := &sequentialProvider{
		responses: []*providers.LLMResponse{
			makeToolCallResponse("my_tool"),  // iter 1
			{Content: "final answer"},        // iter 2
		},
	}

	messages := []providers.Message{
		{Role: "system", Content: "sys"},
		{Role: "user", Content: "task"},
	}

	result, err := RunToolLoop(context.Background(), ToolLoopConfig{
		Provider:      provider,
		Model:         "m",
		Tools:         registry,
		MaxIterations: 5,
	}, messages, "cli", "direct")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.ToolsUsed) != 1 {
		t.Fatalf("ToolsUsed length: want 1, got %d", len(result.ToolsUsed))
	}
	if result.ToolsUsed[0] != "my_tool" {
		t.Errorf("ToolsUsed[0]: want 'my_tool', got %q", result.ToolsUsed[0])
	}
}

func TestRunToolLoop_ToolsUsedMultipleCalls(t *testing.T) {
	registry := NewToolRegistry()
	registry.Register(&noopTool{toolName: "tool_a"})
	registry.Register(&noopTool{toolName: "tool_b"})

	provider := &sequentialProvider{
		responses: []*providers.LLMResponse{
			makeToolCallResponse("tool_a"),  // iter 1
			makeToolCallResponse("tool_b"),  // iter 2
			{Content: "done"},               // iter 3
		},
	}

	messages := []providers.Message{
		{Role: "system", Content: "sys"},
		{Role: "user", Content: "task"},
	}

	result, err := RunToolLoop(context.Background(), ToolLoopConfig{
		Provider:      provider,
		Model:         "m",
		Tools:         registry,
		MaxIterations: 10,
	}, messages, "cli", "direct")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.ToolsUsed) != 2 {
		t.Fatalf("ToolsUsed length: want 2, got %d: %v", len(result.ToolsUsed), result.ToolsUsed)
	}
}

// ── Planning step ─────────────────────────────────────────────────────────────

func TestRunToolLoop_PlanningPromptInjectsPlan(t *testing.T) {
	provider := &sequentialProvider{
		responses: []*providers.LLMResponse{
			{Content: "Step 1: analyse\nStep 2: execute"}, // planning
			{Content: "Final result"},                      // main loop
		},
	}

	messages := []providers.Message{
		{Role: "system", Content: "base prompt"},
		{Role: "user", Content: "do something"},
	}

	result, err := RunToolLoop(context.Background(), ToolLoopConfig{
		Provider:       provider,
		Model:          "m",
		MaxIterations:  5,
		PlanningPrompt: "You are a planner.",
	}, messages, "cli", "direct")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Plan must be injected into the system message (messages[0])
	if !strings.Contains(messages[0].Content, "Your Execution Plan") {
		t.Error("system message should contain 'Your Execution Plan' header")
	}
	if !strings.Contains(messages[0].Content, "Step 1: analyse") {
		t.Error("system message should contain the plan body")
	}

	// Main loop result is correct
	if result.Content != "Final result" {
		t.Errorf("Content: want 'Final result', got %q", result.Content)
	}

	// Two provider calls were made: planning + main loop
	if provider.idx != 2 {
		t.Errorf("provider.idx: want 2, got %d", provider.idx)
	}
}

func TestRunToolLoop_NoPlanningWhenPromptEmpty(t *testing.T) {
	provider := &sequentialProvider{
		responses: []*providers.LLMResponse{
			{Content: "direct answer"},
		},
	}

	messages := []providers.Message{
		{Role: "system", Content: "system"},
		{Role: "user", Content: "task"},
	}

	_, err := RunToolLoop(context.Background(), ToolLoopConfig{
		Provider:       provider,
		Model:          "m",
		MaxIterations:  5,
		PlanningPrompt: "", // empty → no planning step
	}, messages, "cli", "direct")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Only one call: the main loop (no planning call)
	if provider.idx != 1 {
		t.Errorf("provider.idx: want 1 (no planning), got %d", provider.idx)
	}
}

func TestRunToolLoop_PlanningTokensAccumulated(t *testing.T) {
	provider := &sequentialProvider{
		responses: []*providers.LLMResponse{
			// planning call
			{Content: "plan text", Usage: &providers.UsageInfo{PromptTokens: 50, CompletionTokens: 20}},
			// main loop
			{Content: "answer", Usage: &providers.UsageInfo{PromptTokens: 80, CompletionTokens: 15}},
		},
	}

	messages := []providers.Message{
		{Role: "system", Content: "sys"},
		{Role: "user", Content: "task"},
	}

	result, err := RunToolLoop(context.Background(), ToolLoopConfig{
		Provider:       provider,
		Model:          "m",
		MaxIterations:  5,
		PlanningPrompt: "plan prompt",
	}, messages, "cli", "direct")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// 50+80=130 prompt, 20+15=35 completion
	if result.InputTokens != 130 {
		t.Errorf("InputTokens: want 130, got %d", result.InputTokens)
	}
	if result.OutputTokens != 35 {
		t.Errorf("OutputTokens: want 35, got %d", result.OutputTokens)
	}
}

// ── ToolLoopConfig PlanningModel ─────────────────────────────────────────────

func TestToolLoopConfig_PlanningModelField(t *testing.T) {
	cfg := ToolLoopConfig{
		Provider:       &sequentialProvider{},
		Model:          "main-model",
		PlanningPrompt: "plan",
		PlanningModel:  "plan-model",
	}
	if cfg.PlanningModel != "plan-model" {
		t.Errorf("PlanningModel: want 'plan-model', got %q", cfg.PlanningModel)
	}
}
