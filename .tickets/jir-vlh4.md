---
id: jir-vlh4
status: open
deps: []
links: []
created: 2026-11-10T07:47:17Z
type: task
priority: 1
assignee: Filip Wojciechowski
parent: jir-5662
---
# Streams and exit codes: stdout is data only

Design 2.1. Errors and warnings go to stderr in both modes; in --json mode they are JSON objects on stderr (today JSON errors go to stdout, pinned as a quirk in cmd/j4c tests). Usage errors exit 1 and print usage to stderr (today 80). Seam: Run(args, stdin, stdout, stderr, deps) in cmd/j4c/main.go.

## Acceptance Criteria

For every command, text and --json: on failure stdout is empty and stderr holds the error; conversion warnings are on stderr; exit codes table-tested (validation 1, conflict 5, and the rest as error.go maps them); the pinned quirk tests are replaced, not deleted. Tests first, in external test packages with t.Parallel(). make validate passes on the tree that becomes main.

