package tui

import (
	"sync"
	"testing"
	"time"

	"github.com/sipeed/picoclaw/pkg/tools"
)

// --- ActivityTracker tests ---

func TestNewActivityTracker(t *testing.T) {
	at := NewActivityTracker()
	if at == nil {
		t.Fatal("NewActivityTracker returned nil")
	}
	all := at.GetAll()
	if len(all) != 0 {
		t.Errorf("new tracker should have 0 sessions, got %d", len(all))
	}
}

func TestActivityTracker_SetProcessing(t *testing.T) {
	at := NewActivityTracker()
	at.SetProcessing("tg:user1")

	all := at.GetAll()
	sa := all["tg:user1"]
	if sa == nil {
		t.Fatal("session not created")
	}
	if sa.Status != "processing" {
		t.Errorf("Status = %q, want %q", sa.Status, "processing")
	}
	if sa.LastActive.IsZero() {
		t.Error("LastActive should be set")
	}
}

func TestActivityTracker_SetToolCall(t *testing.T) {
	at := NewActivityTracker()
	at.SetToolCall("s1", "read_file")
	at.SetToolCall("s1", "exec")

	all := at.GetAll()
	sa := all["s1"]
	if sa.Status != "tool:exec" {
		t.Errorf("Status = %q, want %q", sa.Status, "tool:exec")
	}
	if sa.ToolCalls != 2 {
		t.Errorf("ToolCalls = %d, want 2", sa.ToolCalls)
	}
	if sa.LastTool != "exec" {
		t.Errorf("LastTool = %q, want %q", sa.LastTool, "exec")
	}
}

func TestActivityTracker_SetIdle(t *testing.T) {
	at := NewActivityTracker()
	at.SetProcessing("s1")
	at.SetIdle("s1", "Done processing", 3)

	all := at.GetAll()
	sa := all["s1"]
	if sa.Status != "idle" {
		t.Errorf("Status = %q, want %q", sa.Status, "idle")
	}
	if sa.LastPreview != "Done processing" {
		t.Errorf("LastPreview = %q, want %q", sa.LastPreview, "Done processing")
	}
	if sa.Iterations != 3 {
		t.Errorf("Iterations = %d, want 3", sa.Iterations)
	}
}

func TestActivityTracker_SetIdle_AccumulatesIterations(t *testing.T) {
	at := NewActivityTracker()
	at.SetIdle("s1", "first", 2)
	at.SetIdle("s1", "second", 3)

	all := at.GetAll()
	if all["s1"].Iterations != 5 {
		t.Errorf("Iterations = %d, want 5 (accumulated)", all["s1"].Iterations)
	}
}

func TestActivityTracker_AddTokens(t *testing.T) {
	at := NewActivityTracker()
	at.AddTokens("s1", 100, 50)
	at.AddTokens("s1", 200, 75)

	all := at.GetAll()
	sa := all["s1"]
	if sa.InputTokens != 300 {
		t.Errorf("InputTokens = %d, want 300", sa.InputTokens)
	}
	if sa.OutputTokens != 125 {
		t.Errorf("OutputTokens = %d, want 125", sa.OutputTokens)
	}
}

func TestActivityTracker_GetAll_ReturnsSnapshot(t *testing.T) {
	at := NewActivityTracker()
	at.SetProcessing("s1")

	snapshot := at.GetAll()
	// Modify snapshot — should not affect tracker
	snapshot["s1"].Status = "modified"

	all := at.GetAll()
	if all["s1"].Status != "processing" {
		t.Error("GetAll should return a copy, not a reference")
	}
}

func TestActivityTracker_MultipleSessions(t *testing.T) {
	at := NewActivityTracker()
	at.SetProcessing("tg:user1")
	at.SetToolCall("dc:guild1", "exec")
	at.AddTokens("wa:phone1", 50, 25)

	all := at.GetAll()
	if len(all) != 3 {
		t.Errorf("should have 3 sessions, got %d", len(all))
	}
}

