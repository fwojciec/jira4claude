package markdown_test

import (
	"encoding/json"
	"testing"

	jira4claude "github.com/fwojciec/jira4claude"
	"github.com/fwojciec/jira4claude/markdown"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConverter_ToMarkdown(t *testing.T) {
	t.Parallel()

	t.Run("converts simple paragraph to plain text", func(t *testing.T) {
		t.Parallel()

		converter := markdown.New()
		adfDoc := &jira4claude.ADFNode{
			Type:    "doc",
			Version: 1,
			Content: []jira4claude.ADFNode{
				{
					Type: "paragraph",
					Content: []jira4claude.ADFNode{
						{
							Type: "text",
							Text: "Hello, world!",
						},
					},
				},
			},
		}

		result, warnings := converter.ToMarkdown(adfDoc)

		assert.Empty(t, warnings)
		assert.Equal(t, "Hello, world!", result)
	})

	t.Run("converts strong mark to bold", func(t *testing.T) {
		t.Parallel()

		converter := markdown.New()
		adfDoc := &jira4claude.ADFNode{
			Type:    "doc",
			Version: 1,
			Content: []jira4claude.ADFNode{
				{
					Type: "paragraph",
					Content: []jira4claude.ADFNode{
						{
							Type: "text",
							Text: "This is ",
						},
						{
							Type: "text",
							Text: "bold",
							Marks: []jira4claude.ADFMark{
								{Type: "strong"},
							},
						},
						{
							Type: "text",
							Text: " text.",
						},
					},
				},
			},
		}

		result, warnings := converter.ToMarkdown(adfDoc)

		assert.Empty(t, warnings)
		assert.Equal(t, "This is **bold** text.", result)
	})

	t.Run("converts em mark to italic", func(t *testing.T) {
		t.Parallel()

		converter := markdown.New()
		adfDoc := &jira4claude.ADFNode{
			Type:    "doc",
			Version: 1,
			Content: []jira4claude.ADFNode{
				{
					Type: "paragraph",
					Content: []jira4claude.ADFNode{
						{
							Type: "text",
							Text: "This is ",
						},
						{
							Type: "text",
							Text: "italic",
							Marks: []jira4claude.ADFMark{
								{Type: "em"},
							},
						},
						{
							Type: "text",
							Text: " text.",
						},
					},
				},
			},
		}

		result, warnings := converter.ToMarkdown(adfDoc)

		assert.Empty(t, warnings)
		assert.Equal(t, "This is *italic* text.", result)
	})

	t.Run("converts code mark to inline code", func(t *testing.T) {
		t.Parallel()

		converter := markdown.New()
		adfDoc := &jira4claude.ADFNode{
			Type:    "doc",
			Version: 1,
			Content: []jira4claude.ADFNode{
				{
					Type: "paragraph",
					Content: []jira4claude.ADFNode{
						{
							Type: "text",
							Text: "Use the ",
						},
						{
							Type: "text",
							Text: "fmt.Println",
							Marks: []jira4claude.ADFMark{
								{Type: "code"},
							},
						},
						{
							Type: "text",
							Text: " function.",
						},
					},
				},
			},
		}

		result, warnings := converter.ToMarkdown(adfDoc)

		assert.Empty(t, warnings)
		assert.Equal(t, "Use the `fmt.Println` function.", result)
	})

	t.Run("converts codeBlock to fenced code block", func(t *testing.T) {
		t.Parallel()

		converter := markdown.New()
		adfDoc := &jira4claude.ADFNode{
			Type:    "doc",
			Version: 1,
			Content: []jira4claude.ADFNode{
				{
					Type:  "codeBlock",
					Attrs: json.RawMessage(`{"language":"go"}`),
					Content: []jira4claude.ADFNode{
						{
							Type: "text",
							Text: "fmt.Println(\"hello\")",
						},
					},
				},
			},
		}

		result, warnings := converter.ToMarkdown(adfDoc)

		assert.Empty(t, warnings)
		assert.Equal(t, "```go\nfmt.Println(\"hello\")\n```", result)
	})

	t.Run("converts heading to markdown heading", func(t *testing.T) {
		t.Parallel()

		converter := markdown.New()
		adfDoc := &jira4claude.ADFNode{
			Type:    "doc",
			Version: 1,
			Content: []jira4claude.ADFNode{
				{
					Type:  "heading",
					Attrs: json.RawMessage(`{"level":2}`),
					Content: []jira4claude.ADFNode{
						{
							Type: "text",
							Text: "My Heading",
						},
					},
				},
			},
		}

		result, warnings := converter.ToMarkdown(adfDoc)

		assert.Empty(t, warnings)
		assert.Equal(t, "## My Heading", result)
	})

	t.Run("converts bulletList to markdown list", func(t *testing.T) {
		t.Parallel()

		converter := markdown.New()
		adfDoc := &jira4claude.ADFNode{
			Type:    "doc",
			Version: 1,
			Content: []jira4claude.ADFNode{
				{
					Type: "bulletList",
					Content: []jira4claude.ADFNode{
						{
							Type: "listItem",
							Content: []jira4claude.ADFNode{
								{
									Type: "paragraph",
									Content: []jira4claude.ADFNode{
										{Type: "text", Text: "Item 1"},
									},
								},
							},
						},
						{
							Type: "listItem",
							Content: []jira4claude.ADFNode{
								{
									Type: "paragraph",
									Content: []jira4claude.ADFNode{
										{Type: "text", Text: "Item 2"},
									},
								},
							},
						},
					},
				},
			},
		}

		result, warnings := converter.ToMarkdown(adfDoc)

		assert.Empty(t, warnings)
		assert.Equal(t, "- Item 1\n- Item 2", result)
	})

	t.Run("converts orderedList to markdown list", func(t *testing.T) {
		t.Parallel()

		converter := markdown.New()
		adfDoc := &jira4claude.ADFNode{
			Type:    "doc",
			Version: 1,
			Content: []jira4claude.ADFNode{
				{
					Type: "orderedList",
					Content: []jira4claude.ADFNode{
						{
							Type: "listItem",
							Content: []jira4claude.ADFNode{
								{
									Type: "paragraph",
									Content: []jira4claude.ADFNode{
										{Type: "text", Text: "First"},
									},
								},
							},
						},
						{
							Type: "listItem",
							Content: []jira4claude.ADFNode{
								{
									Type: "paragraph",
									Content: []jira4claude.ADFNode{
										{Type: "text", Text: "Second"},
									},
								},
							},
						},
					},
				},
			},
		}

		result, warnings := converter.ToMarkdown(adfDoc)

		assert.Empty(t, warnings)
		assert.Equal(t, "1. First\n2. Second", result)
	})

	t.Run("converts link mark to markdown link", func(t *testing.T) {
		t.Parallel()

		converter := markdown.New()
		adfDoc := &jira4claude.ADFNode{
			Type:    "doc",
			Version: 1,
			Content: []jira4claude.ADFNode{
				{
					Type: "paragraph",
					Content: []jira4claude.ADFNode{
						{
							Type: "text",
							Text: "Visit ",
						},
						{
							Type: "text",
							Text: "Google",
							Marks: []jira4claude.ADFMark{
								{
									Type:  "link",
									Attrs: json.RawMessage(`{"href":"https://google.com"}`),
								},
							},
						},
						{
							Type: "text",
							Text: " for more.",
						},
					},
				},
			},
		}

		result, warnings := converter.ToMarkdown(adfDoc)

		assert.Empty(t, warnings)
		assert.Equal(t, "Visit [Google](https://google.com) for more.", result)
	})

	t.Run("converts blockquote to markdown blockquote", func(t *testing.T) {
		t.Parallel()

		converter := markdown.New()
		adfDoc := &jira4claude.ADFNode{
			Type:    "doc",
			Version: 1,
			Content: []jira4claude.ADFNode{
				{
					Type: "blockquote",
					Content: []jira4claude.ADFNode{
						{
							Type: "paragraph",
							Content: []jira4claude.ADFNode{
								{
									Type: "text",
									Text: "This is a quote.",
								},
							},
						},
					},
				},
			},
		}

		result, warnings := converter.ToMarkdown(adfDoc)

		assert.Empty(t, warnings)
		assert.Equal(t, "> This is a quote.", result)
	})

	t.Run("handles multiple paragraphs", func(t *testing.T) {
		t.Parallel()

		converter := markdown.New()
		adfDoc := &jira4claude.ADFNode{
			Type:    "doc",
			Version: 1,
			Content: []jira4claude.ADFNode{
				{
					Type: "paragraph",
					Content: []jira4claude.ADFNode{
						{
							Type: "text",
							Text: "First paragraph.",
						},
					},
				},
				{
					Type: "paragraph",
					Content: []jira4claude.ADFNode{
						{
							Type: "text",
							Text: "Second paragraph.",
						},
					},
				},
			},
		}

		result, warnings := converter.ToMarkdown(adfDoc)

		assert.Empty(t, warnings)
		assert.Equal(t, "First paragraph.\n\nSecond paragraph.", result)
	})

	t.Run("handles combined formatting (bold and italic)", func(t *testing.T) {
		t.Parallel()

		converter := markdown.New()
		adfDoc := &jira4claude.ADFNode{
			Type:    "doc",
			Version: 1,
			Content: []jira4claude.ADFNode{
				{
					Type: "paragraph",
					Content: []jira4claude.ADFNode{
						{
							Type: "text",
							Text: "This is ",
						},
						{
							Type: "text",
							Text: "bold and italic",
							Marks: []jira4claude.ADFMark{
								{Type: "em"},
								{Type: "strong"},
							},
						},
						{
							Type: "text",
							Text: " text.",
						},
					},
				},
			},
		}

		result, warnings := converter.ToMarkdown(adfDoc)

		assert.Empty(t, warnings)
		assert.Equal(t, "This is ***bold and italic*** text.", result)
	})

	t.Run("handles nil input", func(t *testing.T) {
		t.Parallel()

		converter := markdown.New()
		result, warnings := converter.ToMarkdown(nil)

		assert.Empty(t, warnings)
		assert.Empty(t, result)
	})

	t.Run("handles empty document", func(t *testing.T) {
		t.Parallel()

		converter := markdown.New()
		adfDoc := &jira4claude.ADFNode{
			Type:    "doc",
			Version: 1,
			Content: []jira4claude.ADFNode{},
		}

		result, warnings := converter.ToMarkdown(adfDoc)

		assert.Empty(t, warnings)
		assert.Empty(t, result)
	})

	t.Run("returns warning when content is skipped", func(t *testing.T) {
		t.Parallel()

		converter := markdown.New()
		// ADF with an unsupported node type (e.g., "panel")
		adfDoc := &jira4claude.ADFNode{
			Type:    "doc",
			Version: 1,
			Content: []jira4claude.ADFNode{
				{
					Type: "paragraph",
					Content: []jira4claude.ADFNode{
						{
							Type: "text",
							Text: "Before",
						},
					},
				},
				{
					Type:    "panel",
					Content: []jira4claude.ADFNode{},
				},
				{
					Type: "paragraph",
					Content: []jira4claude.ADFNode{
						{
							Type: "text",
							Text: "After",
						},
					},
				},
			},
		}

		result, warnings := converter.ToMarkdown(adfDoc)

		// Should still return converted content (best effort)
		assert.Equal(t, "Before\n\nAfter", result)

		// Should return warning listing skipped content
		require.Len(t, warnings, 1)
		assert.Contains(t, warnings[0], "panel")
	})

	t.Run("accumulates multiple warnings for different skipped node types", func(t *testing.T) {
		t.Parallel()

		converter := markdown.New()
		// ADF with multiple unsupported node types
		adfDoc := &jira4claude.ADFNode{
			Type:    "doc",
			Version: 1,
			Content: []jira4claude.ADFNode{
				{
					Type: "paragraph",
					Content: []jira4claude.ADFNode{
						{
							Type: "text",
							Text: "Start",
						},
					},
				},
				{
					Type:    "panel",
					Content: []jira4claude.ADFNode{},
				},
				{
					Type:    "expand",
					Content: []jira4claude.ADFNode{},
				},
				{
					Type: "paragraph",
					Content: []jira4claude.ADFNode{
						{
							Type: "text",
							Text: "End",
						},
					},
				},
			},
		}

		result, warnings := converter.ToMarkdown(adfDoc)

		// Should still return converted content (best effort)
		assert.Equal(t, "Start\n\nEnd", result)

		// Should return individual warnings for each skipped node type, sorted alphabetically
		require.Len(t, warnings, 2)
		assert.Contains(t, warnings[0], "expand")
		assert.Contains(t, warnings[1], "panel")
	})

	t.Run("returns empty warnings slice when no content is skipped", func(t *testing.T) {
		t.Parallel()

		converter := markdown.New()
		adfDoc := &jira4claude.ADFNode{
			Type:    "doc",
			Version: 1,
			Content: []jira4claude.ADFNode{
				{
					Type: "paragraph",
					Content: []jira4claude.ADFNode{
						{
							Type: "text",
							Text: "Hello",
						},
					},
				},
			},
		}

		result, warnings := converter.ToMarkdown(adfDoc)

		assert.Equal(t, "Hello", result)
		assert.Empty(t, warnings)
	})

	t.Run("handles heading level as float64 from JSON unmarshaling", func(t *testing.T) {
		t.Parallel()

		// With typed structs, JSON unmarshaling produces proper int values via
		// json.RawMessage. This test verifies heading level 3 works correctly.
		converter := markdown.New()
		adfDoc := &jira4claude.ADFNode{
			Type:    "doc",
			Version: 1,
			Content: []jira4claude.ADFNode{
				{
					Type:  "heading",
					Attrs: json.RawMessage(`{"level":3}`),
					Content: []jira4claude.ADFNode{
						{
							Type: "text",
							Text: "Level 3 Heading",
						},
					},
				},
			},
		}

		result, warnings := converter.ToMarkdown(adfDoc)

		assert.Empty(t, warnings)
		assert.Equal(t, "### Level 3 Heading", result)
	})

	t.Run("defaults heading level to 1 when attrs missing", func(t *testing.T) {
		t.Parallel()

		// When attrs are missing entirely, should default to level 1.
		converter := markdown.New()
		adfDoc := &jira4claude.ADFNode{
			Type:    "doc",
			Version: 1,
			Content: []jira4claude.ADFNode{
				{
					Type: "heading",
					// No attrs - level should default to 1
					Content: []jira4claude.ADFNode{
						{
							Type: "text",
							Text: "Default Heading",
						},
					},
				},
			},
		}

		result, warnings := converter.ToMarkdown(adfDoc)

		assert.Empty(t, warnings)
		assert.Equal(t, "# Default Heading", result)
	})

	t.Run("converts rule to horizontal rule", func(t *testing.T) {
		t.Parallel()

		converter := markdown.New()
		adfDoc := &jira4claude.ADFNode{
			Type:    "doc",
			Version: 1,
			Content: []jira4claude.ADFNode{
				{
					Type: "paragraph",
					Content: []jira4claude.ADFNode{
						{
							Type: "text",
							Text: "Above",
						},
					},
				},
				{
					Type: "rule",
				},
				{
					Type: "paragraph",
					Content: []jira4claude.ADFNode{
						{
							Type: "text",
							Text: "Below",
						},
					},
				},
			},
		}

		result, warnings := converter.ToMarkdown(adfDoc)

		assert.Empty(t, warnings)
		assert.Equal(t, "Above\n\n---\n\nBelow", result)
	})

	t.Run("converts table to GFM table", func(t *testing.T) {
		t.Parallel()

		converter := markdown.New()
		adfDoc := &jira4claude.ADFNode{
			Type:    "doc",
			Version: 1,
			Content: []jira4claude.ADFNode{
				{
					Type: "table",
					Content: []jira4claude.ADFNode{
						{
							Type: "tableRow",
							Content: []jira4claude.ADFNode{
								{
									Type: "tableHeader",
									Content: []jira4claude.ADFNode{
										{
											Type: "paragraph",
											Content: []jira4claude.ADFNode{
												{Type: "text", Text: "Name"},
											},
										},
									},
								},
								{
									Type: "tableHeader",
									Content: []jira4claude.ADFNode{
										{
											Type: "paragraph",
											Content: []jira4claude.ADFNode{
												{Type: "text", Text: "Value"},
											},
										},
									},
								},
							},
						},
						{
							Type: "tableRow",
							Content: []jira4claude.ADFNode{
								{
									Type: "tableCell",
									Content: []jira4claude.ADFNode{
										{
											Type: "paragraph",
											Content: []jira4claude.ADFNode{
												{Type: "text", Text: "foo"},
											},
										},
									},
								},
								{
									Type: "tableCell",
									Content: []jira4claude.ADFNode{
										{
											Type: "paragraph",
											Content: []jira4claude.ADFNode{
												{Type: "text", Text: "bar"},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		}

		result, warnings := converter.ToMarkdown(adfDoc)

		assert.Empty(t, warnings)
		assert.Equal(t, "| Name | Value |\n| --- | --- |\n| foo | bar |", result)
	})

	t.Run("converts table without headers", func(t *testing.T) {
		t.Parallel()

		converter := markdown.New()
		adfDoc := &jira4claude.ADFNode{
			Type:    "doc",
			Version: 1,
			Content: []jira4claude.ADFNode{
				{
					Type: "table",
					Content: []jira4claude.ADFNode{
						{
							Type: "tableRow",
							Content: []jira4claude.ADFNode{
								{
									Type: "tableCell",
									Content: []jira4claude.ADFNode{
										{
											Type: "paragraph",
											Content: []jira4claude.ADFNode{
												{Type: "text", Text: "a"},
											},
										},
									},
								},
								{
									Type: "tableCell",
									Content: []jira4claude.ADFNode{
										{
											Type: "paragraph",
											Content: []jira4claude.ADFNode{
												{Type: "text", Text: "b"},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		}

		result, warnings := converter.ToMarkdown(adfDoc)

		assert.Empty(t, warnings)
		// Should synthesize empty header row + separator for valid GFM
		assert.Equal(t, "|  |  |\n| --- | --- |\n| a | b |", result)
	})

	t.Run("escapes pipe characters in table cells", func(t *testing.T) {
		t.Parallel()

		converter := markdown.New()
		adfDoc := &jira4claude.ADFNode{
			Type:    "doc",
			Version: 1,
			Content: []jira4claude.ADFNode{
				{
					Type: "table",
					Content: []jira4claude.ADFNode{
						{
							Type: "tableRow",
							Content: []jira4claude.ADFNode{
								{
									Type: "tableHeader",
									Content: []jira4claude.ADFNode{
										{
											Type: "paragraph",
											Content: []jira4claude.ADFNode{
												{Type: "text", Text: "Col"},
											},
										},
									},
								},
							},
						},
						{
							Type: "tableRow",
							Content: []jira4claude.ADFNode{
								{
									Type: "tableCell",
									Content: []jira4claude.ADFNode{
										{
											Type: "paragraph",
											Content: []jira4claude.ADFNode{
												{Type: "text", Text: "foo | bar"},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		}

		result, warnings := converter.ToMarkdown(adfDoc)

		assert.Empty(t, warnings)
		assert.Equal(t, "| Col |\n| --- |\n| foo \\| bar |", result)
	})

	t.Run("converts taskList to GFM task list", func(t *testing.T) {
		t.Parallel()

		converter := markdown.New()
		adfDoc := &jira4claude.ADFNode{
			Type:    "doc",
			Version: 1,
			Content: []jira4claude.ADFNode{
				{
					Type: "taskList",
					Content: []jira4claude.ADFNode{
						{
							Type:  "taskItem",
							Attrs: json.RawMessage(`{"state":"TODO"}`),
							Content: []jira4claude.ADFNode{
								{
									Type: "paragraph",
									Content: []jira4claude.ADFNode{
										{Type: "text", Text: "Buy milk"},
									},
								},
							},
						},
						{
							Type:  "taskItem",
							Attrs: json.RawMessage(`{"state":"DONE"}`),
							Content: []jira4claude.ADFNode{
								{
									Type: "paragraph",
									Content: []jira4claude.ADFNode{
										{Type: "text", Text: "Write code"},
									},
								},
							},
						},
					},
				},
			},
		}

		result, warnings := converter.ToMarkdown(adfDoc)

		assert.Empty(t, warnings)
		assert.Equal(t, "- [ ] Buy milk\n- [x] Write code", result)
	})

	t.Run("normalizes newlines in table cells", func(t *testing.T) {
		t.Parallel()

		converter := markdown.New()
		adfDoc := &jira4claude.ADFNode{
			Type:    "doc",
			Version: 1,
			Content: []jira4claude.ADFNode{
				{
					Type: "table",
					Content: []jira4claude.ADFNode{
						{
							Type: "tableRow",
							Content: []jira4claude.ADFNode{
								{
									Type: "tableHeader",
									Content: []jira4claude.ADFNode{
										{
											Type: "paragraph",
											Content: []jira4claude.ADFNode{
												{Type: "text", Text: "Col"},
											},
										},
									},
								},
							},
						},
						{
							Type: "tableRow",
							Content: []jira4claude.ADFNode{
								{
									Type: "tableCell",
									Content: []jira4claude.ADFNode{
										{
											Type: "paragraph",
											Content: []jira4claude.ADFNode{
												{Type: "text", Text: "line1"},
												{Type: "hardBreak"},
												{Type: "text", Text: "line2"},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		}

		result, warnings := converter.ToMarkdown(adfDoc)

		assert.Empty(t, warnings)
		// Newlines in cell content should be replaced with spaces to preserve table structure
		assert.Equal(t, "| Col |\n| --- |\n| line1 line2 |", result)
	})

	t.Run("handles table with empty content gracefully", func(t *testing.T) {
		t.Parallel()

		converter := markdown.New()
		adfDoc := &jira4claude.ADFNode{
			Type:    "doc",
			Version: 1,
			Content: []jira4claude.ADFNode{
				{
					Type:    "table",
					Content: []jira4claude.ADFNode{},
				},
			},
		}

		result, warnings := converter.ToMarkdown(adfDoc)

		assert.Empty(t, warnings)
		assert.Empty(t, result)
	})

	t.Run("defaults taskItem without attrs to unchecked", func(t *testing.T) {
		t.Parallel()

		converter := markdown.New()
		adfDoc := &jira4claude.ADFNode{
			Type:    "doc",
			Version: 1,
			Content: []jira4claude.ADFNode{
				{
					Type: "taskList",
					Content: []jira4claude.ADFNode{
						{
							Type: "taskItem",
							// No attrs at all
							Content: []jira4claude.ADFNode{
								{
									Type: "paragraph",
									Content: []jira4claude.ADFNode{
										{Type: "text", Text: "No state"},
									},
								},
							},
						},
					},
				},
			},
		}

		result, warnings := converter.ToMarkdown(adfDoc)

		assert.Empty(t, warnings)
		assert.Equal(t, "- [ ] No state", result)
	})

	t.Run("defaults heading level to 1 when level attr missing", func(t *testing.T) {
		t.Parallel()

		// When attrs exist but level is missing, should default to level 1.
		converter := markdown.New()
		adfDoc := &jira4claude.ADFNode{
			Type:    "doc",
			Version: 1,
			Content: []jira4claude.ADFNode{
				{
					Type:  "heading",
					Attrs: json.RawMessage(`{}`),
					Content: []jira4claude.ADFNode{
						{
							Type: "text",
							Text: "Default Heading",
						},
					},
				},
			},
		}

		result, warnings := converter.ToMarkdown(adfDoc)

		assert.Empty(t, warnings)
		assert.Equal(t, "# Default Heading", result)
	})

	t.Run("handles text node at block level without warning", func(t *testing.T) {
		t.Parallel()

		converter := markdown.New()
		// Jira sometimes returns bare "text" nodes at the block level
		// (e.g., inside a listItem without a wrapping paragraph).
		// These should still be rendered as text content in the surrounding
		// structure and should not produce a warning.
		adfDoc := &jira4claude.ADFNode{
			Type:    "doc",
			Version: 1,
			Content: []jira4claude.ADFNode{
				{
					Type: "bulletList",
					Content: []jira4claude.ADFNode{
						{
							Type: "listItem",
							Content: []jira4claude.ADFNode{
								{
									Type: "text",
									Text: "bare text node",
								},
							},
						},
					},
				},
			},
		}

		result, warnings := converter.ToMarkdown(adfDoc)

		assert.Empty(t, warnings)
		assert.Equal(t, "- bare text node", result)
	})

	t.Run("handles text node at block level with marks", func(t *testing.T) {
		t.Parallel()

		converter := markdown.New()
		adfDoc := &jira4claude.ADFNode{
			Type:    "doc",
			Version: 1,
			Content: []jira4claude.ADFNode{
				{
					Type: "bulletList",
					Content: []jira4claude.ADFNode{
						{
							Type: "listItem",
							Content: []jira4claude.ADFNode{
								{
									Type: "text",
									Text: "bold text",
									Marks: []jira4claude.ADFMark{
										{Type: "strong"},
									},
								},
							},
						},
					},
				},
			},
		}

		result, warnings := converter.ToMarkdown(adfDoc)

		assert.Empty(t, warnings)
		assert.Equal(t, "- **bold text**", result)
	})
}
