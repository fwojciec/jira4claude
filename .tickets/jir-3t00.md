---
id: jir-3t00
status: closed
deps: [jir-bhhg, jir-mi82]
links: []
created: 2026-11-10T07:47:17Z
type: task
priority: 2
assignee: Filip Wojciechowski
parent: jir-vqab
---
# attachment upload

Design Milestone 3. attachment upload KEY FILE...: multipart POST with X-Atlassian-Token: no-check; prints the new IDs as a MutationResult. Upload is a POST, so the retry policy of design 1.5 applies: retried on 429 only, never after a 5xx; the body must be rewindable.

## Acceptance Criteria

Multipart body verified in httptest; several files in one call; a 5xx is not retried and the error says the upload may have been applied; a missing local file fails before any request. Tests first, in external test packages with t.Parallel(). make validate passes on the tree that becomes main.


## Notes

**2026-11-10T10:01:47Z**

Merged to main in cc3646d after a clean second-opinion review and a green gate. One request per file, streamed with an exact Content-Length; 429 retried by re-opening the file; 5xx and connection errors not retried (may have been applied); redirects not followed; 413 is a validation error on every command. API facts from Atlassian's OpenAPI document; unverified on a live site: non-chunked upload, empty files, duplicate names, MIME type set by Jira.
