package proxy

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCacheKeyIdentity(t *testing.T) {
	server := NewCoreServer(false)

	// Mock upstream server
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public")
		w.Write([]byte("hello from upstream"))
	}))
	defer upstream.Close()

	// Make request to proxy
	req := httptest.NewRequest(http.MethodGet, upstream.URL+"/image.png", nil)
	// Add some custom host header to simulate client forging
	req.Host = "fake-host.com"

	w := httptest.NewRecorder()
	server.ServeHTTP(w, req)

	// Check if it was cached
	key := getCacheKey(req)
	_, ok := server.cache.Get(key)
	if !ok {
		t.Errorf("Cache key lookup failed! Expected cache hit for key %v, but got miss. Cache state: %+v", key, server.cache.store)
	}

	// Make another request to test serveFromCache
	req2 := httptest.NewRequest(http.MethodGet, upstream.URL+"/image.png", nil)
	req2.Host = "fake-host.com"

	w2 := httptest.NewRecorder()
	server.ServeHTTP(w2, req2)

	if w2.Header().Get("X-Cache") != "HIT" {
		t.Errorf("Expected cache HIT, got %s", w2.Header().Get("X-Cache"))
	}
}

func BenchmarkLargeFilePoolingEmpirical(b *testing.B) {
	server := NewCoreServer(false)
	largeData := bytes.Repeat([]byte("A"), 4*1024*1024) // 4MB

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodGet, "http://example.com/large.png", nil)
		resp := &http.Response{
			StatusCode:    http.StatusOK,
			Request:       req,
			Header:        make(http.Header),
			Body:          io.NopCloser(bytes.NewReader(largeData)),
			ContentLength: int64(len(largeData)),
		}
		server.cacheResponse(resp)
	}
}
