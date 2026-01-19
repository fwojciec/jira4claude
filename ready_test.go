package jira4claude_test

import (
	"testing"

	"github.com/fwojciec/jira4claude"
	"github.com/stretchr/testify/assert"
)

func TestIsReady(t *testing.T) {
	t.Parallel()

	t.Run("issue with no links is ready", func(t *testing.T) {
		t.Parallel()
		issue := &jira4claude.Issue{
			Key:    "TEST-1",
			Status: "To Do",
			Links:  nil,
		}
		assert.True(t, jira4claude.IsReady(issue))
	})

	t.Run("issue with all blockers Done is ready", func(t *testing.T) {
		t.Parallel()
		issue := &jira4claude.Issue{
			Key:    "TEST-2",
			Status: "To Do",
			Links: []*jira4claude.IssueLink{
				{
					Type: jira4claude.IssueLinkType{
						Name:   "Blocks",
						Inward: "is blocked by",
					},
					InwardIssue: &jira4claude.LinkedIssue{
						Key:    "TEST-1",
						Status: "Done",
					},
				},
			},
		}
		assert.True(t, jira4claude.IsReady(issue))
	})

	t.Run("issue with open blocker is not ready", func(t *testing.T) {
		t.Parallel()
		issue := &jira4claude.Issue{
			Key:    "TEST-3",
			Status: "To Do",
			Links: []*jira4claude.IssueLink{
				{
					Type: jira4claude.IssueLinkType{
						Name:   "Blocks",
						Inward: "is blocked by",
					},
					InwardIssue: &jira4claude.LinkedIssue{
						Key:    "TEST-1",
						Status: "In Progress",
					},
				},
			},
		}
		assert.False(t, jira4claude.IsReady(issue))
	})

	t.Run("issue with mixed blockers where one is open is not ready", func(t *testing.T) {
		t.Parallel()
		issue := &jira4claude.Issue{
			Key:    "TEST-4",
			Status: "To Do",
			Links: []*jira4claude.IssueLink{
				{
					Type: jira4claude.IssueLinkType{
						Name:   "Blocks",
						Inward: "is blocked by",
					},
					InwardIssue: &jira4claude.LinkedIssue{
						Key:    "TEST-1",
						Status: "Done",
					},
				},
				{
					Type: jira4claude.IssueLinkType{
						Name:   "Blocks",
						Inward: "is blocked by",
					},
					InwardIssue: &jira4claude.LinkedIssue{
						Key:    "TEST-2",
						Status: "To Do",
					},
				},
			},
		}
		assert.False(t, jira4claude.IsReady(issue))
	})

	t.Run("outward links do not affect readiness", func(t *testing.T) {
		t.Parallel()
		issue := &jira4claude.Issue{
			Key:    "TEST-5",
			Status: "To Do",
			Links: []*jira4claude.IssueLink{
				{
					Type: jira4claude.IssueLinkType{
						Name:    "Blocks",
						Outward: "blocks",
					},
					OutwardIssue: &jira4claude.LinkedIssue{
						Key:    "TEST-1",
						Status: "To Do",
					},
				},
			},
		}
		assert.True(t, jira4claude.IsReady(issue))
	})

	t.Run("non-blocking link types do not affect readiness", func(t *testing.T) {
		t.Parallel()
		issue := &jira4claude.Issue{
			Key:    "TEST-6",
			Status: "To Do",
			Links: []*jira4claude.IssueLink{
				{
					Type: jira4claude.IssueLinkType{
						Name:   "Relates",
						Inward: "relates to",
					},
					InwardIssue: &jira4claude.LinkedIssue{
						Key:    "TEST-1",
						Status: "To Do",
					},
				},
			},
		}
		assert.True(t, jira4claude.IsReady(issue))
	})

	t.Run("blocked by matching is case-insensitive", func(t *testing.T) {
		t.Parallel()
		issue := &jira4claude.Issue{
			Key:    "TEST-7",
			Status: "To Do",
			Links: []*jira4claude.IssueLink{
				{
					Type: jira4claude.IssueLinkType{
						Name:   "Blocks",
						Inward: "Is Blocked By", // Different casing
					},
					InwardIssue: &jira4claude.LinkedIssue{
						Key:    "TEST-1",
						Status: "In Progress",
					},
				},
			},
		}
		assert.False(t, jira4claude.IsReady(issue))
	})

	t.Run("issue with Won't Do status is not ready", func(t *testing.T) {
		t.Parallel()
		issue := &jira4claude.Issue{
			Key:    "TEST-8",
			Status: "Won't Do",
			Links:  nil,
		}
		assert.False(t, jira4claude.IsReady(issue))
	})

	t.Run("issue with Done status is not ready", func(t *testing.T) {
		t.Parallel()
		issue := &jira4claude.Issue{
			Key:    "TEST-9",
			Status: "Done",
			Links:  nil,
		}
		assert.False(t, jira4claude.IsReady(issue))
	})
}
