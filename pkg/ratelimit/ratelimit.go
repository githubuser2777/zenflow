package ratelimit

import (
	"errors"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

var ErrTooManyRequests = errors.New("too many requests")

type Limiter interface {
	CheckRateLimit(r *http.Request) error
}

type bucket struct {
	mu         sync.Mutex
	tokens     float64
	lastRefill time.Time
	lastSeen   time.Time
}

type TokenBucketLimiter struct {
	rate      int
	burst     int
	buckets   sync.Map
	stopClean chan struct{}
}

func NewLimiter(rate, burst int) *TokenBucketLimiter {
	l := &TokenBucketLimiter{
		rate:      rate,
		burst:     burst,
		stopClean: make(chan struct{}),
	}
	go l.cleanup()
	return l
}

func extractRawIP(r *http.Request) string {
	ipStr := r.Header.Get("X-Forwarded-For")
	if ipStr == "" {
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			return r.RemoteAddr
		}
		return host
	}

	if firstIP, _, ok := strings.Cut(ipStr, ","); ok {
		return strings.TrimSpace(firstIP)
	}
	return strings.TrimSpace(ipStr)
}

func canonicalizeIP(rawIP string) string {
	ipStrClean := rawIP
	if strings.HasPrefix(ipStrClean, "[") && strings.HasSuffix(ipStrClean, "]") {
		ipStrClean = ipStrClean[1 : len(ipStrClean)-1]
	}

	ip := net.ParseIP(ipStrClean)
	if ip == nil {
		return rawIP
	}

	if ip.Equal(net.IPv6loopback) {
		return "127.0.0.1"
	}
	if ipv4 := ip.To4(); ipv4 != nil {
		return ipv4.String()
	}
	return "[" + ip.String() + "]"
}

func extractIP(r *http.Request) string {
	return canonicalizeIP(extractRawIP(r))
}

func (l *TokenBucketLimiter) CheckRateLimit(r *http.Request) error {
	host := extractIP(r)

	now := time.Now()
	var b *bucket
	if val, ok := l.buckets.Load(host); ok {
		b = val.(*bucket)
	} else {
		val, _ = l.buckets.LoadOrStore(host, &bucket{
			tokens:     float64(l.burst),
			lastRefill: now,
			lastSeen:   now,
		})
		b = val.(*bucket)
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
			l.buckets.Range(func(key, value any) bool {
				b := value.(*bucket)
				b.mu.Lock()
				lastSeen := b.lastSeen
				b.mu.Unlock()

				if now.Sub(lastSeen) > 5*time.Minute {
					l.buckets.Delete(key)
				}
				return true
			})
		case <-l.stopClean:
			return
		}
	}
}

func (l *TokenBucketLimiter) Stop() {
	close(l.stopClean)
}
