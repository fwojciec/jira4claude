package gogh_test

import (
	"os"
	"testing"

	"github.com/fwojciec/jira4claude/gogh"
	"github.com/stretchr/testify/assert"
)

//nolint:paralleltest // Modifies environment - cannot run parallel
func TestStyles_NoColor(t *testing.T) {
	original := os.Getenv("NO_COLOR")
	defer func() {
		if original == "" {
			os.Unsetenv("NO_COLOR")
		} else {
			os.Setenv("NO_COLOR", original)
		}
	}()

	os.Setenv("NO_COLOR", "1")
	styles := gogh.NewStyles()

	// When NO_COLOR is set, styled text should equal input
	assert.Equal(t, "TEST-123", styles.Key("TEST-123"))
	assert.Equal(t, "Open", styles.Status("Open"))
	assert.Equal(t, "Error message", styles.Error("Error message"))
	assert.Equal(t, "my-label", styles.Label("my-label"))
	assert.Equal(t, "Header Text", styles.Header("Header Text"))
}

//nolint:paralleltest // Modifies environment - cannot run parallel
func TestStyles_WithColor(t *testing.T) {
	original := os.Getenv("NO_COLOR")
	defer func() {
		if original == "" {
			os.Unsetenv("NO_COLOR")
		} else {
			os.Setenv("NO_COLOR", original)
		}
	}()

	os.Unsetenv("NO_COLOR")
	styles := gogh.NewStyles()

	// With color enabled, output contains the original text.
	// Note: lipgloss strips ANSI codes in non-TTY environments (like tests),
	// so we can only verify the text content is preserved.
	result := styles.Key("TEST-123")
	assert.Contains(t, result, "TEST-123")

	result = styles.Status("Open")
	assert.Contains(t, result, "Open")

	result = styles.Error("Error message")
	assert.Contains(t, result, "Error message")

	result = styles.Label("my-label")
	assert.Contains(t, result, "my-label")

	result = styles.Header("Header Text")
	assert.Contains(t, result, "Header Text")
}
