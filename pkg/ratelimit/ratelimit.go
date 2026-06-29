package ratelimit

import (
	"errors"
	"sync"
	"time"

	"golang.org/x/time/rate"
	"zenflow/pkg/netutil"
)

var ErrTooManyRequests = errors.New("too many requests")

// TokenBucketLimiter uses x/time/rate for rate limiting.
type TokenBucketLimiter struct {
	rate      int
	burst     int
	mu        sync.Mutex
	limiters  map[string]*limiterEntry
	stopClean chan struct{}
}

type limiterEntry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// NewLimiter creates a new rate limiter.
func NewLimiter(r, burst int) *TokenBucketLimiter {
	l := &TokenBucketLimiter{
		rate:      r,
		burst:     burst,
		limiters:  make(map[string]*limiterEntry),
		stopClean: make(chan struct{}),
	}
	go l.cleanup()
	return l
}

func canonicalizeIP(rawIP string) string {
	return netutil.CanonicalizeIP(rawIP)
}

// CheckRateLimit checks the rate limit for the given key (e.g. client IP).
func (l *TokenBucketLimiter) CheckRateLimit(key string) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	entry, ok := l.limiters[key]
	if !ok {
		entry = &limiterEntry{
			limiter: rate.NewLimiter(rate.Limit(l.rate), l.burst),
		}
		l.limiters[key] = entry
	}
	entry.lastSeen = time.Now()

	if !entry.limiter.Allow() {
		return ErrTooManyRequests
	}
	return nil
}

func (l *TokenBucketLimiter) cleanup() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			now := time.Now()
			l.mu.Lock()
			for k, entry := range l.limiters {
				if now.Sub(entry.lastSeen) > 5*time.Minute {
					delete(l.limiters, k)
				}
			}
			l.mu.Unlock()
		case <-l.stopClean:
			return
		}
	}
}

// Stop stops the background cleanup goroutine.
func (l *TokenBucketLimiter) Stop() {
	close(l.stopClean)
}
