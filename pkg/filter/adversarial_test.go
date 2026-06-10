package filter

import (
	"testing"
)

func TestAdversarial_IPv6Bypass(t *testing.T) {
	b := NewBlocker([]string{"[2001:db8::1]"})

	// Legitimate IPv4 blocking works
	b.UpdateBlocklist([]string{"[2001:db8::1]", "example.com"})

	// Verify IPv4 works
	if blocked, _ := b.IsBlocked("example.com:80"); !blocked {
		t.Errorf("Expected example.com:80 to be blocked")
	}

	// Verify IPv6 fails!
	if blocked, _ := b.IsBlocked("[2001:db8::1]:80"); !blocked {
		t.Errorf("Expected [2001:db8::1]:80 to be blocked!")
	} else {
		t.Logf("FIXED: IPv6 address [2001:db8::1]:80 is blocked!")
	}

	// Verify FQDN bypass!
	if blocked, _ := b.IsBlocked("example.com.:80"); !blocked {
		t.Errorf("Expected example.com.:80 to be blocked!")
	} else {
		t.Logf("FIXED: FQDN example.com.:80 is blocked!")
	}
}
