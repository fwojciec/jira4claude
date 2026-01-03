package gogh_test

import (
	"bytes"
	"strings"
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
	// Verify wrapping occurred - output should contain multiple lines
	lines := strings.Split(strings.TrimSpace(output), "\n")
	assert.Greater(t, len(lines), 1, "long text should be wrapped into multiple lines")
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

func TestStyles_RenderMarkdown_ColorMode_BodyTextUsesTerminalDefault(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	r := lipgloss.NewRenderer(&buf)
	r.SetColorProfile(termenv.TrueColor)
	styles := gogh.NewStyles(r)

	// Input with header and plain body text
	input := "## Header\n\nPlain body text here."
	output, err := styles.RenderMarkdown(input)

	require.NoError(t, err)
	assert.Contains(t, output, "Header")
	assert.Contains(t, output, "Plain body text here")

	// Find "Plain body text here" in output and verify no color codes around it
	// ANSI color codes look like \x1b[38;... for foreground colors
	// The body text line should not have color escape sequences
	lines := strings.Split(output, "\n")
	var bodyLine string
	for _, line := range lines {
		if strings.Contains(line, "Plain body text") {
			bodyLine = line
			break
		}
	}
	require.NotEmpty(t, bodyLine, "should find body text line")

	// Body text should not contain foreground color codes
	// We check for common foreground patterns: 38;5;xxx (256-color) and 38;2;xxx (true-color)
	// Any explicit foreground color on body text means we're not using terminal default
	assert.NotContains(t, bodyLine, "\x1b[38;", "body text should use terminal default color, not explicit colors")
}

func TestStyles_RenderMarkdown_ColorMode_HeadersHaveColors(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	r := lipgloss.NewRenderer(&buf)
	r.SetColorProfile(termenv.TrueColor)
	styles := gogh.NewStyles(r)

	input := "## Colored Header"
	output, err := styles.RenderMarkdown(input)

	require.NoError(t, err)
	// Headers should have color styling in color mode
	// The dark style uses color 39 (bright blue) for headers
	assert.Contains(t, output, "\x1b[38;5;39", "headers should have color styling in color mode")
}

func TestStyles_RenderMarkdown_ColorMode_CodeBlocksHaveStyling(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	r := lipgloss.NewRenderer(&buf)
	r.SetColorProfile(termenv.TrueColor)
	styles := gogh.NewStyles(r)

	input := "```go\nfunc main() {}\n```"
	output, err := styles.RenderMarkdown(input)

	require.NoError(t, err)
	assert.Contains(t, output, "func")
	// Code blocks should have syntax highlighting (ANSI codes)
	assert.Contains(t, output, "\x1b[", "code blocks should have syntax highlighting in color mode")
}

func TestStyles_RenderMarkdown_NoColorMode_NoLeadingIndentation(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	r := lipgloss.NewRenderer(&buf)
	r.SetColorProfile(termenv.Ascii)
	styles := gogh.NewStyles(r)

	input := "Simple paragraph text."
	output, err := styles.RenderMarkdown(input)

	require.NoError(t, err)

	// Remove leading/trailing newlines but keep internal structure
	trimmed := strings.Trim(output, "\n")

	// Each line should start at column 0 (no leading spaces)
	lines := strings.Split(trimmed, "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		assert.False(t, strings.HasPrefix(line, " "),
			"line should not have leading indentation: %q", line)
	}
}

func TestStyles_RenderMarkdown_NoColorMode_ListsRenderCorrectly(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	r := lipgloss.NewRenderer(&buf)
	r.SetColorProfile(termenv.Ascii)
	styles := gogh.NewStyles(r)

	input := "- First item\n- Second item\n- Third item"
	output, err := styles.RenderMarkdown(input)

	require.NoError(t, err)
	assert.Contains(t, output, "First item")
	assert.Contains(t, output, "Second item")
	assert.Contains(t, output, "Third item")
	// ASCII style uses • for bullets
	assert.Contains(t, output, "•")
}

func TestStyles_RenderMarkdown_NoColorMode_CodeBlocksRenderCorrectly(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	r := lipgloss.NewRenderer(&buf)
	r.SetColorProfile(termenv.Ascii)
	styles := gogh.NewStyles(r)

	input := "```\ncode here\n```"
	output, err := styles.RenderMarkdown(input)

	require.NoError(t, err)
	assert.Contains(t, output, "code here")
	// Should have no ANSI codes in NO_COLOR mode
	assert.NotContains(t, output, "\x1b[")
}
