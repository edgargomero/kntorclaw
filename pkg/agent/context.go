package agent

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/sipeed/picoclaw/pkg/logger"
	"github.com/sipeed/picoclaw/pkg/providers"
	"github.com/sipeed/picoclaw/pkg/skills"
	"github.com/sipeed/picoclaw/pkg/tools"
)

type ContextBuilder struct {
	workspace    string
	skillsLoader *skills.SkillsLoader
	memory       *MemoryStore
	tools        *tools.ToolRegistry // Direct reference to tool registry
}

func getGlobalConfigDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".picoclaw")
}

func NewContextBuilder(workspace string) *ContextBuilder {
	// builtin skills: skills directory in current project
	// Use the skills/ directory under the current working directory
	wd, _ := os.Getwd()
	builtinSkillsDir := filepath.Join(wd, "skills")
	globalSkillsDir := filepath.Join(getGlobalConfigDir(), "skills")

	return &ContextBuilder{
		workspace:    workspace,
		skillsLoader: skills.NewSkillsLoader(workspace, globalSkillsDir, builtinSkillsDir),
		memory:       NewMemoryStore(workspace),
	}
}

// SetToolsRegistry sets the tools registry for dynamic tool summary generation.
func (cb *ContextBuilder) SetToolsRegistry(registry *tools.ToolRegistry) {
	cb.tools = registry
}

func (cb *ContextBuilder) getIdentity() string {
	now := time.Now().Format("2006-01-02 15:04 (Monday)")
	workspacePath, _ := filepath.Abs(filepath.Join(cb.workspace))
	runtime := fmt.Sprintf("%s %s, Go %s", runtime.GOOS, runtime.GOARCH, runtime.Version())

	// Build tools section dynamically
	toolsSection := cb.buildToolsSection()

	return fmt.Sprintf(`# picoclaw 🦞

You are picoclaw, a helpful AI assistant.

## Current Time
%s

## Runtime
%s

## Workspace
Your workspace is at: %s
- Memory: %s/memory/MEMORY.md
- Daily Notes: %s/memory/YYYYMM/YYYYMMDD.md
- Skills: %s/skills/{skill-name}/SKILL.md

%s

## Important Rules

1. **ALWAYS use tools** - When you need to perform an action (schedule reminders, send messages, execute commands, etc.), you MUST call the appropriate tool. Do NOT just say you'll do it or pretend to do it.

2. **Be helpful and accurate** - When using tools, briefly explain what you're doing.

3. **Memory** - When remembering something, write to %s/memory/MEMORY.md`,
		now, runtime, workspacePath, workspacePath, workspacePath, workspacePath, toolsSection, workspacePath)
}

func (cb *ContextBuilder) buildToolsSection() string {
	if cb.tools == nil {
		return ""
	}

	summaries := cb.tools.GetSummaries()
	if len(summaries) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("## Available Tools\n\n")
	sb.WriteString("**CRITICAL**: You MUST use tools to perform actions. Do NOT pretend to execute commands or schedule tasks.\n\n")
	sb.WriteString("You have access to the following tools:\n\n")
	for _, s := range summaries {
		sb.WriteString(s)
		sb.WriteString("\n")
	}

	return sb.String()
}

func (cb *ContextBuilder) BuildSystemPrompt() string {
	parts := []string{}

	// Core identity section
	parts = append(parts, cb.getIdentity())

	// Bootstrap files
	bootstrapContent := cb.LoadBootstrapFiles()
	if bootstrapContent != "" {
		parts = append(parts, bootstrapContent)
	}

	// Skills - show summary, AI can read full content with read_file tool
	skillsSummary := cb.skillsLoader.BuildSkillsSummary()
	if skillsSummary != "" {
		parts = append(parts, fmt.Sprintf(`# Skills

The following skills extend your capabilities. To use a skill, read its SKILL.md file using the read_file tool.

%s`, skillsSummary))
	}

	// Memory context
	memoryContext := cb.memory.GetMemoryContext()
	if memoryContext != "" {
		parts = append(parts, "# Memory\n\n"+memoryContext)
	}

	// Git context (if workspace is a git repo)
	gitCtx := buildGitContext(cb.workspace)
	if gitCtx != "" {
		parts = append(parts, "# Git Status\n\n"+gitCtx)
	}

	// Join with "---" separator
	return strings.Join(parts, "\n\n---\n\n")
}

