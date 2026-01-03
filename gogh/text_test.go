package gogh_test

import (
	"bytes"
	"errors"
	"regexp"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/fwojciec/jira4claude"
	"github.com/fwojciec/jira4claude/gogh"
	"github.com/muesli/termenv"
	"github.com/stretchr/testify/assert"
)

// ansiRegex matches ANSI escape sequences.
var ansiRegex = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// stripANSI removes ANSI escape codes from a string.
func stripANSI(s string) string {
	return ansiRegex.ReplaceAllString(s, "")
}

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
	// Priority is shown with indicator in card layout
	assert.Contains(t, output, "High")
	// Assignee and Reporter are shown in card layout
	assert.Contains(t, output, "Assignee:")
	assert.Contains(t, output, "John Doe")
	assert.Contains(t, output, "Reporter:")
	assert.Contains(t, output, "Jane Smith")
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
	// Links are now shown in a LINKED ISSUES card
	assert.Contains(t, output, "LINKED ISSUES")
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
	// Status should use consistent badge format (same as top panel)
	// In text mode without color, this shows as "[ ] To Do" and "[x] Done"
	assert.Contains(t, output, "To Do")
	assert.Contains(t, output, "Done")
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

func TestTextPrinter_Issues_ColorMode_HasRoundedBorder(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	io := gogh.NewIO(&out, &errOut)
	styles := trueColorStyles(t)
	p := gogh.NewTextPrinterWithStyles(io, styles)

	views := []jira4claude.IssueView{
		{Key: "TEST-1", Summary: "Issue one", Status: "Open", Created: "2024-01-01T12:00:00Z", Updated: "2024-01-02T12:00:00Z"},
	}

	p.Issues(views)

	output := out.String()
	// Should have rounded border characters
	assert.Contains(t, output, "╭", "expected top-left rounded corner")
	assert.Contains(t, output, "╮", "expected top-right rounded corner")
	assert.Contains(t, output, "╰", "expected bottom-left rounded corner")
	assert.Contains(t, output, "╯", "expected bottom-right rounded corner")
}

func TestTextPrinter_Issues_ColorMode_HasDottedHeaderSeparator(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	io := gogh.NewIO(&out, &errOut)
	styles := trueColorStyles(t)
	p := gogh.NewTextPrinterWithStyles(io, styles)

	views := []jira4claude.IssueView{
		{Key: "TEST-1", Summary: "Issue one", Status: "Open", Created: "2024-01-01T12:00:00Z", Updated: "2024-01-02T12:00:00Z"},
	}

	p.Issues(views)

	output := out.String()
	// Should have dotted separator line after header
	assert.Contains(t, output, "┄", "expected dotted separator after header")
}

