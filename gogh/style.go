package gogh

import (
	"os"

	"github.com/charmbracelet/lipgloss"
)

// Styles provides text styling for terminal output.
type Styles struct {
	noColor bool
	key     lipgloss.Style
	status  lipgloss.Style
	err     lipgloss.Style
	label   lipgloss.Style
	header  lipgloss.Style
}

// NewStyles creates styles respecting NO_COLOR environment variable.
func NewStyles() *Styles {
	noColor := os.Getenv("NO_COLOR") != ""

	return &Styles{
		noColor: noColor,
		key:     lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12")),
		status:  lipgloss.NewStyle().Foreground(lipgloss.Color("10")),
		err:     lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("9")),
		label:   lipgloss.NewStyle().Foreground(lipgloss.Color("14")),
		header:  lipgloss.NewStyle().Bold(true).Underline(true),
	}
}

// Key styles an issue key.
func (s *Styles) Key(text string) string {
	if s.noColor {
		return text
	}
	return s.key.Render(text)
}

// Status styles a status value.
func (s *Styles) Status(text string) string {
	if s.noColor {
		return text
	}
	return s.status.Render(text)
}

// Error styles an error message.
func (s *Styles) Error(text string) string {
	if s.noColor {
		return text
	}
	return s.err.Render(text)
}

// Label styles a label.
func (s *Styles) Label(text string) string {
	if s.noColor {
		return text
	}
	return s.label.Render(text)
}

// Header styles a header.
func (s *Styles) Header(text string) string {
	if s.noColor {
		return text
	}
	return s.header.Render(text)
}

// NoColor returns whether color output is disabled.
func (s *Styles) NoColor() bool {
	return s.noColor
}
