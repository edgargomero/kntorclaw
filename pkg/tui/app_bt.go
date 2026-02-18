package tui

import (
	"fmt"
	"log"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/sipeed/picoclaw/pkg/agent"
	"github.com/sipeed/picoclaw/pkg/bus"
	"github.com/sipeed/picoclaw/pkg/logger"
	"github.com/sipeed/picoclaw/pkg/providers"
	"github.com/sipeed/picoclaw/pkg/tui/domain"
	"github.com/sipeed/picoclaw/pkg/tui/organisms"
)

// BubbleteaApp wraps the bubbletea program and its dependencies.
type BubbleteaApp struct {
	program   *tea.Program
	model     Model
	channel   *BubbleteaTUIChannel
	tracker   *domain.ActivityTracker
	errors    *domain.ErrorTracker
	logWriter *organisms.LogWriter
}

// NewBubbleteaApp creates a new bubbletea-based TUI application.
func NewBubbleteaApp(msgBus *bus.MessageBus) *BubbleteaApp {
	tracker := domain.NewActivityTracker()
	channel := NewBubbleteaTUIChannel(msgBus)

	model := NewModel(tracker, nil)

	// Wire user messages to the channel bus
	model.SetOnUserMessage(func(content string) {
		channel.SendUserMessage(content)
	})

	return &BubbleteaApp{
		model:   model,
		channel: channel,
		tracker: tracker,
	}
}

// SetErrorLogPath initialises the error tracker with the given backlog path
// and re-creates the model so it receives the tracker reference.
func (a *BubbleteaApp) SetErrorLogPath(path string) {
	et, err := domain.NewErrorTracker(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not create error tracker: %v\n", err)
		return
	}
	a.errors = et
	a.model = NewModel(a.tracker, a.errors)
	// Re-wire user message callback
	ch := a.channel
	a.model.SetOnUserMessage(func(content string) {
		ch.SendUserMessage(content)
	})
}

// SetHealthChecker wires provider health status changes to the TUI.
func (a *BubbleteaApp) SetHealthChecker(hc *providers.HealthChecker) {
	hc.SetOnChange(func(statuses map[string]*providers.ProviderStatus) {
		for name, status := range statuses {
			a.channel.SendHealthChange(name, status.Reachable)
		}
	})
}

// SetModelRouter configures the model picker with data from the router and
// wires model selection callbacks to update the router.
func (a *BubbleteaApp) SetModelRouter(router *agent.ModelRouter) {
	a.model.SetModelRouterRef(router)

	// Wire model selection callback
	a.model.SetOnModelSelected(func(model, scope, channel string) {
		switch scope {
		case "session":
			router.SetSessionModel(channel, model)
		case "channel":
			router.SetChannelModel(channel, model)
		case "default":
			router.SetDefaultModel(model)
		}
	})
}

// SetConfigCallbacks sets the config callbacks on the model for the config page.
func (a *BubbleteaApp) SetConfigCallbacks(cb *organisms.ConfigCallbacks) {
	a.model.SetConfigCallbacks(cb)
}

// SetQATracker sets the QA tracker on the model for the focus page.
func (a *BubbleteaApp) SetQATracker(qt *domain.QATracker) {
	a.model.SetQATracker(qt)
}

// SetWorkspacePath sets the workspace path for sessions disk merge and token usage.
func (a *BubbleteaApp) SetWorkspacePath(path string) {
	a.model.SetWorkspacePath(path)
}

// SetUsagePath sets the token usage path for loading historical totals.
func (a *BubbleteaApp) SetUsagePath(path string) {
	a.model.SetUsagePath(path)
}

// Run starts the bubbletea program and redirects log output.
func (a *BubbleteaApp) Run() error {
	a.program = tea.NewProgram(a.model, tea.WithAltScreen(), tea.WithMouseCellMotion())

	// Wire up channel
	a.channel.SetProgram(a.program)

	// Wire up log writer — redirect Go's log package to TUI logs panel
	a.logWriter = organisms.NewLogWriter(a.program)
	log.SetOutput(a.logWriter)
	log.SetFlags(0)

	// Wire error tracker to logger
	if a.errors != nil {
		logger.SetOnError(func(entry logger.LogEntry) {
			a.errors.Add(entry)
		})
	}

	// Run the program (blocks until quit)
	if _, err := a.program.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running TUI: %v\n", err)
		return err
	}
	return nil
}

// GetChannel returns the bubbletea TUI channel for registration with channel manager.
func (a *BubbleteaApp) GetChannel() *BubbleteaTUIChannel {
	return a.channel
}

// GetLogWriter returns the log writer for bridging log output to the TUI.
func (a *BubbleteaApp) GetLogWriter() *organisms.LogWriter {
	return a.logWriter
}

// GetTracker returns the activity tracker.
func (a *BubbleteaApp) GetTracker() *domain.ActivityTracker {
	return a.tracker
}

// GetErrorTracker returns the error tracker (may be nil).
func (a *BubbleteaApp) GetErrorTracker() *domain.ErrorTracker {
	return a.errors
}
