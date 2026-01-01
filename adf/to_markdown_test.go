package adf_test

import (
	"testing"

	"github.com/fwojciec/jira4claude/adf"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConverter_ToMarkdown(t *testing.T) {
	t.Parallel()

	t.Run("converts simple paragraph to plain text", func(t *testing.T) {
		t.Parallel()

		converter := adf.New()
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

		converter := adf.New()
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

		converter := adf.New()
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

		converter := adf.New()
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

		converter := adf.New()
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

		converter := adf.New()
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

		converter := adf.New()
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

		converter := adf.New()
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

		converter := adf.New()
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

		converter := adf.New()
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

		converter := adf.New()
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

		converter := adf.New()
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

		converter := adf.New()
		result, warnings := converter.ToMarkdown(nil)

		assert.Empty(t, warnings)
		assert.Empty(t, result)
	})

	t.Run("handles empty document", func(t *testing.T) {
		t.Parallel()

		converter := adf.New()
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

		converter := adf.New()
		// ADF with an unsupported node type (e.g., "table")
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
					"type": "table",
					"content": []any{
						map[string]any{"type": "tableRow"},
					},
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
		assert.Contains(t, warnings[0], "table")
	})

	t.Run("accumulates multiple warnings for different skipped node types", func(t *testing.T) {
		t.Parallel()

		converter := adf.New()
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
					"type": "table",
					"content": []any{
						map[string]any{"type": "tableRow"},
					},
				},
				map[string]any{
					"type":    "panel",
					"content": []any{},
				},
				map[string]any{
					"type": "rule",
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
		require.Len(t, warnings, 3)
		assert.Contains(t, warnings[0], "panel")
		assert.Contains(t, warnings[1], "rule")
		assert.Contains(t, warnings[2], "table")
	})

	t.Run("returns empty warnings slice when no content is skipped", func(t *testing.T) {
		t.Parallel()

		converter := adf.New()
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
}
