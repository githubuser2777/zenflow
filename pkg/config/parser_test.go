package config

import (
	"strings"
	"testing"
)

func TestParseHostsFormat(t *testing.T) {
	input := `
# some comment
0.0.0.0 doubleclick.net
127.0.0.1 google-analytics.com # inline comment
0.0.0.0 localhost
127.0.0.1 localhost.localdomain
	`

	domains := ParseHostsFormat(strings.NewReader(input))
	if len(domains) != 2 {
		t.Fatalf("expected 2 domains, got %d", len(domains))
	}

	if domains[0] != "doubleclick.net" {
		t.Errorf("expected doubleclick.net, got %s", domains[0])
	}
	if domains[1] != "google-analytics.com" {
		t.Errorf("expected google-analytics.com, got %s", domains[1])
	}
}
