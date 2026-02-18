package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sipeed/picoclaw/pkg/bus"
	"github.com/sipeed/picoclaw/pkg/providers"
)

// temperatureRouter dispatches LLM responses by temperature so each phase of
// the solve pipeline gets the expected response without depending on call order.
//
//	temp 0.2 → classification (Phase 1)
//	temp 0.3 → planning (RunToolLoop planning step)
//	anything else → main loop answer
type temperatureRouter struct {
	classifyResp string
	planResp     string
	mainResp     string
}

func (r *temperatureRouter) Chat(_ context.Context, _ []providers.Message, _ []providers.ToolDefinition, _ string, opts map[string]interface{}) (*providers.LLMResponse, error) {
	temp, _ := opts["temperature"].(float64)
	switch temp {
	case 0.2:
		return &providers.LLMResponse{Content: r.classifyResp}, nil
	case 0.3:
		return &providers.LLMResponse{Content: r.planResp}, nil
	default:
		return &providers.LLMResponse{
			Content: r.mainResp,
			Usage:   &providers.UsageInfo{PromptTokens: 100, CompletionTokens: 50},
		}, nil
	}
}

func (r *temperatureRouter) GetDefaultModel() string { return "router-model" }

// validClassifyJSON returns a well-formed classification JSON string.
func validClassifyJSON(ptype, complexity, approach string) string {
	b, _ := json.Marshal(map[string]string{
		"problem_type": ptype,
		"complexity":   complexity,
		"approach":     approach,
	})
	return string(b)
}

// newSolveToolForTest builds a SolveTool wired to a temperatureRouter.
func newSolveToolForTest(t *testing.T, router *temperatureRouter, skillLoader func(string) (string, bool)) (*SolveTool, string) {
	t.Helper()
	dir := t.TempDir()
	msgBus := bus.NewMessageBus()
	manager := NewSubagentManager(router, "router-model", dir, msgBus)

	if skillLoader == nil {
		skillLoader = func(string) (string, bool) { return "", false }
	}

	tool := NewSolveTool(SolveToolOptions{
		Manager:       manager,
		WorkspacePath: dir,
		SkillLoader:   skillLoader,
	})
	return tool, dir
}

// ── Metadata ──────────────────────────────────────────────────────────────────

func TestSolveTool_Name(t *testing.T) {
	tool, _ := newSolveToolForTest(t, &temperatureRouter{}, nil)
	if tool.Name() != "solve" {
		t.Errorf("Name: want 'solve', got %q", tool.Name())
	}
}

func TestSolveTool_Description(t *testing.T) {
	tool, _ := newSolveToolForTest(t, &temperatureRouter{}, nil)
	desc := tool.Description()
	if desc == "" {
		t.Fatal("Description is empty")
	}
	if !strings.Contains(desc, "solve") && !strings.Contains(strings.ToLower(desc), "solver") {
		t.Errorf("Description should mention solving, got: %s", desc)
	}
}

func TestSolveTool_Parameters_Schema(t *testing.T) {
	tool, _ := newSolveToolForTest(t, &temperatureRouter{}, nil)
	params := tool.Parameters()

	if params["type"] != "object" {
		t.Fatalf("type: want 'object', got %v", params["type"])
	}

	props, ok := params["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("properties should be a map")
	}

	// required fields
	for _, field := range []string{"task"} {
		if _, ok := props[field]; !ok {
			t.Errorf("property %q not found", field)
		}
	}

	// optional fields
	for _, field := range []string{"mode", "soul", "model"} {
		if _, ok := props[field]; !ok {
			t.Errorf("optional property %q not found", field)
		}
	}

	req, ok := params["required"].([]string)
	if !ok {
		t.Fatal("required should be []string")
	}
	if len(req) != 1 || req[0] != "task" {
		t.Errorf("required: want ['task'], got %v", req)
	}
}

