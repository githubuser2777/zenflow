package e2e

import (
	"net/http"
	"testing"
)

// -- Tier 3: Cross-Feature Interactions --

func TestTier3_RateLimitFirstThenAuth(t *testing.T) {
	// Requirements state: Rate limit must be checked BEFORE Auth.
	// Therefore, spamming unauthenticated requests will result in 429.
	proxyURL, cleanup := StartProxy(t, "-auth=admin:secret", "-rate-limit=1")
	defer cleanup()

	ts := setupBackend(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// 1. Spam invalid auth. First gets 407, subsequent get 429.
	clientInvalid := NewProxyClientWithAuth(proxyURL, "admin", "wrong")
	resp1, err := clientInvalid.Get(ts.URL)
	if err == nil {
		resp1.Body.Close()
		if resp1.StatusCode != http.StatusProxyAuthRequired {
			t.Errorf("Expected 407 for first invalid auth, got %d", resp1.StatusCode)
		}
	}

	for i := 0; i < 2; i++ {
		resp, err := clientInvalid.Get(ts.URL)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode != http.StatusTooManyRequests {
				t.Errorf("Expected 429 for spammed invalid auth, got %d", resp.StatusCode)
			}
		}
	}
}

func TestTier3_ValidAuthSpam_Returns429(t *testing.T) {
	proxyURL, cleanup := StartProxy(t, "-auth=admin:secret", "-rate-limit=2")
	defer cleanup()

	ts := setupBackend(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	client := NewProxyClientWithAuth(proxyURL, "admin", "secret")

	resp1, _ := client.Get(ts.URL)
	if resp1 != nil {
		resp1.Body.Close()
	}

	resp2, _ := client.Get(ts.URL)
	if resp2 != nil {
		resp2.Body.Close()
	}

	resp3, _ := client.Get(ts.URL)
	if resp3 != nil {
		defer resp3.Body.Close()
		if resp3.StatusCode != http.StatusTooManyRequests {
			t.Errorf("Expected 429 when rate limit exceeded with valid auth, got %d", resp3.StatusCode)
		}
	}
}
