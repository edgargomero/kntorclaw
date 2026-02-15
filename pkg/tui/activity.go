package tui

import (
	"sync"
	"time"
)

// SessionActivity holds runtime activity state for a single session.
type SessionActivity struct {
	Status       string    // "idle", "processing", "tool:<name>"
	Iterations   int       // total LLM iterations this session
	ToolCalls    int       // total tool calls made
	LastTool     string    // last tool used
	LastPreview  string    // truncated last response
	LastActive   time.Time
	InputTokens  int
	OutputTokens int
}

// ActivityTracker aggregates per-session runtime activity (status, tokens, tool calls).
type ActivityTracker struct {
	sessions map[string]*SessionActivity
	mu       sync.RWMutex
}

func NewActivityTracker() *ActivityTracker {
	return &ActivityTracker{
		sessions: make(map[string]*SessionActivity),
	}
}

func (t *ActivityTracker) getOrCreate(sessionKey string) *SessionActivity {
	sa, ok := t.sessions[sessionKey]
	if !ok {
		sa = &SessionActivity{Status: "idle"}
		t.sessions[sessionKey] = sa
	}
	return sa
}

// SetProcessing marks a session as processing a new request.
func (t *ActivityTracker) SetProcessing(sessionKey string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	sa := t.getOrCreate(sessionKey)
	sa.Status = "processing"
	sa.LastActive = time.Now()
}

// SetToolCall records a tool call for the session.
func (t *ActivityTracker) SetToolCall(sessionKey, toolName string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	sa := t.getOrCreate(sessionKey)
	sa.Status = "tool:" + toolName
	sa.ToolCalls++
	sa.LastTool = toolName
	sa.LastActive = time.Now()
}

// SetIdle marks a session as idle after completing a request.
func (t *ActivityTracker) SetIdle(sessionKey string, responsePreview string, iterations int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	sa := t.getOrCreate(sessionKey)
	sa.Status = "idle"
	sa.LastPreview = responsePreview
	sa.Iterations += iterations
	sa.LastActive = time.Now()
}

// AddTokens accumulates token usage for the session.
func (t *ActivityTracker) AddTokens(sessionKey string, input, output int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	sa := t.getOrCreate(sessionKey)
	sa.InputTokens += input
	sa.OutputTokens += output
}

// GetAll returns a snapshot of all session activities.
func (t *ActivityTracker) GetAll() map[string]*SessionActivity {
	t.mu.RLock()
	defer t.mu.RUnlock()

	result := make(map[string]*SessionActivity, len(t.sessions))
	for k, v := range t.sessions {
		copy := *v
		result[k] = &copy
	}
	return result
}
