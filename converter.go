package jira4claude

// Converter handles conversion between GitHub Flavored Markdown (GFM) and Atlassian Document Format (ADF).
// Methods return warnings as []string to report any skipped or unsupported content.
// Warnings are informational - operations always succeed with best-effort output.
type Converter interface {
	// ToADF converts GitHub-flavored markdown to ADF.
	// Returns the ADF document and any warnings about skipped/unsupported content.
	// Always returns a non-nil *ADFNode, even for empty input.
	ToADF(markdown string) (*ADFNode, []string)

	// ToMarkdown converts ADF to GitHub-flavored markdown.
	// Returns the markdown string and any warnings about skipped/unsupported content.
	// If adf is nil, returns an empty string.
	ToMarkdown(adf *ADFNode) (string, []string)
}
