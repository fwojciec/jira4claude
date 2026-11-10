---
id: jir-zb7m
status: open
deps: [jir-hodd]
links: []
created: 2026-11-10T07:47:17Z
type: task
priority: 2
assignee: Filip Wojciechowski
parent: jir-5662
---
# Hide noisy custom fields in issue view

Design 2.5 (F16). issue view hides empty values ([], {}, "") and a short list of machine-only field types (development summary, rank). It prints the hidden count, as issue fields does. --all-fields shows everything.

## Acceptance Criteria

Hidden count printed; --all-fields restores them; --json follows the same rule (decide and document); --field NAME for a hidden field still returns it. Tests first, in external test packages with t.Parallel(). make validate passes on the tree that becomes main.