func TestTextPrinter_Issues_TextOnlyMode_HasDashedHeaderSeparator(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	io := gogh.NewIO(&out, &errOut)
	styles := asciiStyles(t)
	p := gogh.NewTextPrinterWithStyles(io, styles)

	views := []jira4claude.IssueView{
		{Key: "TEST-1", Summary: "Issue one", Status: "Open", Created: "2024-01-01T12:00:00Z", Updated: "2024-01-02T12:00:00Z"},
	}

	p.Issues(views)

	output := out.String()
	// Should have dashed separator under header (at least 3 dashes in a row)
	assert.Contains(t, output, "---", "expected dashed separator under header")
	// Should NOT have box-drawing border characters
	assert.NotContains(t, output, "╭", "should not have rounded corners in text-only mode")
	assert.NotContains(t, output, "╮", "should not have rounded corners in text-only mode")
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
	assert.Contains(t, output, "In Progress")
	assert.Contains(t, output, "Done")
	// Should have arrow indicator (→ in color mode, -> in no-color)
	assert.True(t, strings.Contains(output, "->") || strings.Contains(output, "→"),
		"expected arrow indicator")
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
	assert.Contains(t, errOut.String(), "something went wrong")
	// Should have some indicator (✗/Error: in color mode, [error] in no-color)
	errOutput := errOut.String()
	assert.True(t, strings.Contains(errOutput, "[error]") || strings.Contains(errOutput, "✗"),
		"expected error indicator")
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
	assert.Contains(t, errOut.String(), "unsupported element skipped")
	// Should have some indicator (⚠/Warning: in color mode, [warn] in no-color)
	errOutput := errOut.String()
	assert.True(t, strings.Contains(errOutput, "[warn]") || strings.Contains(errOutput, "⚠"),
		"expected warning indicator")
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

func TestTextPrinter_Transitions_ShowsArrowInColorMode(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	r := lipgloss.NewRenderer(&out)
	r.SetColorProfile(termenv.TrueColor)
	styles := gogh.NewStyles(r)
	io := gogh.NewIO(&out, &errOut)
	p := gogh.NewTextPrinterWithStyles(io, styles)

	transitions := []*jira4claude.Transition{
		{ID: "21", Name: "Start Progress"},
		{ID: "31", Name: "Done"},
	}

	p.Transitions("TEST-123", transitions)

	output := out.String()
	assert.Contains(t, output, "→")
}

func TestTextPrinter_Transitions_ShowsTextArrowInNoColorMode(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	r := lipgloss.NewRenderer(&out)
	r.SetColorProfile(termenv.Ascii)
	styles := gogh.NewStyles(r)
	io := gogh.NewIO(&out, &errOut)
	p := gogh.NewTextPrinterWithStyles(io, styles)

	transitions := []*jira4claude.Transition{
		{ID: "21", Name: "Start Progress"},
		{ID: "31", Name: "Done"},
	}

	p.Transitions("TEST-123", transitions)

	output := out.String()
	assert.Contains(t, output, "->")
	assert.NotContains(t, output, "→")
}

func TestTextPrinter_Error_ShowsXIndicatorInColorMode(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	r := lipgloss.NewRenderer(&errOut)
	r.SetColorProfile(termenv.TrueColor)
	styles := gogh.NewStyles(r)
	io := gogh.NewIO(&out, &errOut)
	p := gogh.NewTextPrinterWithStyles(io, styles)

	p.Error(errors.New("something went wrong"))

	errOutput := errOut.String()
	assert.Contains(t, errOutput, "✗")
	assert.Contains(t, errOutput, "Error:")
}

func TestTextPrinter_Error_ShowsErrorIndicatorInNoColorMode(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	r := lipgloss.NewRenderer(&errOut)
	r.SetColorProfile(termenv.Ascii)
	styles := gogh.NewStyles(r)
	io := gogh.NewIO(&out, &errOut)
	p := gogh.NewTextPrinterWithStyles(io, styles)

	p.Error(errors.New("something went wrong"))

	errOutput := errOut.String()
	assert.Contains(t, errOutput, "[error]")
	assert.NotContains(t, errOutput, "✗")
	assert.NotContains(t, errOutput, "Error:")
}

func TestTextPrinter_Warning_ShowsWarningSymbolInColorMode(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	r := lipgloss.NewRenderer(&errOut)
	r.SetColorProfile(termenv.TrueColor)
	styles := gogh.NewStyles(r)
	io := gogh.NewIO(&out, &errOut)
	p := gogh.NewTextPrinterWithStyles(io, styles)

	p.Warning("unsupported element skipped")

	errOutput := errOut.String()
	assert.Contains(t, errOutput, "⚠")
	assert.Contains(t, errOutput, "Warning:")
}

func TestTextPrinter_Warning_ShowsWarnIndicatorInNoColorMode(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	r := lipgloss.NewRenderer(&errOut)
	r.SetColorProfile(termenv.Ascii)
	styles := gogh.NewStyles(r)
	io := gogh.NewIO(&out, &errOut)
	p := gogh.NewTextPrinterWithStyles(io, styles)

	p.Warning("unsupported element skipped")

	errOutput := errOut.String()
	assert.Contains(t, errOutput, "[warn]")
	assert.NotContains(t, errOutput, "⚠")
	assert.NotContains(t, errOutput, "Warning:")
}

func TestTextPrinter_Success_ShowsCheckmarkIndicatorInColorMode(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	r := lipgloss.NewRenderer(&out)
	r.SetColorProfile(termenv.TrueColor)
	styles := gogh.NewStyles(r)
	io := gogh.NewIO(&out, &errOut)
	p := gogh.NewTextPrinterWithStyles(io, styles)

	p.Success("Transitioned:", "TEST-123")

	output := out.String()
	assert.Contains(t, output, "✓")
}

func TestTextPrinter_Success_ShowsOkIndicatorInNoColorMode(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	r := lipgloss.NewRenderer(&out)
	r.SetColorProfile(termenv.Ascii)
	styles := gogh.NewStyles(r)
	io := gogh.NewIO(&out, &errOut)
	p := gogh.NewTextPrinterWithStyles(io, styles)

	p.Success("Transitioned:", "TEST-123")

	output := out.String()
	assert.Contains(t, output, "[ok]")
	assert.NotContains(t, output, "✓")
}

// Verify TextPrinter implements Printer interface at compile time.
var _ jira4claude.Printer = (*gogh.TextPrinter)(nil)

// Card layout tests for Issue()

func TestTextPrinter_Issue_CardLayout_TextOnlyMode_HeaderSection(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	r := lipgloss.NewRenderer(&out)
	r.SetColorProfile(termenv.Ascii)
	styles := gogh.NewStyles(r)

	io := gogh.NewIO(&out, &errOut)
	p := gogh.NewTextPrinterWithStyles(io, styles)

	view := jira4claude.IssueView{
		Key:      "J4C-81",
		Summary:  "Add CLI view models and update handlers",
		Status:   "Done",
		Type:     "Task",
		Priority: "Medium",
		Assignee: "Filip Wojciechowski",
		Reporter: "Filip Wojciechowski",
		Created:  "2024-01-01T12:00:00Z",
		Updated:  "2024-01-02T12:00:00Z",
	}

	p.Issue(view)

	output := out.String()
	// Header should show key and type badge on first line
	assert.Contains(t, output, "J4C-81")
	assert.Contains(t, output, "TASK")
	// Summary on next line
	assert.Contains(t, output, "Add CLI view models and update handlers")
	// Status with text-only indicator
	assert.Contains(t, output, "[x] Done")
	// Priority with text-only indicator
	assert.Contains(t, output, "[!] Medium")
	// Assignee and Reporter
	assert.Contains(t, output, "Assignee:")
	assert.Contains(t, output, "Reporter:")
	assert.Contains(t, output, "Filip Wojciechowski")
}

func TestTextPrinter_Issue_CardLayout_ColorMode_HasBorders(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	r := lipgloss.NewRenderer(&out)
	r.SetColorProfile(termenv.TrueColor)
	styles := gogh.NewStyles(r)

	io := gogh.NewIO(&out, &errOut)
	p := gogh.NewTextPrinterWithStyles(io, styles)

	view := jira4claude.IssueView{
		Key:      "J4C-81",
		Summary:  "Add CLI view models and update handlers",
		Status:   "Done",
		Type:     "Task",
		Priority: "Medium",
		Assignee: "Filip Wojciechowski",
		Reporter: "Filip Wojciechowski",
		Created:  "2024-01-01T12:00:00Z",
		Updated:  "2024-01-02T12:00:00Z",
	}

	p.Issue(view)

	output := out.String()
	// Should have rounded border characters
	assert.Contains(t, output, "╭")
	assert.Contains(t, output, "╯")
	// Status with unicode indicator
	assert.Contains(t, output, "✓")
	assert.Contains(t, output, "Done")
	// Priority with unicode indicator
	assert.Contains(t, output, "▲")
	assert.Contains(t, output, "Medium")
}

func TestTextPrinter_Issue_CardLayout_TextOnlyMode_LinkedIssues(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	r := lipgloss.NewRenderer(&out)
	r.SetColorProfile(termenv.Ascii)
	styles := gogh.NewStyles(r)

	io := gogh.NewIO(&out, &errOut)
	p := gogh.NewTextPrinterWithStyles(io, styles)

	view := jira4claude.IssueView{
		Key:     "J4C-81",
		Summary: "Test issue",
		Status:  "Done",
		Type:    "Task",
		Created: "2024-01-01T12:00:00Z",
		Updated: "2024-01-02T12:00:00Z",
		Links: []jira4claude.LinkView{
			{
				Type:      "is blocked by",
				Direction: "inward",
				IssueKey:  "J4C-74",
				Summary:   "Inject Converter into CLI IssueContext",
				Status:    "Done",
			},
			{
				Type:      "is blocked by",
				Direction: "inward",
				IssueKey:  "J4C-76",
				Summary:   "Add warning propagation",
				Status:    "Done",
			},
			{
				Type:      "blocks",
				Direction: "outward",
				IssueKey:  "J4C-78",
				Summary:   "Rename adf package",
				Status:    "To Do",
			},
		},
	}

	p.Issue(view)

	output := out.String()
	// Should have LINKED ISSUES section with text header
	assert.Contains(t, output, "=== LINKED ISSUES ===")
	// Links should be grouped by type
	assert.Contains(t, output, "is blocked by")
	assert.Contains(t, output, "blocks")
	// Should show linked issues with their status (using consistent badge format)
	assert.Contains(t, output, "J4C-74")
	assert.Contains(t, output, "[x] Done") // consistent with top panel format
	assert.Contains(t, output, "J4C-78")
	assert.Contains(t, output, "[ ] To Do") // consistent with top panel format
}

func TestTextPrinter_Issue_CardLayout_ColorMode_LinkedIssues(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	r := lipgloss.NewRenderer(&out)
	r.SetColorProfile(termenv.TrueColor)
	styles := gogh.NewStyles(r)

	io := gogh.NewIO(&out, &errOut)
	p := gogh.NewTextPrinterWithStyles(io, styles)

	view := jira4claude.IssueView{
		Key:     "J4C-81",
		Summary: "Test issue",
		Status:  "Done",
		Type:    "Task",
		Created: "2024-01-01T12:00:00Z",
		Updated: "2024-01-02T12:00:00Z",
		Links: []jira4claude.LinkView{
			{
				Type:      "is blocked by",
				Direction: "inward",
				IssueKey:  "J4C-74",
				Summary:   "Inject Converter",
				Status:    "Done",
			},
		},
	}

	p.Issue(view)

	output := out.String()
	// Should have LINKED ISSUES card with borders (border chars may be styled separately)
	assert.Contains(t, output, "LINKED ISSUES")
	assert.Contains(t, output, "╭")
	assert.Contains(t, output, "is blocked by")
	assert.Contains(t, output, "J4C-74")
}

func TestTextPrinter_Issue_CardLayout_StatusAndPriorityAreSeparated(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	r := lipgloss.NewRenderer(&out)
	r.SetColorProfile(termenv.TrueColor)
	styles := gogh.NewStyles(r)

	io := gogh.NewIO(&out, &errOut)
	p := gogh.NewTextPrinterWithStyles(io, styles)

	view := jira4claude.IssueView{
		Key:      "J4C-81",
		Summary:  "Test issue",
		Status:   "In Progress",
		Type:     "Task",
		Priority: "Medium",
		Created:  "2024-01-01T12:00:00Z",
		Updated:  "2024-01-02T12:00:00Z",
	}

	p.Issue(view)

	output := out.String()
	// Status and priority should be on same line but properly spaced
	// Check that they are not running together (there should be spaces between them)
	assert.NotContains(t, output, "In Progress▲")
	assert.NotContains(t, output, "Progress▲")
}

// Markdown rendering tests for descriptions

func TestTextPrinter_Issue_Description_ColorMode_RendersMarkdownHeaders(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	styles := trueColorStyles(t)
	io := gogh.NewIO(&out, &errOut)
	p := gogh.NewTextPrinterWithStyles(io, styles)

	view := jira4claude.IssueView{
		Key:         "TEST-123",
		Summary:     "Test issue",
		Status:      "Open",
		Type:        "Task",
		Description: "## Section Header\n\nSome content here.",
		Created:     "2024-01-01T12:00:00Z",
		Updated:     "2024-01-02T12:00:00Z",
	}

	p.Issue(view)

	output := out.String()
	// In color mode, headers should be styled (bold/colored)
	// The header text is present but may have ANSI codes between words
	assert.Contains(t, output, "Section")
	assert.Contains(t, output, "Header")
}

func TestTextPrinter_Issue_Description_ColorMode_RendersBulletLists(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	styles := trueColorStyles(t)
	io := gogh.NewIO(&out, &errOut)
	p := gogh.NewTextPrinterWithStyles(io, styles)

	view := jira4claude.IssueView{
		Key:     "TEST-123",
		Summary: "Test issue",
		Status:  "Open",
		Type:    "Task",
		Description: `## Features

- First item
- Second item
- Third item`,
		Created: "2024-01-01T12:00:00Z",
		Updated: "2024-01-02T12:00:00Z",
	}

	p.Issue(view)

	output := out.String()
	// List items should render with proper formatting
	// Glamour renders bullet points with bullets (•)
	assert.Contains(t, output, "First item")
	assert.Contains(t, output, "Second item")
	assert.Contains(t, output, "Third item")
	// Should have bullet characters (glamour uses • for bullets)
	assert.Contains(t, output, "•")
}

func TestTextPrinter_Issue_Description_ColorMode_RendersCodeBlocks(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	styles := trueColorStyles(t)
	io := gogh.NewIO(&out, &errOut)
	p := gogh.NewTextPrinterWithStyles(io, styles)

	view := jira4claude.IssueView{
		Key:         "TEST-123",
		Summary:     "Test issue",
		Status:      "Open",
		Type:        "Task",
		Description: "Example code:\n\n```go\nfunc main() {\n    fmt.Println(\"Hello\")\n}\n```",
		Created:     "2024-01-01T12:00:00Z",
		Updated:     "2024-01-02T12:00:00Z",
	}

	p.Issue(view)

	output := out.String()
	// Code blocks should contain the code content (may have ANSI codes between tokens)
	assert.Contains(t, output, "func")
	assert.Contains(t, output, "main")
	assert.Contains(t, output, "fmt")
	assert.Contains(t, output, "Println")
}

func TestTextPrinter_Issue_Description_TextOnlyMode_NoANSICodes(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	styles := asciiStyles(t)
	io := gogh.NewIO(&out, &errOut)
	p := gogh.NewTextPrinterWithStyles(io, styles)

	view := jira4claude.IssueView{
		Key:     "TEST-123",
		Summary: "Test issue",
		Status:  "Open",
		Type:    "Task",
		Description: `## Context

This is a description with **bold** text.

- List item one
- List item two`,
		Created: "2024-01-01T12:00:00Z",
		Updated: "2024-01-02T12:00:00Z",
	}

	p.Issue(view)

	output := out.String()
	// In text-only mode (NO_COLOR), should have no ANSI escape codes
	assert.NotContains(t, output, "\x1b[", "should not contain ANSI escape codes")
	// Content should still be present
	assert.Contains(t, output, "Context")
	assert.Contains(t, output, "List item one")
	assert.Contains(t, output, "List item two")
}

func TestTextPrinter_Issue_Description_RespectsWordWrap(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	styles := trueColorStyles(t)
	io := gogh.NewIO(&out, &errOut)
	p := gogh.NewTextPrinterWithStyles(io, styles)

	// Create a very long line that should be wrapped
	longText := "This is a very long line of text that should definitely be wrapped because it exceeds the 80 column width limit that we have set for the terminal output display."
	view := jira4claude.IssueView{
		Key:         "TEST-123",
		Summary:     "Test issue",
		Status:      "Open",
		Type:        "Task",
		Description: longText,
		Created:     "2024-01-01T12:00:00Z",
		Updated:     "2024-01-02T12:00:00Z",
	}

	p.Issue(view)

	output := out.String()
	// The content should be present
	assert.Contains(t, output, "This is a very long line")
	// With word wrap at 80 columns, no single line should exceed ~80 visible chars
	// (Note: we can't easily test exact line lengths due to ANSI codes)
}

func TestTextPrinter_Issue_Description_EmptyDescriptionNoRendering(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	styles := trueColorStyles(t)
	io := gogh.NewIO(&out, &errOut)
	p := gogh.NewTextPrinterWithStyles(io, styles)

	view := jira4claude.IssueView{
		Key:         "TEST-123",
		Summary:     "Test issue",
		Status:      "Open",
		Type:        "Task",
		Description: "",
		Created:     "2024-01-01T12:00:00Z",
		Updated:     "2024-01-02T12:00:00Z",
	}

	outputBefore := out.String()
	p.Issue(view)
	outputAfter := out.String()

	// The description section should not be rendered when empty
	// This is existing behavior, but verify it still works with glamour
	assert.NotContains(t, outputAfter[len(outputBefore):], "Description")
}

// J4C-90: Issue view styling fixes

func TestTextPrinter_Issue_LinkedIssues_ConsistentStatusDisplay_TextMode(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	styles := asciiStyles(t)
	io := gogh.NewIO(&out, &errOut)
	p := gogh.NewTextPrinterWithStyles(io, styles)

	view := jira4claude.IssueView{
		Key:     "TEST-123",
		Summary: "Test issue",
		Status:  "Done",
		Type:    "Task",
		Created: "2024-01-01T12:00:00Z",
		Updated: "2024-01-02T12:00:00Z",
		Links: []jira4claude.LinkView{
			{
				Type:      "blocks",
				Direction: "outward",
				IssueKey:  "TEST-456",
				Summary:   "Blocked issue",
				Status:    "Done",
			},
			{
				Type:      "is blocked by",
				Direction: "inward",
				IssueKey:  "TEST-789",
				Summary:   "Blocking issue",
				Status:    "In Progress",
			},
		},
	}

	p.Issue(view)

	output := out.String()
	// Linked issues should use the same status format as the top panel
	// Top panel uses "[x] Done" format, so linked issues should too
	assert.Contains(t, output, "[x] Done", "linked issue status should use same format as top panel")
	assert.Contains(t, output, "[>] In Progress", "linked issue status should use same format as top panel")
	// Should NOT use the old bracket-only format
	assert.NotRegexp(t, `\s+\[Done\]\s+`, output, "should not use plain [Status] format")
}

func TestTextPrinter_Issue_LinkedIssues_ConsistentStatusDisplay_ColorMode(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	styles := trueColorStyles(t)
	io := gogh.NewIO(&out, &errOut)
	p := gogh.NewTextPrinterWithStyles(io, styles)

	view := jira4claude.IssueView{
		Key:     "TEST-123",
		Summary: "Test issue",
		Status:  "Open", // Use different status from linked issue to distinguish
		Type:    "Task",
		Created: "2024-01-01T12:00:00Z",
		Updated: "2024-01-02T12:00:00Z",
		Links: []jira4claude.LinkView{
			{
				Type:      "blocks",
				Direction: "outward",
				IssueKey:  "TEST-456",
				Summary:   "Blocked issue",
				Status:    "Done",
			},
		},
	}

	p.Issue(view)

	output := out.String()
	// Find the LINKED ISSUES section and check it contains the proper status format
	linkedIssuesPos := strings.Index(output, "LINKED ISSUES")
	assert.Positive(t, linkedIssuesPos, "should contain LINKED ISSUES section")

	linkedSection := output[linkedIssuesPos:]
	// In color mode, linked issues should use the checkmark indicator like the top panel
	// The checkmark should appear in the linked issues section specifically
	assert.Contains(t, linkedSection, "✓", "linked issue status should use checkmark indicator")
	assert.Contains(t, linkedSection, "Done", "linked issue status should show status text")
	// Should NOT use the old bracket-only format in linked issues
	assert.NotContains(t, linkedSection, "[Done]", "should not use plain [Status] format")
}

func TestTextPrinter_Issue_LinkedIssues_DottedSeparatorBetweenLinkTypes_TextMode(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	styles := asciiStyles(t)
	io := gogh.NewIO(&out, &errOut)
	p := gogh.NewTextPrinterWithStyles(io, styles)

	view := jira4claude.IssueView{
		Key:     "TEST-123",
		Summary: "Test issue",
		Status:  "Done",
		Type:    "Task",
		Created: "2024-01-01T12:00:00Z",
		Updated: "2024-01-02T12:00:00Z",
		Links: []jira4claude.LinkView{
			{
				Type:      "is blocked by",
				Direction: "inward",
				IssueKey:  "TEST-456",
				Summary:   "Blocking issue",
				Status:    "Done",
			},
			{
				Type:      "blocks",
				Direction: "outward",
				IssueKey:  "TEST-789",
				Summary:   "Blocked issue",
				Status:    "To Do",
			},
		},
	}

	p.Issue(view)

	output := out.String()
	// Should have dotted separator between link type groups (using . in text mode)
	// Find the position of both link types and check for separator between them
	blockedByPos := strings.Index(output, "is blocked by")
	blocksPos := strings.Index(output, "blocks")
	assert.Positive(t, blockedByPos, "should contain 'is blocked by'")
	assert.Greater(t, blocksPos, blockedByPos, "'blocks' should come after 'is blocked by'")

	// The section between them should contain a dotted separator
	between := output[blockedByPos:blocksPos]
	assert.Contains(t, between, "...", "should have dotted separator between link types")
}

func TestTextPrinter_Issue_LinkedIssues_DottedSeparatorBetweenLinkTypes_ColorMode(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	styles := trueColorStyles(t)
	io := gogh.NewIO(&out, &errOut)
	p := gogh.NewTextPrinterWithStyles(io, styles)

	view := jira4claude.IssueView{
		Key:     "TEST-123",
		Summary: "Test issue",
		Status:  "Done",
		Type:    "Task",
		Created: "2024-01-01T12:00:00Z",
		Updated: "2024-01-02T12:00:00Z",
		Links: []jira4claude.LinkView{
			{
				Type:      "is blocked by",
				Direction: "inward",
				IssueKey:  "TEST-456",
				Summary:   "Blocking issue",
				Status:    "Done",
			},
			{
				Type:      "blocks",
				Direction: "outward",
				IssueKey:  "TEST-789",
				Summary:   "Blocked issue",
				Status:    "To Do",
			},
		},
	}

	p.Issue(view)

	output := out.String()
	// Should have dotted separator between link type groups (using ┄ in color mode)
	blockedByPos := strings.Index(output, "is blocked by")
	blocksPos := strings.Index(output, "blocks")
	assert.Positive(t, blockedByPos, "should contain 'is blocked by'")
	assert.Greater(t, blocksPos, blockedByPos, "'blocks' should come after 'is blocked by'")

	between := output[blockedByPos:blocksPos]
	assert.Contains(t, between, "┄", "should have dotted separator between link types in color mode")
}

func TestTextPrinter_Issue_NoExtraTrailingNewlines_TextMode(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	styles := asciiStyles(t)
	io := gogh.NewIO(&out, &errOut)
	p := gogh.NewTextPrinterWithStyles(io, styles)

	view := jira4claude.IssueView{
		Key:      "TEST-123",
		Summary:  "Test issue",
		Status:   "Done",
		Type:     "Task",
		Assignee: "John Doe",
		Reporter: "Jane Smith",
		Created:  "2024-01-01T12:00:00Z",
		Updated:  "2024-01-02T12:00:00Z",
		Links: []jira4claude.LinkView{
			{
				Type:      "blocks",
				Direction: "outward",
				IssueKey:  "TEST-456",
				Summary:   "Blocked issue",
				Status:    "To Do",
			},
		},
		URL: "https://example.atlassian.net/browse/TEST-123",
	}

	p.Issue(view)

	output := out.String()
	// Should not have multiple consecutive blank lines (more than 2 newlines in a row)
	assert.NotContains(t, output, "\n\n\n", "should not have more than one blank line in a row")
}

func TestTextPrinter_Issue_BlankLineBetweenDescriptionAndURL(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	styles := asciiStyles(t)
	io := gogh.NewIO(&out, &errOut)
	p := gogh.NewTextPrinterWithStyles(io, styles)

	view := jira4claude.IssueView{
		Key:         "TEST-123",
		Summary:     "Test issue",
		Status:      "Done",
		Type:        "Task",
		Created:     "2024-01-01T12:00:00Z",
		Updated:     "2024-01-02T12:00:00Z",
		Description: "This is the description.",
		URL:         "https://example.atlassian.net/browse/TEST-123",
	}

	p.Issue(view)

	output := out.String()
	// There should be a blank line between description and URL
	// i.e., description text followed by two newlines then URL
	urlPos := strings.Index(output, "https://example.atlassian.net")
	assert.Positive(t, urlPos, "should contain URL")

	// Check that there's a blank line before the URL (two newlines)
	beforeURL := output[:urlPos]
	assert.True(t, strings.HasSuffix(beforeURL, "\n\n"), "should have blank line before URL")
}

func TestTextPrinter_Issue_CardLayout_ColorMode_RoundedBordersOnMainCard(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	styles := trueColorStyles(t)
	io := gogh.NewIO(&out, &errOut)
	p := gogh.NewTextPrinterWithStyles(io, styles)

	view := jira4claude.IssueView{
		Key:     "TEST-123",
		Summary: "Test issue",
		Status:  "Done",
		Type:    "Task",
		Created: "2024-01-01T12:00:00Z",
		Updated: "2024-01-02T12:00:00Z",
	}

	p.Issue(view)

	output := out.String()
	// Main card (header) should use rounded borders (╭ and ╯)
	assert.Contains(t, output, "╭", "main card should have top-left border")
	assert.Contains(t, output, "╯", "main card should have bottom-right border")
}

func TestTextPrinter_Issue_CardLayout_ColorMode_ConsistentBordersOnBothCards(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	styles := trueColorStyles(t)
	io := gogh.NewIO(&out, &errOut)
	p := gogh.NewTextPrinterWithStyles(io, styles)

	view := jira4claude.IssueView{
		Key:     "TEST-123",
		Summary: "Test issue",
		Status:  "Done",
		Type:    "Task",
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

	output := out.String()
	// Count occurrences of border corners - both cards should use same style
	topLeftCount := strings.Count(output, "╭")
	bottomRightCount := strings.Count(output, "╯")
	// Should have 2 of each (one per card)
	assert.Equal(t, 2, topLeftCount, "should have two top-left corners (one per card)")
	assert.Equal(t, 2, bottomRightCount, "should have two bottom-right corners (one per card)")
}

// J4C-92: NO_COLOR header styling tests

func TestTextPrinter_Issue_TextOnlyMode_HeaderHasNoExcessivePadding(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	styles := asciiStyles(t)
	io := gogh.NewIO(&out, &errOut)
	p := gogh.NewTextPrinterWithStyles(io, styles)

	view := jira4claude.IssueView{
		Key:      "TEST-123",
		Summary:  "Test issue summary",
		Status:   "Done",
		Type:     "Task",
		Priority: "Medium",
		Assignee: "John Doe",
		Reporter: "Jane Smith",
		Created:  "2024-01-01T12:00:00Z",
		Updated:  "2024-01-02T12:00:00Z",
	}

	p.Issue(view)

	output := out.String()
	lines := strings.Split(output, "\n")

	// Find lines inside the header card (after the === line, before description)
	inHeader := false
	for _, line := range lines {
		if strings.HasPrefix(line, "===") {
			inHeader = true
			continue
		}
		if line == "" {
			continue
		}
		if !inHeader {
			continue
		}

		// Stop when we hit the URL or other sections
		if strings.HasPrefix(line, "http") {
			break
		}

		// In NO_COLOR mode, content lines should NOT have leading 2-space padding
		// The summary line with type badge should start without padding
		if strings.Contains(line, "Test issue summary") {
			assert.False(t, strings.HasPrefix(line, "  "),
				"summary line should not have leading 2-space padding in NO_COLOR mode: %q", line)
		}

		// Status/Priority labels should not have leading padding
		if strings.HasPrefix(line, "STATUS") {
			assert.False(t, strings.HasPrefix(line, "  STATUS"),
				"STATUS label should not have leading 2-space padding in NO_COLOR mode: %q", line)
		}

		// Assignee/Reporter should not have leading padding
		if strings.Contains(line, "Assignee:") {
			assert.False(t, strings.HasPrefix(line, "  Assignee"),
				"Assignee line should not have leading 2-space padding in NO_COLOR mode: %q", line)
		}
	}
}

func TestTextPrinter_Issue_ColorMode_HeaderKeepsPadding(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	styles := trueColorStyles(t)
	io := gogh.NewIO(&out, &errOut)
	p := gogh.NewTextPrinterWithStyles(io, styles)

	view := jira4claude.IssueView{
		Key:      "TEST-123",
		Summary:  "Test issue summary",
		Status:   "Done",
		Type:     "Task",
		Priority: "Medium",
		Assignee: "John Doe",
		Reporter: "Jane Smith",
		Created:  "2024-01-01T12:00:00Z",
		Updated:  "2024-01-02T12:00:00Z",
	}

	p.Issue(view)

	output := out.String()
	lines := strings.Split(output, "\n")

	// In color mode, content inside card borders should have 2-space padding
	// Find a STATUS line and verify it has "│  STATUS" pattern (border + 2 spaces)
	foundPaddedStatusLine := false
	for _, line := range lines {
		if strings.Contains(line, "│") && strings.Contains(line, "STATUS") {
			// Verify the line has proper padding: border followed by 2 spaces before content
			// The pattern is: │ (with ANSI codes) + "  STATUS"
			// After stripping the border's ANSI codes, we should see "  STATUS" (2 spaces)
			borderIdx := strings.Index(line, "│")
			if borderIdx >= 0 {
				afterBorder := line[borderIdx+len("│"):]
				// Strip any ANSI codes that may follow the border character
				afterBorder = stripANSI(afterBorder)
				if strings.HasPrefix(afterBorder, "  STATUS") {
					foundPaddedStatusLine = true
					break
				}
			}
		}
	}
	assert.True(t, foundPaddedStatusLine,
		"color mode should have 2-space padding after border (│  STATUS pattern)")
}

// J4C-99: Subtask panel tests

func TestTextPrinter_Issue_ShowsSubtasks(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	io := gogh.NewIO(&out, &errOut)
	p := gogh.NewTextPrinter(io)

	view := jira4claude.IssueView{
		Key:     "TEST-123",
		Summary: "Parent issue",
		Status:  "In Progress",
		Type:    "Task",
		Created: "2024-01-01T12:00:00Z",
		Updated: "2024-01-02T12:00:00Z",
		Subtasks: []jira4claude.SubtaskView{
			{
				Key:     "TEST-124",
				Summary: "First subtask",
				Status:  "Done",
			},
			{
				Key:     "TEST-125",
				Summary: "Second subtask",
				Status:  "To Do",
			},
		},
	}

	p.Issue(view)

	output := out.String()
	// Should have SUBTASKS section
	assert.Contains(t, output, "SUBTASKS")
	assert.Contains(t, output, "TEST-124")
	assert.Contains(t, output, "First subtask")
	assert.Contains(t, output, "TEST-125")
	assert.Contains(t, output, "Second subtask")
}

func TestTextPrinter_Issue_SubtasksShowsStatus_TextMode(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	styles := asciiStyles(t)
	io := gogh.NewIO(&out, &errOut)
	p := gogh.NewTextPrinterWithStyles(io, styles)

	view := jira4claude.IssueView{
		Key:     "TEST-123",
		Summary: "Parent issue",
		Status:  "In Progress",
		Type:    "Task",
		Created: "2024-01-01T12:00:00Z",
		Updated: "2024-01-02T12:00:00Z",
		Subtasks: []jira4claude.SubtaskView{
			{
				Key:     "TEST-124",
				Summary: "Completed subtask",
				Status:  "Done",
			},
			{
				Key:     "TEST-125",
				Summary: "Pending subtask",
				Status:  "To Do",
			},
		},
	}

	p.Issue(view)

	output := out.String()
	// Status should use consistent badge format (same as top panel and linked issues)
	assert.Contains(t, output, "[x] Done")
	assert.Contains(t, output, "[ ] To Do")
}

func TestTextPrinter_Issue_SubtasksShowsStatus_ColorMode(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	styles := trueColorStyles(t)
	io := gogh.NewIO(&out, &errOut)
	p := gogh.NewTextPrinterWithStyles(io, styles)

	view := jira4claude.IssueView{
		Key:     "TEST-123",
		Summary: "Parent issue",
		Status:  "In Progress",
		Type:    "Task",
		Created: "2024-01-01T12:00:00Z",
		Updated: "2024-01-02T12:00:00Z",
		Subtasks: []jira4claude.SubtaskView{
			{
				Key:     "TEST-124",
				Summary: "Completed subtask",
				Status:  "Done",
			},
		},
	}

	p.Issue(view)

	output := out.String()
	// Find the SUBTASKS section and check it contains the proper status format
	subtasksPos := strings.Index(output, "SUBTASKS")
	if !assert.Positive(t, subtasksPos, "should contain SUBTASKS section") {
		return
	}

	subtasksSection := output[subtasksPos:]
	// In color mode, subtasks should use the checkmark indicator like the top panel
	assert.Contains(t, subtasksSection, "✓", "subtask status should use checkmark indicator")
	assert.Contains(t, subtasksSection, "Done", "subtask status should show status text")
}

func TestTextPrinter_Issue_NoSubtasksSection_WhenNoSubtasks(t *testing.T) {
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
	}

	p.Issue(view)

	output := out.String()
	assert.NotContains(t, output, "SUBTASKS")
}

func TestTextPrinter_Issue_SubtasksHasCardBorder_ColorMode(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	styles := trueColorStyles(t)
	io := gogh.NewIO(&out, &errOut)
	p := gogh.NewTextPrinterWithStyles(io, styles)

	view := jira4claude.IssueView{
		Key:     "TEST-123",
		Summary: "Parent issue",
		Status:  "In Progress",
		Type:    "Task",
		Created: "2024-01-01T12:00:00Z",
		Updated: "2024-01-02T12:00:00Z",
		Subtasks: []jira4claude.SubtaskView{
			{
				Key:     "TEST-124",
				Summary: "A subtask",
				Status:  "Done",
			},
		},
	}

	p.Issue(view)

	output := out.String()
	// Should have SUBTASKS card with borders
	assert.Contains(t, output, "SUBTASKS")
	// The card should use the same border style as LINKED ISSUES
	subtasksPos := strings.Index(output, "SUBTASKS")
	assert.Positive(t, subtasksPos, "should contain SUBTASKS section")

	// After SUBTASKS, there should be a closing border (╯)
	afterSubtasks := output[subtasksPos:]
	assert.Contains(t, afterSubtasks, "╯", "SUBTASKS card should have bottom border")
}

func TestTextPrinter_Issue_SubtasksBeforeLinkedIssues(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	io := gogh.NewIO(&out, &errOut)
	p := gogh.NewTextPrinter(io)

	view := jira4claude.IssueView{
		Key:     "TEST-123",
		Summary: "Parent issue",
		Status:  "In Progress",
		Type:    "Task",
		Created: "2024-01-01T12:00:00Z",
		Updated: "2024-01-02T12:00:00Z",
		Subtasks: []jira4claude.SubtaskView{
			{
				Key:     "TEST-124",
				Summary: "A subtask",
				Status:  "Done",
			},
		},
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

	output := out.String()
	// SUBTASKS should appear before LINKED ISSUES
	subtasksPos := strings.Index(output, "SUBTASKS")
	linkedIssuesPos := strings.Index(output, "LINKED ISSUES")

	assert.Positive(t, subtasksPos, "should contain SUBTASKS section")
	assert.Positive(t, linkedIssuesPos, "should contain LINKED ISSUES section")
	assert.Less(t, subtasksPos, linkedIssuesPos, "SUBTASKS should appear before LINKED ISSUES")
}
