package proxy

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"runtime"
	"testing"
)

func TestPoolDiscardingLargeBuffers(t *testing.T) {
	server := NewCoreServer(false)
	largeData := bytes.Repeat([]byte("A"), 4*1024*1024)
	req := httptest.NewRequest(http.MethodGet, "http://example.com/large.png", nil)
	resp := &http.Response{
		StatusCode:    http.StatusOK,
		Request:       req,
		Header:        make(http.Header),
		Body:          io.NopCloser(bytes.NewReader(largeData)),
		ContentLength: int64(len(largeData)),
	}
	resp.Header.Set("Cache-Control", "public")

	var m1, m2 runtime.MemStats
	runtime.ReadMemStats(&m1)
	err := server.cacheResponse(resp)
	if err != nil {
		t.Fatalf("cacheResponse failed: %v", err)
	}
	runtime.ReadMemStats(&m2)

	buf := bodyBufferPool.Get().(*bytes.Buffer)
	fmt.Printf("Buffer capacity after Get: %d\n", buf.Cap())
	fmt.Printf("Allocations: %d\n", m2.Mallocs-m1.Mallocs)

	if buf.Cap() == 32*1024 {
		t.Errorf("Buffer was NOT put back into the pool! A new 32KB buffer was created instead.")
	}
}

func BenchmarkLargeFilePooling(b *testing.B) {
	server := NewCoreServer(false)
	largeData := bytes.Repeat([]byte("A"), 4*1024*1024)
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

func BenchmarkChunkedBypass(b *testing.B) {
	server := NewCoreServer(false)
	largeData := bytes.Repeat([]byte("A"), 4*1024*1024)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodGet, "http://example.com/large.png", nil)
		resp := &http.Response{
			StatusCode:    http.StatusOK,
			Request:       req,
			Header:        make(http.Header),
			Body:          io.NopCloser(bytes.NewReader(largeData)),
			ContentLength: -1,
		}
		server.cacheResponse(resp)
	}
}

func TestCacheKeyIdenticalLookup(t *testing.T) {
	server := NewCoreServer(false)
	req1 := httptest.NewRequest(http.MethodGet, "http://example.com/test.png", nil)

	outReq := req1.Clone(req1.Context())

	resp := &http.Response{
		StatusCode:    http.StatusOK,
		Request:       outReq,
		Header:        make(http.Header),
		Body:          io.NopCloser(bytes.NewReader([]byte("test"))),
		ContentLength: 4,
	}
	resp.Header.Set("Cache-Control", "public")

	err := server.cacheResponse(resp)
	if err != nil {
		t.Fatalf("cacheResponse failed: %v", err)
	}

	w := httptest.NewRecorder()
	hit := server.serveFromCache(w, req1)
	if !hit {
		t.Errorf("Cache miss! Expected the cache key lookup to behave identically between cacheResponse(outReq) and serveFromCache(req1)")
	}
}
