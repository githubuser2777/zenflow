package tier5

import (
	"encoding/base64"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"zenflow/pkg/auth"
	"zenflow/pkg/ratelimit"
)

// Vulnerability 1: Insecure Default / Empty Credentials Bypass
// If PROXY_USER and PROXY_PASS are not set, the proxy is completely open.
func TestAuth_EmptyCredentialsBypass(t *testing.T) {
	os.Setenv("PROXY_USER", "")
	os.Setenv("PROXY_PASS", "")

	authValidator := auth.NewAuthenticator()

	req := httptest.NewRequest("GET", "http://example.com", nil)

	err := authValidator.CheckAuth(req)
	if err == nil {
		t.Fatalf("Expected auth to fail with ErrUnauthorized, got success!")
	} else if err != auth.ErrUnauthorized {
		t.Fatalf("Expected ErrUnauthorized, got error: %v", err)
	}
	t.Logf("FIXED: Empty credentials now default to secure deny-all")
}

// Vulnerability 2: SHA256 CPU Exhaustion DoS
// The authenticator hashes the user input before checking length.
// Large payloads can cause CPU exhaustion.
func TestAuth_LargePayloadDoS(t *testing.T) {
	os.Setenv("PROXY_USER", "admin")
	os.Setenv("PROXY_PASS", "password")
	defer os.Unsetenv("PROXY_USER")
	defer os.Unsetenv("PROXY_PASS")

	authValidator := auth.NewAuthenticator()

	// 1MB payload
	largeUser := strings.Repeat("A", 1024*1024)
	payload := base64.StdEncoding.EncodeToString([]byte(largeUser + ":password"))

	req := httptest.NewRequest("GET", "http://example.com", nil)
	req.Header.Set("Proxy-Authorization", "Basic "+payload)

	start := time.Now()
	err := authValidator.CheckAuth(req)
	duration := time.Since(start)

	if err != auth.ErrUnauthorized {
		t.Fatalf("Expected ErrUnauthorized, got %v", err)
	}

	t.Logf("Hashing 1MB took: %v", duration)
	if duration < 10*time.Millisecond {
		t.Logf("FIXED: Fast rejection before hashing prevents CPU DoS")
	} else {
		t.Fatalf("Processing still took too long: %v", duration)
	}
}

// Vulnerability 3: IP Spoofing / Load Balancer Exhaustion
// The rate limiter ignores X-Forwarded-For, leading to shared limits
// for all clients behind a NAT or Load Balancer.
func TestRateLimit_LoadBalancerExhaustion(t *testing.T) {
	limiter := ratelimit.NewLimiter(1, 1)
	defer limiter.Stop()

	// Client A behind LB
	req1 := httptest.NewRequest("GET", "http://example.com", nil)
	req1.RemoteAddr = "10.0.0.1:1234" // LB IP
	req1.Header.Set("X-Forwarded-For", "192.168.1.100")

	if err := limiter.CheckRateLimit(req1); err != nil {
		t.Fatalf("Client A request failed: %v", err)
	}

	// Client B behind LB
	req2 := httptest.NewRequest("GET", "http://example.com", nil)
	req2.RemoteAddr = "10.0.0.1:5678" // LB IP
	req2.Header.Set("X-Forwarded-For", "192.168.1.200")

	err := limiter.CheckRateLimit(req2)
	if err != nil {
		t.Fatalf("Expected success for Client B (different X-Forwarded-For), got: %v", err)
	}
	t.Logf("FIXED: Rate limiter respects X-Forwarded-For")
}

// Vulnerability 4: Dual-Stack IPv6 Bypass
// Attacker uses IPv4 and IPv6 to bypass IP-based rate limiting.
func TestRateLimit_IPv6Bypass(t *testing.T) {
	limiter := ratelimit.NewLimiter(1, 1)
	defer limiter.Stop()

	// Attacker connects via IPv4
	req1 := httptest.NewRequest("GET", "http://example.com", nil)
	req1.RemoteAddr = "127.0.0.1:1234"

	if err := limiter.CheckRateLimit(req1); err != nil {
		t.Fatalf("IPv4 request failed: %v", err)
	}

	// Attacker connects via IPv6
	req2 := httptest.NewRequest("GET", "http://example.com", nil)
	req2.RemoteAddr = "[::1]:5678"

	err := limiter.CheckRateLimit(req2)
	if err == nil {
		t.Fatalf("Expected IPv6 bypass to FAIL, but it succeeded!")
	} else if err != ratelimit.ErrTooManyRequests {
		t.Fatalf("Expected ErrTooManyRequests, got: %v", err)
	}
	t.Logf("FIXED: Rate limiter canonicalizes IPv6/IPv4")
}

// Vulnerability 5: Fractional Token Discard (Precision Loss)
// If rate is 1 token/sec, and requests come every 1.9s, the 0.9s is discarded.
// Rate becomes strictly lower than configured.
func TestRateLimit_FractionalTokenDiscard(t *testing.T) {
	limiter := ratelimit.NewLimiter(1, 1)
	defer limiter.Stop()

	req := httptest.NewRequest("GET", "http://example.com", nil)
	req.RemoteAddr = "127.0.0.1:1234"

	// Exhaust token
	if err := limiter.CheckRateLimit(req); err != nil {
		t.Fatalf("Initial request failed: %v", err)
	}

	// Wait 1.1 seconds (1 full token generated, 0.1 discarded)
	time.Sleep(1100 * time.Millisecond)

	// Consume token
	if err := limiter.CheckRateLimit(req); err != nil {
		t.Fatalf("Second request failed: %v", err)
	}

	// Wait another 0.95 seconds. Total time since first token generation = 1.1 + 0.95 = 2.05s
	// If fractional tokens were kept, we would have 0.1 + 0.95 = 1.05 token now.
	time.Sleep(950 * time.Millisecond)

	err := limiter.CheckRateLimit(req)
	if err != nil {
		t.Fatalf("Expected rate limit success (fractional tokens kept), but it failed: %v", err)
	}
	t.Logf("FIXED: Fractional tokens are retained")
}
