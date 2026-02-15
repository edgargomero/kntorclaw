package tui

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type TokenUsage struct {
	InputTokens  int
	OutputTokens int
}

type TokenTracker struct {
	sessions map[string]*TokenUsage
	mu       sync.RWMutex
}

func NewTokenTracker() *TokenTracker {
	return &TokenTracker{
		sessions: make(map[string]*TokenUsage),
	}
}

func (t *TokenTracker) Add(sessionKey string, input, output int) {
	t.mu.Lock()
	defer t.mu.Unlock()

	usage, ok := t.sessions[sessionKey]
	if !ok {
		usage = &TokenUsage{}
		t.sessions[sessionKey] = usage
	}
	usage.InputTokens += input
	usage.OutputTokens += output
}

func (t *TokenTracker) GetAll() map[string]*TokenUsage {
	t.mu.RLock()
	defer t.mu.RUnlock()

	result := make(map[string]*TokenUsage, len(t.sessions))
	for k, v := range t.sessions {
		result[k] = &TokenUsage{
			InputTokens:  v.InputTokens,
			OutputTokens: v.OutputTokens,
		}
	}
	return result
}

func (a *App) buildTokensPanel() *tview.Table {
	a.tokensTable = newTable("TOKENS [F4]")
	a.setTokensHeader()
	return a.tokensTable
}

func (a *App) setTokensHeader() {
	headers := []string{"Session", "In", "Out"}
	for i, h := range headers {
		cell := tview.NewTableCell(h).
			SetTextColor(tcell.ColorYellow).
			SetSelectable(false).
			SetExpansion(1)
		a.tokensTable.SetCell(0, i, cell)
	}
}

func (a *App) startTokensRefresh(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	go func() {
		a.refreshTokens()
		for {
			select {
			case <-ctx.Done():
				ticker.Stop()
				return
			case <-ticker.C:
				a.refreshTokens()
			}
		}
	}()
}

func (a *App) refreshTokens() {
	all := a.tokenTracker.GetAll()

	a.tviewApp.QueueUpdateDraw(func() {
		rowCount := a.tokensTable.GetRowCount()
		for r := rowCount - 1; r >= 1; r-- {
			a.tokensTable.RemoveRow(r)
		}

		row := 1
		keys := make([]string, 0, len(all))
		for k := range all {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		for _, key := range keys {
			usage := all[key]
			a.tokensTable.SetCell(row, 0,
				tview.NewTableCell(key).SetExpansion(1))
			a.tokensTable.SetCell(row, 1,
				tview.NewTableCell(formatTokenCount(usage.InputTokens)).SetExpansion(1))
			a.tokensTable.SetCell(row, 2,
				tview.NewTableCell(formatTokenCount(usage.OutputTokens)).SetExpansion(1))
			row++
		}
	})
}

func formatTokenCount(n int) string {
	if n >= 1000 {
		return fmt.Sprintf("%.1fk", float64(n)/1000)
	}
	return fmt.Sprintf("%d", n)
}
