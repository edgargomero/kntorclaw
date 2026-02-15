package tools

import "context"

// CheckpointTool blocks the agent until human approval is received.
// The TUI sends approval/rejection via the internal channel.
type CheckpointTool struct {
	approvalCh chan bool
}

// NewCheckpointTool creates a new CheckpointTool.
func NewCheckpointTool() *CheckpointTool {
	return &CheckpointTool{
		approvalCh: make(chan bool, 1),
	}
}

func (t *CheckpointTool) Name() string {
	return "checkpoint"
}

func (t *CheckpointTool) Description() string {
	return "Request human approval before proceeding. Blocks until the user approves or rejects in the QA panel."
}

func (t *CheckpointTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"message": map[string]interface{}{
				"type":        "string",
				"description": "Message to display to the user explaining what needs approval",
			},
		},
		"required": []string{"message"},
	}
}

func (t *CheckpointTool) Execute(ctx context.Context, args map[string]interface{}) *ToolResult {
	message, _ := args["message"].(string)
	if message == "" {
		message = "Waiting for human approval..."
	}

	// Block until approval or context cancellation
	select {
	case approved := <-t.approvalCh:
		if approved {
			return NewToolResult("Human approved: " + message)
		}
		return ErrorResult("Human rejected: " + message)
	case <-ctx.Done():
		return ErrorResult("Checkpoint cancelled: context expired")
	}
}

// Approve sends approval to unblock the agent.
func (t *CheckpointTool) Approve() {
	select {
	case t.approvalCh <- true:
	default:
	}
}

// Reject sends rejection to unblock the agent.
func (t *CheckpointTool) Reject() {
	select {
	case t.approvalCh <- false:
	default:
	}
}
