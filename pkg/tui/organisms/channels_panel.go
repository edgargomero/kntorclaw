package organisms

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/sipeed/picoclaw/pkg/tui/atoms"
)

// ChannelsPanelModel displays channel health status in a table.
type ChannelsPanelModel struct {
	table            atoms.TableModel
	statuses         map[string]bool // channel name -> online
	providerStatuses map[string]bool // provider name -> reachable
	width            int
	height           int
	focused          bool
}

// NewChannelsPanel creates a ChannelsPanelModel.
func NewChannelsPanel() ChannelsPanelModel {
	table := atoms.NewTable(
		[]string{"Channel", "Status", "Provider"},
		[]int{0, 0, 0},
	)
	m := ChannelsPanelModel{
		table:            table,
		statuses:         make(map[string]bool),
		providerStatuses: make(map[string]bool),
	}
	m.rebuildRows()
	return m
}

// SetSize updates the panel dimensions and recalculates column widths.
func (m *ChannelsPanelModel) SetSize(w, h int) {
	m.width = w
	m.height = h

	inner := w - 2
	if inner < 0 {
		inner = 0
	}
	nameW := inner * 2 / 5
	statusW := inner * 2 / 5
	providerW := inner - nameW - statusW
	m.table.ColWidths = []int{nameW, statusW, providerW}
	m.table.Width = inner
	m.table.Height = h - 3
	m.rebuildRows()
}

// SetFocused sets the focused state for border styling.
func (m *ChannelsPanelModel) SetFocused(focused bool) {
	m.focused = focused
}

// Update handles tick and health-change messages.
func (m ChannelsPanelModel) Update(msg tea.Msg) (ChannelsPanelModel, tea.Cmd) {
	switch msg := msg.(type) {
	case TickChannelsMsg:
		_ = msg
		m.rebuildRows()
	case HealthChangedMsg:
		if strings.HasPrefix(msg.Channel, "provider:") {
			provName := strings.TrimPrefix(msg.Channel, "provider:")
			m.providerStatuses[provName] = msg.Online
		} else {
			m.statuses[msg.Channel] = msg.Online
		}
		m.rebuildRows()
	}
	return m, nil
}

// View renders the channels table inside a styled panel border.
func (m ChannelsPanelModel) View() string {
	return atoms.RenderPanel("CHANNELS [F3]", m.focused, m.width, m.height, m.table.View())
}

// SetStatuses bulk-updates channel statuses.
func (m *ChannelsPanelModel) SetStatuses(statuses map[string]bool) {
	m.statuses = statuses
	m.rebuildRows()
}

func (m *ChannelsPanelModel) rebuildRows() {
	onlineStyle := lipgloss.NewStyle().Foreground(atoms.ColorGreen)
	offlineStyle := lipgloss.NewStyle().Foreground(atoms.ColorRed)
	unknownStyle := lipgloss.NewStyle().Faint(true)

	rows := make([][]string, 0, len(m.statuses))
	for name, online := range m.statuses {
		status := offlineStyle.Render("offline")
		if online {
			status = onlineStyle.Render("online")
		}
		// Provider column
		providerCol := unknownStyle.Render("—")
		if reachable, ok := m.providerStatuses[name]; ok {
			if reachable {
				providerCol = onlineStyle.Render("●")
			} else {
				providerCol = offlineStyle.Render("●")
			}
		}
		rows = append(rows, []string{name, status, providerCol})
	}

	if len(rows) == 0 {
		rows = [][]string{{"--", "--", "--"}}
	}

	m.table.Rows = rows
}

// TickChannelsMsg triggers a periodic refresh.
type TickChannelsMsg struct{}

// HealthChangedMsg notifies about a channel status change.
type HealthChangedMsg struct {
	Channel string
	Online  bool
}
