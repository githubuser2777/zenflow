package config

import (
	"context"
	"net/http"
	"net/http/httptest"
	"runtime"
	"testing"
	"time"

	"zenflow/pkg/filter"
)

func TestStartAutoRefresh_ContextCancellation_Hangs(t *testing.T) {
	// Create a test server that hangs forever when processing the request
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Block until client disconnects or request context is cancelled
		<-r.Context().Done()
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	b := filter.NewBlocker(nil)
	logger := &mockLogger{} // use the existing mockLogger

	// Track number of goroutines before starting
	initialGoroutines := runtime.NumGoroutine()

	// Start auto refresh with a very short interval
	StartAutoRefresh(ctx, srv.URL, b, 10*time.Millisecond, logger)

	// Wait enough time for the ticker to fire and start the HTTP request
	time.Sleep(100 * time.Millisecond)

	// Now cancel the context
	cancel()

	// Wait for the cancellation to take effect
	time.Sleep(100 * time.Millisecond)

	// Check if the goroutine terminated
	finalGoroutines := runtime.NumGoroutine()

	// A successful cancellation should mean we return to the initial number of goroutines
	// or close to it, but specifically the one running StartAutoRefresh should exit.
	if !logger.HasMessage("Auto-refresh stopped") {
		t.Errorf("FAIL: Context cancellation did not result in 'Auto-refresh stopped' log message. The goroutine is likely leaked/stuck.")
	}

	if finalGoroutines > initialGoroutines {
		t.Logf("Warning: Goroutines leaked. Initial: %d, Final: %d", initialGoroutines, finalGoroutines)
	}
}