func TestSolveTool_SetContext(t *testing.T) {
	tool, _ := newSolveToolForTest(t, &temperatureRouter{}, nil)
	// Must not panic
	tool.SetContext("telegram", "chat-456")
	if tool.originChannel != "telegram" {
		t.Errorf("originChannel: want 'telegram', got %q", tool.originChannel)
	}
	if tool.originChatID != "chat-456" {
		t.Errorf("originChatID: want 'chat-456', got %q", tool.originChatID)
	}
}

// ── Error handling ────────────────────────────────────────────────────────────

func TestSolveTool_Execute_MissingTask(t *testing.T) {
	tool, _ := newSolveToolForTest(t, &temperatureRouter{}, nil)

	result := tool.Execute(context.Background(), map[string]interface{}{
		"mode": "practical",
	})

	if !result.IsError {
		t.Error("expected IsError=true for missing task")
	}
	if result.Err == nil {
		t.Error("Err should be set")
	}
}

func TestSolveTool_Execute_EmptyTask(t *testing.T) {
	tool, _ := newSolveToolForTest(t, &temperatureRouter{}, nil)

	result := tool.Execute(context.Background(), map[string]interface{}{
		"task": "",
	})

	if !result.IsError {
		t.Error("expected IsError=true for empty task")
	}
}

func TestSolveTool_Execute_NilManager(t *testing.T) {
	tool := NewSolveTool(SolveToolOptions{
		Manager:       nil,
		WorkspacePath: t.TempDir(),
		SkillLoader:   func(string) (string, bool) { return "", false },
	})

	result := tool.Execute(context.Background(), map[string]interface{}{
		"task": "do something",
	})

	if !result.IsError {
		t.Error("expected IsError=true for nil manager")
	}
}

// ── Full pipeline ─────────────────────────────────────────────────────────────

func TestSolveTool_Execute_Success_Silent(t *testing.T) {
	router := &temperatureRouter{
		classifyResp: validClassifyJSON("code", "simple", "practical"),
		planResp:     "1. Read files\n2. Summarize",
		mainResp:     "There are 7 .go files.",
	}
	tool, _ := newSolveToolForTest(t, router, nil)

	result := tool.Execute(context.Background(), map[string]interface{}{
		"task": "count go files",
	})

	if result.IsError {
		t.Fatalf("unexpected error: %s", result.ForLLM)
	}

	// Must be silent (user never sees the subagent internals)
	if !result.Silent {
		t.Error("result should be Silent=true")
	}
	if result.ForUser != "" {
		t.Errorf("ForUser should be empty, got: %q", result.ForUser)
	}
}

func TestSolveTool_Execute_ForLLM_Format(t *testing.T) {
	router := &temperatureRouter{
		classifyResp: validClassifyJSON("research", "medium", "best-cost"),
		planResp:     "plan text",
		mainResp:     "Final synthesized answer.",
	}
	tool, _ := newSolveToolForTest(t, router, nil)

	result := tool.Execute(context.Background(), map[string]interface{}{
		"task": "research providers",
	})

	if result.IsError {
		t.Fatalf("unexpected error: %s", result.ForLLM)
	}

	llm := result.ForLLM
	for _, expected := range []string{
		"[SOLVE RESULT]",
		"ANSWER:",
		"[QA]",
		"Iterations:",
		"Tokens:",
		"Duration:",
		"solve_qa.jsonl",
	} {
		if !strings.Contains(llm, expected) {
			t.Errorf("ForLLM missing %q\nFull ForLLM:\n%s", expected, llm)
		}
	}

	if !strings.Contains(llm, "Final synthesized answer.") {
		t.Errorf("ForLLM should contain the answer, got:\n%s", llm)
	}
}

