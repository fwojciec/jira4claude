package markdown

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"
	"unicode"

	"github.com/fwojciec/jira4claude"
)

// maxSummaryLength is the maximum length for issue summaries in list format.
const maxSummaryLength = 60

// Printer outputs GitHub-Flavored Markdown format. Data goes to out; errors,
// warnings and [info] notices (an empty result, hidden fields) go to err, so
// that stdout holds only data.
type Printer struct {
	out       io.Writer
	err       io.Writer
	serverURL string
}

// NewPrinter creates a markdown printer that writes to out.
// Warnings, errors and [info] notices are discarded since no stderr writer is
// provided.
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

	// Custom Fields section
	p.renderCustomFields(view.CustomFields)

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

	p.renderAttachmentsSection(view.Attachments)

	// URL as markdown link at end
	if view.URL != "" {
		fmt.Fprintf(p.out, "\n[View in Jira](%s)\n", view.URL)
	}
}

// renderCustomFields prints a "## Custom Fields" section listing each populated
// custom field as "**Name:** value". The field ID is used as a label fallback
// when the display name is empty. Values are the raw API JSON, compacted to a
// single line. Fields are sorted by label for deterministic output.
func (p *Printer) renderCustomFields(fields map[string]jira4claude.CustomFieldValue) {
	if len(fields) == 0 {
		return
	}

	type row struct{ label, value string }
	rows := make([]row, 0, len(fields))
	for id, cf := range fields {
		label := cf.Name
		if label == "" {
			label = id
		}
		rows = append(rows, row{label: label, value: compactJSON(cf.Value)})
	}
	slices.SortFunc(rows, func(a, b row) int {
		return strings.Compare(a.label, b.label)
	})

	fmt.Fprint(p.out, "\n## Custom Fields\n\n")
	for _, r := range rows {
		fmt.Fprintf(p.out, "- **%s:** %s\n", r.label, r.value)
	}
}

// compactJSON renders raw JSON on a single line. If the value is not valid JSON
// (unexpected from the API), the raw bytes are returned unchanged.
func compactJSON(raw json.RawMessage) string {
	var buf bytes.Buffer
	if err := json.Compact(&buf, raw); err != nil {
		return string(raw)
	}
	return buf.String()
}

// Issues prints multiple issues as a markdown list.
func (p *Printer) Issues(view jira4claude.IssueListView) {
	items := view.Issues
	if len(items) == 0 {
		fmt.Fprintln(p.err, "[info] No issues found")
		return
	}

	for _, item := range items {
		summary := truncate(item.Summary, maxSummaryLength)
		fmt.Fprintln(p.out, formatIssueListItem(item.Key, item.Status, item.Priority, summary))
	}
}

// Comment prints a single comment.
func (p *Printer) Comment(view jira4claude.CommentView) {
	p.renderComment(view)
}

// Transitions prints available transitions for an issue.
func (p *Printer) Transitions(key string, ts []*jira4claude.Transition) {
	if len(ts) == 0 {
		fmt.Fprintf(p.err, "[info] No transitions available for %s\n", key)
		return
	}

	for _, t := range ts {
		fmt.Fprintf(p.out, "- %s\n", t.Name)
	}
}

// Field rendering column widths (eyeball-aligned, not pixel-perfect).
const (
	fieldsIDWidth       = 18
	fieldsNameWidth     = 28
	fieldsMaxAllowed    = 5
	fieldsIndent        = "  "
	fieldsAllowedIndent = "    "
)

