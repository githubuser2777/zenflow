package e2e

import (
	"bytes"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

// -- Helpers --

func setupBackend(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)
	return ts
}

func setupHTTPSBackend(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	ts := httptest.NewTLSServer(handler)
	t.Cleanup(ts.Close)
	return ts
}

// -- Feature 1: HTTPS Tunneling (CONNECT) --

func TestTier1_HTTPS_Tunnel_Success(t *testing.T) {
	ts := setupHTTPSBackend(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("secure data"))
	})

	proxyURL, cleanup := StartProxy(t)
	defer cleanup()

	client := NewHTTPSProxyClient(proxyURL)
	resp, err := client.Get(ts.URL)
	if err != nil {
		t.Fatalf("Failed GET request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "secure data" {
		t.Errorf("Expected body 'secure data', got '%s'", string(body))
	}
}

func TestTier1_HTTPS_Tunnel_BadHost(t *testing.T) {
	proxyURL, cleanup := StartProxy(t)
	defer cleanup()

	client := NewHTTPSProxyClient(proxyURL)
	resp, err := client.Get("https://non-existent-domain.internal.test:443")
	if err != nil {
		// Some dialers return err on failed CONNECT, which is fine
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadGateway && resp.StatusCode != http.StatusGatewayTimeout {
		t.Errorf("Expected 502/504, got %d", resp.StatusCode)
	}
}

func TestTier1_HTTPS_Tunnel_DataTransfer(t *testing.T) {
	ts := setupHTTPSBackend(t, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		w.Write(body) // Echo back
	})

	proxyURL, cleanup := StartProxy(t)
	defer cleanup()

	client := NewHTTPSProxyClient(proxyURL)
	payload := []byte("bidirectional test data")
	resp, err := client.Post(ts.URL, "text/plain", bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("Failed POST request: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if !bytes.Equal(body, payload) {
		t.Errorf("Data integrity check failed. Expected %q, got %q", payload, body)
	}
}

func TestTier1_HTTPS_Tunnel_DefaultPort(t *testing.T) {
	proxyURL, cleanup := StartProxy(t)
	defer cleanup()

	proxyAddr := strings.TrimPrefix(proxyURL, "http://")
	conn, err := net.Dial("tcp", proxyAddr)
	if err != nil {
		t.Fatalf("Failed to dial proxy: %v", err)
	}
	defer conn.Close()

	reqStr := "CONNECT example.com HTTP/1.1\r\nHost: example.com\r\n\r\n"
	if _, err := conn.Write([]byte(reqStr)); err != nil {
		t.Fatalf("Failed to write to proxy: %v", err)
	}

	buf := make([]byte, 1024)
	n, err := conn.Read(buf)
	if err != nil {
		t.Fatalf("Failed to read from proxy: %v", err)
	}

	resp := string(buf[:n])
	if !strings.Contains(resp, "200 Connection established") {
		t.Errorf("Expected 200 Connection established, got: %s", resp)
	}
}

func TestTier1_HTTPS_Tunnel_Malformed(t *testing.T) {
	proxyURL, cleanup := StartProxy(t)
	defer cleanup()

	proxyAddr := strings.TrimPrefix(proxyURL, "http://")
	conn, err := net.Dial("tcp", proxyAddr)
	if err != nil {
		t.Fatalf("Failed to dial proxy: %v", err)
	}
	defer conn.Close()

	reqStr := "CONNECT example.com:invalidport HTTP/1.1\r\n\r\n"
	if _, err := conn.Write([]byte(reqStr)); err != nil {
		t.Fatalf("Failed to write to proxy: %v", err)
	}

	buf := make([]byte, 1024)
	n, err := conn.Read(buf)
	if err != nil && err != io.EOF {
		t.Fatalf("Failed to read from proxy: %v", err)
	}

	resp := string(buf[:n])
	if !strings.Contains(resp, "400 Bad Request") {
		t.Errorf("Expected 400 Bad Request, got: %s", resp)
	}
}

// -- Feature 2: Blocklist & Hot-reload --

func TestTier1_Blocklist_BlockedDomain(t *testing.T) {
	configPath := WriteTempConfig(t, "0.0.0.0 blocked.domain.test\n")
	proxyURL, cleanup := StartProxy(t, "-blocklist="+configPath)
	defer cleanup()

	client := NewProxyClient(proxyURL)
	resp, err := client.Get("http://blocked.domain.test")
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("Expected 403 Forbidden, got %d", resp.StatusCode)
	}
}

