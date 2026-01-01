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

func TestJSONPrinter_Issue_WithComments(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	p := gogh.NewJSONPrinter(&out)

	issue := &jira4claude.Issue{
		Key:     "TEST-123",
		Summary: "Test issue",
		Status:  "Open",
		Comments: []*jira4claude.Comment{
			{
				ID: "10001",
				Author: &jira4claude.User{
					AccountID:   "user123",
					DisplayName: "John Doe",
					Email:       "john@example.com",
				},
				Body: "First comment",
			},
			{
				ID:   "10002",
				Body: "Second comment",
			},
		},
	}

	p.Issue(issue)

	var result map[string]any
	err := json.Unmarshal(out.Bytes(), &result)
	require.NoError(t, err)

	comments, ok := result["comments"].([]any)
	require.True(t, ok, "comments should be an array")
	require.Len(t, comments, 2)

	comment1 := comments[0].(map[string]any)
	assert.Equal(t, "10001", comment1["id"])
	assert.Equal(t, "First comment", comment1["body"])
	author1 := comment1["author"].(map[string]any)
	assert.Equal(t, "user123", author1["accountId"])
	assert.Equal(t, "John Doe", author1["displayName"])

	comment2 := comments[1].(map[string]any)
	assert.Equal(t, "10002", comment2["id"])
	assert.Equal(t, "Second comment", comment2["body"])
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

func TestJSONPrinter_Issue_WithServerURL(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	p := gogh.NewJSONPrinter(&out)
	p.SetServerURL("https://example.atlassian.net")

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
	assert.Equal(t, "https://example.atlassian.net/browse/TEST-123", result["url"])
}

func TestJSONPrinter_Issue_NoURLWhenServerURLEmpty(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	p := gogh.NewJSONPrinter(&out)
	// ServerURL not set

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
	_, hasURL := result["url"]
	assert.False(t, hasURL, "url should not be present when serverURL is empty")
}

func TestJSONPrinter_Success_WithServerURL(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	p := gogh.NewJSONPrinter(&out)
	p.SetServerURL("https://example.atlassian.net")

	p.Success("Created:", "TEST-123")

	var result map[string]any
	err := json.Unmarshal(out.Bytes(), &result)
	require.NoError(t, err)
	assert.Equal(t, true, result["success"])
	assert.Equal(t, "Created:", result["message"])
	urls, ok := result["urls"].([]any)
	require.True(t, ok, "urls should be an array")
	assert.Contains(t, urls, "https://example.atlassian.net/browse/TEST-123")
}

func TestJSONPrinter_Success_NoURLsWhenNoKeys(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	p := gogh.NewJSONPrinter(&out)
	p.SetServerURL("https://example.atlassian.net")

	p.Success("Operation complete")

	var result map[string]any
	err := json.Unmarshal(out.Bytes(), &result)
	require.NoError(t, err)
	_, hasURLs := result["urls"]
	assert.False(t, hasURLs, "urls should not be present when no keys provided")
}

func TestJSONPrinter_Success_ShowsMultipleURLsForMultipleKeys(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	p := gogh.NewJSONPrinter(&out)
	p.SetServerURL("https://example.atlassian.net")

	p.Success("Created:", "TEST-1", "TEST-2", "TEST-3")

	var result map[string]any
	err := json.Unmarshal(out.Bytes(), &result)
	require.NoError(t, err)
	urls, ok := result["urls"].([]any)
	require.True(t, ok, "urls should be an array")
	require.Len(t, urls, 3)
	assert.Equal(t, "https://example.atlassian.net/browse/TEST-1", urls[0])
	assert.Equal(t, "https://example.atlassian.net/browse/TEST-2", urls[1])
	assert.Equal(t, "https://example.atlassian.net/browse/TEST-3", urls[2])
}

func TestJSONPrinter_Comment(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	p := gogh.NewJSONPrinter(&out)

	comment := &jira4claude.Comment{
		ID: "10001",
		Author: &jira4claude.User{
			AccountID:   "user123",
			DisplayName: "John Doe",
			Email:       "john@example.com",
		},
		Body:    "This is a test comment",
		Created: parseTime("2024-01-15T10:30:00.000+0000"),
	}

	p.Comment(comment)

	var result map[string]any
	err := json.Unmarshal(out.Bytes(), &result)
	require.NoError(t, err)
	assert.Equal(t, "10001", result["id"])
	assert.Equal(t, "This is a test comment", result["body"])

	author, ok := result["author"].(map[string]any)
	require.True(t, ok, "author should be a map")
	assert.Equal(t, "user123", author["accountId"])
	assert.Equal(t, "John Doe", author["displayName"])
}

func TestJSONPrinter_Comment_WithoutAuthor(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	p := gogh.NewJSONPrinter(&out)

	comment := &jira4claude.Comment{
		ID:      "10002",
		Body:    "Comment without author",
		Created: parseTime("2024-01-15T10:30:00.000+0000"),
	}

	p.Comment(comment)

	var result map[string]any
	err := json.Unmarshal(out.Bytes(), &result)
	require.NoError(t, err)
	assert.Equal(t, "10002", result["id"])
	assert.Equal(t, "Comment without author", result["body"])
	_, hasAuthor := result["author"]
	assert.False(t, hasAuthor, "author should not be present when nil")
}

func TestJSONPrinter_Warning_WritesToStderr(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	p := gogh.NewJSONPrinterWithIO(&out, &errOut)

	p.Warning("unsupported element skipped")

	// Warning should go to stderr, not stdout
	assert.Empty(t, out.String(), "warnings should not go to stdout")
	assert.Contains(t, errOut.String(), "warning:")
	assert.Contains(t, errOut.String(), "unsupported element skipped")
}

func TestJSONPrinter_Warning_NotInJSONFormat(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	p := gogh.NewJSONPrinterWithIO(&out, &errOut)

	p.Warning("unsupported element skipped")

	// Warning should be plain text, not JSON
	errOutput := errOut.String()
	assert.NotContains(t, errOutput, "{", "warnings should be plain text, not JSON")
}

func TestJSONPrinter_Warning_MultipleWarnings(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	p := gogh.NewJSONPrinterWithIO(&out, &errOut)

	p.Warning("first warning")
	p.Warning("second warning")

	errOutput := errOut.String()
	assert.Contains(t, errOutput, "first warning")
	assert.Contains(t, errOutput, "second warning")
}

// Verify JSONPrinter implements Printer interface at compile time.
// This check is in production code (gogh/json.go), but we verify the test
// file compiles with this assignment as an additional check.
var _ jira4claude.Printer = (*gogh.JSONPrinter)(nil)
