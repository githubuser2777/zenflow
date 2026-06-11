# ZenFlow v0.2.b Release Notes

## 🚀 New Features
- **Cross-Platform CI Builds**: Automated GitHub Actions workflow builds tight binaries (`-trimpath`, `-s -w`) for Windows and Linux on every push.

## 🛡️ Security & Privacy
- **IP Spoofing Protection**: Rate limiter now ignores untrusted `X-Forwarded-For` headers outside local networks.
- **Strict Host Blocking**: Patched domain blocker bypasses for requests missing the `Host` header.

## ⚡ Performance & Reliability 
- **Cache Memory Leak Fix**: Eliminated lock contention and reference cycle leaks via background LRU promotion workers.
- **Zero-Allocation Upgrades**: Custom port splitter and optimized cookie scrubbing reduce heap allocations on hot paths.
- **Clean Code Refactor**: Decoupled network utilities and enforced Single Responsibility Principle across core files.
