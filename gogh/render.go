package gogh

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// RenderCard creates a bordered card (or plain section in text-only mode).
func RenderCard(s *Styles, title, content string) string {
	if s.NoColor {
		return renderTextCard(title, content, s.Width)
	}
	return renderColorCard(s, title, content)
}

// RenderCardWithStyledTitle creates a bordered card with an already-styled title.
// This allows the title to have ANSI styling (e.g., colored issue keys).
func RenderCardWithStyledTitle(s *Styles, styledTitle, content string) string {
	if s.NoColor {
		// In text mode, strip styling - use plain title if available
		return renderTextCard(styledTitle, content, s.Width)
	}
	return renderColorCardWithStyledTitle(s, styledTitle, content)
}

func renderColorCard(s *Styles, title, content string) string {
	if title == "" {
		cardStyle := s.Card.Border.Width(s.Width - 2)
		return cardStyle.Render(content)
	}
	// For plain titles, render without special styling
	return renderColorCardWithTitle(s, title, "", content)
}

func renderColorCardWithStyledTitle(s *Styles, styledTitle, content string) string {
	// For styled titles, pass both the styled and plain versions
	return renderColorCardWithTitle(s, styledTitle, styledTitle, content)
}

func renderColorCardWithTitle(s *Styles, title, styledTitle, content string) string {
	cardStyle := s.Card.Border.
		Width(s.Width - 2) // account for border

	// Custom border top with title
	cardStyle = cardStyle.BorderTop(false)

	// Calculate width using the styled title (accounts for ANSI codes)
	displayTitle := title
	if styledTitle != "" {
		displayTitle = styledTitle
	}

	// Title section: "─ TITLE " with proper width calculation
	titleSection := " " + displayTitle + " "
	titleWidth := lipgloss.Width(titleSection) + 2 // +2 for "╭─" prefix
	repeatCount := s.Width - titleWidth - 1        // -1 for "╮" suffix
	if repeatCount < 0 {
		repeatCount = 0
	}

	// Style border characters to match the card border color
	borderStyle := s.Renderer.NewStyle().Foreground(s.Theme.Border)
	topBorder := borderStyle.Render("╭─") + titleSection + borderStyle.Render(strings.Repeat("─", repeatCount)+"╮")

	// Add blank line before content for visual spacing (matches text mode)
	return topBorder + "\n" + cardStyle.Render("\n"+content)
}

func renderTextCard(title, content string, width int) string {
	if title == "" {
		return content
	}

	// Format: === TITLE ===...
	titlePart := "=== " + title + " "
	remaining := width - lipgloss.Width(titlePart)
	if remaining < 3 {
		remaining = 3
	}
	header := titlePart + strings.Repeat("=", remaining)

	return header + "\n\n" + content
}

// RenderStatusBadge formats status with appropriate indicator.
func RenderStatusBadge(s *Styles, status string) string {
	indicator := s.Indicators.StatusToDo
	style := s.Badge.ToDo

	switch strings.ToLower(status) {
	case "done":
		indicator = s.Indicators.StatusDone
		style = s.Badge.Done
	case "in progress":
		indicator = s.Indicators.StatusInProgress
		style = s.Badge.InProgress
	}

	if s.NoColor {
		return indicator + " " + status
	}
	return style.Render(indicator + " " + status)
}

// RenderPriorityBadge formats priority with appropriate indicator.
func RenderPriorityBadge(s *Styles, priority string) string {
	var indicator string
	if s.NoColor {
		indicator = getASCIIPriorityIndicator(priority)
	} else {
		indicator = getUnicodePriorityIndicator(priority)
	}

	if s.NoColor {
		return indicator + " " + priority
	}
	return s.Renderer.NewStyle().Foreground(getPriorityColor(s, priority)).Render(indicator + " " + priority)
}

func getUnicodePriorityIndicator(priority string) string {
	switch strings.ToLower(priority) {
	case "highest":
		return "▲▲▲"
	case "high":
		return "▲▲"
	case "medium":
		return "▲"
	case "low":
		return "▽"
	case "lowest":
		return "▽▽"
	default:
		return "▲"
	}
}

func getASCIIPriorityIndicator(priority string) string {
	switch strings.ToLower(priority) {
	case "highest":
		return "[!!!]"
	case "high":
		return "[!!]"
	case "medium":
		return "[!]"
	case "low":
		return "[-]"
	case "lowest":
		return "[--]"
	default:
		return "[!]"
	}
}

func getPriorityColor(s *Styles, priority string) lipgloss.AdaptiveColor {
	switch strings.ToLower(priority) {
	case "highest":
		return s.Theme.PriorityHighest
	case "high":
		return s.Theme.PriorityHigh
	case "medium":
		return s.Theme.PriorityMedium
	case "low", "lowest":
		return s.Theme.PriorityLow
	default:
		return s.Theme.PriorityMedium
	}
}

// RenderSeparator renders a separator line of the given kind and width.
func RenderSeparator(s *Styles, kind string, width int) string {
	char := s.Separators.Solid
	if kind == "dotted" {
		char = s.Separators.Dotted
	}
	return strings.Repeat(char, width)
}
