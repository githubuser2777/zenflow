# Project: ZenFlow Proxy Refactoring & Validation

## Architecture
- **Proxy Server** (`pkg/proxy/server.go`): Handles incoming HTTP and TCP requests, manages proxying, and routes requests through filters/middleware.
- **Cache** (`pkg/proxy/cache.go`): Cache module that stores/retrieves proxy responses.
- **Rate Limiter** (`pkg/ratelimit/ratelimit.go`): Limits request rates using a token bucket or similar algorithm.
- **IP Blocker** (`pkg/filter/blocker.go`): Blocks requests based on IP rules.

## Milestones
| # | Name | Scope | Dependencies | Status |
|---|------|-------|-------------|--------|
| 1 | Explore & Design | Examine the codebase and identify code smells and hot paths. | None | DONE |
| 2 | Clean Code Refactoring (M3) | Refactor `server.go`, `cache.go`, `ratelimit.go`, and `blocker.go`. | Milestone 1 | DONE |
| 3 | Performance Benchmarks (M4) | (CANCELLED per follow-up instructions) | Milestone 2 | CANCELLED |
| 4 | Verification & Checkpoint | Verify tests/vet pass for M3, update `/root/zenflow/docs/checkpoint.md`, and pause. | Milestone 2 | DONE |

## Interface Contracts
- Proxy -> Cache: Cache retrieval and storage interfaces.
- Proxy -> RateLimiter: Rate limiting check interface.
- Proxy -> Blocker: Connection blocking and request filtering interface.

## Code Layout
- `cmd/proxy/`: Main executable.
- `pkg/proxy/`: Proxy server, cache, TCP proxy, and middleware.
- `pkg/ratelimit/`: Rate limiting logic.
- `pkg/filter/`: IP blocking and filtering logic.
- `tests/e2e/`: E2E test suite.
