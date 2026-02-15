package tui

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"

	"github.com/rivo/tview"
	"github.com/sipeed/picoclaw/pkg/tools"
)

// StageStatus represents the status of a QA pipeline stage.
type StageStatus int

const (
	StagePending StageStatus = iota
	StageRunning
	StagePassed
	StageFailed
	StageBlocked
)

// stageKind classifies what kind of QA step a command represents.
type stageKind int

const (
	kindLint stageKind = iota
	kindTest
	kindE2E
	kindApproval
)

// CheckpointStage represents a single stage in the QA pipeline.
type CheckpointStage struct {
	Name    string       // display name, e.g. "lint (eslint)", "tests (jest)"
	Command string       // the actual command that was executed
	Kind    stageKind    // lint, test, e2e, approval
	Status  StageStatus  // pending/running/passed/failed
	Summary string       // "4/4 pass", "clean"
	Results []TestResult // individual results when parseable
}

// TestResult represents a single test result.
type TestResult struct {
	Package string  // suite or file
	Test    string  // test name
	Status  string  // "pass", "fail", "skip"
	Elapsed float64 // seconds
	Output  string  // error message if failed
}

// goTestEvent represents a single line from go test -json output.
type goTestEvent struct {
	Action  string  `json:"Action"`
	Package string  `json:"Package"`
	Test    string  `json:"Test"`
	Elapsed float64 `json:"Elapsed"`
	Output  string  `json:"Output"`
}

// commandPattern maps a regex to a stage kind and display name.
type commandPattern struct {
	re   *regexp.Regexp
	kind stageKind
	name string // display name template, e.g. "lint (eslint)"
}

