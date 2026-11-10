---
id: jir-6ov5
status: closed
deps: []
links: []
created: 2026-11-10T11:30:36Z
type: bug
priority: 3
assignee: Filip Wojciechowski
parent: jir-5662
---
# netrc lookup uses host:port, so a Jira on a non-default port needs the port in .netrc

The client looks up the netrc machine as host:port when the server URL has a port. curl and other tools match the hostname only, so a self-hosted Jira on a non-default port finds no credentials unless .netrc names host:port. Jira Cloud is unaffected. Found by the end-to-end test (jir-0bgh).

## Acceptance Criteria

A server URL with a port finds a netrc entry for the bare hostname when none exists for host:port; the more specific entry wins; README says so; credentials are still sent only to the configured scheme, host and port.


## Notes

**2026-11-10T16:37:51Z**

Merged to main at 4d9be07. netrc lookup tries host:port, then the bare host name; the error names both and the file.
