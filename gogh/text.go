package gogh

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/fwojciec/jira4claude"
)

// adfToString converts an ADF document to a string for display.
// TODO(J4C-80): Replace with proper ADF-to-markdown conversion at CLI boundary.
func adfToString(adf jira4claude.ADF) string {
	if adf == nil {
		return ""
	}
	b, err := json.Marshal(adf)
	if err != nil {
		return "[ADF conversion error]"
	}
	return string(b)
}

// TextPrinter outputs human-readable text format with styled terminal output.
type TextPrinter struct {
	io     *IO
	styles *Styles
}

// NewTextPrinter creates a text printer using the provided IO.
func NewTextPrinter(io *IO) *TextPrinter {
	return &TextPrinter{
		io:     io,
		styles: NewStyles(),
	}
}

// Issue prints a single issue in detail format.
func (p *TextPrinter) Issue(issue *jira4claude.Issue) {
	fmt.Fprintf(p.io.Out, "%s  %s\n", p.styles.Key(issue.Key), issue.Summary)
	fmt.Fprintf(p.io.Out, "Status: %s  Type: %s", p.styles.Status(issue.Status), issue.Type)
	if issue.Priority != "" {
		fmt.Fprintf(p.io.Out, "  Priority: %s", issue.Priority)
	}
	fmt.Fprintln(p.io.Out)

	if issue.Assignee != nil {
		fmt.Fprintf(p.io.Out, "Assignee: %s\n", issue.Assignee.DisplayName)
	}
	if issue.Reporter != nil {
		fmt.Fprintf(p.io.Out, "Reporter: %s\n", issue.Reporter.DisplayName)
	}
	if issue.Parent != "" {
		fmt.Fprintf(p.io.Out, "Parent: %s\n", p.styles.Key(issue.Parent))
	}
	if len(issue.Labels) > 0 {
		fmt.Fprintf(p.io.Out, "Labels: %s\n", p.styles.Label(strings.Join(issue.Labels, ", ")))
	}
	if len(issue.Links) > 0 {
		fmt.Fprintln(p.io.Out, "Links:")
		for _, link := range issue.Links {
			if link.OutwardIssue != nil {
				fmt.Fprintf(p.io.Out, "  %s %s (%s)\n",
					link.Type.Outward,
					p.styles.Key(link.OutwardIssue.Key),
					link.OutwardIssue.Summary)
			}
			if link.InwardIssue != nil {
				fmt.Fprintf(p.io.Out, "  %s %s (%s)\n",
					link.Type.Inward,
					p.styles.Key(link.InwardIssue.Key),
					link.InwardIssue.Summary)
			}
		}
	}

	if desc := adfToString(issue.Description); desc != "" {
		fmt.Fprintf(p.io.Out, "\n%s\n", desc)
	}

	if len(issue.Comments) > 0 {
		fmt.Fprintln(p.io.Out, "\n## Comments")
		for _, comment := range issue.Comments {
			author := "Unknown"
			if comment.Author != nil {
				author = comment.Author.DisplayName
			}
			fmt.Fprintf(p.io.Out, "\n**%s** (%s):\n%s\n",
				author,
				comment.Created.Format("2006-01-02 15:04"),
				adfToString(comment.Body))
		}
	}

	if p.io.ServerURL != "" {
		fmt.Fprintf(p.io.Out, "\n%s/browse/%s\n", p.io.ServerURL, issue.Key)
	}
}

// Issues prints multiple issues in table format.
func (p *TextPrinter) Issues(issues []*jira4claude.Issue) {
	if len(issues) == 0 {
		fmt.Fprintln(p.io.Out, "No issues found")
		return
	}

	rows := make([][]string, 0, len(issues))
	for _, issue := range issues {
		assignee := "-"
		if issue.Assignee != nil {
			assignee = issue.Assignee.DisplayName
		}
		summary := truncateString(issue.Summary, 50)
		rows = append(rows, []string{issue.Key, issue.Status, assignee, summary})
	}

	t := table.New().
		Border(lipgloss.Border{}).
		BorderHeader(false).
		StyleFunc(func(row, col int) lipgloss.Style {
			style := lipgloss.NewStyle().PaddingRight(2)
			if p.styles.NoColor() {
				return style
			}
			if row == table.HeaderRow {
				return style.Bold(true).Underline(true)
			}
			switch col {
			case 0: // KEY
				return style.Bold(true).Foreground(lipgloss.Color("12"))
			case 1: // STATUS
				return style.Foreground(lipgloss.Color("10"))
			default:
				return style
			}
		}).
		Headers("KEY", "STATUS", "ASSIGNEE", "SUMMARY").
		Rows(rows...)

	fmt.Fprintln(p.io.Out, t)
}

// Transitions prints available transitions for an issue.
func (p *TextPrinter) Transitions(key string, ts []*jira4claude.Transition) {
	if len(ts) == 0 {
		fmt.Fprintf(p.io.Out, "No transitions for %s (issue may be in terminal state)\n", p.styles.Key(key))
		return
	}

	fmt.Fprintf(p.io.Out, "Available transitions for %s:\n", p.styles.Key(key))
	for _, t := range ts {
		fmt.Fprintf(p.io.Out, "  %s: %s\n", t.ID, p.styles.Status(t.Name))
	}
}

// Comment prints a single comment.
func (p *TextPrinter) Comment(comment *jira4claude.Comment) {
	author := "Unknown"
	if comment.Author != nil {
		author = comment.Author.DisplayName
	}
	fmt.Fprintf(p.io.Out, "**%s** (%s):\n%s\n",
		author,
		comment.Created.Format("2006-01-02 15:04"),
		adfToString(comment.Body))
}

// Links prints issue links.
func (p *TextPrinter) Links(key string, links []*jira4claude.IssueLink) {
	if len(links) == 0 {
		fmt.Fprintf(p.io.Out, "No issue links found for %s\n", p.styles.Key(key))
		return
	}

	fmt.Fprintf(p.io.Out, "Links for %s:\n", p.styles.Key(key))
	for _, link := range links {
		if link.OutwardIssue != nil {
			fmt.Fprintf(p.io.Out, "  %s %s (%s)\n",
				link.Type.Outward,
				p.styles.Key(link.OutwardIssue.Key),
				link.OutwardIssue.Summary)
		}
		if link.InwardIssue != nil {
			fmt.Fprintf(p.io.Out, "  %s %s (%s)\n",
				link.Type.Inward,
				p.styles.Key(link.InwardIssue.Key),
				link.InwardIssue.Summary)
		}
	}
}

// Success prints a success message to stdout.
func (p *TextPrinter) Success(msg string, keys ...string) {
	if len(keys) > 0 {
		styledKeys := make([]string, len(keys))
		for i, k := range keys {
			styledKeys[i] = p.styles.Key(k)
		}
		fmt.Fprintf(p.io.Out, "%s %s\n", msg, strings.Join(styledKeys, ", "))
		if p.io.ServerURL != "" {
			for _, k := range keys {
				fmt.Fprintf(p.io.Out, "%s/browse/%s\n", p.io.ServerURL, k)
			}
		}
	} else {
		fmt.Fprintln(p.io.Out, msg)
	}
}

// Error prints an error message to stderr.
func (p *TextPrinter) Error(err error) {
	fmt.Fprintf(p.io.Err, "%s %s\n", p.styles.Error("Error:"), jira4claude.ErrorMessage(err))
}

func truncateString(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen-3]) + "..."
}

// Verify interface compliance at compile time.
var _ jira4claude.Printer = (*TextPrinter)(nil)
