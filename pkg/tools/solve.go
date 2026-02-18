package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/sipeed/picoclaw/pkg/logger"
	"github.com/sipeed/picoclaw/pkg/providers"
)

// defaultSolverPrompt is used when no soul skill is found and no soul
// string is provided directly.
const defaultSolverPrompt = `You are a specialized problem-solving agent.
Investigate thoroughly, use tools systematically, and deliver a clean result.
Return ONLY the result. Never explain your process. Never apologize.
If the task was impossible, say exactly why in one sentence.`

// maxIterationsForComplexity maps classification complexity to max iterations.
var maxIterationsForComplexity = map[string]int{
	"simple":  5,
	"medium":  10,
	"complex": 15,
}

// classifyResult holds the output of Phase 1 (classification).
type classifyResult struct {
	ProblemType string `json:"problem_type"`
	Complexity  string `json:"complexity"`
	Approach    string `json:"approach"`
}

// SolveToolOptions contains constructor arguments for SolveTool.
type SolveToolOptions struct {
	Manager       *SubagentManager
	WorkspacePath string
	// SkillLoader loads a skill's body content (frontmatter stripped) by name.
	// Returns ("", false) when the skill is not found.
	SkillLoader func(name string) (string, bool)
}

// SolveTool is a fully autonomous problem solver. It runs a 4-phase pipeline
// internally (classify → load soul → execute → persist QA) and returns the
// result silently to the parent agent.
type SolveTool struct {
	manager       *SubagentManager
	workspacePath string
	skillLoader   func(name string) (string, bool)
	originChannel string
	originChatID  string
}

// NewSolveTool creates a SolveTool with the given options.
func NewSolveTool(opts SolveToolOptions) *SolveTool {
	return &SolveTool{
		manager:       opts.Manager,
		workspacePath: opts.WorkspacePath,
		skillLoader:   opts.SkillLoader,
		originChannel: "cli",
		originChatID:  "direct",
	}
}

func (t *SolveTool) Name() string { return "solve" }

func (t *SolveTool) Description() string {
	return "Autonomous problem solver. Classifies the task, selects optimal approach and soul, " +
		"executes with specialized tools, and returns the result silently to the parent agent. " +
		"Use for complex, multi-step, or research-heavy sub-tasks."
}

func (t *SolveTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"task": map[string]interface{}{
				"type":        "string",
				"description": "The specific problem to solve",
			},
			"mode": map[string]interface{}{
				"type":        "string",
				"enum":        []string{"practical", "subtle", "best-cost"},
				"description": "Solving approach. Omit to let the agent classify automatically",
			},
			"soul": map[string]interface{}{
				"type":        "string",
				"description": "Preset skill name ('soul-researcher', 'soul-solver') or a direct system prompt string. Defaults to 'soul-solver'",
			},
			"model": map[string]interface{}{
				"type":        "string",
				"description": "Model override (e.g. 'claude-haiku-4-5-20251001'). Defaults to the manager's default model",
			},
		},
		"required": []string{"task"},
	}
}

// SetContext implements ContextualTool so the agent loop can inject the
// current channel and chatID before each execution.
func (t *SolveTool) SetContext(channel, chatID string) {
	t.originChannel = channel
	t.originChatID = chatID
}

