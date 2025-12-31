package http_test

import (
	"testing"

	jirahttp "github.com/fwojciec/jira4claude/http"
	"github.com/stretchr/testify/assert"
)

func TestADFToGFM(t *testing.T) {
	t.Parallel()

	t.Run("converts simple paragraph to plain text", func(t *testing.T) {
		t.Parallel()

		adf := map[string]any{
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

		result := jirahttp.ADFToGFM(adf)

		assert.Equal(t, "Hello, world!", result)
	})

	t.Run("converts strong mark to bold", func(t *testing.T) {
		t.Parallel()

		adf := map[string]any{
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

		result := jirahttp.ADFToGFM(adf)

		assert.Equal(t, "This is **bold** text.", result)
	})

	t.Run("converts em mark to italic", func(t *testing.T) {
		t.Parallel()

		adf := map[string]any{
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

		result := jirahttp.ADFToGFM(adf)

		assert.Equal(t, "This is *italic* text.", result)
	})

	t.Run("converts code mark to inline code", func(t *testing.T) {
		t.Parallel()

		adf := map[string]any{
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

		result := jirahttp.ADFToGFM(adf)

		assert.Equal(t, "Use the `fmt.Println` function.", result)
	})

	t.Run("converts codeBlock to fenced code block", func(t *testing.T) {
		t.Parallel()

		adf := map[string]any{
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

		result := jirahttp.ADFToGFM(adf)

		assert.Equal(t, "```go\nfmt.Println(\"hello\")\n```", result)
	})

	t.Run("converts heading to markdown heading", func(t *testing.T) {
		t.Parallel()

		adf := map[string]any{
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

		result := jirahttp.ADFToGFM(adf)

		assert.Equal(t, "## My Heading", result)
	})

	t.Run("converts bulletList to markdown list", func(t *testing.T) {
		t.Parallel()

		adf := map[string]any{
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

		result := jirahttp.ADFToGFM(adf)

		assert.Equal(t, "- Item 1\n- Item 2", result)
	})

	t.Run("converts orderedList to markdown list", func(t *testing.T) {
		t.Parallel()

		adf := map[string]any{
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

		result := jirahttp.ADFToGFM(adf)

		assert.Equal(t, "1. First\n2. Second", result)
	})

	t.Run("converts link mark to markdown link", func(t *testing.T) {
		t.Parallel()

		adf := map[string]any{
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

		result := jirahttp.ADFToGFM(adf)

		assert.Equal(t, "Visit [Google](https://google.com) for more.", result)
	})

	t.Run("converts blockquote to markdown blockquote", func(t *testing.T) {
		t.Parallel()

		adf := map[string]any{
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

		result := jirahttp.ADFToGFM(adf)

		assert.Equal(t, "> This is a quote.", result)
	})

	t.Run("handles multiple paragraphs", func(t *testing.T) {
		t.Parallel()

		adf := map[string]any{
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

		result := jirahttp.ADFToGFM(adf)

		assert.Equal(t, "First paragraph.\n\nSecond paragraph.", result)
	})

	t.Run("handles combined formatting (bold and italic)", func(t *testing.T) {
		t.Parallel()

		adf := map[string]any{
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

		result := jirahttp.ADFToGFM(adf)

		assert.Equal(t, "This is ***bold and italic*** text.", result)
	})

	t.Run("handles nil input", func(t *testing.T) {
		t.Parallel()

		result := jirahttp.ADFToGFM(nil)

		assert.Empty(t, result)
	})

	t.Run("handles empty document", func(t *testing.T) {
		t.Parallel()

		adf := map[string]any{
			"type":    "doc",
			"version": 1,
			"content": []any{},
		}

		result := jirahttp.ADFToGFM(adf)

		assert.Empty(t, result)
	})
}

func TestGFMToADF(t *testing.T) {
	t.Parallel()

	t.Run("converts plain text to paragraph", func(t *testing.T) {
		t.Parallel()

		result := jirahttp.GFMToADF("Hello, world!")

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

		assert.Equal(t, expected, result)
	})

	t.Run("converts bold text to strong mark", func(t *testing.T) {
		t.Parallel()

		result := jirahttp.GFMToADF("This is **bold** text.")

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

		assert.Equal(t, expected, result)
	})

	t.Run("converts italic text to em mark", func(t *testing.T) {
		t.Parallel()

		result := jirahttp.GFMToADF("This is *italic* text.")

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

		assert.Equal(t, expected, result)
	})

	t.Run("converts inline code to code mark", func(t *testing.T) {
		t.Parallel()

		result := jirahttp.GFMToADF("Use the `fmt.Println` function.")

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

		assert.Equal(t, expected, result)
	})

	t.Run("converts fenced code block to codeBlock node", func(t *testing.T) {
		t.Parallel()

		result := jirahttp.GFMToADF("```go\nfmt.Println(\"hello\")\n```")

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

		assert.Equal(t, expected, result)
	})

	t.Run("converts heading to heading node with level", func(t *testing.T) {
		t.Parallel()

		result := jirahttp.GFMToADF("# Heading 1\n\n## Heading 2")

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

		assert.Equal(t, expected, result)
	})

	t.Run("converts bullet list to bulletList node", func(t *testing.T) {
		t.Parallel()

		result := jirahttp.GFMToADF("- Item 1\n- Item 2")

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

		assert.Equal(t, expected, result)
	})

	t.Run("converts ordered list to orderedList node", func(t *testing.T) {
		t.Parallel()

		result := jirahttp.GFMToADF("1. First\n2. Second")

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

		assert.Equal(t, expected, result)
	})

	t.Run("converts link to link mark", func(t *testing.T) {
		t.Parallel()

		result := jirahttp.GFMToADF("Visit [Google](https://google.com) for more.")

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

		assert.Equal(t, expected, result)
	})

	t.Run("converts blockquote to blockquote node", func(t *testing.T) {
		t.Parallel()

		result := jirahttp.GFMToADF("> This is a quote.")

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

		assert.Equal(t, expected, result)
	})

	t.Run("handles combined formatting", func(t *testing.T) {
		t.Parallel()

		result := jirahttp.GFMToADF("This is ***bold and italic*** text.")

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

		assert.Equal(t, expected, result)
	})

	t.Run("handles empty input", func(t *testing.T) {
		t.Parallel()

		result := jirahttp.GFMToADF("")

		expected := map[string]any{
			"type":    "doc",
			"version": 1,
			"content": []any{},
		}

		assert.Equal(t, expected, result)
	})
}

func TestGFMRoundTrip(t *testing.T) {
	t.Parallel()

	t.Run("plain text preserves structure", func(t *testing.T) {
		t.Parallel()

		original := "Hello, world!"
		adf := jirahttp.GFMToADF(original)
		result := jirahttp.ADFToGFM(adf)

		assert.Equal(t, original, result)
	})

	t.Run("bold text preserves structure", func(t *testing.T) {
		t.Parallel()

		original := "This is **bold** text."
		adf := jirahttp.GFMToADF(original)
		result := jirahttp.ADFToGFM(adf)

		assert.Equal(t, original, result)
	})

	t.Run("italic text preserves structure", func(t *testing.T) {
		t.Parallel()

		original := "This is *italic* text."
		adf := jirahttp.GFMToADF(original)
		result := jirahttp.ADFToGFM(adf)

		assert.Equal(t, original, result)
	})

	t.Run("inline code preserves structure", func(t *testing.T) {
		t.Parallel()

		original := "Use `fmt.Println` function."
		adf := jirahttp.GFMToADF(original)
		result := jirahttp.ADFToGFM(adf)

		assert.Equal(t, original, result)
	})

	t.Run("code block preserves structure", func(t *testing.T) {
		t.Parallel()

		original := "```go\nfmt.Println(\"hello\")\n```"
		adf := jirahttp.GFMToADF(original)
		result := jirahttp.ADFToGFM(adf)

		assert.Equal(t, original, result)
	})

	t.Run("heading preserves structure", func(t *testing.T) {
		t.Parallel()

		original := "## My Heading"
		adf := jirahttp.GFMToADF(original)
		result := jirahttp.ADFToGFM(adf)

		assert.Equal(t, original, result)
	})

	t.Run("bullet list preserves structure", func(t *testing.T) {
		t.Parallel()

		original := "- Item 1\n- Item 2"
		adf := jirahttp.GFMToADF(original)
		result := jirahttp.ADFToGFM(adf)

		assert.Equal(t, original, result)
	})

	t.Run("ordered list preserves structure", func(t *testing.T) {
		t.Parallel()

		original := "1. First\n2. Second"
		adf := jirahttp.GFMToADF(original)
		result := jirahttp.ADFToGFM(adf)

		assert.Equal(t, original, result)
	})

	t.Run("link preserves structure", func(t *testing.T) {
		t.Parallel()

		original := "Visit [Google](https://google.com) for more."
		adf := jirahttp.GFMToADF(original)
		result := jirahttp.ADFToGFM(adf)

		assert.Equal(t, original, result)
	})

	t.Run("blockquote preserves structure", func(t *testing.T) {
		t.Parallel()

		original := "> This is a quote."
		adf := jirahttp.GFMToADF(original)
		result := jirahttp.ADFToGFM(adf)

		assert.Equal(t, original, result)
	})

	t.Run("multiple paragraphs preserve structure", func(t *testing.T) {
		t.Parallel()

		original := "First paragraph.\n\nSecond paragraph."
		adf := jirahttp.GFMToADF(original)
		result := jirahttp.ADFToGFM(adf)

		assert.Equal(t, original, result)
	})

	t.Run("combined bold and italic preserves structure", func(t *testing.T) {
		t.Parallel()

		original := "This is ***bold and italic*** text."
		adf := jirahttp.GFMToADF(original)
		result := jirahttp.ADFToGFM(adf)

		assert.Equal(t, original, result)
	})

	t.Run("complex document preserves structure", func(t *testing.T) {
		t.Parallel()

		original := `# Main Heading

This is a paragraph with **bold** and *italic* text.

## Subheading

- First item
- Second item

1. Numbered one
2. Numbered two

> A blockquote

` + "```go\nfunc main() {}\n```"

		adf := jirahttp.GFMToADF(original)
		result := jirahttp.ADFToGFM(adf)

		assert.Equal(t, original, result)
	})
}
