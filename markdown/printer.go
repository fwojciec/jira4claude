package markdown

import (
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/fwojciec/jira4claude"
)

// maxSummaryLength is the maximum length for issue summaries in list format.
const maxSummaryLength = 60

// Printer outputs GitHub-Flavored Markdown format.
type Printer struct {
	out       io.Writer
	err       io.Writer
	serverURL string
}

// NewPrinter creates a markdown printer that writes to out.
// Warnings are discarded since no stderr writer is provided.
func NewPrinter(out io.Writer) *Printer {
	return &Printer{out: out, err: io.Discard}
}

// NewPrinterWithIO creates a markdown printer with explicit stdout and stderr writers.
func NewPrinterWithIO(out, err io.Writer) *Printer {
	return &Printer{out: out, err: err}
}

// SetServerURL sets the server URL for generating issue URLs.
func (p *Printer) SetServerURL(url string) {
	p.serverURL = url
}

// Issue prints a single issue in markdown format.
func (p *Printer) Issue(view jira4claude.IssueView) {
	// Header: # KEY: Summary
	fmt.Fprintf(p.out, "# %s: %s\n\n", view.Key, view.Summary)

	// Metadata fields - one per line, omit if empty
	fmt.Fprintf(p.out, "**Type:** %s\n", view.Type)
	fmt.Fprintf(p.out, "**Status:** %s\n", view.Status)
	if view.Priority != "" {
		fmt.Fprintf(p.out, "**Priority:** %s\n", view.Priority)
	}
	if view.Assignee != "" {
		fmt.Fprintf(p.out, "**Assignee:** %s\n", view.Assignee)
	}
	if view.Reporter != "" {
		fmt.Fprintf(p.out, "**Reporter:** %s\n", view.Reporter)
	}
	// Parent from RelatedIssues
	for _, rel := range view.RelatedIssues {
		if rel.Relationship == "parent" {
			fmt.Fprintf(p.out, "**Parent:** %s\n", rel.Key)
			break
		}
	}
	if len(view.Labels) > 0 {
		fmt.Fprintf(p.out, "**Labels:** %s\n", strings.Join(view.Labels, ", "))
	}

	// Description - passes through as-is (already markdown)
	if view.Description != "" {
		fmt.Fprintf(p.out, "\n%s\n", view.Description)
	}

	// Related Issues section (unified), excluding parent (already shown in metadata)
	nonParentRelated := make([]jira4claude.RelatedIssueView, 0, len(view.RelatedIssues))
	for _, rel := range view.RelatedIssues {
		if rel.Relationship != "parent" {
			nonParentRelated = append(nonParentRelated, rel)
		}
	}
	if len(nonParentRelated) > 0 {
		fmt.Fprint(p.out, "\n## Related Issues\n\n")
		p.renderRelatedIssuesGrouped(nonParentRelated)
	}

	// Comments section
	if len(view.Comments) > 0 {
		fmt.Fprint(p.out, "\n## Comments\n\n")
		for _, comment := range view.Comments {
			p.renderComment(comment)
		}
	}

	// URL as markdown link at end
	if view.URL != "" {
		fmt.Fprintf(p.out, "\n[View in Jira](%s)\n", view.URL)
	}
}

// Issues prints multiple issues as a markdown list.
func (p *Printer) Issues(views []jira4claude.IssueView) {
	if len(views) == 0 {
		fmt.Fprintln(p.out, "[info] No issues found")
		return
	}

	for _, view := range views {
		summary := truncate(view.Summary, maxSummaryLength)
		fmt.Fprintln(p.out, formatIssueListItem(view.Key, view.Status, view.Priority, summary))
	}
}

// Comment prints a single comment.
func (p *Printer) Comment(view jira4claude.CommentView) {
	p.renderComment(view)
}

// Transitions prints available transitions for an issue.
func (p *Printer) Transitions(key string, ts []*jira4claude.Transition) {
	if len(ts) == 0 {
		fmt.Fprintf(p.out, "[info] No transitions available for %s\n", key)
		return
	}

	for _, t := range ts {
		fmt.Fprintf(p.out, "- %s\n", t.Name)
	}
}

// Links prints issue links using RelatedIssueView.
func (p *Printer) Links(key string, links []jira4claude.RelatedIssueView) {
	if len(links) == 0 {
		fmt.Fprintf(p.out, "[info] No links for %s\n", key)
		return
	}

	p.renderRelatedIssuesGrouped(links)
}

