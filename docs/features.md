# Project Features

This document tracks the capabilities of the ZenFlow HTTP Proxy.

## Core Features

* **HTTP Proxying & HTTPS Tunneling**: Forwards unencrypted HTTP requests to target servers, and supports the `CONNECT` method for blind TCP tunneling of HTTPS traffic. Current E2E coverage still has failing CONNECT edge cases around missing or malformed ports.
* **Dual-Filter Domain Blocking**: Runs two separate domain filters (Ads/Trackers and Malware). Intercepts requests and returns HTTP 403 Forbidden with specific reasons if the requested host matches (Phase 3).
* **Subdomain Matching**: The filter automatically blocks subdomains of a blocked domain (e.g., blocking `example.com` will also block `ads.example.com`).
* **Hot-Reload & Auto-Refresh**: Blocklists can be loaded from local files or remote URLs. Local files are watched for changes, and remote URLs are refreshed periodically (every 24 hours) via a background goroutine (Phase 4).
* **Terminal User Interface (TUI)**: An interactive UI built with Bubble Tea that displays real-time scrolling logs (allow/block) and basic statistics (request counts, block rate) via a channel-based logging architecture (Phase 4). Uptime and real blocklist management are still incomplete.
* **Proxy Authentication**: Enforces Basic Authentication checks to restrict proxy access to authorized users. Includes protection against large payload DoS.
* **Rate Limiting**: IP-based token bucket rate limiter to prevent abuse, correctly handling `X-Forwarded-For` and canonicalizing IPv6/IPv4 addresses.
* **Privacy Header Scrubbing**: Strips known tracking cookies (e.g., `_ga`) and removes privacy-invasive headers (like `Referer`). The expected `User-Agent` behavior is currently inconsistent between code and tests.
* **Static Asset Caching**: An LRU cache stores responses for static assets (`.png`, `.js`, etc.) with OOM protection enforcing total cache size and per-item size limits (Phase 2).
* **Graceful Shutdown**: The server listens for OS interrupt signals (or `[q]` from the TUI) and safely drains active connections before shutting down.
