---
id: jir-0z1l
status: open
deps: []
links: []
created: 2026-11-11T03:06:00Z
type: bug
priority: 3
assignee: Filip Wojciechowski
parent: jir-vqab
---
# attachment upload: an applied single-file upload whose 2xx answer cannot be read prints nothing

issue create and issue comment print their result when Jira applied the write but the 2xx answer could not be read (jir-dqbz, UnreadableAnswerError). A single-file attachment upload in the same situation exits 7 with an error that says to check attachment list, but stdout is empty, so a script reading stdout sees no record of the applied upload.

## Acceptance Criteria

A single-file upload answered 2xx with an unreadable body prints a result on stdout (text and --json) naming the issue and the file, without an attachment ID, and exits 7 with a message saying the write was applied and must not be repeated. Several-file uploads behave the same per file. Covered by a cmd/j4c test and an e2e scenario.

