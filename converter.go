package jira4claude

import "strings"

// Conversion issue kinds.
const (
	// ConversionDropped means content or behaviour was lost: the node is not
	// present in the output at all. Writes refuse on dropped content unless
	// forced.
	ConversionDropped = "dropped"

	// ConversionDegraded means the node changed form but its content was
	// kept. Degraded issues are always warnings, never refusals.
	ConversionDegraded = "degraded"
)

// ConversionIssue reports one kind of content that a conversion could not
// carry over faithfully.
type ConversionIssue struct {
	Kind   string `json:"kind"`   // ConversionDropped or ConversionDegraded
	Node   string `json:"node"`   // e.g. "Image", "HTMLBlock", "extension"
	Detail string `json:"detail"` // human-readable explanation
}

// String renders the issue as a single human-readable line.
func (i ConversionIssue) String() string {
	s := i.Kind + " " + i.Node
	if i.Detail != "" {
		s += ": " + i.Detail
	}
	return s
}

// DroppedIssues returns only the issues of kind ConversionDropped.
func DroppedIssues(issues []ConversionIssue) []ConversionIssue {
	var dropped []ConversionIssue
	for _, i := range issues {
		if i.Kind == ConversionDropped {
			dropped = append(dropped, i)
		}
	}
	return dropped
}

// DroppedContentError returns a validation error naming every dropped node in
// issues, or nil when nothing was dropped. field names the input being
// converted (e.g. "description", "comment body").
func DroppedContentError(field string, issues []ConversionIssue) error {
	dropped := DroppedIssues(issues)
	if len(dropped) == 0 {
		return nil
	}
	nodes := make([]string, len(dropped))
	for i, d := range dropped {
		nodes[i] = d.Node
	}
	return &Error{
		Code: EValidation,
		Message: field + " would drop unsupported content (" + strings.Join(nodes, ", ") +
			"); nothing was written. Remove it, or pass --force to write without it",
	}
}

// Converter handles conversion between GitHub Flavored Markdown (GFM) and
// Atlassian Document Format (ADF).
//
// Both methods return best-effort output plus a list of ConversionIssue
// describing anything that was dropped or degraded. Callers decide what an
// issue means: writes refuse on ConversionDropped (unless forced), while reads
// only warn.
type Converter interface {
	// ToADF converts GitHub-flavored markdown to ADF.
	// Returns the ADF document and any conversion issues.
	// Always returns a non-nil *ADFNode, even for empty input.
	ToADF(markdown string) (*ADFNode, []ConversionIssue)

	// ToMarkdown converts ADF to GitHub-flavored markdown.
	// Returns the markdown string and any conversion issues.
	// If adf is nil, returns an empty string.
	ToMarkdown(adf *ADFNode) (string, []ConversionIssue)
}
