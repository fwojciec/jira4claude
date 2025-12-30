package jira4claude

// Config holds the application configuration.
type Config struct {
	// Server is the Jira server URL (e.g., "https://example.atlassian.net").
	Server string

	// Project is the default Jira project key (e.g., "J4C").
	Project string
}
