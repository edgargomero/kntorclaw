package channels

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	_ "modernc.org/sqlite"

	"github.com/mdp/qrterminal/v3"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"

	"github.com/sipeed/picoclaw/pkg/bus"
	"github.com/sipeed/picoclaw/pkg/config"
	"github.com/sipeed/picoclaw/pkg/logger"
	"github.com/sipeed/picoclaw/pkg/utils"
	"github.com/sipeed/picoclaw/pkg/voice"
)

type WhatsAppChannel struct {
	*BaseChannel
	client      *whatsmeow.Client
	container   *sqlstore.Container
	config      config.WhatsAppConfig
	transcriber *voice.GroqTranscriber
	mu          sync.Mutex
}

func NewWhatsAppChannel(cfg config.WhatsAppConfig, bus *bus.MessageBus) (*WhatsAppChannel, error) {
	base := NewBaseChannel("whatsapp", cfg, bus, cfg.AllowFrom)

	return &WhatsAppChannel{
		BaseChannel: base,
		config:      cfg,
	}, nil
}

func (c *WhatsAppChannel) SetTranscriber(transcriber *voice.GroqTranscriber) {
	c.transcriber = transcriber
}

func (c *WhatsAppChannel) Start(ctx context.Context) error {
	sessionPath := expandSessionPath(c.config.SessionPath)

	dir := filepath.Dir(sessionPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create session directory: %w", err)
	}

	waLogger := logger.NewWaLogger("whatsapp")

	db, err := sql.Open("sqlite", fmt.Sprintf("file:%s?_pragma=foreign_keys(1)", sessionPath))
	if err != nil {
		return fmt.Errorf("failed to open whatsapp session database: %w", err)
	}

	container := sqlstore.NewWithDB(db, "sqlite3", waLogger)
	if err := container.Upgrade(ctx); err != nil {
		db.Close()
		return fmt.Errorf("failed to upgrade whatsapp session store: %w", err)
	}
	c.container = container

	device, err := container.GetFirstDevice(ctx)
	if err != nil {
		return fmt.Errorf("failed to get whatsapp device: %w", err)
	}

	client := whatsmeow.NewClient(device, waLogger)
	client.AddEventHandler(c.handleEvent)
	c.client = client

	if client.Store.ID == nil {
		logger.InfoC("whatsapp", "No session found. Please scan QR code to login:")

		qrChan, err := client.GetQRChannel(ctx)
		if err != nil {
			return fmt.Errorf("failed to get QR channel: %w", err)
		}

		if err := client.Connect(); err != nil {
			return fmt.Errorf("failed to connect for QR login: %w", err)
		}

		go c.handleQRChannel(qrChan)
	} else {
		if err := client.Connect(); err != nil {
			return fmt.Errorf("failed to connect whatsapp: %w", err)
		}
		logger.InfoC("whatsapp", "Client connected (resuming existing session)")
	}

	c.SetRunning(true)
	return nil
}

func (c *WhatsAppChannel) handleQRChannel(qrChan <-chan whatsmeow.QRChannelItem) {
	for evt := range qrChan {
		switch evt.Event {
		case whatsmeow.QRChannelEventCode:
			fmt.Println("Scan the QR code below with WhatsApp:")
			qrterminal.GenerateHalfBlock(evt.Code, qrterminal.L, os.Stdout)
		case "success":
			logger.InfoC("whatsapp", "QR login successful")
		case "timeout":
			logger.WarnC("whatsapp", "QR code timed out. Restart to try again.")
		default:
			if evt.Error != nil {
				logger.ErrorCF("whatsapp", "QR pairing error", map[string]interface{}{
					"event": evt.Event,
					"error": evt.Error.Error(),
				})
			} else {
				logger.WarnCF("whatsapp", "QR channel event", map[string]interface{}{
					"event": evt.Event,
				})
			}
		}
	}
}

func (c *WhatsAppChannel) Stop(ctx context.Context) error {
	logger.InfoC("whatsapp", "Stopping WhatsApp channel...")

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.client != nil {
		c.client.Disconnect()
		c.client = nil
	}

	c.SetRunning(false)
	return nil
}

func (c *WhatsAppChannel) Send(ctx context.Context, msg bus.OutboundMessage) error {
	c.mu.Lock()
	client := c.client
	c.mu.Unlock()

	if client == nil {
		return fmt.Errorf("whatsapp client not connected")
	}

	jid, err := types.ParseJID(msg.ChatID)
	if err != nil {
		return fmt.Errorf("invalid WhatsApp JID %q: %w", msg.ChatID, err)
	}

	// Stop typing indicator
	_ = client.SendChatPresence(ctx, jid, types.ChatPresencePaused, types.ChatPresenceMediaText)

	message := &waE2E.Message{
		Conversation: proto.String(msg.Content),
	}

	_, err = client.SendMessage(ctx, jid, message)
	if err != nil {
		return fmt.Errorf("failed to send whatsapp message: %w", err)
	}

	return nil
}

func (c *WhatsAppChannel) handleEvent(evt interface{}) {
	switch v := evt.(type) {
	case *events.Message:
		c.handleMessage(v)
	case *events.Connected:
		logger.InfoC("whatsapp", "Client connected")
	case *events.Disconnected:
		logger.WarnC("whatsapp", "Client disconnected")
	case *events.LoggedOut:
		logger.WarnCF("whatsapp", "Client logged out", map[string]interface{}{
			"reason": v.Reason.String(),
		})
	}
}

