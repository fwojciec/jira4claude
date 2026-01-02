package gogh

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
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
		styles: DefaultStyles(),
	}
}

// Issue prints a single issue in detail format.
func (p *TextPrinter) Issue(view jira4claude.IssueView) {
	fmt.Fprintf(p.io.Out, "%s  %s\n", p.styles.Key(view.Key), view.Summary)
	fmt.Fprintf(p.io.Out, "Status: %s  Type: %s", p.styles.Status(view.Status), view.Type)
	if view.Priority != "" {
		fmt.Fprintf(p.io.Out, "  Priority: %s", view.Priority)
	}
	fmt.Fprintln(p.io.Out)

	if view.Assignee != "" {
		fmt.Fprintf(p.io.Out, "Assignee: %s\n", view.Assignee)
	}
	if view.Reporter != "" {
		fmt.Fprintf(p.io.Out, "Reporter: %s\n", view.Reporter)
	}
	if view.Parent != "" {
		fmt.Fprintf(p.io.Out, "Parent: %s\n", p.styles.Key(view.Parent))
	}
	if len(view.Labels) > 0 {
		fmt.Fprintf(p.io.Out, "Labels: %s\n", p.styles.Label(strings.Join(view.Labels, ", ")))
	}
	if len(view.Links) > 0 {
		fmt.Fprintln(p.io.Out, "Links:")
		for _, link := range view.Links {
			fmt.Fprintf(p.io.Out, "  %s %s [%s] (%s)\n",
				link.Type,
				p.styles.Key(link.IssueKey),
				p.styles.Status(link.Status),
				link.Summary)
		}
	}

	if view.Description != "" {
		fmt.Fprintf(p.io.Out, "\n%s\n", view.Description)
	}

	if len(view.Comments) > 0 {
		fmt.Fprintln(p.io.Out, "\n## Comments")
		for _, comment := range view.Comments {
			author := comment.Author
			if author == "" {
				author = "Unknown"
			}
			// Parse the RFC3339 timestamp for display
			created := comment.Created
			if len(created) >= 16 {
				created = created[:10] + " " + created[11:16]
			}
			fmt.Fprintf(p.io.Out, "\n**%s** (%s):\n%s\n", author, created, comment.Body)
		}
	}

	if view.URL != "" {
		fmt.Fprintf(p.io.Out, "\n%s\n", view.URL)
	}
}

// Issues prints multiple issues in table format.
func (p *TextPrinter) Issues(views []jira4claude.IssueView) {
	if len(views) == 0 {
		fmt.Fprintln(p.io.Out, "No issues found")
		return
	}

	rows := make([][]string, 0, len(views))
	for _, view := range views {
		assignee := "-"
		if view.Assignee != "" {
			assignee = view.Assignee
		}
		summary := truncateString(view.Summary, 50)
		rows = append(rows, []string{view.Key, view.Status, assignee, summary})
	}

	t := table.New().
		Border(lipgloss.Border{}).
		BorderHeader(false).
		StyleFunc(func(row, col int) lipgloss.Style {
			style := lipgloss.NewStyle().PaddingRight(2)
			if p.styles.NoColor {
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
func (p *TextPrinter) Comment(view jira4claude.CommentView) {
	author := view.Author
	if author == "" {
		author = "Unknown"
	}
	// Parse the RFC3339 timestamp for display
	created := view.Created
	if len(created) >= 16 {
		created = created[:10] + " " + created[11:16]
	}
	fmt.Fprintf(p.io.Out, "**%s** (%s):\n%s\n", author, created, view.Body)
}

// Links prints issue links.
func (p *TextPrinter) Links(key string, links []jira4claude.LinkView) {
	if len(links) == 0 {
		fmt.Fprintf(p.io.Out, "No issue links found for %s\n", p.styles.Key(key))
		return
	}

	fmt.Fprintf(p.io.Out, "Links for %s:\n", p.styles.Key(key))
	for _, link := range links {
		fmt.Fprintf(p.io.Out, "  %s %s [%s] (%s)\n",
			link.Type,
			p.styles.Key(link.IssueKey),
			p.styles.Status(link.Status),
			link.Summary)
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

// Warning prints a warning message to stderr.
func (p *TextPrinter) Warning(msg string) {
	fmt.Fprintln(p.io.Err, p.styles.Warning(msg))
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
