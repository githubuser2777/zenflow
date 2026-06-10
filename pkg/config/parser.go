package config

import (
	"bufio"
	"io"
	"strings"
)

// ParseHostsFormat parses a StevenBlack hosts formatted file
// and extracts the domains to be blocked.
func ParseHostsFormat(r io.Reader) []string {
	var domains []string
	scanner := bufio.NewScanner(r)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Ignore comments and empty lines
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Remove inline comments
		if idx := strings.Index(line, "#"); idx != -1 {
			line = strings.TrimSpace(line[:idx])
		}

		// Look for "0.0.0.0 domain" or "127.0.0.1 domain" lines
		parts := strings.Fields(line)
		if len(parts) >= 2 {
			if parts[0] == "0.0.0.0" || parts[0] == "127.0.0.1" {
				domain := parts[1]
				if domain != "localhost" && domain != "localhost.localdomain" && domain != "local" {
					domains = append(domains, domain)
				}
			}
		}
	}

	return domains
}
