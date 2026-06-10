package config

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"zenflow/pkg/filter"
)

// Logger interface
type Logger interface {
	Printf(format string, v ...interface{})
}

// StartAutoRefresh periodically refreshes a remote blocklist URL and updates the blocker
func StartAutoRefresh(ctx context.Context, url string, b *filter.Blocker, interval time.Duration, logger Logger) {
	if logger == nil {
		logger = log.Default()
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				logger.Printf("[CONFIG] Auto-refresh stopped for %s", url)
				return
			case <-ticker.C:
				logger.Printf("[CONFIG] Auto-refreshing remote blocklist from %s...", url)
				domains, err := FetchFromURL(ctx, url)
				if err != nil {
					logger.Printf("[CONFIG] Error auto-refreshing blocklist from %s: %v", url, err)
					continue
				}
				b.UpdateBlocklist(domains)
				logger.Printf("[CONFIG] Blocklist auto-refreshed successfully from %s (%d domains loaded)", url, len(domains))
			}
		}
	}()
}

// FetchFromFile reads the blocklist from a local file
func FetchFromFile(filePath string) ([]string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	return ParseHostsFormat(file), nil
}

// FetchFromURL reads the blocklist from a remote URL
func FetchFromURL(ctx context.Context, url string) ([]string, error) {
	reqCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return ParseHostsFormat(resp.Body), nil
}

// WatchLocalFile starts a goroutine that watches a local file for changes
// using os.Stat polling and updates the blocker when it changes.
func WatchLocalFile(filePath string, b *filter.Blocker, pollInterval time.Duration) {
	go func() {
		var lastModTime time.Time

		// Initial load to get the first mod time if possible
		info, err := os.Stat(filePath)
		if err == nil {
			lastModTime = info.ModTime()
		}

		ticker := time.NewTicker(pollInterval)
		defer ticker.Stop()

		for range ticker.C {
			info, err := os.Stat(filePath)
			if err != nil {
				// File might have been deleted or not available, keep waiting
				continue
			}

			if info.ModTime().After(lastModTime) {
				log.Printf("[CONFIG] Detected change in %s, reloading blocklist...", filePath)
				domains, err := FetchFromFile(filePath)
				if err != nil {
					log.Printf("[CONFIG] Error reloading blocklist: %v", err)
					continue
				}

				b.UpdateBlocklist(domains)
				log.Printf("[CONFIG] Blocklist reloaded successfully (%d domains)", len(domains))
				lastModTime = info.ModTime()
			}
		}
	}()
}
