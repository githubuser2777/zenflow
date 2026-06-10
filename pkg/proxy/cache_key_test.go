package proxy

import (
	"net/http"
	"testing"
)

func TestCacheKeyAdversarial(t *testing.T) {
	req1, _ := http.NewRequest("GET", "http://example.com/image.png?v=1", nil)
	req2, _ := http.NewRequest("GET", "http://example.com/image.png?v=2", nil)

	key1 := getCacheKey(req1)
	key2 := getCacheKey(req2)

	if key1 == key2 {
		t.Errorf("Cache keys should be different for different query params: %s == %s", key1, key2)
	}
}
