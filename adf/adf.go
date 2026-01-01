// Package adf provides conversion between GitHub Flavored Markdown (GFM)
// and Atlassian Document Format (ADF).
package adf

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
func (c *Converter) ToADF(markdown string) (map[string]any, error) {
	return toADF(markdown)
}

// ToMarkdown converts ADF to GitHub-flavored markdown.
func (c *Converter) ToMarkdown(adfDoc map[string]any) (string, error) {
	return toMarkdown(adfDoc)
}
