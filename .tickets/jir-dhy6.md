---
id: jir-dhy6
status: open
deps: [jir-vlh4]
links: []
created: 2026-11-10T07:47:17Z
type: task
priority: 2
assignee: Filip Wojciechowski
parent: jir-5662
---
# List and view detail: type, assignee, full summaries, priority names, comment IDs

Design 2.2 (F15). issue list items gain type and assignee; summaries are not cut; priority prints as Jira names it. The Markdown view shows comment IDs so a comment can be deleted without a JSON read.

## Acceptance Criteria

list and ready, text and --json, show type and assignee; a 200-character summary prints whole; comment IDs appear in issue view text output. Tests first, in external test packages with t.Parallel(). make validate passes on the tree that becomes main.

