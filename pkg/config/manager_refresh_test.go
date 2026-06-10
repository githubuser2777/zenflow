package config

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
	"zenflow/pkg/filter"
)

type mockLogger struct {
	mu  sync.Mutex
	msg []string
}

func (m *mockLogger) Printf(format string, v ...interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.msg = append(m.msg, fmt.Sprintf(format, v...))
}

func (m *mockLogger) HasMessage(substring string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, msg := range m.msg {
		if stringContains(msg, substring) {
			return true
		}
	}
	return false
}

func stringContains(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	if len(s) < len(substr) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestAutoRefresh_WorksAndCancels(t *testing.T) {
	var mu sync.Mutex
	fetchCount := 0

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		fetchCount++
		c := fetchCount
		mu.Unlock()

		if c == 1 {
			fmt.Fprintln(w, "0.0.0.0 bad.com")
		} else {
			fmt.Fprintln(w, "0.0.0.0 bad.com\n0.0.0.0 worse.com")
		}
	}))
	defer ts.Close()

	b := filter.NewBlocker(nil)
	logger := &mockLogger{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	interval := 10 * time.Millisecond
	StartAutoRefresh(ctx, ts.URL, b, interval, logger)

	// Wait for at least 2 fetches
	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	c := fetchCount
	mu.Unlock()

	if c < 2 {
		t.Errorf("Expected at least 2 fetches, got %d", c)
	}

	if blocked, _ := b.IsBlocked("worse.com"); !blocked {
		t.Errorf("Blocklist not updated, worse.com not blocked")
	}

	// Test cancellation
	cancel()

	// Wait a bit to ensure cancellation is processed
	time.Sleep(20 * time.Millisecond)

	if !logger.HasMessage("Auto-refresh stopped") {
		t.Errorf("Expected 'Auto-refresh stopped' message in logger")
	}

	mu.Lock()
	cAfterCancel := fetchCount
	mu.Unlock()

	// Wait more and see if it fetches again
	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	cFinal := fetchCount
	mu.Unlock()

	if cFinal > cAfterCancel {
		t.Errorf("Auto-refresh continued after context cancellation (fetch count %d -> %d)", cAfterCancel, cFinal)
	}
}
