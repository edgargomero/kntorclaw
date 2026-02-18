package domain

import (
	"encoding/json"
	"os"
	"sync"

	"github.com/sipeed/picoclaw/pkg/logger"
)

const maxRecentErrors = 100

type ErrorTracker struct {
	mu      sync.Mutex
	errors  []logger.LogEntry
	count   int
	backlog *os.File
}

func NewErrorTracker(path string) (*ErrorTracker, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, err
	}
	return &ErrorTracker{backlog: f}, nil
}

func (t *ErrorTracker) Add(entry logger.LogEntry) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.count++

	// Circular buffer
	if len(t.errors) >= maxRecentErrors {
		t.errors = t.errors[1:]
	}
	t.errors = append(t.errors, entry)

	// Persist to backlog
	if t.backlog != nil {
		if data, err := json.Marshal(entry); err == nil {
			t.backlog.WriteString(string(data) + "\n")
		}
	}
}

func (t *ErrorTracker) Count() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.count
}

func (t *ErrorTracker) Clear() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.count = 0
}

func (t *ErrorTracker) Recent(n int) []logger.LogEntry {
	t.mu.Lock()
	defer t.mu.Unlock()

	if n > len(t.errors) {
		n = len(t.errors)
	}
	start := len(t.errors) - n
	result := make([]logger.LogEntry, n)
	copy(result, t.errors[start:])
	return result
}

func (t *ErrorTracker) Close() {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.backlog != nil {
		t.backlog.Close()
		t.backlog = nil
	}
}