// knownPatterns are ordered from most specific to least specific.
// The first match wins.
var knownPatterns = []commandPattern{
	// -- E2E / integration (check before generic test) --
	{re: regexp.MustCompile(`\bcypress\b`), kind: kindE2E, name: "e2e (cypress)"},
	{re: regexp.MustCompile(`\bplaywright\b`), kind: kindE2E, name: "e2e (playwright)"},
	{re: regexp.MustCompile(`\bselenium\b`), kind: kindE2E, name: "e2e (selenium)"},
	{re: regexp.MustCompile(`go\s+test\b.*-tags=e2e`), kind: kindE2E, name: "e2e (go)"},
	{re: regexp.MustCompile(`go\s+test\b.*-tags=integration`), kind: kindTest, name: "integration (go)"},

	// -- Linters --
	{re: regexp.MustCompile(`\beslint\b`), kind: kindLint, name: "lint (eslint)"},
	{re: regexp.MustCompile(`\bbiome\s+(check|lint)\b`), kind: kindLint, name: "lint (biome)"},
	{re: regexp.MustCompile(`\bprettier\b.*--check`), kind: kindLint, name: "lint (prettier)"},
	{re: regexp.MustCompile(`\btsc\b.*--noEmit`), kind: kindLint, name: "typecheck (tsc)"},
	{re: regexp.MustCompile(`\bphpstan\b`), kind: kindLint, name: "lint (phpstan)"},
	{re: regexp.MustCompile(`\bphp-cs-fixer\b.*--dry-run`), kind: kindLint, name: "lint (php-cs-fixer)"},
	{re: regexp.MustCompile(`\bpint\b.*--test`), kind: kindLint, name: "lint (pint)"},
	{re: regexp.MustCompile(`\bpylint\b`), kind: kindLint, name: "lint (pylint)"},
	{re: regexp.MustCompile(`\bruff\s+check\b`), kind: kindLint, name: "lint (ruff)"},
	{re: regexp.MustCompile(`\bflake8\b`), kind: kindLint, name: "lint (flake8)"},
	{re: regexp.MustCompile(`\bmypy\b`), kind: kindLint, name: "typecheck (mypy)"},
	{re: regexp.MustCompile(`\brubocop\b`), kind: kindLint, name: "lint (rubocop)"},
	{re: regexp.MustCompile(`\bgolangci-lint\b`), kind: kindLint, name: "lint (golangci-lint)"},
	{re: regexp.MustCompile(`\bgo\s+vet\b`), kind: kindLint, name: "lint (go vet)"},
	{re: regexp.MustCompile(`\bcargo\s+clippy\b`), kind: kindLint, name: "lint (clippy)"},
	{re: regexp.MustCompile(`\bswiftlint\b`), kind: kindLint, name: "lint (swiftlint)"},
	{re: regexp.MustCompile(`\bktlint\b`), kind: kindLint, name: "lint (ktlint)"},

	// -- Test runners --
	{re: regexp.MustCompile(`\b(npx\s+)?jest\b`), kind: kindTest, name: "tests (jest)"},
	{re: regexp.MustCompile(`\b(npx\s+)?vitest\b`), kind: kindTest, name: "tests (vitest)"},
	{re: regexp.MustCompile(`\b(npx\s+)?mocha\b`), kind: kindTest, name: "tests (mocha)"},
	{re: regexp.MustCompile(`\bnpm\s+test\b`), kind: kindTest, name: "tests (npm)"},
	{re: regexp.MustCompile(`\byarn\s+test\b`), kind: kindTest, name: "tests (yarn)"},
	{re: regexp.MustCompile(`\bpnpm\s+test\b`), kind: kindTest, name: "tests (pnpm)"},
	{re: regexp.MustCompile(`\bbun\s+test\b`), kind: kindTest, name: "tests (bun)"},
	{re: regexp.MustCompile(`\bphpunit\b`), kind: kindTest, name: "tests (phpunit)"},
	{re: regexp.MustCompile(`\b(php\s+)?artisan\s+test\b`), kind: kindTest, name: "tests (laravel)"},
	{re: regexp.MustCompile(`\bpest\b`), kind: kindTest, name: "tests (pest)"},
	{re: regexp.MustCompile(`\bpytest\b`), kind: kindTest, name: "tests (pytest)"},
	{re: regexp.MustCompile(`\bpython\s+-m\s+unittest\b`), kind: kindTest, name: "tests (unittest)"},
	{re: regexp.MustCompile(`\bgo\s+test\b`), kind: kindTest, name: "tests (go)"},
	{re: regexp.MustCompile(`\bcargo\s+test\b`), kind: kindTest, name: "tests (cargo)"},
	{re: regexp.MustCompile(`\brspec\b`), kind: kindTest, name: "tests (rspec)"},
	{re: regexp.MustCompile(`\bbundle\s+exec\s+rake\s+test\b`), kind: kindTest, name: "tests (rake)"},
	{re: regexp.MustCompile(`\brails\s+test\b`), kind: kindTest, name: "tests (rails)"},
	{re: regexp.MustCompile(`\bdotnet\s+test\b`), kind: kindTest, name: "tests (dotnet)"},
	{re: regexp.MustCompile(`\bmvn\s+test\b`), kind: kindTest, name: "tests (maven)"},
	{re: regexp.MustCompile(`\bgradle\s+test\b`), kind: kindTest, name: "tests (gradle)"},
	{re: regexp.MustCompile(`\bswift\s+test\b`), kind: kindTest, name: "tests (swift)"},
	{re: regexp.MustCompile(`\bmake\s+test\b`), kind: kindTest, name: "tests (make)"},
}

// classifyCommand returns the kind and display name for a command, or -1 if unrecognized.
func classifyCommand(command string) (stageKind, string) {
	for _, p := range knownPatterns {
		if p.re.MatchString(command) {
			return p.kind, p.name
		}
	}
	return -1, ""
}

// QATracker tracks the QA pipeline state for the focus mode panel.
// Stages are created dynamically as the agent executes recognized commands.
type QATracker struct {
	mu              sync.RWMutex
	Feature         string
	Stages          []*CheckpointStage
	WaitingApproval bool
	checkpointTool  *tools.CheckpointTool
}

