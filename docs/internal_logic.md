# Internal Logic & Developer Guide

This document explains the core technical decisions for AI Developer and Reviewer Agents. It serves as a technical context transfer between agents.

## 1. Concurrency and Thread Safety
The proxy handles hundreds of concurrent requests via Go's HTTP server.
* **`filter.Blocker` (Phase 3):** We use `sync.RWMutex` to protect the `blockedDomains` map. Since checking if a domain is blocked (Read) happens much more frequently than updating the blocklist (Write), `RLock()` allows multiple goroutines to read the map simultaneously without blocking each other.
* **`cache.Cache` (Phase 2):** The LRU cache uses a `sync.RWMutex` to protect its internal data structures (`items` map, `evictList`). It employs `bodyBufferPool` and `bufferPool` (`sync.Pool`) for memory reuse during proxy data transfers and response buffering, significantly reducing GC pressure on hot paths.
* **TUI Logger (Phase 4):** `pkg/logger` uses a buffered channel (`EventChannel`) to pass log events from the proxy worker goroutines to the main TUI goroutine. This decoupling ensures the proxy is never blocked by UI rendering. If the channel fills, events are dropped to prioritize traffic flow over logs.

## 2. Resource Management
To strictly adhere to `RULES.md` regarding memory and connection leaks:
* **Server Timeouts:** The `http.Server` in `main.go` is configured with `ReadTimeout`, `WriteTimeout`, and `IdleTimeout`. This ensures that malicious or extremely slow clients/servers cannot hold connections open indefinitely.
* **OOM Protection:** The LRU cache tracks total bytes used. It preemptively rejects caching payloads that exceed `maxCacheSize` and evicts old entries when global capacity is reached.
* **Context Cancellation:** We use `context.WithCancel` for the root background context, passing it to `config.StartAutoRefresh` to clean up timers. `context.WithTimeout` is used during graceful shutdown to ensure the server doesn't hang forever waiting for connections to close.

## 3. The ReverseProxy Standard Library
Instead of manually parsing HTTP requests and managing TCP sockets for plaintext HTTP, we use `httputil.ReverseProxy`.
* **Director Function:** Modifies the incoming request to point to the actual target server. We explicitly delete the `Proxy-Connection` header, `Referer` header, and scrub cookies to enforce privacy.
* **ModifyResponse:** We intercept responses to transparently cache static assets.
* **Error Handler:** Captures upstream errors (e.g., the target website is down) and safely returns an HTTP 502 Bad Gateway to the client.

## 4. HTTPS Tunneling
HTTPS traffic is handled via the `CONNECT` method. The proxy hijacks the client connection (`http.Hijacker`), dials the upstream host, and spins up two goroutines to `io.CopyBuffer` data bidirectionally. This blind TCP tunneling prevents MITM but relies on the SNI/Host header for domain filtering.
