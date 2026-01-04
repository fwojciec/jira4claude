package json_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"

	"github.com/fwojciec/jira4claude"
	jsonpkg "github.com/fwojciec/jira4claude/json"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPrinter_Issue(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	p := jsonpkg.NewPrinter(&out)

	view := jira4claude.IssueView{
		Key:     "TEST-123",
		Summary: "Test issue",
		Status:  "Open",
		Type:    "Task",
		Created: "2024-01-01T12:00:00Z",
		Updated: "2024-01-02T12:00:00Z",
	}

	p.Issue(view)

	var result map[string]any
	err := json.Unmarshal(out.Bytes(), &result)
	require.NoError(t, err)
	assert.Equal(t, "TEST-123", result["key"])
	assert.Equal(t, "Test issue", result["summary"])
	assert.Equal(t, "Open", result["status"])
}

func TestPrinter_Issue_WithAssignee(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	p := jsonpkg.NewPrinter(&out)

	view := jira4claude.IssueView{
		Key:      "TEST-123",
		Summary:  "Test issue",
		Status:   "Open",
		Assignee: "John Doe",
		Created:  "2024-01-01T12:00:00Z",
		Updated:  "2024-01-02T12:00:00Z",
	}

	p.Issue(view)

	var result map[string]any
	err := json.Unmarshal(out.Bytes(), &result)
	require.NoError(t, err)
	assert.Equal(t, "John Doe", result["assignee"])
}

func TestPrinter_Issue_WithReporter(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	p := jsonpkg.NewPrinter(&out)

	view := jira4claude.IssueView{
		Key:      "TEST-123",
		Summary:  "Test issue",
		Status:   "Open",
		Reporter: "Jane Smith",
		Created:  "2024-01-01T12:00:00Z",
		Updated:  "2024-01-02T12:00:00Z",
	}

	p.Issue(view)

	var result map[string]any
	err := json.Unmarshal(out.Bytes(), &result)
	require.NoError(t, err)
	assert.Equal(t, "Jane Smith", result["reporter"])
}

func TestPrinter_Issue_WithLinks(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	p := jsonpkg.NewPrinter(&out)

	view := jira4claude.IssueView{
		Key:     "TEST-123",
		Summary: "Test issue",
		Status:  "Open",
		Created: "2024-01-01T12:00:00Z",
		Updated: "2024-01-02T12:00:00Z",
		Links: []jira4claude.LinkView{
			{
				Type:      "blocks",
				Direction: "outward",
				IssueKey:  "TEST-456",
				Summary:   "Blocked issue",
				Status:    "To Do",
			},
		},
	}

	p.Issue(view)

	var result map[string]any
	err := json.Unmarshal(out.Bytes(), &result)
	require.NoError(t, err)
	links, ok := result["links"].([]any)
	require.True(t, ok, "links should be an array")
	require.Len(t, links, 1)

	link := links[0].(map[string]any)
	assert.Equal(t, "blocks", link["type"])
	assert.Equal(t, "outward", link["direction"])
	assert.Equal(t, "TEST-456", link["issueKey"])
}

func TestPrinter_Issue_WithComments(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	p := jsonpkg.NewPrinter(&out)

	view := jira4claude.IssueView{
		Key:     "TEST-123",
		Summary: "Test issue",
		Status:  "Open",
		Created: "2024-01-01T12:00:00Z",
		Updated: "2024-01-02T12:00:00Z",
		Comments: []jira4claude.CommentView{
			{
				ID:      "10001",
				Author:  "John Doe",
				Body:    "First comment",
				Created: "2024-01-15T10:30:00Z",
			},
			{
				ID:      "10002",
				Author:  "",
				Body:    "Second comment",
				Created: "2024-01-16T14:00:00Z",
			},
		},
	}

	p.Issue(view)

	var result map[string]any
	err := json.Unmarshal(out.Bytes(), &result)
	require.NoError(t, err)

	comments, ok := result["comments"].([]any)
	require.True(t, ok, "comments should be an array")
	require.Len(t, comments, 2)

	comment1 := comments[0].(map[string]any)
	assert.Equal(t, "10001", comment1["id"])
	assert.Equal(t, "First comment", comment1["body"])
	assert.Equal(t, "John Doe", comment1["author"])

	comment2 := comments[1].(map[string]any)
	assert.Equal(t, "10002", comment2["id"])
	assert.Equal(t, "Second comment", comment2["body"])
}

