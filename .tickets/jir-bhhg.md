---
id: jir-bhhg
status: closed
deps: [jir-vlh4]
links: []
created: 2026-11-10T07:47:17Z
type: task
priority: 2
assignee: Filip Wojciechowski
parent: jir-vqab
---
# Attachment type, service, list command and view section

Design Milestone 3. Attachment{ID, Filename, MimeType, Size, Author, Created} and AttachmentService in the root; http implementation reading the issue's attachment field; mock. 'attachment list KEY' and an attachments section in issue view (ID, filename, size, type).

## Acceptance Criteria

list in text and --json; view shows the section only when there are attachments; httptest covers the field parsing. Tests first, in external test packages with t.Parallel(). make validate passes on the tree that becomes main.


## Notes

**2026-11-10T09:03:17Z**

Merged to main in 2120e2c after a clean second-opinion review and a green gate. API field names from Atlassian's OpenAPI document (not a live site); the parser accepts id as string or number and created as timestamp or epoch milliseconds. For download: DoRequest buffers the body and forces Accept: application/json, so a streaming path is needed; handle the 303 under the credentials rule. Text-mode filenames are escaped Go-style; match --name against the exact name.
