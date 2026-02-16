package tui

import (
	"context"
	"fmt"
	"log"
	syslog "log"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"

	"github.com/rivo/tview"
	"github.com/sipeed/picoclaw/pkg/agent"
	"github.com/sipeed/picoclaw/pkg/bus"
	"github.com/sipeed/picoclaw/pkg/channels"
	"github.com/sipeed/picoclaw/pkg/config"
	"github.com/sipeed/picoclaw/pkg/logger"
)

// debugLog writes to ~/.picoclaw/tui_debug.log when PICOCLAW_TUI_DEBUG=1.
// Use from any TUI code to trace key events, modal opens, focus changes, etc.
var debugLog func(format string, args ...interface{})

func init() {
	if os.Getenv("PICOCLAW_TUI_DEBUG") == "1" {
		home, _ := os.UserHomeDir()
		path := filepath.Join(home, ".picoclaw", "tui_debug.log")
		f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
		if err != nil {
			syslog.Printf("[tui-debug] Failed to open debug log: %v", err)
			debugLog = func(string, ...interface{}) {}
			return
		}
		debugLog = func(format string, args ...interface{}) {
			ts := time.Now().Format("15:04:05.000")
			fmt.Fprintf(f, ts+" "+format+"\n", args...)
			f.Sync()
		}
		debugLog("=== TUI debug log started (v3-lockfree) ===")
	} else {
		debugLog = func(string, ...interface{}) {}
	}
}

type App struct {
	tviewApp *tview.Application
	version  string

	// Panels
	chatHistory   *tview.TextView
	chatInput     *tview.TextArea
	logsView      *tview.TextView
	channelsTable *tview.Table
	tokensTable   *tview.Table
	sessionsTable *tview.Table
	configView    *tview.TextView
	statusBar     *tview.TextView

	// Focus management
	panels     []tview.Primitive
	focusIndex int

	// Focus mode (lazygit-style layout)
	focusMode     bool
	normalLayout  *tview.Flex
	focusLayout   *tview.Flex
	focusFiles    *tview.List
	focusBranches *tview.List
	focusCommits  *tview.List
	focusDiff     *tview.TextView
	focusQA       *tview.TextView
	qaTracker     *QATracker
	isProjectMode bool

	// Config mode (F8 interactive view)
	configMode     bool
	configLayout   *tview.Flex
	configSections *tview.List
	configItems    *tview.List
	configInfo     *tview.TextView

	// Model routing
	modelRouter *agent.ModelRouter
	configPath  string
	modalOpen   bool
	configBusy  atomic.Bool // suppresses background refreshes during config save

	// Error tracking
	errorTracker *ErrorTracker

	// Core dependencies
	config          *config.Config
	msgBus          *bus.MessageBus
	channelManager  *channels.Manager
	tuiChannel      *TUIChannel
	logWriter       *LogWriter
	activityTracker *ActivityTracker

	ctx    context.Context
	cancel context.CancelFunc
}

func NewApp(cfg *config.Config, msgBus *bus.MessageBus, version string) *App {
	ctx, cancel := context.WithCancel(context.Background())

	app := &App{
		tviewApp:        tview.NewApplication(),
		version:         version,
		config:          cfg,
		msgBus:          msgBus,
		activityTracker: NewActivityTracker(),
		ctx:             ctx,
		cancel:          cancel,
	}

	return app
}

func (a *App) SetProjectMode(enabled bool) {
	a.isProjectMode = enabled
}

func (a *App) Init() {
	// Build all panels (order matters: chat builds chatInput used by layout)
	a.logsView = newPanel("LOGS [F2]")
	a.buildConfigPanel() // config not in layout, used by renderConfig + config mode

	// Create TUI channel immediately so chat works as soon as tview starts
	a.tuiChannel = NewTUIChannel(a.msgBus, a)

	// Build layout (this calls build* methods for each panel)
	layout := a.buildLayout()
	a.normalLayout = layout

	// Build focus mode panels (lazy, not visible yet)
	a.focusFiles = a.buildFocusFilesPanel()
	a.focusBranches = a.buildFocusBranchesPanel()
	a.focusCommits = a.buildFocusCommitsPanel()
	a.focusDiff = a.buildFocusDiffPanel()
	a.focusQA = a.buildFocusQAPanel()

	// Build config mode panels (F8 interactive view)
	a.buildConfigViewPanels()

	// Load historical token usage
	if a.configPath != "" {
		a.activityTracker.LoadTotals(filepath.Join(filepath.Dir(a.configPath), "token_usage.json"))
	}

	// Initialize error tracker with backlog in config dir
	if a.configPath != "" {
		errLogPath := filepath.Join(filepath.Dir(a.configPath), "errors.log")
		if et, err := NewErrorTracker(errLogPath); err == nil {
			a.errorTracker = et
			logger.SetOnError(func(entry logger.LogEntry) {
				et.Add(entry)
				a.tviewApp.QueueUpdate(func() {
					a.updateStatusBar()
				})
			})
		}
	}

	// Setup keybindings (needs panels to be created)
	a.setupKeybindings()

	// Redirect standard log output to TUI logs panel
	a.logWriter = NewLogWriter(a, a.logsView)
	log.SetOutput(a.logWriter)
	log.SetFlags(0)

	a.tviewApp.SetRoot(layout, true)
	a.tviewApp.SetFocus(a.chatInput)
}

func (a *App) SetConfigPath(path string) {
	a.configPath = path
}

func (a *App) SetModelRouter(router *agent.ModelRouter) {
	a.modelRouter = router
}

func (a *App) GetModelRouter() *agent.ModelRouter {
	return a.modelRouter
}

func (a *App) SetChannelManager(cm *channels.Manager) {
	a.channelManager = cm
}

func (a *App) GetTUIChannel() *TUIChannel {
	return a.tuiChannel
}

func (a *App) GetActivityTracker() *ActivityTracker {
	return a.activityTracker
}

func (a *App) SetQATracker(qt *QATracker) {
	a.qaTracker = qt
}

func (a *App) GetQATracker() *QATracker {
	return a.qaTracker
}

func (a *App) StartBackgroundTasks() {
	a.startChannelsRefresh(a.ctx)
	a.startSessionsRefresh(a.ctx)
	a.startTokensRefresh(a.ctx)
	a.startFocusRefresh(a.ctx)
}

func (a *App) Run() error {
	return a.tviewApp.Run()
}

func (a *App) Stop() {
	a.cancel()
	if a.errorTracker != nil {
		a.errorTracker.Close()
	}
	a.tviewApp.Stop()
}

// QueueUpdateDraw queues a UI update on the tview event loop.
func (a *App) QueueUpdateDraw(f func()) {
	a.tviewApp.QueueUpdateDraw(f)
}
