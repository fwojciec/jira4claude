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

// NewTextPrinterWithStyles creates a text printer with custom styles.
// This is primarily used for testing with controlled color profiles.
func NewTextPrinterWithStyles(io *IO, styles *Styles) *TextPrinter {
	return &TextPrinter{
		io:     io,
		styles: styles,
	}
}

// Issue prints a single issue in detail format with card layout.
func (p *TextPrinter) Issue(view jira4claude.IssueView) {
	// Build header card content
	headerContent := p.renderIssueHeader(view)
	header := RenderCard(p.styles, "", headerContent)
	fmt.Fprintln(p.io.Out, header)

	// Linked issues card
	if len(view.Links) > 0 {
		fmt.Fprintln(p.io.Out)
		linksContent := p.renderLinksContent(view.Links)
		links := RenderCard(p.styles, "LINKED ISSUES", linksContent)
		fmt.Fprintln(p.io.Out, links)
	}

	// Description (no card, just content)
	if view.Description != "" {
		fmt.Fprintf(p.io.Out, "\n%s\n", view.Description)
	}

	// Comments section
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

	// URL
	if view.URL != "" {
		fmt.Fprintf(p.io.Out, "\n%s\n", view.URL)
	}
}

// renderIssueHeader builds the content for the issue header card.
func (p *TextPrinter) renderIssueHeader(view jira4claude.IssueView) string {
	var b strings.Builder
	width := p.styles.Width

	// Line 1: Key (left) and Type badge (right)
	typeBadge := strings.ToUpper(view.Type)
	keyWidth := len(view.Key)
	typeWidth := len(typeBadge)
	padding := width - keyWidth - typeWidth - 4 // account for margins
	if padding < 1 {
		padding = 1
	}
	fmt.Fprintf(&b, "  %s%s%s  \n", p.styles.Key(view.Key), strings.Repeat(" ", padding), typeBadge)

	// Line 2: Summary
	fmt.Fprintf(&b, "  %s\n", view.Summary)

	// Separator
	separatorWidth := width - 4 // account for margins
	fmt.Fprintf(&b, "  %s\n", RenderSeparator(p.styles, "dotted", separatorWidth))

	// Status and Priority section
	statusBadge := RenderStatusBadge(p.styles, view.Status)
	priorityBadge := ""
	if view.Priority != "" {
		priorityBadge = RenderPriorityBadge(p.styles, view.Priority)
	}

	// Format as columns: STATUS (left), PRIORITY (right)
	fmt.Fprintf(&b, "  STATUS              PRIORITY\n")
	if priorityBadge != "" {
		// Use lipgloss.Width to calculate visible width (excludes ANSI codes)
		statusWidth := lipgloss.Width(statusBadge)
		padding := 20 - statusWidth
		if padding < 1 {
			padding = 1
		}
		fmt.Fprintf(&b, "  %s%s%s\n", statusBadge, strings.Repeat(" ", padding), priorityBadge)
	} else {
		fmt.Fprintf(&b, "  %s\n", statusBadge)
	}

	// Separator before assignee/reporter
	fmt.Fprintf(&b, "  %s\n", RenderSeparator(p.styles, "dotted", separatorWidth))

	// Assignee and Reporter (right-aligned values)
	if view.Assignee != "" {
		padding := width - len("Assignee:") - len(view.Assignee) - 4
		if padding < 1 {
			padding = 1
		}
		fmt.Fprintf(&b, "  Assignee:%s%s  \n", strings.Repeat(" ", padding), view.Assignee)
	}
	if view.Reporter != "" {
		padding := width - len("Reporter:") - len(view.Reporter) - 4
		if padding < 1 {
			padding = 1
		}
		fmt.Fprintf(&b, "  Reporter:%s%s  \n", strings.Repeat(" ", padding), view.Reporter)
	}

	// Parent and Labels (if present)
	if view.Parent != "" {
		fmt.Fprintf(&b, "\n  Parent: %s", p.styles.Key(view.Parent))
	}
	if len(view.Labels) > 0 {
		fmt.Fprintf(&b, "\n  Labels: %s", p.styles.Label(strings.Join(view.Labels, ", ")))
	}

	return b.String()
}