// Fields prints settable issue fields grouped by REQUIRED and CUSTOM (optional).
func (p *Printer) Fields(view jira4claude.IssueFieldsView) {
	if len(view.Fields) == 0 {
		if view.Scope == jira4claude.FieldScopeFiltered {
			fmt.Fprintf(p.err, "[info] No fields match the filter for %s\n", view.Source)
		} else {
			fmt.Fprintf(p.err, "[info] No fields for %s\n", view.Source)
		}
		return
	}

	fmt.Fprintf(p.out, "Fields for %s:\n", view.Source)

	// Filtered results span all field kinds (required, custom, optional
	// builtins), so a flat list is clearer than the required/custom buckets —
	// and avoids dropping optional builtins the buckets would discard.
	if view.Scope == jira4claude.FieldScopeFiltered {
		fmt.Fprint(p.out, "\nMATCHES\n")
		for _, f := range view.Fields {
			p.renderFieldLine(f)
		}
		return
	}

	// Partition into required, optional-custom, and optional-builtin buckets.
	required := make([]*jira4claude.IssueField, 0, len(view.Fields))
	optionalCustom := make([]*jira4claude.IssueField, 0, len(view.Fields))
	optionalOther := make([]*jira4claude.IssueField, 0, len(view.Fields))
	for _, f := range view.Fields {
		switch {
		case f.Required:
			required = append(required, f)
		case strings.HasPrefix(f.ID, "customfield_"):
			optionalCustom = append(optionalCustom, f)
		default:
			optionalOther = append(optionalOther, f)
		}
	}

	// Within REQUIRED: customfields first, then builtins (stable order).
	sortRequiredFields(required)

	if len(required) > 0 {
		fmt.Fprint(p.out, "\nREQUIRED\n")
		for _, f := range required {
			p.renderFieldLine(f)
		}
	}

	if len(optionalCustom) > 0 {
		fmt.Fprint(p.out, "\nCUSTOM (optional)\n")
		for _, f := range optionalCustom {
			p.renderFieldLine(f)
		}
	}

	// Optional builtins are only surfaced under --all (scope "all"); the
	// default selection deliberately hides them, and the command never passes
	// them in that case.
	if view.Scope == jira4claude.FieldScopeAll && len(optionalOther) > 0 {
		fmt.Fprint(p.out, "\nOPTIONAL\n")
		for _, f := range optionalOther {
			p.renderFieldLine(f)
		}
	}

	if view.Omitted > 0 {
		fmt.Fprintf(p.err, "[info] %d more field(s) hidden — use --all to show all, or --filter to find one.\n", view.Omitted)
	}
}

// sortRequiredFields reorders the slice in place so that customfields come
// before builtins, preserving the relative order within each group.
func sortRequiredFields(fields []*jira4claude.IssueField) {
	customs := make([]*jira4claude.IssueField, 0, len(fields))
	builtins := make([]*jira4claude.IssueField, 0, len(fields))
	for _, f := range fields {
		if strings.HasPrefix(f.ID, "customfield_") {
			customs = append(customs, f)
		} else {
			builtins = append(builtins, f)
		}
	}
	copy(fields, append(customs, builtins...))
}

// renderFieldLine emits a single field row, plus an `allowed:` line when
// the field has allowed values.
func (p *Printer) renderFieldLine(f *jira4claude.IssueField) {
	fmt.Fprintf(
		p.out,
		"%s%s  %s  %s\n",
		fieldsIndent,
		padRight(f.ID, fieldsIDWidth),
		padRight(truncate(f.Name, fieldsNameWidth), fieldsNameWidth),
		schemaTypeString(f),
	)
	if len(f.AllowedValues) > 0 {
		fmt.Fprintf(p.out, "%sallowed: %s\n", fieldsAllowedIndent, formatAllowedValues(f.AllowedValues))
	}
}

// schemaTypeString produces "array<items>" for array types, the bare type
// otherwise, and "unknown" when the schema type is empty.
func schemaTypeString(f *jira4claude.IssueField) string {
	if f.SchemaType == "" {
		return "unknown"
	}
	if f.SchemaType == "array" {
		items := f.SchemaItems
		if items == "" {
			items = "unknown"
		}
		return "array<" + items + ">"
	}
	return f.SchemaType
}

