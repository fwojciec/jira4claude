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
	Node   string `json:"node"`   // e.g. "Image", "HTMLBlock", or a token such as "{{mention:2 Jane Doe}}"
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
	var nodes, tokens []string
	for _, d := range dropped {
		if IsProtectedToken(d.Node) {
			tokens = append(tokens, d.Node)
		} else {
			nodes = append(nodes, d.Node)
		}
	}
	var parts []string
	if len(nodes) > 0 {
		parts = append(parts, field+" would drop unsupported content ("+strings.Join(nodes, ", ")+
			"); remove it")
	}
	if len(tokens) > 0 {
		parts = append(parts, field+" would lose existing content ("+strings.Join(tokens, ", ")+
			"); put each token back, unchanged and on a line of its own for a block, where the content belongs")
	}
	return &Error{
		Code: EValidation,
		Message: strings.Join(parts, "; ") +
			". Nothing was written. Pass --force to write anyway, without that content",
	}
}

// IsProtectedToken reports whether node, the Node of a ConversionIssue,
// is a protected-node token (see Converter.ToMarkdown) rather than a node
// type: such an issue is about existing content of the document.
func IsProtectedToken(node string) bool {
	return strings.HasPrefix(node, "{{") && strings.HasSuffix(node, "}}")
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
	//
	// Protected nodes, whose content or behaviour plain text would lose
	// (media, mediaSingle, mediaGroup, mediaInline, mention, and node types
	// the converter does not know), are written as tokens numbered in
	// document order, such as {{mention:2 Jane Doe}}. The label after the
	// number is for the reader only.
	ToMarkdown(adf *ADFNode) (string, []ConversionIssue)

	// Reattach puts protected nodes back into an edited document. original
	// is the document ToMarkdown read; edited is what ToADF produced from the
	// edited Markdown. Each token of original still present in edited is
	// replaced by the original node, unchanged. The token's number and kind
	// are checked; its label is not trusted.
	//
	// Every node that would be lost is reported as ConversionDropped, with
	// the token in Node (see IsProtectedToken): a token missing from edited,
	// a token used more than once (only the first is restored), a token whose
	// number original does not have or names another kind of node (original
	// changed since it was read), and a block token placed where its node is
	// not allowed. Such tokens other than the missing ones stay in the result
	// as text.
	//
	// edited is not modified. A nil document counts as an empty one. Always
	// returns a non-nil *ADFNode.
	Reattach(original, edited *ADFNode) (*ADFNode, []ConversionIssue)
}
