package tools

import "context"

// CheckpointNotifyFunc is called when the agent requests human approval.
// It receives the checkpoint message so external channels (e.g. Telegram)
// can notify the operator and wait for their response.
type CheckpointNotifyFunc func(message string)

// CheckpointTool blocks the agent until human approval is received.
// The TUI sends approval/rejection via the internal channel.
// An optional NotifyFunc can be set to forward the request to an external channel.
type CheckpointTool struct {
	approvalCh chan bool
	// NotifyFunc, if set, is called when Execute blocks waiting for approval.
	// Use SetNotifyFunc to register external approval channels (e.g. Telegram).
	NotifyFunc CheckpointNotifyFunc
}

// NewCheckpointTool creates a new CheckpointTool.
func NewCheckpointTool() *CheckpointTool {
	return &CheckpointTool{
		approvalCh: make(chan bool, 1),
	}
}

// SetNotifyFunc registers a callback that is invoked when the agent calls
// checkpoint. The callback should notify the operator (e.g. send a Telegram
// message) so they can approve or reject via /approve or /reject.
func (t *CheckpointTool) SetNotifyFunc(fn CheckpointNotifyFunc) {
	t.NotifyFunc = fn
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

	// Notify external channel (e.g. Telegram) if configured.
	if t.NotifyFunc != nil {
		t.NotifyFunc(message)
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