func TestTier1_Blocklist_AllowedDomain(t *testing.T) {
	configPath := WriteTempConfig(t, "blocked.com\n")
	proxyURL, cleanup := StartProxy(t, "-blocklist="+configPath)
	defer cleanup()

	ts := setupBackend(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	client := NewProxyClient(proxyURL)
	resp, err := client.Get(ts.URL)
	if err != nil {
		t.Fatalf("Failed GET: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected 200 OK, got %d", resp.StatusCode)
	}
}

func TestTier1_Blocklist_HotReload_Add(t *testing.T) {
	configPath := WriteTempConfig(t, "0.0.0.0 blocked.domain.test\n")
	proxyURL, cleanup := StartProxy(t, "-blocklist="+configPath)
	defer cleanup()

	client := NewProxyClient(proxyURL)
	
	// Overwrite config
	time.Sleep(1 * time.Second)
	tmpPath := configPath + ".tmp"
	os.WriteFile(tmpPath, []byte("0.0.0.0 blocked.domain.test\n0.0.0.0 newblocked.domain.test\n"), 0644)
	os.Remove(configPath)
	os.Rename(tmpPath, configPath)
	SendReloadSignal(t, proxyURL)

	resp, err := client.Get("http://newblocked.domain.test")
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("Expected 403 after hot reload, got %d", resp.StatusCode)
	}
}

func TestTier1_Blocklist_HotReload_Remove(t *testing.T) {
	configPath := WriteTempConfig(t, "0.0.0.0 blocked.domain.test\n")
	proxyURL, cleanup := StartProxy(t, "-blocklist="+configPath)
	defer cleanup()

	client := NewProxyClient(proxyURL)
	
	// Remove from config
	time.Sleep(1 * time.Second)
	tmpPath := configPath + ".tmp"
	os.WriteFile(tmpPath, []byte("0.0.0.0 dummy.test\n"), 0644)
	os.Remove(configPath)
	os.Rename(tmpPath, configPath)
	SendReloadSignal(t, proxyURL)

	// Since blocked.domain.test is removed, it might not resolve, but proxy shouldn't return 403
	resp, err := client.Get("http://blocked.domain.test")
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusForbidden {
		t.Errorf("Expected not 403 after domain removal, got %d", resp.StatusCode)
	}
}

func TestTier1_Blocklist_CaseInsensitive(t *testing.T) {
	configPath := WriteTempConfig(t, "0.0.0.0 BLOCKED.DOMAIN.TEST\n")
	proxyURL, cleanup := StartProxy(t, "-blocklist="+configPath)
	defer cleanup()

	client := NewProxyClient(proxyURL)
	resp, err := client.Get("http://BLOCKED.DOMAIN.TEST")
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("Expected 403, got %d", resp.StatusCode)
	}
}

// -- Feature 3: Static Caching --

func TestTier1_Cache_Miss(t *testing.T) {
	calls := 0
	ts := setupBackend(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Cache-Control", "public, max-age=60")
		w.Write([]byte("image data"))
	})

	proxyURL, cleanup := StartProxy(t)
	defer cleanup()

	client := NewProxyClient(proxyURL)
	resp, err := client.Get(ts.URL + "/image.jpg")
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.Header.Get("X-Cache") != "MISS" && resp.Header.Get("X-Cache") != "" {
		t.Errorf("Expected cache MISS")
	}
}

