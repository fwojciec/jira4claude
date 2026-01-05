package jira4claude_test

import (
	"testing"
	"time"

	"github.com/fwojciec/jira4claude"
	"github.com/fwojciec/jira4claude/mock"
	"github.com/stretchr/testify/assert"
)

func TestToIssueView(t *testing.T) {
	t.Parallel()

	t.Run("converts ADF description to markdown", func(t *testing.T) {
		t.Parallel()

		conv := &mock.Converter{
			ToMarkdownFn: func(adf jira4claude.ADF) (string, []string) {
				return "# Hello World", nil
			},
		}

		issue := &jira4claude.Issue{
			Key:     "TEST-1",
			Summary: "Test issue",
			Status:  "To Do",
			Type:    "Task",
			Description: jira4claude.ADF{
				"type":    "doc",
				"version": 1,
			},
			Created: time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
			Updated: time.Date(2024, 1, 2, 12, 0, 0, 0, time.UTC),
		}

		var warnings []string
		view := jira4claude.ToIssueView(issue, conv, func(w string) { warnings = append(warnings, w) }, "https://test.atlassian.net")

		assert.Equal(t, "TEST-1", view.Key)
		assert.Equal(t, "Test issue", view.Summary)
		assert.Equal(t, "# Hello World", view.Description)
		assert.Empty(t, warnings)
	})

	t.Run("propagates conversion warnings", func(t *testing.T) {
		t.Parallel()

		conv := &mock.Converter{
			ToMarkdownFn: func(adf jira4claude.ADF) (string, []string) {
				return "text", []string{"unsupported element: emoji", "unknown node type"}
			},
		}

		issue := &jira4claude.Issue{
			Key:     "TEST-1",
			Summary: "Test issue",
			Status:  "To Do",
			Type:    "Task",
			Description: jira4claude.ADF{
				"type": "doc",
			},
			Created: time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
			Updated: time.Date(2024, 1, 2, 12, 0, 0, 0, time.UTC),
		}

		var warnings []string
		_ = jira4claude.ToIssueView(issue, conv, func(w string) { warnings = append(warnings, w) }, "")

		assert.Equal(t, []string{"unsupported element: emoji", "unknown node type"}, warnings)
	})

	t.Run("converts comment bodies to markdown", func(t *testing.T) {
		t.Parallel()

		conv := &mock.Converter{
			ToMarkdownFn: func(adf jira4claude.ADF) (string, []string) {
				if text, ok := adf["text"].(string); ok {
					return text + " (converted)", nil
				}
				return "", nil
			},
		}

		issue := &jira4claude.Issue{
			Key:     "TEST-1",
			Summary: "Test issue",
			Status:  "To Do",
			Type:    "Task",
			Comments: []*jira4claude.Comment{
				{
					ID:      "10001",
					Author:  &jira4claude.User{DisplayName: "John Doe"},
					Body:    jira4claude.ADF{"text": "comment body"},
					Created: time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
				},
			},
			Created: time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
			Updated: time.Date(2024, 1, 2, 12, 0, 0, 0, time.UTC),
		}

		var warnings []string
		view := jira4claude.ToIssueView(issue, conv, func(w string) { warnings = append(warnings, w) }, "")

		assert.Len(t, view.Comments, 1)
		assert.Equal(t, "comment body (converted)", view.Comments[0].Body)
		assert.Equal(t, "John Doe", view.Comments[0].Author)
	})

	t.Run("propagates comment conversion warnings", func(t *testing.T) {
		t.Parallel()

		conv := &mock.Converter{
			ToMarkdownFn: func(adf jira4claude.ADF) (string, []string) {
				return "text", []string{"comment warning"}
			},
		}

		issue := &jira4claude.Issue{
			Key:     "TEST-1",
			Summary: "Test issue",
			Status:  "To Do",
			Type:    "Task",
			Comments: []*jira4claude.Comment{
				{
					ID:      "10001",
					Body:    jira4claude.ADF{"type": "doc"},
					Created: time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
				},
			},
			Created: time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
			Updated: time.Date(2024, 1, 2, 12, 0, 0, 0, time.UTC),
		}

		var warnings []string
		_ = jira4claude.ToIssueView(issue, conv, func(w string) { warnings = append(warnings, w) }, "")

		// Description warning + comment warning
		assert.Contains(t, warnings, "comment warning")
	})

	t.Run("handles nil description", func(t *testing.T) {
		t.Parallel()

		conv := &mock.Converter{
			ToMarkdownFn: func(adf jira4claude.ADF) (string, []string) {
				return "", nil
			},
		}

		issue := &jira4claude.Issue{
			Key:         "TEST-1",
			Summary:     "Test issue",
			Status:      "To Do",
			Type:        "Task",
			Description: nil,
			Created:     time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
			Updated:     time.Date(2024, 1, 2, 12, 0, 0, 0, time.UTC),
		}

		var warnings []string
		view := jira4claude.ToIssueView(issue, conv, func(w string) { warnings = append(warnings, w) }, "")

		assert.Empty(t, view.Description)
	})

	t.Run("converts links to RelatedIssues", func(t *testing.T) {
		t.Parallel()

		conv := &mock.Converter{
			ToMarkdownFn: func(adf jira4claude.ADF) (string, []string) {
				return "", nil
			},
		}

		issue := &jira4claude.Issue{
			Key:     "TEST-1",
			Summary: "Test issue",
			Status:  "To Do",
			Type:    "Task",
			Links: []*jira4claude.IssueLink{
				{
					Type: jira4claude.IssueLinkType{
						Name:    "Blocks",
						Outward: "blocks",
						Inward:  "is blocked by",
					},
					OutwardIssue: &jira4claude.LinkedIssue{
						Key:     "TEST-2",
						Summary: "Blocked issue",
						Status:  "To Do",
						Type:    "Task",
					},
				},
				{
					Type: jira4claude.IssueLinkType{
						Name:    "Blocks",
						Outward: "blocks",
						Inward:  "is blocked by",
					},
					InwardIssue: &jira4claude.LinkedIssue{
						Key:     "TEST-3",
						Summary: "Blocking issue",
						Status:  "Done",
						Type:    "Bug",
					},
				},
			},
			Created: time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
			Updated: time.Date(2024, 1, 2, 12, 0, 0, 0, time.UTC),
		}

		var warnings []string
		view := jira4claude.ToIssueView(issue, conv, func(w string) { warnings = append(warnings, w) }, "")

		assert.Len(t, view.RelatedIssues, 2)
		// Outward link (blocks) comes first
		assert.Equal(t, "blocks", view.RelatedIssues[0].Relationship)
		assert.Equal(t, "TEST-2", view.RelatedIssues[0].Key)
		assert.Equal(t, "Task", view.RelatedIssues[0].Type)
		assert.Equal(t, "Blocked issue", view.RelatedIssues[0].Summary)
		assert.Equal(t, "To Do", view.RelatedIssues[0].Status)
		// Inward link (is blocked by) comes second
		assert.Equal(t, "is blocked by", view.RelatedIssues[1].Relationship)
		assert.Equal(t, "TEST-3", view.RelatedIssues[1].Key)
		assert.Equal(t, "Bug", view.RelatedIssues[1].Type)
		assert.Equal(t, "Done", view.RelatedIssues[1].Status)
	})

	t.Run("includes URL when server URL is provided", func(t *testing.T) {
		t.Parallel()

		conv := &mock.Converter{
			ToMarkdownFn: func(adf jira4claude.ADF) (string, []string) {
				return "", nil
			},
		}

		issue := &jira4claude.Issue{
			Key:     "TEST-1",
			Summary: "Test issue",
			Status:  "To Do",
			Type:    "Task",
			Created: time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
			Updated: time.Date(2024, 1, 2, 12, 0, 0, 0, time.UTC),
		}

		view := jira4claude.ToIssueView(issue, conv, func(w string) {}, "https://test.atlassian.net")

		assert.Equal(t, "https://test.atlassian.net/browse/TEST-1", view.URL)
	})

	t.Run("formats timestamps as RFC3339", func(t *testing.T) {
		t.Parallel()

		conv := &mock.Converter{
			ToMarkdownFn: func(adf jira4claude.ADF) (string, []string) {
				return "", nil
			},
		}

		issue := &jira4claude.Issue{
			Key:     "TEST-1",
			Summary: "Test issue",
			Status:  "To Do",
			Type:    "Task",
			Created: time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
			Updated: time.Date(2024, 1, 16, 14, 45, 0, 0, time.UTC),
		}

		view := jira4claude.ToIssueView(issue, conv, func(w string) {}, "")

		assert.Equal(t, "2024-01-15T10:30:00Z", view.Created)
		assert.Equal(t, "2024-01-16T14:45:00Z", view.Updated)
	})

	t.Run("includes optional fields when present", func(t *testing.T) {
		t.Parallel()

		conv := &mock.Converter{
			ToMarkdownFn: func(adf jira4claude.ADF) (string, []string) {
				return "", nil
			},
		}

		issue := &jira4claude.Issue{
			Key:      "TEST-1",
			Project:  "TEST",
			Summary:  "Test issue",
			Status:   "To Do",
			Type:     "Task",
			Priority: "High",
			Assignee: &jira4claude.User{DisplayName: "John Doe"},
			Reporter: &jira4claude.User{DisplayName: "Jane Smith"},
			Labels:   []string{"bug", "urgent"},
			Parent: &jira4claude.LinkedIssue{
				Key:     "TEST-100",
				Summary: "Parent issue",
				Status:  "In Progress",
				Type:    "Epic",
			},
			Created: time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
			Updated: time.Date(2024, 1, 2, 12, 0, 0, 0, time.UTC),
		}

		view := jira4claude.ToIssueView(issue, conv, func(w string) {}, "")

		assert.Equal(t, "TEST", view.Project)
		assert.Equal(t, "High", view.Priority)
		assert.Equal(t, "John Doe", view.Assignee)
		assert.Equal(t, "Jane Smith", view.Reporter)
		assert.Equal(t, []string{"bug", "urgent"}, view.Labels)
		// Parent is now in RelatedIssues
		assert.Len(t, view.RelatedIssues, 1)
		assert.Equal(t, "parent", view.RelatedIssues[0].Relationship)
		assert.Equal(t, "TEST-100", view.RelatedIssues[0].Key)
	})

	t.Run("returns empty RelatedIssues when issue has no parent", func(t *testing.T) {
		t.Parallel()

		conv := &mock.Converter{
			ToMarkdownFn: func(adf jira4claude.ADF) (string, []string) {
				return "", nil
			},
		}

		issue := &jira4claude.Issue{
			Key:     "TEST-1",
			Summary: "Test issue",
			Status:  "To Do",
			Type:    "Task",
			Parent:  nil, // Explicitly nil
			Created: time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
			Updated: time.Date(2024, 1, 2, 12, 0, 0, 0, time.UTC),
		}

		view := jira4claude.ToIssueView(issue, conv, func(w string) {}, "")

		assert.Empty(t, view.RelatedIssues)
	})
}