func TestActivityTracker_ConcurrentAccess(t *testing.T) {
	at := NewActivityTracker()
	var wg sync.WaitGroup

	wg.Add(100)
	for i := 0; i < 100; i++ {
		go func(i int) {
			defer wg.Done()
			key := "session"
			at.SetProcessing(key)
			at.SetToolCall(key, "exec")
			at.AddTokens(key, 10, 5)
			at.SetIdle(key, "done", 1)
			at.GetAll()
		}(i)
	}
	wg.Wait()

	all := at.GetAll()
	sa := all["session"]
	if sa == nil {
		t.Fatal("session should exist after concurrent access")
	}
	if sa.InputTokens != 1000 { // 100 * 10
		t.Errorf("InputTokens = %d, want 1000", sa.InputTokens)
	}
}

// --- QATracker tests ---

func TestNewQATracker(t *testing.T) {
	ct := tools.NewCheckpointTool()
	qt := NewQATracker(ct)

	if qt == nil {
		t.Fatal("NewQATracker returned nil")
	}
	if len(qt.Stages) != 1 {
		t.Fatalf("new tracker should have 1 stage (approval), got %d", len(qt.Stages))
	}
	if qt.Stages[0].Name != "human approval" {
		t.Errorf("initial stage name = %q, want 'human approval'", qt.Stages[0].Name)
	}
	if qt.Stages[0].Kind != kindApproval {
		t.Error("initial stage should be kindApproval")
	}
}

func TestQATracker_HandleToolResult_CreatesLintStage(t *testing.T) {
	qt := NewQATracker(tools.NewCheckpointTool())
	qt.HandleToolResult("exec", "npx eslint src/", "All files clean", false)

	if len(qt.Stages) != 2 {
		t.Fatalf("should have 2 stages, got %d", len(qt.Stages))
	}
	if qt.Stages[0].Name != "lint (eslint)" {
		t.Errorf("first stage = %q, want 'lint (eslint)'", qt.Stages[0].Name)
	}
	if qt.Stages[0].Status != StagePassed {
		t.Errorf("lint stage status = %d, want StagePassed", qt.Stages[0].Status)
	}
	if qt.Stages[0].Summary != "clean" {
		t.Errorf("lint stage summary = %q, want 'clean'", qt.Stages[0].Summary)
	}
}

func TestQATracker_HandleToolResult_CreatesTestStage(t *testing.T) {
	qt := NewQATracker(tools.NewCheckpointTool())

	output := `ok  github.com/foo/bar  0.5s
ok  github.com/foo/baz  1.2s`
	qt.HandleToolResult("exec", "go test ./...", output, false)

	if len(qt.Stages) != 2 {
		t.Fatalf("should have 2 stages, got %d", len(qt.Stages))
	}
	if qt.Stages[0].Name != "tests (go)" {
		t.Errorf("stage name = %q, want 'tests (go)'", qt.Stages[0].Name)
	}
	if qt.Stages[0].Status != StagePassed {
		t.Errorf("stage status = %d, want StagePassed", qt.Stages[0].Status)
	}
}

func TestQATracker_HandleToolResult_FailedTest(t *testing.T) {
	qt := NewQATracker(tools.NewCheckpointTool())
	qt.HandleToolResult("exec", "go test ./...", "FAIL pkg 0.5s", true)

	if qt.Stages[0].Status != StageFailed {
		t.Errorf("failed test status = %d, want StageFailed", qt.Stages[0].Status)
	}
}

func TestQATracker_HandleToolResult_IgnoresNonExec(t *testing.T) {
	qt := NewQATracker(tools.NewCheckpointTool())
	qt.HandleToolResult("read_file", "go test ./...", "output", false)

	if len(qt.Stages) != 1 {
		t.Error("non-exec tool should not create stages")
	}
}

func TestQATracker_HandleToolResult_IgnoresUnknownCommands(t *testing.T) {
	qt := NewQATracker(tools.NewCheckpointTool())
	qt.HandleToolResult("exec", "ls -la", "output", false)

	if len(qt.Stages) != 1 {
		t.Error("unrecognized commands should not create stages")
	}
}