func TestTier1_Cache_Hit(t *testing.T) {
	calls := 0
	ts := setupBackend(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Cache-Control", "public, max-age=60")
		w.Write([]byte("image data"))
	})

	proxyURL, cleanup := StartProxy(t)
	defer cleanup()

	client := NewProxyClient(proxyURL)
	resp1, err := client.Get(ts.URL + "/image.jpg")
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	resp1.Body.Close()

	resp2, err := client.Get(ts.URL + "/image.jpg")
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp2.Body.Close()
	if resp2.Header.Get("X-Cache") != "HIT" && calls > 1 {
		t.Errorf("Expected cache HIT, but got MISS or no header")
	}
}

func TestTier1_Cache_Bypass_NonStatic(t *testing.T) {
	calls := 0
	ts := setupBackend(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Cache-Control", "public, max-age=60")
		w.Write([]byte(`{"status":"ok"}`))
	})

	proxyURL, cleanup := StartProxy(t)
	defer cleanup()

	client := NewProxyClient(proxyURL)
	req1, _ := http.NewRequest("POST", ts.URL+"/api", strings.NewReader(`{}`))
	resp1, err := client.Do(req1)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	resp1.Body.Close()

	req2, _ := http.NewRequest("POST", ts.URL+"/api", strings.NewReader(`{}`))
	resp2, err := client.Do(req2)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp2.Body.Close()
	if resp2.Header.Get("X-Cache") == "HIT" {
		t.Errorf("Expected POST requests not to be cached")
	}
}

func TestTier1_Cache_Expiration(t *testing.T) {
	ts := setupBackend(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=0") // Expire immediately
		w.Write([]byte("data"))
	})

	proxyURL, cleanup := StartProxy(t)
	defer cleanup()

	client := NewProxyClient(proxyURL)
	resp1, err := client.Get(ts.URL + "/image.jpg")
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	resp1.Body.Close()

	resp2, err := client.Get(ts.URL + "/image.jpg")
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp2.Body.Close()
	if resp2.Header.Get("X-Cache") == "HIT" {
		t.Errorf("Expected cache to expire and miss")
	}
}

func TestTier1_Cache_ClientBypass(t *testing.T) {
	ts := setupBackend(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=60")
		w.Write([]byte("data"))
	})

	proxyURL, cleanup := StartProxy(t)
	defer cleanup()

	client := NewProxyClient(proxyURL)
	resp1, err := client.Get(ts.URL + "/image.jpg")
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	resp1.Body.Close()

	req, _ := http.NewRequest("GET", ts.URL+"/image.jpg", nil)
	req.Header.Set("Cache-Control", "no-cache")
	resp2, err := client.Do(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp2.Body.Close()
	if resp2.Header.Get("X-Cache") == "HIT" {
		t.Errorf("Expected client Cache-Control: no-cache to bypass cache")
	}
}

// -- Feature 4: Rate Limiting --

func TestTier1_RateLimit_UnderLimit(t *testing.T) {
	proxyURL, cleanup := StartProxy(t, "-rate-limit=10")
	defer cleanup()

	ts := setupBackend(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	client := NewProxyClient(proxyURL)
	resp, err := client.Get(ts.URL)
	if err != nil { t.Fatalf("Request failed: %v", err) }
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected 200, got %d", resp.StatusCode)
		}
}

