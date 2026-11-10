---
id: jir-wd4f
status: open
deps: []
links: []
created: 2026-11-10T12:08:27Z
type: bug
priority: 3
assignee: Filip Wojciechowski
parent: jir-pskt
---
# Rewrite restore: three edge cases from the second Codex round

Found by Codex on h44x-rewrite-property after 9d139c2; none is a regression.
1. A paragraph holding annotated text and an inline card: appending a plain link with the card's URL in another paragraph makes card planning restore the reference's card but leave the edit's as a link, so the unchanged paragraph fails comparison, its annotation is reported dropped and the update is refused (markdown/protect.go ~600). False refusal.
2. Known node types protected only because of where they sit (protectMisplaced, e.g. paragraphs directly inside a bullet list) still have their localIds left out of the comparison key (markdown/restore.go ~353, readerKnows goes by type): swapping two such tokens that differ only in localId is undone with no issue. Protection must follow token provenance, not node type.
3. A list item holding an empty paragraph, then two paragraphs: editing the last one restores the empty paragraph but rereading still warns degraded listItem 'is left out' (markdown/to_markdown.go ~373). False warning.

## Acceptance Criteria

A failing test for each case first; case 1 writes with no dropped issue and keeps the annotation; case 2 returns the swapped order; case 3 gives no warning; inventory, property and fuzz targets still pass.

