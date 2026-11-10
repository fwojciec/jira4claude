---
id: jir-bhhg
status: open
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