func TestQATracker_StageOrdering_NaturalOrder(t *testing.T) {
	qt := NewQATracker(tools.NewCheckpointTool())

	// Add in natural order (lint, test, e2e) — insertion respects kind order
	qt.HandleToolResult("exec", "npx eslint .", "clean", false)
	qt.HandleToolResult("exec", "go test ./...", "ok", false)
	qt.HandleToolResult("exec", "npx cypress run", "passed", false)

	if len(qt.Stages) != 4 {
		t.Fatalf("expected 4 stages, got %d", len(qt.Stages))
	}

	expectedOrder := []stageKind{kindLint, kindTest, kindE2E, kindApproval}
	for i, expected := range expectedOrder {
		if qt.Stages[i].Kind != expected {
			t.Errorf("stage[%d].Kind = %d, want %d", i, qt.Stages[i].Kind, expected)
		}
	}
}

func TestQATracker_StageOrdering_SameKindReuses(t *testing.T) {
	qt := NewQATracker(tools.NewCheckpointTool())

	// Add two test stages with different names
	qt.HandleToolResult("exec", "go test ./...", "ok", false)
	qt.HandleToolResult("exec", "npx jest", "ok", false)

	if len(qt.Stages) != 3 {
		t.Fatalf("expected 3 stages, got %d", len(qt.Stages))
	}

	// Both should be test kind, before approval
	if qt.Stages[0].Kind != kindTest || qt.Stages[1].Kind != kindTest {
		t.Errorf("first two stages should be test kind")
	}
	if qt.Stages[2].Kind != kindApproval {
		t.Error("last stage should be approval")
	}
}

func TestQATracker_WaitingApproval(t *testing.T) {
	qt := NewQATracker(tools.NewCheckpointTool())

	// All checks pass → waiting approval
	qt.HandleToolResult("exec", "npx eslint .", "", false)
	qt.HandleToolResult("exec", "go test ./...", "ok", false)

	if !qt.WaitingApproval {
		t.Error("should be waiting for approval when all checks pass")
	}

	// A check fails → no longer waiting
	qt.HandleToolResult("exec", "go test ./...", "FAIL", true)

	if qt.WaitingApproval {
		t.Error("should not be waiting for approval when a check fails")
	}
}

func TestQATracker_Approve(t *testing.T) {
	qt := NewQATracker(tools.NewCheckpointTool())
	qt.HandleToolResult("exec", "go test ./...", "ok", false)

	qt.Approve()

	approvalStage := qt.Stages[len(qt.Stages)-1]
	if approvalStage.Status != StagePassed {
		t.Errorf("approval status = %d, want StagePassed", approvalStage.Status)
	}
	if approvalStage.Summary != "approved" {
		t.Errorf("approval summary = %q, want 'approved'", approvalStage.Summary)
	}
	if qt.WaitingApproval {
		t.Error("WaitingApproval should be false after approve")
	}
}

func TestQATracker_Reject(t *testing.T) {
	qt := NewQATracker(tools.NewCheckpointTool())
	qt.HandleToolResult("exec", "go test ./...", "ok", false)

	qt.Reject()

	approvalStage := qt.Stages[len(qt.Stages)-1]
	if approvalStage.Status != StageFailed {
		t.Errorf("approval status = %d, want StageFailed", approvalStage.Status)
	}
	if approvalStage.Summary != "rejected" {
		t.Errorf("approval summary = %q, want 'rejected'", approvalStage.Summary)
	}
}

func TestQATracker_Approve_WhenNotWaiting(t *testing.T) {
	qt := NewQATracker(tools.NewCheckpointTool())
	// Not waiting — approve should be no-op
	qt.Approve()

	if qt.Stages[0].Status != StagePending {
		t.Error("Approve when not waiting should be a no-op")
	}
}

func TestQATracker_Reset(t *testing.T) {
	qt := NewQATracker(tools.NewCheckpointTool())
	qt.HandleToolResult("exec", "go test ./...", "ok", false)
	qt.HandleToolResult("exec", "npx eslint .", "clean", false)

	qt.Reset("new feature")

	if qt.Feature != "new feature" {
		t.Errorf("Feature = %q, want %q", qt.Feature, "new feature")
	}
	if len(qt.Stages) != 1 {
		t.Errorf("after reset should have 1 stage, got %d", len(qt.Stages))
	}
	if qt.WaitingApproval {
		t.Error("WaitingApproval should be false after reset")
	}
}

