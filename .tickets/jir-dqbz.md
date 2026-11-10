---
id: jir-dqbz
status: closed
deps: []
links: []
created: 2026-11-10T11:30:36Z
type: bug
priority: 3
assignee: Filip Wojciechowski
parent: jir-5662
---
# comment and create: an applied write whose 2xx answer cannot be parsed prints nothing

issue update now reports an applied write whose read-back failed (ReadBackError). issue comment and issue create still end with empty stdout and a non-zero exit when Jira answers 2xx with a body the client cannot parse, although the write was applied; a retry duplicates the comment or issue.

## Acceptance Criteria

Both commands report the write as applied, with what is known, and say not to repeat it; tests at the HTTP and CLI level.


## Notes

**2026-11-10T16:37:51Z**

Merged to main at 4d9be07. New UnreadableAnswerError: create and comment answered 2xx with an unreadable body print the result with what is known and say the write was applied; any 2xx counts as success; writes with no answer body treat an unreadable 2xx body as success. Codex finding (raw project override named wrongly) fixed. Unchanged: a single-file attachment upload with an unreadable answer prints nothing on stdout, its error says to check attachment list.
