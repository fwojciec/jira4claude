package markdown_test

import (
	"testing"

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

		expected := map[string]any{
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

		assert.Empty(t, warnings)
		assert.Equal(t, expected, result)
	})

	t.Run("converts bold text to strong mark", func(t *testing.T) {
		t.Parallel()

		converter := markdown.New()
		result, warnings := converter.ToADF("This is **bold** text.")

		expected := map[string]any{
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

		assert.Empty(t, warnings)
		assert.Equal(t, expected, result)
	})

	t.Run("converts italic text to em mark", func(t *testing.T) {
		t.Parallel()

		converter := markdown.New()
		result, warnings := converter.ToADF("This is *italic* text.")

		expected := map[string]any{
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

		assert.Empty(t, warnings)
		assert.Equal(t, expected, result)
	})

	t.Run("converts inline code to code mark", func(t *testing.T) {
		t.Parallel()

		converter := markdown.New()
		result, warnings := converter.ToADF("Use the `fmt.Println` function.")

		expected := map[string]any{
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

		assert.Empty(t, warnings)
		assert.Equal(t, expected, result)
	})

	t.Run("converts fenced code block to codeBlock node", func(t *testing.T) {
		t.Parallel()

		converter := markdown.New()
		result, warnings := converter.ToADF("```go\nfmt.Println(\"hello\")\n```")

		expected := map[string]any{
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

		assert.Empty(t, warnings)
		assert.Equal(t, expected, result)
	})

	t.Run("converts heading to heading node with level", func(t *testing.T) {
		t.Parallel()

		converter := markdown.New()
		result, warnings := converter.ToADF("# Heading 1\n\n## Heading 2")

		expected := map[string]any{
			"type":    "doc",
			"version": 1,
			"content": []any{
				map[string]any{
					"type": "heading",
					"attrs": map[string]any{
						"level": 1,
					},
					"content": []any{
						map[string]any{
							"type": "text",
							"text": "Heading 1",
						},
					},
				},
				map[string]any{
					"type": "heading",
					"attrs": map[string]any{
						"level": 2,
					},
					"content": []any{
						map[string]any{
							"type": "text",
							"text": "Heading 2",
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

		expected := map[string]any{
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

		assert.Empty(t, warnings)
		assert.Equal(t, expected, result)
	})

	t.Run("converts ordered list to orderedList node", func(t *testing.T) {
		t.Parallel()

		converter := markdown.New()
		result, warnings := converter.ToADF("1. First\n2. Second")

		expected := map[string]any{
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

		assert.Empty(t, warnings)
		assert.Equal(t, expected, result)
	})

	t.Run("converts link to link mark", func(t *testing.T) {
		t.Parallel()

		converter := markdown.New()
		result, warnings := converter.ToADF("Visit [Google](https://google.com) for more.")

		expected := map[string]any{
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

		assert.Empty(t, warnings)
		assert.Equal(t, expected, result)
	})

	t.Run("converts blockquote to blockquote node", func(t *testing.T) {
		t.Parallel()

		converter := markdown.New()
		result, warnings := converter.ToADF("> This is a quote.")

		expected := map[string]any{
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

		assert.Empty(t, warnings)
		assert.Equal(t, expected, result)
	})

	t.Run("handles combined formatting", func(t *testing.T) {
		t.Parallel()

		converter := markdown.New()
		result, warnings := converter.ToADF("This is ***bold and italic*** text.")

		expected := map[string]any{
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

		assert.Empty(t, warnings)
		assert.Equal(t, expected, result)
	})

	t.Run("handles empty input", func(t *testing.T) {
		t.Parallel()

		converter := markdown.New()
		result, warnings := converter.ToADF("")

		expected := map[string]any{
			"type":    "doc",
			"version": 1,
			"content": []any{},
		}

		assert.Empty(t, warnings)
		assert.Equal(t, expected, result)
	})

	t.Run("returns warning when content is skipped", func(t *testing.T) {
		t.Parallel()

		converter := markdown.New()
		// Horizontal rules (thematic breaks) are not supported
		result, warnings := converter.ToADF("Before\n\n---\n\nAfter")

		// Should still return converted content (best effort)
		require.NotNil(t, result)
		assert.Equal(t, "doc", result["type"])

		// Content should have the paragraphs that were converted
		content, ok := result["content"].([]any)
		require.True(t, ok)
		assert.Len(t, content, 2) // "Before" and "After" paragraphs

		// Should return warning listing skipped content
		require.Len(t, warnings, 1)
		assert.Contains(t, warnings[0], "ThematicBreak")
	})

	t.Run("accumulates multiple warnings for different skipped node types", func(t *testing.T) {
		t.Parallel()

		converter := markdown.New()
		// Multiple unsupported block elements: horizontal rule and raw HTML block
		result, warnings := converter.ToADF("Start\n\n---\n\n<div>html block</div>\n\nEnd")

		// Should still return converted content (best effort)
		require.NotNil(t, result)
		assert.Equal(t, "doc", result["type"])

		// Should return warnings for each skipped type, sorted alphabetically
		require.Len(t, warnings, 2)
		assert.Contains(t, warnings[0], "HTMLBlock")
		assert.Contains(t, warnings[1], "ThematicBreak")
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

		expected := map[string]any{
			"type":    "doc",
			"version": 1,
			"content": []any{
				map[string]any{
					"type": "paragraph",
					"content": []any{
						map[string]any{
							"type": "text",
							"text": "Hello world",
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

		content, ok := result["content"].([]any)
		require.True(t, ok)
		require.Len(t, content, 1)

		paragraph, ok := content[0].(map[string]any)
		require.True(t, ok)

		paragraphContent, ok := paragraph["content"].([]any)
		require.True(t, ok)
		// Should have 2 separate text nodes: "bold" with mark, "plain" without
		assert.Len(t, paragraphContent, 2)
	})

	t.Run("produces link mark with nested attrs map", func(t *testing.T) {
		t.Parallel()

		// Links produce marks with nested attrs maps (href).
		// This tests the mapEqual code path for nested map comparison.
		converter := markdown.New()
		result, warnings := converter.ToADF("[click here](https://example.com)")

		expected := map[string]any{
			"type":    "doc",
			"version": 1,
			"content": []any{
				map[string]any{
					"type": "paragraph",
					"content": []any{
						map[string]any{
							"type": "text",
							"text": "click here",
							"marks": []any{
								map[string]any{
									"type": "link",
									"attrs": map[string]any{
										"href": "https://example.com",
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

	t.Run("does not consolidate text nodes with different link hrefs", func(t *testing.T) {
		t.Parallel()

		// Two adjacent links with different hrefs should remain separate
		converter := markdown.New()
		result, warnings := converter.ToADF("[one](https://one.com)[two](https://two.com)")

		require.NotNil(t, result)
		assert.Empty(t, warnings)

		content, ok := result["content"].([]any)
		require.True(t, ok)
		require.Len(t, content, 1)

		paragraph, ok := content[0].(map[string]any)
		require.True(t, ok)

		paragraphContent, ok := paragraph["content"].([]any)
		require.True(t, ok)
		// Should have 2 separate text nodes with different links
		assert.Len(t, paragraphContent, 2)
	})

	t.Run("does not consolidate when one has marks and other does not", func(t *testing.T) {
		t.Parallel()

		// Text without marks followed by text with marks should remain separate
		converter := markdown.New()
		result, warnings := converter.ToADF("plain**bold**")

		require.NotNil(t, result)
		assert.Empty(t, warnings)

		content, ok := result["content"].([]any)
		require.True(t, ok)
		require.Len(t, content, 1)

		paragraph, ok := content[0].(map[string]any)
		require.True(t, ok)

		paragraphContent, ok := paragraph["content"].([]any)
		require.True(t, ok)
		// Should have 2 separate text nodes: "plain" without mark, "bold" with mark
		assert.Len(t, paragraphContent, 2)
	})

	t.Run("does not consolidate when marks have different lengths", func(t *testing.T) {
		t.Parallel()

		// Bold text vs bold+italic text should remain separate (different mark counts)
		converter := markdown.New()
		result, warnings := converter.ToADF("**just bold*****bold and italic***")

		require.NotNil(t, result)
		assert.Empty(t, warnings)

		content, ok := result["content"].([]any)
		require.True(t, ok)
		require.Len(t, content, 1)

		paragraph, ok := content[0].(map[string]any)
		require.True(t, ok)

		paragraphContent, ok := paragraph["content"].([]any)
		require.True(t, ok)
		// Should have 2 separate text nodes with different mark counts
		assert.Len(t, paragraphContent, 2)
	})
}
