package tui

import (
	"context"

	"github.com/sipeed/picoclaw/pkg/bus"
	"github.com/sipeed/picoclaw/pkg/channels"
)

type TUIChannel struct {
	*channels.BaseChannel
	app *App
}

func NewTUIChannel(msgBus *bus.MessageBus, app *App) *TUIChannel {
	return &TUIChannel{
		BaseChannel: channels.NewBaseChannel("tui", nil, msgBus, nil),
		app:         app,
	}
}

func (c *TUIChannel) Start(ctx context.Context) error {
	c.SetRunning(true)
	return nil
}

func (c *TUIChannel) Stop(ctx context.Context) error {
	c.SetRunning(false)
	return nil
}

func (c *TUIChannel) Send(ctx context.Context, msg bus.OutboundMessage) error {
	c.app.appendChat("Agent", msg.Content)
	return nil
}

func (c *TUIChannel) IsAllowed(senderID string) bool {
	return true
}

func (c *TUIChannel) sendMessage(content string) {
	c.HandleMessage("local", "local", content, nil, nil)
}
