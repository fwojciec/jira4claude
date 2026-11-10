---
id: jir-3t00
status: in_progress
deps: [jir-bhhg, jir-mi82]
links: []
created: 2026-11-10T07:47:17Z
type: task
priority: 2
assignee: Filip Wojciechowski
parent: jir-vqab
---
# attachment upload

Design Milestone 3. attachment upload KEY FILE...: multipart POST with X-Atlassian-Token: no-check; prints the new IDs as a MutationResult. Upload is a POST, so the retry policy of design 1.5 applies: retried on 429 only, never after a 5xx; the body must be rewindable.

## Acceptance Criteria

Multipart body verified in httptest; several files in one call; a 5xx is not retried and the error says the upload may have been applied; a missing local file fails before any request. Tests first, in external test packages with t.Parallel(). make validate passes on the tree that becomes main.

