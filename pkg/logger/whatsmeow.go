package logger

import (
	"fmt"

	waLog "go.mau.fi/whatsmeow/util/log"
)

// WaLogger adapts PicoClaw's logger to whatsmeow's waLog.Logger interface.
type WaLogger struct {
	module string
}

var _ waLog.Logger = (*WaLogger)(nil)

func NewWaLogger(module string) *WaLogger {
	return &WaLogger{module: module}
}

func (w *WaLogger) Errorf(msg string, args ...interface{}) {
	ErrorC(w.module, fmt.Sprintf(msg, args...))
}

func (w *WaLogger) Warnf(msg string, args ...interface{}) {
	WarnC(w.module, fmt.Sprintf(msg, args...))
}

func (w *WaLogger) Infof(msg string, args ...interface{}) {
	InfoC(w.module, fmt.Sprintf(msg, args...))
}

func (w *WaLogger) Debugf(msg string, args ...interface{}) {
	DebugC(w.module, fmt.Sprintf(msg, args...))
}

func (w *WaLogger) Sub(module string) waLog.Logger {
	return &WaLogger{module: fmt.Sprintf("%s/%s", w.module, module)}
}
