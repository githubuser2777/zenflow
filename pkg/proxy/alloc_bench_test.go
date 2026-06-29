package proxy

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func BenchmarkIsStaticAsset_Allocations(b *testing.B) {
	req := httptest.NewRequest(http.MethodGet, "http://example.com/images/test_image_for_caching.PNG", nil)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = isStaticAsset(req)
	}
}



func BenchmarkServeFromCache_Allocations(b *testing.B) {
	server := NewCoreServer(false)
	req := httptest.NewRequest(http.MethodGet, "http://example.com/test.png", nil)
	
	// Pre-populate cache
	resp := &http.Response{
		StatusCode:    http.StatusOK,
		Request:       req,
		Header:        make(http.Header),
		Body:          http.NoBody,
		ContentLength: 0,
	}
	server.cacheResponse(resp)

	w := httptest.NewRecorder()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Just to prevent response body accumulation in the recorder if any
		w.Body.Reset() 
		server.serveFromCache(w, req)
	}
}
