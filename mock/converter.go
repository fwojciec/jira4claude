package mock

import "github.com/fwojciec/jira4claude"

// Compile-time interface verification.
var _ jira4claude.Converter = (*Converter)(nil)

// Converter is a mock implementation of jira4claude.Converter.
// Each method delegates to its corresponding function field (e.g., ToADF calls ToADFFn).
// Calling a method without setting its function field will panic.
type Converter struct {
	ToADFFn      func(markdown string) (jira4claude.ADF, []string)
	ToMarkdownFn func(adf jira4claude.ADF) (string, []string)
}

func (c *Converter) ToADF(markdown string) (jira4claude.ADF, []string) {
	return c.ToADFFn(markdown)
}

func (c *Converter) ToMarkdown(adf jira4claude.ADF) (string, []string) {
	return c.ToMarkdownFn(adf)
}
