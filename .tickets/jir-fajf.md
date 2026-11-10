---
id: jir-fajf
status: closed
deps: []
links: []
created: 2026-11-10T10:23:36Z
type: bug
priority: 2
assignee: Filip Wojciechowski
parent: jir-pskt
---
# Unprotected rich nodes change form on write-back without a degraded report

Design 1.4 and the Decisions say emoji, status, date and inlineCard keep their readable rendering and come back as text or links, 'reported as degraded'. The documentation pass found no degraded issue for an emoji, on read or on re-attach: an unchanged description write turns an emoji node into text silently. Decide per node type: report degraded on read (noisy on every view), report on the write that changes them (Reattach knows the original), or protect them with tokens like mentions. status and date carry data text cannot (colour, timestamp); emoji carries an id.

## Acceptance Criteria

For each of emoji, status, date, inlineCard: an unchanged view-then-update either restores the original node or reports the change; the choice is recorded in the design doc. Tests first, in external test packages with t.Parallel(). make validate passes on the tree that becomes main.


## Notes

**2026-11-10T10:49:31Z**

Merged to main in 5254b46 after a clean second-opinion review and a green gate. status and date are protected tokens; an inlineCard is restored when its link comes back unchanged and the match is unambiguous, else reported degraded on the write; emoji are reported degraded on the write (not restored). An emoji with neither text nor shortName is dropped on read. placeholder nodes are still dropped silently (editor-only).
