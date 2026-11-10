package markdown_test

import (
	"testing"

	"github.com/fwojciec/jira4claude"
	"github.com/fwojciec/jira4claude/markdown"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestToADF_EmptyCodeBlock covers fenced code blocks with no text. The ADF
// schema rejects an empty text node, so an empty codeBlock has no content
// (found by FuzzToADFSchemaValid, #147).
func TestToADF_EmptyCodeBlock(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		md   string
		want jira4claude.ADFNode
	}{
		"unclosed fence": {md: "```", want: jira4claude.ADFNode{Type: "codeBlock"}},
		"closed fence":   {md: "```\n```", want: jira4claude.ADFNode{Type: "codeBlock"}},
		"one blank line": {md: "```\n\n```", want: jira4claude.ADFNode{Type: "codeBlock"}},
		"with language": {md: "```go\n```", want: jira4claude.ADFNode{
			Type:  "codeBlock",
			Attrs: []byte(`{"language":"go"}`),
		}},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			conv := markdown.New()
			result, issues := conv.ToADF(tc.md)

			requireValidADF(t, result)
			assert.Empty(t, issues)
			assert.Equal(t, []jira4claude.ADFNode{tc.want}, result.Content)

			md, _ := conv.ToMarkdown(result)
			back, _ := conv.ToADF(md)
			assert.Equal(t, result, back, "read back as %q", md)
		})
	}
}

// TestToADF_EmptyTableCell covers table cells with no text. The ADF schema
// requires cell content, so an empty cell holds an empty paragraph (found by
// FuzzToADFSchemaValid, #147).
func TestToADF_EmptyTableCell(t *testing.T) {
	t.Parallel()

	emptyCell := []jira4claude.ADFNode{{Type: "paragraph"}}

	for _, md := range []string{"|\n|-", "| a | b |\n|---|---|\n| | x |"} {
		t.Run(md, func(t *testing.T) {
			t.Parallel()

			conv := markdown.New()
			result, issues := conv.ToADF(md)

			requireValidADF(t, result)
			assert.Empty(t, issues)
			var empty []jira4claude.ADFNode
			for _, row := range result.Content[0].Content {
				for _, cell := range row.Content {
					if len(cell.Content) == 1 && len(cell.Content[0].Content) == 0 {
						empty = append(empty, cell.Content...)
					}
				}
			}
			assert.Equal(t, emptyCell, empty)

			read, _ := conv.ToMarkdown(result)
			back, _ := conv.ToADF(read)
			assert.Equal(t, result, back, "read back as %q", read)
		})
	}

	t.Run("empty cell of a table unwrapped in a list item is reported as before", func(t *testing.T) {
		t.Parallel()

		result, issues := markdown.New().ToADF("- item\n\n  | a | b |\n  |---|---|\n  |   | x |")

		requireValidADF(t, result)
		// The placeholder paragraph does not leak into the list item. The
		// dropped tableCell is a known false refusal (#147 comments), kept
		// as it was.
		assert.Equal(t, []jira4claude.ConversionIssue{
			unwrappedIssue("table", "listItem"),
			droppedIssue("tableCell"),
		}, issues)
		para := func(s string) jira4claude.ADFNode {
			return jira4claude.ADFNode{Type: "paragraph", Content: []jira4claude.ADFNode{txt(s)}}
		}
		assert.Equal(t, []jira4claude.ADFNode{para("item"), para("a"), para("b"), para("x")},
			result.Content[0].Content[0].Content)
	})
}

// TestToADF_EmptyDetails covers a <details> block with no body. The ADF
// schema requires expand content, so an empty expand holds an empty paragraph
// (found by FuzzToADFSchemaValid, #147).
func TestToADF_EmptyDetails(t *testing.T) {
	t.Parallel()

	for _, md := range []string{
		"<details><summary></summary>\n\n</details>",
		"<details><summary>Title</summary>\n\n</details>",
		"<details>\n<summary>Title</summary>\n\n![a](http://x/y.png)\n\n</details>",
	} {
		t.Run(md, func(t *testing.T) {
			t.Parallel()

			conv := markdown.New()
			result, _ := conv.ToADF(md)

			requireValidADF(t, result)
			require.Len(t, result.Content, 1)
			assert.Equal(t, "expand", result.Content[0].Type)
			assert.Equal(t, []jira4claude.ADFNode{{Type: "paragraph"}}, result.Content[0].Content)

			read, _ := conv.ToMarkdown(result)
			back, _ := conv.ToADF(read)
			assert.Equal(t, result, back, "read back as %q", read)
		})
	}
}

