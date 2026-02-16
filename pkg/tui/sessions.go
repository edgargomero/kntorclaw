package tui

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type sessionInfo struct {
	Key      string
	Updated  time.Time
	MsgCount int
}

func (a *App) buildSessionsPanel() *tview.Table {
	a.sessionsTable = newTable("SESSIONS [F5]")
	a.setSessionsHeader()
	return a.sessionsTable
}

func (a *App) setSessionsHeader() {
	headers := []string{"Key", "Status", "Tokens", "Last Activity"}
	for i, h := range headers {
		cell := tview.NewTableCell(h).
			SetTextColor(tcell.ColorYellow).
			SetSelectable(false).
			SetExpansion(1)
		a.sessionsTable.SetCell(0, i, cell)
	}
}

func (a *App) startSessionsRefresh(ctx context.Context) {
	ticker := time.NewTicker(3 * time.Second)
	go func() {
		a.refreshSessions()
		for {
			select {
			case <-ctx.Done():
				ticker.Stop()
				return
			case <-ticker.C:
				a.refreshSessions()
			}
		}
	}()
}

func (a *App) refreshSessions() {
	if a.configBusy.Load() {
		return
	}
	// Gather disk sessions
	sessionsDir := filepath.Join(a.config.WorkspacePath(), "sessions")
	entries, _ := os.ReadDir(sessionsDir)

	diskSessions := make(map[string]sessionInfo)
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(sessionsDir, entry.Name()))
		if err != nil {
			continue
		}
		var raw struct {
			Key      string            `json:"key"`
			Updated  time.Time         `json:"updated"`
			Messages []json.RawMessage `json:"messages"`
		}
		if json.Unmarshal(data, &raw) != nil {
			continue
		}
		diskSessions[raw.Key] = sessionInfo{
			Key:      raw.Key,
			Updated:  raw.Updated,
			MsgCount: len(raw.Messages),
		}
	}

	// Merge with activity tracker data
	activities := a.activityTracker.GetAll()

	// Collect all unique session keys
	allKeys := make(map[string]struct{})
	for k := range diskSessions {
		allKeys[k] = struct{}{}
	}
	for k := range activities {
		allKeys[k] = struct{}{}
	}

	type sessionRow struct {
		Key          string
		Status       string
		StatusColor  string
		Tokens       string
		LastActivity string
		SortTime     time.Time
	}

	var rows []sessionRow
	for key := range allKeys {
		row := sessionRow{Key: abbreviateKey(key)}

		act, hasActivity := activities[key]
		disk, hasDisk := diskSessions[key]

		// Status
		if hasActivity {
			switch {
			case strings.HasPrefix(act.Status, "tool:"):
				row.Status = act.Status
				row.StatusColor = "cyan"
			case act.Status == "processing":
				row.Status = "processing"
				row.StatusColor = "yellow"
			default:
				row.Status = "idle"
				row.StatusColor = "green"
			}
		} else {
			row.Status = "idle"
			row.StatusColor = "green"
		}

		// Tokens
		if hasActivity && (act.InputTokens > 0 || act.OutputTokens > 0) {
			row.Tokens = fmt.Sprintf("%s/%s", formatTokenCount(act.InputTokens), formatTokenCount(act.OutputTokens))
		} else {
			row.Tokens = "-"
		}

		// Last activity
		if hasActivity && act.LastPreview != "" {
			row.LastActivity = truncate(act.LastPreview, 40)
		} else if hasDisk {
			row.LastActivity = fmt.Sprintf("%d msgs", disk.MsgCount)
		} else {
			row.LastActivity = "-"
		}

		// Sort time
		if hasActivity && !act.LastActive.IsZero() {
			row.SortTime = act.LastActive
		} else if hasDisk {
			row.SortTime = disk.Updated
		}

		rows = append(rows, row)
	}

	sort.Slice(rows, func(i, j int) bool {
		return rows[i].SortTime.After(rows[j].SortTime)
	})

	a.tviewApp.QueueUpdateDraw(func() {
		rowCount := a.sessionsTable.GetRowCount()
		for r := rowCount - 1; r >= 1; r-- {
			a.sessionsTable.RemoveRow(r)
		}

		for i, s := range rows {
			row := i + 1
			a.sessionsTable.SetCell(row, 0,
				tview.NewTableCell(s.Key).SetExpansion(1))
			a.sessionsTable.SetCell(row, 1,
				tview.NewTableCell(fmt.Sprintf("[%s]%s[-]", s.StatusColor, s.Status)).SetExpansion(1))
			a.sessionsTable.SetCell(row, 2,
				tview.NewTableCell(s.Tokens).SetExpansion(1))
			a.sessionsTable.SetCell(row, 3,
				tview.NewTableCell(s.LastActivity).SetExpansion(1))
		}
	})
}

// abbreviateKey shortens common channel prefixes for display.
func abbreviateKey(key string) string {
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
	return key
}

// truncate returns at most n characters from s, appending "..." if truncated.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	if n > 3 {
		return s[:n-3] + "..."
	}
	return s[:n]
}

func timeAgo(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds ago", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
}
