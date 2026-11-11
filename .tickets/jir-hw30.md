---
id: jir-hw30
status: open
deps: []
links: []
created: 2026-11-11T03:06:00Z
type: task
priority: 3
assignee: Filip Wojciechowski
parent: jir-pskt
---
# Rewrite: a media node goes back equal as JSON but not byte for byte

Found by the end-to-end test (jir-0bgh): an unchanged description rewrite sends media nodes that are equal to the original as JSON values but not byte-identical, because ADFNode decodes and re-encodes attributes (key order, number formatting). No loss is known; the risk is a Jira site that treats the re-encoded form differently, which only the live smoke test (jir-d2q1) can show.

## Acceptance Criteria

Either unchanged nodes are sent with their original bytes (raw passthrough for nodes the restorer did not touch), pinned by the e2e test comparing bytes, or the live smoke test shows Jira accepts the re-encoded form and this is closed with a note saying so.

