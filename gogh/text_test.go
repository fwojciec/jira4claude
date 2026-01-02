package gogh_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/fwojciec/jira4claude"
	"github.com/fwojciec/jira4claude/gogh"
	"github.com/stretchr/testify/assert"
)

func TestTextPrinter_Issue_ShowsKeyAndSummaryAndStatus(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	io := gogh.NewIO(&out, &errOut)
	p := gogh.NewTextPrinter(io)

	view := jira4claude.IssueView{
		Key:     "TEST-123",
		Summary: "Test issue summary",
		Status:  "In Progress",
		Type:    "Task",
		Created: "2024-01-01T12:00:00Z",
		Updated: "2024-01-02T12:00:00Z",
	}

	p.Issue(view)

	output := out.String()
	assert.Contains(t, output, "TEST-123")
	assert.Contains(t, output, "Test issue summary")
	assert.Contains(t, output, "In Progress")
}

func TestTextPrinter_Issue_ShowsOptionalFields(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	io := gogh.NewIO(&out, &errOut)
	p := gogh.NewTextPrinter(io)

	view := jira4claude.IssueView{
		Key:      "TEST-123",
		Summary:  "Test issue",
		Status:   "Open",
		Type:     "Task",
		Priority: "High",
		Assignee: "John Doe",
		Reporter: "Jane Smith",
		Parent:   "TEST-100",
		Labels:   []string{"bug", "urgent"},
		Created:  "2024-01-01T12:00:00Z",
		Updated:  "2024-01-02T12:00:00Z",
	}

	p.Issue(view)

	output := out.String()
	assert.Contains(t, output, "Priority: High")
	assert.Contains(t, output, "Assignee: John Doe")
	assert.Contains(t, output, "Reporter: Jane Smith")
	assert.Contains(t, output, "TEST-100")
	assert.Contains(t, output, "bug")
	assert.Contains(t, output, "urgent")
}

func TestTextPrinter_Issue_ShowsDescription(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	io := gogh.NewIO(&out, &errOut)
	p := gogh.NewTextPrinter(io)

	view := jira4claude.IssueView{
		Key:         "TEST-123",
		Summary:     "Test issue",
		Status:      "Open",
		Description: "This is the issue description.",
		Created:     "2024-01-01T12:00:00Z",
		Updated:     "2024-01-02T12:00:00Z",
	}

	p.Issue(view)

	output := out.String()
	assert.Contains(t, output, "This is the issue description.")
}

func TestTextPrinter_Issue_ShowsLinks(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	io := gogh.NewIO(&out, &errOut)
	p := gogh.NewTextPrinter(io)

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
			{
				Type:      "is blocked by",
				Direction: "inward",
				IssueKey:  "TEST-789",
				Summary:   "Blocking issue",
				Status:    "Done",
			},
		},
	}

	p.Issue(view)

	output := out.String()
	assert.Contains(t, output, "Links:")
	assert.Contains(t, output, "blocks")
	assert.Contains(t, output, "TEST-456")
	assert.Contains(t, output, "is blocked by")
	assert.Contains(t, output, "TEST-789")
}

func TestTextPrinter_Issue_ShowsLinkedIssueStatus(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	io := gogh.NewIO(&out, &errOut)
	p := gogh.NewTextPrinter(io)

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
			{
				Type:      "is blocked by",
				Direction: "inward",
				IssueKey:  "TEST-789",
				Summary:   "Blocking issue",
				Status:    "Done",
			},
		},
	}

	p.Issue(view)

	output := out.String()
	// Status should appear in brackets before the summary
	assert.Contains(t, output, "[To Do]")
	assert.Contains(t, output, "[Done]")
}

func TestTextPrinter_Issue_ShowsComments(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	io := gogh.NewIO(&out, &errOut)
	p := gogh.NewTextPrinter(io)

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
				Author:  "Jane Smith",
				Body:    "Second comment",
				Created: "2024-01-16T14:20:00Z",
			},
		},
	}

	p.Issue(view)

	output := out.String()
	assert.Contains(t, output, "## Comments")
	assert.Contains(t, output, "John Doe")
	assert.Contains(t, output, "First comment")
	assert.Contains(t, output, "Jane Smith")
	assert.Contains(t, output, "Second comment")
}