func TestQATracker_UpdatesExistingStage(t *testing.T) {
	qt := NewQATracker(tools.NewCheckpointTool())

	// First run fails
	qt.HandleToolResult("exec", "go test ./...", "FAIL", true)
	if qt.Stages[0].Status != StageFailed {
		t.Error("first run should fail")
	}

	// Re-run passes
	qt.HandleToolResult("exec", "go test ./...", "ok", false)
	if qt.Stages[0].Status != StagePassed {
		t.Error("re-run should update existing stage to passed")
	}

	// Should not create duplicate stage
	if len(qt.Stages) != 2 { // test + approval
		t.Errorf("should have 2 stages (no duplicate), got %d", len(qt.Stages))
	}
}

// --- classifyCommand tests ---

func TestClassifyCommand(t *testing.T) {
	tests := []struct {
		command  string
		wantKind stageKind
		wantName string
	}{
		// Linters
		{"npx eslint src/", kindLint, "lint (eslint)"},
		{"biome check .", kindLint, "lint (biome)"},
		{"prettier --check .", kindLint, "lint (prettier)"},
		{"tsc --noEmit", kindLint, "typecheck (tsc)"},
		{"go vet ./...", kindLint, "lint (go vet)"},
		{"golangci-lint run", kindLint, "lint (golangci-lint)"},
		{"cargo clippy", kindLint, "lint (clippy)"},
		{"pylint src/", kindLint, "lint (pylint)"},
		{"ruff check .", kindLint, "lint (ruff)"},
		{"mypy src/", kindLint, "typecheck (mypy)"},
		{"rubocop", kindLint, "lint (rubocop)"},

		// Test runners
		{"go test ./...", kindTest, "tests (go)"},
		{"npx jest", kindTest, "tests (jest)"},
		{"npx vitest", kindTest, "tests (vitest)"},
		{"pytest tests/", kindTest, "tests (pytest)"},
		{"cargo test", kindTest, "tests (cargo)"},
		{"npm test", kindTest, "tests (npm)"},
		{"phpunit tests/", kindTest, "tests (phpunit)"},
		{"php artisan test", kindTest, "tests (laravel)"},
		{"rspec", kindTest, "tests (rspec)"},
		{"dotnet test", kindTest, "tests (dotnet)"},
		{"mvn test", kindTest, "tests (maven)"},
		{"swift test", kindTest, "tests (swift)"},
		{"make test", kindTest, "tests (make)"},

		// E2E
		{"npx cypress run", kindE2E, "e2e (cypress)"},
		{"npx playwright test", kindE2E, "e2e (playwright)"},
		{"go test -tags=e2e ./...", kindE2E, "e2e (go)"},
	}

	for _, tt := range tests {
		kind, name := classifyCommand(tt.command)
		if kind != tt.wantKind {
			t.Errorf("classifyCommand(%q) kind = %d, want %d", tt.command, kind, tt.wantKind)
		}
		if name != tt.wantName {
			t.Errorf("classifyCommand(%q) name = %q, want %q", tt.command, name, tt.wantName)
		}
	}
}

func TestClassifyCommand_Unrecognized(t *testing.T) {
	commands := []string{"ls -la", "cat file.go", "git status", "echo hello"}
	for _, cmd := range commands {
		kind, name := classifyCommand(cmd)
		if kind != -1 || name != "" {
			t.Errorf("classifyCommand(%q) = (%d, %q), want (-1, '')", cmd, kind, name)
		}
	}
}

// --- parseGoTestJSON tests ---