func TestTier1_RateLimit_ExceedLimit(t *testing.T) {
	proxyURL, cleanup := StartProxy(t, "-rate-limit=1")
	defer cleanup()

	ts := setupBackend(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	client := NewProxyClient(proxyURL)
	
	resp1, err := client.Get(ts.URL)
	if err != nil { t.Fatalf("Request failed: %v", err) }
	resp1.Body.Close()

	resp2, err := client.Get(ts.URL)
	if err != nil { t.Fatalf("Request failed: %v", err) }
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusTooManyRequests {
		t.Errorf("Expected 429 Too Many Requests, got %d", resp2.StatusCode)
	}
}

func TestTier1_RateLimit_WindowReset(t *testing.T) {
	proxyURL, cleanup := StartProxy(t, "-rate-limit=1")
	defer cleanup()

	ts := setupBackend(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	client := NewProxyClient(proxyURL)
	
	resp1, err := client.Get(ts.URL)
	if err != nil { t.Fatalf("Request failed: %v", err) }
	resp1.Body.Close()

	resp2, err := client.Get(ts.URL)
	if err != nil { t.Fatalf("Request failed: %v", err) }
	if resp2.StatusCode != http.StatusTooManyRequests {
		t.Errorf("Expected 429 Too Many Requests, got %d", resp2.StatusCode)
	}
	resp2.Body.Close()

	time.Sleep(1500 * time.Millisecond)

	resp3, err := client.Get(ts.URL)
	if err != nil { t.Fatalf("Request failed: %v", err) }
	defer resp3.Body.Close()
	if resp3.StatusCode != http.StatusOK {
		t.Errorf("Expected 200 OK after window reset, got %d", resp3.StatusCode)
	}
}

func TestTier1_RateLimit_IndependentIPs(t *testing.T) {
	proxyURL, cleanup := StartProxy(t, "-rate-limit=1")
	defer cleanup()

	ts := setupBackend(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	client1 := NewProxyClient(proxyURL)
	req1, _ := http.NewRequest("GET", ts.URL, nil)
	req1.Header.Set("X-Forwarded-For", "192.168.1.1")
	resp1, err := client1.Do(req1)
	if err != nil { t.Fatalf("Request failed: %v", err) }
	resp1.Body.Close()

	client2 := NewProxyClient(proxyURL)
	req2, _ := http.NewRequest("GET", ts.URL, nil)
	req2.Header.Set("X-Forwarded-For", "192.168.1.2")
	resp2, err := client2.Do(req2)
	if err != nil { t.Fatalf("Request failed: %v", err) }
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Errorf("Expected 200 OK for independent IP, got %d", resp2.StatusCode)
	}
}

func TestTier1_RateLimit_Headers(t *testing.T) {
	proxyURL, cleanup := StartProxy(t, "-rate-limit=10")
	defer cleanup()

	ts := setupBackend(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	client := NewProxyClient(proxyURL)
	resp, err := client.Get(ts.URL)
	if err != nil { t.Fatalf("Request failed: %v", err) }
	defer resp.Body.Close()
	if resp.Header.Get("X-RateLimit-Limit") == "" {
			t.Errorf("Expected rate limit headers in response")
		}
}

// -- Feature 5: Proxy Auth (Basic Auth) --

func TestTier1_ProxyAuth_MissingHeader(t *testing.T) {
	proxyURL, cleanup := StartProxy(t, "-auth=user:pass")
	defer cleanup()
	client := NewProxyClient(proxyURL)
	resp, err := client.Get("http://example.com")
	if err != nil { t.Fatalf("Request failed: %v", err) }
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusProxyAuthRequired {
			t.Errorf("Expected 407 Proxy Authentication Required, got %d", resp.StatusCode)
		}
}

func TestTier1_ProxyAuth_ValidAuth(t *testing.T) {
	proxyURL, cleanup := StartProxy(t, "-auth=user:pass")
	defer cleanup()
	client := NewProxyClientWithAuth(proxyURL, "user", "pass")
	
	ts := setupBackend(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	
	resp, err := client.Get(ts.URL)
	if err != nil { t.Fatalf("Request failed: %v", err) }
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected 200, got %d", resp.StatusCode)
		}
}

func TestTier1_ProxyAuth_InvalidAuth(t *testing.T) {
	proxyURL, cleanup := StartProxy(t, "-auth=user:pass")
	defer cleanup()
	client := NewProxyClientWithAuth(proxyURL, "user", "wrongpass")
	resp, err := client.Get("http://example.com")
	if err != nil { t.Fatalf("Request failed: %v", err) }
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusProxyAuthRequired {
			t.Errorf("Expected 407 Proxy Authentication Required, got %d", resp.StatusCode)
		}
}

func TestTier1_ProxyAuth_HeaderStripped(t *testing.T) {
	proxyURL, cleanup := StartProxy(t, "-auth=user:pass")
	defer cleanup()
	client := NewProxyClientWithAuth(proxyURL, "user", "pass")
	
	ts := setupBackend(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Proxy-Authorization") != "" {
			t.Errorf("Proxy-Authorization header was not stripped")
		}
		w.WriteHeader(http.StatusOK)
	})
	
	resp, err := client.Get(ts.URL)
	if err != nil { t.Fatalf("Request failed: %v", err) }
	resp.Body.Close()
}

func TestTier1_ProxyAuth_ConnectCompat(t *testing.T) {
	proxyURL, cleanup := StartProxy(t, "-auth=user:pass")
	defer cleanup()
	
	ts := setupHTTPSBackend(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	client := NewProxyClientWithAuth(proxyURL, "user", "pass")
	resp, err := client.Get(ts.URL)
	if err != nil { t.Fatalf("Request failed: %v", err) }
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected 200 for CONNECT with auth, got %d", resp.StatusCode)
		}
}

// -- Feature 6: Privacy Header Stripping --

func TestTier1_PrivacyStrip_UserAgent(t *testing.T) {
	proxyURL, cleanup := StartProxy(t)
	defer cleanup()
	client := NewProxyClient(proxyURL)
	
	ts := setupBackend(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") != "Mozilla/5.0" {
			t.Errorf("User-Agent header was not stripped to Mozilla/5.0")
		}
		w.WriteHeader(http.StatusOK)
	})
	
	req, _ := http.NewRequest("GET", ts.URL, nil)
	req.Header.Set("User-Agent", "Test-Agent")
	resp, err := client.Do(req)
	if err != nil { t.Fatalf("Request failed: %v", err) }
	resp.Body.Close()
}

