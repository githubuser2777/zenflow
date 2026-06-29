package proxy

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestPrivacyHeaderScrubbing(t *testing.T) {
	os.Setenv("PROXY_USER", "admin")
	os.Setenv("PROXY_PASS", "admin")
	// Create a backend server to inspect headers
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check Referer
		if r.Header.Get("Referer") != "" {
			t.Errorf("Referer header not scrubbed, got: %s", r.Header.Get("Referer"))
		}
		// Check X-Forwarded-For
		if r.Header.Get("X-Forwarded-For") != "" {
			t.Errorf("X-Forwarded-For header not scrubbed, got: %s", r.Header.Get("X-Forwarded-For"))
		}
		// Check User-Agent
		if r.Header.Get("User-Agent") != "Mozilla/5.0" {
			t.Errorf("User-Agent header not properly set, got: %s", r.Header.Get("User-Agent"))
		}
		// Check Cookies
		cookieHeader := r.Header.Get("Cookie")
		if strings.Contains(cookieHeader, "_ga=") || strings.Contains(cookieHeader, "_gid=") || strings.Contains(cookieHeader, "_fbp=") || strings.Contains(cookieHeader, "_hj=") {
			t.Errorf("Tracking cookies not scrubbed, got: %s", cookieHeader)
		}
		if !strings.Contains(cookieHeader, "session_id=12345") {
			t.Errorf("Legitimate cookies missing, got: %s", cookieHeader)
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer backend.Close()

	// Create proxy server
	p := NewServer(nil, nil, true)
	proxySrv := httptest.NewServer(p)
	defer proxySrv.Close()

	// Create a client request directly to proxy server
	// We want the proxy to forward to the backend, so we need to set the Host
	req, err := http.NewRequest("GET", backend.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Referer", "http://example.com")
	req.Header.Set("X-Forwarded-For", "192.168.1.1")
	req.Header.Set("User-Agent", "TrackingAgent/1.0")
	// Multiple tracking and non-tracking cookies
	req.Header.Set("Cookie", "_ga=GA1.2.123456789; session_id=12345; _gid=GA1.2.987654321; _fbp=fb.1.123456789; custom=value; _hj=hjid123")

	// Ensure we test the reverse proxy routing properly
	// The Director in server.go uses req.URL.Scheme = "http" and req.URL.Host = r.Host
	// Let's send the request to proxySrv.URL but with the Host header of backend
	req2, _ := http.NewRequest("GET", proxySrv.URL, nil)
	req2.Host = strings.TrimPrefix(backend.URL, "http://")
	req2.Header = req.Header
	req2.SetBasicAuth("admin", "admin")
	// For Proxy Auth, we need Proxy-Authorization, not just Authorization. SetBasicAuth sets Authorization.
	// But let's just do it manually:
	req2.Header.Set("Proxy-Authorization", req2.Header.Get("Authorization"))

	client := &http.Client{}
	resp, err := client.Do(req2)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
}

func TestCookieScrubbingEdgeCases(t *testing.T) {
	os.Setenv("PROXY_USER", "admin")
	os.Setenv("PROXY_PASS", "admin")

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookieHeader := r.Header.Get("Cookie")
		if strings.Contains(cookieHeader, "_ga") {
			t.Errorf("Tracking cookie _ga not scrubbed, got: %s", cookieHeader)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	p := NewServer(nil, nil, true)
	proxySrv := httptest.NewServer(p)
	defer proxySrv.Close()

	req, _ := http.NewRequest("GET", proxySrv.URL, nil)
	req.Host = strings.TrimPrefix(backend.URL, "http://")
	// Test cookie with spaces before the equal sign
	req.Header.Set("Cookie", "_ga =bypassed; session=ok")
	req.SetBasicAuth("admin", "admin")
	req.Header.Set("Proxy-Authorization", req.Header.Get("Authorization"))

	client := &http.Client{}
	resp, _ := client.Do(req)
	resp.Body.Close()
}


func TestScrubCookiesOptimized(t *testing.T) {

	// No tracking cookies -> should return the exact same slice
	noTracking := []string{"session=123", "user=john; theme=dark"}
	res1 := scrubCookies(noTracking)
	if len(res1) != len(noTracking) || &res1[0] != &noTracking[0] {
		t.Errorf("expected no allocation or copying for no tracking cookies, got new slice pointer")
	}

	// Tracking cookies present -> should scrub and return new slice
	withTracking := []string{"session=123", "_ga=12345; user=john", "_hj=abc"}
	res2 := scrubCookies(withTracking)
	if len(res2) != 2 {
		t.Errorf("expected 2 cookies after scrubbing, got %d", len(res2))
	}
	if res2[0] != "session=123" || res2[1] != "user=john" {
		t.Errorf("unexpected scrubbed cookies: %v", res2)
	}
}
