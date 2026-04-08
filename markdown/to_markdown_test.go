package markdown_test

import (
	"testing"

	"github.com/fwojciec/jira4claude/markdown"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConverter_ToMarkdown(t *testing.T) {
	t.Parallel()

	t.Run("converts simple paragraph to plain text", func(t *testing.T) {
		t.Parallel()

		converter := markdown.New()
		adfDoc := map[string]any{
			"type":    "doc",
			"version": 1,
			"content": []any{
				map[string]any{
					"type": "paragraph",
					"content": []any{
						map[string]any{
							"type": "text",
							"text": "Hello, world!",
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
		adfDoc := map[string]any{
			"type":    "doc",
			"version": 1,
			"content": []any{
				map[string]any{
					"type": "paragraph",
					"content": []any{
						map[string]any{
							"type": "text",
							"text": "This is ",
						},
						map[string]any{
							"type": "text",
							"text": "bold",
							"marks": []any{
								map[string]any{
									"type": "strong",
								},
							},
						},
						map[string]any{
							"type": "text",
							"text": " text.",
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
		adfDoc := map[string]any{
			"type":    "doc",
			"version": 1,
			"content": []any{
				map[string]any{
					"type": "paragraph",
					"content": []any{
						map[string]any{
							"type": "text",
							"text": "This is ",
						},
						map[string]any{
							"type": "text",
							"text": "italic",
							"marks": []any{
								map[string]any{
									"type": "em",
								},
							},
						},
						map[string]any{
							"type": "text",
							"text": " text.",
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
		adfDoc := map[string]any{
			"type":    "doc",
			"version": 1,
			"content": []any{
				map[string]any{
					"type": "paragraph",
					"content": []any{
						map[string]any{
							"type": "text",
							"text": "Use the ",
						},
						map[string]any{
							"type": "text",
							"text": "fmt.Println",
							"marks": []any{
								map[string]any{
									"type": "code",
								},
							},
						},
						map[string]any{
							"type": "text",
							"text": " function.",
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
		adfDoc := map[string]any{
			"type":    "doc",
			"version": 1,
			"content": []any{
				map[string]any{
					"type": "codeBlock",
					"attrs": map[string]any{
						"language": "go",
					},
					"content": []any{
						map[string]any{
							"type": "text",
							"text": "fmt.Println(\"hello\")",
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
		adfDoc := map[string]any{
			"type":    "doc",
			"version": 1,
			"content": []any{
				map[string]any{
					"type": "heading",
					"attrs": map[string]any{
						"level": 2,
					},
					"content": []any{
						map[string]any{
							"type": "text",
							"text": "My Heading",
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
		adfDoc := map[string]any{
			"type":    "doc",
			"version": 1,
			"content": []any{
				map[string]any{
					"type": "bulletList",
					"content": []any{
						map[string]any{
							"type": "listItem",
							"content": []any{
								map[string]any{
									"type": "paragraph",
									"content": []any{
										map[string]any{
											"type": "text",
											"text": "Item 1",
										},
									},
								},
							},
						},
						map[string]any{
							"type": "listItem",
							"content": []any{
								map[string]any{
									"type": "paragraph",
									"content": []any{
										map[string]any{
											"type": "text",
											"text": "Item 2",
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
		assert.Equal(t, "- Item 1\n- Item 2", result)
	})

	t.Run("converts orderedList to markdown list", func(t *testing.T) {
		t.Parallel()

		converter := markdown.New()
		adfDoc := map[string]any{
			"type":    "doc",
			"version": 1,
			"content": []any{
				map[string]any{
					"type": "orderedList",
					"content": []any{
						map[string]any{
							"type": "listItem",
							"content": []any{
								map[string]any{
									"type": "paragraph",
									"content": []any{
										map[string]any{
											"type": "text",
											"text": "First",
										},
									},
								},
							},
						},
						map[string]any{
							"type": "listItem",
							"content": []any{
								map[string]any{
									"type": "paragraph",
									"content": []any{
										map[string]any{
											"type": "text",
											"text": "Second",
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
		assert.Equal(t, "1. First\n2. Second", result)
	})

	t.Run("converts link mark to markdown link", func(t *testing.T) {
		t.Parallel()

		converter := markdown.New()
		adfDoc := map[string]any{
			"type":    "doc",
			"version": 1,
			"content": []any{
				map[string]any{
					"type": "paragraph",
					"content": []any{
						map[string]any{
							"type": "text",
							"text": "Visit ",
						},
						map[string]any{
							"type": "text",
							"text": "Google",
							"marks": []any{
								map[string]any{
									"type": "link",
									"attrs": map[string]any{
										"href": "https://google.com",
									},
								},
							},
						},
						map[string]any{
							"type": "text",
							"text": " for more.",
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
		adfDoc := map[string]any{
			"type":    "doc",
			"version": 1,
			"content": []any{
				map[string]any{
					"type": "blockquote",
					"content": []any{
						map[string]any{
							"type": "paragraph",
							"content": []any{
								map[string]any{
									"type": "text",
									"text": "This is a quote.",
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
		adfDoc := map[string]any{
			"type":    "doc",
			"version": 1,
			"content": []any{
				map[string]any{
					"type": "paragraph",
					"content": []any{
						map[string]any{
							"type": "text",
							"text": "First paragraph.",
						},
					},
				},
				map[string]any{
					"type": "paragraph",
					"content": []any{
						map[string]any{
							"type": "text",
							"text": "Second paragraph.",
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
		adfDoc := map[string]any{
			"type":    "doc",
			"version": 1,
			"content": []any{
				map[string]any{
					"type": "paragraph",
					"content": []any{
						map[string]any{
							"type": "text",
							"text": "This is ",
						},
						map[string]any{
							"type": "text",
							"text": "bold and italic",
							"marks": []any{
								map[string]any{
									"type": "em",
								},
								map[string]any{
									"type": "strong",
								},
							},
						},
						map[string]any{
							"type": "text",
							"text": " text.",
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
		adfDoc := map[string]any{
			"type":    "doc",
			"version": 1,
			"content": []any{},
		}

		result, warnings := converter.ToMarkdown(adfDoc)

		assert.Empty(t, warnings)
		assert.Empty(t, result)
	})

	t.Run("returns warning when content is skipped", func(t *testing.T) {
		t.Parallel()

		converter := markdown.New()
		// ADF with an unsupported node type (e.g., "panel")
		adfDoc := map[string]any{
			"type":    "doc",
			"version": 1,
			"content": []any{
				map[string]any{
					"type": "paragraph",
					"content": []any{
						map[string]any{
							"type": "text",
							"text": "Before",
						},
					},
				},
				map[string]any{
					"type":    "panel",
					"content": []any{},
				},
				map[string]any{
					"type": "paragraph",
					"content": []any{
						map[string]any{
							"type": "text",
							"text": "After",
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
		adfDoc := map[string]any{
			"type":    "doc",
			"version": 1,
			"content": []any{
				map[string]any{
					"type": "paragraph",
					"content": []any{
						map[string]any{
							"type": "text",
							"text": "Start",
						},
					},
				},
				map[string]any{
					"type":    "panel",
					"content": []any{},
				},
				map[string]any{
					"type":    "expand",
					"content": []any{},
				},
				map[string]any{
					"type": "paragraph",
					"content": []any{
						map[string]any{
							"type": "text",
							"text": "End",
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
		adfDoc := map[string]any{
			"type":    "doc",
			"version": 1,
			"content": []any{
				map[string]any{
					"type": "paragraph",
					"content": []any{
						map[string]any{
							"type": "text",
							"text": "Hello",
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

		// When ADF is unmarshaled from JSON, numbers become float64.
		// This test verifies the float64 code path in adfHeadingToGFM.
		converter := markdown.New()
		adfDoc := map[string]any{
			"type":    "doc",
			"version": 1,
			"content": []any{
				map[string]any{
					"type": "heading",
					"attrs": map[string]any{
						"level": float64(3), // Simulates JSON unmarshaling
					},
					"content": []any{
						map[string]any{
							"type": "text",
							"text": "Level 3 Heading",
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
		adfDoc := map[string]any{
			"type":    "doc",
			"version": 1,
			"content": []any{
				map[string]any{
					"type": "heading",
					// No attrs - level should default to 1
					"content": []any{
						map[string]any{
							"type": "text",
							"text": "Default Heading",
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
		adfDoc := map[string]any{
			"type":    "doc",
			"version": 1,
			"content": []any{
				map[string]any{
					"type": "paragraph",
					"content": []any{
						map[string]any{
							"type": "text",
							"text": "Above",
						},
					},
				},
				map[string]any{
					"type": "rule",
				},
				map[string]any{
					"type": "paragraph",
					"content": []any{
						map[string]any{
							"type": "text",
							"text": "Below",
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
		adfDoc := map[string]any{
			"type":    "doc",
			"version": 1,
			"content": []any{
				map[string]any{
					"type": "table",
					"content": []any{
						map[string]any{
							"type": "tableRow",
							"content": []any{
								map[string]any{
									"type": "tableHeader",
									"content": []any{
										map[string]any{
											"type": "paragraph",
											"content": []any{
												map[string]any{"type": "text", "text": "Name"},
											},
										},
									},
								},
								map[string]any{
									"type": "tableHeader",
									"content": []any{
										map[string]any{
											"type": "paragraph",
											"content": []any{
												map[string]any{"type": "text", "text": "Value"},
											},
										},
									},
								},
							},
						},
						map[string]any{
							"type": "tableRow",
							"content": []any{
								map[string]any{
									"type": "tableCell",
									"content": []any{
										map[string]any{
											"type": "paragraph",
											"content": []any{
												map[string]any{"type": "text", "text": "foo"},
											},
										},
									},
								},
								map[string]any{
									"type": "tableCell",
									"content": []any{
										map[string]any{
											"type": "paragraph",
											"content": []any{
												map[string]any{"type": "text", "text": "bar"},
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
		adfDoc := map[string]any{
			"type":    "doc",
			"version": 1,
			"content": []any{
				map[string]any{
					"type": "table",
					"content": []any{
						map[string]any{
							"type": "tableRow",
							"content": []any{
								map[string]any{
									"type": "tableCell",
									"content": []any{
										map[string]any{
											"type": "paragraph",
											"content": []any{
												map[string]any{"type": "text", "text": "a"},
											},
										},
									},
								},
								map[string]any{
									"type": "tableCell",
									"content": []any{
										map[string]any{
											"type": "paragraph",
											"content": []any{
												map[string]any{"type": "text", "text": "b"},
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
		adfDoc := map[string]any{
			"type":    "doc",
			"version": 1,
			"content": []any{
				map[string]any{
					"type": "table",
					"content": []any{
						map[string]any{
							"type": "tableRow",
							"content": []any{
								map[string]any{
									"type": "tableHeader",
									"content": []any{
										map[string]any{
											"type": "paragraph",
											"content": []any{
												map[string]any{"type": "text", "text": "Col"},
											},
										},
									},
								},
							},
						},
						map[string]any{
							"type": "tableRow",
							"content": []any{
								map[string]any{
									"type": "tableCell",
									"content": []any{
										map[string]any{
											"type": "paragraph",
											"content": []any{
												map[string]any{"type": "text", "text": "foo | bar"},
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
		adfDoc := map[string]any{
			"type":    "doc",
			"version": 1,
			"content": []any{
				map[string]any{
					"type": "taskList",
					"content": []any{
						map[string]any{
							"type": "taskItem",
							"attrs": map[string]any{
								"state": "TODO",
							},
							"content": []any{
								map[string]any{
									"type": "paragraph",
									"content": []any{
										map[string]any{"type": "text", "text": "Buy milk"},
									},
								},
							},
						},
						map[string]any{
							"type": "taskItem",
							"attrs": map[string]any{
								"state": "DONE",
							},
							"content": []any{
								map[string]any{
									"type": "paragraph",
									"content": []any{
										map[string]any{"type": "text", "text": "Write code"},
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

	t.Run("converts taskItem with paragraph content", func(t *testing.T) {
		t.Parallel()

		// Real Jira ADF wraps taskItem text in a paragraph node.
		converter := markdown.New()
		adfDoc := map[string]any{
			"type":    "doc",
			"version": 1,
			"content": []any{
				map[string]any{
					"type": "taskList",
					"content": []any{
						map[string]any{
							"type": "taskItem",
							"attrs": map[string]any{
								"state": "TODO",
							},
							"content": []any{
								map[string]any{
									"type": "paragraph",
									"content": []any{
										map[string]any{"type": "text", "text": "Task in paragraph"},
									},
								},
							},
						},
						map[string]any{
							"type": "taskItem",
							"attrs": map[string]any{
								"state": "DONE",
							},
							"content": []any{
								map[string]any{
									"type": "paragraph",
									"content": []any{
										map[string]any{"type": "text", "text": "Done task"},
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
		assert.Equal(t, "- [ ] Task in paragraph\n- [x] Done task", result)
	})

	t.Run("normalizes newlines in table cells", func(t *testing.T) {
		t.Parallel()

		converter := markdown.New()
		adfDoc := map[string]any{
			"type":    "doc",
			"version": 1,
			"content": []any{
				map[string]any{
					"type": "table",
					"content": []any{
						map[string]any{
							"type": "tableRow",
							"content": []any{
								map[string]any{
									"type": "tableHeader",
									"content": []any{
										map[string]any{
											"type": "paragraph",
											"content": []any{
												map[string]any{"type": "text", "text": "Col"},
											},
										},
									},
								},
							},
						},
						map[string]any{
							"type": "tableRow",
							"content": []any{
								map[string]any{
									"type": "tableCell",
									"content": []any{
										map[string]any{
											"type": "paragraph",
											"content": []any{
												map[string]any{"type": "text", "text": "line1"},
												map[string]any{"type": "hardBreak"},
												map[string]any{"type": "text", "text": "line2"},
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
		adfDoc := map[string]any{
			"type":    "doc",
			"version": 1,
			"content": []any{
				map[string]any{
					"type":    "table",
					"content": []any{},
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
		adfDoc := map[string]any{
			"type":    "doc",
			"version": 1,
			"content": []any{
				map[string]any{
					"type": "taskList",
					"content": []any{
						map[string]any{
							"type": "taskItem",
							// No attrs at all
							"content": []any{
								map[string]any{
									"type": "paragraph",
									"content": []any{
										map[string]any{"type": "text", "text": "No state"},
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
		adfDoc := map[string]any{
			"type":    "doc",
			"version": 1,
			"content": []any{
				map[string]any{
					"type":  "heading",
					"attrs": map[string]any{
						// level is missing
					},
					"content": []any{
						map[string]any{
							"type": "text",
							"text": "Default Heading",
						},
					},
				},
			},
		}

		result, warnings := converter.ToMarkdown(adfDoc)

		assert.Empty(t, warnings)
		assert.Equal(t, "# Default Heading", result)
	})
}
