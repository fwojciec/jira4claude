package markdown

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/fwojciec/jira4claude"
)

// toMarkdown converts an Atlassian Document Format (ADF) document to GitHub-flavored markdown.
// This is useful for displaying Jira issue content in a readable format.
// Returns warnings for any elements that were skipped during conversion.
func toMarkdown(adfDoc *jira4claude.ADFNode) (string, []string) {
	if adfDoc == nil {
		return "", nil
	}

	if len(adfDoc.Content) == 0 {
		return "", nil
	}

	skipped := newSkippedCollector()
	var parts []string
	for _, node := range adfDoc.Content {
		part := adfNodeToGFM(&node, skipped)
		if part != "" {
			parts = append(parts, part)
		}
	}

	return strings.Join(parts, "\n\n"), skipped.warnings()
}

// adfNodeToGFM converts a single ADF node to markdown.
func adfNodeToGFM(node *jira4claude.ADFNode, skipped *skippedCollector) string {
	switch node.Type {
	case "paragraph":
		return adfInlineToGFM(node)
	case "heading":
		return adfHeadingToGFM(node)
	case "codeBlock":
		return adfCodeBlockToGFM(node)
	case "bulletList":
		return adfBulletListToGFM(node, skipped)
	case "orderedList":
		return adfOrderedListToGFM(node, skipped)
	case "blockquote":
		return adfBlockquoteToGFM(node, skipped)
	case "rule":
		return "---"
	case "table":
		return adfTableToGFM(node, skipped)
	case "taskList":
		return adfTaskListToGFM(node, skipped)
	case "hardBreak":
		return "\n"
	case "text":
		// Jira sometimes places bare "text" nodes at block level (e.g., inside listItem
		// without a wrapping paragraph). Reuse the inline rendering path so any marks
		// on the text node (strong/em/code/link) are preserved.
		wrapper := &jira4claude.ADFNode{
			Content: []jira4claude.ADFNode{*node},
		}
		return adfInlineToGFM(wrapper)
	default:
		// Record the skipped node type
		skipped.add(node.Type)
		return ""
	}
}

// adfHeadingToGFM converts an ADF heading to markdown.
func adfHeadingToGFM(node *jira4claude.ADFNode) string {
	level := 1
	if node.Attrs != nil {
		var attrs map[string]any
		if err := json.Unmarshal(node.Attrs, &attrs); err == nil {
			if l, ok := attrs["level"].(float64); ok {
				level = int(l)
			}
		}
	}

	text := adfInlineToGFM(node)
	return strings.Repeat("#", level) + " " + text
}

// adfCodeBlockToGFM converts an ADF codeBlock to a fenced code block.
func adfCodeBlockToGFM(node *jira4claude.ADFNode) string {
	lang := ""
	if node.Attrs != nil {
		var attrs map[string]any
		if err := json.Unmarshal(node.Attrs, &attrs); err == nil {
			if l, ok := attrs["language"].(string); ok {
				lang = l
			}
		}
	}

	var code string
	for _, child := range node.Content {
		if child.Type == "text" {
			code += child.Text
		}
	}

	return fmt.Sprintf("```%s\n%s\n```", lang, code)
}

// adfBulletListToGFM converts an ADF bulletList to markdown.
func adfBulletListToGFM(node *jira4claude.ADFNode, skipped *skippedCollector) string {
	items := make([]string, 0, len(node.Content))
	for i := range node.Content {
		child := &node.Content[i]
		if child.Type != "listItem" {
			continue
		}
		itemText := adfListItemToGFM(child, skipped)
		items = append(items, "- "+itemText)
	}

	return strings.Join(items, "\n")
}

// adfOrderedListToGFM converts an ADF orderedList to markdown.
func adfOrderedListToGFM(node *jira4claude.ADFNode, skipped *skippedCollector) string {
	items := make([]string, 0, len(node.Content))
	num := 0
	for i := range node.Content {
		child := &node.Content[i]
		if child.Type != "listItem" {
			continue
		}
		num++
		itemText := adfListItemToGFM(child, skipped)
		items = append(items, fmt.Sprintf("%d. %s", num, itemText))
	}

	return strings.Join(items, "\n")
}

// adfListItemToGFM extracts the text content from a list item.
func adfListItemToGFM(node *jira4claude.ADFNode, skipped *skippedCollector) string {
	if len(node.Content) == 0 {
		return ""
	}

	// List items typically contain paragraphs or nested lists
	parts := make([]string, 0, len(node.Content))
	for i := range node.Content {
		child := &node.Content[i]
		part := adfNodeToGFM(child, skipped)
		if part != "" {
			parts = append(parts, part)
		}
	}

	return strings.Join(parts, " ")
}

// adfBlockquoteToGFM converts an ADF blockquote to markdown.
func adfBlockquoteToGFM(node *jira4claude.ADFNode, skipped *skippedCollector) string {
	var lines []string
	for i := range node.Content {
		child := &node.Content[i]
		text := adfNodeToGFM(child, skipped)
		// Prefix each line with >
		for _, line := range strings.Split(text, "\n") {
			lines = append(lines, "> "+line)
		}
	}

	return strings.Join(lines, "\n")
}

