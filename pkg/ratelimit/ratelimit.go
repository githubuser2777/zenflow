package ratelimit

import (
	"errors"
	"sync"
	"time"

	"zenflow/pkg/netutil"
)

var ErrTooManyRequests = errors.New("too many requests")

// Limiter defines the rate limiter interface.
type Limiter interface {
	CheckRateLimit(key string) error
}

type bucket struct {
	mu         sync.Mutex
	tokens     float64
	lastRefill time.Time
	lastSeen   time.Time
}

type limiterShard struct {
	mu      sync.RWMutex
	buckets map[string]*bucket
}

type TokenBucketLimiter struct {
	rate      int
	burst     int
	shards    [32]*limiterShard
	stopClean chan struct{}
}

// NewLimiter creates a new sharded token bucket rate limiter.
func NewLimiter(rate, burst int) *TokenBucketLimiter {
	l := &TokenBucketLimiter{
		rate:      rate,
		burst:     burst,
		stopClean: make(chan struct{}),
	}
	for i := 0; i < 32; i++ {
		l.shards[i] = &limiterShard{
			buckets: make(map[string]*bucket),
		}
	}
	go l.cleanup()
	return l
}

func getShardIndex(key string) uint32 {
	var hash uint32 = 2166136261
	for i := 0; i < len(key); i++ {
		hash ^= uint32(key[i])
		hash *= 16777619
	}
	return hash % 32
}

func canonicalizeIP(rawIP string) string {
	return netutil.CanonicalizeIP(rawIP)
}

// CheckRateLimit checks the rate limit for the given key (e.g. client IP).
func (l *TokenBucketLimiter) CheckRateLimit(key string) error {
	shardIdx := getShardIndex(key)
	shard := l.shards[shardIdx]

	now := time.Now()

	shard.mu.RLock()
	b, ok := shard.buckets[key]
	shard.mu.RUnlock()

	if !ok {
		shard.mu.Lock()
		b, ok = shard.buckets[key]
		if !ok {
			b = &bucket{
				tokens:     float64(l.burst),
				lastRefill: now,
				lastSeen:   now,
			}
			shard.buckets[key] = b
		}
		shard.mu.Unlock()
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	elapsed := now.Sub(b.lastRefill).Seconds()
	newTokens := elapsed * float64(l.rate)
	if newTokens > 0 {
		b.tokens += newTokens
		b.lastRefill = now
	}
	b.lastSeen = now

	if b.tokens >= 1.0 {
		b.tokens -= 1.0
		if b.tokens > float64(l.burst) {
			b.tokens = float64(l.burst)
		}
		return nil
	}

	if b.tokens > float64(l.burst) {
		b.tokens = float64(l.burst)
	}
	return ErrTooManyRequests
}

func (l *TokenBucketLimiter) cleanup() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			now := time.Now()
			for _, s := range l.shards {
				s.mu.Lock()
				for key, b := range s.buckets {
					b.mu.Lock()
					lastSeen := b.lastSeen
					b.mu.Unlock()

					if now.Sub(lastSeen) > 5*time.Minute {
						delete(s.buckets, key)
					}
				}
				s.mu.Unlock()
			}
		case <-l.stopClean:
			return
		}
	}
}

// Stop stops the background cleanup goroutine.
func (l *TokenBucketLimiter) Stop() {
	close(l.stopClean)
}