// NewQATracker creates a new QATracker with only the approval stage.
// Lint/test/e2e stages are added dynamically as the agent runs commands.
func NewQATracker(ct *tools.CheckpointTool) *QATracker {
	return &QATracker{
		checkpointTool: ct,
		Stages: []*CheckpointStage{
			{Name: "human approval", Kind: kindApproval, Status: StagePending},
		},
	}
}

// HandleToolResult processes a tool result event and updates the QA pipeline.
// It auto-creates stages for recognized lint/test/e2e commands.
func (qt *QATracker) HandleToolResult(toolName, command, output string, isError bool) {
	if toolName != "exec" || command == "" {
		return
	}

	kind, name := classifyCommand(command)
	if kind == -1 {
		return // not a lint/test/e2e command
	}

	qt.mu.Lock()
	defer qt.mu.Unlock()

	// Find or create stage for this command
	stage := qt.findOrCreateStage(kind, name, command)

	// Determine status from exit code
	if isError {
		stage.Status = StageFailed
	} else {
		stage.Status = StagePassed
	}

	// Try to parse detailed results
	stage.Results = nil
	stage.Summary = ""

	if kind == kindTest || kind == kindE2E {
		// Try go test -json first
		if strings.Contains(command, "-json") || looksLikeGoTestJSON(output) {
			results := parseGoTestJSON(output)
			if len(results) > 0 {
				stage.Results = results
				stage.Summary = summarizeResults(results)
				if hasFailures(results) {
					stage.Status = StageFailed
				}
			}
		}

		// Try generic test output parsing if no structured results yet
		if len(stage.Results) == 0 {
			if summary := parseGenericTestOutput(output); summary != "" {
				stage.Summary = summary
			}
		}
	}

	// Fallback summaries
	if stage.Summary == "" {
		if isError {
			stage.Summary = "FAIL"
		} else {
			stage.Summary = "pass"
		}
	}

	if kind == kindLint && !isError {
		stage.Summary = "clean"
	}

	qt.updateApprovalState()
}

// findOrCreateStage finds an existing stage by kind+name or creates a new one.
// New stages are inserted before the approval stage in kind order (lint < test < e2e).
func (qt *QATracker) findOrCreateStage(kind stageKind, name, command string) *CheckpointStage {
	// Look for existing stage with same name
	for _, s := range qt.Stages {
		if s.Name == name {
			s.Command = command // update to latest command
			return s
		}
	}

	// Create new stage
	stage := &CheckpointStage{
		Name:    name,
		Command: command,
		Kind:    kind,
		Status:  StagePending,
	}

	// Insert before approval stage, respecting kind order
	insertIdx := 0
	for i, s := range qt.Stages {
		if s.Kind == kindApproval {
			insertIdx = i
			break
		}
		if s.Kind <= kind {
			insertIdx = i + 1
		}
	}

	// Insert at position
	qt.Stages = append(qt.Stages, nil)
	copy(qt.Stages[insertIdx+1:], qt.Stages[insertIdx:])
	qt.Stages[insertIdx] = stage

	return stage
}

// updateApprovalState checks whether the approval stage should become active.
func (qt *QATracker) updateApprovalState() {
	// Need at least one non-approval stage
	hasChecks := false
	allPassed := true
	for _, stage := range qt.Stages {
		if stage.Kind == kindApproval {
			continue
		}
		hasChecks = true
		if stage.Status != StagePassed {
			allPassed = false
			break
		}
	}

	if !hasChecks {
		return
	}

	for _, stage := range qt.Stages {
		if stage.Kind == kindApproval {
			if allPassed {
				qt.WaitingApproval = true
				stage.Summary = "WAITING"
			} else {
				qt.WaitingApproval = false
				stage.Summary = ""
			}
			break
		}
	}
}

