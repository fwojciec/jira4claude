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

func renderColorCard(s *Styles, title, content string) string {
	var titleLine string
	if title != "" {
		titleLine = "─ " + title + " "
	}

	cardStyle := s.Card.Border.
		Width(s.Width - 2) // account for border

	if title != "" {
		// Custom border top with title
		cardStyle = cardStyle.BorderTop(false)
		titleWidth := lipgloss.Width(titleLine)
		repeatCount := s.Width - titleWidth - 3
		if repeatCount < 0 {
			repeatCount = 0
		}
		topBorder := "╭" + titleLine + strings.Repeat("─", repeatCount) + "╮"
		return topBorder + "\n" + cardStyle.Render(content)
	}

	return cardStyle.Render(content)
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
	return s.Renderer.NewStyle().Foreground(getPriorityColor(priority)).Render(indicator + " " + priority)
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

func getPriorityColor(priority string) lipgloss.Color {
	switch strings.ToLower(priority) {
	case "highest":
		return lipgloss.Color("9") // red
	case "high":
		return lipgloss.Color("208") // orange
	case "medium":
		return lipgloss.Color("11") // yellow
	case "low", "lowest":
		return lipgloss.Color("8") // gray
	default:
		return lipgloss.Color("11")
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