func TestPrinter_Issues(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	p := jsonpkg.NewPrinter(&out)

	views := []jira4claude.IssueView{
		{Key: "TEST-1", Summary: "First", Status: "Open", Created: "2024-01-01T12:00:00Z", Updated: "2024-01-02T12:00:00Z"},
		{Key: "TEST-2", Summary: "Second", Status: "Done", Created: "2024-01-01T12:00:00Z", Updated: "2024-01-02T12:00:00Z"},
	}

	p.Issues(views)

	var result []map[string]any
	err := json.Unmarshal(out.Bytes(), &result)
	require.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Equal(t, "TEST-1", result[0]["key"])
	assert.Equal(t, "TEST-2", result[1]["key"])
}

func TestPrinter_Issues_Empty(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	p := jsonpkg.NewPrinter(&out)

	p.Issues([]jira4claude.IssueView{})

	var result []map[string]any
	err := json.Unmarshal(out.Bytes(), &result)
	require.NoError(t, err)
	assert.Empty(t, result)
}

func TestPrinter_Transitions(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	p := jsonpkg.NewPrinter(&out)

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

func TestPrinter_Links(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	p := jsonpkg.NewPrinter(&out)

	links := []jira4claude.LinkView{
		{
			Type:      "blocks",
			Direction: "outward",
			IssueKey:  "TEST-456",
			Summary:   "Blocked issue",
			Status:    "To Do",
		},
	}

	p.Links("TEST-123", links)

	var result []map[string]any
	err := json.Unmarshal(out.Bytes(), &result)
	require.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, "blocks", result[0]["type"])
	assert.Equal(t, "TEST-456", result[0]["issueKey"])
}

func TestPrinter_Links_WithInwardIssue(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	p := jsonpkg.NewPrinter(&out)

	links := []jira4claude.LinkView{
		{
			Type:      "is blocked by",
			Direction: "inward",
			IssueKey:  "TEST-789",
			Summary:   "Blocking issue",
			Status:    "In Progress",
		},
	}

	p.Links("TEST-123", links)

	var result []map[string]any
	err := json.Unmarshal(out.Bytes(), &result)
	require.NoError(t, err)
	require.Len(t, result, 1)

	link := result[0]
	assert.Equal(t, "is blocked by", link["type"])
	assert.Equal(t, "inward", link["direction"])
	assert.Equal(t, "TEST-789", link["issueKey"])
	assert.Equal(t, "In Progress", link["status"])
}

