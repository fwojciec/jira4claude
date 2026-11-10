---
id: jir-u77o
status: closed
deps: [jir-vlh4]
links: []
created: 2026-11-10T07:47:17Z
type: task
priority: 1
assignee: Filip Wojciechowski
parent: jir-5662
---
# File and stdin input: --description-file, --body-file

Design 2.3 (F17). --description-file PATH and --body-file PATH on create, update and comment; '-' means stdin (Run already takes stdin). Mutually exclusive with the inline flags. Applies to --append-description too (decide the flag name).

## Acceptance Criteria

File and stdin input produce the same write as the inline flag; both given is a validation error; a missing file is a validation error naming the path; an empty file follows the same rule as an empty inline value; trailing newline handling is stated and tested. Tests first, in external test packages with t.Parallel(). make validate passes on the tree that becomes main.


## Notes

**2026-11-10T09:43:08Z**

Merged to main in c4fab74. --description-file, --append-description-file, --body-file; '-' is stdin. One trailing newline and a leading BOM are stripped; invalid UTF-8 is refused; 1 MiB cap (Jira's 32,767 limit is not enforced: what it counts is unverified). A blank inline or file value counts as empty; comment --body '' is now an error.