// TestToADF_EmptyPanel covers an alert blockquote with no body. The ADF
// schema requires panel content, so an empty panel holds an empty paragraph
// (found by FuzzToADFSchemaValid, #147).
func TestToADF_EmptyPanel(t *testing.T) {
	t.Parallel()

	for md, issues := range map[string][]jira4claude.ConversionIssue{
		"> [!NOTE]":                   nil,
		"> [!WARNING]\n> ![x](y.png)": {droppedIssue("Image")},
	} {
		t.Run(md, func(t *testing.T) {
			t.Parallel()

			conv := markdown.New()
			result, got := conv.ToADF(md)

			requireValidADF(t, result)
			assert.Equal(t, issues, got)
			require.Len(t, result.Content, 1)
			assert.Equal(t, "panel", result.Content[0].Type)
			assert.Equal(t, []jira4claude.ADFNode{{Type: "paragraph"}}, result.Content[0].Content)

			read, _ := conv.ToMarkdown(result)
			back, _ := conv.ToADF(read)
			assert.Equal(t, result, back, "read back as %q", read)
		})
	}
}

// TestToADF_EmptyContainerUnwrapped checks that an empty panel or expand
// unwrapped inside a list item is still reported as dropped, as before its
// placeholder paragraph existed, and leaves a schema-valid item.
func TestToADF_EmptyContainerUnwrapped(t *testing.T) {
	t.Parallel()

	for md, node := range map[string]string{
		"- > [!NOTE]": "panel",
		"- a\n\n  <details><summary>T</summary>\n\n  </details>": "expand",
	} {
		t.Run(md, func(t *testing.T) {
			t.Parallel()

			result, issues := markdown.New().ToADF(md)

			requireValidADF(t, result)
			assert.Equal(t, []jira4claude.ConversionIssue{droppedIssue(node)}, issues)
		})
	}
}

// TestToADF_EmptyListItem covers list items with no content, or whose
// content opens with a list. The ADF schema requires a listItem to open with
// a paragraph (or code or media), so such an item opens with an empty
// paragraph (found by FuzzToADFSchemaValid, #147).
func TestToADF_EmptyListItem(t *testing.T) {
	t.Parallel()

	emptyItem := jira4claude.ADFNode{
		Type:    "listItem",
		Content: []jira4claude.ADFNode{{Type: "paragraph"}},
	}

	t.Run("bullet", func(t *testing.T) {
		t.Parallel()

		result, issues := markdown.New().ToADF("*")

		requireValidADF(t, result)
		assert.Empty(t, issues)
		assert.Equal(t, []jira4claude.ADFNode{{
			Type:    "bulletList",
			Content: []jira4claude.ADFNode{emptyItem},
		}}, result.Content)
	})

	t.Run("ordered, between items", func(t *testing.T) {
		t.Parallel()

		result, issues := markdown.New().ToADF("1. a\n2.\n3. c")

		requireValidADF(t, result)
		assert.Empty(t, issues)
		assert.Equal(t, []jira4claude.ADFNode{
			{Type: "listItem", Content: []jira4claude.ADFNode{{Type: "paragraph", Content: []jira4claude.ADFNode{txt("a")}}}},
			emptyItem,
			{Type: "listItem", Content: []jira4claude.ADFNode{{Type: "paragraph", Content: []jira4claude.ADFNode{txt("c")}}}},
		}, result.Content[0].Content)
	})

	t.Run("item that opens with a nested list", func(t *testing.T) {
		t.Parallel()

		conv := markdown.New()
		for _, md := range []string{"* *", "- 1. a", "1. - a\n   - b"} {
			result, issues := conv.ToADF(md)

			requireValidADF(t, result)
			assert.Empty(t, issues, md)
			item := result.Content[0].Content[0]
			if assert.Len(t, item.Content, 2, md) {
				assert.Equal(t, jira4claude.ADFNode{Type: "paragraph"}, item.Content[0], md)
			}

			read, _ := conv.ToMarkdown(result)
			back, _ := conv.ToADF(read)
			assert.Equal(t, result, back, "%q read back as %q", md, read)
		}
	})

	t.Run("item whose only content is dropped", func(t *testing.T) {
		t.Parallel()

		result, issues := markdown.New().ToADF("- ![a](http://x/y.png)")

		requireValidADF(t, result)
		assert.Equal(t, []jira4claude.ConversionIssue{droppedIssue("Image")}, issues)
	})
}
