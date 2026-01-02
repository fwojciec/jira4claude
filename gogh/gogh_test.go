package gogh_test

import (
	"bytes"
	"os"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/fwojciec/jira4claude/gogh"
	"github.com/muesli/termenv"
	"github.com/stretchr/testify/assert"
)

// trueColorStyles creates a Styles instance with TrueColor profile for testing color mode.
func trueColorStyles(t *testing.T) *gogh.Styles {
	t.Helper()
	var buf bytes.Buffer
	r := lipgloss.NewRenderer(&buf)
	r.SetColorProfile(termenv.TrueColor)
	return gogh.NewStyles(r)
}

// asciiStyles creates a Styles instance with Ascii profile for testing text-only mode.
func asciiStyles(t *testing.T) *gogh.Styles {
	t.Helper()
	var buf bytes.Buffer
	r := lipgloss.NewRenderer(&buf)
	r.SetColorProfile(termenv.Ascii)
	return gogh.NewStyles(r)
}

func TestNewIO_WithBuffers(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	io := gogh.NewIO(&out, &errOut)

	assert.Equal(t, &out, io.Out)
	assert.Equal(t, &errOut, io.Err)
	assert.False(t, io.IsTerminal, "buffers should not be detected as terminal")
}

func TestNewIO_WithStdout(t *testing.T) {
	t.Parallel()

	io := gogh.NewIO(os.Stdout, os.Stderr)

	assert.Equal(t, os.Stdout, io.Out)
	assert.Equal(t, os.Stderr, io.Err)
	// IsTerminal depends on actual terminal state - don't assert specific value
}