func TestToIssuesView(t *testing.T) {
	t.Parallel()

	t.Run("converts multiple issues", func(t *testing.T) {
		t.Parallel()

		conv := &mock.Converter{
			ToMarkdownFn: func(adf jira4claude.ADF) (string, []string) {
				return "", nil
			},
		}

		issues := []*jira4claude.Issue{
			{
				Key:     "TEST-1",
				Summary: "First issue",
				Status:  "To Do",
				Type:    "Task",
				Created: time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
				Updated: time.Date(2024, 1, 2, 12, 0, 0, 0, time.UTC),
			},
			{
				Key:     "TEST-2",
				Summary: "Second issue",
				Status:  "Done",
				Type:    "Bug",
				Created: time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
				Updated: time.Date(2024, 1, 2, 12, 0, 0, 0, time.UTC),
			},
		}

		var warnings []string
		views := jira4claude.ToIssuesView(issues, conv, func(w string) { warnings = append(warnings, w) }, "")

		assert.Len(t, views, 2)
		assert.Equal(t, "TEST-1", views[0].Key)
		assert.Equal(t, "TEST-2", views[1].Key)
	})
}

func TestToCommentView(t *testing.T) {
	t.Parallel()

	t.Run("converts comment body to markdown", func(t *testing.T) {
		t.Parallel()

		conv := &mock.Converter{
			ToMarkdownFn: func(adf jira4claude.ADF) (string, []string) {
				return "**bold** text", nil
			},
		}

		comment := &jira4claude.Comment{
			ID:      "10001",
			Author:  &jira4claude.User{DisplayName: "John Doe"},
			Body:    jira4claude.ADF{"type": "doc"},
			Created: time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
		}

		var warnings []string
		view := jira4claude.ToCommentView(comment, conv, func(w string) { warnings = append(warnings, w) })

		assert.Equal(t, "10001", view.ID)
		assert.Equal(t, "John Doe", view.Author)
		assert.Equal(t, "**bold** text", view.Body)
		assert.Equal(t, "2024-01-15T10:30:00Z", view.Created)
		assert.Empty(t, warnings)
	})

	t.Run("propagates conversion warnings", func(t *testing.T) {
		t.Parallel()

		conv := &mock.Converter{
			ToMarkdownFn: func(adf jira4claude.ADF) (string, []string) {
				return "text", []string{"unsupported node type", "emoji not supported"}
			},
		}

		comment := &jira4claude.Comment{
			ID:      "10001",
			Author:  &jira4claude.User{DisplayName: "John Doe"},
			Body:    jira4claude.ADF{"type": "doc"},
			Created: time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
		}

		var warnings []string
		_ = jira4claude.ToCommentView(comment, conv, func(w string) { warnings = append(warnings, w) })

		assert.Equal(t, []string{"unsupported node type", "emoji not supported"}, warnings)
	})

	t.Run("handles nil author", func(t *testing.T) {
		t.Parallel()

		conv := &mock.Converter{
			ToMarkdownFn: func(adf jira4claude.ADF) (string, []string) {
				return "text", nil
			},
		}

		comment := &jira4claude.Comment{
			ID:      "10001",
			Author:  nil,
			Body:    jira4claude.ADF{"type": "doc"},
			Created: time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
		}

		view := jira4claude.ToCommentView(comment, conv, func(w string) {})

		assert.Empty(t, view.Author)
	})
}

