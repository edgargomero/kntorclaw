package tools

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func makeSolveResult(task, mode string) *SolveResult {
	return &SolveResult{
		Answer:       "42",
		Success:      true,
		ProblemType:  "research",
		Complexity:   "medium",
		Mode:         mode,
		Model:        "test-model",
		Soul:         "soul-solver",
		Iterations:   4,
		ToolsUsed:    []string{"read_file", "exec"},
		InputTokens:  1000,
		OutputTokens: 200,
		StartedAt:    1700000000000,
		DurationMs:   3500,
		Task:         task,
		SessionKey:   "cli:direct",
	}
}

// ── SolveResult struct ───────────────────────────────────────────────────────

func TestSolveResult_Fields(t *testing.T) {
	r := makeSolveResult("find files", "practical")

	if r.Answer != "42" {
		t.Errorf("Answer: got %q", r.Answer)
	}
	if !r.Success {
		t.Error("Success should be true")
	}
	if r.ProblemType != "research" {
		t.Errorf("ProblemType: got %q", r.ProblemType)
	}
	if r.Complexity != "medium" {
		t.Errorf("Complexity: got %q", r.Complexity)
	}
	if r.Mode != "practical" {
		t.Errorf("Mode: got %q", r.Mode)
	}
	if r.Iterations != 4 {
		t.Errorf("Iterations: got %d", r.Iterations)
	}
	if len(r.ToolsUsed) != 2 {
		t.Errorf("ToolsUsed length: got %d", len(r.ToolsUsed))
	}
	if r.InputTokens != 1000 || r.OutputTokens != 200 {
		t.Errorf("tokens: in=%d out=%d", r.InputTokens, r.OutputTokens)
	}
	if r.DurationMs != 3500 {
		t.Errorf("DurationMs: got %d", r.DurationMs)
	}
	if r.SessionKey != "cli:direct" {
		t.Errorf("SessionKey: got %q", r.SessionKey)
	}
}

func TestSolveResult_JSONRoundtrip(t *testing.T) {
	orig := makeSolveResult("json test", "subtle")

	data, err := json.Marshal(orig)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded SolveResult
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.Answer != orig.Answer {
		t.Errorf("Answer: want %q, got %q", orig.Answer, decoded.Answer)
	}
	if decoded.Success != orig.Success {
		t.Errorf("Success: want %v, got %v", orig.Success, decoded.Success)
	}
	if decoded.Iterations != orig.Iterations {
		t.Errorf("Iterations: want %d, got %d", orig.Iterations, decoded.Iterations)
	}
	if decoded.InputTokens != orig.InputTokens {
		t.Errorf("InputTokens: want %d, got %d", orig.InputTokens, decoded.InputTokens)
	}
}

// ── AppendSolveQA ─────────────────────────────────────────────────────────────

func TestAppendSolveQA_CreatesFile(t *testing.T) {
	dir := t.TempDir()
	result := makeSolveResult("create test", "practical")

	if err := AppendSolveQA(dir, result); err != nil {
		t.Fatalf("AppendSolveQA: %v", err)
	}

	path := filepath.Join(dir, "memory", "solve_qa.jsonl")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("file not created: %v", err)
	}
}

func TestAppendSolveQA_ContentIsValidJSON(t *testing.T) {
	dir := t.TempDir()
	result := makeSolveResult("valid json test", "best-cost")

	if err := AppendSolveQA(dir, result); err != nil {
		t.Fatalf("AppendSolveQA: %v", err)
	}

	path := filepath.Join(dir, "memory", "solve_qa.jsonl")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	var decoded SolveResult
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("file content is not valid JSON: %v\ncontent: %s", err, data)
	}

	if decoded.Task != "valid json test" {
		t.Errorf("Task: want 'valid json test', got %q", decoded.Task)
	}
	if decoded.Mode != "best-cost" {
		t.Errorf("Mode: want 'best-cost', got %q", decoded.Mode)
	}
}

func TestAppendSolveQA_AppendsMultipleEntries(t *testing.T) {
	dir := t.TempDir()

	r1 := makeSolveResult("task one", "practical")
	r2 := makeSolveResult("task two", "subtle")

	if err := AppendSolveQA(dir, r1); err != nil {
		t.Fatalf("first append: %v", err)
	}
	if err := AppendSolveQA(dir, r2); err != nil {
		t.Fatalf("second append: %v", err)
	}

	// JSONL: each line is a separate JSON object
	path := filepath.Join(dir, "memory", "solve_qa.jsonl")
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer f.Close()

	lineCount := 0
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		var entry SolveResult
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Errorf("line %d is not valid JSON: %v", lineCount+1, err)
		}
		lineCount++
	}

	if lineCount != 2 {
		t.Errorf("expected 2 JSONL lines, got %d", lineCount)
	}
}

func TestAppendSolveQA_EachLineIsIndependent(t *testing.T) {
	dir := t.TempDir()

	results := []*SolveResult{
		makeSolveResult("t1", "practical"),
		makeSolveResult("t2", "subtle"),
		makeSolveResult("t3", "best-cost"),
	}
	for _, r := range results {
		if err := AppendSolveQA(dir, r); err != nil {
			t.Fatalf("AppendSolveQA: %v", err)
		}
	}

	path := filepath.Join(dir, "memory", "solve_qa.jsonl")
	f, _ := os.Open(path)
	defer f.Close()

	tasks := []string{}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		var entry SolveResult
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Fatalf("invalid JSON line: %v", err)
		}
		tasks = append(tasks, entry.Task)
	}

	if len(tasks) != 3 {
		t.Fatalf("want 3 entries, got %d", len(tasks))
	}
	want := []string{"t1", "t2", "t3"}
	for i, task := range tasks {
		if task != want[i] {
			t.Errorf("entry %d Task: want %q, got %q", i, want[i], task)
		}
	}
}

func TestAppendSolveQA_CreatesMemoryDir(t *testing.T) {
	dir := t.TempDir()
	// memory/ subdir does not exist yet
	memoryDir := filepath.Join(dir, "memory")
	if _, err := os.Stat(memoryDir); !os.IsNotExist(err) {
		t.Skip("memory dir already exists")
	}

	if err := AppendSolveQA(dir, makeSolveResult("dir test", "practical")); err != nil {
		t.Fatalf("AppendSolveQA: %v", err)
	}

	if _, err := os.Stat(memoryDir); err != nil {
		t.Errorf("memory dir should have been created: %v", err)
	}
}
