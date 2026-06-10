package e2e

import (
	"net/http"
	"testing"
)

// -- Tier 4: Real-World Scenarios --

func TestTier4_ValidUserBrowsingSafely(t *testing.T) {
	// A user browses normally, within rate limits, with correct credentials.
	proxyURL, cleanup := StartProxy(t, "-auth=user:pass", "-rate-limit=100")
	defer cleanup()

	ts := setupBackend(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	client := NewProxyClientWithAuth(proxyURL, "user", "pass")

	// Simulate page loads (HTML, CSS, JS)
	for i := 0; i < 5; i++ {
		resp, err := client.Get(ts.URL)
		if err != nil {
			t.Fatalf("Failed to load resource: %v", err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected 200, got %d", resp.StatusCode)
		}
	}
}

func TestTier4_AuthenticatedDDoSAttempt(t *testing.T) {
	// A malicious user has valid credentials but tries to overwhelm the proxy.
	// The rate limiter should catch them.
	proxyURL, cleanup := StartProxy(t, "-auth=user:pass", "-rate-limit=5")
	defer cleanup()

	ts := setupBackend(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	client := NewProxyClientWithAuth(proxyURL, "user", "pass")

	successCount := 0
	rateLimitedCount := 0

	for i := 0; i < 10; i++ {
		resp, _ := client.Get(ts.URL)
		if resp != nil {
			if resp.StatusCode == http.StatusOK {
				successCount++
			} else if resp.StatusCode == http.StatusTooManyRequests {
				rateLimitedCount++
			}
			resp.Body.Close()
		}
	}

	if rateLimitedCount == 0 {
		t.Errorf("Expected some requests to be rate limited during DDoS attempt")
	}
}

func TestTier4_UnauthenticatedDDoSAttempt(t *testing.T) {
	// An attacker tries to flood the proxy without valid credentials.
	// They should be rejected with 407 quickly.
	proxyURL, cleanup := StartProxy(t, "-auth=user:pass", "-rate-limit=5")
	defer cleanup()

	ts := setupBackend(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	client := NewProxyClient(proxyURL) // No auth provided

	for i := 0; i < 10; i++ {
		resp, _ := client.Get(ts.URL)
		if resp != nil {
			if resp.StatusCode != http.StatusProxyAuthRequired && resp.StatusCode != http.StatusTooManyRequests {
				t.Errorf("Expected 407 or 429, got %d", resp.StatusCode)
			}
			resp.Body.Close()
		}
	}
}

func TestTier4_BruteForceAuthAttempt(t *testing.T) {
	// An attacker tries many different passwords.
	proxyURL, cleanup := StartProxy(t, "-auth=user:pass", "-rate-limit=10")
	defer cleanup()

	ts := setupBackend(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	passwords := []string{"123456", "password", "admin", "12345678", "qwerty", "pass"}

	for _, pwd := range passwords {
		client := NewProxyClientWithAuth(proxyURL, "user", pwd)
		resp, _ := client.Get(ts.URL)
		if resp != nil {
			if pwd == "pass" {
				if resp.StatusCode != http.StatusOK {
					t.Errorf("Expected 200 for correct password, got %d", resp.StatusCode)
				}
			} else {
				if resp.StatusCode != http.StatusProxyAuthRequired {
					t.Errorf("Expected 407 for incorrect password '%s', got %d", pwd, resp.StatusCode)
				}
			}
			resp.Body.Close()
		}
	}
}

func TestTier4_MultiClientSharedNetwork(t *testing.T) {
	// Multiple users behind the same NAT (same IP) trying to access.
	// Since rate limiting is IP-based, they share the bucket.
	proxyURL, cleanup := StartProxy(t, "-auth=user:pass", "-rate-limit=4")
	defer cleanup()

	ts := setupBackend(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	client1 := NewProxyClientWithAuth(proxyURL, "user", "pass")
	client2 := NewProxyClientWithAuth(proxyURL, "user", "pass")

	// Client 1 uses 3 tokens
	for i := 0; i < 3; i++ {
		resp, _ := client1.Get(ts.URL)
		if resp != nil {
			resp.Body.Close()
		}
	}

	// Client 2 tries to use 2 tokens
	resp1, _ := client2.Get(ts.URL)
	if resp1 != nil {
		resp1.Body.Close()
	}

	resp2, _ := client2.Get(ts.URL)
	if resp2 != nil {
		defer resp2.Body.Close()
		// They share the IP limit, so this should fail
		if resp2.StatusCode != http.StatusTooManyRequests {
			t.Errorf("Expected 429 for shared IP limit exceeded, got %d", resp2.StatusCode)
		}
	}
}