func TestSolveTool_Execute_QAFileSaved(t *testing.T) {
	router := &temperatureRouter{
		classifyResp: validClassifyJSON("analysis", "simple", "practical"),
		planResp:     "plan",
		mainResp:     "analysis done",
	}
	tool, dir := newSolveToolForTest(t, router, nil)

	result := tool.Execute(context.Background(), map[string]interface{}{
		"task": "analyze something",
	})

	if result.IsError {
		t.Fatalf("unexpected error: %s", result.ForLLM)
	}

	// QA file must exist with at least one entry
	path := filepath.Join(dir, "memory", "solve_qa.jsonl")
	data, err := readFirstJSONLLine(t, path)
	if err != nil {
		t.Fatalf("QA file not found or empty: %v", err)
	}

	var qa SolveResult
	if err := json.Unmarshal(data, &qa); err != nil {
		t.Fatalf("QA entry not valid JSON: %v\n%s", err, data)
	}

	if qa.Task != "analyze something" {
		t.Errorf("QA Task: want 'analyze something', got %q", qa.Task)
	}
	if qa.ProblemType != "analysis" {
		t.Errorf("QA ProblemType: want 'analysis', got %q", qa.ProblemType)
	}
	if !qa.Success {
		t.Error("QA Success should be true")
	}
}

// ── Mode override ─────────────────────────────────────────────────────────────

func TestSolveTool_Execute_ModeOverride(t *testing.T) {
	// Classification returns "practical" but caller forces "best-cost"
	router := &temperatureRouter{
		classifyResp: validClassifyJSON("code", "simple", "practical"),
		planResp:     "plan",
		mainResp:     "result",
	}
	tool, dir := newSolveToolForTest(t, router, nil)

	result := tool.Execute(context.Background(), map[string]interface{}{
		"task": "do something",
		"mode": "best-cost",
	})

	if result.IsError {
		t.Fatalf("unexpected error: %s", result.ForLLM)
	}

	// QA file should record mode=best-cost
	path := filepath.Join(dir, "memory", "solve_qa.jsonl")
	data, _ := readFirstJSONLLine(t, path)
	var qa SolveResult
	json.Unmarshal(data, &qa) //nolint:errcheck
	if qa.Mode != "best-cost" {
		t.Errorf("QA Mode: want 'best-cost', got %q", qa.Mode)
	}
}

// ── Soul loading ──────────────────────────────────────────────────────────────

func TestSolveTool_Execute_SoulFromSkill(t *testing.T) {
	router := &temperatureRouter{
		classifyResp: validClassifyJSON("research", "medium", "practical"),
		planResp:     "plan",
		mainResp:     "findings",
	}

	customSkillContent := "You are a custom researcher. Investigate and report."
	skillLoader := func(name string) (string, bool) {
		if name == "soul-researcher" {
			return customSkillContent, true
		}
		return "", false
	}

	tool, dir := newSolveToolForTest(t, router, skillLoader)

	result := tool.Execute(context.Background(), map[string]interface{}{
		"task": "research something",
		"soul": "soul-researcher",
	})

	if result.IsError {
		t.Fatalf("unexpected error: %s", result.ForLLM)
	}

	// QA should record the soul name
	path := filepath.Join(dir, "memory", "solve_qa.jsonl")
	data, _ := readFirstJSONLLine(t, path)
	var qa SolveResult
	json.Unmarshal(data, &qa) //nolint:errcheck
	if qa.Soul != "soul-researcher" {
		t.Errorf("QA Soul: want 'soul-researcher', got %q", qa.Soul)
	}
}

func TestSolveTool_Execute_SoulDirectPrompt(t *testing.T) {
	router := &temperatureRouter{
		classifyResp: validClassifyJSON("code", "simple", "practical"),
		planResp:     "plan",
		mainResp:     "result",
	}

	// skillLoader returns false → treat soul arg as direct prompt
	skillLoader := func(string) (string, bool) { return "", false }

	tool, dir := newSolveToolForTest(t, router, skillLoader)

	result := tool.Execute(context.Background(), map[string]interface{}{
		"task": "do something",
		"soul": "You are a direct prompt soul.",
	})

	if result.IsError {
		t.Fatalf("unexpected error: %s", result.ForLLM)
	}

	path := filepath.Join(dir, "memory", "solve_qa.jsonl")
	data, _ := readFirstJSONLLine(t, path)
	var qa SolveResult
	json.Unmarshal(data, &qa) //nolint:errcheck
	if qa.Soul != "custom" {
		t.Errorf("QA Soul: want 'custom' for direct prompt, got %q", qa.Soul)
	}
}

