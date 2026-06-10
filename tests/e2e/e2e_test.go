package e2e

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var proxyBinPath string
var proxyTempDir string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "zenflow-e2e-*")
	if err != nil {
		fmt.Printf("Failed to create temp dir: %v\n", err)
		os.Exit(1)
	}
	proxyTempDir = dir
	binPath := filepath.Join(dir, "proxy.exe")

	cmd := exec.Command("go", "build", "-o", binPath, "../../cmd/proxy")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		fmt.Printf("Failed to build proxy binary: %v\n", err)
		os.RemoveAll(proxyTempDir)
		os.Exit(1)
	}

	proxyBinPath = binPath

	code := m.Run()

	os.RemoveAll(proxyTempDir)
	os.Exit(code)
}

// BuildProxyBinary returns the path to the pre-built proxy executable.
func BuildProxyBinary(t *testing.T) string {
	if proxyBinPath == "" {
		t.Fatalf("proxyBinPath is empty, TestMain did not run correctly")
	}
	return proxyBinPath
}

// StartProxy starts the proxy binary on a random available port and returns its URL.
func StartProxy(t *testing.T, extraArgs ...string) (proxyURL string, cleanup func()) {
	t.Helper()

	binPath := BuildProxyBinary(t)

	// Get a free port
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to listen on a port: %v", err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()

	args := []string{fmt.Sprintf("-port=%d", port), "-no-tui=true"}
	args = append(args, extraArgs...)

	cmd := exec.Command(binPath, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	var newEnv []string
	for _, env := range os.Environ() {
		if !strings.HasPrefix(env, "PROXY_RATE_LIMIT=") && !strings.HasPrefix(env, "PROXY_USER=") && !strings.HasPrefix(env, "PROXY_PASS=") {
			newEnv = append(newEnv, env)
		}
	}
	cmd.Env = newEnv

	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start proxy: %v", err)
	}

	proxyURL = fmt.Sprintf("http://127.0.0.1:%d", port)

	// Wait for the proxy to be ready
	if err := WaitForProxy(proxyURL); err != nil {
		cmd.Process.Kill()
		t.Fatalf("Proxy failed to start: %v", err)
	}

	cleanup = func() {
		cmd.Process.Kill()
		cmd.Wait()
	}

	return proxyURL, cleanup
}

// WaitForProxy waits until the proxy is accepting connections.
func WaitForProxy(proxyURL string) error {
	u, err := url.Parse(proxyURL)
	if err != nil {
		return err
	}
	
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", u.Host, 100*time.Millisecond)
		if err == nil {
			conn.Close()
			// Sleep a tiny bit to ensure the HTTP server is fully ready to process requests
			// after the listener accepts
			time.Sleep(50 * time.Millisecond)
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}

	return fmt.Errorf("proxy did not start in time")
}

// NewProxyClient creates an HTTP client configured to use the given proxy.
func NewProxyClient(proxyURL string) *http.Client {
	pURL, _ := url.Parse(proxyURL)
	transport := &http.Transport{
		Proxy: http.ProxyURL(pURL),
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	return &http.Client{
		Transport: transport,
		Timeout:   10 * time.Second,
	}
}

// NewProxyClientWithAuth creates a client with basic auth for the proxy.
func NewProxyClientWithAuth(proxyURL, username, password string) *http.Client {
	pURL, _ := url.Parse(proxyURL)
	pURL.User = url.UserPassword(username, password)
	transport := &http.Transport{
		Proxy: http.ProxyURL(pURL),
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	return &http.Client{
		Transport: transport,
		Timeout:   10 * time.Second,
	}
}

// NewHTTPSProxyClient creates a client tailored for HTTPS tunneling tests.
// Note: standard http.Client supports CONNECT natively through http.ProxyURL for HTTPS URLs.
func NewHTTPSProxyClient(proxyURL string) *http.Client {
	return NewProxyClient(proxyURL)
}

// WriteTempConfig writes a string to a temporary file and returns its path.
func WriteTempConfig(t *testing.T, content string) string {
	t.Helper()
	file, err := os.CreateTemp("", "proxy-config-*.txt")
	if err != nil {
		t.Fatalf("Failed to create temp config: %v", err)
	}
	defer file.Close()

	if _, err := file.WriteString(content); err != nil {
		t.Fatalf("Failed to write temp config: %v", err)
	}

	return file.Name()
}

// SendReloadSignal triggers a hot reload. The implementation may vary.
// We use a 6 second sleep because config.WatchLocalFile polls every 5 seconds.
func SendReloadSignal(t *testing.T, proxyURL string) {
	t.Helper()
	time.Sleep(6 * time.Second)
}

// BasicAuthHeader creates a basic auth header value.
func BasicAuthHeader(username, password string) string {
	// Standard http library handles base64 encoding
	req, _ := http.NewRequest("GET", "/", nil)
	req.SetBasicAuth(username, password)
	return req.Header.Get("Authorization")
}