func TestTextPrinter_Issue_NoCommentsSection_WhenNoComments(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	io := gogh.NewIO(&out, &errOut)
	p := gogh.NewTextPrinter(io)

	view := jira4claude.IssueView{
		Key:     "TEST-123",
		Summary: "Test issue",
		Status:  "Open",
		Created: "2024-01-01T12:00:00Z",
		Updated: "2024-01-02T12:00:00Z",
	}

	p.Issue(view)

	output := out.String()
	assert.NotContains(t, output, "Comments")
}

func TestTextPrinter_Issues_ShowsTableWithHeaders(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	io := gogh.NewIO(&out, &errOut)
	p := gogh.NewTextPrinter(io)

	views := []jira4claude.IssueView{
		{Key: "TEST-1", Summary: "First issue", Status: "Open", Created: "2024-01-01T12:00:00Z", Updated: "2024-01-02T12:00:00Z"},
		{Key: "TEST-2", Summary: "Second issue", Status: "Done", Created: "2024-01-01T12:00:00Z", Updated: "2024-01-02T12:00:00Z"},
	}

	p.Issues(views)

	output := out.String()
	// Should have headers
	assert.Contains(t, output, "KEY")
	assert.Contains(t, output, "STATUS")
	assert.Contains(t, output, "SUMMARY")
	// Should have issue data
	assert.Contains(t, output, "TEST-1")
	assert.Contains(t, output, "TEST-2")
	assert.Contains(t, output, "First issue")
	assert.Contains(t, output, "Open")
	assert.Contains(t, output, "Done")
}

func TestTextPrinter_Issues_ShowsNoIssuesFoundWhenEmpty(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	io := gogh.NewIO(&out, &errOut)
	p := gogh.NewTextPrinter(io)

	p.Issues([]jira4claude.IssueView{})

	output := out.String()
	assert.Contains(t, output, "No issues found")
}

func TestTextPrinter_Issues_TruncatesLongSummaries(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	io := gogh.NewIO(&out, &errOut)
	p := gogh.NewTextPrinter(io)

	longSummary := "This is a very long summary that should be truncated because it exceeds the maximum allowed length for display in the table format"
	views := []jira4claude.IssueView{
		{Key: "TEST-1", Summary: longSummary, Status: "Open", Created: "2024-01-01T12:00:00Z", Updated: "2024-01-02T12:00:00Z"},
	}

	p.Issues(views)

	output := out.String()
	// Should not contain full summary
	assert.NotContains(t, output, longSummary)
	// Should contain truncation indicator
	assert.Contains(t, output, "...")
}

func TestTextPrinter_Issues_ShowsAssignee(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	io := gogh.NewIO(&out, &errOut)
	p := gogh.NewTextPrinter(io)

	views := []jira4claude.IssueView{
		{
			Key:      "TEST-1",
			Summary:  "Assigned issue",
			Status:   "Open",
			Assignee: "John Doe",
			Created:  "2024-01-01T12:00:00Z",
			Updated:  "2024-01-02T12:00:00Z",
		},
		{
			Key:     "TEST-2",
			Summary: "Unassigned issue",
			Status:  "Open",
			Created: "2024-01-01T12:00:00Z",
			Updated: "2024-01-02T12:00:00Z",
		},
	}

	p.Issues(views)

	output := out.String()
	assert.Contains(t, output, "ASSIGNEE")
	assert.Contains(t, output, "John Doe")
	assert.Contains(t, output, "-") // unassigned marker
}

func TestTextPrinter_Transitions_ShowsAvailableTransitions(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	io := gogh.NewIO(&out, &errOut)
	p := gogh.NewTextPrinter(io)

	transitions := []*jira4claude.Transition{
		{ID: "21", Name: "In Progress"},
		{ID: "31", Name: "Done"},
	}

	p.Transitions("TEST-123", transitions)

	output := out.String()
	assert.Contains(t, output, "TEST-123")
	assert.Contains(t, output, "21")
	assert.Contains(t, output, "In Progress")
	assert.Contains(t, output, "31")
	assert.Contains(t, output, "Done")
}