func TestTier1_PrivacyStrip_Referer(t *testing.T) {
	proxyURL, cleanup := StartProxy(t)
	defer cleanup()
	client := NewProxyClient(proxyURL)
	
	ts := setupBackend(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Referer") != "" {
			t.Errorf("Referer header was not stripped")
		}
		w.WriteHeader(http.StatusOK)
	})
	
	req, _ := http.NewRequest("GET", ts.URL, nil)
	req.Header.Set("Referer", "http://example.com")
	resp, err := client.Do(req)
	if err != nil { t.Fatalf("Request failed: %v", err) }
	resp.Body.Close()
}

func TestTier1_PrivacyStrip_NoIPLeak(t *testing.T) {
	proxyURL, cleanup := StartProxy(t)
	defer cleanup()
	client := NewProxyClient(proxyURL)
	
	ts := setupBackend(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Forwarded-For") != "" {
			t.Errorf("IP leak: X-Forwarded-For header was present")
		}
		w.WriteHeader(http.StatusOK)
	})
	
	resp, err := client.Get(ts.URL)
	if err != nil { t.Fatalf("Request failed: %v", err) }
	resp.Body.Close()
}

func TestTier1_PrivacyStrip_UnlistedIntact(t *testing.T) {
	proxyURL, cleanup := StartProxy(t)
	defer cleanup()
	client := NewProxyClient(proxyURL)
	
	ts := setupBackend(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Accept-Language") != "en-US" {
			t.Errorf("Accept-Language header was modified or stripped")
		}
		w.WriteHeader(http.StatusOK)
	})
	
	req, _ := http.NewRequest("GET", ts.URL, nil)
	req.Header.Set("Accept-Language", "en-US")
	resp, err := client.Do(req)
	if err != nil { t.Fatalf("Request failed: %v", err) }
	resp.Body.Close()
}

