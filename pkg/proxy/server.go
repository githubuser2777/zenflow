package proxy

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"zenflow/pkg/auth"
	"zenflow/pkg/filter"
	"zenflow/pkg/logger"
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

// Server represents the HTTP Proxy Server.
type Server struct {
	blocker        *filter.Blocker
	malwareBlocker *filter.Blocker
	cache          *Cache
	auth           auth.Authenticator
	ratelimit      ratelimit.Limiter
	proxy          *httputil.ReverseProxy
	privacy        bool
	rateLimitStr   string
}

// NewServer creates a new Server instance.
func NewServer(domainBlocker *filter.Blocker, malwareBlocker *filter.Blocker, privacy bool) *Server {
	rate := 50
	limitStr := os.Getenv("PROXY_RATE_LIMIT")
	if limitStr != "" {
		if r, err := strconv.Atoi(limitStr); err == nil {
			rate = r
		}
	} else {
		limitStr = "50"
	}
	s := &Server{
		blocker:        domainBlocker,
		malwareBlocker: malwareBlocker,
		cache:          NewCache(),
		auth:           auth.NewAuthenticator(),
		ratelimit:      ratelimit.NewLimiter(rate, rate),
		privacy:        privacy,
		rateLimitStr:   limitStr,
	}

	s.proxy = &httputil.ReverseProxy{
		Director:       s.rewriteRequest,
		ModifyResponse: s.interceptResponse,
		ErrorHandler:   s.handleProxyError,
	}

	return s
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
	ext := strings.ToLower(path[idx:])
	switch ext {
	case ".png", ".jpg", ".jpeg", ".gif", ".css", ".js", ".ico":
		return true
	}
	return false
}

type BlockDecision struct {
	Blocked    bool
	Reason     string
	FilterName string
}

func (s *Server) checkBlocking(r *http.Request) BlockDecision {
	if s.blocker != nil {
		if blocked, reason := s.blocker.IsBlocked(r.Host); blocked {
			return BlockDecision{Blocked: true, Reason: reason, FilterName: "proxy filter"}
		}
	}

	if s.malwareBlocker != nil {
		if blocked, reason := s.malwareBlocker.IsBlocked(r.Host); blocked {
			return BlockDecision{Blocked: true, Reason: "Malware: " + reason, FilterName: "malware filter"}
		}
	}
	return BlockDecision{Blocked: false}
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

func scrubCookies(cookies []string) []string {
	var newCookies []string
	for _, cookieStr := range cookies {
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



func shouldCache(resp *http.Response, isStatic bool) bool {
	if !isStatic || resp.Request.Method != http.MethodGet || resp.StatusCode != http.StatusOK {
		return false
	}
	hasAuthHeader := resp.Request.Header.Get("Authorization") != "" || resp.Request.Header.Get("Proxy-Authorization") != ""
	if hasAuthHeader {
		return false
	}
	cacheControl := strings.ToLower(resp.Header.Get("Cache-Control"))
	preventCache := strings.Contains(cacheControl, "private") || strings.Contains(cacheControl, "no-cache") || strings.Contains(cacheControl, "no-store") || strings.Contains(cacheControl, "max-age=0")
	return !preventCache
}

func getCacheKey(r *http.Request) string {
	return r.Host + r.URL.Path
}

func readBodyBytes(resp *http.Response) ([]byte, error) {
	buf := bodyBufferPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer func() {
		if buf.Cap() <= 64*1024 {
			bodyBufferPool.Put(buf)
		}
	}()

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



func (s *Server) validateRequest(w http.ResponseWriter, r *http.Request) bool {
	if err := s.ratelimit.CheckRateLimit(r); err != nil {
		http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
		return false
	}
	w.Header().Set("X-RateLimit-Limit", s.rateLimitStr)

	if s.auth != nil {
		if err := s.auth.CheckAuth(r); err != nil {
			w.Header().Set("Proxy-Authenticate", `Basic realm="Proxy"`)
			http.Error(w, "Proxy Authentication Required", http.StatusProxyAuthRequired)
			return false
		}
	}

	if decision := s.checkBlocking(r); decision.Blocked {
		logger.LogBlock(r.Method, r.Host, r.URL.String(), decision.Reason)
		http.Error(w, "Blocked by "+decision.FilterName, http.StatusForbidden)
		return false
	}

	logger.LogAllow(r.Method, r.Host, r.URL.String())
	return true
}

func (s *Server) serveFromCache(w http.ResponseWriter, r *http.Request) bool {
	hasAuthHeader := r.Header.Get("Authorization") != "" || r.Header.Get("Proxy-Authorization") != ""
	if r.Method != http.MethodGet || hasAuthHeader {
		return false
	}
	if strings.Contains(strings.ToLower(r.Header.Get("Cache-Control")), "no-cache") {
		return false
	}

	if resp, ok := s.cache.Get(getCacheKey(r)); ok {
		for k, vv := range resp.Headers {
			for _, v := range vv {
				w.Header().Add(k, v)
			}
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
	if !s.validateRequest(w, r) {
		return
	}

	if r.Method == http.MethodConnect {
		s.handleConnect(w, r)
		return
	}

	if isStaticAsset(r) && s.serveFromCache(w, r) {
		return
	}

	s.proxy.ServeHTTP(w, r)
}

func resolveConnectHost(host string) (string, error) {
	_, portStr, err := net.SplitHostPort(host)
	if err != nil {
		if strings.Contains(err.Error(), "missing port in address") {
			return host + ":443", nil
		}
		return "", err
	}
	if _, err := strconv.Atoi(portStr); err != nil {
		return "", err
	}
	return host, nil
}

func hijackConnection(w http.ResponseWriter) (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		return nil, nil, errors.New("hijacking not supported")
	}
	return hijacker.Hijack()
}

func (s *Server) handleConnect(w http.ResponseWriter, r *http.Request) {
	host, err := resolveConnectHost(r.Host)
	if err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	r.Host = host

	clientConn, rw, err := hijackConnection(w)
	if err != nil {
		if err.Error() == "hijacking not supported" {
			http.Error(w, "Hijacking not supported", http.StatusInternalServerError)
		} else {
			http.Error(w, err.Error(), http.StatusServiceUnavailable)
		}
		return
	}

	targetConn, err := net.DialTimeout("tcp", r.Host, 10*time.Second)
	if err != nil {
		io.WriteString(clientConn, "HTTP/1.1 503 Service Unavailable\r\n\r\n")
		clientConn.Close()
		return
	}

	io.WriteString(clientConn, "HTTP/1.1 200 Connection established\r\n\r\n")

	if buffered := rw.Reader.Buffered(); buffered > 0 {
		peeked, _ := rw.Reader.Peek(buffered)
		targetConn.Write(peeked)
	}

	go s.transfer(targetConn, clientConn)
	go s.transfer(clientConn, targetConn)
}

func (s *Server) transfer(destination io.WriteCloser, source io.ReadCloser) {
	defer destination.Close()
	defer source.Close()

	bufPtr := bufferPool.Get().(*[]byte)
	defer bufferPool.Put(bufPtr)

	io.CopyBuffer(destination, source, *bufPtr)
}
