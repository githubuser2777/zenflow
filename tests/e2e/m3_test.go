package e2e

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestM3_MalwareBlocklist(t *testing.T) {
	os.Unsetenv("PROXY_USER")
	os.Unsetenv("PROXY_PASS")
	os.Unsetenv("PROXY_RATE_LIMIT")

	malwarePath := WriteTempConfig(t, "0.0.0.0 malware.test\n")
	defer os.Remove(malwarePath)

	adsPath := WriteTempConfig(t, "0.0.0.0 ads.test\n")
	defer os.Remove(adsPath)

	proxyURL, cleanup := StartProxy(t, "-malware-blocklist="+malwarePath, "-blocklist="+adsPath)
	defer cleanup()

	// Normal backend
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer ts.Close()

	client := NewProxyClient(proxyURL)

	t.Run("Blocked by Malware Filter", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "http://malware.test/", nil)
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)

		if resp.StatusCode != 403 {
			t.Errorf("Expected 403, got %d", resp.StatusCode)
		}
		if !strings.Contains(string(body), "Blocked by malware filter") {
			t.Errorf("Expected malware filter message, got %s", body)
		}
	})

	t.Run("Blocked by Ad Filter", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "http://ads.test/", nil)
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)

		if resp.StatusCode != 403 {
			t.Errorf("Expected 403, got %d", resp.StatusCode)
		}
		if !strings.Contains(string(body), "Blocked by proxy filter") {
			t.Errorf("Expected ad filter message, got %s", body)
		}
	})

	t.Run("Normal Domain", func(t *testing.T) {
		req, _ := http.NewRequest("GET", ts.URL, nil)
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)

		if resp.StatusCode != 200 {
			t.Errorf("Expected 200, got %d", resp.StatusCode)
		}
		if string(body) != "OK" {
			t.Errorf("Expected body OK, got %s", string(body))
		}
	})
}
