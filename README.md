# ZenFlow

![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)
![Build Status](https://github.com/zenflow/zenflow/actions/workflows/release.yml/badge.svg)

A high-performance, minimalist HTTP/HTTPS proxy and ad blocker written in Go. Built with a focus on zero-allocation data paths and simplicity.

## Features

- **Ad & Malware Blocking**: Dynamic, auto-refreshing blocklists.
- **Zero-Allocation**: Optimized hot paths (cache, IP tracking) with no heap allocations.
- **TUI**: Real-time terminal user interface for monitoring request logs.
- **Security & Privacy**: Rate limiting, Basic Auth, and privacy header scrubbing.
- **HTTPS Tunneling**: Native support for HTTP `CONNECT`.

## Usage

Start the proxy:

```bash
go run ./cmd/proxy --blocklist https://raw.githubusercontent.com/StevenBlack/hosts/master/hosts
```

Then configure your system or browser to use the proxy (default: `localhost:8080`).
Test it with curl:
```bash
curl -x http://localhost:8080 https://example.com
```

### Configuration

| Flag | Description | Default |
| :--- | :--- | :--- |
| `--port` | Listen port | `8080` |
| `--blocklist` | Ad blocklist path/URL | `hosts.txt` |
| `--malware-blocklist`| Malware blocklist path/URL | |
| `--auth` | Basic Auth (`user:pass`) | |
| `--rate-limit` | Requests per second limit | |
| `--no-tui` | Run in headless mode | `false` |

## Documentation

See the `docs/` directory for architecture details, system design, and testing flows.

## Development

```bash
go test ./...
```
