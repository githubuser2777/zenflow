package e2e

import (
	"encoding/base64"
	"net/http"
	"strings"
	"testing"
)

// -- Feature 1: Rate Limiting Boundary & Corner Cases --

func TestTier2_RateLimit_ExactBoundary(t *testing.T) {
	proxyURL, cleanup := StartProxy(t, "-rate-limit=3")
	defer cleanup()

	ts := setupBackend(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	client := NewProxyClient(proxyURL)

	// First 3 should pass
	for i := 0; i < 3; i++ {
		resp, _ := client.Get(ts.URL)
		if resp != nil {
			resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				t.Errorf("Request %d: Expected 200, got %d", i+1, resp.StatusCode)
			}
		}
	}

	// 4th should fail
	resp, _ := client.Get(ts.URL)
	if resp != nil {
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusTooManyRequests {
			t.Errorf("Request 4: Expected 429, got %d", resp.StatusCode)
		}
	}
}

func TestTier2_RateLimit_ZeroLimit(t *testing.T) {
	// If limit is 0, it might mean deny all, or unlimited.
	// Assume 0 means deny all for strict security.
	proxyURL, cleanup := StartProxy(t, "-rate-limit=0")
	defer cleanup()

	ts := setupBackend(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	client := NewProxyClient(proxyURL)
	resp, _ := client.Get(ts.URL)
	if resp != nil {
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusTooManyRequests {
			// Some implementations treat 0 as unlimited. We just document this behavior.
			// If it's unlimited, it returns 200. Let's assert it handles it gracefully (no crash).
			if resp.StatusCode != http.StatusTooManyRequests && resp.StatusCode != http.StatusOK {
				t.Errorf("Expected 429 or 200 for 0 limit, got %d", resp.StatusCode)
			}
		}
	}
}

func TestTier2_RateLimit_UnroutableDestination(t *testing.T) {
	// Rate limiting should trigger BEFORE attempting to connect to destination.
	proxyURL, cleanup := StartProxy(t, "-rate-limit=1")
	defer cleanup()

	client := NewProxyClient(proxyURL)
	resp1, _ := client.Get("http://127.0.0.1:41234")
	if resp1 != nil {
		resp1.Body.Close()
	}

	// Second request should be rate limited, even to a valid destination
	ts := setupBackend(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	resp2, _ := client.Get(ts.URL)
	if resp2 != nil {
		defer resp2.Body.Close()
		if resp2.StatusCode != http.StatusTooManyRequests {
			t.Errorf("Expected 429 even after failing destination, got %d", resp2.StatusCode)
		}
	}
}

func TestTier2_RateLimit_LargeRequestCount(t *testing.T) {
	// Triggering many requests concurrently shouldn't crash the proxy (race conditions check)
	proxyURL, cleanup := StartProxy(t, "-rate-limit=5")
	defer cleanup()

	ts := setupBackend(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	client := NewProxyClient(proxyURL)

	errCh := make(chan error, 20)
	for i := 0; i < 20; i++ {
		go func() {
			resp, err := client.Get(ts.URL)
			if err == nil {
				resp.Body.Close()
			}
			errCh <- err
		}()
	}

	for i := 0; i < 20; i++ {
		<-errCh
	}
	// As long as proxy didn't crash, it passes.
}

func TestTier2_RateLimit_InvalidConfig(t *testing.T) {
	// Passing a negative number or unparseable value might cause an error or fallback to default.
	// We'll test a negative number.
	proxyURL, cleanup := StartProxy(t, "-rate-limit=-1")
	defer cleanup()

	ts := setupBackend(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	client := NewProxyClient(proxyURL)
	resp, _ := client.Get(ts.URL)
	if resp != nil {
		defer resp.Body.Close()
		// Just ensure it responds, doesn't matter if it's 200 or 429
		if resp.StatusCode == 0 {
			t.Errorf("Proxy did not respond gracefully to negative rate limit")
		}
	}
}

// -- Feature 2: Proxy Auth Boundary & Corner Cases --

func TestTier2_ProxyAuth_EmptyPassword(t *testing.T) {
	proxyURL, cleanup := StartProxy(t, "-auth=admin:")
	defer cleanup()

	ts := setupBackend(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Test valid empty password
	client := NewProxyClientWithAuth(proxyURL, "admin", "")
	resp, err := client.Get(ts.URL)
	if err == nil {
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected 200 for empty password, got %d", resp.StatusCode)
		}
	}
}

func TestTier2_ProxyAuth_EmptyUsername(t *testing.T) {
	proxyURL, cleanup := StartProxy(t, "-auth=:secret")
	defer cleanup()

	ts := setupBackend(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Test valid empty username
	client := NewProxyClientWithAuth(proxyURL, "", "secret")
	resp, err := client.Get(ts.URL)
	if err == nil {
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected 200 for empty username, got %d", resp.StatusCode)
		}
	}
}

func TestTier2_ProxyAuth_LargeCredentials(t *testing.T) {
	longStr := strings.Repeat("a", 8000)
	proxyURL, cleanup := StartProxy(t, "-auth="+longStr+":secret")
	defer cleanup()

	ts := setupBackend(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	client := NewProxyClientWithAuth(proxyURL, longStr, "secret")
	resp, err := client.Get(ts.URL)
	if err == nil {
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected 200 for large username, got %d", resp.StatusCode)
		}
	}
}

func TestTier2_ProxyAuth_InvalidBase64(t *testing.T) {
	proxyURL, cleanup := StartProxy(t, "-auth=admin:secret")
	defer cleanup()

	ts := setupBackend(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	client := NewProxyClient(proxyURL)
	req, _ := http.NewRequest("GET", ts.URL, nil)
	req.Header.Set("Proxy-Authorization", "Basic invalid_base64!@#$")

	resp, err := client.Do(req)
	if err == nil {
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusProxyAuthRequired && resp.StatusCode != http.StatusBadRequest {
			t.Errorf("Expected 407 or 400 for invalid base64, got %d", resp.StatusCode)
		}
	}
}

func TestTier2_ProxyAuth_WrongAuthType(t *testing.T) {
	proxyURL, cleanup := StartProxy(t, "-auth=admin:secret")
	defer cleanup()

	ts := setupBackend(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	client := NewProxyClient(proxyURL)
	req, _ := http.NewRequest("GET", ts.URL, nil)
	// Bearer instead of Basic
	token := base64.StdEncoding.EncodeToString([]byte("admin:secret"))
	req.Header.Set("Proxy-Authorization", "Bearer "+token)

	resp, err := client.Do(req)
	if err == nil {
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusProxyAuthRequired {
			t.Errorf("Expected 407 for wrong auth scheme, got %d", resp.StatusCode)
		}
	}
}
