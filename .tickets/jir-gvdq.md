---
id: jir-gvdq
status: in_progress
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

