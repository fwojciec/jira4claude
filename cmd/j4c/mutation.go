package main

import "github.com/fwojciec/jira4claude"

// newResult starts the result of a write to the issue key, with its URL on
// the configured server.
func newResult(cfg *jira4claude.Config, key, action string) jira4claude.MutationResult {
	var server string
	if cfg != nil {
		server = cfg.Server
	}
	return jira4claude.MutationResult{Key: key, URL: jira4claude.IssueURL(server, key), Action: action}
}
