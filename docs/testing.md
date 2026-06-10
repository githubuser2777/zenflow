# Testing Workflow

ZenFlow has unit, integration, and adversarial (E2E) tests. The suite is still being stabilized; do not assume all commands pass in the current workspace. Use `docs/test_flows.md` for the full completion and real-world validation checklist.

## Current Status

Last checked: 2026-06-06.

- `go build -o proxy.exe ./cmd/proxy` passes when `GOCACHE` points to a writable workspace directory.
- `go test ./pkg/... -v` fails in `pkg/proxy` at `TestPrivacyHeaderScrubbing` because `User-Agent` is empty while the test expects `Mozilla/5.0`.
- `go test ./tests/e2e/... -v -timeout 60s` fails at `TestTier1_HTTPS_Tunnel_DefaultPort` and `TestTier1_HTTPS_Tunnel_Malformed`.
- On this Windows workspace, the default Go build cache under `AppData\Local\go-build` may return `Access is denied`; use a workspace-local cache when verifying.

## Unit Tests
Run all unit tests across the packages:
```powershell
$env:GOCACHE='C:\Users\Admin\Documents\CODE_WORKSPACE\ZenFlow\.tmp_go_cache'
go test ./pkg/... -v -count=1
```

## E2E and Adversarial Tests
The E2E tests verify real-world scenarios including privacy stripping, malware blocking, rate limiting, and adversarial DoS/bypasses:
```powershell
# Full E2E suite
go test ./tests/e2e/... -v -count=1 -timeout 60s

# Tier 5 Adversarial tests
go test ./tests/tier5/... -v -count=1 -timeout 30s
```

## Build Check
Verify the binary builds cleanly without warnings:
```powershell
go build -o proxy.exe ./cmd/proxy
```

## Manual TUI Smoke Test
Run the proxy and observe the TUI in the terminal:
```bash
./proxy.exe --blocklist hosts.txt
```
Verify:
- Live request logs populate on the left pane.
- Stats update on the right pane.
- Press `[q]` to shut down cleanly.

## Static Analysis
Run static analysis and formatting checks:
```bash
go vet ./...
go fmt ./...
```