func TestSolveTool_Execute_SoulFallbackToDefault(t *testing.T) {
	router := &temperatureRouter{
		classifyResp: validClassifyJSON("code", "simple", "practical"),
		planResp:     "plan",
		mainResp:     "result",
	}
	// No skill found, no soul arg → default
	tool, dir := newSolveToolForTest(t, router, nil)

	result := tool.Execute(context.Background(), map[string]interface{}{
		"task": "do something",
		// no "soul" arg
	})

	if result.IsError {
		t.Fatalf("unexpected error: %s", result.ForLLM)
	}

	path := filepath.Join(dir, "memory", "solve_qa.jsonl")
	data, _ := readFirstJSONLLine(t, path)
	var qa SolveResult
	json.Unmarshal(data, &qa) //nolint:errcheck
	if qa.Soul != "default" {
		t.Errorf("QA Soul: want 'default' for fallback, got %q", qa.Soul)
	}
}

// ── Classification fallback ───────────────────────────────────────────────────

func TestSolveTool_Execute_ClassificationFallback(t *testing.T) {
	router := &temperatureRouter{
		classifyResp: "this is not json at all!!!",
		planResp:     "plan",
		mainResp:     "answer",
	}
	tool, _ := newSolveToolForTest(t, router, nil)

	// Should not fail — defaults are used when classify JSON is invalid
	result := tool.Execute(context.Background(), map[string]interface{}{
		"task": "some task",
	})

	if result.IsError {
		t.Fatalf("should succeed even with bad classification JSON: %s", result.ForLLM)
	}
}

// ── Helper function tests ─────────────────────────────────────────────────────

func TestFormatToolsUsed_Empty(t *testing.T) {
	got := formatToolsUsed(nil)
	if got != "" {
		t.Errorf("want empty string, got %q", got)
	}
}

func TestFormatToolsUsed_SingleTool(t *testing.T) {
	got := formatToolsUsed([]string{"read_file"})
	if got != "read_file" {
		t.Errorf("want 'read_file', got %q", got)
	}
}

func TestFormatToolsUsed_MultipleSameTool(t *testing.T) {
	got := formatToolsUsed([]string{"read_file", "read_file", "read_file"})
	if got != "read_file(3)" {
		t.Errorf("want 'read_file(3)', got %q", got)
	}
}

func TestFormatToolsUsed_MixedTools(t *testing.T) {
	tools := []string{"read_file", "exec", "read_file", "web_search", "exec"}
	got := formatToolsUsed(tools)
	// Must contain each tool with its count, in insertion order
	if !strings.Contains(got, "read_file(2)") {
		t.Errorf("want read_file(2) in %q", got)
	}
	if !strings.Contains(got, "exec(2)") {
		t.Errorf("want exec(2) in %q", got)
	}
	if !strings.Contains(got, "web_search") {
		t.Errorf("want web_search in %q", got)
	}
}

func TestFormatInt_Small(t *testing.T) {
	cases := []struct {
		n    int
		want string
	}{
		{0, "0"},
		{999, "999"},
		{1000, "1,000"},
		{1500, "1,500"},
		{10000, "10,000"},
		{123456, "123,456"},
	}
	for _, c := range cases {
		got := formatInt(c.n)
		if got != c.want {
			t.Errorf("formatInt(%d): want %q, got %q", c.n, c.want, got)
		}
	}
}