// renderLinksContent builds the content for the linked issues card.
func (p *TextPrinter) renderLinksContent(links []jira4claude.LinkView) string {
	// Group links by type
	grouped := make(map[string][]jira4claude.LinkView)
	var order []string
	for _, link := range links {
		if _, exists := grouped[link.Type]; !exists {
			order = append(order, link.Type)
		}
		grouped[link.Type] = append(grouped[link.Type], link)
	}

	var b strings.Builder
	for i, linkType := range order {
		if i > 0 {
			b.WriteString("\n")
		}
		fmt.Fprintf(&b, "  %s\n", linkType)
		for _, link := range grouped[linkType] {
			statusIndicator := formatStatusIndicator(link.Status)
			fmt.Fprintf(&b, "  %s  %s   %s\n",
				p.styles.Key(link.IssueKey),
				statusIndicator,
				link.Summary)
		}
	}

	return b.String()
}

// formatStatusIndicator returns a bracketed status indicator.
func formatStatusIndicator(status string) string {
	return fmt.Sprintf("[%s]", status)
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
		Border(p.issueTableBorder()).
		BorderHeader(true).
		BorderStyle(p.styles.Renderer.NewStyle().Foreground(p.styles.Theme.Border)).
		StyleFunc(func(row, col int) lipgloss.Style {
			style := p.styles.Renderer.NewStyle().PaddingRight(2)
			if p.styles.NoColor {
				return style
			}
			if row == table.HeaderRow {
				return style.Bold(true)
			}
			switch col {
			case 0: // KEY
				return style.Bold(true).Foreground(p.styles.Theme.Primary)
			case 1: // STATUS
				return style.Foreground(p.styles.Theme.Success)
			default:
				return style
			}
		}).
		Headers("KEY", "STATUS", "ASSIGNEE", "SUMMARY").
		Rows(rows...)

	fmt.Fprintln(p.io.Out, t)
}

// issueTableBorder returns the appropriate border style for the issue table.
func (p *TextPrinter) issueTableBorder() lipgloss.Border {
	if p.styles.NoColor {
		// Text-only mode: no outer border, just dashed header separator
		return lipgloss.Border{
			Middle: p.styles.Separators.Solid,
		}
	}
	// Color mode: rounded border with dotted header separator
	b := lipgloss.RoundedBorder()
	b.Middle = p.styles.Separators.Dotted
	b.MiddleLeft = "│"
	b.MiddleRight = "│"
	return b
}

// Transitions prints available transitions for an issue.
func (p *TextPrinter) Transitions(key string, ts []*jira4claude.Transition) {
	if len(ts) == 0 {
		fmt.Fprintf(p.io.Out, "No transitions for %s (issue may be in terminal state)\n", p.styles.Key(key))
		return
	}

	arrow := p.styles.Indicators.Arrow
	fmt.Fprintf(p.io.Out, "Available transitions for %s:\n", p.styles.Key(key))
	for _, t := range ts {
		fmt.Fprintf(p.io.Out, "  %s %s\n", arrow, p.styles.Status(t.Name))
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
	indicator := p.styles.Indicators.Success
	if len(keys) > 0 {
		styledKeys := make([]string, len(keys))
		for i, k := range keys {
			styledKeys[i] = p.styles.Key(k)
		}
		fmt.Fprintf(p.io.Out, "%s %s %s\n", indicator, msg, strings.Join(styledKeys, ", "))
		if p.io.ServerURL != "" {
			for _, k := range keys {
				fmt.Fprintf(p.io.Out, "%s/browse/%s\n", p.io.ServerURL, k)
			}
		}
	} else {
		fmt.Fprintf(p.io.Out, "%s %s\n", indicator, msg)
	}
}

// Warning prints a warning message to stderr.
func (p *TextPrinter) Warning(msg string) {
	indicator := p.styles.Indicators.Warning
	if p.styles.NoColor {
		fmt.Fprintf(p.io.Err, "%s %s\n", indicator, msg)
	} else {
		fmt.Fprintf(p.io.Err, "%s Warning: %s\n", indicator, msg)
	}
}

// Error prints an error message to stderr.
func (p *TextPrinter) Error(err error) {
	indicator := p.styles.Indicators.Error
	if p.styles.NoColor {
		fmt.Fprintf(p.io.Err, "%s %s\n", indicator, jira4claude.ErrorMessage(err))
	} else {
		fmt.Fprintf(p.io.Err, "%s Error: %s\n", indicator, jira4claude.ErrorMessage(err))
	}
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
