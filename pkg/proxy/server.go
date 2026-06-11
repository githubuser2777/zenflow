package proxy

import (
	"bytes"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"os"
	"strconv"
	"strings"
	"sync"

	"zenflow/pkg/auth"
	"zenflow/pkg/filter"
	"zenflow/pkg/ratelimit"
)

var (
	bufferPool = sync.Pool{
		New: func() interface{} {
			buf := make([]byte, 32*1024)
			return &buf
		},
	}

	bodyBufferPool = sync.Pool{
		New: func() interface{} {
			return bytes.NewBuffer(make([]byte, 0, 32*1024))
		},
	}
)

// Server represents the core HTTP Proxy logic without middlewares.
type Server struct {
	cache   *Cache
	proxy   *httputil.ReverseProxy
	privacy bool
}

// NewCoreServer creates the un-wrapped Server for tests or manual composition.
func NewCoreServer(privacy bool) *Server {
	s := &Server{
		cache:   NewCache(),
		privacy: privacy,
	}
	s.proxy = &httputil.ReverseProxy{
		Director:       s.rewriteRequest,
		ModifyResponse: s.interceptResponse,
		ErrorHandler:   s.handleProxyError,
	}
	return s
}

// NewServer creates a new HTTP handler that chains Middlewares around the core Server.
func NewServer(domainBlocker *filter.Blocker, malwareBlocker *filter.Blocker, privacy bool) http.Handler {
	rate := 50
	limitStr := os.Getenv("PROXY_RATE_LIMIT")
	if limitStr != "" {
		if r, err := strconv.Atoi(limitStr); err == nil {
			rate = r
		}
	} else {
		limitStr = "50"
	}

	s := NewCoreServer(privacy)

	return Chain(s,
		RateLimitMiddleware(ratelimit.NewLimiter(rate, rate), limitStr),
		AuthMiddleware(auth.NewAuthenticator()),
		BlockerMiddleware(domainBlocker, malwareBlocker),
	)
}

func (s *Server) rewriteRequest(r *http.Request) {
	if r.URL.Scheme == "" {
		r.URL.Scheme = "http"
	}
	if r.URL.Host == "" && r.Host != "" {
		r.URL.Host = r.Host
	}
	r.Header.Del("Proxy-Connection")
	if s.privacy {
		r.Header.Del("Referer")
		r.Header["X-Forwarded-For"] = nil
		r.Header.Set("User-Agent", "Mozilla/5.0")
	}

	if cookies, ok := r.Header["Cookie"]; ok {
		newCookies := scrubCookies(cookies)
		if len(newCookies) > 0 {
			r.Header["Cookie"] = newCookies
		} else {
			r.Header.Del("Cookie")
		}
	}
}

func (s *Server) interceptResponse(resp *http.Response) error {
	isStatic := isStaticAsset(resp.Request)
	if shouldCache(resp, isStatic) {
		return s.cacheResponse(resp)
	}
	return nil
}

func (s *Server) handleProxyError(w http.ResponseWriter, r *http.Request, err error) {
	log.Printf("[PROXY ERROR] Failed to proxy %s: %v", r.URL.String(), err)
	http.Error(w, "Proxy Error: Bad Gateway", http.StatusBadGateway)
}

func isStaticAsset(r *http.Request) bool {
	path := r.URL.Path
	idx := strings.LastIndexByte(path, '.')
	if idx < 0 {
		return false
	}
	ext := path[idx:]
	if strings.EqualFold(ext, ".png") ||
		strings.EqualFold(ext, ".jpg") ||
		strings.EqualFold(ext, ".jpeg") ||
		strings.EqualFold(ext, ".gif") ||
		strings.EqualFold(ext, ".css") ||
		strings.EqualFold(ext, ".js") ||
		strings.EqualFold(ext, ".ico") {
		return true
	}
	return false
}

var trackingCookies = []string{"_ga", "_gid", "_fbp", "_hj"}

func isTrackingCookie(name string) bool {
	for _, tc := range trackingCookies {
		if strings.HasPrefix(name, tc) {
			return true
		}
	}
	return false
}

func hasTrackingCookie(cookieStr string) bool {
	remaining := cookieStr
	for len(remaining) > 0 {
		var part string
		part, remaining, _ = strings.Cut(remaining, ";")
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		name, _, _ := strings.Cut(trimmed, "=")
		name = strings.TrimSpace(name)
		if isTrackingCookie(name) {
			return true
		}
	}
	return false
}

