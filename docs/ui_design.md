# Terminal User Interface (TUI) Design (docs/ui_design.md)

This document outlines the conceptual design and proposed implementation for the ZenFlow Proxy Terminal User Interface (TUI). This interface will allow users to manage the proxy and monitor traffic directly within the terminal, maintaining a developer-centric, lightweight footprint.

## 1. Design Aesthetics & Core Principles
Following our First Principles and system rules:
- **Hacker/Terminal Aesthetic:** A sleek, keyboard-driven interface running entirely in the terminal. No browsers needed.
- **Minimal Resource Usage:** TUIs use a fraction of the RAM and CPU compared to web dashboards, perfectly aligning with a high-performance proxy.
- **Technology Choice:** We will use a modern Go TUI framework like `bubbletea` and `lipgloss` (from Charmbracelet) for beautiful, responsive terminal rendering, or stick to raw ANSI escape sequences for a zero-dependency approach.

## 2. Proposed TUI Layout

### A. Main Overview (Dashboard)
- **Top Header:** Proxy Status (🟢 Running on :8080), Uptime, and Memory Usage.
- **Traffic Split View:** 
  - *Left Pane (Logs):* Real-time scrolling list of incoming requests. Color-coded (Green for allowed, Red for blocked).
  - *Right Pane (Stats):* Counters for Total Requests, Blocked Ads, and Current Bandwidth.
- **Bottom Footer:** Hotkeys for navigation (e.g., `[q] Quit`, `[b] Blocklist`, `[c] Clear Logs`).

### B. Blocklist Management Mode
- When pressing `[b]`, the view shifts to managing the blocklist.
- **List View:** Scrollable list of currently blocked domains.
- **Input Mode:** Press `[a]` to open a prompt at the bottom to add a new domain (e.g., `Add domain: doubleclick.net`).
- **Delete Mode:** Select a domain and press `[d]` or `[Delete]` to remove it.

## 3. Technical Architecture
1. **Goroutine Separation:** The HTTP Proxy server (`pkg/proxy`) and the TUI event loop will run in separate goroutines. They will communicate via Go channels (`chan`) to ensure the TUI does not block proxy requests.
2. **Channel Logging:** Instead of writing directly to `os.Stdout` (which breaks TUI rendering), the proxy's `logger` will send log structs into a channel that the TUI reads and renders.
3. **Graceful Shutdown:** Pressing `Ctrl+C` or `q` in the TUI will trigger context cancellation, safely shutting down the proxy server before exiting the terminal application.

## 4. Conceptual Layout
```text
┌────────────────────────────────────────────────────────────┐
│ ⚡ ZenFlow Proxy                       🟢 Online (99.9%)   │
├────────────────────────────────────────┬───────────────────┤
│ Live Traffic Feed                      │ Metrics           │
│ 🟢 [ALLOW] GET github.com              │ 🌐 Total: 10,243  │
│ 🔴 [BLOCK] GET doubleclick.net         │ 🛡️ Blocked: 1,402 │
│ 🟢 [ALLOW] GET golang.org              │ ⏱️ Uptime: 24h    │
│ 🟢 [ALLOW] GET pkg.go.dev              │                   │
│                                        │ [ Blocklist ]     │
│                                        │ > doubleclick.net │
│                                        │ > ads.example.com │
├────────────────────────────────────────┴───────────────────┤
│ [q] Quit  [b] Manage Blocklist  [c] Clear Logs             │
└────────────────────────────────────────────────────────────┘
```
