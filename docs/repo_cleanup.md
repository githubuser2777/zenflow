# Repo Cleanup Notes

This document should be used after the implementation is complete, the expected test suite is green, and the current checkpoint no longer needs temporary debugging artifacts.

## Cleanup Goal

Keep the repository easy to review, build, test, and release. Cleanup is repo hygiene; it does not replace code quality review, test verification, or release validation.

## Do Not Clean Until

- `go build -o proxy.exe ./cmd/proxy` passes.
- Unit tests and E2E tests pass according to `docs/testing.md`.
- The current checkpoint has been updated with the final verified status.
- Any temporary files needed to debug failing tests have been reviewed.

## Safe Cleanup Candidates

- Build outputs: `*.exe`, `*.test`, `*.out`, `zenproxy`, `main.exe`, `proxy.exe`, `test_proxy.exe`, `test_client.exe`.
- Logs and run outputs: `*.log`, `*_out.txt`, `*_output.txt`, `build_err*.txt`, `build_error.txt`, `err*.txt`, `test_results.txt`.
- Local caches: `.tmp_go_cache/`, coverage output files.
- Throwaway manual test inputs: temporary `hosts.txt`, `blocklist.txt`, `malware.txt`, `test_hosts.txt`, `myhosts.txt` if they are not part of documented fixtures.
- One-off local scripts: loose `test_*.py`, `test_*.ps1`, and root-level temporary `.go` files if they are not part of the maintained test suite.
- Agent coordination folders: `.agents/` after all useful findings have been moved into docs or issues.
- Legacy draft folder: `2/`, after confirming it contains no authoritative code or docs.

## Keep

- Source code under `cmd/`, `pkg/`, and maintained tests under `tests/`.
- `go.mod` and `go.sum`.
- GitHub workflow files under `.github/`.
- Maintained docs under `docs/`, especially `checkpoint.md`, `testing.md`, `test_flows.md`, and this cleanup note.
- Root docs that are intentionally part of the project: `README.md`, `PROJECT.md`, `CONTRIBUTING.md`, `SECURITY.md`, `LICENSE`.

## Recommended Process

1. Run `git status --short`.
2. Classify every untracked file as source/test/doc, generated artifact, or unknown.
3. Move any useful debugging knowledge from artifacts into `docs/checkpoint.md` or an issue before deleting.
4. Update `.gitignore` for repeatable generated files.
5. Delete only confirmed artifacts.
6. Run build and tests again after cleanup.
7. Confirm `git status --short` contains only intentional source, test, and doc changes.

## Avoid

- Do not use `git clean -fdx` while there are untracked source files or tests.
- Do not delete `.agents/` or `2/` blindly until their useful content has been reviewed.
- Do not remove failing-test output before the failure has been captured in a stable doc or issue.
- Do not treat cleanup as proof that the code is complete.

