---
id: jir-mq5o
status: closed
deps: [jir-bhhg]
links: []
created: 2026-11-10T07:47:17Z
type: task
priority: 2
assignee: Filip Wojciechowski
parent: jir-vqab
---
# attachment download

Design Milestone 3 download rules. attachment download KEY (--id ID | --name NAME | --all) --dir DIR; prints one saved path per line.

## Acceptance Criteria

Only the base name of the server's filename is used and nothing is written outside --dir (path traversal names tested); an existing file is not overwritten without --overwrite; credentials are sent only to the configured host, never to a redirect target on another host (httptest with a cross-host redirect); two attachments with the same name are both saved or the clash is an error (decide). Tests first, in external test packages with t.Parallel(). make validate passes on the tree that becomes main.


## Notes

**2026-11-10T09:36:21Z**

Merged to main in 3e7aad8 after two second-opinion rounds and a green gate. Files go through fs.Dir (os.Root, base names only, hard-link placement; on file systems without hard links it copies into a file it created exclusively). Client.Stream follows redirects itself and attaches credentials only to the configured scheme, host and port; errors name hosts only, never a server-supplied URL. --all continues past per-file failures and stops early on unauthorized, forbidden and rate-limit. Unverified on a live site: that redirect=false returns the bytes directly (safe either way). No download timeout.
