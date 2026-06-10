# ZenFlow HTTP Proxy

![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)![Release](https://github.com/zenflow/zenflow/actions/workflows/release.yml/badge.svg)

ZenFlow is a minimalist, First-Principles-driven HTTP Proxy and Ad Blocker written in Go.

> **Lưu ý:** Dự án hiện vẫn đang trong quá trình thử nghiệm và hoàn thiện (vẫn trong quá trình test).


## Features (Phase 2-4 Additions Included)

- **Ad & Tracker Blocking**: Dual-blocker system for filtering ads and malware domains dynamically (Phase 3).
- **Auto-Refresh Blocklists**: Periodically fetches and hot-reloads blocklists (Phase 4).
- **Terminal User Interface (TUI)**: Live request logs and basic proxy statistics via channel architecture (Phase 4). Uptime and full blocklist management are not implemented yet.
- **Rate Limiting**: IP-based token bucket rate limiting.
- **Proxy Authentication**: Restrict access using Basic Auth.
- **Privacy Header Scrubbing**: Strips tracking cookies and privacy-invasive headers. Current tests show the User-Agent behavior still needs reconciliation.
- **HTTPS Tunneling**: Native support for HTTP `CONNECT` method.
- **LRU Cache & OOM Protection**: In-memory caching for static assets with strict memory bounds (Phase 2).

## Architecture Overview

ZenFlow acts as an intermediate proxy server between client applications (such as browsers) and destination servers. Its primary components include:

1. **Proxy Server**: Based on Go's `net/http/httputil.ReverseProxy`, it handles HTTP request forwarding and TLS connection tunneling for HTTPS.
2. **Filter Engine**: Intercepts requests and matches destination domains against an internal blocklist to drop ads and trackers, supporting periodic auto-refresh.
3. **Authentication Manager**: Enforces Basic Authentication checks.
4. **Rate Limiter**: Token-bucket based rate limiting to prevent abuse.
5. **LRU Cache**: Caches static assets to improve performance and reduce upstream bandwidth.
6. **TUI Logger**: Channel-based logging powering a real-time terminal UI.

For detailed system design and logic, please refer to the `docs/` directory:

- Architecture - System context and component wiring.
- Features - Current capabilities.
- Internal Logic - Deep dive into the Go code and concurrency model.
- Testing - Full test workflow.
- Test Flows - Completion, edge-case, and real-world validation flows.
- Repo Cleanup - Cleanup checklist for after the project is verified complete.

## Configuration & Usage

### Running the Proxy

```bash
# Basic start
go run ./cmd/proxy

# With TUI and remote blocklist auto-refresh
go run ./cmd/proxy --blocklist https://raw.githubusercontent.com/StevenBlack/hosts/master/hosts
```

### CLI Flags

- `--blocklist <path_or_url>`: Path or URL to the ad blocklist (default: `hosts.txt`)
- `--malware-blocklist <path_or_url>`: Path or URL to the malware blocklist.
- `--port <port>`: Port to listen on (default: `8080`)
- `--auth <user:pass>`: Basic auth credentials.
- `--rate-limit <rps>`: Rate limit in requests per second.
- `--no-tui`: Disable the Terminal UI and run in headless mode.

### Testing

Run all unit and E2E tests:

```bash
go test ./... -v
```

Current workspace status: the binary builds, but the full test suite is not green yet. Known failures are documented in `docs/testing.md` and `docs/checkpoint.md`.

See `docs/testing.md` for more testing commands.