// Execute runs the 4-phase solve pipeline synchronously.
func (t *SolveTool) Execute(ctx context.Context, args map[string]interface{}) *ToolResult {
	task, ok := args["task"].(string)
	if !ok || task == "" {
		return ErrorResult("task parameter is required").WithError(fmt.Errorf("task is empty"))
	}
	if t.manager == nil {
		return ErrorResult("solve: subagent manager not configured").WithError(fmt.Errorf("manager is nil"))
	}

	modeArg, _ := args["mode"].(string)
	soulArg, _ := args["soul"].(string)
	modelArg, _ := args["model"].(string)

	startedAt := time.Now()

	// ── Phase 1: Classify ────────────────────────────────────────────────────
	classification := t.classify(ctx, task, modeArg)

	// If caller provided mode explicitly, honour it
	if modeArg != "" {
		classification.Approach = modeArg
	}

	logger.DebugCF("solve", "Task classified",
		map[string]any{
			"problem_type": classification.ProblemType,
			"complexity":   classification.Complexity,
			"approach":     classification.Approach,
		})

	// ── Phase 2: Load soul ───────────────────────────────────────────────────
	soulName, soulContent := t.loadSoul(soulArg)

	// ── Phase 3: Execute ─────────────────────────────────────────────────────
	maxIter := maxIterationsForComplexity[classification.Complexity]
	if maxIter == 0 {
		maxIter = 10
	}

	planningPrompt := fmt.Sprintf(
		"You are planning execution for a %s task of %s complexity using the %s approach.\n"+
			"Create a concise numbered plan before starting. Focus on the actual steps needed.",
		classification.ProblemType, classification.Complexity, classification.Approach,
	)

	model := t.manager.DefaultModel()
	if modelArg != "" {
		model = modelArg
	}

	cfg := &SubagentConfig{
		SystemPrompt:   soulContent,
		Model:          model,
		MaxIterations:  maxIter,
		MaxTokens:      6144,
		PlanningPrompt: planningPrompt,
	}

	loopResult, err := t.manager.RunWithConfig(ctx, task, cfg, t.originChannel, t.originChatID)

	durationMs := time.Since(startedAt).Milliseconds()
	sessionKey := fmt.Sprintf("%s:%s", t.originChannel, t.originChatID)

	// ── Phase 4: QA + Result ─────────────────────────────────────────────────
	success := err == nil
	answer := ""
	if loopResult != nil {
		answer = loopResult.Content
	}
	if err != nil {
		answer = fmt.Sprintf("Error: %v", err)
	}

	qa := &SolveResult{
		Answer:       answer,
		Success:      success,
		ProblemType:  classification.ProblemType,
		Complexity:   classification.Complexity,
		Mode:         classification.Approach,
		Model:        model,
		Soul:         soulName,
		Iterations:   0,
		ToolsUsed:    []string{},
		StartedAt:    startedAt.UnixMilli(),
		DurationMs:   durationMs,
		Task:         task,
		SessionKey:   sessionKey,
	}
	if loopResult != nil {
		qa.Iterations = loopResult.Iterations
		qa.ToolsUsed = loopResult.ToolsUsed
		qa.InputTokens = loopResult.InputTokens
		qa.OutputTokens = loopResult.OutputTokens
	}

	if appendErr := AppendSolveQA(t.workspacePath, qa); appendErr != nil {
		logger.WarnCF("solve", "Failed to persist QA data",
			map[string]any{"error": appendErr.Error()})
	}

	// Build the structured ForLLM block for the parent agent
	forLLM := t.buildForLLM(qa)

	return &ToolResult{
		ForLLM:  forLLM,
		ForUser: "",
		Silent:  true,
		IsError: !success,
	}
}

// classify calls the LLM once (no tools) to classify the task.
// Falls back to safe defaults if the call fails or parsing fails.
func (t *SolveTool) classify(ctx context.Context, task, modeHint string) classifyResult {
	defaults := classifyResult{
		ProblemType: "analysis",
		Complexity:  "medium",
		Approach:    "practical",
	}
	if modeHint != "" {
		defaults.Approach = modeHint
	}

	classifySystem := `You are a task classifier. Analyze the task and return ONLY a JSON object with these exact fields:
- "problem_type": one of "research", "code", "analysis", "search", "synthesis"
- "complexity": one of "simple", "medium", "complex"
- "approach": one of "practical", "subtle", "best-cost"

Return ONLY the JSON object, no markdown, no explanation.`

	classifyUser := fmt.Sprintf("Classify this task:\n\n%s", task)
	if modeHint != "" {
		classifyUser += fmt.Sprintf("\n\nNote: the caller has requested approach=%q, use it in your response.", modeHint)
	}

	provider := t.manager.Provider()
	if provider == nil {
		return defaults
	}

	resp, err := provider.Chat(ctx,
		[]providers.Message{
			{Role: "system", Content: classifySystem},
			{Role: "user", Content: classifyUser},
		},
		nil,
		t.manager.DefaultModel(),
		map[string]any{"max_tokens": 300, "temperature": 0.2},
	)
	if err != nil || resp == nil || resp.Content == "" {
		return defaults
	}

	// Extract JSON from the response (handle markdown fences)
	raw := resp.Content
	if start := strings.Index(raw, "{"); start >= 0 {
		if end := strings.LastIndex(raw, "}"); end > start {
			raw = raw[start : end+1]
		}
	}

	var result classifyResult
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		logger.DebugCF("solve", "Classification parse failed, using defaults",
			map[string]any{"raw": resp.Content, "error": err.Error()})
		return defaults
	}

	// Sanitize values
	if !validProblemType(result.ProblemType) {
		result.ProblemType = defaults.ProblemType
	}
	if !validComplexity(result.Complexity) {
		result.Complexity = defaults.Complexity
	}
	if !validApproach(result.Approach) {
		result.Approach = defaults.Approach
	}

	return result
}

