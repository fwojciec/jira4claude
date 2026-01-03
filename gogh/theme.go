package gogh

import (
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/glamour/ansi"
	"github.com/charmbracelet/glamour/styles"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// Theme defines the color palette for terminal output.
type Theme struct {
	Primary lipgloss.AdaptiveColor
	Success lipgloss.AdaptiveColor
	Warning lipgloss.AdaptiveColor
	Error   lipgloss.AdaptiveColor
	Muted   lipgloss.AdaptiveColor
	Label   lipgloss.AdaptiveColor
	Border  lipgloss.AdaptiveColor

	// Priority colors (semantic: urgency level)
	PriorityHighest lipgloss.AdaptiveColor
	PriorityHigh    lipgloss.AdaptiveColor
	PriorityMedium  lipgloss.AdaptiveColor
	PriorityLow     lipgloss.AdaptiveColor
}

// Indicators contains status and message indicators that vary by mode.
type Indicators struct {
	StatusDone       string
	StatusInProgress string
	StatusToDo       string
	Arrow            string
	Success          string
	Warning          string
	Error            string
}

// Separators contains separator characters that vary by mode.
type Separators struct {
	Dotted string
	Solid  string
}

// CardStyles contains styles for card rendering.
type CardStyles struct {
	Border lipgloss.Style
}

// BadgeStyles contains styles for status badges.
type BadgeStyles struct {
	Done       lipgloss.Style
	InProgress lipgloss.Style
	ToDo       lipgloss.Style
}

// Styles contains all application styles for terminal output.
type Styles struct {
	Theme      Theme
	NoColor    bool
	Width      int
	Renderer   *lipgloss.Renderer
	Card       CardStyles
	Badge      BadgeStyles
	Indicators Indicators
	Separators Separators
}

// NewStyles creates styles based on the renderer's color profile.
func NewStyles(r *lipgloss.Renderer) *Styles {
	profile := r.ColorProfile()
	noColor := profile == termenv.Ascii

	s := &Styles{
		Renderer: r,
		NoColor:  noColor,
		Width:    80,
		Theme: Theme{
			Primary: lipgloss.AdaptiveColor{Light: "12", Dark: "12"},
			Success: lipgloss.AdaptiveColor{Light: "10", Dark: "10"},
			Warning: lipgloss.AdaptiveColor{Light: "11", Dark: "11"},
			Error:   lipgloss.AdaptiveColor{Light: "9", Dark: "9"},
			Muted:   lipgloss.AdaptiveColor{Light: "8", Dark: "8"},
			Label:   lipgloss.AdaptiveColor{Light: "14", Dark: "14"},
			Border:  lipgloss.AdaptiveColor{Light: "240", Dark: "240"},

			PriorityHighest: lipgloss.AdaptiveColor{Light: "9", Dark: "9"},     // red
			PriorityHigh:    lipgloss.AdaptiveColor{Light: "208", Dark: "208"}, // orange
			PriorityMedium:  lipgloss.AdaptiveColor{Light: "11", Dark: "11"},   // yellow
			PriorityLow:     lipgloss.AdaptiveColor{Light: "8", Dark: "8"},     // gray
		},
	}

	// Select indicators based on mode
	if noColor {
		s.Indicators = Indicators{
			StatusDone:       "[x]",
			StatusInProgress: "[>]",
			StatusToDo:       "[ ]",
			Arrow:            "->",
			Success:          "[ok]",
			Warning:          "[warn]",
			Error:            "[error]",
		}
		s.Separators = Separators{
			Dotted: ".",
			Solid:  "-",
		}
	} else {
		s.Indicators = Indicators{
			StatusDone:       "✓",
			StatusInProgress: "▶",
			StatusToDo:       "○",
			Arrow:            "→",
			Success:          "✓",
			Warning:          "⚠",
			Error:            "✗",
		}
		s.Separators = Separators{
			Dotted: "┄",
			Solid:  "─",
		}
	}

	// Configure card styles
	if noColor {
		s.Card = CardStyles{
			Border: r.NewStyle(),
		}
	} else {
		s.Card = CardStyles{
			Border: r.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(s.Theme.Border),
		}
	}

	// Configure badge styles
	s.Badge = BadgeStyles{
		Done:       r.NewStyle().Foreground(s.Theme.Success),
		InProgress: r.NewStyle().Foreground(s.Theme.Primary),
		ToDo:       r.NewStyle().Foreground(s.Theme.Muted),
	}

	return s
}

// DefaultStyles creates styles for stdout with auto-detection.
func DefaultStyles() *Styles {
	return NewStyles(lipgloss.DefaultRenderer())
}

// Backward-compatible styling methods used by TextPrinter.
// These will be removed or refactored in J4C-86/87/88.

// Key styles an issue key.
func (s *Styles) Key(text string) string {
	if s.NoColor {
		return text
	}
	return s.Renderer.NewStyle().Bold(true).Foreground(s.Theme.Primary).Render(text)
}

// Status styles a status value.
func (s *Styles) Status(text string) string {
	if s.NoColor {
		return text
	}
	return s.Renderer.NewStyle().Foreground(s.Theme.Success).Render(text)
}

// Error styles an error message.
func (s *Styles) Error(text string) string {
	if s.NoColor {
		return text
	}
	return s.Renderer.NewStyle().Bold(true).Foreground(s.Theme.Error).Render(text)
}

// Warning styles a warning message.
func (s *Styles) Warning(msg string) string {
	if s.NoColor {
		return "warning: " + msg
	}
	return s.Renderer.NewStyle().Foreground(s.Theme.Warning).Render("warning: " + msg)
}

// Label styles a label.
func (s *Styles) Label(text string) string {
	if s.NoColor {
		return text
	}
	return s.Renderer.NewStyle().Foreground(s.Theme.Label).Render(text)
}

// Header styles a header.
func (s *Styles) Header(text string) string {
	if s.NoColor {
		return text
	}
	return s.Renderer.NewStyle().Bold(true).Underline(true).Render(text)
}

// IsNoColor returns whether color output is disabled.
// Deprecated: Use NoColor field directly.
func (s *Styles) IsNoColor() bool {
	return s.NoColor
}

// RenderMarkdown renders markdown text with appropriate styling.
// In color mode, uses a custom style based on glamour's dark theme with body
// text using terminal default colors for consistency with the CLI's styling.
// Headers, code blocks, and other elements retain their distinct styling.
// In text-only mode (NO_COLOR), uses ascii style with no ANSI codes.
// Word wrapping is applied at Width columns (default 80).
//
// Note: Creates a new renderer per call for simplicity. This is acceptable
// for single-issue views but could be optimized if used in batch operations.
func (s *Styles) RenderMarkdown(input string) (string, error) {
	if input == "" {
		return "", nil
	}

	var opts []glamour.TermRendererOption

	if s.NoColor {
		opts = append(opts, glamour.WithStyles(noColorMarkdownStyle()))
	} else {
		opts = append(opts, glamour.WithStyles(markdownStyle()))
	}

	opts = append(opts, glamour.WithWordWrap(s.Width))

	r, err := glamour.NewTermRenderer(opts...)
	if err != nil {
		return "", err
	}

	return r.Render(input)
}

// markdownStyle returns a custom glamour style based on the dark theme
// with body text using terminal default colors instead of explicit colors.
// This ensures consistency with the CLI's styling while preserving
// syntax highlighting for code blocks and styling for other elements.
//
// Note: We use DarkStyleConfig as the base rather than auto-detecting
// terminal background because the CLI's Theme colors use the same values
// for both light and dark modes (see AdaptiveColor definitions above).
func markdownStyle() ansi.StyleConfig {
	style := styles.DarkStyleConfig

	// Set 3-space margin to align with card panel content
	// Card layout: │ (col 0) + 2 spaces padding = content at column 3
	cardAlignMargin := uint(3)
	style.Document.Margin = &cardAlignMargin

	// Remove explicit colors from body text elements so they use terminal default
	// The Document style applies to the overall text wrapper
	style.Document.Color = nil
	style.Document.BackgroundColor = nil

	// Paragraph is the main body text element
	style.Paragraph.Color = nil
	style.Paragraph.BackgroundColor = nil

	// Text is for inline text within paragraphs
	style.Text.Color = nil
	style.Text.BackgroundColor = nil

	// List items should also use terminal default
	style.Item.Color = nil
	style.Item.BackgroundColor = nil

	// Enumeration (numbered lists)
	style.Enumeration.Color = nil
	style.Enumeration.BackgroundColor = nil

	return style
}

// noColorMarkdownStyle returns a custom glamour style for NO_COLOR mode
// based on the ASCII style but with zero margins to avoid indentation.
// This ensures description text starts at column 0 without awkward indentation.
func noColorMarkdownStyle() ansi.StyleConfig {
	style := styles.ASCIIStyleConfig

	// Remove document margin to eliminate the 2-space indentation
	style.Document.Margin = nil

	// Remove code block margin as well for consistency
	style.CodeBlock.Margin = nil

	return style
}
