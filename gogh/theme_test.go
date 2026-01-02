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
