package jira4claude_test

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"github.com/fwojciec/jira4claude"
	"github.com/fwojciec/jira4claude/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToIssueView(t *testing.T) {
	t.Parallel()

	t.Run("converts ADF description to markdown", func(t *testing.T) {
		t.Parallel()

		conv := &mock.Converter{
			ToMarkdownFn: func(adf *jira4claude.ADFNode) (string, []jira4claude.ConversionIssue) {
				return "# Hello World", nil
			},
		}

		issue := &jira4claude.Issue{
			Key:     "TEST-1",
			Summary: "Test issue",
			Status:  "To Do",
			Type:    "Task",
			Description: &jira4claude.ADFNode{
				Type:    "doc",
				Version: 1,
			},
			Created: time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
			Updated: time.Date(2024, 1, 2, 12, 0, 0, 0, time.UTC),
		}

		var warnings []jira4claude.ConversionIssue
		view := jira4claude.ToIssueView(issue, conv, func(w jira4claude.ConversionIssue) { warnings = append(warnings, w) }, "https://test.atlassian.net")

		assert.Equal(t, "TEST-1", view.Key)
		assert.Equal(t, "Test issue", view.Summary)
		assert.Equal(t, "# Hello World", view.Description)
		assert.Empty(t, warnings)
	})

	t.Run("propagates conversion warnings", func(t *testing.T) {
		t.Parallel()

		conv := &mock.Converter{
			ToMarkdownFn: func(adf *jira4claude.ADFNode) (string, []jira4claude.ConversionIssue) {
				return "text", []jira4claude.ConversionIssue{{Kind: jira4claude.ConversionDropped, Node: "unsupported element: emoji"}, {Kind: jira4claude.ConversionDropped, Node: "unknown node type"}}
			},
		}

		issue := &jira4claude.Issue{
			Key:     "TEST-1",
			Summary: "Test issue",
			Status:  "To Do",
			Type:    "Task",
			Description: &jira4claude.ADFNode{
				Type: "doc",
			},
			Created: time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
			Updated: time.Date(2024, 1, 2, 12, 0, 0, 0, time.UTC),
		}

		var warnings []jira4claude.ConversionIssue
		_ = jira4claude.ToIssueView(issue, conv, func(w jira4claude.ConversionIssue) { warnings = append(warnings, w) }, "")

		assert.Equal(t, []jira4claude.ConversionIssue{{Kind: jira4claude.ConversionDropped, Node: "unsupported element: emoji"}, {Kind: jira4claude.ConversionDropped, Node: "unknown node type"}}, warnings)
	})

	t.Run("converts comment bodies to markdown", func(t *testing.T) {
		t.Parallel()

		conv := &mock.Converter{
			ToMarkdownFn: func(adf *jira4claude.ADFNode) (string, []jira4claude.ConversionIssue) {
				if adf != nil && adf.Text != "" {
					return adf.Text + " (converted)", nil
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
					Body:    &jira4claude.ADFNode{Text: "comment body"},
					Created: time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
				},
			},
			Created: time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
			Updated: time.Date(2024, 1, 2, 12, 0, 0, 0, time.UTC),
		}

		var warnings []jira4claude.ConversionIssue
		view := jira4claude.ToIssueView(issue, conv, func(w jira4claude.ConversionIssue) { warnings = append(warnings, w) }, "")

		assert.Len(t, view.Comments, 1)
		assert.Equal(t, "comment body (converted)", view.Comments[0].Body)
		assert.Equal(t, "John Doe", view.Comments[0].Author)
	})

	t.Run("propagates comment conversion warnings", func(t *testing.T) {
		t.Parallel()

		conv := &mock.Converter{
			ToMarkdownFn: func(adf *jira4claude.ADFNode) (string, []jira4claude.ConversionIssue) {
				return "text", []jira4claude.ConversionIssue{{Kind: jira4claude.ConversionDropped, Node: "comment warning"}}
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
					Body:    &jira4claude.ADFNode{Type: "doc"},
					Created: time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
				},
			},
			Created: time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
			Updated: time.Date(2024, 1, 2, 12, 0, 0, 0, time.UTC),
		}

		var warnings []jira4claude.ConversionIssue
		_ = jira4claude.ToIssueView(issue, conv, func(w jira4claude.ConversionIssue) { warnings = append(warnings, w) }, "")

		// Description warning + comment warning
		assert.Contains(t, warnings, jira4claude.ConversionIssue{Kind: jira4claude.ConversionDropped, Node: "comment warning"})
	})

	t.Run("handles nil description", func(t *testing.T) {
		t.Parallel()

		conv := &mock.Converter{
			ToMarkdownFn: func(adf *jira4claude.ADFNode) (string, []jira4claude.ConversionIssue) {
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

		var warnings []jira4claude.ConversionIssue
		view := jira4claude.ToIssueView(issue, conv, func(w jira4claude.ConversionIssue) { warnings = append(warnings, w) }, "")

		assert.Empty(t, view.Description)
	})

	t.Run("converts links to RelatedIssues", func(t *testing.T) {
		t.Parallel()

		conv := &mock.Converter{
			ToMarkdownFn: func(adf *jira4claude.ADFNode) (string, []jira4claude.ConversionIssue) {
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

		var warnings []jira4claude.ConversionIssue
		view := jira4claude.ToIssueView(issue, conv, func(w jira4claude.ConversionIssue) { warnings = append(warnings, w) }, "")

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
			ToMarkdownFn: func(adf *jira4claude.ADFNode) (string, []jira4claude.ConversionIssue) {
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

		view := jira4claude.ToIssueView(issue, conv, func(w jira4claude.ConversionIssue) {}, "https://test.atlassian.net")

		assert.Equal(t, "https://test.atlassian.net/browse/TEST-1", view.URL)
	})

	t.Run("formats updated with milliseconds, to compare with --if-updated", func(t *testing.T) {
		t.Parallel()

		conv := &mock.Converter{
			ToMarkdownFn: func(adf *jira4claude.ADFNode) (string, []jira4claude.ConversionIssue) {
				return "", nil
			},
		}

		issue := &jira4claude.Issue{
			Key:     "TEST-1",
			Updated: time.Date(2026, 11, 9, 10, 15, 30, 123_000_000, time.UTC),
		}

		view := jira4claude.ToIssueView(issue, conv, func(w jira4claude.ConversionIssue) {}, "")

		assert.Equal(t, "2026-11-09T10:15:30.123Z", view.Updated)
	})

	t.Run("formats timestamps as RFC3339", func(t *testing.T) {
		t.Parallel()

		conv := &mock.Converter{
			ToMarkdownFn: func(adf *jira4claude.ADFNode) (string, []jira4claude.ConversionIssue) {
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

		view := jira4claude.ToIssueView(issue, conv, func(w jira4claude.ConversionIssue) {}, "")

		assert.Equal(t, "2024-01-15T10:30:00Z", view.Created)
		assert.Equal(t, "2024-01-16T14:45:00.000Z", view.Updated)
	})

	t.Run("includes optional fields when present", func(t *testing.T) {
		t.Parallel()

		conv := &mock.Converter{
			ToMarkdownFn: func(adf *jira4claude.ADFNode) (string, []jira4claude.ConversionIssue) {
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

		view := jira4claude.ToIssueView(issue, conv, func(w jira4claude.ConversionIssue) {}, "")

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
			ToMarkdownFn: func(adf *jira4claude.ADFNode) (string, []jira4claude.ConversionIssue) {
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

		view := jira4claude.ToIssueView(issue, conv, func(w jira4claude.ConversionIssue) {}, "")

		assert.Empty(t, view.RelatedIssues)
	})
}

func TestToIssueView_CustomFields(t *testing.T) {
	t.Parallel()

	newConv := func() *mock.Converter {
		return &mock.Converter{
			ToMarkdownFn: func(adf *jira4claude.ADFNode) (string, []jira4claude.ConversionIssue) {
				return "", nil
			},
		}
	}

	t.Run("passes ReadCustomFields through to CustomFields", func(t *testing.T) {
		t.Parallel()

		customFields := map[string]jira4claude.CustomFieldValue{
			"customfield_10801": {Name: "Story Points", Value: json.RawMessage(`5`)},
			"customfield_10900": {Name: "Sprint", Value: json.RawMessage(`"Sprint 1"`)},
		}

		issue := &jira4claude.Issue{
			Key:              "TEST-1",
			Summary:          "Test issue",
			Status:           "To Do",
			Type:             "Task",
			ReadCustomFields: customFields,
			Created:          time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
			Updated:          time.Date(2024, 1, 2, 12, 0, 0, 0, time.UTC),
		}

		view := jira4claude.ToIssueView(issue, newConv(), func(w jira4claude.ConversionIssue) {}, "")

		assert.Equal(t, customFields, view.CustomFields)
	})

	t.Run("marshals custom fields keyed by id with name and value", func(t *testing.T) {
		t.Parallel()

		issue := &jira4claude.Issue{
			Key:     "TEST-1",
			Summary: "Test issue",
			Status:  "To Do",
			Type:    "Task",
			ReadCustomFields: map[string]jira4claude.CustomFieldValue{
				"customfield_10801": {Name: "Story Points", Value: json.RawMessage(`5`)},
			},
			Created: time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
			Updated: time.Date(2024, 1, 2, 12, 0, 0, 0, time.UTC),
		}

		view := jira4claude.ToIssueView(issue, newConv(), func(w jira4claude.ConversionIssue) {}, "")

		data, err := json.Marshal(view)
		require.NoError(t, err)

		var got struct {
			CustomFields json.RawMessage `json:"customFields"`
		}
		require.NoError(t, json.Unmarshal(data, &got))

		assert.JSONEq(t,
			`{"customfield_10801":{"name":"Story Points","value":5}}`,
			string(got.CustomFields),
		)
	})

	t.Run("omits customFields key when ReadCustomFields is nil", func(t *testing.T) {
		t.Parallel()

		issue := &jira4claude.Issue{
			Key:              "TEST-1",
			Summary:          "Test issue",
			Status:           "To Do",
			Type:             "Task",
			ReadCustomFields: nil,
			Created:          time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
			Updated:          time.Date(2024, 1, 2, 12, 0, 0, 0, time.UTC),
		}

		view := jira4claude.ToIssueView(issue, newConv(), func(w jira4claude.ConversionIssue) {}, "")

		data, err := json.Marshal(view)
		require.NoError(t, err)

		assert.NotContains(t, string(data), "customFields")
	})

	t.Run("omits customFields key when ReadCustomFields is empty", func(t *testing.T) {
		t.Parallel()

		issue := &jira4claude.Issue{
			Key:              "TEST-1",
			Summary:          "Test issue",
			Status:           "To Do",
			Type:             "Task",
			ReadCustomFields: map[string]jira4claude.CustomFieldValue{},
			Created:          time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
			Updated:          time.Date(2024, 1, 2, 12, 0, 0, 0, time.UTC),
		}

		view := jira4claude.ToIssueView(issue, newConv(), func(w jira4claude.ConversionIssue) {}, "")

		data, err := json.Marshal(view)
		require.NoError(t, err)

		assert.NotContains(t, string(data), "customFields")
	})
}

func TestToIssueListItems(t *testing.T) {
	t.Parallel()

	t.Run("extracts only list-relevant fields", func(t *testing.T) {
		t.Parallel()

		issues := []*jira4claude.Issue{
			{
				Key:      "TEST-1",
				Summary:  "First issue",
				Status:   "To Do",
				Priority: "High",
				Type:     "Task",
			},
			{
				Key:      "TEST-2",
				Summary:  "Second issue",
				Status:   "Done",
				Priority: "Low",
				Type:     "Bug",
			},
		}

		items := jira4claude.ToIssueListItems(issues)

		assert.Len(t, items, 2)
		assert.Equal(t, jira4claude.IssueListItem{Key: "TEST-1", Status: "To Do", Priority: "High", Summary: "First issue"}, items[0])
		assert.Equal(t, jira4claude.IssueListItem{Key: "TEST-2", Status: "Done", Priority: "Low", Summary: "Second issue"}, items[1])
	})

	t.Run("handles empty slice", func(t *testing.T) {
		t.Parallel()

		items := jira4claude.ToIssueListItems([]*jira4claude.Issue{})

		assert.Empty(t, items)
	})

	t.Run("omits empty priority", func(t *testing.T) {
		t.Parallel()

		issues := []*jira4claude.Issue{
			{
				Key:     "TEST-1",
				Summary: "No priority",
				Status:  "To Do",
			},
		}

		items := jira4claude.ToIssueListItems(issues)

		assert.Empty(t, items[0].Priority)
	})
}

func TestToCommentView(t *testing.T) {
	t.Parallel()

	t.Run("converts comment body to markdown", func(t *testing.T) {
		t.Parallel()

		conv := &mock.Converter{
			ToMarkdownFn: func(adf *jira4claude.ADFNode) (string, []jira4claude.ConversionIssue) {
				return "**bold** text", nil
			},
		}

		comment := &jira4claude.Comment{
			ID:      "10001",
			Author:  &jira4claude.User{DisplayName: "John Doe"},
			Body:    &jira4claude.ADFNode{Type: "doc"},
			Created: time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
		}

		var warnings []jira4claude.ConversionIssue
		view := jira4claude.ToCommentView(comment, conv, func(w jira4claude.ConversionIssue) { warnings = append(warnings, w) })

		assert.Equal(t, "10001", view.ID)
		assert.Equal(t, "John Doe", view.Author)
		assert.Equal(t, "**bold** text", view.Body)
		assert.Equal(t, "2024-01-15T10:30:00Z", view.Created)
		assert.Empty(t, warnings)
	})

	t.Run("propagates conversion warnings", func(t *testing.T) {
		t.Parallel()

		conv := &mock.Converter{
			ToMarkdownFn: func(adf *jira4claude.ADFNode) (string, []jira4claude.ConversionIssue) {
				return "text", []jira4claude.ConversionIssue{{Kind: jira4claude.ConversionDropped, Node: "unsupported node type"}, {Kind: jira4claude.ConversionDropped, Node: "emoji not supported"}}
			},
		}

		comment := &jira4claude.Comment{
			ID:      "10001",
			Author:  &jira4claude.User{DisplayName: "John Doe"},
			Body:    &jira4claude.ADFNode{Type: "doc"},
			Created: time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
		}

		var warnings []jira4claude.ConversionIssue
		_ = jira4claude.ToCommentView(comment, conv, func(w jira4claude.ConversionIssue) { warnings = append(warnings, w) })

		assert.Equal(t, []jira4claude.ConversionIssue{{Kind: jira4claude.ConversionDropped, Node: "unsupported node type"}, {Kind: jira4claude.ConversionDropped, Node: "emoji not supported"}}, warnings)
	})

	t.Run("handles nil author", func(t *testing.T) {
		t.Parallel()

		conv := &mock.Converter{
			ToMarkdownFn: func(adf *jira4claude.ADFNode) (string, []jira4claude.ConversionIssue) {
				return "text", nil
			},
		}

		comment := &jira4claude.Comment{
			ID:      "10001",
			Author:  nil,
			Body:    &jira4claude.ADFNode{Type: "doc"},
			Created: time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
		}

		view := jira4claude.ToCommentView(comment, conv, func(w jira4claude.ConversionIssue) {})

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
			ToMarkdownFn: func(adf *jira4claude.ADFNode) (string, []jira4claude.ConversionIssue) {
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

		view := jira4claude.ToIssueView(issue, conv, func(w jira4claude.ConversionIssue) {}, "")

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
			ToMarkdownFn: func(adf *jira4claude.ADFNode) (string, []jira4claude.ConversionIssue) {
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

		view := jira4claude.ToIssueView(issue, conv, func(w jira4claude.ConversionIssue) {}, "")

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

func TestSelectIssueFields(t *testing.T) {
	t.Parallel()

	fields := func() []*jira4claude.IssueField {
		return []*jira4claude.IssueField{
			{ID: "summary", Name: "Summary", Required: true},
			{ID: "description", Name: "Description", Required: false},
			{ID: "customfield_10010", Name: "Story Points", Required: false},
			{ID: "customfield_10801", Name: "Urgency", Required: true},
			{ID: "priority", Name: "Priority", Required: false},
		}
	}

	idsOf := func(fs []*jira4claude.IssueField) []string {
		ids := make([]string, len(fs))
		for i, f := range fs {
			ids[i] = f.ID
		}
		return ids
	}

	t.Run("default keeps required and custom fields", func(t *testing.T) {
		t.Parallel()
		selected, scope, omitted := jira4claude.SelectIssueFields(fields(), "", false)
		assert.ElementsMatch(t, []string{"summary", "customfield_10010", "customfield_10801"}, idsOf(selected))
		assert.Equal(t, jira4claude.FieldScopeDefault, scope)
		assert.Equal(t, 2, omitted) // description, priority
	})

	t.Run("all keeps every field with zero omitted", func(t *testing.T) {
		t.Parallel()
		selected, scope, omitted := jira4claude.SelectIssueFields(fields(), "", true)
		assert.Len(t, selected, 5)
		assert.Equal(t, jira4claude.FieldScopeAll, scope)
		assert.Equal(t, 0, omitted)
	})

	t.Run("filter matches name or id across all fields, case-insensitive", func(t *testing.T) {
		t.Parallel()
		// "prio" matches the optional builtin "priority" that default mode would drop.
		selected, scope, omitted := jira4claude.SelectIssueFields(fields(), "PRIO", false)
		assert.Equal(t, []string{"priority"}, idsOf(selected))
		assert.Equal(t, jira4claude.FieldScopeFiltered, scope)
		assert.Equal(t, 4, omitted)
	})

	t.Run("filter overrides all", func(t *testing.T) {
		t.Parallel()
		selected, scope, _ := jira4claude.SelectIssueFields(fields(), "customfield_10801", true)
		assert.Equal(t, []string{"customfield_10801"}, idsOf(selected))
		assert.Equal(t, jira4claude.FieldScopeFiltered, scope)
	})

	t.Run("filter matches against field id substring", func(t *testing.T) {
		t.Parallel()
		selected, _, _ := jira4claude.SelectIssueFields(fields(), "10010", false)
		assert.Equal(t, []string{"customfield_10010"}, idsOf(selected))
	})

	t.Run("filter with no matches returns empty filtered scope", func(t *testing.T) {
		t.Parallel()
		selected, scope, omitted := jira4claude.SelectIssueFields(fields(), "nonesuch", false)
		assert.Empty(t, selected)
		assert.Equal(t, jira4claude.FieldScopeFiltered, scope)
		assert.Equal(t, 5, omitted)
	})
}

func TestParseViewFields(t *testing.T) {
	t.Parallel()

	t.Run("no names selects nothing", func(t *testing.T) {
		t.Parallel()

		got, err := jira4claude.ParseViewFields(nil)

		require.NoError(t, err)
		assert.Empty(t, got)
	})

	t.Run("names are matched without case and returned as the JSON keys", func(t *testing.T) {
		t.Parallel()

		got, err := jira4claude.ParseViewFields([]string{"Description", "RELATEDISSUES", "CustomField_10016"})

		require.NoError(t, err)
		assert.Equal(t, []string{"description", "relatedIssues", "customfield_10016"}, got)
	})

	t.Run("a repeated name is kept once, at its first position", func(t *testing.T) {
		t.Parallel()

		got, err := jira4claude.ParseViewFields([]string{"status", "summary", "Status"})

		require.NoError(t, err)
		assert.Equal(t, []string{"status", "summary"}, got)
	})

	t.Run("every built-in name is accepted", func(t *testing.T) {
		t.Parallel()

		got, err := jira4claude.ParseViewFields(jira4claude.ViewFieldNames())

		require.NoError(t, err)
		assert.Equal(t, jira4claude.ViewFieldNames(), got)
	})

	t.Run("an unknown name is a validation error naming the valid ones", func(t *testing.T) {
		t.Parallel()

		_, err := jira4claude.ParseViewFields([]string{"summary", "descripton"})

		require.Error(t, err)
		assert.Equal(t, jira4claude.EValidation, jira4claude.ErrorCode(err))
		assert.Contains(t, err.Error(), `unknown field "descripton"`)
		assert.Contains(t, err.Error(), "key, project, summary, description, status, type, priority, assignee, reporter, labels, relatedIssues, comments, created, updated, url")
		assert.Contains(t, err.Error(), "customfield_")
	})
}

// objectKeys returns the keys of the flat JSON object doc, in order.
func objectKeys(t *testing.T, doc []byte) []string {
	t.Helper()
	dec := json.NewDecoder(bytes.NewReader(doc))
	_, err := dec.Token() // {
	require.NoError(t, err)
	var keys []string
	for dec.More() {
		tok, err := dec.Token()
		require.NoError(t, err)
		keys = append(keys, tok.(string))
		var value json.RawMessage
		require.NoError(t, dec.Decode(&value))
	}
	return keys
}

func TestSelectViewFields(t *testing.T) {
	t.Parallel()

	view := jira4claude.IssueView{
		Key:          "TEST-1",
		Summary:      "Sum",
		CustomFields: map[string]jira4claude.CustomFieldValue{"customfield_1": {Name: "Points", Value: json.RawMessage(`5`)}},
	}

	t.Run("values follow the fields asked", func(t *testing.T) {
		t.Parallel()

		got := jira4claude.SelectViewFields(view, []string{"summary", "labels", "comments", "relatedIssues", "customfield_1", "customfield_2"})

		assert.Equal(t, "TEST-1", got.Key)
		assert.Equal(t, []jira4claude.ViewFieldValue{
			{Name: "summary", Value: "Sum"},
			{Name: "labels", Value: []string{}},
			{Name: "comments", Value: []jira4claude.CommentView{}},
			{Name: "relatedIssues", Value: []jira4claude.RelatedIssueView{}},
			{Name: "customfield_1", Value: json.RawMessage(`5`)},
			{Name: "customfield_2", Value: json.RawMessage(nil)},
		}, got.Fields)
	})

	t.Run("JSON puts the key first and keeps the order asked", func(t *testing.T) {
		t.Parallel()

		got, err := json.Marshal(jira4claude.SelectViewFields(view, []string{"summary", "key", "customfield_2", "customfield_1"}))

		require.NoError(t, err)
		assert.JSONEq(t, `{"key":"TEST-1","summary":"Sum","customfield_2":null,"customfield_1":5}`, string(got))
		assert.Equal(t, []string{"key", "summary", "customfield_2", "customfield_1"}, objectKeys(t, got), "key order")
	})
}
