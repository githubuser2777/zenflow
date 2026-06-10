package filter

import (
	"net"
	"strings"
	"sync"
)

// Blocker is responsible for filtering out domains.
type Blocker struct {
	blockedDomains map[string]bool
	mu             sync.RWMutex
}

func normalizeDomain(d string) string {
	d = strings.ToLower(d)
	d = strings.TrimSuffix(d, ".")
	d = strings.Trim(d, "[]")
	return d
}

// NewBlocker creates a new Blocker with an initial list of blocked domains.
func NewBlocker(initialDomains []string) *Blocker {
	b := &Blocker{
		blockedDomains: make(map[string]bool),
	}
	for _, domain := range initialDomains {
		b.blockedDomains[normalizeDomain(domain)] = true
	}
	return b
}

// IsBlocked checks if a given host is in the block list.
func (b *Blocker) IsBlocked(host string) (bool, string) {
	hostOnly, _, err := net.SplitHostPort(host)
	if err != nil {
		hostOnly = host
	}
	hostOnly = normalizeDomain(hostOnly)

	b.mu.RLock()
	defer b.mu.RUnlock()

	domain := hostOnly
	for {
		if b.blockedDomains[domain] {
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

	b.mu.Lock()
	defer b.mu.Unlock()
	b.blockedDomains = newBlocked
}
