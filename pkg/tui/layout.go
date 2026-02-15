package tui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func (a *App) buildLayout() *tview.Flex {
	// Left column: Chat (60%) + Logs (40%)
	leftColumn := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(a.buildChatPanel(), 0, 6, true).
		AddItem(a.buildLogsPanel(), 0, 4, false)

	// Right column: Channels (25%) + Tokens (25%) + Sessions (25%) + Config (25%)
	rightColumn := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(a.buildChannelsPanel(), 0, 1, false).
		AddItem(a.buildTokensPanel(), 0, 1, false).
		AddItem(a.buildSessionsPanel(), 0, 1, false).
		AddItem(a.buildConfigPanel(), 0, 1, false)

	// Main content: Left (55%) + Right (45%)
	content := tview.NewFlex().
		AddItem(leftColumn, 0, 55, true).
		AddItem(rightColumn, 0, 45, false)

	// Full layout: Header + Content + Input
	layout := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(a.buildHeader(), 1, 0, false).
		AddItem(content, 0, 1, true).
		AddItem(a.chatInput, 3, 0, false)

	return layout
}

func (a *App) buildHeader() *tview.TextView {
	a.statusBar = tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft)
	a.statusBar.SetBackgroundColor(tcell.ColorDarkBlue)
	a.updateStatusBar()
	return a.statusBar
}

func (a *App) updateStatusBar() {
	projectIndicator := ""
	if a.isProjectMode {
		projectIndicator = " [black:green] PROJECT [-:-] "
	}
	if a.focusMode {
		a.statusBar.SetText(" PicoClaw " + a.version + projectIndicator + " [black:yellow] FOCUS MODE [-:-]  [yellow]F1[white]-Files [yellow]F2[white]-Branches [yellow]F3[white]-Commits [yellow]F4[white]-Chat [yellow]F5[white]-Diff [yellow]F6[white]-QA  [yellow]F9[white] exit  [yellow]Esc[white] input  [yellow]Ctrl+C[white] quit")
	} else {
		a.statusBar.SetText(" PicoClaw " + a.version + projectIndicator + " [yellow]F1[white]-Chat [yellow]F2[white]-Logs [yellow]F3[white]-Channels [yellow]F4[white]-Tokens [yellow]F5[white]-Sessions [yellow]F6[white]-Config  [yellow]F9[white] focus  [yellow]Tab[white]/[yellow]Shift+Tab[white] navigate  [yellow]Shift+Enter[white] newline  [yellow]Ctrl+C[white] quit")
	}
}

func (a *App) buildFocusLayout() *tview.Flex {
	// Left column: Files + Branches + Commits + Spec (1:1:1:1)
	leftColumn := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(a.focusFiles, 0, 1, false).
		AddItem(a.focusBranches, 0, 1, false).
		AddItem(a.focusCommits, 0, 1, false).
		AddItem(a.focusQA, 0, 1, false)

	// Right column: Chat (60%) + Diff (40%)
	rightColumn := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(a.chatHistory, 0, 6, true).
		AddItem(a.focusDiff, 0, 4, false)

	// Main content: Left (30%) + Right (70%)
	content := tview.NewFlex().
		AddItem(leftColumn, 0, 30, false).
		AddItem(rightColumn, 0, 70, true)

	// Full layout: Header + Content + Input
	layout := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(a.statusBar, 1, 0, false).
		AddItem(content, 0, 1, true).
		AddItem(a.chatInput, 3, 0, false)

	return layout
}

func (a *App) toggleFocusMode() {
	if a.focusMode {
		// Exit focus mode
		a.focusMode = false
		a.panels = []tview.Primitive{
			a.chatHistory,
			a.logsView,
			a.channelsTable,
			a.tokensTable,
			a.sessionsTable,
			a.configView,
		}
		a.focusIndex = 0
		a.updateStatusBar()
		a.tviewApp.SetRoot(a.normalLayout, true)
		a.tviewApp.SetFocus(a.chatInput)
	} else {
		// Enter focus mode
		a.focusMode = true
		a.refreshFocusPanels()
		a.focusLayout = a.buildFocusLayout()
		a.panels = []tview.Primitive{
			a.focusFiles,
			a.focusBranches,
			a.focusCommits,
			a.chatHistory,
			a.focusDiff,
			a.focusQA,
		}
		a.focusIndex = 3 // Chat
		a.updateStatusBar()
		a.tviewApp.SetRoot(a.focusLayout, true)
		a.tviewApp.SetFocus(a.chatInput)
	}
}

func (a *App) buildLogsPanel() *tview.TextView {
	return a.logsView
}

func newPanel(title string) *tview.TextView {
	tv := tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true).
		SetWordWrap(true)
	tv.SetBorder(true).SetTitle(" " + title + " ")
	return tv
}

func newTable(title string) *tview.Table {
	t := tview.NewTable().
		SetBorders(false).
		SetSelectable(false, false).
		SetFixed(1, 0)
	t.SetBorder(true).SetTitle(" " + title + " ")
	return t
}
