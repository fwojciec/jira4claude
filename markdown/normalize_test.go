package markdown_test

import (
	"testing"

	"github.com/fwojciec/jira4claude/markdown"
	"github.com/stretchr/testify/assert"
)

func TestNormalizeUnicode(t *testing.T) {
	t.Parallel()

	t.Run("preserves common Unicode punctuation and symbols", func(t *testing.T) {
		t.Parallel()

		testCases := []struct {
			name  string
			input string
		}{
			{"right arrow", "Use this → see results"},
			{"left arrow", "Go ← back"},
			{"double arrow", "a ⇒ b"},
			{"em dash", "value — important"},
			{"en dash", "pages 1–10"},
			{"smart double quotes", "“Hello”"},
			{"smart single quotes", "it’s fine"},
			{"ellipsis", "wait…"},
			{"star symbol", "x ★ y"},
			{"bullet", "item • next"},
			{"section sign", "§ 1.2"},
			{"non-breaking space", "hello world"},
			{"degree sign", "30°C"},
			{"copyright", "© 2026"},
			{"trademark", "Brand™"},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				result := markdown.NormalizeUnicode(tc.input)
				assert.Equal(t, tc.input, result, "expected character to be preserved")
			})
		}
	})

	t.Run("preserves ASCII characters", func(t *testing.T) {
		t.Parallel()

		testCases := []struct {
			name  string
			input string
		}{
			{"plain text", "Hello, world!"},
			{"markdown formatting", "normal **bold** `code`"},
			{"empty string", ""},
			{"ASCII symbols", "x -> y -- z != w"},
			{"newline preserved", "line1\nline2"},
			{"tab preserved", "col1\tcol2"},
			{"carriage return preserved", "line1\rline2"},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				result := markdown.NormalizeUnicode(tc.input)
				assert.Equal(t, tc.input, result)
			})
		}
	})

	t.Run("preserves non-ASCII letters and numbers", func(t *testing.T) {
		t.Parallel()

		testCases := []struct {
			name  string
			input string
		}{
			{"accented characters", "café naïve"},
			{"CJK characters", "你好世界"},
			{"Cyrillic", "Привет"},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				result := markdown.NormalizeUnicode(tc.input)
				assert.Equal(t, tc.input, result)
			})
		}
	})

	t.Run("replaces control characters with space", func(t *testing.T) {
		t.Parallel()

		testCases := []struct {
			name     string
			input    string
			expected string
		}{
			{"null", "a\x00b", "a b"},
			{"bell", "a\x07b", "a b"},
			{"backspace", "a\x08b", "a b"},
			{"vertical tab", "a\x0bb", "a b"},
			{"form feed", "a\x0cb", "a b"},
			{"escape", "a\x1bb", "a b"},
			{"unit separator", "a\x1fb", "a b"},
			{"delete", "a\x7fb", "a b"},
			{"C1 control (PAD)", "a\u0080b", "a b"},
			{"C1 control (APC)", "a\u009fb", "a b"},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				result := markdown.NormalizeUnicode(tc.input)
				assert.Equal(t, tc.expected, result)
			})
		}
	})
}