func TestTextPrinter_Transitions_ShowsEmptyListWithContext(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	io := gogh.NewIO(&out, &errOut)
	p := gogh.NewTextPrinter(io)

	p.Transitions("TEST-123", []*jira4claude.Transition{})

	output := out.String()
	assert.Contains(t, output, "No transitions")
	assert.Contains(t, output, "TEST-123")
	assert.Contains(t, output, "terminal state")
}

func TestTextPrinter_Links_ShowsLinksForIssue(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	io := gogh.NewIO(&out, &errOut)
	p := gogh.NewTextPrinter(io)

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

	output := out.String()
	assert.Contains(t, output, "TEST-123")
	assert.Contains(t, output, "blocks")
	assert.Contains(t, output, "TEST-456")
}

func TestTextPrinter_Links_ShowsLinkedIssueStatus(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	io := gogh.NewIO(&out, &errOut)
	p := gogh.NewTextPrinter(io)

	links := []jira4claude.LinkView{
		{
			Type:      "blocks",
			Direction: "outward",
			IssueKey:  "TEST-456",
			Summary:   "Blocked issue",
			Status:    "To Do",
		},
		{
			Type:      "is blocked by",
			Direction: "inward",
			IssueKey:  "TEST-789",
			Summary:   "Blocking issue",
			Status:    "Done",
		},
	}

	p.Links("TEST-123", links)

	output := out.String()
	// Status should appear in brackets before the summary
	assert.Contains(t, output, "[To Do]")
	assert.Contains(t, output, "[Done]")
}

func TestTextPrinter_Links_ShowsNoLinksMessageWithClarity(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	io := gogh.NewIO(&out, &errOut)
	p := gogh.NewTextPrinter(io)

	p.Links("TEST-123", []jira4claude.LinkView{})

	output := out.String()
	assert.Contains(t, output, "No issue links found")
	assert.Contains(t, output, "TEST-123")
}

func TestTextPrinter_Success_PrintsMessage(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	io := gogh.NewIO(&out, &errOut)
	p := gogh.NewTextPrinter(io)

	p.Success("Created issue", "TEST-123")

	output := out.String()
	assert.Contains(t, output, "Created issue")
	assert.Contains(t, output, "TEST-123")
}

func TestTextPrinter_Success_PrintsMessageWithoutKeys(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	io := gogh.NewIO(&out, &errOut)
	p := gogh.NewTextPrinter(io)

	p.Success("Operation complete")

	output := out.String()
	assert.Contains(t, output, "Operation complete")
}

func TestTextPrinter_Success_PrintsMultipleKeys(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	io := gogh.NewIO(&out, &errOut)
	p := gogh.NewTextPrinter(io)

	p.Success("Created issues", "TEST-1", "TEST-2", "TEST-3")

	output := out.String()
	assert.Contains(t, output, "Created issues")
	assert.Contains(t, output, "TEST-1")
	assert.Contains(t, output, "TEST-2")
	assert.Contains(t, output, "TEST-3")
}

func TestTextPrinter_Error_WritesToStderr(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	io := gogh.NewIO(&out, &errOut)
	p := gogh.NewTextPrinter(io)

	p.Error(errors.New("something went wrong"))

	// Error should go to stderr, not stdout
	assert.Empty(t, out.String(), "errors should not go to stdout")
	assert.Contains(t, errOut.String(), "Error:")
	assert.Contains(t, errOut.String(), "something went wrong")
}

func TestTextPrinter_Error_UsesErrorMessage(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	io := gogh.NewIO(&out, &errOut)
	p := gogh.NewTextPrinter(io)

	appErr := &jira4claude.Error{
		Code:    jira4claude.ENotFound,
		Message: "Issue not found",
	}

	p.Error(appErr)

	assert.Contains(t, errOut.String(), "Issue not found")
}