func (cb *ContextBuilder) LoadBootstrapFiles() string {
	bootstrapFiles := []string{
		"AGENTS.md",
		"SOUL.md",
		"USER.md",
		"IDENTITY.md",
	}

	var result string
	for _, filename := range bootstrapFiles {
		filePath := filepath.Join(cb.workspace, filename)
		if data, err := os.ReadFile(filePath); err == nil {
			result += fmt.Sprintf("## %s\n\n%s\n\n", filename, string(data))
		}
	}

	// Load CLAUDE.md from workspace root (project instructions)
	claudePath := filepath.Join(cb.workspace, "CLAUDE.md")
	if data, err := os.ReadFile(claudePath); err == nil {
		result += fmt.Sprintf("## Project Instructions (CLAUDE.md)\n\n%s\n\n", string(data))
	}

	return result
}

func (cb *ContextBuilder) BuildMessages(history []providers.Message, summary string, currentMessage string, media []string, channel, chatID string) []providers.Message {
	messages := []providers.Message{}

	systemPrompt := cb.BuildSystemPrompt()

	// Add Current Session info if provided
	if channel != "" && chatID != "" {
		systemPrompt += fmt.Sprintf("\n\n## Current Session\nChannel: %s\nChat ID: %s", channel, chatID)
	}

	// Log system prompt summary for debugging (debug mode only)
	logger.DebugCF("agent", "System prompt built",
		map[string]interface{}{
			"total_chars":   len(systemPrompt),
			"total_lines":   strings.Count(systemPrompt, "\n") + 1,
			"section_count": strings.Count(systemPrompt, "\n\n---\n\n") + 1,
		})

	// Log preview of system prompt (avoid logging huge content)
	preview := systemPrompt
	if len(preview) > 500 {
		preview = preview[:500] + "... (truncated)"
	}
	logger.DebugCF("agent", "System prompt preview",
		map[string]interface{}{
			"preview": preview,
		})

	if summary != "" {
		systemPrompt += "\n\n## Summary of Previous Conversation\n\n" + summary
	}

	// Remove orphaned tool messages from start of history
	for len(history) > 0 && (history[0].Role == "tool") {
		logger.DebugCF("agent", "Removing orphaned tool message from history start",
			map[string]interface{}{"role": history[0].Role})
		history = history[1:]
	}

	// Remove tool_result messages whose tool_use_id has no matching tool_use in
	// any prior assistant message. This happens when switching providers mid-session
	// (e.g. OpenAI tool IDs like "chatcmpl-tool-..." are invalid for Claude).
	history = sanitizeOrphanedToolResults(history)

	messages = append(messages, providers.Message{
		Role:    "system",
		Content: systemPrompt,
	})

	messages = append(messages, history...)

	messages = append(messages, providers.Message{
		Role:    "user",
		Content: currentMessage,
	})

	return messages
}

func (cb *ContextBuilder) AddToolResult(messages []providers.Message, toolCallID, toolName, result string) []providers.Message {
	messages = append(messages, providers.Message{
		Role:       "tool",
		Content:    result,
		ToolCallID: toolCallID,
	})
	return messages
}

func (cb *ContextBuilder) AddAssistantMessage(messages []providers.Message, content string, toolCalls []map[string]interface{}) []providers.Message {
	msg := providers.Message{
		Role:    "assistant",
		Content: content,
	}
	// Always add assistant message, whether or not it has tool calls
	messages = append(messages, msg)
	return messages
}

func (cb *ContextBuilder) loadSkills() string {
	allSkills := cb.skillsLoader.ListSkills()
	if len(allSkills) == 0 {
		return ""
	}

	var skillNames []string
	for _, s := range allSkills {
		skillNames = append(skillNames, s.Name)
	}

	content := cb.skillsLoader.LoadSkillsForContext(skillNames)
	if content == "" {
		return ""
	}

	return "# Skill Definitions\n\n" + content
}

