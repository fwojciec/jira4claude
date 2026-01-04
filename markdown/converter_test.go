package markdown_test

import (
	"testing"

	"github.com/fwojciec/jira4claude/markdown"
	"github.com/stretchr/testify/assert"
)

func TestRoundTrip(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		markdown string
	}{
		{"plain text", "Hello, world!"},
		{"bold text", "This is **bold** text."},
		{"italic text", "This is *italic* text."},
		{"inline code", "Use `fmt.Println` function."},
		{"code block", "```go\nfmt.Println(\"hello\")\n```"},
		{"heading", "## My Heading"},
		{"bullet list", "- Item 1\n- Item 2"},
		{"ordered list", "1. First\n2. Second"},
		{"link", "Visit [Google](https://google.com) for more."},
		{"blockquote", "> This is a quote."},
		{"multiple paragraphs", "First paragraph.\n\nSecond paragraph."},
		{"combined bold and italic", "This is ***bold and italic*** text."},
		{"complex document", `# Main Heading

This is a paragraph with **bold** and *italic* text.

## Subheading

- First item
- Second item

1. Numbered one
2. Numbered two

> A blockquote

` + "```go\nfunc main() {}\n```"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			converter := markdown.New()

			// Markdown -> ADF -> Markdown
			adfDoc, warnings := converter.ToADF(tc.markdown)
			assert.Empty(t, warnings)

			result, warnings := converter.ToMarkdown(adfDoc)
			assert.Empty(t, warnings)

			assert.Equal(t, tc.markdown, result)
		})
	}
}
