---
id: jir-gvdq
status: closed
deps: []
links: []
created: 2026-11-10T10:52:57Z
type: bug
priority: 1
assignee: Filip Wojciechowski
parent: jir-5662
---
# Request timeout: no command may hang on a stalled connection

The HTTP client sets no timeout and commands use context.Background(), so a server that accepts a connection and never answers hangs any command forever. The tool's first rule is never hang. Add a default overall timeout for API calls and a separate rule for attachment download and upload (time to first byte and stalled-transfer detection rather than a cap on total duration), configurable by flag or config, with a clear error and exit code.

## Acceptance Criteria

httptest with a handler that never answers: every command class returns an error within the timeout; a slow but progressing download is not cut; the timeout is documented in --help and README. Tests first, in external test packages with t.Parallel(). make validate passes on the tree that becomes main.


## Notes

**2026-11-10T11:30:35Z**

Merged to main at 092929d. Global --timeout (default 30s, 0 = none): a per-attempt deadline for API calls, an idle watchdog for download and upload, no retry after a timeout, exit 7, several-item commands stop at the first timeout. Codex found redirect bodies not counted as progress; fixed in the last commit.
