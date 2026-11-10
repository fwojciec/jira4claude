---
id: jir-vtkn
status: open
deps: [jir-bhhg, jir-mi82]
links: []
created: 2026-11-10T07:47:17Z
type: task
priority: 3
assignee: Filip Wojciechowski
parent: jir-vqab
---
# attachment delete

Design Milestone 3. attachment delete ID deletes one attachment and reports a MutationResult.

## Acceptance Criteria

httptest covers success, 404 and 403; no retry on failure beyond the existing policy for DELETE. Tests first, in external test packages with t.Parallel(). make validate passes on the tree that becomes main.