func (c *WhatsAppChannel) handleMessage(msg *events.Message) {
	if msg.Info.IsFromMe {
		return
	}

	// Ignore status broadcast messages (WhatsApp Stories/Status updates)
	if msg.Info.Chat.Server == "broadcast" {
		return
	}

	senderID := msg.Info.Sender.String()
	chatID := msg.Info.Chat.String()

	// Early allowlist check before downloading media
	if !c.IsAllowed(senderID) {
		logger.DebugCF("whatsapp", "Message rejected by allowlist", map[string]interface{}{
			"sender": senderID,
		})
		return
	}

	content := ""
	var mediaPaths []string
	var localFiles []string

	defer func() {
		for _, file := range localFiles {
			os.Remove(file)
		}
	}()

	if msg.Message.GetConversation() != "" {
		content = msg.Message.GetConversation()
	} else if msg.Message.GetExtendedTextMessage() != nil {
		content = msg.Message.GetExtendedTextMessage().GetText()
	}

	if msg.Message.GetImageMessage() != nil {
		path := c.downloadMedia(msg.Message)
		if path != "" {
			localFiles = append(localFiles, path)
			mediaPaths = append(mediaPaths, path)
			if content != "" {
				content += "\n"
			}
			content += "[image]"
		}
		if caption := msg.Message.GetImageMessage().GetCaption(); caption != "" {
			if content != "" {
				content += "\n"
			}
			content += caption
		}
	}

	if msg.Message.GetVideoMessage() != nil {
		path := c.downloadMedia(msg.Message)
		if path != "" {
			localFiles = append(localFiles, path)
			mediaPaths = append(mediaPaths, path)
			if content != "" {
				content += "\n"
			}
			content += "[video]"
		}
		if caption := msg.Message.GetVideoMessage().GetCaption(); caption != "" {
			if content != "" {
				content += "\n"
			}
			content += caption
		}
	}

	if msg.Message.GetAudioMessage() != nil {
		path := c.downloadMedia(msg.Message)
		if path != "" {
			localFiles = append(localFiles, path)
			mediaPaths = append(mediaPaths, path)

			transcribedText := ""
			if c.transcriber != nil && c.transcriber.IsAvailable() {
				tctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()

				result, err := c.transcriber.Transcribe(tctx, path)
				if err != nil {
					logger.ErrorCF("whatsapp", "Voice transcription failed", map[string]interface{}{
						"error": err.Error(),
						"path":  path,
					})
					transcribedText = "[voice (transcription failed)]"
				} else {
					transcribedText = fmt.Sprintf("[voice transcription: %s]", result.Text)
					logger.InfoCF("whatsapp", "Voice transcribed successfully", map[string]interface{}{
						"text": result.Text,
					})
				}
			} else {
				transcribedText = "[voice]"
			}

			if content != "" {
				content += "\n"
			}
			content += transcribedText
		}
	}

	if msg.Message.GetDocumentMessage() != nil {
		path := c.downloadMedia(msg.Message)
		if path != "" {
			localFiles = append(localFiles, path)
			mediaPaths = append(mediaPaths, path)
			if content != "" {
				content += "\n"
			}
			content += "[document]"
		}
	}

	if content == "" {
		content = "[empty message]"
	}

	logger.DebugCF("whatsapp", "Received message", map[string]interface{}{
		"sender":  senderID,
		"chat":    chatID,
		"preview": utils.Truncate(content, 50),
	})

	// Send typing indicator
	c.mu.Lock()
	client := c.client
	c.mu.Unlock()
	if client != nil {
		_ = client.SendChatPresence(context.Background(), msg.Info.Chat, types.ChatPresenceComposing, types.ChatPresenceMediaText)
	}

	metadata := map[string]string{
		"message_id": msg.Info.ID,
		"sender_jid": senderID,
	}
	if msg.Info.PushName != "" {
		metadata["push_name"] = msg.Info.PushName
	}
	if msg.Info.IsGroup {
		metadata["is_group"] = "true"
	}

	c.HandleMessage(senderID, chatID, content, mediaPaths, metadata)
}

func (c *WhatsAppChannel) downloadMedia(msg *waE2E.Message) string {
	c.mu.Lock()
	client := c.client
	c.mu.Unlock()

	if client == nil {
		return ""
	}

	data, err := client.DownloadAny(context.Background(), msg)
	if err != nil {
		logger.ErrorCF("whatsapp", "Failed to download media", map[string]interface{}{
			"error": err.Error(),
		})
		return ""
	}

	ext := ".bin"
	switch {
	case msg.GetImageMessage() != nil:
		ext = ".jpg"
	case msg.GetVideoMessage() != nil:
		ext = ".mp4"
	case msg.GetAudioMessage() != nil:
		ext = ".ogg"
	case msg.GetDocumentMessage() != nil:
		if fname := msg.GetDocumentMessage().GetFileName(); fname != "" {
			ext = filepath.Ext(fname)
			if ext == "" {
				ext = ".bin"
			}
		}
	}

	tmpFile, err := os.CreateTemp("", "picoclaw-wa-*"+ext)
	if err != nil {
		logger.ErrorCF("whatsapp", "Failed to create temp file", map[string]interface{}{
			"error": err.Error(),
		})
		return ""
	}

	if _, err := tmpFile.Write(data); err != nil {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
		return ""
	}
	tmpFile.Close()

	return tmpFile.Name()
}

func expandSessionPath(path string) string {
	if path == "" {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, ".picoclaw", "whatsapp.db")
	}
	if len(path) > 0 && path[0] == '~' {
		home, _ := os.UserHomeDir()
		if len(path) > 1 && (path[1] == '/' || path[1] == '\\') {
			return filepath.Join(home, path[2:])
		}
		return home
	}
	return path
}