func TestToLinksView(t *testing.T) {
	t.Parallel()

	t.Run("converts outward and inward links to RelatedIssueView", func(t *testing.T) {
		t.Parallel()

		links := []*jira4claude.IssueLink{
			{
				Type: jira4claude.IssueLinkType{
					Name:    "Blocks",
					Outward: "blocks",
					Inward:  "is blocked by",
				},
				OutwardIssue: &jira4claude.LinkedIssue{
					Key:     "TEST-2",
					Summary: "Blocked issue",
					Status:  "To Do",
					Type:    "Task",
				},
			},
			{
				Type: jira4claude.IssueLinkType{
					Name:    "Blocks",
					Outward: "blocks",
					Inward:  "is blocked by",
				},
				InwardIssue: &jira4claude.LinkedIssue{
					Key:     "TEST-3",
					Summary: "Blocking issue",
					Status:  "Done",
					Type:    "Bug",
				},
			},
		}

		views := jira4claude.ToLinksView(links)

		assert.Len(t, views, 2)
		// Outward links (blocks) come first
		assert.Equal(t, "blocks", views[0].Relationship)
		assert.Equal(t, "TEST-2", views[0].Key)
		assert.Equal(t, "Task", views[0].Type)
		assert.Equal(t, "To Do", views[0].Status)
		// Inward links (is blocked by) come second
		assert.Equal(t, "is blocked by", views[1].Relationship)
		assert.Equal(t, "TEST-3", views[1].Key)
		assert.Equal(t, "Bug", views[1].Type)
		assert.Equal(t, "Done", views[1].Status)
	})

	t.Run("handles empty links", func(t *testing.T) {
		t.Parallel()

		views := jira4claude.ToLinksView([]*jira4claude.IssueLink{})

		assert.Empty(t, views)
	})
}

