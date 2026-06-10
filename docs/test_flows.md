# Test Flows

This document describes the verification flows to run when ZenFlow is believed to be complete. It complements `docs/testing.md`: that file lists commands, while this file describes the behavior and real-world cases to validate.

## Completion Gate

ZenFlow should only be considered complete when all of these are true:

- Build passes from a clean checkout.
- Unit tests pass.
- E2E tests pass.
- Manual proxy smoke tests pass for HTTP and HTTPS traffic.
- Auth, rate limiting, blocking, privacy scrubbing, malware blocking, cache behavior, TUI, and graceful shutdown have each been exercised.
- Docs reflect the verified behavior.
- Repo cleanup has been performed according to `docs/repo_cleanup.md`.

## Baseline Commands

```powershell
$env:GOCACHE='C:\Users\Admin\Documents\CODE_WORKSPACE\ZenFlow\.tmp_go_cache'
go build -o proxy.exe ./cmd/proxy
go test ./pkg/... -v -count=1
go test ./tests/e2e/... -v -count=1 -timeout 60s
go test ./tests/tier5/... -v -count=1 -timeout 30s
```

Optional deeper checks:

```powershell
go test ./... -race -count=1
go vet ./...
```

## Flow 1: Basic HTTP Proxying

Purpose: confirm normal HTTP forwarding still works.

Setup:

```powershell
.\proxy.exe --no-tui --blocklist hosts.txt --port 8080
```

Check:

```powershell
curl -x http://localhost:8080 http://example.com
```

Expected:

- Client receives a successful upstream response.
- Proxy logs an allow event.
- No auth challenge is returned when auth is not configured.

## Flow 2: HTTPS CONNECT Tunneling

Purpose: confirm HTTPS traffic is tunneled without MITM behavior.

Check:

```powershell
curl -x http://localhost:8080 https://example.com
```

Expected:

- HTTPS request succeeds.
- CONNECT tunnel returns a successful connection.
- Edge cases are covered by tests: explicit port, missing port, malformed port, unreachable host.

## Flow 3: Domain Blocking

Purpose: confirm ad/tracker blocklist enforcement.

Setup:

Create a local blocklist containing `blocked.example`.

Check:

```powershell
curl -x http://localhost:8080 http://blocked.example
```

Expected:

- Response is `403 Forbidden`.
- Subdomains such as `ads.blocked.example` are also blocked.
- Non-matching domains are allowed.

## Flow 4: Malware Blocklist

Purpose: confirm primary ad/tracker and secondary malware filters are independent.

Setup:

Run with both lists:

```powershell
.\proxy.exe --no-tui --blocklist hosts.txt --malware-blocklist malware.txt --port 8080
```

Expected:

- Domains in the primary list are blocked with the normal filter reason.
- Domains in the malware list are blocked with a malware-specific reason.
- Clean domains are not blocked by either list.

## Flow 5: Hot Reload And Auto Refresh

Purpose: confirm blocklists can change without restarting the proxy.

Local file flow:

1. Start with a local blocklist.
2. Request a domain that is not blocked.
3. Add that domain to the blocklist file.
4. Wait for the watcher interval.
5. Request the same domain again.

Expected:

- The domain changes from allowed to blocked without restarting.
- Empty or malformed reloads do not wipe the active protection list.

Remote URL flow:

- Use a test HTTP server or controlled URL that returns a hosts-format list.
- Confirm refresh updates the blocker.
- Confirm context cancellation stops the refresh goroutine.

## Flow 6: Proxy Authentication

Purpose: confirm Basic Proxy Auth behavior.

Setup:

```powershell
.\proxy.exe --no-tui --auth admin:secret --port 8080
```

Checks:

```powershell
curl -x http://localhost:8080 http://example.com
curl -x http://admin:wrong@localhost:8080 http://example.com
curl -x http://admin:secret@localhost:8080 http://example.com
```

Expected:

