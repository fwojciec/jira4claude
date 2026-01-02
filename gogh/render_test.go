package gogh_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/fwojciec/jira4claude/gogh"
	"github.com/muesli/termenv"
	"github.com/stretchr/testify/assert"
)

func TestRenderCard(t *testing.T) {
	t.Parallel()

	t.Run("with color mode produces card with rounded borders", func(t *testing.T) {
		t.Parallel()

		var buf bytes.Buffer
		r := lipgloss.NewRenderer(&buf)
		r.SetColorProfile(termenv.TrueColor)
		styles := gogh.NewStyles(r)

		result := gogh.RenderCard(styles, "LINKED ISSUES", "test content")

		// Should contain border characters
		assert.Contains(t, result, "╭")
		assert.Contains(t, result, "╯")
		assert.Contains(t, result, "LINKED ISSUES")
		assert.Contains(t, result, "test content")
	})

	t.Run("with no color mode produces section with header", func(t *testing.T) {
		t.Parallel()

		var buf bytes.Buffer
		r := lipgloss.NewRenderer(&buf)
		r.SetColorProfile(termenv.Ascii)
		styles := gogh.NewStyles(r)

		result := gogh.RenderCard(styles, "LINKED ISSUES", "test content")

		// Should not contain border characters
		assert.NotContains(t, result, "╭")
		assert.NotContains(t, result, "╯")
		// Should contain title with separators
		assert.Contains(t, result, "=== LINKED ISSUES ===")
		assert.Contains(t, result, "test content")
	})

	t.Run("with empty title renders content only", func(t *testing.T) {
		t.Parallel()

		var buf bytes.Buffer
		r := lipgloss.NewRenderer(&buf)
		r.SetColorProfile(termenv.Ascii)
		styles := gogh.NewStyles(r)

		result := gogh.RenderCard(styles, "", "just content")

		assert.Contains(t, result, "just content")
		assert.NotContains(t, result, "===")
	})
}

func TestRenderStatusBadge(t *testing.T) {
	t.Parallel()

	t.Run("with color mode uses unicode indicators", func(t *testing.T) {
		t.Parallel()

		var buf bytes.Buffer
		r := lipgloss.NewRenderer(&buf)
		r.SetColorProfile(termenv.TrueColor)
		styles := gogh.NewStyles(r)

		done := gogh.RenderStatusBadge(styles, "Done")
		assert.Contains(t, done, "✓")
		assert.Contains(t, done, "Done")

		inProgress := gogh.RenderStatusBadge(styles, "In Progress")
		assert.Contains(t, inProgress, "▶")
		assert.Contains(t, inProgress, "In Progress")

		toDo := gogh.RenderStatusBadge(styles, "To Do")
		assert.Contains(t, toDo, "○")
		assert.Contains(t, toDo, "To Do")
	})

	t.Run("with no color mode uses ascii indicators", func(t *testing.T) {
		t.Parallel()

		var buf bytes.Buffer
		r := lipgloss.NewRenderer(&buf)
		r.SetColorProfile(termenv.Ascii)
		styles := gogh.NewStyles(r)

		done := gogh.RenderStatusBadge(styles, "Done")
		assert.Contains(t, done, "[x]")
		assert.Contains(t, done, "Done")

		inProgress := gogh.RenderStatusBadge(styles, "In Progress")
		assert.Contains(t, inProgress, "[>]")
		assert.Contains(t, inProgress, "In Progress")

		toDo := gogh.RenderStatusBadge(styles, "To Do")
		assert.Contains(t, toDo, "[ ]")
		assert.Contains(t, toDo, "To Do")
	})
}

func TestRenderPriorityBadge(t *testing.T) {
	t.Parallel()

	t.Run("with color mode uses unicode indicators", func(t *testing.T) {
		t.Parallel()

		var buf bytes.Buffer
		r := lipgloss.NewRenderer(&buf)
		r.SetColorProfile(termenv.TrueColor)
		styles := gogh.NewStyles(r)

		highest := gogh.RenderPriorityBadge(styles, "Highest")
		assert.Contains(t, highest, "▲▲▲")

		high := gogh.RenderPriorityBadge(styles, "High")
		assert.Contains(t, high, "▲▲")
		// Make sure it doesn't match "▲▲▲"
		assert.Equal(t, 2, strings.Count(high, "▲"))

		medium := gogh.RenderPriorityBadge(styles, "Medium")
		assert.Contains(t, medium, "▲")
		assert.Equal(t, 1, strings.Count(medium, "▲"))

		low := gogh.RenderPriorityBadge(styles, "Low")
		assert.Contains(t, low, "▽")

		lowest := gogh.RenderPriorityBadge(styles, "Lowest")
		assert.Contains(t, lowest, "▽▽")
	})

	t.Run("with no color mode uses ascii indicators", func(t *testing.T) {
		t.Parallel()

		var buf bytes.Buffer
		r := lipgloss.NewRenderer(&buf)
		r.SetColorProfile(termenv.Ascii)
		styles := gogh.NewStyles(r)

		highest := gogh.RenderPriorityBadge(styles, "Highest")
		assert.Contains(t, highest, "[!!!]")

		high := gogh.RenderPriorityBadge(styles, "High")
		assert.Contains(t, high, "[!!]")

		medium := gogh.RenderPriorityBadge(styles, "Medium")
		assert.Contains(t, medium, "[!]")

		low := gogh.RenderPriorityBadge(styles, "Low")
		assert.Contains(t, low, "[-]")

		lowest := gogh.RenderPriorityBadge(styles, "Lowest")
		assert.Contains(t, lowest, "[--]")
	})
}

func TestRenderSeparator(t *testing.T) {
	t.Parallel()

	t.Run("dotted separator with color mode", func(t *testing.T) {
		t.Parallel()

		var buf bytes.Buffer
		r := lipgloss.NewRenderer(&buf)
		r.SetColorProfile(termenv.TrueColor)
		styles := gogh.NewStyles(r)

		result := gogh.RenderSeparator(styles, "dotted", 10)

		// Should be 10 dotted characters
		assert.Equal(t, "┄┄┄┄┄┄┄┄┄┄", result)
	})

	t.Run("solid separator with color mode", func(t *testing.T) {
		t.Parallel()

		var buf bytes.Buffer
		r := lipgloss.NewRenderer(&buf)
		r.SetColorProfile(termenv.TrueColor)
		styles := gogh.NewStyles(r)

		result := gogh.RenderSeparator(styles, "solid", 10)

		assert.Equal(t, "──────────", result)
	})

	t.Run("dotted separator with no color mode", func(t *testing.T) {
		t.Parallel()

		var buf bytes.Buffer
		r := lipgloss.NewRenderer(&buf)
		r.SetColorProfile(termenv.Ascii)
		styles := gogh.NewStyles(r)

		result := gogh.RenderSeparator(styles, "dotted", 10)

		assert.Equal(t, "..........", result)
	})

	t.Run("solid separator with no color mode", func(t *testing.T) {
		t.Parallel()

		var buf bytes.Buffer
		r := lipgloss.NewRenderer(&buf)
		r.SetColorProfile(termenv.Ascii)
		styles := gogh.NewStyles(r)

		result := gogh.RenderSeparator(styles, "solid", 10)

		assert.Equal(t, "----------", result)
	})
}
