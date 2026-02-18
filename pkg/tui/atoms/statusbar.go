package atoms

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

type clearToastMsg struct{}

// StatusBarModel displays status text with optional toast messages
type StatusBarModel struct {
	Text      string
	ToastText string
	toastEnd  time.Time
}

func NewStatusBar(text string) StatusBarModel {
	return StatusBarModel{Text: text}
}

func (s *StatusBarModel) SetToast(msg string, duration time.Duration) tea.Cmd {
	s.ToastText = msg
	s.toastEnd = time.Now().Add(duration)
	return tea.Tick(duration, func(time.Time) tea.Msg {
		return clearToastMsg{}
	})
}

func (s *StatusBarModel) Update(msg tea.Msg) tea.Cmd {
	switch msg.(type) {
	case clearToastMsg:
		if time.Now().After(s.toastEnd) {
			s.ToastText = ""
		}
	}
	return nil
}

func (s StatusBarModel) View(width int) string {
	text := s.Text
	if s.ToastText != "" {
		text = s.ToastText
	}
	return StatusBarStyle.Width(width).Render(text)
}
