# ZenFlow HTTP Proxy

![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)![Release](https://github.com/zenflow/zenflow/actions/workflows/release.yml/badge.svg)

ZenFlow is a minimalist, First-Principles-driven HTTP Proxy and Ad Blocker written in Go.

> **Lưu ý:** Dự án hiện vẫn đang trong quá trình thử nghiệm và hoàn thiện (vẫn trong quá trình test).


## Features (v0.2.b)

- **Cross-Platform Automated Builds**: GitHub Actions CI builds optimized binaries (Windows/Linux) automatically.
- **Zero-Allocation Data Paths**: High performance on hot paths (cache, IP tracking, blocking) with no heap allocations.
- **Ad & Tracker Blocking**: Dual-blocker system for filtering ads and malware domains dynamically, with protection against Host header bypasses.
- **Auto-Refresh Blocklists**: Periodically fetches and hot-reloads blocklists.
- **Terminal User Interface (TUI)**: Live request logs and basic proxy statistics via channel architecture.
- **Rate Limiting**: IP-based token bucket rate limiting with built-in IP spoofing resistance (`X-Forwarded-For` safety).
- **Proxy Authentication**: Restrict access using Basic Auth.
- **Privacy Header Scrubbing**: Strips tracking cookies and privacy-invasive headers.
- **HTTPS Tunneling**: Native support for HTTP `CONNECT` method.
- **LRU Cache**: Background-promoted LRU cache prevents lock contention and memory leaks.

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

### Testing & CI

Run all unit and E2E tests:

```bash
go test ./... -v
```

Current workspace status: **All tests pass 100%.** Phase 5 Optimization and Clean Code (M3) is complete. 

### GitHub Actions CI
The project includes an automated GitHub Actions workflow (`.github/workflows/build.yml`) that builds optimized Windows and Linux binaries using `-trimpath` and `-ldflags="-s -w"`. Download the compiled `zenflow.exe` or `zenflow` from the GitHub Actions artifacts on every push!