func TestToIssueView_Subtasks(t *testing.T) {
	t.Parallel()

	t.Run("converts subtasks to RelatedIssues", func(t *testing.T) {
		t.Parallel()

		conv := &mock.Converter{
			ToMarkdownFn: func(adf jira4claude.ADF) (string, []string) {
				return "", nil
			},
		}

		issue := &jira4claude.Issue{
			Key:     "TEST-1",
			Summary: "Parent issue",
			Status:  "In Progress",
			Type:    "Task",
			Subtasks: []*jira4claude.LinkedIssue{
				{
					Key:     "TEST-2",
					Summary: "First subtask",
					Status:  "Done",
					Type:    "Sub-task",
				},
				{
					Key:     "TEST-3",
					Summary: "Second subtask",
					Status:  "To Do",
					Type:    "Sub-task",
				},
			},
			Created: time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
			Updated: time.Date(2024, 1, 2, 12, 0, 0, 0, time.UTC),
		}

		view := jira4claude.ToIssueView(issue, conv, func(w string) {}, "")

		assert.Len(t, view.RelatedIssues, 2)
		assert.Equal(t, "subtask", view.RelatedIssues[0].Relationship)
		assert.Equal(t, "TEST-2", view.RelatedIssues[0].Key)
		assert.Equal(t, "First subtask", view.RelatedIssues[0].Summary)
		assert.Equal(t, "Done", view.RelatedIssues[0].Status)
		assert.Equal(t, "subtask", view.RelatedIssues[1].Relationship)
		assert.Equal(t, "TEST-3", view.RelatedIssues[1].Key)
		assert.Equal(t, "Second subtask", view.RelatedIssues[1].Summary)
		assert.Equal(t, "To Do", view.RelatedIssues[1].Status)
	})

	t.Run("handles no subtasks", func(t *testing.T) {
		t.Parallel()

		conv := &mock.Converter{
			ToMarkdownFn: func(adf jira4claude.ADF) (string, []string) {
				return "", nil
			},
		}

		issue := &jira4claude.Issue{
			Key:     "TEST-1",
			Summary: "Issue without subtasks",
			Status:  "To Do",
			Type:    "Task",
			Created: time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
			Updated: time.Date(2024, 1, 2, 12, 0, 0, 0, time.UTC),
		}

		view := jira4claude.ToIssueView(issue, conv, func(w string) {}, "")

		assert.Empty(t, view.RelatedIssues)
	})
}

