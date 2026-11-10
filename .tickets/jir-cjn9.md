---
id: jir-cjn9
status: open
deps: [jir-u77o, jir-mi82]
links: []
created: 2026-11-10T07:47:24Z
type: task
priority: 2
assignee: Filip Wojciechowski
parent: jir-5662
---
# Dry run for create, update and comment (--dry-run)

Design 2.3. Prints the text as Jira will hold it (the Markdown read back from the converted ADF, after re-attachment for an update) plus the conversion report, and writes nothing. For update it still reads the issue and checks --if-updated.

## Acceptance Criteria

No mutating service call happens under --dry-run (mock asserts); output shows the resulting Markdown and each conversion issue; a write that would be refused exits non-zero with the same error as the real run. Tests first, in external test packages with t.Parallel(). make validate passes on the tree that becomes main.