func scrubCookies(cookies []string) []string {
	var newCookies []string
	for _, cookieStr := range cookies {
		if !hasTrackingCookie(cookieStr) {
			newCookies = append(newCookies, cookieStr)
			continue
		}

		var builder strings.Builder
		first := true
		remaining := cookieStr
		for len(remaining) > 0 {
			var part string
			part, remaining, _ = strings.Cut(remaining, ";")
			trimmed := strings.TrimSpace(part)
			if trimmed == "" {
				continue
			}

			name, _, _ := strings.Cut(trimmed, "=")
			name = strings.TrimSpace(name)

			if !isTrackingCookie(name) {
				if !first {
					builder.WriteString("; ")
				}
				builder.WriteString(trimmed)
				first = false
			}
		}

		if builder.Len() > 0 {
			newCookies = append(newCookies, builder.String())
		}
	}
	return newCookies
}

func containsIgnoreCase(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	if len(substr) > len(s) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		match := true
		for j := 0; j < len(substr); j++ {
			c1 := s[i+j]
			c2 := substr[j]
			if c1 >= 'A' && c1 <= 'Z' {
				c1 += 'a' - 'A'
			}
			if c2 >= 'A' && c2 <= 'Z' {
				c2 += 'a' - 'A'
			}
			if c1 != c2 {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

func shouldCache(resp *http.Response, isStatic bool) bool {
	if !isStatic || resp.Request.Method != http.MethodGet || resp.StatusCode != http.StatusOK {
		return false
	}
	hasAuthHeader := resp.Request.Header.Get("Authorization") != "" || resp.Request.Header.Get("Proxy-Authorization") != ""
	if hasAuthHeader {
		return false
	}
	cacheControl := resp.Header.Get("Cache-Control")
	preventCache := containsIgnoreCase(cacheControl, "private") || containsIgnoreCase(cacheControl, "no-cache") || containsIgnoreCase(cacheControl, "no-store") || containsIgnoreCase(cacheControl, "max-age=0")
	return !preventCache
}

func getCacheKey(r *http.Request) cacheKey {
	return cacheKey{host: r.Host, path: r.URL.Path, query: r.URL.RawQuery}
}

func readBodyBytes(resp *http.Response) ([]byte, error) {
	buf := bodyBufferPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer func() {
		if buf.Cap() <= 2*maxCacheSize {
			bodyBufferPool.Put(buf)
		}
	}()

	if resp.ContentLength > 0 && resp.ContentLength <= int64(maxCacheSize) {
		buf.Grow(int(resp.ContentLength))
	}

	_, err := buf.ReadFrom(io.LimitReader(resp.Body, maxCacheSize+1))
	if err != nil {
		return nil, err
	}

	bodyBytes := make([]byte, buf.Len())
	copy(bodyBytes, buf.Bytes())
	return bodyBytes, nil
}

func (s *Server) cacheResponse(resp *http.Response) error {
	if resp.ContentLength > int64(maxCacheSize) {
		return nil
	}

	bodyBytes, err := readBodyBytes(resp)
	if err != nil {
		return err
	}

	if len(bodyBytes) <= maxCacheSize {
		resp.Body.Close()
		s.cache.Set(getCacheKey(resp.Request), resp.Header, bodyBytes)
		resp.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	} else {
		resp.Body = struct {
			io.Reader
			io.Closer
		}{
			io.MultiReader(bytes.NewReader(bodyBytes), resp.Body),
			resp.Body,
		}
	}
	resp.Header.Set("X-Cache", "MISS")
	return nil
}

func (s *Server) serveFromCache(w http.ResponseWriter, r *http.Request) bool {
	hasAuthHeader := r.Header.Get("Authorization") != "" || r.Header.Get("Proxy-Authorization") != ""
	if r.Method != http.MethodGet || hasAuthHeader {
		return false
	}
	if containsIgnoreCase(r.Header.Get("Cache-Control"), "no-cache") {
		return false
	}

	if resp, ok := s.cache.Get(getCacheKey(r)); ok {
		for _, field := range resp.Headers {
			w.Header().Add(field.Key, field.Value)
		}
		w.Header().Set("X-Cache", "HIT")
		w.WriteHeader(http.StatusOK)
		w.Write(resp.Body)
		return true
	}
	return false
}

// ServeHTTP handles the incoming HTTP requests.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodConnect {
		handleConnect(w, r)
		return
	}

	if isStaticAsset(r) && s.serveFromCache(w, r) {
		return
	}

	s.proxy.ServeHTTP(w, r)
}
