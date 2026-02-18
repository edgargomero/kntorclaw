package organisms

import (
	"fmt"
	"sort"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/sipeed/picoclaw/pkg/tui/atoms"
	"github.com/sipeed/picoclaw/pkg/tui/domain"
)

// TokensPanelModel displays per-session and cumulative token usage in a table.
type TokensPanelModel struct {
	table   atoms.TableModel
	tracker *domain.ActivityTracker
	width   int
	height  int
	focused bool
}

// NewTokensPanel creates a TokensPanelModel bound to the given ActivityTracker.
// tracker may be nil; the panel will display an empty table in that case.
func NewTokensPanel(tracker *domain.ActivityTracker) TokensPanelModel {
	table := atoms.NewTable(
		[]string{"Session", "In", "Out", "\u03a3In", "\u03a3Out"},
		[]int{0, 0, 0, 0, 0}, // will be computed on SetSize
	)
	m := TokensPanelModel{
		table:   table,
		tracker: tracker,
	}
	m.refresh()
	return m
}

// SetSize updates the panel dimensions and recalculates column widths.
func (m *TokensPanelModel) SetSize(w, h int) {
	m.width = w
	m.height = h

	// Account for border (1 left + 1 right)
	inner := w - 2
	if inner < 0 {
		inner = 0
	}
	cols := len(m.table.Header)
	if cols > 0 {
		each := inner / cols
		widths := make([]int, cols)
		for i := range widths {
			widths[i] = each
		}
		m.table.ColWidths = widths
	}
	m.table.Width = inner
	// height minus border (2) minus header (1)
	m.table.Height = h - 3
	m.refresh()
}

// SetFocused sets the focused state for border styling.
func (m *TokensPanelModel) SetFocused(focused bool) {
	m.focused = focused
}

// Update handles tick messages to refresh data from the tracker.
func (m TokensPanelModel) Update(msg tea.Msg) (TokensPanelModel, tea.Cmd) {
	switch msg.(type) {
	case TickTokensMsg:
		m.refresh()
	}
	return m, nil
}

// View renders the token table inside a styled panel border.
func (m TokensPanelModel) View() string {
	return atoms.RenderPanel("TOKENS [F4]", m.focused, m.width, m.height, m.table.View())
}

// refresh rebuilds the table rows from the ActivityTracker.
func (m *TokensPanelModel) refresh() {
	if m.tracker == nil {
		m.table.Rows = nil
		return
	}

	all := m.tracker.GetAll()
	totals := m.tracker.GetTotals()

	// Collect all session keys that have token data
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

	rows := make([][]string, 0, len(keys))
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
		rows = append(rows, []string{
			atoms.AbbreviateKey(key),
			atoms.FormatTokenCount(int64(inTok)),
			atoms.FormatTokenCount(int64(outTok)),
			atoms.FormatTokenCount(int64(totalIn)),
			atoms.FormatTokenCount(int64(totalOut)),
		})
	}

	if len(rows) == 0 {
		rows = [][]string{{"--", "0", "0", "0", "0"}}
	}

	m.table.Rows = rows
}

// SetUsagePath loads historical token totals from the given JSON file path.
func (m *TokensPanelModel) SetUsagePath(path string) {
	if m.tracker != nil {
		m.tracker.LoadTotals(path)
	}
}

// TickTokensMsg triggers a refresh of token data.
type TickTokensMsg struct{}

// FormatSummary returns a one-line summary for the status bar.
func (m TokensPanelModel) FormatSummary() string {
	if m.tracker == nil {
		return "tokens: 0/0"
	}
	all := m.tracker.GetAll()
	var totalIn, totalOut int
	for _, sa := range all {
		totalIn += sa.InputTokens
		totalOut += sa.OutputTokens
	}
	return fmt.Sprintf("tokens: %s/%s",
		atoms.FormatTokenCount(int64(totalIn)),
		atoms.FormatTokenCount(int64(totalOut)),
	)
}
