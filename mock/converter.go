package mock

import "github.com/fwojciec/jira4claude"

// Compile-time interface verification.
var _ jira4claude.Converter = (*Converter)(nil)

// Converter is a mock implementation of jira4claude.Converter.
// Each method delegates to its corresponding function field (e.g., ToADF calls ToADFFn).
// Calling a method without setting its function field will panic.
type Converter struct {
	ToADFFn      func(markdown string) (map[string]any, error)
	ToMarkdownFn func(adf map[string]any) (string, error)
}

func (c *Converter) ToADF(markdown string) (map[string]any, error) {
	return c.ToADFFn(markdown)
}

func (c *Converter) ToMarkdown(adf map[string]any) (string, error) {
	return c.ToMarkdownFn(adf)
}
