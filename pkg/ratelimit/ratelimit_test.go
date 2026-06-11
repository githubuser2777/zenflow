package ratelimit

import (
	"testing"
	"time"
)

func TestTokenBucketLimiter(t *testing.T) {
	// Rate: 10 req/sec, Burst: 5
	limiter := NewLimiter(10, 5)
	defer limiter.Stop()

	key := "192.168.1.1"

	// First 5 requests should pass (burst)
	for i := 0; i < 5; i++ {
		err := limiter.CheckRateLimit(key)
		if err != nil {
			t.Fatalf("Request %d failed unexpectedly: %v", i+1, err)
		}
	}

	// 6th request should fail immediately
	err := limiter.CheckRateLimit(key)
	if err != ErrTooManyRequests {
		t.Fatalf("Expected ErrTooManyRequests, got %v", err)
	}

	// Wait 100ms for 1 token to refill (rate is 10/sec, so 1 token every 100ms)
	time.Sleep(150 * time.Millisecond)

	// 1 request should pass now
	err = limiter.CheckRateLimit(key)
	if err != nil {
		t.Fatalf("Request after wait failed unexpectedly: %v", err)
	}

	// Next should fail again
	err = limiter.CheckRateLimit(key)
	if err != ErrTooManyRequests {
		t.Fatalf("Expected ErrTooManyRequests, got %v", err)
	}
}

func TestTokenBucketLimiter_DifferentIPs(t *testing.T) {
	limiter := NewLimiter(10, 1)
	defer limiter.Stop()

	key1 := "192.168.1.1"
	key2 := "10.0.0.1"

	// key1 passes
	if err := limiter.CheckRateLimit(key1); err != nil {
		t.Fatalf("key1 failed: %v", err)
	}

	// key1 fails
	if err := limiter.CheckRateLimit(key1); err != ErrTooManyRequests {
		t.Fatalf("key1 expected limit: %v", err)
	}

	// key2 passes (different IP)
	if err := limiter.CheckRateLimit(key2); err != nil {
		t.Fatalf("key2 failed: %v", err)
	}
}
