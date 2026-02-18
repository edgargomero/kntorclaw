package tui

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/sipeed/picoclaw/pkg/bus"
	"github.com/sipeed/picoclaw/pkg/channels"
	"github.com/sipeed/picoclaw/pkg/tui/organisms"
)

// BubbleteaTUIChannel implements channels.Channel for bubbletea.
// It sends messages to the TUI via tea.Program.Send and handles
// inbound messages through BaseChannel.HandleMessage.
type BubbleteaTUIChannel struct {
	*channels.BaseChannel
	program *tea.Program
}

// NewBubbleteaTUIChannel creates a new bubbletea-compatible TUI channel.
func NewBubbleteaTUIChannel(msgBus *bus.MessageBus) *BubbleteaTUIChannel {
	return &BubbleteaTUIChannel{
		BaseChannel: channels.NewBaseChannel("tui", nil, msgBus, nil),
	}
}

// SetProgram assigns the tea.Program used to deliver messages.
func (c *BubbleteaTUIChannel) SetProgram(p *tea.Program) {
	c.program = p
}

// Start implements channels.Channel.
func (c *BubbleteaTUIChannel) Start(ctx context.Context) error {
	c.SetRunning(true)
	return nil
}

// Stop implements channels.Channel.
func (c *BubbleteaTUIChannel) Stop(ctx context.Context) error {
	c.SetRunning(false)
	return nil
}

// Send implements channels.Channel — pushes agent responses to the chat panel.
func (c *BubbleteaTUIChannel) Send(ctx context.Context, msg bus.OutboundMessage) error {
	if c.program != nil {
		c.program.Send(organisms.ChatResponseMsg{
			Sender:  "Agent",
			Content: msg.Content,
		})
	}
	return nil
}

// IsAllowed implements channels.Channel — TUI always allows all senders.
func (c *BubbleteaTUIChannel) IsAllowed(senderID string) bool {
	return true
}

// SendUserMessage sends a user message through the bus for agent processing.
func (c *BubbleteaTUIChannel) SendUserMessage(content string) {
	c.HandleMessage("local", "local", content, nil, nil)
}

// SendLog pushes a log line to the logs panel.
func (c *BubbleteaTUIChannel) SendLog(line string) {
	if c.program != nil {
		c.program.Send(organisms.LogLineMsg{Line: line})
	}
}

// SendHealthChange notifies the TUI about a channel status change.
func (c *BubbleteaTUIChannel) SendHealthChange(channel string, online bool) {
	if c.program != nil {
		c.program.Send(organisms.HealthChangedMsg{
			Channel: channel,
			Online:  online,
		})
	}
}
