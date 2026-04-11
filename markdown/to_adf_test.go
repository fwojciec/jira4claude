package markdown_test

import (
	"encoding/json"
	"testing"

	jira4claude "github.com/fwojciec/jira4claude"
	"github.com/fwojciec/jira4claude/markdown"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConverter_ToADF(t *testing.T) {
	t.Parallel()

	t.Run("converts plain text to paragraph", func(t *testing.T) {
		t.Parallel()

		converter := markdown.New()
		result, warnings := converter.ToADF("Hello, world!")

		expected := &jira4claude.ADFNode{
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

		assert.Empty(t, warnings)
		assert.Equal(t, expected, result)
	})

	t.Run("converts bold text to strong mark", func(t *testing.T) {
		t.Parallel()

		converter := markdown.New()
		result, warnings := converter.ToADF("This is **bold** text.")

		expected := &jira4claude.ADFNode{
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

		assert.Empty(t, warnings)
		assert.Equal(t, expected, result)
	})

	t.Run("converts italic text to em mark", func(t *testing.T) {
		t.Parallel()

		converter := markdown.New()
		result, warnings := converter.ToADF("This is *italic* text.")

		expected := &jira4claude.ADFNode{
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

		assert.Empty(t, warnings)
		assert.Equal(t, expected, result)
	})

	t.Run("converts inline code to code mark", func(t *testing.T) {
		t.Parallel()

		converter := markdown.New()
		result, warnings := converter.ToADF("Use the `fmt.Println` function.")

		expected := &jira4claude.ADFNode{
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

		assert.Empty(t, warnings)
		assert.Equal(t, expected, result)
	})

	t.Run("converts fenced code block to codeBlock node", func(t *testing.T) {
		t.Parallel()

		converter := markdown.New()
		result, warnings := converter.ToADF("```go\nfmt.Println(\"hello\")\n```")

		expected := &jira4claude.ADFNode{
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

		assert.Empty(t, warnings)
		assert.Equal(t, expected, result)
	})

	t.Run("converts heading to heading node with level", func(t *testing.T) {
		t.Parallel()

		converter := markdown.New()
		result, warnings := converter.ToADF("# Heading 1\n\n## Heading 2")

		expected := &jira4claude.ADFNode{
			Type:    "doc",
			Version: 1,
			Content: []jira4claude.ADFNode{
				{
					Type:  "heading",
					Attrs: json.RawMessage(`{"level":1}`),
					Content: []jira4claude.ADFNode{
						{
							Type: "text",
							Text: "Heading 1",
						},
					},
				},
				{
					Type:  "heading",
					Attrs: json.RawMessage(`{"level":2}`),
					Content: []jira4claude.ADFNode{
						{
							Type: "text",
							Text: "Heading 2",
						},
					},
				},
			},
		}

		assert.Empty(t, warnings)
		assert.Equal(t, expected, result)
	})

	t.Run("converts bullet list to bulletList node", func(t *testing.T) {
		t.Parallel()

		converter := markdown.New()
		result, warnings := converter.ToADF("- Item 1\n- Item 2")

		expected := &jira4claude.ADFNode{
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
										{
											Type: "text",
											Text: "Item 1",
										},
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
										{
											Type: "text",
											Text: "Item 2",
										},
									},
								},
							},
						},
					},
				},
			},
		}

		assert.Empty(t, warnings)
		assert.Equal(t, expected, result)
	})

	t.Run("converts ordered list to orderedList node", func(t *testing.T) {
		t.Parallel()

		converter := markdown.New()
		result, warnings := converter.ToADF("1. First\n2. Second")

		expected := &jira4claude.ADFNode{
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
										{
											Type: "text",
											Text: "First",
										},
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
										{
											Type: "text",
											Text: "Second",
										},
									},
								},
							},
						},
					},
				},
			},
		}

		assert.Empty(t, warnings)
		assert.Equal(t, expected, result)
	})

	t.Run("converts link to link mark", func(t *testing.T) {
		t.Parallel()

		converter := markdown.New()
		result, warnings := converter.ToADF("Visit [Google](https://google.com) for more.")

		expected := &jira4claude.ADFNode{
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

		assert.Empty(t, warnings)
		assert.Equal(t, expected, result)
	})

	t.Run("converts blockquote to blockquote node", func(t *testing.T) {
		t.Parallel()

		converter := markdown.New()
		result, warnings := converter.ToADF("> This is a quote.")

		expected := &jira4claude.ADFNode{
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

		assert.Empty(t, warnings)
		assert.Equal(t, expected, result)
	})

	t.Run("handles combined formatting", func(t *testing.T) {
		t.Parallel()

		converter := markdown.New()
		result, warnings := converter.ToADF("This is ***bold and italic*** text.")

		expected := &jira4claude.ADFNode{
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

		assert.Empty(t, warnings)
		assert.Equal(t, expected, result)
	})

	t.Run("handles empty input", func(t *testing.T) {
		t.Parallel()

		converter := markdown.New()
		result, warnings := converter.ToADF("")

		expected := &jira4claude.ADFNode{
			Type:    "doc",
			Version: 1,
			Content: []jira4claude.ADFNode{},
		}

		assert.Empty(t, warnings)
		assert.Equal(t, expected, result)
	})

	t.Run("converts thematic break to rule node", func(t *testing.T) {
		t.Parallel()

		converter := markdown.New()
		result, warnings := converter.ToADF("Before\n\n---\n\nAfter")

		expected := &jira4claude.ADFNode{
			Type:    "doc",
			Version: 1,
			Content: []jira4claude.ADFNode{
				{
					Type: "paragraph",
					Content: []jira4claude.ADFNode{
						{Type: "text", Text: "Before"},
					},
				},
				{Type: "rule"},
				{
					Type: "paragraph",
					Content: []jira4claude.ADFNode{
						{Type: "text", Text: "After"},
					},
				},
			},
		}

		assert.Empty(t, warnings)
		assert.Equal(t, expected, result)
	})

	t.Run("accumulates multiple warnings for different skipped node types", func(t *testing.T) {
		t.Parallel()

		converter := markdown.New()
		// Multiple unsupported block elements: raw HTML blocks
		result, warnings := converter.ToADF("Start\n\n<div>html block one</div>\n\n<section>html block two</section>\n\nEnd")

		// Should still return converted content (best effort)
		require.NotNil(t, result)
		assert.Equal(t, "doc", result.Type)

		// Both HTML blocks produce the same skipped type, so only one warning
		require.Len(t, warnings, 1)
		assert.Contains(t, warnings[0], "HTMLBlock")
	})

	t.Run("returns empty warnings slice when no content is skipped", func(t *testing.T) {
		t.Parallel()

		converter := markdown.New()
		result, warnings := converter.ToADF("Hello")

		require.NotNil(t, result)
		assert.Empty(t, warnings)
	})

	// Tests for consolidateTextNodes via marksEqual and mapEqual.
	// These test text node behavior by observing the output structure.

	t.Run("produces single text node for unmarked text", func(t *testing.T) {
		t.Parallel()

		// Simple unmarked text produces a single text node with no marks field.
		// This establishes baseline behavior for comparison with marked text.
		converter := markdown.New()
		result, warnings := converter.ToADF("Hello world")

		expected := &jira4claude.ADFNode{
			Type:    "doc",
			Version: 1,
			Content: []jira4claude.ADFNode{
				{
					Type: "paragraph",
					Content: []jira4claude.ADFNode{
						{
							Type: "text",
							Text: "Hello world",
						},
					},
				},
			},
		}

		assert.Empty(t, warnings)
		assert.Equal(t, expected, result)
	})

	t.Run("does not consolidate text nodes with different marks", func(t *testing.T) {
		t.Parallel()

		// Text with bold followed by text without bold should remain separate
		converter := markdown.New()
		result, warnings := converter.ToADF("**bold**plain")

		require.NotNil(t, result)
		assert.Empty(t, warnings)

		require.Len(t, result.Content, 1)

		paragraph := result.Content[0]
		// Should have 2 separate text nodes: "bold" with mark, "plain" without
		assert.Len(t, paragraph.Content, 2)
	})

	t.Run("produces link mark with nested attrs map", func(t *testing.T) {
		t.Parallel()

		// Links produce marks with nested attrs maps (href).
		// This tests the mapEqual code path for nested map comparison.
		converter := markdown.New()
		result, warnings := converter.ToADF("[click here](https://example.com)")

		expected := &jira4claude.ADFNode{
			Type:    "doc",
			Version: 1,
			Content: []jira4claude.ADFNode{
				{
					Type: "paragraph",
					Content: []jira4claude.ADFNode{
						{
							Type: "text",
							Text: "click here",
							Marks: []jira4claude.ADFMark{
								{
									Type:  "link",
									Attrs: json.RawMessage(`{"href":"https://example.com"}`),
								},
							},
						},
					},
				},
			},
		}

		assert.Empty(t, warnings)
		assert.Equal(t, expected, result)
	})

	t.Run("does not consolidate text nodes with different link hrefs", func(t *testing.T) {
		t.Parallel()

		// Two adjacent links with different hrefs should remain separate
		converter := markdown.New()
		result, warnings := converter.ToADF("[one](https://one.com)[two](https://two.com)")

		require.NotNil(t, result)
		assert.Empty(t, warnings)

		require.Len(t, result.Content, 1)

		paragraph := result.Content[0]
		// Should have 2 separate text nodes with different links
		assert.Len(t, paragraph.Content, 2)
	})

	t.Run("does not consolidate when one has marks and other does not", func(t *testing.T) {
		t.Parallel()

		// Text without marks followed by text with marks should remain separate
		converter := markdown.New()
		result, warnings := converter.ToADF("plain**bold**")

		require.NotNil(t, result)
		assert.Empty(t, warnings)

		require.Len(t, result.Content, 1)

		paragraph := result.Content[0]
		// Should have 2 separate text nodes: "plain" without mark, "bold" with mark
		assert.Len(t, paragraph.Content, 2)
	})

	t.Run("does not consolidate when marks have different lengths", func(t *testing.T) {
		t.Parallel()

		// Bold text vs bold+italic text should remain separate (different mark counts)
		converter := markdown.New()
		result, warnings := converter.ToADF("**just bold*****bold and italic***")

		require.NotNil(t, result)
		assert.Empty(t, warnings)

		require.Len(t, result.Content, 1)

		paragraph := result.Content[0]
		// Should have 2 separate text nodes with different mark counts
		assert.Len(t, paragraph.Content, 2)
	})

	t.Run("converts bare URL autolink to link mark", func(t *testing.T) {
		t.Parallel()

		converter := markdown.New()
		result, warnings := converter.ToADF("Visit https://example.com for details.")

		expected := &jira4claude.ADFNode{
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
							Text: "https://example.com",
							Marks: []jira4claude.ADFMark{
								{
									Type:  "link",
									Attrs: json.RawMessage(`{"href":"https://example.com"}`),
								},
							},
						},
						{
							Type: "text",
							Text: " for details.",
						},
					},
				},
			},
		}

		assert.Empty(t, warnings)
		assert.Equal(t, expected, result)
	})

	t.Run("converts angle bracket autolink to link mark", func(t *testing.T) {
		t.Parallel()

		converter := markdown.New()
		result, warnings := converter.ToADF("Check <https://example.com> now.")

		expected := &jira4claude.ADFNode{
			Type:    "doc",
			Version: 1,
			Content: []jira4claude.ADFNode{
				{
					Type: "paragraph",
					Content: []jira4claude.ADFNode{
						{
							Type: "text",
							Text: "Check ",
						},
						{
							Type: "text",
							Text: "https://example.com",
							Marks: []jira4claude.ADFMark{
								{
									Type:  "link",
									Attrs: json.RawMessage(`{"href":"https://example.com"}`),
								},
							},
						},
						{
							Type: "text",
							Text: " now.",
						},
					},
				},
			},
		}

		assert.Empty(t, warnings)
		assert.Equal(t, expected, result)
	})

	t.Run("normalizes Unicode symbols to spaces in ADF output", func(t *testing.T) {
		t.Parallel()

		converter := markdown.New()
		result, warnings := converter.ToADF("Use this → see results — important")

		expected := &jira4claude.ADFNode{
			Type:    "doc",
			Version: 1,
			Content: []jira4claude.ADFNode{
				{
					Type: "paragraph",
					Content: []jira4claude.ADFNode{
						{
							Type: "text",
							Text: "Use this   see results   important",
						},
					},
				},
			},
		}

		assert.Empty(t, warnings)
		assert.Equal(t, expected, result)
	})

	t.Run("converts hard line break to hardBreak node", func(t *testing.T) {
		t.Parallel()

		converter := markdown.New()
		// Two trailing spaces before newline create a hard line break in markdown
		result, warnings := converter.ToADF("Line one  \nLine two")

		expected := &jira4claude.ADFNode{
			Type:    "doc",
			Version: 1,
			Content: []jira4claude.ADFNode{
				{
					Type: "paragraph",
					Content: []jira4claude.ADFNode{
						{Type: "text", Text: "Line one"},
						{Type: "hardBreak"},
						{Type: "text", Text: "Line two"},
					},
				},
			},
		}

		assert.Empty(t, warnings)
		assert.Equal(t, expected, result)
	})

	t.Run("converts strikethrough to strike mark", func(t *testing.T) {
		t.Parallel()

		converter := markdown.New()
		result, warnings := converter.ToADF("This is ~~deleted~~ text.")

		expected := &jira4claude.ADFNode{
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

		assert.Empty(t, warnings)
		assert.Equal(t, expected, result)
	})

	t.Run("converts strikethrough with other marks", func(t *testing.T) {
		t.Parallel()

		converter := markdown.New()
		result, warnings := converter.ToADF("**~~bold and deleted~~**")

		expected := &jira4claude.ADFNode{
			Type:    "doc",
			Version: 1,
			Content: []jira4claude.ADFNode{
				{
					Type: "paragraph",
					Content: []jira4claude.ADFNode{
						{
							Type: "text",
							Text: "bold and deleted",
							Marks: []jira4claude.ADFMark{
								{Type: "strong"},
								{Type: "strike"},
							},
						},
					},
				},
			},
		}

		assert.Empty(t, warnings)
		assert.Equal(t, expected, result)
	})

	t.Run("converts GFM table to ADF table", func(t *testing.T) {
		t.Parallel()

		converter := markdown.New()
		result, warnings := converter.ToADF("| Name | Age |\n| --- | --- |\n| Alice | 30 |")

		expected := &jira4claude.ADFNode{
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
												{Type: "text", Text: "Age"},
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
												{Type: "text", Text: "Alice"},
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
												{Type: "text", Text: "30"},
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

		assert.Empty(t, warnings)
		assert.Equal(t, expected, result)
	})

	t.Run("converts GFM table with multiple data rows", func(t *testing.T) {
		t.Parallel()

		converter := markdown.New()
		result, warnings := converter.ToADF("| H1 | H2 |\n| --- | --- |\n| a | b |\n| c | d |")

		require.NotNil(t, result)
		assert.Empty(t, warnings)
		require.Len(t, result.Content, 1)

		table := result.Content[0]
		assert.Equal(t, "table", table.Type)
		// 1 header row + 2 data rows = 3 tableRow nodes
		require.Len(t, table.Content, 3)

		// Header row
		assert.Equal(t, "tableRow", table.Content[0].Type)
		assert.Equal(t, "tableHeader", table.Content[0].Content[0].Type)

		// Data rows
		assert.Equal(t, "tableRow", table.Content[1].Type)
		assert.Equal(t, "tableCell", table.Content[1].Content[0].Type)
		assert.Equal(t, "tableRow", table.Content[2].Type)
		assert.Equal(t, "tableCell", table.Content[2].Content[0].Type)
	})

	t.Run("converts table with inline formatting in cells", func(t *testing.T) {
		t.Parallel()

		converter := markdown.New()
		result, warnings := converter.ToADF("| Feature | Status |\n| --- | --- |\n| **Bold** | *done* |")

		require.NotNil(t, result)
		assert.Empty(t, warnings)

		table := result.Content[0]
		// Data row, first cell should have bold text
		dataRow := table.Content[1]
		cell := dataRow.Content[0]
		assert.Equal(t, "tableCell", cell.Type)
		require.Len(t, cell.Content, 1)
		para := cell.Content[0]
		require.Len(t, para.Content, 1)
		assert.Equal(t, "Bold", para.Content[0].Text)
		require.Len(t, para.Content[0].Marks, 1)
		assert.Equal(t, "strong", para.Content[0].Marks[0].Type)
	})

	t.Run("converts table alongside other blocks", func(t *testing.T) {
		t.Parallel()

		converter := markdown.New()
		result, warnings := converter.ToADF("Before\n\n| A | B |\n| --- | --- |\n| 1 | 2 |\n\nAfter")

		require.NotNil(t, result)
		assert.Empty(t, warnings)
		// paragraph, table, paragraph
		require.Len(t, result.Content, 3)
		assert.Equal(t, "paragraph", result.Content[0].Type)
		assert.Equal(t, "table", result.Content[1].Type)
		assert.Equal(t, "paragraph", result.Content[2].Type)
	})

	t.Run("converts task list with unchecked items to taskList", func(t *testing.T) {
		t.Parallel()

		converter := markdown.New()
		result, warnings := converter.ToADF("- [ ] Buy milk\n- [ ] Walk dog")

		require.NotNil(t, result)
		assert.Empty(t, warnings)
		require.Len(t, result.Content, 1)

		taskList := result.Content[0]
		assert.Equal(t, "taskList", taskList.Type)
		require.Len(t, taskList.Content, 2)

		// First task item
		item0 := taskList.Content[0]
		assert.Equal(t, "taskItem", item0.Type)
		require.NotNil(t, item0.Attrs)
		var attrs0 map[string]any
		require.NoError(t, json.Unmarshal(item0.Attrs, &attrs0))
		assert.Equal(t, "TODO", attrs0["state"])
		assert.NotEmpty(t, attrs0["localId"])
		// Content should be paragraph with text
		require.Len(t, item0.Content, 1)
		assert.Equal(t, "paragraph", item0.Content[0].Type)
		require.Len(t, item0.Content[0].Content, 1)
		assert.Equal(t, "Buy milk", item0.Content[0].Content[0].Text)

		// Second task item
		item1 := taskList.Content[1]
		assert.Equal(t, "taskItem", item1.Type)
		var attrs1 map[string]any
		require.NoError(t, json.Unmarshal(item1.Attrs, &attrs1))
		assert.Equal(t, "TODO", attrs1["state"])
	})

	t.Run("converts task list with checked items to DONE state", func(t *testing.T) {
		t.Parallel()

		converter := markdown.New()
		result, warnings := converter.ToADF("- [x] Done task\n- [ ] Pending task")

		require.NotNil(t, result)
		assert.Empty(t, warnings)
		require.Len(t, result.Content, 1)

		taskList := result.Content[0]
		assert.Equal(t, "taskList", taskList.Type)
		require.Len(t, taskList.Content, 2)

		// First item - checked
		var attrs0 map[string]any
		require.NoError(t, json.Unmarshal(taskList.Content[0].Attrs, &attrs0))
		assert.Equal(t, "DONE", attrs0["state"])

		// Second item - unchecked
		var attrs1 map[string]any
		require.NoError(t, json.Unmarshal(taskList.Content[1].Attrs, &attrs1))
		assert.Equal(t, "TODO", attrs1["state"])
	})

	t.Run("generates unique localIds for each task item", func(t *testing.T) {
		t.Parallel()

		converter := markdown.New()
		result, _ := converter.ToADF("- [ ] One\n- [ ] Two\n- [ ] Three")

		require.Len(t, result.Content, 1)
		taskList := result.Content[0]
		require.Len(t, taskList.Content, 3)

		ids := make(map[string]struct{})
		for _, item := range taskList.Content {
			var attrs map[string]any
			require.NoError(t, json.Unmarshal(item.Attrs, &attrs))
			id, ok := attrs["localId"].(string)
			require.True(t, ok)
			assert.NotEmpty(t, id)
			ids[id] = struct{}{}
		}
		// All IDs should be unique
		assert.Len(t, ids, 3)
	})

	t.Run("regular list without checkboxes stays as bulletList", func(t *testing.T) {
		t.Parallel()

		converter := markdown.New()
		result, warnings := converter.ToADF("- Regular item\n- Another item")

		require.NotNil(t, result)
		assert.Empty(t, warnings)
		require.Len(t, result.Content, 1)
		assert.Equal(t, "bulletList", result.Content[0].Type)
	})
}
