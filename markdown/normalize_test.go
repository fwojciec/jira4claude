package markdown_test

import (
	"testing"

	"github.com/fwojciec/jira4claude/markdown"
	"github.com/stretchr/testify/assert"
)

func TestNormalizeUnicode(t *testing.T) {
	t.Parallel()

	t.Run("replaces non-ASCII symbols with space", func(t *testing.T) {
		t.Parallel()

		testCases := []struct {
			name     string
			input    string
			expected string
		}{
			{"right arrow", "Use this \u2192 see results", "Use this   see results"},
			{"left arrow", "Go \u2190 back", "Go   back"},
			{"em dash", "value \u2014 important", "value   important"},
			{"en dash", "pages 1\u201310", "pages 1 10"},
			{"smart double quotes", "\u201CHello\u201D", " Hello "},
			{"smart single quotes", "it\u2019s fine", "it s fine"},
			{"ellipsis", "wait\u2026", "wait "},
			{"star symbol", "x \u2605 y", "x   y"},
			{"bullet", "item \u2022 next", "item   next"},
			{"double arrow", "a \u21D2 b", "a   b"},
			{"section sign", "\u00A7 1.2", "  1.2"},
			{"non-breaking space", "hello\u00A0world", "hello world"},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				result := markdown.NormalizeUnicode(tc.input)
				assert.Equal(t, tc.expected, result)
			})
		}
	})

	t.Run("preserves ASCII characters", func(t *testing.T) {
		t.Parallel()

		testCases := []struct {
			name     string
			input    string
			expected string
		}{
			{"plain text", "Hello, world!", "Hello, world!"},
			{"markdown formatting", "normal **bold** `code`", "normal **bold** `code`"},
			{"empty string", "", ""},
			{"ASCII symbols", "x -> y -- z != w", "x -> y -- z != w"},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				result := markdown.NormalizeUnicode(tc.input)
				assert.Equal(t, tc.expected, result)
			})
		}
	})

	t.Run("preserves non-ASCII letters and numbers", func(t *testing.T) {
		t.Parallel()

		testCases := []struct {
			name     string
			input    string
			expected string
		}{
			{"accented characters", "caf\u00E9 na\u00EFve", "caf\u00E9 na\u00EFve"},
			{"CJK characters", "\u4F60\u597D\u4E16\u754C", "\u4F60\u597D\u4E16\u754C"},
			{"Cyrillic", "\u041F\u0440\u0438\u0432\u0435\u0442", "\u041F\u0440\u0438\u0432\u0435\u0442"},
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
