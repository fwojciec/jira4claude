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
	// Token is set, to the token text, when the issue is about a
	// protected-node token (see Converter.ToMarkdown) rather than a node
	// type; Node then holds the same token.
	Token string `json:"token,omitempty"`
	// Existing marks an issue about content already in the document being
	// written over, rather than in the new text (see Converter.Reattach): a
	// dropped issue with Existing set is content the write removes.
	Existing bool `json:"existing,omitempty"`
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
	var nodes, tokens, unreadable []string
	for _, d := range dropped {
		switch {
		case d.Token != "":
			tokens = append(tokens, d.Node)
		case d.Existing:
			unreadable = append(unreadable, d.Node)
		default:
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
	if len(unreadable) > 0 {
		parts = append(parts, "rewriting the "+field+" would remove existing content that has no Markdown form ("+
			strings.Join(unreadable, ", ")+"); use --append-description to add to it without rewriting it")
	}
	return &Error{
		Code: EValidation,
		Message: strings.Join(parts, "; ") +
			". Nothing was written. Pass --force to write anyway, without that content",
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
	//
	// Protected nodes, whose content or behaviour plain text would lose
	// (media, mediaSingle, mediaGroup, mediaInline, mention, status, date,
	// placeholder, and node types the converter does not know), are written
	// as tokens numbered in document order, such as {{mention:2 Jane Doe}}. The label
	// after the number is for the reader only. Emoji and inline cards are
	// written as their text and as links; reading reports nothing about
	// them.
	ToMarkdown(adf *ADFNode) (string, []ConversionIssue)

	// Reattach puts protected nodes back into an edited document. original
	// is the document ToMarkdown read; edited is what ToADF produced from the
	// edited Markdown. Each token of original still present in edited is
	// replaced by the original node, unchanged. The token's number and kind
	// are checked; its label is not trusted.
	//
	// Every node that would be lost is reported as ConversionDropped, with
	// the token in Node and Token: a token missing from edited,
	// a token used more than once (only the first is restored), a token whose
	// number original does not have or names another kind of node (original
	// changed since it was read), and a block token placed where its node is
	// not allowed. Such tokens other than the missing ones stay in the result
	// as text. Existing is set on the issues whose node of original the
	// result does not hold (a missing token, a misplaced block token), which
	// the write would remove; a token that is only left as text (unknown
	// number, wrong kind, used twice) removes nothing. A node whose token is
	// missing is not reported when the result holds it anyway, given back
	// with a part of original the edit left unchanged.
	//
	// Content of original that Markdown cannot carry is restored where the
	// edit left it alone: each part of edited that equals the unchanged read
	// of original, written back, is replaced by the part of original it was
	// read from, so an unchanged rewrite gives original back. A changed node
	// that matches a node of original (same type, as many siblings of that
	// type on each side, between the same unchanged siblings or ends of its
	// container, under matched parents) keeps the original's attributes and block
	// marks where the edit left them as they read, and always its localId; a
	// node put back from its token is never merged. An inline card whose link
	// comes back in a form only that card can take is restored; an unchanged
	// part holding one comes back with it, whatever the rest of the edit.
	//
	// What the result still changes is reported with Existing set, one issue
	// per node or mark type with the count: emoji turned into text, inline
	// cards into plain links, decision lists, layouts, nested expands and
	// block task items flattened, marks and attribute values Markdown cannot
	// show, localIds not kept, and containers whose shape the read changes
	// as ConversionDegraded; inline comment (annotation) marks as
	// ConversionDropped. What the read itself reports about the parts not
	// given back whole is reported again: dropped content as
	// ConversionDropped, a change of form as ConversionDegraded. A nil edited
	// (a clear) reports none of these but the dropped content.
	//
	// edited is not modified. A nil document counts as an empty one. Always
	// returns a non-nil *ADFNode.
	Reattach(original, edited *ADFNode) (*ADFNode, []ConversionIssue)

	// FindTokens returns the text in doc, in document order, that reads as
	// a protected-node token, whatever its salt. It is for new content (a
	// create, a comment, an append), where nothing restores a token. Text in
	// code is not read. A nil doc has none.
	FindTokens(doc *ADFNode) []string
}
