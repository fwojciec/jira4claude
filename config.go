package jira4claude

import "time"

// Config holds the application configuration.
type Config struct {
	// Server is the Jira server URL (e.g., "https://example.atlassian.net").
	Server string

	// Project is the default Jira project key (e.g., "J4C").
	Project string

	// Timeout bounds each request's wait on Jira: the whole answer of an
	// API call, and the longest silence of an attachment transfer. Zero
	// means no bound. It comes from the --timeout flag, not the file.
	Timeout time.Duration
}