// Approve triggers the checkpoint tool approval.
func (qt *QATracker) Approve() {
	qt.mu.Lock()
	defer qt.mu.Unlock()

	if !qt.WaitingApproval {
		return
	}

	qt.WaitingApproval = false
	for _, stage := range qt.Stages {
		if stage.Kind == kindApproval {
			stage.Status = StagePassed
			stage.Summary = "approved"
			break
		}
	}

	if qt.checkpointTool != nil {
		qt.checkpointTool.Approve()
	}
}

// Reject triggers the checkpoint tool rejection.
func (qt *QATracker) Reject() {
	qt.mu.Lock()
	defer qt.mu.Unlock()

	if !qt.WaitingApproval {
		return
	}

	qt.WaitingApproval = false
	for _, stage := range qt.Stages {
		if stage.Kind == kindApproval {
			stage.Status = StageFailed
			stage.Summary = "rejected"
			break
		}
	}

	if qt.checkpointTool != nil {
		qt.checkpointTool.Reject()
	}
}

// Reset resets the tracker for a new feature.
func (qt *QATracker) Reset(feature string) {
	qt.mu.Lock()
	defer qt.mu.Unlock()

	qt.Feature = feature
	qt.Stages = []*CheckpointStage{
		{Name: "human approval", Kind: kindApproval, Status: StagePending},
	}
	qt.WaitingApproval = false
}

// --- Output parsers ---

// looksLikeGoTestJSON checks if output contains go test -json formatted lines.
func looksLikeGoTestJSON(output string) bool {
	for _, line := range strings.SplitN(output, "\n", 5) {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "{") && strings.Contains(line, `"Action"`) {
			return true
		}
	}
	return false
}

// parseGoTestJSON parses go test -json output and returns test results.
func parseGoTestJSON(output string) []TestResult {
	var results []TestResult
	outputMap := make(map[string]string)

	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var event goTestEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			continue
		}

		if event.Test == "" {
			continue
		}

		key := event.Package + "/" + event.Test

		switch event.Action {
		case "output":
			outputMap[key] += event.Output
		case "pass", "fail", "skip":
			results = append(results, TestResult{
				Package: event.Package,
				Test:    event.Test,
				Status:  event.Action,
				Elapsed: event.Elapsed,
				Output:  outputMap[key],
			})
		}
	}

	return results
}

// Common patterns for test output summaries across frameworks.
var genericTestPatterns = []*regexp.Regexp{
	// Jest/Vitest: "Tests:  2 passed, 1 failed, 3 total"
	regexp.MustCompile(`Tests:\s+(?:(\d+)\s+passed)?[,\s]*(?:(\d+)\s+failed)?[,\s]*(\d+)\s+total`),
	// PHPUnit/Pest: "Tests: 5, Assertions: 10, Failures: 1"
	regexp.MustCompile(`Tests:\s*(\d+).*?Failures:\s*(\d+)`),
	// PHPUnit OK: "OK (5 tests, 10 assertions)"
	regexp.MustCompile(`OK\s*\((\d+)\s+tests?`),
	// pytest: "5 passed, 2 failed" or "5 passed"
	regexp.MustCompile(`(\d+)\s+passed(?:.*?(\d+)\s+failed)?`),
	// RSpec: "5 examples, 1 failure"
	regexp.MustCompile(`(\d+)\s+examples?,\s*(\d+)\s+failures?`),
	// Go test: "ok  pkg  0.5s" or "FAIL pkg 0.5s" (plain output)
	regexp.MustCompile(`(?m)^(ok|FAIL)\s+\S+\s+[\d.]+s`),
	// cargo test: "test result: ok. 5 passed; 0 failed"
	regexp.MustCompile(`test result:.*?(\d+)\s+passed;\s*(\d+)\s+failed`),
	// dotnet: "Passed!  - Failed: 0, Passed: 5"
	regexp.MustCompile(`Failed:\s*(\d+),\s*Passed:\s*(\d+)`),
}

