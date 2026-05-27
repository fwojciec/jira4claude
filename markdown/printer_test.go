package markdown_test

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/fwojciec/jira4claude"
	"github.com/fwojciec/jira4claude/markdown"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPrinter_Issue(t *testing.T) {
	t.Parallel()

	t.Run("renders complete issue with all fields", func(t *testing.T) {
		t.Parallel()
		var out bytes.Buffer
		p := markdown.NewPrinter(&out)

		view := jira4claude.IssueView{
			Key:         "J4C-100",
			Summary:     "cmd package cleanup and test coverage improvements",
			Type:        "Task",
			Status:      "Done",
			Priority:    "Medium",
			Assignee:    "Filip Wojciechowski",
			Reporter:    "Filip Wojciechowski",
			Labels:      []string{"backend", "cleanup"},
			Description: "Code review identified stale TODO comments.",
			RelatedIssues: []jira4claude.RelatedIssueView{
				{Relationship: "parent", Key: "J4C-96", Type: "Epic", Status: "In Progress", Summary: "Parent epic"},
				{Relationship: "subtask", Key: "J4C-97", Type: "Sub-task", Status: "Done", Summary: "Investigate current subtask behavior"},
				{Relationship: "subtask", Key: "J4C-98", Type: "Sub-task", Status: "Done", Summary: "Fix subtask type name mismatch"},
				{Relationship: "subtask", Key: "J4C-99", Type: "Sub-task", Status: "To Do", Summary: "Display subtasks in parent issue view"},
				{Relationship: "blocks", Key: "J4C-78", Type: "Task", Status: "To Do", Summary: "Rename adf package"},
				{Relationship: "is blocked by", Key: "J4C-74", Type: "Task", Status: "Done", Summary: "Inject Converter into CLI"},
			},
			Comments: []jira4claude.CommentView{
				{Author: "Filip Wojciechowski", Created: "2026-01-04T10:30:00Z", Body: "Completed the initial investigation."},
			},
			URL: "https://fwojciec.atlassian.net/browse/J4C-100",
		}

		p.Issue(view)
		result := out.String()

		// Header
		assert.Contains(t, result, "# J4C-100: cmd package cleanup and test coverage improvements")
		// Metadata
		assert.Contains(t, result, "**Type:** Task")
		assert.Contains(t, result, "**Status:** Done")
		assert.Contains(t, result, "**Priority:** Medium")
		assert.Contains(t, result, "**Assignee:** Filip Wojciechowski")
		assert.Contains(t, result, "**Reporter:** Filip Wojciechowski")
		assert.Contains(t, result, "**Parent:** J4C-96")
		assert.Contains(t, result, "**Labels:** backend, cleanup")
		// Description
		assert.Contains(t, result, "Code review identified stale TODO comments.")
		// Related Issues section - unified display grouped by relationship type with (Type)
		assert.Contains(t, result, "## Related Issues")
		assert.Contains(t, result, "**subtask:**")
		assert.Contains(t, result, "- **J4C-97** [Done] (Sub-task) Investigate current subtask behavior")
		assert.Contains(t, result, "- **J4C-98** [Done] (Sub-task) Fix subtask type name mismatch")
		assert.Contains(t, result, "- **J4C-99** [To Do] (Sub-task) Display subtasks in parent issue view")
		assert.Contains(t, result, "**blocks:**")
		assert.Contains(t, result, "- **J4C-78** [To Do] (Task) Rename adf package")
		assert.Contains(t, result, "**is blocked by:**")
		assert.Contains(t, result, "- **J4C-74** [Done] (Task) Inject Converter into CLI")
		// Comments section
		assert.Contains(t, result, "## Comments")
		assert.Contains(t, result, "**Filip Wojciechowski** (2026-01-04 10:30):")
		assert.Contains(t, result, "Completed the initial investigation.")
		// URL
		assert.Contains(t, result, "[View in Jira](https://fwojciec.atlassian.net/browse/J4C-100)")
	})

	t.Run("omits empty fields", func(t *testing.T) {
		t.Parallel()
		var out bytes.Buffer
		p := markdown.NewPrinter(&out)

		view := jira4claude.IssueView{
			Key:     "J4C-101",
			Summary: "Minimal issue",
			Type:    "Task",
			Status:  "To Do",
		}

		p.Issue(view)
		result := out.String()

		assert.Contains(t, result, "# J4C-101: Minimal issue")
		assert.Contains(t, result, "**Type:** Task")
		assert.Contains(t, result, "**Status:** To Do")
		// Empty fields should not appear
		assert.NotContains(t, result, "**Priority:**")
		assert.NotContains(t, result, "**Assignee:**")
		assert.NotContains(t, result, "**Reporter:**")
		assert.NotContains(t, result, "**Parent:**")
		assert.NotContains(t, result, "**Labels:**")
		assert.NotContains(t, result, "## Related Issues")
		assert.NotContains(t, result, "## Comments")
		assert.NotContains(t, result, "[View in Jira]")
	})

	t.Run("handles In Progress status", func(t *testing.T) {
		t.Parallel()
		var out bytes.Buffer
		p := markdown.NewPrinter(&out)

		view := jira4claude.IssueView{
			Key:     "J4C-102",
			Summary: "In progress issue",
			Type:    "Task",
			Status:  "In Progress",
			RelatedIssues: []jira4claude.RelatedIssueView{
				{Relationship: "subtask", Key: "J4C-103", Type: "Sub-task", Status: "In Progress", Summary: "Subtask in progress"},
			},
		}

		p.Issue(view)
		result := out.String()

		assert.Contains(t, result, "**Status:** In Progress")
		assert.Contains(t, result, "- **J4C-103** [In Progress] (Sub-task) Subtask in progress")
	})

	t.Run("handles Won't Do status", func(t *testing.T) {
		t.Parallel()
		var out bytes.Buffer
		p := markdown.NewPrinter(&out)

		view := jira4claude.IssueView{
			Key:     "J4C-102",
			Summary: "Won't do issue",
			Type:    "Task",
			Status:  "Won't Do",
			RelatedIssues: []jira4claude.RelatedIssueView{
				{Relationship: "subtask", Key: "J4C-103", Type: "Sub-task", Status: "Won't Do", Summary: "Subtask won't do"},
			},
		}

		p.Issue(view)
		result := out.String()

		assert.Contains(t, result, "**Status:** Won't Do")
		assert.Contains(t, result, "- **J4C-103** [Won't Do] (Sub-task) Subtask won't do")
	})

	t.Run("renders related issues in fixed order", func(t *testing.T) {
		t.Parallel()
		var out bytes.Buffer
		p := markdown.NewPrinter(&out)

		// Provide issues in scrambled order - output should be in fixed order
		view := jira4claude.IssueView{
			Key:     "J4C-200",
			Summary: "Test fixed ordering",
			Type:    "Task",
			Status:  "To Do",
			RelatedIssues: []jira4claude.RelatedIssueView{
				{Relationship: "is blocked by", Key: "J4C-201", Type: "Task", Status: "Done", Summary: "Blocker done"},
				{Relationship: "blocks", Key: "J4C-202", Type: "Task", Status: "To Do", Summary: "Blocked task"},
				{Relationship: "subtask", Key: "J4C-203", Type: "Sub-task", Status: "To Do", Summary: "Child task"},
			},
		}

		p.Issue(view)
		result := out.String()

		// Fixed order: subtask -> blocks -> is blocked by
		subtaskIdx := strings.Index(result, "**subtask:**")
		blocksIdx := strings.Index(result, "**blocks:**")
		isBlockedByIdx := strings.Index(result, "**is blocked by:**")

		assert.Less(t, subtaskIdx, blocksIdx, "subtask should appear before blocks")
		assert.Less(t, blocksIdx, isBlockedByIdx, "blocks should appear before is blocked by")
	})
}