// adfTableToGFM converts an ADF table to a GFM pipe table.
func adfTableToGFM(node *jira4claude.ADFNode, skipped *skippedCollector) string {
	if len(node.Content) == 0 {
		return ""
	}

	var lines []string
	hasHeader := false
	firstRendered := true
	maxCols := 0

	for i := range node.Content {
		rowNode := &node.Content[i]
		if rowNode.Type != "tableRow" {
			continue
		}

		// Check if the first rendered row contains header cells
		isHeaderRow := false
		if firstRendered && len(rowNode.Content) > 0 {
			firstRendered = false
			isHeaderRow = rowNode.Content[0].Type == "tableHeader"
		}

		var cellTexts []string
		for j := range rowNode.Content {
			cellNode := &rowNode.Content[j]
			cellTexts = append(cellTexts, adfTableCellToGFM(cellNode, skipped))
		}

		if len(cellTexts) > maxCols {
			maxCols = len(cellTexts)
		}

		lines = append(lines, "| "+strings.Join(cellTexts, " | ")+" |")

		if isHeaderRow {
			hasHeader = true
			seps := make([]string, len(cellTexts))
			for j := range seps {
				seps[j] = "---"
			}
			lines = append(lines, "| "+strings.Join(seps, " | ")+" |")
		}
	}

	// If no header row, synthesize an empty header + separator
	if !hasHeader && maxCols > 0 {
		empties := make([]string, maxCols)
		seps := make([]string, maxCols)
		for j := range seps {
			seps[j] = "---"
		}
		headerLine := "| " + strings.Join(empties, " | ") + " |"
		sepLine := "| " + strings.Join(seps, " | ") + " |"
		lines = append([]string{headerLine, sepLine}, lines...)
	}

	return strings.Join(lines, "\n")
}

// adfTableCellToGFM extracts text from a table cell (tableHeader or tableCell).
func adfTableCellToGFM(node *jira4claude.ADFNode, skipped *skippedCollector) string {
	var parts []string
	for i := range node.Content {
		child := &node.Content[i]
		part := adfNodeToGFM(child, skipped)
		if part != "" {
			parts = append(parts, part)
		}
	}

	text := strings.Join(parts, " ")
	// Normalize newlines to spaces so cell content cannot break GFM table rows.
	text = strings.ReplaceAll(text, "\n", " ")
	// Escape pipe characters to prevent breaking GFM table column alignment.
	return strings.ReplaceAll(text, "|", "\\|")
}

// adfTaskListToGFM converts an ADF taskList to GFM task list items.
func adfTaskListToGFM(node *jira4claude.ADFNode, skipped *skippedCollector) string {
	var items []string
	for i := range node.Content {
		taskItem := &node.Content[i]
		if taskItem.Type != "taskItem" {
			continue
		}

		checkbox := "[ ]"
		if taskItem.Attrs != nil {
			var attrs map[string]any
			if err := json.Unmarshal(taskItem.Attrs, &attrs); err == nil {
				if state, ok := attrs["state"].(string); ok && state == "DONE" {
					checkbox = "[x]"
				}
			}
		}

		text := adfListItemToGFM(taskItem, skipped)
		items = append(items, "- "+checkbox+" "+text)
	}

	return strings.Join(items, "\n")
}

// adfInlineToGFM converts inline content to markdown.
func adfInlineToGFM(node *jira4claude.ADFNode) string {
	if len(node.Content) == 0 {
		return ""
	}

	var result strings.Builder
	for _, child := range node.Content {
		if child.Type == "hardBreak" {
			result.WriteString("\n")
			continue
		}

		if child.Type != "text" {
			continue
		}

		if len(child.Marks) == 0 {
			result.WriteString(child.Text)
			continue
		}

		// Apply marks
		result.WriteString(applyMarks(child.Text, child.Marks))
	}

	return result.String()
}

// applyMarks wraps text with the appropriate markdown syntax for its marks.
func applyMarks(text string, marks []jira4claude.ADFMark) string {
	var hasStrong, hasEm, hasCode, hasStrike bool
	var linkHref string

	for _, mark := range marks {
		switch mark.Type {
		case "strong":
			hasStrong = true
		case "em":
			hasEm = true
		case "code":
			hasCode = true
		case "strike":
			hasStrike = true
		case "link":
			if mark.Attrs != nil {
				var attrs map[string]any
				if err := json.Unmarshal(mark.Attrs, &attrs); err == nil {
					if href, ok := attrs["href"].(string); ok {
						linkHref = href
					}
				}
			}
		}
	}

	// Apply marks in specific order.
	// If code is present, skip em/strong/strike since markdown doesn't support emphasis inside backticks.
	result := text

	if hasCode {
		result = "`" + result + "`"
	} else {
		if hasEm && hasStrong {
			result = "***" + result + "***"
		} else {
			if hasEm {
				result = "*" + result + "*"
			}
			if hasStrong {
				result = "**" + result + "**"
			}
		}
		if hasStrike {
			result = "~~" + result + "~~"
		}
	}
	if linkHref != "" && result != linkHref {
		result = "[" + result + "](" + linkHref + ")"
	}

	return result
}
