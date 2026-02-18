// PicoClaw - Ultra-lightweight personal AI agent
// Inspired by and based on nanobot: https://github.com/HKUDS/nanobot
// License: MIT
//
// Copyright (c) 2026 PicoClaw contributors

package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/sipeed/picoclaw/pkg/logger"
	"github.com/sipeed/picoclaw/pkg/providers"
	"github.com/sipeed/picoclaw/pkg/utils"
)

// ToolLoopConfig configures the tool execution loop.
type ToolLoopConfig struct {
	Provider      providers.LLMProvider
	Model         string
	Tools         *ToolRegistry
	MaxIterations int
	LLMOptions    map[string]any
	// PlanningPrompt, if non-empty, executes a planning step before the main
	// tool loop. The plan is injected into the system message so the agent
	// knows which steps to follow.
	PlanningPrompt string
	// PlanningModel is the model used for the planning step. Defaults to
	// Model if empty.
	PlanningModel string
}

// ToolLoopResult contains the result of running the tool loop.
type ToolLoopResult struct {
	Content      string
	Iterations   int
	InputTokens  int
	OutputTokens int
	ToolsUsed    []string
}

// RunToolLoop executes the LLM + tool call iteration loop.
// This is the core agent logic that can be reused by both main agent and subagents.
func RunToolLoop(ctx context.Context, config ToolLoopConfig, messages []providers.Message, channel, chatID string) (*ToolLoopResult, error) {
	iteration := 0
	var finalContent string
	var inputTokens, outputTokens int
	var toolsUsed []string

	// Planning step: runs a single LLM call (no tools) to produce an execution
	// plan that is injected into the system message before the main loop.
	if config.PlanningPrompt != "" && len(messages) >= 2 {
		planModel := config.PlanningModel
		if planModel == "" {
			planModel = config.Model
		}
		planUserContent := messages[1].Content + "\n\nCreate a step-by-step execution plan."
		planResp, err := config.Provider.Chat(ctx,
			[]providers.Message{
				{Role: "system", Content: config.PlanningPrompt},
				{Role: "user", Content: planUserContent},
			},
			nil, planModel,
			map[string]any{"max_tokens": 1024, "temperature": 0.3},
		)
		if err == nil && planResp != nil && planResp.Content != "" {
			messages[0].Content += "\n\n## Your Execution Plan\n" + planResp.Content
			if planResp.Usage != nil {
				inputTokens += planResp.Usage.PromptTokens
				outputTokens += planResp.Usage.CompletionTokens
			}
			logger.DebugCF("toolloop", "Planning step completed",
				map[string]any{"plan_length": len(planResp.Content), "model": planModel})
		} else if err != nil {
			logger.WarnCF("toolloop", "Planning step failed (continuing without plan)",
				map[string]any{"error": err.Error()})
		}
	}

	for iteration < config.MaxIterations {
		iteration++

		logger.DebugCF("toolloop", "LLM iteration",
			map[string]any{
				"iteration": iteration,
				"max":       config.MaxIterations,
			})

		// 1. Build tool definitions
		var providerToolDefs []providers.ToolDefinition
		if config.Tools != nil {
			providerToolDefs = config.Tools.ToProviderDefs()
		}

		// 2. Set default LLM options
		llmOpts := config.LLMOptions
		if llmOpts == nil {
			llmOpts = map[string]any{
				"max_tokens":  4096,
				"temperature": 0.7,
			}
		}

		// 3. Call LLM
		response, err := config.Provider.Chat(ctx, messages, providerToolDefs, config.Model, llmOpts)
		if err != nil {
			logger.ErrorCF("toolloop", "LLM call failed",
				map[string]any{
					"iteration": iteration,
					"error":     err.Error(),
				})
			return nil, fmt.Errorf("LLM call failed: %w", err)
		}

		// 4. Track token usage
		if response.Usage != nil {
			inputTokens += response.Usage.PromptTokens
			outputTokens += response.Usage.CompletionTokens
		}

		// 5. If no tool calls, we're done
		if len(response.ToolCalls) == 0 {
			finalContent = response.Content
			logger.InfoCF("toolloop", "LLM response without tool calls (direct answer)",
				map[string]any{
					"iteration":     iteration,
					"content_chars": len(finalContent),
				})
			break
		}

		// 6. Log tool calls
		toolNames := make([]string, 0, len(response.ToolCalls))
		for _, tc := range response.ToolCalls {
			toolNames = append(toolNames, tc.Name)
		}
		logger.InfoCF("toolloop", "LLM requested tool calls",
			map[string]any{
				"tools":     toolNames,
				"count":     len(response.ToolCalls),
				"iteration": iteration,
			})

		// 7. Build assistant message with tool calls
		assistantMsg := providers.Message{
			Role:    "assistant",
			Content: response.Content,
		}
		for _, tc := range response.ToolCalls {
			argumentsJSON, _ := json.Marshal(tc.Arguments)
			assistantMsg.ToolCalls = append(assistantMsg.ToolCalls, providers.ToolCall{
				ID:   tc.ID,
				Type: "function",
				Name: tc.Name,
				Function: &providers.FunctionCall{
					Name:      tc.Name,
					Arguments: string(argumentsJSON),
				},
			})
		}
		messages = append(messages, assistantMsg)

		// 8. Execute tool calls
		for _, tc := range response.ToolCalls {
			toolsUsed = append(toolsUsed, tc.Name)
			argsJSON, _ := json.Marshal(tc.Arguments)
			argsPreview := utils.Truncate(string(argsJSON), 200)
			logger.InfoCF("toolloop", fmt.Sprintf("Tool call: %s(%s)", tc.Name, argsPreview),
				map[string]any{
					"tool":      tc.Name,
					"iteration": iteration,
				})

			// Execute tool (no async callback for subagents - they run independently)
			var toolResult *ToolResult
			if config.Tools != nil {
				toolResult = config.Tools.ExecuteWithContext(ctx, tc.Name, tc.Arguments, channel, chatID, nil)
			} else {
				toolResult = ErrorResult("No tools available")
			}

			// Determine content for LLM
			contentForLLM := toolResult.ForLLM
			if contentForLLM == "" && toolResult.Err != nil {
				contentForLLM = toolResult.Err.Error()
			}

			// Add tool result message
			toolResultMsg := providers.Message{
				Role:       "tool",
				Content:    contentForLLM,
				ToolCallID: tc.ID,
			}
			messages = append(messages, toolResultMsg)
		}
	}

	return &ToolLoopResult{
		Content:      finalContent,
		Iterations:   iteration,
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
		ToolsUsed:    toolsUsed,
	}, nil
}
