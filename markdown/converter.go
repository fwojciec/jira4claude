// Package markdown provides conversion between GitHub Flavored Markdown (GFM)
// and Atlassian Document Format (ADF) using the goldmark library.
package markdown

import "github.com/fwojciec/jira4claude"

// Compile-time interface verification.
var _ jira4claude.Converter = (*Converter)(nil)

// Converter implements jira4claude.Converter using goldmark for GFM parsing.
type Converter struct {
	localID func(n int) string
}

// Option configures a Converter.
type Option func(*Converter)

// WithLocalIDs sets the source of the localId attributes that ToADF gives
// task lists and task items. The function receives the sequence number of the
// id within one conversion, starting at 0 for every ToADF call, so a pure
// function gives identical output for identical input. The default source
// ignores n and returns a random UUID-like id.
func WithLocalIDs(localID func(n int) string) Option {
	return func(c *Converter) {
		c.localID = localID
	}
}

// New creates a new Converter instance.
func New(opts ...Option) *Converter {
	c := &Converter{localID: randomLocalID}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// ToADF converts GitHub-flavored markdown to ADF.
// Returns the ADF document and any conversion issues (dropped or degraded content).
// Always returns a non-nil *ADFNode, even for empty input.
func (c *Converter) ToADF(markdown string) (*jira4claude.ADFNode, []jira4claude.ConversionIssue) {
	return toADF(markdown, c.localID)
}

// ToMarkdown converts ADF to GitHub-flavored markdown.
// Returns the markdown string and any conversion issues (dropped or degraded content).
// If adfDoc is nil, returns an empty string.
func (c *Converter) ToMarkdown(adfDoc *jira4claude.ADFNode) (string, []jira4claude.ConversionIssue) {
	return toMarkdown(adfDoc)
}
