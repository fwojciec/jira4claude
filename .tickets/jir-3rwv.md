---
id: jir-3rwv
status: open
deps: []
links: []
created: 2026-11-10T12:10:34Z
type: task
priority: 3
assignee: Filip Wojciechowski
parent: jir-5662
---
# Small scripted-use gaps left from the milestone review

1. issue update --append-description-file --dry-run: the 'text' preview omits existing content that has no Markdown form. 2. Inline flags (--description, --body, --summary) replace invalid UTF-8 silently, while the file flags refuse it. 3. Confirm the fuzz harness bounds the schema validator, whose anyOf matching can be exponential. 4. The issue comment usage line shows both --body and --body-file as required.

## Acceptance Criteria

Each is fixed with a test, or listed under Known limits in CHANGELOG.md with the reason.

