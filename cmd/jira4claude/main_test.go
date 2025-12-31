package main

import (
	"bytes"
	"context"
	"testing"

	"github.com/fwojciec/jira4claude"
	"github.com/fwojciec/jira4claude/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIssueToMap_WithParent(t *testing.T) {
	t.Parallel()

	t.Run("includes parent when present", func(t *testing.T) {
		t.Parallel()

		issue := &jira4claude.Issue{
			Key:     "TEST-2",
			Summary: "Test subtask",
			Status:  "To Do",
			Type:    "Subtask",
			Parent:  "TEST-1",
		}

		m := issueToMap(issue, "https://test.atlassian.net")

		assert.Equal(t, "TEST-1", m["parent"])
	})

	t.Run("includes empty parent when not present", func(t *testing.T) {
		t.Parallel()

		issue := &jira4claude.Issue{
			Key:     "TEST-1",
			Summary: "Test task",
			Status:  "To Do",
			Type:    "Task",
			Parent:  "",
		}

		m := issueToMap(issue, "https://test.atlassian.net")

		assert.Empty(t, m["parent"])
	})
}

func TestPrintIssueDetail_WithParent(t *testing.T) {
	t.Parallel()

	t.Run("shows parent when present", func(t *testing.T) {
		t.Parallel()

		issue := &jira4claude.Issue{
			Key:     "TEST-2",
			Summary: "Test subtask",
			Status:  "To Do",
			Type:    "Subtask",
			Parent:  "TEST-1",
		}

		var buf bytes.Buffer
		printIssueDetail(&buf, issue, "https://test.atlassian.net")

		assert.Contains(t, buf.String(), "Parent: TEST-1")
	})

	t.Run("does not show parent when empty", func(t *testing.T) {
		t.Parallel()

		issue := &jira4claude.Issue{
			Key:     "TEST-1",
			Summary: "Test task",
			Status:  "To Do",
			Type:    "Task",
			Parent:  "",
		}

		var buf bytes.Buffer
		printIssueDetail(&buf, issue, "https://test.atlassian.net")

		assert.NotContains(t, buf.String(), "Parent:")
	})
}

func TestListCmd_WithParent(t *testing.T) {
	t.Parallel()

	t.Run("populates parent filter when specified", func(t *testing.T) {
		t.Parallel()

		var capturedFilter jira4claude.IssueFilter
		svc := &mock.IssueService{
			ListFn: func(ctx context.Context, filter jira4claude.IssueFilter) ([]*jira4claude.Issue, error) {
				capturedFilter = filter
				return []*jira4claude.Issue{}, nil
			},
		}

		app := &App{
			config:  &jira4claude.Config{Project: "TEST", Server: "https://test.atlassian.net"},
			service: svc,
			jsonOut: false,
		}

		cmd := &ListCmd{
			Parent: "TEST-1",
		}

		err := cmd.Run(app)
		require.NoError(t, err)
		assert.Equal(t, "TEST-1", capturedFilter.Parent)
	})
}

func TestCreateCmd_WithParent(t *testing.T) {
	t.Parallel()

	t.Run("sets parent field on issue", func(t *testing.T) {
		t.Parallel()

		var capturedIssue *jira4claude.Issue
		svc := &mock.IssueService{
			CreateFn: func(ctx context.Context, issue *jira4claude.Issue) (*jira4claude.Issue, error) {
				capturedIssue = issue
				return &jira4claude.Issue{Key: "TEST-2", Parent: issue.Parent}, nil
			},
		}

		app := &App{
			config:  &jira4claude.Config{Project: "TEST", Server: "https://test.atlassian.net"},
			service: svc,
			jsonOut: false,
		}

		cmd := &CreateCmd{
			Summary: "Test subtask",
			Parent:  "TEST-1",
		}

		err := cmd.Run(app)
		require.NoError(t, err)
		require.NotNil(t, capturedIssue)
		assert.Equal(t, "TEST-1", capturedIssue.Parent)
	})

	t.Run("auto-sets type to Subtask when parent is specified", func(t *testing.T) {
		t.Parallel()

		var capturedIssue *jira4claude.Issue
		svc := &mock.IssueService{
			CreateFn: func(ctx context.Context, issue *jira4claude.Issue) (*jira4claude.Issue, error) {
				capturedIssue = issue
				return &jira4claude.Issue{Key: "TEST-2", Type: issue.Type}, nil
			},
		}

		app := &App{
			config:  &jira4claude.Config{Project: "TEST", Server: "https://test.atlassian.net"},
			service: svc,
			jsonOut: false,
		}

		cmd := &CreateCmd{
			Summary: "Test subtask",
			Parent:  "TEST-1",
			Type:    "Task", // default value from Kong
		}

		err := cmd.Run(app)
		require.NoError(t, err)
		require.NotNil(t, capturedIssue)
		assert.Equal(t, "Subtask", capturedIssue.Type)
	})
}
