---
id: jir-kz84
status: in_progress
deps: []
links: []
created: 2026-11-10T07:47:17Z
type: bug
priority: 1
assignee: Filip Wojciechowski
external-ref: gh-161
parent: jir-pskt
---
# Round trip: links with empty, odd or self-referential destinations

GitHub issue 161 is the contract, plus the nine link causes handed over by the other round-trip fixes (listed in the dispatch; summarised in the issue's closing comment). Last cause class left in TestPropertyKnownFailures.

## Acceptance Criteria

The five remaining known-failure cases are seeds and pass. Several consecutive clean 120 s runs of FuzzToADFFixedPoint. Tests first, in external test packages with t.Parallel(). make validate passes on the tree that becomes main.

