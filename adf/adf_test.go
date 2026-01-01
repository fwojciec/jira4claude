package adf_test

import (
	"testing"

	"github.com/fwojciec/jira4claude"
	"github.com/fwojciec/jira4claude/adf"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	t.Parallel()

	t.Run("returns a Converter that implements jira4claude.Converter", func(t *testing.T) {
		t.Parallel()

		converter := adf.New()

		// This verifies that the returned type satisfies the interface
		var _ jira4claude.Converter = converter
		assert.NotNil(t, converter)
	})
}

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

			converter := adf.New()

			// Markdown -> ADF -> Markdown
			adfDoc, err := converter.ToADF(tc.markdown)
			require.NoError(t, err)

			result, err := converter.ToMarkdown(adfDoc)
			require.NoError(t, err)

			assert.Equal(t, tc.markdown, result)
		})
	}
}
