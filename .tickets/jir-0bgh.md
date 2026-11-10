---
id: jir-0bgh
status: in_progress
deps: []
links: []
created: 2026-11-10T10:52:57Z
type: task
priority: 1
assignee: Filip Wojciechowski
parent: jir-5662
---
# End-to-end test of the built binary against a stub Jira

CLI tests drive Run with mock services and HTTP tests exercise each service alone; nothing runs the real wiring: defaultDeps, netrc lookup, loadConfig walk-up, stdin from main, real printers, real HTTP services together. Build the binary in a test and run the README's scripting loops against an httptest stub Jira with a scratch HOME (netrc) and working directory passed through the child's environment and cmd.Dir, never os.Chdir or t.Setenv.

## Acceptance Criteria

Covered end to end with the real binary: init; view --field; the guarded edit with a mention and media (PUT body holds the re-attached nodes); --if-updated conflict exits 5; --description-file - from a pipe; --dry-run makes no write; clear refused then forced; list; transition by status; attachment download through a redirect to a second server which receives no Authorization; upload; delete; --json errors on stderr. Tests first, in external test packages with t.Parallel(). make validate passes on the tree that becomes main.

