package gogh_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"

	"github.com/fwojciec/jira4claude"
	"github.com/fwojciec/jira4claude/gogh"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJSONPrinter_Issue(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	p := gogh.NewJSONPrinter(&out)

	issue := &jira4claude.Issue{
		Key:     "TEST-123",
		Summary: "Test issue",
		Status:  "Open",
		Type:    "Task",
	}

	p.Issue(issue)

	var result map[string]any
	err := json.Unmarshal(out.Bytes(), &result)
	require.NoError(t, err)
	assert.Equal(t, "TEST-123", result["key"])
	assert.Equal(t, "Test issue", result["summary"])
	assert.Equal(t, "Open", result["status"])
}

func TestJSONPrinter_Issue_WithAssignee(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	p := gogh.NewJSONPrinter(&out)

	issue := &jira4claude.Issue{
		Key:     "TEST-123",
		Summary: "Test issue",
		Status:  "Open",
		Assignee: &jira4claude.User{
			AccountID:   "abc123",
			DisplayName: "John Doe",
			Email:       "john@example.com",
		},
	}

	p.Issue(issue)

	var result map[string]any
	err := json.Unmarshal(out.Bytes(), &result)
	require.NoError(t, err)
	assignee, ok := result["assignee"].(map[string]any)
	require.True(t, ok, "assignee should be a map")
	assert.Equal(t, "abc123", assignee["accountId"])
	assert.Equal(t, "John Doe", assignee["displayName"])
}

func TestJSONPrinter_Issue_WithReporter(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	p := gogh.NewJSONPrinter(&out)

	issue := &jira4claude.Issue{
		Key:     "TEST-123",
		Summary: "Test issue",
		Status:  "Open",
		Reporter: &jira4claude.User{
			AccountID:   "xyz789",
			DisplayName: "Jane Smith",
			Email:       "jane@example.com",
		},
	}

	p.Issue(issue)

	var result map[string]any
	err := json.Unmarshal(out.Bytes(), &result)
	require.NoError(t, err)
	reporter, ok := result["reporter"].(map[string]any)
	require.True(t, ok, "reporter should be a map")
	assert.Equal(t, "xyz789", reporter["accountId"])
	assert.Equal(t, "Jane Smith", reporter["displayName"])
}

func TestJSONPrinter_Issue_WithLinks(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	p := gogh.NewJSONPrinter(&out)

	issue := &jira4claude.Issue{
		Key:     "TEST-123",
		Summary: "Test issue",
		Status:  "Open",
		Links: []*jira4claude.IssueLink{
			{
				ID: "link-1",
				Type: jira4claude.IssueLinkType{
					Name:    "Blocks",
					Inward:  "is blocked by",
					Outward: "blocks",
				},
				OutwardIssue: &jira4claude.LinkedIssue{
					Key:     "TEST-456",
					Summary: "Blocked issue",
					Status:  "To Do",
				},
			},
		},
	}

	p.Issue(issue)

	var result map[string]any
	err := json.Unmarshal(out.Bytes(), &result)
	require.NoError(t, err)
	links, ok := result["links"].([]any)
	require.True(t, ok, "links should be an array")
	require.Len(t, links, 1)

	link := links[0].(map[string]any)
	assert.Equal(t, "link-1", link["id"])

	linkType := link["type"].(map[string]any)
	assert.Equal(t, "Blocks", linkType["name"])

	outwardIssue := link["outwardIssue"].(map[string]any)
	assert.Equal(t, "TEST-456", outwardIssue["key"])
}

func TestJSONPrinter_Issues(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	p := gogh.NewJSONPrinter(&out)

	issues := []*jira4claude.Issue{
		{Key: "TEST-1", Summary: "First", Status: "Open"},
		{Key: "TEST-2", Summary: "Second", Status: "Done"},
	}

	p.Issues(issues)

	var result []map[string]any
	err := json.Unmarshal(out.Bytes(), &result)
	require.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Equal(t, "TEST-1", result[0]["key"])
	assert.Equal(t, "TEST-2", result[1]["key"])
}

func TestJSONPrinter_Issues_Empty(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	p := gogh.NewJSONPrinter(&out)

	p.Issues([]*jira4claude.Issue{})

	var result []map[string]any
	err := json.Unmarshal(out.Bytes(), &result)
	require.NoError(t, err)
	assert.Empty(t, result)
}

