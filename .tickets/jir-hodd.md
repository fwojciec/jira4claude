---
id: jir-hodd
status: in_progress
deps: [jir-vlh4]
links: []
created: 2026-11-10T07:47:17Z
type: task
priority: 1
assignee: Filip Wojciechowski
parent: jir-5662
---
# issue view KEY... with --field selection

Design 2.2. issue view accepts several keys. --field NAME (repeatable) selects fields; with one key and one field the output is the raw value, so 'issue view KEY --field description > d.md' works and the file can be fed back with --description-file.

## Acceptance Criteria

Several keys in text and --json (JSON is an array or keyed object; decide and document); unknown field is a validation error naming valid fields; raw single value has no decoration and ends with one newline; a failed key among several is reported on stderr with a non-zero exit and the others still print. Tests first, in external test packages with t.Parallel(). make validate passes on the tree that becomes main.

