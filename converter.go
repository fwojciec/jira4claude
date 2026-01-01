package jira4claude

// Converter handles conversion between markdown and Atlassian Document Format.
type Converter interface {
	ToADF(markdown string) map[string]any
	ToMarkdown(adf map[string]any) string
}