func TestPrinter_Success(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	p := jsonpkg.NewPrinter(&out)

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

func TestPrinter_Success_NoKeys(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	p := jsonpkg.NewPrinter(&out)

	p.Success("Operation complete")

	var result map[string]any
	err := json.Unmarshal(out.Bytes(), &result)
	require.NoError(t, err)
	assert.Equal(t, true, result["success"])
	assert.Equal(t, "Operation complete", result["message"])
	_, hasKeys := result["keys"]
	assert.False(t, hasKeys, "keys should not be present when no keys provided")
}

func TestPrinter_Error(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	p := jsonpkg.NewPrinter(&out)

	p.Error(errors.New("something went wrong"))

	var result map[string]any
	err := json.Unmarshal(out.Bytes(), &result)
	require.NoError(t, err)
	assert.Equal(t, true, result["error"])
	assert.Equal(t, "something went wrong", result["message"])
}

func TestPrinter_Error_WithCode(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	p := jsonpkg.NewPrinter(&out)

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

func TestPrinter_Issue_WithURL(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	p := jsonpkg.NewPrinter(&out)

	view := jira4claude.IssueView{
		Key:     "TEST-123",
		Summary: "Test issue",
		Status:  "Open",
		Type:    "Task",
		Created: "2024-01-01T12:00:00Z",
		Updated: "2024-01-02T12:00:00Z",
		URL:     "https://example.atlassian.net/browse/TEST-123",
	}

	p.Issue(view)

	var result map[string]any
	err := json.Unmarshal(out.Bytes(), &result)
	require.NoError(t, err)
	assert.Equal(t, "https://example.atlassian.net/browse/TEST-123", result["url"])
}

func TestPrinter_Issue_NoURLWhenEmpty(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	p := jsonpkg.NewPrinter(&out)

	view := jira4claude.IssueView{
		Key:     "TEST-123",
		Summary: "Test issue",
		Status:  "Open",
		Type:    "Task",
		Created: "2024-01-01T12:00:00Z",
		Updated: "2024-01-02T12:00:00Z",
		// URL not set
	}

	p.Issue(view)

	var result map[string]any
	err := json.Unmarshal(out.Bytes(), &result)
	require.NoError(t, err)
	_, hasURL := result["url"]
	assert.False(t, hasURL, "url should not be present when empty")
}

func TestPrinter_Success_WithServerURL(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	p := jsonpkg.NewPrinter(&out)
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

func TestPrinter_Success_NoURLsWhenNoKeys(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	p := jsonpkg.NewPrinter(&out)
	p.SetServerURL("https://example.atlassian.net")

	p.Success("Operation complete")

	var result map[string]any
	err := json.Unmarshal(out.Bytes(), &result)
	require.NoError(t, err)
	_, hasURLs := result["urls"]
	assert.False(t, hasURLs, "urls should not be present when no keys provided")
}

func TestPrinter_Success_ShowsMultipleURLsForMultipleKeys(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	p := jsonpkg.NewPrinter(&out)
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

func TestPrinter_Comment(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	p := jsonpkg.NewPrinter(&out)

	view := jira4claude.CommentView{
		ID:      "10001",
		Author:  "John Doe",
		Body:    "This is a test comment",
		Created: "2024-01-15T10:30:00Z",
	}

	p.Comment(view)

	var result map[string]any
	err := json.Unmarshal(out.Bytes(), &result)
	require.NoError(t, err)
	assert.Equal(t, "10001", result["id"])
	assert.Equal(t, "This is a test comment", result["body"])
	assert.Equal(t, "John Doe", result["author"])
}

func TestPrinter_Comment_WithoutAuthor(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	p := jsonpkg.NewPrinter(&out)

	view := jira4claude.CommentView{
		ID:      "10002",
		Author:  "",
		Body:    "Comment without author",
		Created: "2024-01-15T10:30:00Z",
	}

	p.Comment(view)

	var result map[string]any
	err := json.Unmarshal(out.Bytes(), &result)
	require.NoError(t, err)
	assert.Equal(t, "10002", result["id"])
	assert.Equal(t, "Comment without author", result["body"])
	// Author is empty string; without omitempty it appears as "author": ""
	assert.Empty(t, result["author"])
}

func TestPrinter_Warning_WritesToStderr(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	p := jsonpkg.NewPrinterWithIO(&out, &errOut)

	p.Warning("unsupported element skipped")

	// Warning should go to stderr, not stdout
	assert.Empty(t, out.String(), "warnings should not go to stdout")
	assert.Contains(t, errOut.String(), "warning:")
	assert.Contains(t, errOut.String(), "unsupported element skipped")
}

func TestPrinter_Warning_NotInJSONFormat(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	p := jsonpkg.NewPrinterWithIO(&out, &errOut)

	p.Warning("unsupported element skipped")

	// Warning should be plain text, not JSON
	errOutput := errOut.String()
	assert.NotContains(t, errOutput, "{", "warnings should be plain text, not JSON")
}

func TestPrinter_Warning_MultipleWarnings(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	p := jsonpkg.NewPrinterWithIO(&out, &errOut)

	p.Warning("first warning")
	p.Warning("second warning")

	errOutput := errOut.String()
	assert.Contains(t, errOutput, "first warning")
	assert.Contains(t, errOutput, "second warning")
}