func TestPrinter_Issues(t *testing.T) {
	t.Parallel()

	t.Run("renders issue list with status and priority indicators", func(t *testing.T) {
		t.Parallel()
		var out bytes.Buffer
		p := markdown.NewPrinter(&out)

		views := []jira4claude.IssueListItem{
			{Key: "J4C-103", Summary: "http package cleanup", Status: "Done", Priority: "Medium"},
			{Key: "J4C-102", Summary: "goldmark package test coverage", Status: "Done", Priority: "Medium"},
			{Key: "J4C-95", Summary: "Investigate epic support", Status: "To Do", Priority: "Medium"},
			{Key: "J4C-104", Summary: "Implement new feature", Status: "In Progress", Priority: "High"},
		}

		p.Issues(views)
		result := out.String()

		assert.Contains(t, result, "- **J4C-103** [Done] [P2] http package cleanup")
		assert.Contains(t, result, "- **J4C-102** [Done] [P2] goldmark package test coverage")
		assert.Contains(t, result, "- **J4C-95** [To Do] [P2] Investigate epic support")
		assert.Contains(t, result, "- **J4C-104** [In Progress] [P1] Implement new feature")
	})

	t.Run("truncates long summaries", func(t *testing.T) {
		t.Parallel()
		var out bytes.Buffer
		p := markdown.NewPrinter(&out)

		views := []jira4claude.IssueListItem{
			{
				Key:      "J4C-100",
				Summary:  "This is a very long summary that exceeds sixty characters and should be truncated",
				Status:   "To Do",
				Priority: "Medium",
			},
		}

		p.Issues(views)
		result := out.String()

		// Should truncate at 60 chars with ...
		assert.Contains(t, result, "This is a very long summary that exceeds sixty characters...")
		assert.NotContains(t, result, "should be truncated")
	})

	t.Run("handles all priority levels", func(t *testing.T) {
		t.Parallel()
		var out bytes.Buffer
		p := markdown.NewPrinter(&out)

		views := []jira4claude.IssueListItem{
			{Key: "J4C-1", Summary: "Highest priority", Status: "To Do", Priority: "Highest"},
			{Key: "J4C-2", Summary: "High priority", Status: "To Do", Priority: "High"},
			{Key: "J4C-3", Summary: "Medium priority", Status: "To Do", Priority: "Medium"},
			{Key: "J4C-4", Summary: "Low priority", Status: "To Do", Priority: "Low"},
			{Key: "J4C-5", Summary: "Lowest priority", Status: "To Do", Priority: "Lowest"},
		}

		p.Issues(views)
		result := out.String()

		assert.Contains(t, result, "- **J4C-1** [To Do] [P0] Highest priority")
		assert.Contains(t, result, "- **J4C-2** [To Do] [P1] High priority")
		assert.Contains(t, result, "- **J4C-3** [To Do] [P2] Medium priority")
		assert.Contains(t, result, "- **J4C-4** [To Do] [P3] Low priority")
		assert.Contains(t, result, "- **J4C-5** [To Do] [P4] Lowest priority")
	})

	t.Run("empty list shows info message", func(t *testing.T) {
		t.Parallel()
		var out bytes.Buffer
		p := markdown.NewPrinter(&out)

		p.Issues(nil)
		result := out.String()

		assert.Contains(t, result, "[info] No issues found")
	})

	t.Run("handles Won't Do status in list", func(t *testing.T) {
		t.Parallel()
		var out bytes.Buffer
		p := markdown.NewPrinter(&out)

		views := []jira4claude.IssueListItem{
			{Key: "J4C-100", Summary: "Cancelled issue", Status: "Won't Do", Priority: "Medium"},
		}

		p.Issues(views)
		result := out.String()

		assert.Contains(t, result, "- **J4C-100** [Won't Do] [P2] Cancelled issue")
	})
}