// buildGitContext generates a markdown summary of the git state for the workspace.
// Returns empty string if the workspace is not a git repo.
func buildGitContext(workspace string) string {
	// Check if workspace is a git repo
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	cmd.Dir = workspace
	if _, err := cmd.Output(); err != nil {
		return ""
	}

	var parts []string

	// Current branch
	branchCmd := exec.Command("git", "branch", "--show-current")
	branchCmd.Dir = workspace
	if out, err := branchCmd.Output(); err == nil {
		branch := strings.TrimSpace(string(out))
		if branch != "" {
			parts = append(parts, fmt.Sprintf("**Branch:** `%s`", branch))
		}
	}

	// Modified/staged files
	statusCmd := exec.Command("git", "status", "--porcelain")
	statusCmd.Dir = workspace
	if out, err := statusCmd.Output(); err == nil {
		status := strings.TrimSpace(string(out))
		if status != "" {
			parts = append(parts, "**Changed files:**\n```\n"+status+"\n```")
		}
	}

	// Recent commits
	logCmd := exec.Command("git", "log", "--oneline", "-5")
	logCmd.Dir = workspace
	if out, err := logCmd.Output(); err == nil {
		logOutput := strings.TrimSpace(string(out))
		if logOutput != "" {
			parts = append(parts, "**Recent commits:**\n```\n"+logOutput+"\n```")
		}
	}

	if len(parts) == 0 {
		return ""
	}

	return strings.Join(parts, "\n\n")
}

// GetSkillsInfo returns information about loaded skills.
func (cb *ContextBuilder) GetSkillsInfo() map[string]interface{} {
	allSkills := cb.skillsLoader.ListSkills()
	skillNames := make([]string, 0, len(allSkills))
	for _, s := range allSkills {
		skillNames = append(skillNames, s.Name)
	}
	return map[string]interface{}{
		"total":     len(allSkills),
		"available": len(allSkills),
		"names":     skillNames,
	}
}

// sanitizeOrphanedToolResults removes tool_result messages whose tool_use_id
// doesn't match any tool_use in the history's assistant messages. Also removes
// assistant tool_use messages whose results are missing. This prevents errors
// when switching providers mid-session (e.g. OpenAI → Claude).
func sanitizeOrphanedToolResults(history []providers.Message) []providers.Message {
	// Collect all tool_use IDs from assistant messages
	toolUseIDs := make(map[string]bool)
	for _, msg := range history {
		if msg.Role == "assistant" {
			for _, tc := range msg.ToolCalls {
				if tc.ID != "" {
					toolUseIDs[tc.ID] = true
				}
			}
		}
	}

	// Collect all tool_result IDs
	toolResultIDs := make(map[string]bool)
	for _, msg := range history {
		if (msg.Role == "tool" || msg.Role == "user") && msg.ToolCallID != "" {
			toolResultIDs[msg.ToolCallID] = true
		}
	}

	// Filter: keep messages that aren't orphaned
	var cleaned []providers.Message
	dropped := 0
	for _, msg := range history {
		// Drop tool_result without matching tool_use
		if (msg.Role == "tool" || (msg.Role == "user" && msg.ToolCallID != "")) && msg.ToolCallID != "" {
			if !toolUseIDs[msg.ToolCallID] {
				dropped++
				logger.DebugCF("agent", "Dropping orphaned tool_result",
					map[string]interface{}{"tool_call_id": msg.ToolCallID})
				continue
			}
		}

		// Drop assistant tool_use messages if ALL their tool results are missing
		if msg.Role == "assistant" && len(msg.ToolCalls) > 0 {
			allMissing := true
			for _, tc := range msg.ToolCalls {
				if toolResultIDs[tc.ID] {
					allMissing = false
					break
				}
			}
			if allMissing {
				// Keep the text content but strip the tool calls
				if msg.Content != "" {
					cleaned = append(cleaned, providers.Message{
						Role:    "assistant",
						Content: msg.Content,
					})
				}
				dropped++
				logger.DebugCF("agent", "Stripping orphaned tool_use from assistant message",
					map[string]interface{}{"tool_count": len(msg.ToolCalls)})
				continue
			}
		}

		cleaned = append(cleaned, msg)
	}

	if dropped > 0 {
		logger.DebugCF("agent", "Sanitized history: dropped orphaned tool messages",
			map[string]interface{}{"dropped": dropped, "remaining": len(cleaned)})
	}

	return cleaned
}
