package markdown_test

import (
	"testing"

	"github.com/fwojciec/jira4claude"
	"github.com/fwojciec/jira4claude/markdown"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// literalLines builds an ADF paragraph holding the given lines as plain text
// separated by hard breaks. An empty line is two consecutive hard breaks.
func literalLines(lines ...string) jira4claude.ADFNode {
	var content []jira4claude.ADFNode
	for i, line := range lines {
		if i > 0 {
			content = append(content, jira4claude.ADFNode{Type: "hardBreak"})
		}
		if line != "" {
			content = append(content, jira4claude.ADFNode{Type: "text", Text: line})
		}
	}
	return jira4claude.ADFNode{Type: "paragraph", Content: content}
}

// adfDoc wraps block nodes in an ADF document.
func adfDoc(content ...jira4claude.ADFNode) *jira4claude.ADFNode {
	return &jira4claude.ADFNode{Type: "doc", Version: 1, Content: content}
}

// TestToADF_LiteralBlockHTML covers raw HTML blocks that are not part of a
// <details> construct: their source text is kept as a literal paragraph, one
// hard break between lines, and nothing is reported.
func TestToADF_LiteralBlockHTML(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		input    string
		expected *jira4claude.ADFNode
	}{
		{
			name:     "a lone unknown tag becomes a paragraph of its text",
			input:    "<word>",
			expected: adfDoc(para("<word>")),
		},
		{
			name:     "a lone placeholder between paragraphs is kept in place",
			input:    "Run it with\n\n<path>\n\nand wait.",
			expected: adfDoc(para("Run it with"), para("<path>"), para("and wait.")),
		},
		{
			name:     "a multi-line div keeps every line",
			input:    "<div class=\"x\">\n  <p>Hello</p>\n  world\n</div>",
			expected: adfDoc(literalLines(`<div class="x">`, "  <p>Hello</p>", "  world", "</div>")),
		},
		{
			name:     "a single-line div with content is kept",
			input:    "<div>html block one</div>",
			expected: adfDoc(para("<div>html block one</div>")),
		},
		{
			name:     "a single-line comment is kept",
			input:    "<!-- note -->",
			expected: adfDoc(para("<!-- note -->")),
		},
		{
			name:     "a multi-line comment keeps its closing line",
			input:    "<!--\nnote\n-->\n\nafter",
			expected: adfDoc(literalLines("<!--", "note", "-->"), para("after")),
		},
		{
			name:     "a pre block keeps an interior blank line and its closing line",
			input:    "<pre>\na\n\nb\n</pre>",
			expected: adfDoc(literalLines("<pre>", "a", "", "b", "</pre>")),
		},
		{
			name:     "a whitespace-only line keeps its whitespace",
			input:    "<pre>\n   \nx\n</pre>",
			expected: adfDoc(literalLines("<pre>", "   ", "x", "</pre>")),
		},
		{
			name:     "lines under a lone tag are kept uninterpreted",
			input:    "<word>\nmore **text**",
			expected: adfDoc(literalLines("<word>", "more **text**")),
		},
		{
			name:     "an unterminated comment keeps the rest of the document",
			input:    "<!--\nfoo\n\nbar",
			expected: adfDoc(literalLines("<!--", "foo", "", "bar")),
		},
		{
			name:  "a block inside a details body is kept",
			input: "<details><summary>T</summary>\n\n<word>\n\n</details>",
			expected: adfDoc(
				expandNode(t, "T", para("<word>")),
			),
		},
		{
			name:  "a multi-line block inside a list item is kept",
			input: "- item\n\n  <div>\n  inner\n  </div>",
			expected: adfDoc(jira4claude.ADFNode{
				Type: "bulletList",
				Content: []jira4claude.ADFNode{{
					Type:    "listItem",
					Content: []jira4claude.ADFNode{para("item"), literalLines("<div>", "inner", "</div>")},
				}},
			}),
		},
		{
			name:  "a block inside a blockquote is kept",
			input: "> <word>",
			expected: adfDoc(jira4claude.ADFNode{
				Type:    "blockquote",
				Content: []jira4claude.ADFNode{para("<word>")},
			}),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			result, issues := markdown.New().ToADF(tc.input)

			requireValidADF(t, result)
			assert.Equal(t, tc.expected, result)
			assert.Empty(t, issues)
		})
	}
}

// TestToADF_LiteralBlockHTMLRoundTrip asserts that literal block HTML read
// back from Jira and written again gives identical ADF.
func TestToADF_LiteralBlockHTMLRoundTrip(t *testing.T) {
	t.Parallel()

	inputs := map[string]string{
		"lone tag":               "<word>",
		"placeholder":            "Run it with\n\n<path>\n\nand wait.",
		"multi-line div":         "<div class=\"x\">\n  <p>Hello</p>\n  world\n</div>",
		"single-line comment":    "<!-- note -->",
		"multi-line comment":     "<!--\nnote\n-->\n\nafter",
		"pre with blank line":    "<pre>\na\n\nb\n</pre>",
		"inside details":         "<details><summary>T</summary>\n\n<div>\ninner\n</div>\n\n</details>",
		"inside list item":       "- item\n\n  <div>\n  inner\n  </div>",
		"inside blockquote":      "> <div>\n> inner\n> </div>",
		"whitespace line":        "<pre>\n   \nx\n</pre>",
		"unterminated comment":   "<!--\nfoo\n\nbar",
		"markdown syntax inside": "<div>\n# heading\n*star* `code` | pipe\n\\escaped\n</div>",
		"indented lines":         "   <div>\n x\n</div>",
		"lines under a lone tag": "<word>\nmore **text**",
	}

	for name, input := range inputs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			converter := markdown.New()
			first, issues := converter.ToADF(input)
			require.Empty(t, issues)

			md, mdIssues := converter.ToMarkdown(first)
			require.Empty(t, mdIssues)
			second, issues := converter.ToADF(md)

			assert.Empty(t, issues)
			assert.Equal(t, first, second, "markdown read back: %q", md)
		})
	}
}

// TestToADF_DetailsBlockHTMLStillRefused guards the <details> constructs that
// the literal-text fallback must not swallow: each still reports the HTML
// block as dropped, so a write refuses.
func TestToADF_DetailsBlockHTMLStillRefused(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"stray closing tag":                     "before\n\n</details>\n\nafter",
		"opener without a summary":              "<details>\n\nbody\n\n</details>",
		"opener with attributes":                "<details open><summary>T</summary>\n\nbody\n\n</details>",
		"opener in upper case":                  "<DETAILS><summary>T</summary>\n\nbody\n\n</DETAILS>",
		"opens and closes in one block":         "<details><summary>T</summary>\nbody\n</details>",
		"closing tag followed by text":          "<details><summary>T</summary>\n\nbody\n\n</details>\ntrailing",
		"unclosed opener":                       "<details><summary>T</summary>\n\nbody",
		"one-block details nested in a list":    "- item\n\n  <details><summary>T</summary>\n  body\n  </details>",
		"one-block details inside a blockquote": "> quote\n>\n> <details><summary>T</summary>\n> body\n> </details>",
	}

	for name, input := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			result, issues := markdown.New().ToADF(input)

			requireValidADF(t, result)
			assert.Contains(t, issues, droppedHTMLBlock())
		})
	}
}
