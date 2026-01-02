package gogh_test

import (
	"bytes"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/fwojciec/jira4claude/gogh"
	"github.com/muesli/termenv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewStyles(t *testing.T) {
	t.Parallel()

	t.Run("with color renderer uses unicode indicators", func(t *testing.T) {
		t.Parallel()

		// Create a renderer with color support
		var buf bytes.Buffer
		r := lipgloss.NewRenderer(&buf)
		r.SetColorProfile(termenv.TrueColor)

		styles := gogh.NewStyles(r)

		require.NotNil(t, styles)
		assert.False(t, styles.NoColor)
		assert.Equal(t, "✓", styles.Indicators.StatusDone)
		assert.Equal(t, "▶", styles.Indicators.StatusInProgress)
		assert.Equal(t, "○", styles.Indicators.StatusToDo)
		assert.Equal(t, "→", styles.Indicators.Arrow)
		assert.Equal(t, "✓", styles.Indicators.Success)
		assert.Equal(t, "⚠", styles.Indicators.Warning)
		assert.Equal(t, "✗", styles.Indicators.Error)
	})

	t.Run("with no color renderer uses ascii indicators", func(t *testing.T) {
		t.Parallel()

		// Create a renderer with no color support (ASCII mode)
		var buf bytes.Buffer
		r := lipgloss.NewRenderer(&buf)
		r.SetColorProfile(termenv.Ascii)

		styles := gogh.NewStyles(r)

		require.NotNil(t, styles)
		assert.True(t, styles.NoColor)
		assert.Equal(t, "[x]", styles.Indicators.StatusDone)
		assert.Equal(t, "[>]", styles.Indicators.StatusInProgress)
		assert.Equal(t, "[ ]", styles.Indicators.StatusToDo)
		assert.Equal(t, "->", styles.Indicators.Arrow)
		assert.Equal(t, "[ok]", styles.Indicators.Success)
		assert.Equal(t, "[warn]", styles.Indicators.Warning)
		assert.Equal(t, "[error]", styles.Indicators.Error)
	})

	t.Run("with color renderer uses unicode separators", func(t *testing.T) {
		t.Parallel()

		var buf bytes.Buffer
		r := lipgloss.NewRenderer(&buf)
		r.SetColorProfile(termenv.TrueColor)

		styles := gogh.NewStyles(r)

		assert.Equal(t, "┄", styles.Separators.Dotted)
		assert.Equal(t, "─", styles.Separators.Solid)
	})

	t.Run("with no color renderer uses ascii separators", func(t *testing.T) {
		t.Parallel()

		var buf bytes.Buffer
		r := lipgloss.NewRenderer(&buf)
		r.SetColorProfile(termenv.Ascii)

		styles := gogh.NewStyles(r)

		assert.Equal(t, ".", styles.Separators.Dotted)
		assert.Equal(t, "-", styles.Separators.Solid)
	})

	t.Run("sets default width to 80", func(t *testing.T) {
		t.Parallel()

		var buf bytes.Buffer
		r := lipgloss.NewRenderer(&buf)
		r.SetColorProfile(termenv.TrueColor)

		styles := gogh.NewStyles(r)

		assert.Equal(t, 80, styles.Width)
	})
}

func TestDefaultStyles(t *testing.T) {
	t.Parallel()

	// DefaultStyles should return a valid Styles struct
	styles := gogh.DefaultStyles()

	require.NotNil(t, styles)
	assert.Equal(t, 80, styles.Width)
}

func TestStyles_RenderMarkdown_ColorMode_ReturnsStyledOutput(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	r := lipgloss.NewRenderer(&buf)
	r.SetColorProfile(termenv.TrueColor)
	styles := gogh.NewStyles(r)

	input := "## Header\n\nSome text."
	output, err := styles.RenderMarkdown(input)

	require.NoError(t, err)
	assert.Contains(t, output, "Header")
	assert.Contains(t, output, "Some text")
}

func TestStyles_RenderMarkdown_ColorMode_RendersBullets(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	r := lipgloss.NewRenderer(&buf)
	r.SetColorProfile(termenv.TrueColor)
	styles := gogh.NewStyles(r)

	input := "- Item one\n- Item two"
	output, err := styles.RenderMarkdown(input)

	require.NoError(t, err)
	assert.Contains(t, output, "Item one")
	assert.Contains(t, output, "Item two")
	// Glamour uses • for bullet points
	assert.Contains(t, output, "•")
}

func TestStyles_RenderMarkdown_TextOnlyMode_NoANSICodes(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	r := lipgloss.NewRenderer(&buf)
	r.SetColorProfile(termenv.Ascii)
	styles := gogh.NewStyles(r)

	input := "## Header\n\n**Bold text** and normal."
	output, err := styles.RenderMarkdown(input)

	require.NoError(t, err)
	assert.Contains(t, output, "Header")
	assert.Contains(t, output, "Bold text")
	// Should have no ANSI escape codes in text-only mode
	assert.NotContains(t, output, "\x1b[")
}

func TestStyles_RenderMarkdown_WordWrap(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	r := lipgloss.NewRenderer(&buf)
	r.SetColorProfile(termenv.TrueColor)
	styles := gogh.NewStyles(r)

	// Long line that should be wrapped at 80 columns
	input := "This is a very long line of text that should definitely be wrapped because it exceeds the 80 column width limit."
	output, err := styles.RenderMarkdown(input)

	require.NoError(t, err)
	assert.Contains(t, output, "This is a very long line")
}

func TestStyles_RenderMarkdown_EmptyInput_ReturnsEmpty(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	r := lipgloss.NewRenderer(&buf)
	r.SetColorProfile(termenv.TrueColor)
	styles := gogh.NewStyles(r)

	output, err := styles.RenderMarkdown("")

	require.NoError(t, err)
	assert.Empty(t, output)
}
