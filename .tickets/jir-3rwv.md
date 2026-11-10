---
id: jir-3rwv
status: closed
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


## Notes

**2026-11-10T16:37:51Z**

Merged to main at 4d9be07. 1: premise did not reproduce; the append dry-run text is what issue view prints after the write, pinned by a test. 2: inline text flags refuse invalid UTF-8 (breaking, in the changelog). 3: the test validator was exponential in depth (77 s at 20 levels); now a linear verdict with a bounded report. 4: comment usage reads as one of --body and --body-file.
