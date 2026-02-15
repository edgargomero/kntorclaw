package tui

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
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
	headers := []string{"Key", "Msgs", "Updated"}
	for i, h := range headers {
		cell := tview.NewTableCell(h).
			SetTextColor(tcell.ColorYellow).
			SetSelectable(false).
			SetExpansion(1)
		a.sessionsTable.SetCell(0, i, cell)
	}
}

func (a *App) startSessionsRefresh(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Second)
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
	sessionsDir := filepath.Join(a.config.WorkspacePath(), "sessions")
	entries, err := os.ReadDir(sessionsDir)
	if err != nil {
		return
	}

	var sessions []sessionInfo
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(sessionsDir, entry.Name()))
		if err != nil {
			continue
		}
		var raw struct {
			Key      string    `json:"key"`
			Updated  time.Time `json:"updated"`
			Messages []json.RawMessage `json:"messages"`
		}
		if json.Unmarshal(data, &raw) != nil {
			continue
		}
		sessions = append(sessions, sessionInfo{
			Key:      raw.Key,
			Updated:  raw.Updated,
			MsgCount: len(raw.Messages),
		})
	}

	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].Updated.After(sessions[j].Updated)
	})

	a.tviewApp.QueueUpdateDraw(func() {
		rowCount := a.sessionsTable.GetRowCount()
		for r := rowCount - 1; r >= 1; r-- {
			a.sessionsTable.RemoveRow(r)
		}

		for i, s := range sessions {
			row := i + 1
			a.sessionsTable.SetCell(row, 0,
				tview.NewTableCell(s.Key).SetExpansion(1))
			a.sessionsTable.SetCell(row, 1,
				tview.NewTableCell(fmt.Sprintf("%d", s.MsgCount)).SetExpansion(1))
			a.sessionsTable.SetCell(row, 2,
				tview.NewTableCell(timeAgo(s.Updated)).SetExpansion(1))
		}
	})
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
