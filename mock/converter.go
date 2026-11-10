package mock

import "github.com/fwojciec/jira4claude"

// Compile-time interface verification.
var _ jira4claude.Converter = (*Converter)(nil)

// Converter is a mock implementation of jira4claude.Converter.
// Each method delegates to its corresponding function field (e.g., ToADF calls ToADFFn).
// Calling a method without setting its function field will panic.
type Converter struct {
	ToADFFn      func(markdown string) (*jira4claude.ADFNode, []jira4claude.ConversionIssue)
	ToMarkdownFn func(adf *jira4claude.ADFNode) (string, []jira4claude.ConversionIssue)
	ReattachFn   func(original, edited *jira4claude.ADFNode) (*jira4claude.ADFNode, []jira4claude.ConversionIssue)
}

func (c *Converter) ToADF(markdown string) (*jira4claude.ADFNode, []jira4claude.ConversionIssue) {
	return c.ToADFFn(markdown)
}

func (c *Converter) ToMarkdown(adf *jira4claude.ADFNode) (string, []jira4claude.ConversionIssue) {
	return c.ToMarkdownFn(adf)
}

func (c *Converter) Reattach(original, edited *jira4claude.ADFNode) (*jira4claude.ADFNode, []jira4claude.ConversionIssue) {
	return c.ReattachFn(original, edited)
}