// parseGenericTestOutput tries to extract a summary from generic test runner output.
func parseGenericTestOutput(output string) string {
	// Jest/Vitest pattern
	if m := genericTestPatterns[0].FindStringSubmatch(output); m != nil {
		passed, failed, total := m[1], m[2], m[3]
		if failed != "" && failed != "0" {
			return fmt.Sprintf("%s/%s FAIL", failed, total)
		}
		if passed != "" {
			return fmt.Sprintf("%s/%s pass", passed, total)
		}
	}

	// PHPUnit with failures
	if m := genericTestPatterns[1].FindStringSubmatch(output); m != nil {
		tests, failures := m[1], m[2]
		if failures != "0" {
			return fmt.Sprintf("%s/%s FAIL", failures, tests)
		}
		return fmt.Sprintf("%s/%s pass", tests, tests)
	}

	// PHPUnit OK
	if m := genericTestPatterns[2].FindStringSubmatch(output); m != nil {
		return fmt.Sprintf("%s/%s pass", m[1], m[1])
	}

	// pytest
	if m := genericTestPatterns[3].FindStringSubmatch(output); m != nil {
		passed := m[1]
		failed := m[2]
		if failed != "" && failed != "0" {
			total := atoi(passed) + atoi(failed)
			return fmt.Sprintf("%s/%d FAIL", failed, total)
		}
		return fmt.Sprintf("%s pass", passed)
	}

	// RSpec
	if m := genericTestPatterns[4].FindStringSubmatch(output); m != nil {
		examples, failures := m[1], m[2]
		if failures != "0" {
			return fmt.Sprintf("%s/%s FAIL", failures, examples)
		}
		return fmt.Sprintf("%s/%s pass", examples, examples)
	}

	// Go test plain
	if m := genericTestPatterns[5].FindAllStringSubmatch(output, -1); len(m) > 0 {
		passed, failed := 0, 0
		for _, match := range m {
			if match[1] == "ok" {
				passed++
			} else {
				failed++
			}
		}
		total := passed + failed
		if failed > 0 {
			return fmt.Sprintf("%d/%d FAIL", failed, total)
		}
		return fmt.Sprintf("%d/%d pass", passed, total)
	}

	// cargo test
	if m := genericTestPatterns[6].FindStringSubmatch(output); m != nil {
		passed, failed := m[1], m[2]
		total := atoi(passed) + atoi(failed)
		if failed != "0" {
			return fmt.Sprintf("%s/%d FAIL", failed, total)
		}
		return fmt.Sprintf("%s/%d pass", passed, total)
	}

	// dotnet
	if m := genericTestPatterns[7].FindStringSubmatch(output); m != nil {
		failed, passed := m[1], m[2]
		total := atoi(passed) + atoi(failed)
		if failed != "0" {
			return fmt.Sprintf("%s/%d FAIL", failed, total)
		}
		return fmt.Sprintf("%s/%d pass", passed, total)
	}

	return ""
}

func summarizeResults(results []TestResult) string {
	passed, failed, skipped := 0, 0, 0
	for _, r := range results {
		switch r.Status {
		case "pass":
			passed++
		case "fail":
			failed++
		case "skip":
			skipped++
		}
	}
	total := passed + failed + skipped
	if failed > 0 {
		return fmt.Sprintf("%d/%d FAIL", failed, total)
	}
	return fmt.Sprintf("%d/%d pass", passed, total)
}

func hasFailures(results []TestResult) bool {
	for _, r := range results {
		if r.Status == "fail" {
			return true
		}
	}
	return false
}

func atoi(s string) int {
	n := 0
	for _, c := range s {
		if c >= '0' && c <= '9' {
			n = n*10 + int(c-'0')
		}
	}
	return n
}