// loadSoul returns the name and content of the soul to use.
// Priority: explicit soul arg > skill lookup > fallback to default prompt.
func (t *SolveTool) loadSoul(soulArg string) (name, content string) {
	soulName := soulArg
	if soulName == "" {
		soulName = "soul-solver"
	}

	// Try to load as a named skill first
	if t.skillLoader != nil {
		if skillContent, ok := t.skillLoader(soulName); ok {
			return soulName, skillContent
		}
	}

	// If soulArg was provided but not found as a skill, treat it as a direct prompt
	if soulArg != "" {
		return "custom", soulArg
	}

	// Final fallback: built-in default
	return "default", defaultSolverPrompt
}

// buildForLLM constructs the structured result block sent to the parent agent.
func (t *SolveTool) buildForLLM(qa *SolveResult) string {
	modeAuto := "auto-classified"
	toolsSummary := formatToolsUsed(qa.ToolsUsed)

	var sb strings.Builder
	sb.WriteString("[SOLVE RESULT]\n")
	sb.WriteString(fmt.Sprintf("Task: %s\n", qa.Task))
	sb.WriteString(fmt.Sprintf("Mode: %s (%s)\n", qa.Mode, modeAuto))
	sb.WriteString(fmt.Sprintf("Problem Type: %s\n", qa.ProblemType))
	sb.WriteString(fmt.Sprintf("Model: %s\n", qa.Model))
	sb.WriteString("\nANSWER:\n")
	sb.WriteString(qa.Answer)
	sb.WriteString("\n\n[QA]\n")
	sb.WriteString(fmt.Sprintf("Iterations: %d\n", qa.Iterations))
	if toolsSummary != "" {
		sb.WriteString(fmt.Sprintf("Tools used: %s\n", toolsSummary))
	}
	sb.WriteString(fmt.Sprintf("Tokens: %s in / %s out\n",
		formatInt(qa.InputTokens), formatInt(qa.OutputTokens)))
	sb.WriteString(fmt.Sprintf("Duration: %.1fs\n", float64(qa.DurationMs)/1000))
	sb.WriteString("Saved to: workspace/memory/solve_qa.jsonl\n")
	return sb.String()
}

// formatToolsUsed returns a compact summary like "read_file(3), exec(2)".
func formatToolsUsed(toolsUsed []string) string {
	if len(toolsUsed) == 0 {
		return ""
	}
	counts := make(map[string]int)
	order := []string{}
	for _, t := range toolsUsed {
		if counts[t] == 0 {
			order = append(order, t)
		}
		counts[t]++
	}
	parts := make([]string, 0, len(order))
	for _, name := range order {
		if counts[name] > 1 {
			parts = append(parts, fmt.Sprintf("%s(%d)", name, counts[name]))
		} else {
			parts = append(parts, name)
		}
	}
	return strings.Join(parts, ", ")
}

// formatInt formats an integer with thousands separators.
func formatInt(n int) string {
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}
	s := fmt.Sprintf("%d", n)
	result := ""
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			result += ","
		}
		result += string(c)
	}
	return result
}

func validProblemType(s string) bool {
	switch s {
	case "research", "code", "analysis", "search", "synthesis":
		return true
	}
	return false
}

func validComplexity(s string) bool {
	switch s {
	case "simple", "medium", "complex":
		return true
	}
	return false
}

func validApproach(s string) bool {
	switch s {
	case "practical", "subtle", "best-cost":
		return true
	}
	return false
}
