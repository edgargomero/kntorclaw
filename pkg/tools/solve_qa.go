package tools

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// SolveResult is the enriched output of the solve tool, containing both the
// final answer and the control metadata needed for multivariable QA analysis.
type SolveResult struct {
	// Result
	Answer  string `json:"answer"`
	Success bool   `json:"success"`

	// Classification (produced by Phase 1)
	ProblemType string `json:"problem_type"` // "research"|"code"|"analysis"|"search"|"synthesis"
	Complexity  string `json:"complexity"`   // "simple"|"medium"|"complex"
	Mode        string `json:"mode"`         // "practical"|"subtle"|"best-cost"

	// Execution
	Model      string   `json:"model"`
	Soul       string   `json:"soul"`
	Iterations int      `json:"iterations"`
	ToolsUsed  []string `json:"tools_used"`

	// Tokens
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`

	// Timing
	StartedAt  int64 `json:"started_at"`  // Unix ms
	DurationMs int64 `json:"duration_ms"`

	// Origin
	Task       string `json:"task"`
	SessionKey string `json:"session_key"` // "channel:chatID"
}

// AppendSolveQA appends a SolveResult as a single JSON line to
// workspace/memory/solve_qa.jsonl. The JSONL format makes it easy to
// analyse with jq, pandas, or any line-oriented tool.
func AppendSolveQA(workspacePath string, result *SolveResult) error {
	path := filepath.Join(workspacePath, "memory", "solve_qa.jsonl")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	data, err := json.Marshal(result)
	if err != nil {
		return err
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.Write(append(data, '\n'))
	return err
}
