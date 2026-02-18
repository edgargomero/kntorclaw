package organisms

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/sipeed/picoclaw/pkg/tui/atoms"
	"github.com/sipeed/picoclaw/pkg/tui/domain"
)

// SessionEntry represents a single session row for display.
type SessionEntry struct {
	Key        string
	Channel    string
	LastActive time.Time
	Messages   int
	Status     string // "idle", "processing", "tool:<name>"
	InTokens   int
	OutTokens  int
}

// SessionsPanelModel displays active sessions in a table.
type SessionsPanelModel struct {
	table         atoms.TableModel
	tracker       *domain.ActivityTracker
	sessions      []SessionEntry
	workspacePath string
	width         int
	height        int
	focused       bool
}

// SetWorkspacePath sets the workspace path for reading sessions from disk.
func (m *SessionsPanelModel) SetWorkspacePath(path string) { m.workspacePath = path }

// NewSessionsPanel creates a SessionsPanelModel bound to an ActivityTracker.
func NewSessionsPanel(tracker *domain.ActivityTracker) SessionsPanelModel {
	table := atoms.NewTable(
		[]string{"Key", "Status", "Tokens", "Last Activity"},
		[]int{0, 0, 0, 0},
	)
	m := SessionsPanelModel{
		table:   table,
		tracker: tracker,
	}
	m.rebuildRows()
	return m
}

// SetSize updates the panel dimensions and recalculates column widths.
func (m *SessionsPanelModel) SetSize(w, h int) {
	m.width = w
	m.height = h

	inner := w - 2
	if inner < 0 {
		inner = 0
	}
	// Distribute: Key(30%), Status(20%), Tokens(20%), LastActivity(30%)
	keyW := inner * 3 / 10
	statusW := inner * 2 / 10
	tokensW := inner * 2 / 10
	actW := inner - keyW - statusW - tokensW
	m.table.ColWidths = []int{keyW, statusW, tokensW, actW}
	m.table.Width = inner
	m.table.Height = h - 3
	m.rebuildRows()
}

// SetFocused sets the focused state for border styling.
func (m *SessionsPanelModel) SetFocused(focused bool) {
	m.focused = focused
}

// SetSessions directly sets the session entries (for external data sources).
func (m *SessionsPanelModel) SetSessions(entries []SessionEntry) {
	m.sessions = entries
	m.rebuildRows()
}

// Update handles tick messages to refresh from the activity tracker.
func (m SessionsPanelModel) Update(msg tea.Msg) (SessionsPanelModel, tea.Cmd) {
	switch msg.(type) {
	case TickSessionsMsg:
		m.refreshFromTracker()
		m.rebuildRows()
	}
	return m, nil
}

// View renders the sessions table inside a styled panel border.
func (m SessionsPanelModel) View() string {
	return atoms.RenderPanel("SESSIONS [F5]", m.focused, m.width, m.height, m.table.View())
}

func (m *SessionsPanelModel) refreshFromTracker() {
	if m.tracker == nil {
		return
	}

	all := m.tracker.GetAll()
	entries := make([]SessionEntry, 0, len(all))
	for key, sa := range all {
		entries = append(entries, SessionEntry{
			Key:        key,
			Channel:    channelFromKey(key),
			LastActive: sa.LastActive,
			Status:     sa.Status,
			InTokens:   sa.InputTokens,
			OutTokens:  sa.OutputTokens,
		})
	}
	// Sort by most recently active
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].LastActive.After(entries[j].LastActive)
	})
	m.sessions = entries
	m.mergeDiskSessions()
}

func (m *SessionsPanelModel) mergeDiskSessions() {
	if m.workspacePath == "" {
		return
	}
	sessDir := filepath.Join(m.workspacePath, "sessions")
	entries, err := os.ReadDir(sessDir)
	if err != nil {
		return
	}

	// Build lookup of existing session keys
	existing := make(map[string]bool)
	for _, s := range m.sessions {
		existing[s.Key] = true
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(sessDir, entry.Name()))
		if err != nil {
			continue
		}
		var diskSess struct {
			Key          string            `json:"key"`
			Messages     []json.RawMessage `json:"messages"`
			LastActivity time.Time         `json:"last_activity"`
		}
		if err := json.Unmarshal(data, &diskSess); err != nil {
			continue
		}
		if diskSess.Key == "" {
			diskSess.Key = strings.TrimSuffix(entry.Name(), ".json")
		}
		if existing[diskSess.Key] {
			continue // tracker has fresher data
		}
		m.sessions = append(m.sessions, SessionEntry{
			Key:        diskSess.Key,
			Channel:    channelFromKey(diskSess.Key),
			LastActive: diskSess.LastActivity,
			Messages:   len(diskSess.Messages),
			Status:     "saved",
		})
	}
}

func (m *SessionsPanelModel) rebuildRows() {
	statusProcessing := lipgloss.NewStyle().Foreground(atoms.ColorYellow)
	statusTool := lipgloss.NewStyle().Foreground(atoms.ColorCyan)
	statusIdle := lipgloss.NewStyle().Foreground(atoms.ColorGreen)

	rows := make([][]string, 0, len(m.sessions))
	for _, s := range m.sessions {
		// Format status with color
		var statusStr string
		switch {
		case strings.HasPrefix(s.Status, "tool:"):
			statusStr = statusTool.Render(s.Status)
		case s.Status == "processing":
			statusStr = statusProcessing.Render("processing")
		default:
			statusStr = statusIdle.Render("idle")
		}

		// Format tokens
		tokenStr := "-"
		if s.InTokens > 0 || s.OutTokens > 0 {
			tokenStr = fmt.Sprintf("%s/%s",
				atoms.FormatTokenCount(int64(s.InTokens)),
				atoms.FormatTokenCount(int64(s.OutTokens)),
			)
		}

		// Format last active
		lastActive := "-"
		if !s.LastActive.IsZero() {
			lastActive = atoms.TimeAgo(s.LastActive)
		}

		rows = append(rows, []string{
			abbreviateSessionKey(s.Key),
			statusStr,
			tokenStr,
			lastActive,
		})
	}

	if len(rows) == 0 {
		rows = [][]string{{"--", "--", "--", "--"}}
	}

	m.table.Rows = rows
}

// abbreviateSessionKey shortens common channel prefixes for display.
func abbreviateSessionKey(key string) string {
	replacements := map[string]string{
		"whatsapp:": "wa:",
		"telegram:": "tg:",
		"discord:":  "dc:",
	}
	for prefix, short := range replacements {
		if strings.HasPrefix(key, prefix) {
			return short + key[len(prefix):]
		}
	}
	return atoms.AbbreviateKey(key)
}

// channelFromKey extracts the channel name from a session key like "telegram:12345".
func channelFromKey(key string) string {
	if idx := strings.Index(key, ":"); idx >= 0 {
		return key[:idx]
	}
	return "cli"
}

// TickSessionsMsg triggers a periodic refresh of session data.
type TickSessionsMsg struct{}
