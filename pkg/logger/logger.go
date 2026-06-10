package logger

import (
	"time"
)

var (
	// EventChannel is a buffered channel used to pass log events to the TUI.
	EventChannel = make(chan LogEvent, 1000)
)

// LogEvent represents a single proxy access log event.
type LogEvent struct {
	Timestamp time.Time
	Action    string // ALLOW or BLOCK
	Method    string
	Host      string
	URL       string
	Reason    string
}

// LogAccess logs an allowed or blocked access request
func LogAccess(action, method, host, url, reason string) {

	event := LogEvent{
		Timestamp: time.Now(),
		Action:    action,
		Method:    method,
		Host:      host,
		URL:       url,
		Reason:    reason,
	}

	select {
	case EventChannel <- event:
	default:
		// channel full, drop to avoid blocking proxy
	}
}

// LogAllow logs an allowed request
func LogAllow(method, host, url string) {
	LogAccess("ALLOW", method, host, url, "")
}

// LogBlock logs a blocked request
func LogBlock(method, host, url, reason string) {
	LogAccess("BLOCK", method, host, url, reason)
}