func TestTier1_PrivacyStrip_ToggleOff(t *testing.T) {
	proxyURL, cleanup := StartProxy(t, "-privacy=false")
	defer cleanup()
	client := NewProxyClient(proxyURL)
	
	ts := setupBackend(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") != "Test-Agent" {
			t.Errorf("User-Agent was modified when privacy was disabled")
		}
		w.WriteHeader(http.StatusOK)
	})
	
	req, _ := http.NewRequest("GET", ts.URL, nil)
	req.Header.Set("User-Agent", "Test-Agent")
	resp, err := client.Do(req)
	if err != nil { t.Fatalf("Request failed: %v", err) }
	resp.Body.Close()
}

// -- Feature 7: Malware Blocklist --

func TestTier1_Malware_Blocked(t *testing.T) {
	tsList := setupBackend(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("0.0.0.0 malware.domain.test\n"))
	})
	proxyURL, cleanup := StartProxy(t, "-malware-blocklist="+tsList.URL)
	defer cleanup()
	client := NewProxyClient(proxyURL)
	
	resp, err := client.Get("http://malware.domain.test")
	if err != nil { t.Fatalf("Request failed: %v", err) }
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
			t.Errorf("Expected 403, got %d", resp.StatusCode)
		}
}

func TestTier1_Malware_Clean(t *testing.T) {
	tsList := setupBackend(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("0.0.0.0 malware.domain.test\n"))
	})
	proxyURL, cleanup := StartProxy(t, "-malware-blocklist="+tsList.URL)
	defer cleanup()
	
	ts := setupBackend(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	client := NewProxyClient(proxyURL)
	resp, err := client.Get(ts.URL)
	if err != nil { t.Fatalf("Request failed: %v", err) }
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected 200 for clean domain, got %d", resp.StatusCode)
		}
}

func TestTier1_Malware_BlockPageBody(t *testing.T) {
	tsList := setupBackend(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("0.0.0.0 malware.domain.test\n"))
	})
	proxyURL, cleanup := StartProxy(t, "-malware-blocklist="+tsList.URL)
	defer cleanup()
	client := NewProxyClient(proxyURL)
	
	resp, err := client.Get("http://malware.domain.test")
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(strings.ToLower(string(body)), "malware") {
		t.Errorf("Expected malware block warning in body")
	}
}

func TestTier1_Malware_SubdomainBlock(t *testing.T) {
	tsList := setupBackend(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("0.0.0.0 malware.domain.test\n"))
	})
	proxyURL, cleanup := StartProxy(t, "-malware-blocklist="+tsList.URL)
	defer cleanup()
	client := NewProxyClient(proxyURL)
	
	resp, err := client.Get("http://sub.malware.domain.test")
	if err != nil { t.Fatalf("Request failed: %v", err) }
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
			t.Errorf("Expected 403 for malware subdomain, got %d", resp.StatusCode)
		}
}

func TestTier1_Malware_NetworkResilience(t *testing.T) {
	requestCount := 0
	tsList := setupBackend(t, func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		if requestCount > 1 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Write([]byte("0.0.0.0 malware.domain.test\n"))
	})
	proxyURL, cleanup := StartProxy(t, "-malware-blocklist="+tsList.URL)
	defer cleanup()

	client := NewProxyClient(proxyURL)

	// Trigger an auto-refresh after the proxy is started
	// Wait a bit to ensure it doesn't crash on failure
	time.Sleep(1 * time.Second)

	// Since we can't easily wait 24h, let's just make sure the initial list is still present 
	// and the proxy is still running and blocking
	resp, err := client.Get("http://malware.domain.test")
	if err != nil { t.Fatalf("Request failed: %v", err) }
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("Expected 403 for malware domain after network error, got %d", resp.StatusCode)
	}
}
