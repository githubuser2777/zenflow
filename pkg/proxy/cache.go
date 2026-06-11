package proxy

import (
	"container/list"
	"net/http"
	"os"
	"strconv"
	"sync"
)

// HeaderField stores a single HTTP header key-value pair.
type HeaderField struct {
	Key   string
	Value string
}

// CachedResponse stores HTTP headers and body bytes for a cached response.
type CachedResponse struct {
	Headers []HeaderField
	Body    []byte
}

const maxCacheSize = 5 * 1024 * 1024 // 5MB per-item cap — DO NOT CHANGE

type cacheKey struct {
	host  string
	path  string
	query string
}

type cacheEntry struct {
	key  cacheKey
	resp CachedResponse
	size int64 // estimateItemSize — used for totalSize accounting
}

// Cache is a memory-bounded LRU cache.
type Cache struct {
	mu           sync.RWMutex
	store        map[cacheKey]*list.Element
	ll           *list.List
	totalSize    int64
	maxTotalSize int64
}

const defaultMaxTotalSize = 100 * 1024 * 1024 // 100MB

// NewCache creates a new Cache, reading PROXY_CACHE_SIZE_MB from the environment.
// It defaults to 100MB if unset or invalid.
func NewCache() *Cache {
	limit := int64(defaultMaxTotalSize)
	if str := os.Getenv("PROXY_CACHE_SIZE_MB"); str != "" {
		if mb, err := strconv.Atoi(str); err == nil {
			limit = int64(mb) * 1024 * 1024
		}
	}
	return NewCacheWithLimit(limit)
}

// NewCacheWithLimit creates a new Cache with the specified total size limit in bytes.
func NewCacheWithLimit(maxBytes int64) *Cache {
	return &Cache{
		store:        make(map[cacheKey]*list.Element),
		ll:           list.New(),
		maxTotalSize: maxBytes,
	}
}

func (c *Cache) Get(key cacheKey) (CachedResponse, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if elem, ok := c.store[key]; ok {
		c.ll.MoveToFront(elem)
		return elem.Value.(*cacheEntry).resp, true
	}
	return CachedResponse{}, false
}

const (
	stringOverheadBytes = 16
	sliceOverheadBytes  = 24
	entryOverheadBytes  = 100
)

func estimateItemSize(key cacheKey, resp CachedResponse) int64 {
	size := int64(stringOverheadBytes*3+len(key.host)+len(key.path)+len(key.query)) + int64(len(resp.Body))
	size += sliceOverheadBytes
	for _, field := range resp.Headers {
		size += int64(stringOverheadBytes*2 + len(field.Key) + len(field.Value))
	}
	size += entryOverheadBytes
	return size
}

// Set stores a response in the cache. Items exceeding the per-item cap or total
// limit are silently dropped. Existing entries for the same key are replaced.
// LRU entries are evicted as needed to make room.
func (c *Cache) Set(key cacheKey, headers http.Header, body []byte) {
	var h []HeaderField
	for k, vv := range headers {
		for _, v := range vv {
			h = append(h, HeaderField{Key: k, Value: v})
		}
	}

	itemSize := estimateItemSize(key, CachedResponse{Headers: h, Body: body})
	// Per-item cap
	if itemSize > maxCacheSize {
		return
	}
	// Reject if single item exceeds total limit
	if itemSize > c.maxTotalSize {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// If key already exists, remove old entry
	if elem, ok := c.store[key]; ok {
		entry := elem.Value.(*cacheEntry)
		c.ll.Remove(elem)
		c.totalSize -= entry.size
		delete(c.store, key)
	}

	// Evict LRU entries until there's room
	for c.totalSize+itemSize > c.maxTotalSize && c.ll.Len() > 0 {
		lru := c.ll.Back()
		if lru != nil {
			entry := lru.Value.(*cacheEntry)
			c.ll.Remove(lru)
			c.totalSize -= entry.size
			delete(c.store, entry.key)
		}
	}

	// Add new entry
	entry := &cacheEntry{
		key:  key,
		resp: CachedResponse{Headers: h, Body: body},
		size: itemSize,
	}
	elem := c.ll.PushFront(entry)
	c.store[key] = elem
	c.totalSize += itemSize
}
