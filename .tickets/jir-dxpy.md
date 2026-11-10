---
id: jir-dxpy
status: open
deps: []
links: []
created: 2026-11-10T11:30:36Z
type: bug
priority: 3
assignee: Filip Wojciechowski
parent: jir-vqab
---
# Flaky: TestAttachmentService_Download 'redirects are capped' once saw 11 requests instead of 6

Seen once in a full go test ./... run, not reproduced in 30 targeted runs. Probably Stream retrying a whole redirect chain after a transient transport error (connection reuse), so the cap holds per attempt but the count doubles. Decide whether a redirect-cap failure can ever be preceded by a retry, and make the test assert what is guaranteed.

## Acceptance Criteria

The cause is identified; the test passes 200 runs under -race; if Stream can retry a chain, the documented cap says per attempt.

