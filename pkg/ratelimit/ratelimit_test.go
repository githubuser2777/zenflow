package ratelimit

import (
	"net/http"
	"testing"
	"time"
)

func TestTokenBucketLimiter(t *testing.T) {
	// Rate: 10 req/sec, Burst: 5
	limiter := NewLimiter(10, 5)
	defer limiter.Stop()

	req, _ := http.NewRequest("GET", "http://example.com", nil)
	req.RemoteAddr = "192.168.1.1:12345"

	// First 5 requests should pass (burst)
	for i := 0; i < 5; i++ {
		err := limiter.CheckRateLimit(req)
		if err != nil {
			t.Fatalf("Request %d failed unexpectedly: %v", i+1, err)
		}
	}

	// 6th request should fail immediately
	err := limiter.CheckRateLimit(req)
	if err != ErrTooManyRequests {
		t.Fatalf("Expected ErrTooManyRequests, got %v", err)
	}

	// Wait 100ms for 1 token to refill (rate is 10/sec, so 1 token every 100ms)
	time.Sleep(150 * time.Millisecond)

	// 1 request should pass now
	err = limiter.CheckRateLimit(req)
	if err != nil {
		t.Fatalf("Request after wait failed unexpectedly: %v", err)
	}

	// Next should fail again
	err = limiter.CheckRateLimit(req)
	if err != ErrTooManyRequests {
		t.Fatalf("Expected ErrTooManyRequests, got %v", err)
	}
}

func TestTokenBucketLimiter_DifferentIPs(t *testing.T) {
	limiter := NewLimiter(10, 1)
	defer limiter.Stop()

	req1, _ := http.NewRequest("GET", "http://example.com", nil)
	req1.RemoteAddr = "192.168.1.1:12345"

	req2, _ := http.NewRequest("GET", "http://example.com", nil)
	req2.RemoteAddr = "10.0.0.1:54321"

	// req1 passes
	if err := limiter.CheckRateLimit(req1); err != nil {
		t.Fatalf("req1 failed: %v", err)
	}

	// req1 fails
	if err := limiter.CheckRateLimit(req1); err != ErrTooManyRequests {
		t.Fatalf("req1 expected limit: %v", err)
	}

	// req2 passes (different IP)
	if err := limiter.CheckRateLimit(req2); err != nil {
		t.Fatalf("req2 failed: %v", err)
	}
}
