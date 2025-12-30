package http_test

import (
	"testing"

	jirahttp "github.com/fwojciec/jira4claude/http"
	"github.com/stretchr/testify/assert"
)

func TestTextToADF(t *testing.T) {
	t.Parallel()

	t.Run("wraps plain text in ADF structure", func(t *testing.T) {
		t.Parallel()

		result := jirahttp.TextToADF("Hello, world!")

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
}

func TestADFToText(t *testing.T) {
	t.Parallel()

	t.Run("extracts text from simple ADF paragraph", func(t *testing.T) {
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

		result := jirahttp.ADFToText(adf)

		assert.Equal(t, "Hello, world!", result)
	})

	t.Run("returns empty string for nil input", func(t *testing.T) {
		t.Parallel()

		result := jirahttp.ADFToText(nil)

		assert.Empty(t, result)
	})

	t.Run("returns empty string for empty document", func(t *testing.T) {
		t.Parallel()

		adf := map[string]any{
			"type":    "doc",
			"version": 1,
			"content": []any{},
		}

		result := jirahttp.ADFToText(adf)

		assert.Empty(t, result)
	})

	t.Run("separates multiple paragraphs with newlines", func(t *testing.T) {
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

		result := jirahttp.ADFToText(adf)

		assert.Equal(t, "First paragraph.\n\nSecond paragraph.", result)
	})

	t.Run("converts hardBreak to newline", func(t *testing.T) {
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
							"text": "Line one",
						},
						map[string]any{
							"type": "hardBreak",
						},
						map[string]any{
							"type": "text",
							"text": "Line two",
						},
					},
				},
			},
		}

		result := jirahttp.ADFToText(adf)

		assert.Equal(t, "Line one\nLine two", result)
	})
}

func TestRoundTrip(t *testing.T) {
	t.Parallel()

	t.Run("text survives round-trip conversion", func(t *testing.T) {
		t.Parallel()

		original := "This is a test message with special chars: <>&\""

		adf := jirahttp.TextToADF(original)
		result := jirahttp.ADFToText(adf)

		assert.Equal(t, original, result)
	})

	t.Run("empty string survives round-trip", func(t *testing.T) {
		t.Parallel()

		original := ""

		adf := jirahttp.TextToADF(original)
		result := jirahttp.ADFToText(adf)

		assert.Equal(t, original, result)
	})
}
