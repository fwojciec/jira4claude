---
id: jir-h44x
status: in_progress
deps: []
links: []
created: 2026-11-10T10:52:57Z
type: bug
priority: 1
assignee: Filip Wojciechowski
parent: jir-pskt
---
# Unchanged rewrite of every schema node type is identical or reported

Some node types still change on an unchanged view-then-rewrite of a description with nothing reported: placeholder nodes are dropped silently on read (markdown/to_markdown.go, the placeholder case), layoutSection and layoutColumn are flattened by adfFlattenToGFM with no degraded issue, and decisionList reads as a bullet list. There may be more (node attrs such as panel type, table layout, heading or paragraph alignment marks, textColor and other marks the reader does not write, expand titles, ordered list start, code block attrs, breakout marks). The README promises that existing content the tool cannot show as Markdown makes a rewrite refuse. Make it a property: for a document built from every node type and mark in markdown/testdata/adf-schema.json, read, write back unchanged through ToADF and Reattach, and the result is identical to the original or every difference is covered by a reported issue (dropped refuses, degraded warns).

## Acceptance Criteria

A table-driven test with one schema-valid sample per node type and per mark; each either round-trips identically or reports an issue naming the node or mark; placeholder and layout are fixed or reported per the approved rule (loss of content or behaviour is dropped, form change with content kept is degraded); the fuzz property over fixtures still passes. Tests first, in external test packages with t.Parallel(). make validate passes on the tree that becomes main.