// Success prints a success message to stdout.
func (p *Printer) Success(msg string, keys ...string) {
	if len(keys) > 0 {
		fmt.Fprintf(p.out, "[ok] %s %s\n", msg, strings.Join(keys, ", "))
		if p.serverURL != "" {
			for _, k := range keys {
				fmt.Fprintf(p.out, "%s/browse/%s\n", p.serverURL, k)
			}
		}
	} else {
		fmt.Fprintf(p.out, "[ok] %s\n", msg)
	}
}

// Warning prints a warning message to stderr.
func (p *Printer) Warning(msg string) {
	fmt.Fprintf(p.err, "[warn] %s\n", msg)
}

// Error prints an error message to stderr.
func (p *Printer) Error(err error) {
	fmt.Fprintf(p.err, "[error] %s\n", jira4claude.ErrorMessage(err))
}

// renderComment formats a comment with author and timestamp.
func (p *Printer) renderComment(view jira4claude.CommentView) {
	author := view.Author
	if author == "" {
		author = "Unknown"
	}
	// Parse the RFC3339 timestamp for display (YYYY-MM-DD HH:MM)
	created := view.Created
	if len(created) >= 16 {
		created = created[:10] + " " + created[11:16]
	}
	fmt.Fprintf(p.out, "**%s** (%s):\n%s\n", author, created, view.Body)
}

// renderRelatedIssuesGrouped groups related issues by relationship type and renders them.
// Groups are displayed in a fixed order: subtask -> blocks -> is blocked by.
func (p *Printer) renderRelatedIssuesGrouped(related []jira4claude.RelatedIssueView) {
	// Fixed display order for relationship types (parent is excluded, shown in metadata)
	relationshipOrder := []string{"subtask", "blocks", "is blocked by"}

	// Group by relationship type
	grouped := make(map[string][]jira4claude.RelatedIssueView)
	for _, rel := range related {
		grouped[rel.Relationship] = append(grouped[rel.Relationship], rel)
	}

	// Render in fixed order, then any remaining types
	first := true
	for _, relType := range relationshipOrder {
		if issues, exists := grouped[relType]; exists {
			if !first {
				fmt.Fprintln(p.out)
			}
			first = false
			fmt.Fprintf(p.out, "**%s:**\n", relType)
			for _, rel := range issues {
				fmt.Fprintln(p.out, formatRelatedIssueItem(rel.Key, rel.Status, rel.Type, rel.Summary))
			}
			delete(grouped, relType)
		}
	}

	// Render any remaining relationship types not in the fixed order (sorted for deterministic output)
	remainingTypes := make([]string, 0, len(grouped))
	for relType := range grouped {
		remainingTypes = append(remainingTypes, relType)
	}
	slices.Sort(remainingTypes)

	for _, relType := range remainingTypes {
		if !first {
			fmt.Fprintln(p.out)
		}
		first = false
		fmt.Fprintf(p.out, "**%s:**\n", relType)
		for _, rel := range grouped[relType] {
			fmt.Fprintln(p.out, formatRelatedIssueItem(rel.Key, rel.Status, rel.Type, rel.Summary))
		}
	}
}

// formatRelatedIssueItem formats a related issue item with type annotation.
// Format: - **KEY** [Status] (Type) Summary
func formatRelatedIssueItem(key, status, issueType, summary string) string {
	statusInd := statusIndicator(status)
	return fmt.Sprintf("- **%s** [%s] (%s) %s", key, statusInd, issueType, summary)
}

// formatIssueListItem formats an issue list item in the standard format.
// Format with priority: - **KEY** [Status] [Priority] Summary
// Format without priority: - **KEY** [Status] Summary
func formatIssueListItem(key, status, priority, summary string) string {
	statusInd := statusIndicator(status)
	if priority == "" {
		return fmt.Sprintf("- **%s** [%s] %s", key, statusInd, summary)
	}
	priorityInd := priorityIndicator(priority)
	return fmt.Sprintf("- **%s** [%s] [%s] %s", key, statusInd, priorityInd, summary)
}

// statusIndicator returns a human-readable status marker.
func statusIndicator(status string) string {
	switch status {
	case "Done":
		return "Done"
	case "In Progress":
		return "In Progress"
	default: // "To Do" and others
		return "To Do"
	}
}

// priorityIndicator returns a P0-P4 priority marker.
func priorityIndicator(priority string) string {
	switch priority {
	case "Highest":
		return "P0"
	case "High":
		return "P1"
	case "Medium":
		return "P2"
	case "Low":
		return "P3"
	case "Lowest":
		return "P4"
	default:
		return "P2"
	}
}

// truncate shortens a string to maxLen, adding "..." if truncated.
func truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen-3]) + "..."
}

// Verify interface compliance at compile time.
var _ jira4claude.Printer = (*Printer)(nil)
