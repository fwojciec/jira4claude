---
id: jir-h44x
status: closed
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


## Notes

**2026-11-10T12:08:27Z**

Merged to main at 9d139c2 (branch h44x-rewrite-property). markdown/rewrite_inventory_test.go samples every schema node type, mark and attribute (81 rows); 45 were changed silently by an unchanged rewrite before. Reattach now compares the original, the read written back and the edit (markdown/restore.go): an unchanged block comes back as Jira held it, an edited block keeps hidden values where it matches, and the rest is reported. Placeholders are tokens. Codex round 1: token swap of nodes differing only in localId, and a false refusal on a reshaped table, both fixed in 9d139c2. Round 2 found three edge cases, ticketed separately. Known unreported limit, in the README: a block removed and another of the same kind added in the same container inherits the old one's hidden values.
