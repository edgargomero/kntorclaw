package tui

import (
	"time"
)

// Tick messages for periodic updates
type tickTokensMsg time.Time
type tickChannelsMsg time.Time
type tickSessionsMsg time.Time
type tickFocusMsg time.Time
