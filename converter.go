package jira4claude

// Converter handles conversion between markdown and Atlassian Document Format.
// Methods return errors to report any skipped or unsupported content.
type Converter interface {
	ToADF(markdown string) (map[string]any, error)
	ToMarkdown(adf map[string]any) (string, error)
}
