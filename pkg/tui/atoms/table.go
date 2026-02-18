package atoms

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// TableModel is a generic table component
type TableModel struct {
	Header    []string
	Rows      [][]string
	ColWidths []int
	Width     int
	Height    int
}

func NewTable(header []string, colWidths []int) TableModel {
	return TableModel{
		Header:    header,
		ColWidths: colWidths,
	}
}

func (t TableModel) View() string {
	if len(t.Header) == 0 {
		return ""
	}

	var b strings.Builder

	// Header
	headerCells := make([]string, len(t.Header))
	for i, h := range t.Header {
		w := t.colWidth(i)
		headerCells[i] = HeaderStyle.Width(w).Render(h)
	}
	b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, headerCells...))
	b.WriteString("\n")

	// Rows
	for rowIdx, row := range t.Rows {
		cells := make([]string, len(row))
		rowStyle := lipgloss.NewStyle()
		if rowIdx%2 == 1 {
			rowStyle = rowStyle.Faint(true)
		}
		for i, cell := range row {
			w := t.colWidth(i)
			cells[i] = rowStyle.Width(w).Render(truncate(cell, w))
		}
		b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, cells...))
		if rowIdx < len(t.Rows)-1 {
			b.WriteString("\n")
		}
	}

	return b.String()
}

func (t TableModel) colWidth(i int) int {
	if i < len(t.ColWidths) && t.ColWidths[i] > 0 {
		return t.ColWidths[i]
	}
	if t.Width > 0 && len(t.Header) > 0 {
		return t.Width / len(t.Header)
	}
	return 15
}

func truncate(s string, maxWidth int) string {
	if lipgloss.Width(s) <= maxWidth {
		return s
	}
	if maxWidth <= 1 {
		return "…"
	}
	runes := []rune(s)
	for i := len(runes); i > 0; i-- {
		candidate := string(runes[:i]) + "…"
		if lipgloss.Width(candidate) <= maxWidth {
			return candidate
		}
	}
	return "…"
}