func TestToRelatedIssuesView(t *testing.T) {
	t.Parallel()

	t.Run("returns empty slice when no relationships exist", func(t *testing.T) {
		t.Parallel()

		issue := &jira4claude.Issue{
			Key:     "TEST-1",
			Summary: "Standalone issue",
			Status:  "To Do",
			Type:    "Task",
		}

		related := jira4claude.ToRelatedIssuesView(issue)

		assert.Empty(t, related)
	})

	t.Run("includes parent relationship", func(t *testing.T) {
		t.Parallel()

		issue := &jira4claude.Issue{
			Key:     "TEST-2",
			Summary: "Subtask",
			Status:  "To Do",
			Type:    "Sub-task",
			Parent: &jira4claude.LinkedIssue{
				Key:     "TEST-1",
				Summary: "Parent task",
				Status:  "In Progress",
				Type:    "Task",
			},
		}

		related := jira4claude.ToRelatedIssuesView(issue)

		assert.Len(t, related, 1)
		assert.Equal(t, "parent", related[0].Relationship)
		assert.Equal(t, "TEST-1", related[0].Key)
		assert.Equal(t, "Task", related[0].Type)
		assert.Equal(t, "In Progress", related[0].Status)
		assert.Equal(t, "Parent task", related[0].Summary)
	})

	t.Run("includes subtask relationships", func(t *testing.T) {
		t.Parallel()

		issue := &jira4claude.Issue{
			Key:     "TEST-1",
			Summary: "Parent task",
			Status:  "In Progress",
			Type:    "Task",
			Subtasks: []*jira4claude.LinkedIssue{
				{
					Key:     "TEST-2",
					Summary: "First subtask",
					Status:  "Done",
					Type:    "Sub-task",
				},
			},
		}

		related := jira4claude.ToRelatedIssuesView(issue)

		assert.Len(t, related, 1)
		assert.Equal(t, "subtask", related[0].Relationship)
		assert.Equal(t, "TEST-2", related[0].Key)
		assert.Equal(t, "Sub-task", related[0].Type)
		assert.Equal(t, "Done", related[0].Status)
		assert.Equal(t, "First subtask", related[0].Summary)
	})

	t.Run("includes blocks relationship from outward links", func(t *testing.T) {
		t.Parallel()

		issue := &jira4claude.Issue{
			Key:     "TEST-1",
			Summary: "Blocking issue",
			Status:  "In Progress",
			Type:    "Task",
			Links: []*jira4claude.IssueLink{
				{
					Type: jira4claude.IssueLinkType{
						Name:    "Blocks",
						Outward: "blocks",
						Inward:  "is blocked by",
					},
					OutwardIssue: &jira4claude.LinkedIssue{
						Key:     "TEST-2",
						Summary: "Blocked issue",
						Status:  "To Do",
						Type:    "Task",
					},
				},
			},
		}

		related := jira4claude.ToRelatedIssuesView(issue)

		assert.Len(t, related, 1)
		assert.Equal(t, "blocks", related[0].Relationship)
		assert.Equal(t, "TEST-2", related[0].Key)
		assert.Equal(t, "Task", related[0].Type)
		assert.Equal(t, "To Do", related[0].Status)
		assert.Equal(t, "Blocked issue", related[0].Summary)
	})

	t.Run("includes is blocked by relationship from inward links", func(t *testing.T) {
		t.Parallel()

		issue := &jira4claude.Issue{
			Key:     "TEST-1",
			Summary: "Blocked issue",
			Status:  "To Do",
			Type:    "Task",
			Links: []*jira4claude.IssueLink{
				{
					Type: jira4claude.IssueLinkType{
						Name:    "Blocks",
						Outward: "blocks",
						Inward:  "is blocked by",
					},
					InwardIssue: &jira4claude.LinkedIssue{
						Key:     "TEST-2",
						Summary: "Blocking issue",
						Status:  "In Progress",
						Type:    "Bug",
					},
				},
			},
		}

		related := jira4claude.ToRelatedIssuesView(issue)

		assert.Len(t, related, 1)
		assert.Equal(t, "is blocked by", related[0].Relationship)
		assert.Equal(t, "TEST-2", related[0].Key)
		assert.Equal(t, "Bug", related[0].Type)
		assert.Equal(t, "In Progress", related[0].Status)
		assert.Equal(t, "Blocking issue", related[0].Summary)
	})

	t.Run("orders relationships: parent, subtask, blocks, is blocked by", func(t *testing.T) {
		t.Parallel()

		issue := &jira4claude.Issue{
			Key:     "TEST-1",
			Summary: "Complex issue",
			Status:  "In Progress",
			Type:    "Task",
			Parent: &jira4claude.LinkedIssue{
				Key:     "TEST-PARENT",
				Summary: "Parent",
				Status:  "In Progress",
				Type:    "Epic",
			},
			Subtasks: []*jira4claude.LinkedIssue{
				{
					Key:     "TEST-SUBTASK",
					Summary: "Subtask",
					Status:  "Done",
					Type:    "Sub-task",
				},
			},
			Links: []*jira4claude.IssueLink{
				{
					Type: jira4claude.IssueLinkType{
						Name:    "Blocks",
						Outward: "blocks",
						Inward:  "is blocked by",
					},
					InwardIssue: &jira4claude.LinkedIssue{
						Key:     "TEST-BLOCKER",
						Summary: "Blocker",
						Status:  "In Progress",
						Type:    "Bug",
					},
				},
				{
					Type: jira4claude.IssueLinkType{
						Name:    "Blocks",
						Outward: "blocks",
						Inward:  "is blocked by",
					},
					OutwardIssue: &jira4claude.LinkedIssue{
						Key:     "TEST-BLOCKED",
						Summary: "Blocked",
						Status:  "To Do",
						Type:    "Task",
					},
				},
			},
		}

		related := jira4claude.ToRelatedIssuesView(issue)

		assert.Len(t, related, 4)
		assert.Equal(t, "parent", related[0].Relationship)
		assert.Equal(t, "TEST-PARENT", related[0].Key)
		assert.Equal(t, "subtask", related[1].Relationship)
		assert.Equal(t, "TEST-SUBTASK", related[1].Key)
		assert.Equal(t, "blocks", related[2].Relationship)
		assert.Equal(t, "TEST-BLOCKED", related[2].Key)
		assert.Equal(t, "is blocked by", related[3].Relationship)
		assert.Equal(t, "TEST-BLOCKER", related[3].Key)
	})
}
