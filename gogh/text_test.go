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

	issue := &jira4claude.Issue{
		Key:     "TEST-123",
		Summary: "Test issue summary",
		Status:  "In Progress",
		Type:    "Task",
	}

	p.Issue(issue)

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

	issue := &jira4claude.Issue{
		Key:      "TEST-123",
		Summary:  "Test issue",
		Status:   "Open",
		Type:     "Task",
		Priority: "High",
		Assignee: &jira4claude.User{DisplayName: "John Doe"},
		Reporter: &jira4claude.User{DisplayName: "Jane Smith"},
		Parent:   "TEST-100",
		Labels:   []string{"bug", "urgent"},
	}

	p.Issue(issue)

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

	issue := &jira4claude.Issue{
		Key:         "TEST-123",
		Summary:     "Test issue",
		Status:      "Open",
		Description: "This is the issue description.",
	}

	p.Issue(issue)

	output := out.String()
	assert.Contains(t, output, "This is the issue description.")
}

func TestTextPrinter_Issue_ShowsLinks(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	io := gogh.NewIO(&out, &errOut)
	p := gogh.NewTextPrinter(io)

	issue := &jira4claude.Issue{
		Key:     "TEST-123",
		Summary: "Test issue",
		Status:  "Open",
		Links: []*jira4claude.IssueLink{
			{
				Type: jira4claude.IssueLinkType{
					Outward: "blocks",
				},
				OutwardIssue: &jira4claude.LinkedIssue{
					Key:     "TEST-456",
					Summary: "Blocked issue",
				},
			},
			{
				Type: jira4claude.IssueLinkType{
					Inward: "is blocked by",
				},
				InwardIssue: &jira4claude.LinkedIssue{
					Key:     "TEST-789",
					Summary: "Blocking issue",
				},
			},
		},
	}

	p.Issue(issue)

	output := out.String()
	assert.Contains(t, output, "Links:")
	assert.Contains(t, output, "blocks")
	assert.Contains(t, output, "TEST-456")
	assert.Contains(t, output, "is blocked by")
	assert.Contains(t, output, "TEST-789")
}

func TestTextPrinter_Issues_ShowsTableWithHeaders(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	io := gogh.NewIO(&out, &errOut)
	p := gogh.NewTextPrinter(io)

	issues := []*jira4claude.Issue{
		{Key: "TEST-1", Summary: "First issue", Status: "Open"},
		{Key: "TEST-2", Summary: "Second issue", Status: "Done"},
	}

	p.Issues(issues)

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

	p.Issues([]*jira4claude.Issue{})

	output := out.String()
	assert.Contains(t, output, "No issues found")
}

func TestTextPrinter_Issues_TruncatesLongSummaries(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	io := gogh.NewIO(&out, &errOut)
	p := gogh.NewTextPrinter(io)

	longSummary := "This is a very long summary that should be truncated because it exceeds the maximum allowed length for display in the table format"
	issues := []*jira4claude.Issue{
		{Key: "TEST-1", Summary: longSummary, Status: "Open"},
	}

	p.Issues(issues)

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

	issues := []*jira4claude.Issue{
		{
			Key:      "TEST-1",
			Summary:  "Assigned issue",
			Status:   "Open",
			Assignee: &jira4claude.User{DisplayName: "John Doe"},
		},
		{
			Key:     "TEST-2",
			Summary: "Unassigned issue",
			Status:  "Open",
		},
	}

	p.Issues(issues)

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

func TestTextPrinter_Transitions_ShowsEmptyList(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	io := gogh.NewIO(&out, &errOut)
	p := gogh.NewTextPrinter(io)

	p.Transitions("TEST-123", []*jira4claude.Transition{})

	output := out.String()
	assert.Contains(t, output, "TEST-123")
}

func TestTextPrinter_Links_ShowsLinksForIssue(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	io := gogh.NewIO(&out, &errOut)
	p := gogh.NewTextPrinter(io)

	links := []*jira4claude.IssueLink{
		{
			Type: jira4claude.IssueLinkType{
				Outward: "blocks",
			},
			OutwardIssue: &jira4claude.LinkedIssue{
				Key:     "TEST-456",
				Summary: "Blocked issue",
			},
		},
	}

	p.Links("TEST-123", links)

	output := out.String()
	assert.Contains(t, output, "TEST-123")
	assert.Contains(t, output, "blocks")
	assert.Contains(t, output, "TEST-456")
}

func TestTextPrinter_Links_ShowsNoLinksMessage(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	io := gogh.NewIO(&out, &errOut)
	p := gogh.NewTextPrinter(io)

	p.Links("TEST-123", []*jira4claude.IssueLink{})

	output := out.String()
	assert.Contains(t, output, "No links")
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

// Verify TextPrinter implements Printer interface at compile time.
var _ jira4claude.Printer = (*gogh.TextPrinter)(nil)
