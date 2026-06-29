package filter

import (
	"strings"
	"sync/atomic"
)

// Blocker is responsible for filtering out domains.
type Blocker struct {
	blockedDomains atomic.Value // stores map[string]bool
}

func normalizeDomain(d string) string {
	d = strings.TrimSuffix(d, ".")
	d = strings.Trim(d, "[]")
	return strings.ToLower(d)
}

// NewBlocker creates a new Blocker with an initial list of blocked domains.
func NewBlocker(initialDomains []string) *Blocker {
	b := &Blocker{}
	m := make(map[string]bool)
	for _, domain := range initialDomains {
		m[normalizeDomain(domain)] = true
	}
	b.blockedDomains.Store(m)
	return b
}

func splitHost(host string) string {
	if idx := strings.LastIndexByte(host, ':'); idx != -1 {
		// Ensure it's not a raw IPv6 address like [::1]
		if !strings.HasSuffix(host, "]") {
			return host[:idx]
		}
	}
	return host
}

// IsBlocked checks if a given host is in the block list.
func (b *Blocker) IsBlocked(host string) (bool, string) {
	hostOnly := splitHost(host)
	hostOnly = normalizeDomain(hostOnly)

	m, _ := b.blockedDomains.Load().(map[string]bool)
	if m == nil {
		return false, ""
	}

	domain := hostOnly
	for {
		if m[domain] {
			return true, domain
		}
		idx := strings.IndexByte(domain, '.')
		if idx == -1 {
			break
		}
		domain = domain[idx+1:]
	}

	return false, ""
}

// UpdateBlocklist replaces the current blocklist with a new one
func (b *Blocker) UpdateBlocklist(domains []string) {
	newBlocked := make(map[string]bool)
	for _, domain := range domains {
		newBlocked[normalizeDomain(domain)] = true
	}
	b.blockedDomains.Store(newBlocked)
}

// Count returns the number of blocked domains.
func (b *Blocker) Count() int {
	m, _ := b.blockedDomains.Load().(map[string]bool)
	return len(m)
}

// GetPreview returns a slice of up to n blocked domains for previewing.
func (b *Blocker) GetPreview(n int) []string {
	m, _ := b.blockedDomains.Load().(map[string]bool)
	var preview []string
	count := 0
	for k := range m {
		preview = append(preview, k)
		count++
		if count >= n {
			break
		}
	}
	return preview
}

// GetAll returns a slice of all blocked domains.
func (b *Blocker) GetAll() []string {
	m, _ := b.blockedDomains.Load().(map[string]bool)
	var all []string
	for k := range m {
		all = append(all, k)
	}
	return all
}
