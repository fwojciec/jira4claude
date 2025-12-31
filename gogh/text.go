package gogh

import (
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/fwojciec/jira4claude"
)

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

	if issue.Description != "" {
		fmt.Fprintf(p.io.Out, "\n%s\n", issue.Description)
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

	w := tabwriter.NewWriter(p.io.Out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, p.styles.Header("KEY")+"\t"+p.styles.Header("STATUS")+"\t"+p.styles.Header("ASSIGNEE")+"\t"+p.styles.Header("SUMMARY"))
	for _, issue := range issues {
		assignee := "-"
		if issue.Assignee != nil {
			assignee = issue.Assignee.DisplayName
		}
		summary := truncateString(issue.Summary, 50)
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
			p.styles.Key(issue.Key),
			p.styles.Status(issue.Status),
			assignee,
			summary,
		)
	}
	w.Flush()
}

// Transitions prints available transitions for an issue.
func (p *TextPrinter) Transitions(key string, ts []*jira4claude.Transition) {
	if len(ts) == 0 {
		fmt.Fprintf(p.io.Out, "No transitions for %s\n", p.styles.Key(key))
		return
	}

	fmt.Fprintf(p.io.Out, "Available transitions for %s:\n", p.styles.Key(key))
	for _, t := range ts {
		fmt.Fprintf(p.io.Out, "  %s: %s\n", t.ID, p.styles.Status(t.Name))
	}
}

// Links prints issue links.
func (p *TextPrinter) Links(key string, links []*jira4claude.IssueLink) {
	if len(links) == 0 {
		fmt.Fprintf(p.io.Out, "No links for %s\n", p.styles.Key(key))
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
