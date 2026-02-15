package tui

import (
	"strings"

	"github.com/rivo/tview"
)

const maxLogLines = 1000

type LogWriter struct {
	app      *App
	logsView *tview.TextView
}

func NewLogWriter(app *App, logsView *tview.TextView) *LogWriter {
	return &LogWriter{
		app:      app,
		logsView: logsView,
	}
}

func (w *LogWriter) Write(p []byte) (n int, err error) {
	text := strings.TrimRight(string(p), "\n")
	if text == "" {
		return len(p), nil
	}

	w.app.tviewApp.QueueUpdateDraw(func() {
		w.logsView.Write([]byte(text + "\n"))
		trimLogView(w.logsView)
	})
	return len(p), nil
}

func trimLogView(tv *tview.TextView) {
	text := tv.GetText(false)
	lines := strings.Split(text, "\n")
	if len(lines) > maxLogLines {
		trimmed := strings.Join(lines[len(lines)-maxLogLines:], "\n")
		tv.SetText(trimmed)
	}
}

