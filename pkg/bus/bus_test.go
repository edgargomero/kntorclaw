package bus

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestNewMessageBus(t *testing.T) {
	mb := NewMessageBus()
	if mb == nil {
		t.Fatal("NewMessageBus() returned nil")
	}
	if mb.inbound == nil || mb.outbound == nil || mb.handlers == nil {
		t.Fatal("NewMessageBus() did not initialize channels/handlers")
	}
}

func TestPublishAndConsumeInbound(t *testing.T) {
	mb := NewMessageBus()
	ctx := context.Background()

	msg := InboundMessage{
		Channel:    "telegram",
		SenderID:   "user123",
		ChatID:     "chat456",
		Content:    "Hello, world!",
		SessionKey: "tg:user123",
	}

	go func() {
		mb.PublishInbound(msg)
	}()

	received, ok := mb.ConsumeInbound(ctx)
	if !ok {
		t.Fatal("ConsumeInbound returned ok=false")
	}
	if received.Channel != "telegram" {
		t.Errorf("Channel = %q, want %q", received.Channel, "telegram")
	}
	if received.SenderID != "user123" {
		t.Errorf("SenderID = %q, want %q", received.SenderID, "user123")
	}
	if received.Content != "Hello, world!" {
		t.Errorf("Content = %q, want %q", received.Content, "Hello, world!")
	}
	if received.SessionKey != "tg:user123" {
		t.Errorf("SessionKey = %q, want %q", received.SessionKey, "tg:user123")
	}
}

func TestConsumeInbound_CancelledContext(t *testing.T) {
	mb := NewMessageBus()
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	_, ok := mb.ConsumeInbound(ctx)
	if ok {
		t.Error("ConsumeInbound should return ok=false when context is cancelled")
	}
}

func TestPublishAndSubscribeOutbound(t *testing.T) {
	mb := NewMessageBus()
	ctx := context.Background()

	msg := OutboundMessage{
		Channel: "discord",
		ChatID:  "guild789",
		Content: "Reply from agent",
	}

	go func() {
		mb.PublishOutbound(msg)
	}()

	received, ok := mb.SubscribeOutbound(ctx)
	if !ok {
		t.Fatal("SubscribeOutbound returned ok=false")
	}
	if received.Channel != "discord" {
		t.Errorf("Channel = %q, want %q", received.Channel, "discord")
	}
	if received.Content != "Reply from agent" {
		t.Errorf("Content = %q, want %q", received.Content, "Reply from agent")
	}
}

func TestSubscribeOutbound_CancelledContext(t *testing.T) {
	mb := NewMessageBus()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, ok := mb.SubscribeOutbound(ctx)
	if ok {
		t.Error("SubscribeOutbound should return ok=false when context is cancelled")
	}
}

func TestRegisterAndGetHandler(t *testing.T) {
	mb := NewMessageBus()

	called := false
	handler := func(msg InboundMessage) error {
		called = true
		return nil
	}

	mb.RegisterHandler("telegram", handler)

	got, ok := mb.GetHandler("telegram")
	if !ok {
		t.Fatal("GetHandler returned ok=false for registered handler")
	}
	if got == nil {
		t.Fatal("GetHandler returned nil handler")
	}

	// Verify it's callable
	got(InboundMessage{})
	if !called {
		t.Error("handler was not called")
	}
}

func TestGetHandler_NotRegistered(t *testing.T) {
	mb := NewMessageBus()

	_, ok := mb.GetHandler("nonexistent")
	if ok {
		t.Error("GetHandler should return ok=false for unregistered channel")
	}
}

func TestRegisterHandler_Overwrite(t *testing.T) {
	mb := NewMessageBus()

	var callOrder []string
	mb.RegisterHandler("slack", func(msg InboundMessage) error {
		callOrder = append(callOrder, "first")
		return nil
	})
	mb.RegisterHandler("slack", func(msg InboundMessage) error {
		callOrder = append(callOrder, "second")
		return nil
	})

	handler, ok := mb.GetHandler("slack")
	if !ok {
		t.Fatal("GetHandler returned ok=false")
	}
	handler(InboundMessage{})

	if len(callOrder) != 1 || callOrder[0] != "second" {
		t.Errorf("expected only second handler to be called, got %v", callOrder)
	}
}

func TestMessageBus_ConcurrentAccess(t *testing.T) {
	mb := NewMessageBus()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	const numMessages = 50
	var wg sync.WaitGroup

	// Concurrent publishers
	wg.Add(numMessages)
	for i := 0; i < numMessages; i++ {
		go func(i int) {
			defer wg.Done()
			mb.PublishInbound(InboundMessage{
				Content: "msg",
				ChatID:  "chat",
			})
		}(i)
	}

	// Concurrent consumers
	received := make(chan InboundMessage, numMessages)
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < numMessages; i++ {
			msg, ok := mb.ConsumeInbound(ctx)
			if !ok {
				return
			}
			received <- msg
		}
	}()

	wg.Wait()
	close(received)

	count := 0
	for range received {
		count++
	}
	if count != numMessages {
		t.Errorf("received %d messages, want %d", count, numMessages)
	}
}

func TestMessageBus_ConcurrentHandlerRegistration(t *testing.T) {
	mb := NewMessageBus()
	var wg sync.WaitGroup

	channels := []string{"telegram", "discord", "slack", "whatsapp", "line"}
	wg.Add(len(channels))
	for _, ch := range channels {
		go func(ch string) {
			defer wg.Done()
			mb.RegisterHandler(ch, func(msg InboundMessage) error { return nil })
		}(ch)
	}
	wg.Wait()

	for _, ch := range channels {
		if _, ok := mb.GetHandler(ch); !ok {
			t.Errorf("handler for %q not found after concurrent registration", ch)
		}
	}
}

func TestInboundMessage_WithMedia(t *testing.T) {
	mb := NewMessageBus()
	ctx := context.Background()

	msg := InboundMessage{
		Channel: "whatsapp",
		ChatID:  "chat1",
		Content: "Check this out",
		Media:   []string{"/tmp/photo.jpg", "/tmp/audio.ogg"},
		Metadata: map[string]string{
			"mime_type": "image/jpeg",
		},
	}

	go mb.PublishInbound(msg)
	received, ok := mb.ConsumeInbound(ctx)
	if !ok {
		t.Fatal("ConsumeInbound returned ok=false")
	}
	if len(received.Media) != 2 {
		t.Errorf("len(Media) = %d, want 2", len(received.Media))
	}
	if received.Metadata["mime_type"] != "image/jpeg" {
		t.Errorf("Metadata[mime_type] = %q, want %q", received.Metadata["mime_type"], "image/jpeg")
	}
}

func TestMessageBus_FIFO_Order(t *testing.T) {
	mb := NewMessageBus()
	ctx := context.Background()

	// Publish 3 messages in order
	for i := 0; i < 3; i++ {
		mb.PublishInbound(InboundMessage{Content: string(rune('A' + i))})
	}

	// Consume and verify order
	for i := 0; i < 3; i++ {
		msg, ok := mb.ConsumeInbound(ctx)
		if !ok {
			t.Fatalf("ConsumeInbound %d returned ok=false", i)
		}
		expected := string(rune('A' + i))
		if msg.Content != expected {
			t.Errorf("message %d: Content = %q, want %q", i, msg.Content, expected)
		}
	}
}
