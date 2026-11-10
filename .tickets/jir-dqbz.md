---
id: jir-dqbz
status: open
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