// formatAllowedValues renders allowed values as comma-separated quoted
// strings, truncating with `, …` past fieldsMaxAllowed. Values are quoted
// with strconv.Quote so that embedded quotes, backslashes, and control
// characters are escaped instead of breaking the output line.
func formatAllowedValues(values []jira4claude.FieldAllowedValue) string {
	limit := len(values)
	truncated := false
	if limit > fieldsMaxAllowed {
		limit = fieldsMaxAllowed
		truncated = true
	}
	parts := make([]string, 0, limit)
	for i := 0; i < limit; i++ {
		parts = append(parts, strconv.Quote(values[i].Value))
	}
	out := strings.Join(parts, ", ")
	if truncated {
		out += ", …"
	}
	return out
}

// padRight returns s padded with spaces on the right to width runes.
// If s is longer than width it is returned unchanged.
func padRight(s string, width int) string {
	r := []rune(s)
	if len(r) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(r))
}

// Links prints issue links using RelatedIssueView.
func (p *Printer) Links(key string, links []jira4claude.RelatedIssueView) {
	if len(links) == 0 {
		fmt.Fprintf(p.err, "[info] No links for %s\n", key)
		return
	}

	p.renderRelatedIssuesGrouped(links)
}

// Attachments prints one line per attachment for a script to split on tabs:
// ID, exact size in bytes, MIME type and filename. The filename, the only
// free-form field, comes last, with control characters escaped (see
// escapeControl) so that it cannot break the line or add a column.
func (p *Printer) Attachments(key string, attachments []jira4claude.AttachmentView) {
	if len(attachments) == 0 {
		fmt.Fprintf(p.err, "[info] No attachments for %s\n", key)
		return
	}

	for _, a := range attachments {
		fmt.Fprintf(p.out, "%s\t%d\t%s\t%s\n",
			escapeControl(a.ID), a.Size, escapeControl(a.MimeType), escapeControl(a.Filename))
	}
}

// renderAttachmentsSection prints the "## Attachments" section of an issue,
// one "- ID filename (size, type)" line per attachment, with a human-readable
// size. It prints nothing when there are no attachments.
func (p *Printer) renderAttachmentsSection(attachments []jira4claude.AttachmentView) {
	if len(attachments) == 0 {
		return
	}
	fmt.Fprint(p.out, "\n## Attachments\n\n")
	for _, a := range attachments {
		fmt.Fprintf(p.out, "- %s %s (%s, %s)\n",
			escapeControl(a.ID), escapeControl(a.Filename), humanSize(a.Size), escapeControl(a.MimeType))
	}
}

// humanSize formats a byte count in binary units with one decimal, such as
// "22.6 KiB"; counts under 1 KiB are shown exactly, such as "512 B".
func humanSize(n int64) string {
	const unit = 1024
	if n < unit {
		return strconv.FormatInt(n, 10) + " B"
	}
	div, exp := int64(unit), 0
	for m := n / unit; m >= unit && exp < 4; m /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGTP"[exp])
}

// escapeControl replaces control characters (tab, newline, escape and the
// like) with their Go escape sequences, and a backslash with two, so that a
// server-supplied string stays on one line and in one column and the escaping
// can be undone (strconv.Unquote of the value in double quotes).
func escapeControl(s string) string {
	needsEscape := func(r rune) bool { return r == '\\' || unicode.IsControl(r) }
	if !strings.ContainsFunc(s, needsEscape) {
		return s
	}
	var b strings.Builder
	for _, r := range s {
		if !needsEscape(r) {
			b.WriteRune(r)
			continue
		}
		quoted := strconv.QuoteRune(r)
		b.WriteString(quoted[1 : len(quoted)-1])
	}
	return b.String()
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

// ConversionIssue prints a conversion issue to stderr as a warning.
func (p *Printer) ConversionIssue(issue jira4claude.ConversionIssue) {
	p.Warning(issue.String())
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
// Passes through the status as-is to preserve custom statuses like "Won't Do".
func statusIndicator(status string) string {
	return status
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