// --- Panel rendering ---

// buildFocusQAPanel creates the QA panel widget for focus mode.
func (a *App) buildFocusQAPanel() *tview.TextView {
	tv := tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true).
		SetWordWrap(true)
	tv.SetBorder(true).SetTitle(" QA [F6] ")
	return tv
}

// refreshQAPanel updates the QA panel with the current tracker state.
func (a *App) refreshQAPanel() {
	if a.focusQA == nil || a.qaTracker == nil {
		return
	}

	a.qaTracker.mu.RLock()
	defer a.qaTracker.mu.RUnlock()

	a.focusQA.Clear()

	var b strings.Builder

	// Feature name
	if a.qaTracker.Feature != "" {
		b.WriteString(fmt.Sprintf("[white::b]Feature: %s[-::-]\n\n", tview.Escape(a.qaTracker.Feature)))
	}

	// Empty state
	if len(a.qaTracker.Stages) <= 1 {
		b.WriteString("[gray]No checks yet.\nThe pipeline builds as the agent\nruns lint and test commands.[-]\n")
		fmt.Fprintf(a.focusQA, "%s", b.String())
		return
	}

	// Pipeline stages
	passedCount := 0
	totalCheckable := 0

	for i, stage := range a.qaTracker.Stages {
		if stage.Kind != kindApproval {
			totalCheckable++
		}

		var icon, color string
		switch stage.Status {
		case StagePassed:
			icon = "[green]\u2705[-]"
			color = "green"
			if stage.Kind != kindApproval {
				passedCount++
			}
		case StageFailed:
			icon = "[red]\u274c[-]"
			color = "red"
		case StageRunning:
			icon = "[yellow]\u23f3[-]"
			color = "yellow"
		case StageBlocked:
			icon = "[gray][ ][-]"
			color = "gray"
		default:
			icon = "[gray][ ][-]"
			color = "gray"
		}

		connector := "  \u251c\u2500\u2500 "
		if i == len(a.qaTracker.Stages)-1 {
			connector = "  \u2514\u2500\u2500 "
		}

		summary := stage.Summary
		if summary == "" {
			switch stage.Status {
			case StageRunning:
				summary = "running"
			case StageBlocked:
				summary = "blocked"
			default:
				summary = "pending"
			}
		}

		b.WriteString(fmt.Sprintf("%s%s [%s]%-20s %s[-]\n",
			connector, icon, color, tview.Escape(stage.Name), tview.Escape(summary)))

		// Show failed test details
		if stage.Status == StageFailed && len(stage.Results) > 0 {
			for _, r := range stage.Results {
				if r.Status == "fail" {
					b.WriteString(fmt.Sprintf("  [gray]\u2502[-]   [red]%s  FAIL %.2fs[-]\n",
						tview.Escape(r.Test), r.Elapsed))
				}
			}
		}
	}

	// Progress bar
	b.WriteString("\n")
	if totalCheckable > 0 {
		barWidth := 12
		filled := barWidth * passedCount / totalCheckable
		bar := strings.Repeat("\u2588", filled) + strings.Repeat("\u2591", barWidth-filled)
		b.WriteString(fmt.Sprintf(" [yellow][%s][-] %d/%d\n", bar, passedCount, totalCheckable))
	}

	// Status message
	b.WriteString("\n")
	if a.qaTracker.WaitingApproval {
		b.WriteString("[green::b]\u2705 All checks pass![-::-]\n")
		b.WriteString("[white][Enter][-] approve  [white][Esc][-] reject\n")
	} else {
		failCount := 0
		for _, stage := range a.qaTracker.Stages {
			if stage.Status == StageFailed {
				failCount++
			}
		}
		if failCount > 0 {
			b.WriteString(fmt.Sprintf("[red]\u274c %d stage(s) failed[-]\n", failCount))
		}
	}

	fmt.Fprintf(a.focusQA, "%s", b.String())
}