func TestTextPrinter_Warning_WritesToStderr(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	io := gogh.NewIO(&out, &errOut)
	p := gogh.NewTextPrinter(io)

	p.Warning("unsupported element skipped")

	// Warning should go to stderr, not stdout
	assert.Empty(t, out.String(), "warnings should not go to stdout")
	assert.Contains(t, errOut.String(), "warning:")
	assert.Contains(t, errOut.String(), "unsupported element skipped")
}

func TestTextPrinter_Warning_MultipleWarnings(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	io := gogh.NewIO(&out, &errOut)
	p := gogh.NewTextPrinter(io)

	p.Warning("first warning")
	p.Warning("second warning")

	errOutput := errOut.String()
	assert.Contains(t, errOutput, "first warning")
	assert.Contains(t, errOutput, "second warning")
}

func TestTextPrinter_Issue_ShowsURLWhenSet(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	io := gogh.NewIO(&out, &errOut)
	p := gogh.NewTextPrinter(io)

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

	output := out.String()
	assert.Contains(t, output, "https://example.atlassian.net/browse/TEST-123")
}

func TestTextPrinter_Issue_NoURLWhenEmpty(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	io := gogh.NewIO(&out, &errOut)
	p := gogh.NewTextPrinter(io)

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

	output := out.String()
	assert.NotContains(t, output, "/browse/")
}

func TestTextPrinter_Success_ShowsURLWhenServerURLSet(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	io := gogh.NewIO(&out, &errOut)
	io.ServerURL = "https://example.atlassian.net"
	p := gogh.NewTextPrinter(io)

	p.Success("Created:", "TEST-123")

	output := out.String()
	assert.Contains(t, output, "Created:")
	assert.Contains(t, output, "TEST-123")
	assert.Contains(t, output, "https://example.atlassian.net/browse/TEST-123")
}

func TestTextPrinter_Success_NoURLWhenNoKeys(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	io := gogh.NewIO(&out, &errOut)
	io.ServerURL = "https://example.atlassian.net"
	p := gogh.NewTextPrinter(io)

	p.Success("Operation complete")

	output := out.String()
	assert.Contains(t, output, "Operation complete")
	assert.NotContains(t, output, "/browse/")
}

func TestTextPrinter_Success_ShowsMultipleURLsForMultipleKeys(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	io := gogh.NewIO(&out, &errOut)
	io.ServerURL = "https://example.atlassian.net"
	p := gogh.NewTextPrinter(io)

	p.Success("Created:", "TEST-1", "TEST-2", "TEST-3")

	output := out.String()
	assert.Contains(t, output, "https://example.atlassian.net/browse/TEST-1")
	assert.Contains(t, output, "https://example.atlassian.net/browse/TEST-2")
	assert.Contains(t, output, "https://example.atlassian.net/browse/TEST-3")
}

func TestTextPrinter_Comment_ShowsAuthorTimestampAndBody(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	io := gogh.NewIO(&out, &errOut)
	p := gogh.NewTextPrinter(io)

	view := jira4claude.CommentView{
		ID:      "10001",
		Author:  "John Doe",
		Body:    "This is a test comment",
		Created: "2024-01-15T10:30:00Z",
	}

	p.Comment(view)

	output := out.String()
	assert.Contains(t, output, "John Doe")
	assert.Contains(t, output, "2024-01-15 10:30")
	assert.Contains(t, output, "This is a test comment")
}

func TestTextPrinter_Comment_ShowsUnknownWhenNoAuthor(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	io := gogh.NewIO(&out, &errOut)
	p := gogh.NewTextPrinter(io)

	view := jira4claude.CommentView{
		ID:      "10001",
		Author:  "",
		Body:    "Comment without author",
		Created: "2024-01-15T10:30:00Z",
	}

	p.Comment(view)

	output := out.String()
	assert.Contains(t, output, "Unknown")
	assert.Contains(t, output, "Comment without author")
}

// Verify TextPrinter implements Printer interface at compile time.
var _ jira4claude.Printer = (*gogh.TextPrinter)(nil)
