package tui

import (
	"context"
	"sort"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func (a *App) buildChannelsPanel() *tview.Table {
	a.channelsTable = newTable("CHANNELS [F3]")
	a.setChannelsHeader()
	return a.channelsTable
}

func (a *App) setChannelsHeader() {
	headers := []string{"Channel", "Status"}
	for i, h := range headers {
		cell := tview.NewTableCell(h).
			SetTextColor(tcell.ColorYellow).
			SetSelectable(false).
			SetExpansion(1)
		a.channelsTable.SetCell(0, i, cell)
	}
}

func (a *App) startChannelsRefresh(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	go func() {
		a.refreshChannels()
		for {
			select {
			case <-ctx.Done():
				ticker.Stop()
				return
			case <-ticker.C:
				a.refreshChannels()
			}
		}
	}()
}

func (a *App) refreshChannels() {
	if a.channelManager == nil {
		return
	}

	status := a.channelManager.GetStatus()

	a.tviewApp.QueueUpdateDraw(func() {
		// Clear rows except header
		rowCount := a.channelsTable.GetRowCount()
		for r := rowCount - 1; r >= 1; r-- {
			a.channelsTable.RemoveRow(r)
		}

		row := 1
		names := make([]string, 0, len(status))
		for name := range status {
			names = append(names, name)
		}
		sort.Strings(names)

		for _, name := range names {
			info := status[name].(map[string]interface{})
			running := info["running"].(bool)

			statusText := "[red]offline"
			if running {
				statusText = "[green]online"
			}

			a.channelsTable.SetCell(row, 0,
				tview.NewTableCell(name).SetExpansion(1))
			a.channelsTable.SetCell(row, 1,
				tview.NewTableCell(statusText).SetExpansion(1))
			row++
		}
	})
}
