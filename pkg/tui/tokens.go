package tui

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func (a *App) buildTokensPanel() *tview.Table {
	a.tokensTable = newTable("TOKENS [F4]")
	a.setTokensHeader()
	return a.tokensTable
}

func (a *App) setTokensHeader() {
	headers := []string{"Session", "In", "Out", "ΣIn", "ΣOut"}
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
	if a.configBusy.Load() {
		return
	}
	all := a.activityTracker.GetAll()
	totals := a.activityTracker.GetTotals()

	a.tviewApp.QueueUpdateDraw(func() {
		rowCount := a.tokensTable.GetRowCount()
		for r := rowCount - 1; r >= 1; r-- {
			a.tokensTable.RemoveRow(r)
		}

		// Collect keys from both session and historical totals
		keySet := make(map[string]struct{})
		for k, sa := range all {
			if sa.InputTokens > 0 || sa.OutputTokens > 0 {
				keySet[k] = struct{}{}
			}
		}
		for k, tt := range totals {
			if tt.InputTokens > 0 || tt.OutputTokens > 0 {
				keySet[k] = struct{}{}
			}
		}
		keys := make([]string, 0, len(keySet))
		for k := range keySet {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		row := 1
		for _, key := range keys {
			sa := all[key]
			tt := totals[key]
			var inTok, outTok int
			if sa != nil {
				inTok = sa.InputTokens
				outTok = sa.OutputTokens
			}
			var totalIn, totalOut int
			if tt != nil {
				totalIn = tt.InputTokens
				totalOut = tt.OutputTokens
			}
			a.tokensTable.SetCell(row, 0,
				tview.NewTableCell(key).SetExpansion(1))
			a.tokensTable.SetCell(row, 1,
				tview.NewTableCell(formatTokenCount(inTok)).SetExpansion(1))
			a.tokensTable.SetCell(row, 2,
				tview.NewTableCell(formatTokenCount(outTok)).SetExpansion(1))
			a.tokensTable.SetCell(row, 3,
				tview.NewTableCell(formatTokenCount(totalIn)).SetExpansion(1))
			a.tokensTable.SetCell(row, 4,
				tview.NewTableCell(formatTokenCount(totalOut)).SetExpansion(1))
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