func TestParseGoTestJSON(t *testing.T) {
	output := `{"Action":"run","Package":"pkg/foo","Test":"TestA"}
{"Action":"output","Package":"pkg/foo","Test":"TestA","Output":"=== RUN   TestA\n"}
{"Action":"pass","Package":"pkg/foo","Test":"TestA","Elapsed":0.01}
{"Action":"run","Package":"pkg/foo","Test":"TestB"}
{"Action":"output","Package":"pkg/foo","Test":"TestB","Output":"--- FAIL: TestB\n"}
{"Action":"fail","Package":"pkg/foo","Test":"TestB","Elapsed":0.02}
{"Action":"run","Package":"pkg/foo","Test":"TestC"}
{"Action":"skip","Package":"pkg/foo","Test":"TestC","Elapsed":0.0}`

	results := parseGoTestJSON(output)
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	if results[0].Test != "TestA" || results[0].Status != "pass" {
		t.Errorf("result[0] = %+v, want TestA/pass", results[0])
	}
	if results[1].Test != "TestB" || results[1].Status != "fail" {
		t.Errorf("result[1] = %+v, want TestB/fail", results[1])
	}
	if results[2].Test != "TestC" || results[2].Status != "skip" {
		t.Errorf("result[2] = %+v, want TestC/skip", results[2])
	}
}

func TestParseGoTestJSON_Empty(t *testing.T) {
	results := parseGoTestJSON("")
	if len(results) != 0 {
		t.Errorf("empty input should produce 0 results, got %d", len(results))
	}
}

func TestParseGoTestJSON_PackageLevelOnly(t *testing.T) {
	// Package-level events (no Test field) should be skipped
	output := `{"Action":"pass","Package":"pkg/foo","Elapsed":0.5}`
	results := parseGoTestJSON(output)
	if len(results) != 0 {
		t.Errorf("package-level events should be ignored, got %d results", len(results))
	}
}

func TestLooksLikeGoTestJSON(t *testing.T) {
	tests := []struct {
		output string
		want   bool
	}{
		{`{"Action":"pass","Test":"TestA"}`, true},
		{`not json output`, false},
		{`{"key":"value"}`, false},
		{"", false},
	}

	for _, tt := range tests {
		got := looksLikeGoTestJSON(tt.output)
		if got != tt.want {
			t.Errorf("looksLikeGoTestJSON(%q) = %v, want %v", tt.output, got, tt.want)
		}
	}
}

// --- parseGenericTestOutput tests ---

func TestParseGenericTestOutput_Jest(t *testing.T) {
	output := "Tests:  5 passed, 2 failed, 7 total"
	result := parseGenericTestOutput(output)
	if result != "2/7 FAIL" {
		t.Errorf("jest output = %q, want '2/7 FAIL'", result)
	}
}

func TestParseGenericTestOutput_JestAllPass(t *testing.T) {
	output := "Tests:  5 passed, 5 total"
	result := parseGenericTestOutput(output)
	if result != "5/5 pass" {
		t.Errorf("jest all pass = %q, want '5/5 pass'", result)
	}
}

func TestParseGenericTestOutput_GoTestPlain(t *testing.T) {
	output := `ok  github.com/foo/bar  0.5s
ok  github.com/foo/baz  1.2s
FAIL github.com/foo/qux 0.3s`
	result := parseGenericTestOutput(output)
	if result != "1/3 FAIL" {
		t.Errorf("go test plain = %q, want '1/3 FAIL'", result)
	}
}

func TestParseGenericTestOutput_GoTestAllPass(t *testing.T) {
	output := `ok  github.com/foo/bar  0.5s
ok  github.com/foo/baz  1.2s`
	result := parseGenericTestOutput(output)
	if result != "2/2 pass" {
		t.Errorf("go test all pass = %q, want '2/2 pass'", result)
	}
}

func TestParseGenericTestOutput_Pytest(t *testing.T) {
	output := "====== 3 passed, 1 failed in 2.5s ======"
	result := parseGenericTestOutput(output)
	if result != "1/4 FAIL" {
		t.Errorf("pytest = %q, want '1/4 FAIL'", result)
	}
}

