// Package markdown provides conversion between GitHub Flavored Markdown (GFM)
// and Atlassian Document Format (ADF) using the goldmark library.
package markdown

import "github.com/fwojciec/jira4claude"

// Compile-time interface verification.
var _ jira4claude.Converter = (*Converter)(nil)

// Converter implements jira4claude.Converter using goldmark for GFM parsing.
type Converter struct{}

// New creates a new Converter instance.
func New() *Converter {
	return &Converter{}
}

// ToADF converts GitHub-flavored markdown to ADF.
// Returns the ADF document and any warnings about skipped/unsupported content.
// Always returns a non-nil *ADFNode, even for empty input.
func (c *Converter) ToADF(markdown string) (*jira4claude.ADFNode, []string) {
	return toADF(markdown)
}

// ToMarkdown converts ADF to GitHub-flavored markdown.
// Returns the markdown string and any warnings about skipped/unsupported content.
// If adfDoc is nil, returns an empty string.
func (c *Converter) ToMarkdown(adfDoc *jira4claude.ADFNode) (string, []string) {
	return toMarkdown(adfDoc)
}