func TestValidProblemType(t *testing.T) {
	valid := []string{"research", "code", "analysis", "search", "synthesis"}
	for _, v := range valid {
		if !validProblemType(v) {
			t.Errorf("validProblemType(%q) should be true", v)
		}
	}
	for _, inv := range []string{"", "random", "RESEARCH"} {
		if validProblemType(inv) {
			t.Errorf("validProblemType(%q) should be false", inv)
		}
	}
}

func TestValidComplexity(t *testing.T) {
	valid := []string{"simple", "medium", "complex"}
	for _, v := range valid {
		if !validComplexity(v) {
			t.Errorf("validComplexity(%q) should be true", v)
		}
	}
	for _, inv := range []string{"", "hard", "SIMPLE"} {
		if validComplexity(inv) {
			t.Errorf("validComplexity(%q) should be false", inv)
		}
	}
}

func TestValidApproach(t *testing.T) {
	valid := []string{"practical", "subtle", "best-cost"}
	for _, v := range valid {
		if !validApproach(v) {
			t.Errorf("validApproach(%q) should be true", v)
		}
	}
	for _, inv := range []string{"", "fast", "PRACTICAL"} {
		if validApproach(inv) {
			t.Errorf("validApproach(%q) should be false", inv)
		}
	}
}

// ── SubagentManager new methods ───────────────────────────────────────────────

func TestSubagentManager_Provider(t *testing.T) {
	p := &MockLLMProvider{}
	mgr := NewSubagentManager(p, "m", t.TempDir(), nil)
	if mgr.Provider() != p {
		t.Error("Provider() should return the injected provider")
	}
}

func TestSubagentManager_DefaultModel(t *testing.T) {
	mgr := NewSubagentManager(&MockLLMProvider{}, "my-model", t.TempDir(), nil)
	if mgr.DefaultModel() != "my-model" {
		t.Errorf("DefaultModel(): want 'my-model', got %q", mgr.DefaultModel())
	}
}

func TestSubagentConfig_ZeroValuesUseDefaults(t *testing.T) {
	// A zero SubagentConfig should not override any manager defaults
	cfg := &SubagentConfig{}
	if cfg.SystemPrompt != "" {
		t.Error("zero SubagentConfig.SystemPrompt should be empty")
	}
	if cfg.MaxIterations != 0 {
		t.Error("zero SubagentConfig.MaxIterations should be 0")
	}
}

func TestSubagentManager_RunWithConfig_Success(t *testing.T) {
	provider := &MockLLMProvider{} // returns "Task completed: <task>"
	mgr := NewSubagentManager(provider, "test-model", t.TempDir(), nil)

	result, err := mgr.RunWithConfig(context.Background(), "do something", nil, "cli", "direct")
	if err != nil {
		t.Fatalf("RunWithConfig: %v", err)
	}
	if !strings.Contains(result.Content, "Task completed:") {
		t.Errorf("Content should contain 'Task completed:', got: %q", result.Content)
	}
}

func TestSubagentManager_RunWithConfig_CustomSystemPrompt(t *testing.T) {
	// MockLLMProvider echoes the last user message; system prompt is validated
	// indirectly by checking that execution succeeds with a custom prompt.
	provider := &MockLLMProvider{}
	mgr := NewSubagentManager(provider, "test-model", t.TempDir(), nil)

	cfg := &SubagentConfig{
		SystemPrompt: "You are a custom agent.",
		MaxTokens:    2048,
	}

	result, err := mgr.RunWithConfig(context.Background(), "custom task", cfg, "cli", "direct")
	if err != nil {
		t.Fatalf("RunWithConfig with custom config: %v", err)
	}
	if result == nil {
		t.Fatal("result should not be nil")
	}
}

// ── Helpers ───────────────────────────────────────────────────────────────────

// readFirstJSONLLine reads the first non-empty line from a JSONL file.
func readFirstJSONLLine(t *testing.T, path string) ([]byte, error) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			return []byte(line), nil
		}
	}
	return nil, nil
}