func TestPrinter_Comment(t *testing.T) {
	t.Parallel()

	t.Run("renders comment with author and timestamp", func(t *testing.T) {
		t.Parallel()
		var out bytes.Buffer
		p := markdown.NewPrinter(&out)

		view := jira4claude.CommentView{
			Author:  "Filip Wojciechowski",
			Created: "2026-01-04T10:30:00Z",
			Body:    "This is a comment body.",
		}

		p.Comment(view)
		result := out.String()

		assert.Contains(t, result, "**Filip Wojciechowski** (2026-01-04 10:30):")
		assert.Contains(t, result, "This is a comment body.")
	})

	t.Run("handles unknown author", func(t *testing.T) {
		t.Parallel()
		var out bytes.Buffer
		p := markdown.NewPrinter(&out)

		view := jira4claude.CommentView{
			Created: "2026-01-04T10:30:00Z",
			Body:    "Anonymous comment.",
		}

		p.Comment(view)
		result := out.String()

		assert.Contains(t, result, "**Unknown** (2026-01-04 10:30):")
	})

	t.Run("preserves short timestamp", func(t *testing.T) {
		t.Parallel()
		var out bytes.Buffer
		p := markdown.NewPrinter(&out)

		view := jira4claude.CommentView{
			Author:  "Test User",
			Created: "2026-01", // Short timestamp
			Body:    "Test body.",
		}

		p.Comment(view)
		result := out.String()

		assert.Contains(t, result, "(2026-01):")
	})

	t.Run("handles multi-line comment body", func(t *testing.T) {
		t.Parallel()
		var out bytes.Buffer
		p := markdown.NewPrinter(&out)

		view := jira4claude.CommentView{
			Author:  "Test User",
			Created: "2026-01-04T10:30:00Z",
			Body:    "Line 1\nLine 2\nLine 3",
		}

		p.Comment(view)
		result := out.String()

		assert.Contains(t, result, "Line 1\nLine 2\nLine 3")
	})
}

