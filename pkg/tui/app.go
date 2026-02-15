package tui

import (
	"context"
	"log"

	"github.com/rivo/tview"
	"github.com/sipeed/picoclaw/pkg/bus"
	"github.com/sipeed/picoclaw/pkg/channels"
	"github.com/sipeed/picoclaw/pkg/config"
)

type App struct {
	tviewApp *tview.Application
	version  string

	// Panels
	chatHistory   *tview.TextView
	chatInput     *tview.InputField
	logsView      *tview.TextView
	channelsTable *tview.Table
	tokensTable   *tview.Table
	sessionsTable *tview.Table
	configView    *tview.TextView
	statusBar     *tview.TextView

	// Focus management
	panels     []tview.Primitive
	focusIndex int

	// Core dependencies
	config         *config.Config
	msgBus         *bus.MessageBus
	channelManager *channels.Manager
	tuiChannel     *TUIChannel
	logWriter      *LogWriter
	tokenTracker   *TokenTracker

	ctx    context.Context
	cancel context.CancelFunc
}

func NewApp(cfg *config.Config, msgBus *bus.MessageBus, version string) *App {
	ctx, cancel := context.WithCancel(context.Background())

	app := &App{
		tviewApp:     tview.NewApplication(),
		version:      version,
		config:       cfg,
		msgBus:       msgBus,
		tokenTracker: NewTokenTracker(),
		ctx:          ctx,
		cancel:       cancel,
	}

	return app
}

func (a *App) Init() {
	// Build all panels (order matters: chat builds chatInput used by layout)
	a.logsView = newPanel("LOGS [F2]")

	// Create TUI channel immediately so chat works as soon as tview starts
	a.tuiChannel = NewTUIChannel(a.msgBus, a)

	// Build layout (this calls build* methods for each panel)
	layout := a.buildLayout()

	// Setup keybindings (needs panels to be created)
	a.setupKeybindings()

	// Redirect standard log output to TUI logs panel
	a.logWriter = NewLogWriter(a, a.logsView)
	log.SetOutput(a.logWriter)
	log.SetFlags(0)

	a.tviewApp.SetRoot(layout, true)
	a.tviewApp.SetFocus(a.chatInput)
}

func (a *App) SetChannelManager(cm *channels.Manager) {
	a.channelManager = cm
}

func (a *App) GetTUIChannel() *TUIChannel {
	return a.tuiChannel
}

func (a *App) GetTokenTracker() *TokenTracker {
	return a.tokenTracker
}

func (a *App) StartBackgroundTasks() {
	a.startChannelsRefresh(a.ctx)
	a.startSessionsRefresh(a.ctx)
	a.startTokensRefresh(a.ctx)
}

func (a *App) Run() error {
	return a.tviewApp.Run()
}

func (a *App) Stop() {
	a.cancel()
	a.tviewApp.Stop()
}
