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
		// ADF with an unsupported node type (e.g., "bodiedExtension")
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
					Type:    "bodiedExtension",
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
		assert.Contains(t, warnings[0], "bodiedExtension")
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
					Type:    "layoutSection",
					Content: []jira4claude.ADFNode{},
				},
				{
					Type:    "bodiedExtension",
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
		assert.Contains(t, warnings[0], "bodiedExtension")
		assert.Contains(t, warnings[1], "layoutSection")
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

	t.Run("converts strike mark to strikethrough", func(t *testing.T) {
		t.Parallel()

		converter := markdown.New()
		adfDoc := &jira4claude.ADFNode{
			Type:    "doc",
			Version: 1,
			Content: []jira4claude.ADFNode{
				{
					Type: "paragraph",
					Content: []jira4claude.ADFNode{
						{Type: "text", Text: "This is "},
						{
							Type: "text",
							Text: "deleted",
							Marks: []jira4claude.ADFMark{
								{Type: "strike"},
							},
						},
						{Type: "text", Text: " text."},
					},
				},
			},
		}

		result, warnings := converter.ToMarkdown(adfDoc)

		assert.Empty(t, warnings)
		assert.Equal(t, "This is ~~deleted~~ text.", result)
	})

	t.Run("converts strike with strong to combined markdown", func(t *testing.T) {
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
							Text: "bold deleted",
							Marks: []jira4claude.ADFMark{
								{Type: "strong"},
								{Type: "strike"},
							},
						},
					},
				},
			},
		}

		result, warnings := converter.ToMarkdown(adfDoc)

		assert.Empty(t, warnings)
		assert.Equal(t, "~~**bold deleted**~~", result)
	})

	t.Run("round trips strikethrough through MD to ADF to MD", func(t *testing.T) {
		t.Parallel()

		converter := markdown.New()
		original := "This is ~~strikethrough~~ text."

		adf, w1 := converter.ToADF(original)
		result, w2 := converter.ToMarkdown(adf)

		assert.Empty(t, w1)
		assert.Empty(t, w2)
		assert.Equal(t, original, result)
	})

	t.Run("converts panel with info type to NOTE alert blockquote", func(t *testing.T) {
		t.Parallel()

		converter := markdown.New()
		adfDoc := &jira4claude.ADFNode{
			Type:    "doc",
			Version: 1,
			Content: []jira4claude.ADFNode{
				{
					Type:  "panel",
					Attrs: json.RawMessage(`{"panelType":"info"}`),
					Content: []jira4claude.ADFNode{
						{
							Type: "paragraph",
							Content: []jira4claude.ADFNode{
								{Type: "text", Text: "Panel content"},
							},
						},
					},
				},
			},
		}

		result, warnings := converter.ToMarkdown(adfDoc)

		assert.Empty(t, warnings)
		assert.Equal(t, "> [!NOTE]\n> Panel content", result)
	})

	t.Run("converts all five panel types to correct alert syntax", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			panelType string
			alert     string
		}{
			{"info", "NOTE"},
			{"warning", "WARNING"},
			{"error", "CAUTION"},
			{"success", "TIP"},
			{"note", "IMPORTANT"},
		}

		converter := markdown.New()
		for _, tc := range cases {
			t.Run(tc.panelType, func(t *testing.T) {
				t.Parallel()

				adfDoc := &jira4claude.ADFNode{
					Type:    "doc",
					Version: 1,
					Content: []jira4claude.ADFNode{
						{
							Type:  "panel",
							Attrs: json.RawMessage(`{"panelType":"` + tc.panelType + `"}`),
							Content: []jira4claude.ADFNode{
								{
									Type: "paragraph",
									Content: []jira4claude.ADFNode{
										{Type: "text", Text: "Content"},
									},
								},
							},
						},
					},
				}

				result, warnings := converter.ToMarkdown(adfDoc)

				assert.Empty(t, warnings)
				assert.Equal(t, "> [!"+tc.alert+"]\n> Content", result)
			})
		}
	})

	t.Run("converts panel with multiple paragraphs", func(t *testing.T) {
		t.Parallel()

		converter := markdown.New()
		adfDoc := &jira4claude.ADFNode{
			Type:    "doc",
			Version: 1,
			Content: []jira4claude.ADFNode{
				{
					Type:  "panel",
					Attrs: json.RawMessage(`{"panelType":"warning"}`),
					Content: []jira4claude.ADFNode{
						{
							Type: "paragraph",
							Content: []jira4claude.ADFNode{
								{Type: "text", Text: "First paragraph"},
							},
						},
						{
							Type: "paragraph",
							Content: []jira4claude.ADFNode{
								{Type: "text", Text: "Second paragraph"},
							},
						},
					},
				},
			},
		}

		result, warnings := converter.ToMarkdown(adfDoc)

		assert.Empty(t, warnings)
		assert.Equal(t, "> [!WARNING]\n> First paragraph\n>\n> Second paragraph", result)
	})

	t.Run("converts expand to details HTML", func(t *testing.T) {
		t.Parallel()

		converter := markdown.New()
		adfDoc := &jira4claude.ADFNode{
			Type:    "doc",
			Version: 1,
			Content: []jira4claude.ADFNode{
				{
					Type:  "expand",
					Attrs: json.RawMessage(`{"title":"Click to expand"}`),
					Content: []jira4claude.ADFNode{
						{
							Type: "paragraph",
							Content: []jira4claude.ADFNode{
								{Type: "text", Text: "Expanded content here."},
							},
						},
					},
				},
			},
		}

		result, warnings := converter.ToMarkdown(adfDoc)

		assert.Empty(t, warnings)
		assert.Equal(t, "<details><summary>Click to expand</summary>\n\nExpanded content here.\n\n</details>", result)
	})

	t.Run("converts expand with multiline body", func(t *testing.T) {
		t.Parallel()

		converter := markdown.New()
		adfDoc := &jira4claude.ADFNode{
			Type:    "doc",
			Version: 1,
			Content: []jira4claude.ADFNode{
				{
					Type:  "expand",
					Attrs: json.RawMessage(`{"title":"Details"}`),
					Content: []jira4claude.ADFNode{
						{
							Type: "paragraph",
							Content: []jira4claude.ADFNode{
								{Type: "text", Text: "First paragraph."},
							},
						},
						{
							Type: "bulletList",
							Content: []jira4claude.ADFNode{
								{
									Type: "listItem",
									Content: []jira4claude.ADFNode{
										{Type: "paragraph", Content: []jira4claude.ADFNode{{Type: "text", Text: "Item 1"}}},
									},
								},
								{
									Type: "listItem",
									Content: []jira4claude.ADFNode{
										{Type: "paragraph", Content: []jira4claude.ADFNode{{Type: "text", Text: "Item 2"}}},
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
		assert.Equal(t, "<details><summary>Details</summary>\n\nFirst paragraph.\n\n- Item 1\n- Item 2\n\n</details>", result)
	})

	t.Run("converts nestedExpand same as expand", func(t *testing.T) {
		t.Parallel()

		converter := markdown.New()
		adfDoc := &jira4claude.ADFNode{
			Type:    "doc",
			Version: 1,
			Content: []jira4claude.ADFNode{
				{
					Type:  "nestedExpand",
					Attrs: json.RawMessage(`{"title":"Nested"}`),
					Content: []jira4claude.ADFNode{
						{
							Type: "paragraph",
							Content: []jira4claude.ADFNode{
								{Type: "text", Text: "Nested content."},
							},
						},
					},
				},
			},
		}

		result, warnings := converter.ToMarkdown(adfDoc)

		assert.Empty(t, warnings)
		assert.Equal(t, "<details><summary>Nested</summary>\n\nNested content.\n\n</details>", result)
	})

	t.Run("converts expand without title attr", func(t *testing.T) {
		t.Parallel()

		converter := markdown.New()
		adfDoc := &jira4claude.ADFNode{
			Type:    "doc",
			Version: 1,
			Content: []jira4claude.ADFNode{
				{
					Type: "expand",
					Content: []jira4claude.ADFNode{
						{
							Type: "paragraph",
							Content: []jira4claude.ADFNode{
								{Type: "text", Text: "No title content."},
							},
						},
					},
				},
			},
		}

		result, warnings := converter.ToMarkdown(adfDoc)

		assert.Empty(t, warnings)
		assert.Equal(t, "<details><summary></summary>\n\nNo title content.\n\n</details>", result)
	})

	t.Run("converts mediaSingle with URL-carrying media to image", func(t *testing.T) {
		t.Parallel()

		converter := markdown.New()
		urlAttrs, _ := json.Marshal(map[string]any{
			"id":         "abc-123",
			"type":       "file",
			"collection": "coll-1",
			"url":        "https://example.com/image.png",
			"alt":        "screenshot",
		})
		layoutAttrs, _ := json.Marshal(map[string]any{
			"layout": "center",
		})
		adfDoc := &jira4claude.ADFNode{
			Type:    "doc",
			Version: 1,
			Content: []jira4claude.ADFNode{
				{
					Type:  "mediaSingle",
					Attrs: layoutAttrs,
					Content: []jira4claude.ADFNode{
						{
							Type:  "media",
							Attrs: urlAttrs,
						},
					},
				},
			},
		}

		result, warnings := converter.ToMarkdown(adfDoc)

		assert.Empty(t, warnings)
		assert.Equal(t, "![screenshot](https://example.com/image.png)", result)
	})

	t.Run("converts mediaSingle without URL to fallback text with filename", func(t *testing.T) {
		t.Parallel()

		converter := markdown.New()
		attrs, _ := json.Marshal(map[string]any{
			"id":         "abc-123",
			"type":       "file",
			"collection": "coll-1",
		})
		adfDoc := &jira4claude.ADFNode{
			Type:    "doc",
			Version: 1,
			Content: []jira4claude.ADFNode{
				{
					Type: "mediaSingle",
					Content: []jira4claude.ADFNode{
						{
							Type:  "media",
							Attrs: attrs,
						},
					},
				},
			},
		}

		result, warnings := converter.ToMarkdown(adfDoc)

		assert.Empty(t, warnings)
		assert.Equal(t, "[image]", result)
	})

	t.Run("converts mediaSingle without URL but with filename to fallback text", func(t *testing.T) {
		t.Parallel()

		converter := markdown.New()
		attrs, _ := json.Marshal(map[string]any{
			"id":         "abc-123",
			"type":       "file",
			"collection": "coll-1",
			"__fileName": "diagram.png",
		})
		adfDoc := &jira4claude.ADFNode{
			Type:    "doc",
			Version: 1,
			Content: []jira4claude.ADFNode{
				{
					Type: "mediaSingle",
					Content: []jira4claude.ADFNode{
						{
							Type:  "media",
							Attrs: attrs,
						},
					},
				},
			},
		}

		result, warnings := converter.ToMarkdown(adfDoc)

		assert.Empty(t, warnings)
		assert.Equal(t, "[image: diagram.png]", result)
	})

	t.Run("converts mediaGroup with multiple media items", func(t *testing.T) {
		t.Parallel()

		converter := markdown.New()
		urlAttrs, _ := json.Marshal(map[string]any{
			"id":   "abc-1",
			"type": "file",
			"url":  "https://example.com/a.png",
			"alt":  "first",
		})
		noURLAttrs, _ := json.Marshal(map[string]any{
			"id":         "abc-2",
			"type":       "file",
			"__fileName": "second.png",
		})
		adfDoc := &jira4claude.ADFNode{
			Type:    "doc",
			Version: 1,
			Content: []jira4claude.ADFNode{
				{
					Type: "mediaGroup",
					Content: []jira4claude.ADFNode{
						{Type: "media", Attrs: urlAttrs},
						{Type: "media", Attrs: noURLAttrs},
					},
				},
			},
		}

		result, warnings := converter.ToMarkdown(adfDoc)

		assert.Empty(t, warnings)
		assert.Equal(t, "![first](https://example.com/a.png)\n[image: second.png]", result)
	})

	t.Run("converts mediaInline with filename to inline text", func(t *testing.T) {
		t.Parallel()

		converter := markdown.New()
		attrs, _ := json.Marshal(map[string]any{
			"id":         "abc-123",
			"type":       "file",
			"collection": "coll-1",
			"__fileName": "report.pdf",
		})
		adfDoc := &jira4claude.ADFNode{
			Type:    "doc",
			Version: 1,
			Content: []jira4claude.ADFNode{
				{
					Type: "paragraph",
					Content: []jira4claude.ADFNode{
						{Type: "text", Text: "See "},
						{Type: "mediaInline", Attrs: attrs},
						{Type: "text", Text: " for details."},
					},
				},
			},
		}

		result, warnings := converter.ToMarkdown(adfDoc)

		assert.Empty(t, warnings)
		assert.Equal(t, "See [report.pdf] for details.", result)
	})

	t.Run("converts mediaInline without filename to attachment fallback", func(t *testing.T) {
		t.Parallel()

		converter := markdown.New()
		attrs, _ := json.Marshal(map[string]any{
			"id":   "abc-123",
			"type": "file",
		})
		adfDoc := &jira4claude.ADFNode{
			Type:    "doc",
			Version: 1,
			Content: []jira4claude.ADFNode{
				{
					Type: "paragraph",
					Content: []jira4claude.ADFNode{
						{Type: "text", Text: "See "},
						{Type: "mediaInline", Attrs: attrs},
						{Type: "text", Text: " attached."},
					},
				},
			},
		}

		result, warnings := converter.ToMarkdown(adfDoc)

		assert.Empty(t, warnings)
		assert.Equal(t, "See [attachment] attached.", result)
	})

	t.Run("converts mention with display name", func(t *testing.T) {
		t.Parallel()

		converter := markdown.New()
		adfDoc := &jira4claude.ADFNode{
			Type:    "doc",
			Version: 1,
			Content: []jira4claude.ADFNode{
				{
					Type: "paragraph",
					Content: []jira4claude.ADFNode{
						{Type: "text", Text: "Assigned to "},
						{
							Type:  "mention",
							Attrs: json.RawMessage(`{"id":"abc123","text":"John Smith"}`),
						},
						{Type: "text", Text: " for review."},
					},
				},
			},
		}

		result, warnings := converter.ToMarkdown(adfDoc)

		assert.Empty(t, warnings)
		assert.Equal(t, "Assigned to @John Smith for review.", result)
	})

	t.Run("converts mention with @ prefix in text avoids double @", func(t *testing.T) {
		t.Parallel()

		converter := markdown.New()
		adfDoc := &jira4claude.ADFNode{
			Type:    "doc",
			Version: 1,
			Content: []jira4claude.ADFNode{
				{
					Type: "paragraph",
					Content: []jira4claude.ADFNode{
						{Type: "text", Text: "Ask "},
						{
							Type:  "mention",
							Attrs: json.RawMessage(`{"id":"abc123","text":"@Bradley Ayers"}`),
						},
					},
				},
			},
		}

		result, warnings := converter.ToMarkdown(adfDoc)

		assert.Empty(t, warnings)
		assert.Equal(t, "Ask @Bradley Ayers", result)
	})

	t.Run("converts mention without display name falls back to id", func(t *testing.T) {
		t.Parallel()

		converter := markdown.New()
		adfDoc := &jira4claude.ADFNode{
			Type:    "doc",
			Version: 1,
			Content: []jira4claude.ADFNode{
				{
					Type: "paragraph",
					Content: []jira4claude.ADFNode{
						{Type: "text", Text: "Ask "},
						{
							Type:  "mention",
							Attrs: json.RawMessage(`{"id":"user-456"}`),
						},
						{Type: "text", Text: " about it."},
					},
				},
			},
		}

		result, warnings := converter.ToMarkdown(adfDoc)

		assert.Empty(t, warnings)
		assert.Equal(t, "Ask @user-456 about it.", result)
	})

	t.Run("converts emoji with unicode text", func(t *testing.T) {
		t.Parallel()

		converter := markdown.New()
		adfDoc := &jira4claude.ADFNode{
			Type:    "doc",
			Version: 1,
			Content: []jira4claude.ADFNode{
				{
					Type: "paragraph",
					Content: []jira4claude.ADFNode{
						{Type: "text", Text: "Great work "},
						{
							Type:  "emoji",
							Attrs: json.RawMessage(`{"shortName":":thumbsup:","id":"1f44d","text":"👍"}`),
						},
					},
				},
			},
		}

		result, warnings := converter.ToMarkdown(adfDoc)

		assert.Empty(t, warnings)
		assert.Equal(t, "Great work 👍", result)
	})

	t.Run("converts emoji without unicode falls back to shortName", func(t *testing.T) {
		t.Parallel()

		converter := markdown.New()
		adfDoc := &jira4claude.ADFNode{
			Type:    "doc",
			Version: 1,
			Content: []jira4claude.ADFNode{
				{
					Type: "paragraph",
					Content: []jira4claude.ADFNode{
						{Type: "text", Text: "Custom "},
						{
							Type:  "emoji",
							Attrs: json.RawMessage(`{"shortName":":atlassian:","id":"custom-1"}`),
						},
					},
				},
			},
		}

		result, warnings := converter.ToMarkdown(adfDoc)

		assert.Empty(t, warnings)
		assert.Equal(t, "Custom :atlassian:", result)
	})

	t.Run("converts inlineCard to markdown link", func(t *testing.T) {
		t.Parallel()

		converter := markdown.New()
		adfDoc := &jira4claude.ADFNode{
			Type:    "doc",
			Version: 1,
			Content: []jira4claude.ADFNode{
				{
					Type: "paragraph",
					Content: []jira4claude.ADFNode{
						{Type: "text", Text: "See "},
						{
							Type:  "inlineCard",
							Attrs: json.RawMessage(`{"url":"https://jira.example.com/browse/PROJ-123"}`),
						},
						{Type: "text", Text: " for details."},
					},
				},
			},
		}

		result, warnings := converter.ToMarkdown(adfDoc)

		assert.Empty(t, warnings)
		assert.Equal(t, "See [https://jira.example.com/browse/PROJ-123](https://jira.example.com/browse/PROJ-123) for details.", result)
	})

	t.Run("converts status to bold text", func(t *testing.T) {
		t.Parallel()

		converter := markdown.New()
		adfDoc := &jira4claude.ADFNode{
			Type:    "doc",
			Version: 1,
			Content: []jira4claude.ADFNode{
				{
					Type: "paragraph",
					Content: []jira4claude.ADFNode{
						{Type: "text", Text: "Status: "},
						{
							Type:  "status",
							Attrs: json.RawMessage(`{"text":"IN PROGRESS","color":"blue"}`),
						},
					},
				},
			},
		}

		result, warnings := converter.ToMarkdown(adfDoc)

		assert.Empty(t, warnings)
		assert.Equal(t, "Status: **IN PROGRESS**", result)
	})

	t.Run("converts date to human-readable format", func(t *testing.T) {
		t.Parallel()

		converter := markdown.New()
		adfDoc := &jira4claude.ADFNode{
			Type:    "doc",
			Version: 1,
			Content: []jira4claude.ADFNode{
				{
					Type: "paragraph",
					Content: []jira4claude.ADFNode{
						{Type: "text", Text: "Due: "},
						{
							Type:  "date",
							Attrs: json.RawMessage(`{"timestamp":"1712707200000"}`),
						},
					},
				},
			},
		}

		result, warnings := converter.ToMarkdown(adfDoc)

		assert.Empty(t, warnings)
		assert.Equal(t, "Due: 2024-04-10", result)
	})

	t.Run("drops placeholder silently", func(t *testing.T) {
		t.Parallel()

		converter := markdown.New()
		adfDoc := &jira4claude.ADFNode{
			Type:    "doc",
			Version: 1,
			Content: []jira4claude.ADFNode{
				{
					Type: "paragraph",
					Content: []jira4claude.ADFNode{
						{Type: "text", Text: "Enter text here"},
						{
							Type:  "placeholder",
							Attrs: json.RawMessage(`{"text":"Type something here"}`),
						},
						{Type: "text", Text: " please."},
					},
				},
			},
		}

		result, warnings := converter.ToMarkdown(adfDoc)

		assert.Empty(t, warnings)
		assert.Equal(t, "Enter text here please.", result)
	})

	t.Run("drops inlineExtension with warning", func(t *testing.T) {
		t.Parallel()

		converter := markdown.New()
		adfDoc := &jira4claude.ADFNode{
			Type:    "doc",
			Version: 1,
			Content: []jira4claude.ADFNode{
				{
					Type: "paragraph",
					Content: []jira4claude.ADFNode{
						{Type: "text", Text: "Content: "},
						{
							Type:  "inlineExtension",
							Attrs: json.RawMessage(`{"extensionType":"com.atlassian.macro","extensionKey":"jira"}`),
						},
						{Type: "text", Text: " end."},
					},
				},
			},
		}

		result, warnings := converter.ToMarkdown(adfDoc)

		assert.Equal(t, "Content:  end.", result)
		require.Len(t, warnings, 1)
		assert.Contains(t, warnings[0], "inlineExtension")
	})

	t.Run("handles unknown inline node with warning", func(t *testing.T) {
		t.Parallel()

		converter := markdown.New()
		adfDoc := &jira4claude.ADFNode{
			Type:    "doc",
			Version: 1,
			Content: []jira4claude.ADFNode{
				{
					Type: "paragraph",
					Content: []jira4claude.ADFNode{
						{Type: "text", Text: "Before "},
						{Type: "futureInlineNode"},
						{Type: "text", Text: " after."},
					},
				},
			},
		}

		result, warnings := converter.ToMarkdown(adfDoc)

		assert.Equal(t, "Before  after.", result)
		require.Len(t, warnings, 1)
		assert.Contains(t, warnings[0], "futureInlineNode")
	})

	t.Run("converts underline mark to u tag", func(t *testing.T) {
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
							Text: "underlined",
							Marks: []jira4claude.ADFMark{
								{Type: "underline"},
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
		assert.Equal(t, "This is <u>underlined</u> text.", result)
	})

	t.Run("converts subsup mark with type sub", func(t *testing.T) {
		t.Parallel()

		converter := markdown.New()
		subsupAttrs, _ := json.Marshal(map[string]any{"type": "sub"})
		adfDoc := &jira4claude.ADFNode{
			Type:    "doc",
			Version: 1,
			Content: []jira4claude.ADFNode{
				{
					Type: "paragraph",
					Content: []jira4claude.ADFNode{
						{
							Type: "text",
							Text: "H",
						},
						{
							Type:  "text",
							Text:  "2",
							Marks: []jira4claude.ADFMark{{Type: "subsup", Attrs: subsupAttrs}},
						},
						{
							Type: "text",
							Text: "O",
						},
					},
				},
			},
		}

		result, warnings := converter.ToMarkdown(adfDoc)

		assert.Empty(t, warnings)
		assert.Equal(t, "H<sub>2</sub>O", result)
	})

	t.Run("converts subsup mark with type sup", func(t *testing.T) {
		t.Parallel()

		converter := markdown.New()
		subsupAttrs, _ := json.Marshal(map[string]any{"type": "sup"})
		adfDoc := &jira4claude.ADFNode{
			Type:    "doc",
			Version: 1,
			Content: []jira4claude.ADFNode{
				{
					Type: "paragraph",
					Content: []jira4claude.ADFNode{
						{
							Type: "text",
							Text: "x",
						},
						{
							Type:  "text",
							Text:  "2",
							Marks: []jira4claude.ADFMark{{Type: "subsup", Attrs: subsupAttrs}},
						},
					},
				},
			},
		}

		result, warnings := converter.ToMarkdown(adfDoc)

		assert.Empty(t, warnings)
		assert.Equal(t, "x<sup>2</sup>", result)
	})
}