- Missing auth returns `407 Proxy Authentication Required`.
- Invalid auth returns `407`.
- Valid auth allows the request.
- `Proxy-Authorization` is not forwarded upstream.

## Flow 7: Rate Limiting

Purpose: confirm abusive clients are throttled.

Setup:

```powershell
.\proxy.exe --no-tui --rate-limit 1 --port 8080
```

Expected:

- First request is allowed.
- Rapid repeated requests from the same client IP return `429 Too Many Requests`.
- Different client identity logic is covered by tests, including `X-Forwarded-For` handling and IPv6 canonicalization.

## Flow 8: Privacy Scrubbing

Purpose: confirm privacy-sensitive headers and tracking cookies are handled as designed.

Send a request with:

- `Referer`
- `X-Forwarded-For`
- tracking cookies such as `_ga`, `_gid`, `_fbp`, `_hj`
- normal session cookies
- `User-Agent`

Expected:

- `Referer` is removed upstream.
- `X-Forwarded-For` is removed upstream.
- Tracking cookies are removed.
- Legitimate cookies are preserved.
- `User-Agent` behavior must match the final accepted product decision and tests.

## Flow 9: Static Asset Cache

Purpose: confirm cache correctness and memory limits.

Check:

- First GET for a cacheable static asset returns `X-Cache: MISS`.
- Second GET for the same cacheable asset returns `X-Cache: HIT`.
- `Cache-Control: private`, `no-cache`, and `no-store` are not cached.
- Authenticated requests are not cached.
- Oversized items are not cached.
- LRU eviction happens when total cache size is exceeded.

## Flow 10: TUI Smoke Test

Purpose: confirm the user-facing terminal UI works during real traffic.

Setup:

```powershell
.\proxy.exe --blocklist hosts.txt --port 8080
```

Expected:

- Live allow/block logs appear.
- Allowed and blocked counters update.
- `[c]` clears visible logs.
- `[b]` toggles the blocklist view state.
- `[q]` exits and triggers graceful shutdown.
- If uptime or real blocklist management are part of the release target, verify them explicitly.

## Flow 11: Graceful Shutdown

Purpose: confirm the proxy exits without leaking server state or corrupting the terminal.

Checks:

- Press `[q]` in TUI mode.
- Press `Ctrl+C` in TUI mode.
- Send an interrupt signal in `--no-tui` mode.

Expected:

- Root context is cancelled.
- HTTP server shutdown runs with timeout.
- TUI restores terminal state.
- No process remains listening on the configured port.

## Flow 12: Real-World Application

Purpose: validate normal usage beyond synthetic tests.

Suggested scenario:

1. Start ZenFlow with auth, rate limit, local ad blocklist, and malware blocklist.
2. Configure a browser or CLI client to use `localhost:8080` as HTTP and HTTPS proxy.
3. Browse several HTTP and HTTPS sites.
4. Request a blocked ad/tracker domain.
5. Request a malware-listed domain.
6. Fetch repeated static assets.
7. Leave the proxy running long enough to observe logs, stats, and shutdown behavior.

Expected:

- Clean browsing works.
- Blocked domains fail fast with `403`.
- Auth failures return `407`.
- Abuse returns `429`.
- Logs and stats remain responsive.
- No unexpected crashes, hangs, or terminal corruption occur.

## Final Completeness Checklist

- [ ] Build passes.
- [ ] Unit tests pass.
- [ ] E2E tests pass.
- [ ] Race/vet checks reviewed.
- [ ] Manual HTTP flow passes.
- [ ] Manual HTTPS flow passes.
- [ ] Auth flow passes.
- [ ] Rate limit flow passes.
- [ ] Blocking and malware flows pass.
- [ ] Privacy flow passes.
- [ ] Cache flow passes.
- [ ] TUI flow passes.
- [ ] Graceful shutdown passes.
- [ ] Docs updated with final status.
- [ ] Repo cleanup completed.