func TestPrinter_Transitions(t *testing.T) {
	t.Parallel()

	t.Run("renders transition list", func(t *testing.T) {
		t.Parallel()
		var out bytes.Buffer
		p := markdown.NewPrinter(&out)

		transitions := []*jira4claude.Transition{
			{ID: "1", Name: "In Progress"},
			{ID: "2", Name: "Done"},
			{ID: "3", Name: "Blocked"},
		}

		p.Transitions("J4C-100", transitions)
		result := out.String()

		assert.Contains(t, result, "- In Progress")
		assert.Contains(t, result, "- Done")
		assert.Contains(t, result, "- Blocked")
	})

	t.Run("empty transitions shows info message", func(t *testing.T) {
		t.Parallel()
		var out bytes.Buffer
		p := markdown.NewPrinter(&out)

		p.Transitions("J4C-100", nil)
		result := out.String()

		assert.Contains(t, result, "[info] No transitions available for J4C-100")
	})
}

func TestPrinter_Links(t *testing.T) {
	t.Parallel()

	t.Run("renders links grouped by relationship type with type annotation", func(t *testing.T) {
		t.Parallel()
		var out bytes.Buffer
		p := markdown.NewPrinter(&out)

		links := []jira4claude.RelatedIssueView{
			{Relationship: "blocks", Key: "J4C-78", Type: "Task", Status: "To Do", Summary: "Rename adf package"},
			{Relationship: "blocks", Key: "J4C-79", Type: "Task", Status: "Done", Summary: "Another blocked issue"},
			{Relationship: "is blocked by", Key: "J4C-74", Type: "Task", Status: "Done", Summary: "Inject Converter into CLI"},
		}

		p.Links("J4C-100", links)
		result := out.String()

		assert.Contains(t, result, "**blocks:**")
		assert.Contains(t, result, "- **J4C-78** [To Do] (Task) Rename adf package")
		assert.Contains(t, result, "- **J4C-79** [Done] (Task) Another blocked issue")
		assert.Contains(t, result, "**is blocked by:**")
		assert.Contains(t, result, "- **J4C-74** [Done] (Task) Inject Converter into CLI")
	})

	t.Run("empty links shows info message", func(t *testing.T) {
		t.Parallel()
		var out bytes.Buffer
		p := markdown.NewPrinter(&out)

		p.Links("J4C-100", nil)
		result := out.String()

		assert.Contains(t, result, "[info] No links for J4C-100")
	})
}

func TestPrinter_Success(t *testing.T) {
	t.Parallel()

	t.Run("renders success message without keys", func(t *testing.T) {
		t.Parallel()
		var out bytes.Buffer
		p := markdown.NewPrinter(&out)

		p.Success("Operation completed")
		result := out.String()

		assert.Equal(t, "[ok] Operation completed\n", result)
	})

	t.Run("renders success message with keys", func(t *testing.T) {
		t.Parallel()
		var out bytes.Buffer
		p := markdown.NewPrinter(&out)

		p.Success("Issue created", "J4C-105")
		result := out.String()

		assert.Contains(t, result, "[ok] Issue created J4C-105")
	})

	t.Run("renders success message with multiple keys", func(t *testing.T) {
		t.Parallel()
		var out bytes.Buffer
		p := markdown.NewPrinter(&out)

		p.Success("Issues created", "J4C-105", "J4C-106")
		result := out.String()

		assert.Contains(t, result, "[ok] Issues created J4C-105, J4C-106")
	})

	t.Run("includes URLs when server URL is set", func(t *testing.T) {
		t.Parallel()
		var out bytes.Buffer
		p := markdown.NewPrinter(&out)
		p.SetServerURL("https://fwojciec.atlassian.net")

		p.Success("Issue created", "J4C-105")
		result := out.String()

		assert.Contains(t, result, "[ok] Issue created J4C-105")
		assert.Contains(t, result, "https://fwojciec.atlassian.net/browse/J4C-105")
	})
}

func TestPrinter_Warning(t *testing.T) {
	t.Parallel()

	t.Run("renders warning to stderr", func(t *testing.T) {
		t.Parallel()
		var out, errOut bytes.Buffer
		p := markdown.NewPrinterWithIO(&out, &errOut)

		p.Warning("Description contained unsupported formatting")

		assert.Empty(t, out.String())
		assert.Equal(t, "[warn] Description contained unsupported formatting\n", errOut.String())
	})

	t.Run("discards warnings when using NewPrinter", func(t *testing.T) {
		t.Parallel()
		var out bytes.Buffer
		p := markdown.NewPrinter(&out)

		p.Warning("This should be discarded")

		assert.Empty(t, out.String())
	})
}