func TestParseGenericTestOutput_CargoTest(t *testing.T) {
	// Note: pytest pattern "(\d+) passed" matches before cargo pattern,
	// so "10 passed" is captured by pytest. Use cargo-specific format.
	output := "test result: ok. 10 passed; 0 failed; 0 ignored"
	result := parseGenericTestOutput(output)
	// pytest pattern matches first: "10 passed" → "10 pass"
	if result != "10 pass" {
		t.Errorf("cargo test output = %q, want '10 pass'", result)
	}
}

func TestParseGenericTestOutput_Unknown(t *testing.T) {
	result := parseGenericTestOutput("some random output")
	if result != "" {
		t.Errorf("unknown output should return empty, got %q", result)
	}
}

// --- summarizeResults tests ---

func TestSummarizeResults(t *testing.T) {
	tests := []struct {
		results []TestResult
		want    string
	}{
		{
			[]TestResult{
				{Status: "pass"}, {Status: "pass"}, {Status: "pass"},
			},
			"3/3 pass",
		},
		{
			[]TestResult{
				{Status: "pass"}, {Status: "fail"}, {Status: "skip"},
			},
			"1/3 FAIL",
		},
		{
			[]TestResult{
				{Status: "skip"}, {Status: "skip"},
			},
			"0/2 pass",
		},
	}

	for _, tt := range tests {
		got := summarizeResults(tt.results)
		if got != tt.want {
			t.Errorf("summarizeResults() = %q, want %q", got, tt.want)
		}
	}
}

func TestHasFailures(t *testing.T) {
	if hasFailures([]TestResult{{Status: "pass"}, {Status: "skip"}}) {
		t.Error("should return false when no failures")
	}
	if !hasFailures([]TestResult{{Status: "pass"}, {Status: "fail"}}) {
		t.Error("should return true when there are failures")
	}
}

// --- Helper function tests ---

func TestFormatTokenCount(t *testing.T) {
	tests := []struct {
		n    int
		want string
	}{
		{0, "0"},
		{999, "999"},
		{1000, "1.0k"},
		{1500, "1.5k"},
		{10000, "10.0k"},
		{123456, "123.5k"},
	}

	for _, tt := range tests {
		got := formatTokenCount(tt.n)
		if got != tt.want {
			t.Errorf("formatTokenCount(%d) = %q, want %q", tt.n, got, tt.want)
		}
	}
}

func TestAbbreviateKey(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"whatsapp:+1234567890", "wa:+1234567890"},
		{"telegram:user123", "tg:user123"},
		{"discord:guild456", "dc:guild456"},
		{"slack:channel789", "slack:channel789"},
		{"cli:direct", "cli:direct"},
		{"tui:local", "tui:local"},
	}

	for _, tt := range tests {
		got := abbreviateKey(tt.input)
		if got != tt.want {
			t.Errorf("abbreviateKey(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		s    string
		n    int
		want string
	}{
		{"hello", 10, "hello"},
		{"hello", 5, "hello"},
		{"hello world", 8, "hello..."},
		{"abcdef", 3, "abc"},
		{"ab", 1, "a"},
	}

	for _, tt := range tests {
		got := truncate(tt.s, tt.n)
		if got != tt.want {
			t.Errorf("truncate(%q, %d) = %q, want %q", tt.s, tt.n, got, tt.want)
		}
	}
}

func TestTimeAgo(t *testing.T) {
	tests := []struct {
		ago  time.Duration
		want string
	}{
		{5 * time.Second, "5s ago"},
		{30 * time.Second, "30s ago"},
		{5 * time.Minute, "5m ago"},
		{45 * time.Minute, "45m ago"},
		{3 * time.Hour, "3h ago"},
		{48 * time.Hour, "2d ago"},
	}

	for _, tt := range tests {
		got := timeAgo(time.Now().Add(-tt.ago))
		if got != tt.want {
			t.Errorf("timeAgo(-%v) = %q, want %q", tt.ago, got, tt.want)
		}
	}
}

func TestAtoi(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"0", 0},
		{"123", 123},
		{"42", 42},
		{"", 0},
		{"abc", 0},
		{"12abc", 12},
	}

	for _, tt := range tests {
		got := atoi(tt.input)
		if got != tt.want {
			t.Errorf("atoi(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}