func TestJSONPrinter_Transitions(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	p := gogh.NewJSONPrinter(&out)

	transitions := []*jira4claude.Transition{
		{ID: "1", Name: "In Progress"},
		{ID: "2", Name: "Done"},
	}

	p.Transitions("TEST-123", transitions)

	var result []map[string]any
	err := json.Unmarshal(out.Bytes(), &result)
	require.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Equal(t, "1", result[0]["id"])
	assert.Equal(t, "In Progress", result[0]["name"])
}

func TestJSONPrinter_Links(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	p := gogh.NewJSONPrinter(&out)

	links := []*jira4claude.IssueLink{
		{
			ID: "link-1",
			Type: jira4claude.IssueLinkType{
				Name:    "Blocks",
				Inward:  "is blocked by",
				Outward: "blocks",
			},
			OutwardIssue: &jira4claude.LinkedIssue{
				Key:     "TEST-456",
				Summary: "Blocked issue",
				Status:  "To Do",
			},
		},
	}

	p.Links("TEST-123", links)

	var result []map[string]any
	err := json.Unmarshal(out.Bytes(), &result)
	require.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, "link-1", result[0]["id"])
}

func TestJSONPrinter_Links_WithInwardIssue(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	p := gogh.NewJSONPrinter(&out)

	links := []*jira4claude.IssueLink{
		{
			ID: "link-2",
			Type: jira4claude.IssueLinkType{
				Name:    "Blocks",
				Inward:  "is blocked by",
				Outward: "blocks",
			},
			InwardIssue: &jira4claude.LinkedIssue{
				Key:     "TEST-789",
				Summary: "Blocking issue",
				Status:  "In Progress",
			},
		},
	}

	p.Links("TEST-123", links)

	var result []map[string]any
	err := json.Unmarshal(out.Bytes(), &result)
	require.NoError(t, err)
	require.Len(t, result, 1)

	link := result[0]
	assert.Equal(t, "link-2", link["id"])

	inwardIssue, ok := link["inwardIssue"].(map[string]any)
	require.True(t, ok, "inwardIssue should be a map")
	assert.Equal(t, "TEST-789", inwardIssue["key"])
	assert.Equal(t, "Blocking issue", inwardIssue["summary"])
	assert.Equal(t, "In Progress", inwardIssue["status"])
}

func TestJSONPrinter_Success(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	p := gogh.NewJSONPrinter(&out)

	p.Success("Created issue", "TEST-123")

	var result map[string]any
	err := json.Unmarshal(out.Bytes(), &result)
	require.NoError(t, err)
	assert.Equal(t, true, result["success"])
	assert.Equal(t, "Created issue", result["message"])
	keys, ok := result["keys"].([]any)
	require.True(t, ok)
	assert.Contains(t, keys, "TEST-123")
}

func TestJSONPrinter_Success_NoKeys(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	p := gogh.NewJSONPrinter(&out)

	p.Success("Operation complete")

	var result map[string]any
	err := json.Unmarshal(out.Bytes(), &result)
	require.NoError(t, err)
	assert.Equal(t, true, result["success"])
	assert.Equal(t, "Operation complete", result["message"])
	_, hasKeys := result["keys"]
	assert.False(t, hasKeys, "keys should not be present when no keys provided")
}

func TestJSONPrinter_Error(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	p := gogh.NewJSONPrinter(&out)

	p.Error(errors.New("something went wrong"))

	var result map[string]any
	err := json.Unmarshal(out.Bytes(), &result)
	require.NoError(t, err)
	assert.Equal(t, true, result["error"])
	assert.Equal(t, "something went wrong", result["message"])
}

func TestJSONPrinter_Error_WithCode(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	p := gogh.NewJSONPrinter(&out)

	appErr := &jira4claude.Error{
		Code:    jira4claude.ENotFound,
		Message: "Issue not found",
	}

	p.Error(appErr)

	var result map[string]any
	err := json.Unmarshal(out.Bytes(), &result)
	require.NoError(t, err)
	assert.Equal(t, true, result["error"])
	assert.Equal(t, jira4claude.ENotFound, result["code"])
	assert.Equal(t, "Issue not found", result["message"])
}

// Verify JSONPrinter implements Printer interface at compile time.
// This check is in production code (gogh/json.go), but we verify the test
// file compiles with this assignment as an additional check.
var _ jira4claude.Printer = (*gogh.JSONPrinter)(nil)