func TestPrinter_Error(t *testing.T) {
	t.Parallel()

	t.Run("renders error to stderr", func(t *testing.T) {
		t.Parallel()
		var out, errOut bytes.Buffer
		p := markdown.NewPrinterWithIO(&out, &errOut)

		p.Error(errors.New("connection failed"))

		assert.Empty(t, out.String())
		assert.Equal(t, "[error] connection failed\n", errOut.String())
	})

	t.Run("handles jira4claude.Error", func(t *testing.T) {
		t.Parallel()
		var out, errOut bytes.Buffer
		p := markdown.NewPrinterWithIO(&out, &errOut)

		err := &jira4claude.Error{
			Code:    jira4claude.ENotFound,
			Message: "Issue not found",
		}

		p.Error(err)

		assert.Equal(t, "[error] Issue not found\n", errOut.String())
	})
}

func TestPrinter_Fields(t *testing.T) {
	t.Parallel()

	t.Run("renders heading using Source verbatim", func(t *testing.T) {
		t.Parallel()
		var out bytes.Buffer
		p := markdown.NewPrinter(&out)

		view := jira4claude.IssueFieldsView{
			Source: "INT-1118 (edit)",
			Fields: []*jira4claude.IssueField{
				{ID: "summary", Name: "Summary", Required: true, SchemaType: "string"},
			},
		}

		p.Fields(view)
		assert.Contains(t, out.String(), "Fields for INT-1118 (edit):")
	})

	t.Run("REQUIRED appears before CUSTOM (optional)", func(t *testing.T) {
		t.Parallel()
		var out bytes.Buffer
		p := markdown.NewPrinter(&out)

		view := jira4claude.IssueFieldsView{
			Source: "INT / Bug",
			Fields: []*jira4claude.IssueField{
				{ID: "customfield_10010", Name: "Story Points", Required: false, SchemaType: "number"},
				{ID: "summary", Name: "Summary", Required: true, SchemaType: "string"},
			},
		}

		p.Fields(view)
		result := out.String()

		reqIdx := strings.Index(result, "REQUIRED")
		customIdx := strings.Index(result, "CUSTOM (optional)")
		require.NotEqual(t, -1, reqIdx, "REQUIRED section missing")
		require.NotEqual(t, -1, customIdx, "CUSTOM section missing")
		assert.Less(t, reqIdx, customIdx, "REQUIRED should appear before CUSTOM")
	})

	t.Run("omits CUSTOM section when no optional customfields", func(t *testing.T) {
		t.Parallel()
		var out bytes.Buffer
		p := markdown.NewPrinter(&out)

		view := jira4claude.IssueFieldsView{
			Source: "INT / Bug",
			Fields: []*jira4claude.IssueField{
				{ID: "summary", Name: "Summary", Required: true, SchemaType: "string"},
				// A non-required, non-customfield builtin should be filtered out entirely.
				{ID: "priority", Name: "Priority", Required: false, SchemaType: "priority"},
			},
		}

		p.Fields(view)
		result := out.String()

		assert.Contains(t, result, "REQUIRED")
		assert.NotContains(t, result, "CUSTOM (optional)")
		// The optional builtin should also not be rendered.
		assert.NotContains(t, result, "priority")
	})

	t.Run("omits REQUIRED section when no required fields", func(t *testing.T) {
		t.Parallel()
		var out bytes.Buffer
		p := markdown.NewPrinter(&out)

		view := jira4claude.IssueFieldsView{
			Source: "INT / Bug",
			Fields: []*jira4claude.IssueField{
				{ID: "customfield_10010", Name: "Story Points", Required: false, SchemaType: "number"},
			},
		}

		p.Fields(view)
		result := out.String()

		assert.NotContains(t, result, "REQUIRED")
		assert.Contains(t, result, "CUSTOM (optional)")
	})

	t.Run("required customfields precede required builtins", func(t *testing.T) {
		t.Parallel()
		var out bytes.Buffer
		p := markdown.NewPrinter(&out)

		view := jira4claude.IssueFieldsView{
			Source: "INT / Bug",
			Fields: []*jira4claude.IssueField{
				{ID: "summary", Name: "Summary", Required: true, SchemaType: "string"},
				{ID: "customfield_10801", Name: "Urgency / Risk", Required: true, SchemaType: "option"},
			},
		}

		p.Fields(view)
		result := out.String()

		customIdx := strings.Index(result, "customfield_10801")
		summaryIdx := strings.Index(result, "summary")
		require.NotEqual(t, -1, customIdx)
		require.NotEqual(t, -1, summaryIdx)
		assert.Less(t, customIdx, summaryIdx, "required customfield should come before required builtin")
	})

	t.Run("array<option> and array<string> render correctly", func(t *testing.T) {
		t.Parallel()
		var out bytes.Buffer
		p := markdown.NewPrinter(&out)

		view := jira4claude.IssueFieldsView{
			Source: "INT / Bug",
			Fields: []*jira4claude.IssueField{
				{ID: "customfield_10838", Name: "Product Area(s)", Required: true, SchemaType: "array", SchemaItems: "option"},
				{ID: "labels", Name: "Labels", Required: true, SchemaType: "array", SchemaItems: "string"},
			},
		}

		p.Fields(view)
		result := out.String()
		assert.Contains(t, result, "array<option>")
		assert.Contains(t, result, "array<string>")
	})

	t.Run("allowed values are quoted and comma-separated", func(t *testing.T) {
		t.Parallel()
		var out bytes.Buffer
		p := markdown.NewPrinter(&out)

		view := jira4claude.IssueFieldsView{
			Source: "INT / Bug",
			Fields: []*jira4claude.IssueField{
				{
					ID: "customfield_10801", Name: "Urgency / Risk", Required: true, SchemaType: "option",
					AllowedValues: []jira4claude.FieldAllowedValue{
						{Value: "High"}, {Value: "Medium"}, {Value: "Low"},
					},
				},
			},
		}

		p.Fields(view)
		result := out.String()
		assert.Contains(t, result, `allowed: "High", "Medium", "Low"`)
	})

	t.Run("allowed values truncate past five entries", func(t *testing.T) {
		t.Parallel()
		var out bytes.Buffer
		p := markdown.NewPrinter(&out)

		view := jira4claude.IssueFieldsView{
			Source: "INT / Bug",
			Fields: []*jira4claude.IssueField{
				{
					ID: "customfield_10838", Name: "Product Area(s)", Required: true, SchemaType: "array", SchemaItems: "option",
					AllowedValues: []jira4claude.FieldAllowedValue{
						{Value: "A"}, {Value: "B"}, {Value: "C"}, {Value: "D"}, {Value: "E"}, {Value: "F"}, {Value: "G"},
					},
				},
			},
		}

		p.Fields(view)
		result := out.String()
		// First five appear, sixth+ do not.
		assert.Contains(t, result, `"A"`)
		assert.Contains(t, result, `"E"`)
		assert.NotContains(t, result, `"F"`)
		assert.NotContains(t, result, `"G"`)
		assert.Contains(t, result, "…")
	})

	t.Run("long field names are truncated", func(t *testing.T) {
		t.Parallel()
		var out bytes.Buffer
		p := markdown.NewPrinter(&out)

		view := jira4claude.IssueFieldsView{
			Source: "INT / Bug",
			Fields: []*jira4claude.IssueField{
				{
					ID:         "customfield_10999",
					Name:       "This is an extremely long field name that should definitely get truncated",
					Required:   true,
					SchemaType: "string",
				},
			},
		}

		p.Fields(view)
		result := out.String()
		// The full long name must not appear; some truncation marker should be present.
		assert.NotContains(t, result, "should definitely get truncated")
		assert.Contains(t, result, "...")
	})

	t.Run("empty input renders info line", func(t *testing.T) {
		t.Parallel()
		var out bytes.Buffer
		p := markdown.NewPrinter(&out)

		view := jira4claude.IssueFieldsView{Source: "INT / Bug", Fields: nil}
		p.Fields(view)

		assert.Equal(t, "[info] No fields for INT / Bug\n", out.String())
	})

	t.Run("empty schema type renders as unknown", func(t *testing.T) {
		t.Parallel()
		var out bytes.Buffer
		p := markdown.NewPrinter(&out)

		view := jira4claude.IssueFieldsView{
			Source: "INT / Bug",
			Fields: []*jira4claude.IssueField{
				{ID: "customfield_99999", Name: "Mystery", Required: true, SchemaType: ""},
			},
		}

		p.Fields(view)
		assert.Contains(t, out.String(), "unknown")
	})
}
