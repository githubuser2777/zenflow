package proxy

import (
	"fmt"
	"net/http"
	"testing"
	"sync"
)

func TestTotalSizeEnforcement(t *testing.T) {
	c := NewCacheWithLimit(10 * 1024 * 1024) // 10MB
	oneMB := make([]byte, 1024*1024)
	for i := 0; i < 200; i++ {
		key := fmt.Sprintf("key-%d", i)
		c.Set(key, http.Header{}, oneMB)
	}
	if c.totalSize > 10*1024*1024 {
		t.Errorf("totalSize %d exceeds limit 10MB", c.totalSize)
	}
	if len(c.store) > 10 {
		t.Errorf("store has %d entries, expected at most 10", len(c.store))
	}
}

func TestLRUEvictionOrder(t *testing.T) {
	c := NewCacheWithLimit(4000)

	oneKB := make([]byte, 1024)

	c.Set("A", http.Header{}, oneKB)
	c.Set("B", http.Header{}, oneKB)
	c.Set("C", http.Header{}, oneKB)

	// Access A — A becomes MRU
	if _, ok := c.Get("A"); !ok {
		t.Errorf("expected A to be in cache")
	}

	// Insert D — must evict LRU entry (B since A was accessed)
	c.Set("D", http.Header{}, oneKB)

	// B should be evicted
	if _, ok := c.Get("B"); ok {
		t.Errorf("expected B to be evicted")
	}

	// A, C, D should be present
	for _, key := range []string{"A", "C", "D"} {
		if _, ok := c.Get(key); !ok {
			t.Errorf("expected %s to be in cache", key)
		}
	}
}

func TestSingleItemExceedsTotalLimit(t *testing.T) {
	c := NewCacheWithLimit(1024) // 1KB total
	bigBody := make([]byte, 2048)
	c.Set("A", http.Header{}, bigBody)

	if _, ok := c.Get("A"); ok {
		t.Errorf("expected A not to be cached")
	}
	if c.totalSize != 0 {
		t.Errorf("expected totalSize 0, got %d", c.totalSize)
	}
}

func TestUpdateExistingKey(t *testing.T) {
	c := NewCacheWithLimit(5 * 1024) // 5KB
	oneKB := make([]byte, 1024)
	twoKB := make([]byte, 2048)

	c.Set("A", http.Header{}, oneKB)
	expectedOne := estimateItemSize("A", CachedResponse{Headers: http.Header{}, Body: oneKB})
	if c.totalSize != expectedOne {
		t.Errorf("expected totalSize %d, got %d", expectedOne, c.totalSize)
	}

	c.Set("A", http.Header{}, twoKB)
	expectedTwo := estimateItemSize("A", CachedResponse{Headers: http.Header{}, Body: twoKB})
	if c.totalSize != expectedTwo {
		t.Errorf("expected totalSize %d after update, got %d", expectedTwo, c.totalSize)
	}

	resp, ok := c.Get("A")
	if !ok || len(resp.Body) != 2048 {
		t.Errorf("expected body length 2048, got %d", len(resp.Body))
	}
}

func TestPerItemCapStillWorks(t *testing.T) {
	c := NewCacheWithLimit(100 * 1024 * 1024) // 100MB — plenty of room
	sixMB := make([]byte, 6*1024*1024)        // exceeds 5MB per-item cap
	c.Set("A", http.Header{}, sixMB)

	if _, ok := c.Get("A"); ok {
		t.Errorf("expected A not to be cached")
	}
}

func TestEmptyBodyCaching(t *testing.T) {
	c := NewCache()
	h := http.Header{}
	h.Set("Content-Type", "text/plain")
	c.Set("A", h, []byte{})

	resp, ok := c.Get("A")
	if !ok || len(resp.Body) != 0 {
		t.Errorf("expected empty body, got length %d", len(resp.Body))
	}
	if resp.Headers.Get("Content-Type") != "text/plain" {
		t.Errorf("expected Content-Type text/plain, got %s", resp.Headers.Get("Content-Type"))
	}
}

func TestNewCacheDefaultLimit(t *testing.T) {
	c := NewCache()
	expected := int64(100 * 1024 * 1024) // 100MB
	if c.maxTotalSize != expected {
		t.Errorf("expected default maxTotalSize %d, got %d", expected, c.maxTotalSize)
	}
}

func TestMemoryAccountingLargeHeaders(t *testing.T) {
	c := NewCacheWithLimit(5000) // 5000 bytes

	h := make(http.Header)
	h.Set("X-Large-Header", string(make([]byte, 6000)))

	c.Set("A", h, []byte{})

	if _, ok := c.Get("A"); ok {
		t.Errorf("expected A not to be cached")
	}
	if c.totalSize != 0 {
		t.Errorf("expected totalSize 0, got %d", c.totalSize)
	}
}

func TestConcurrency(t *testing.T) {
	c := NewCache()
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			c.Set(fmt.Sprintf("K%d", i), http.Header{}, []byte("test"))
			c.Get(fmt.Sprintf("K%d", i))
		}(i)
	}
	wg.Wait()
}
