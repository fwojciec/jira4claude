package adf

import (
	"fmt"
	"strings"
)

// toMarkdown converts an Atlassian Document Format (ADF) document to GitHub-flavored markdown.
// This is useful for displaying Jira issue content in a readable format.
// Returns an error if any elements were skipped during conversion.
func toMarkdown(adfDoc map[string]any) (string, error) {
	if adfDoc == nil {
		return "", nil
	}

	content, ok := adfDoc["content"].([]any)
	if !ok || len(content) == 0 {
		return "", nil
	}

	skipped := newSkippedCollector()
	var parts []string
	for _, item := range content {
		node, ok := item.(map[string]any)
		if !ok {
			continue
		}
		part := adfNodeToGFM(node, "", skipped)
		if part != "" {
			parts = append(parts, part)
		}
	}

	return strings.Join(parts, "\n\n"), skipped.error()
}

// adfNodeToGFM converts a single ADF node to markdown.
// The prefix is used for nested contexts like blockquotes.
func adfNodeToGFM(node map[string]any, prefix string, skipped *skippedCollector) string {
	nodeType, _ := node["type"].(string)

	switch nodeType {
	case "paragraph":
		return prefix + adfInlineToGFM(node)
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
	case "hardBreak":
		return "\n"
	default:
		// Record the skipped node type
		skipped.add(nodeType)
		return ""
	}
}

// adfHeadingToGFM converts an ADF heading to markdown.
func adfHeadingToGFM(node map[string]any) string {
	level := 1
	if attrs, ok := node["attrs"].(map[string]any); ok {
		if l, ok := attrs["level"].(int); ok {
			level = l
		} else if l, ok := attrs["level"].(float64); ok {
			level = int(l)
		}
	}

	text := adfInlineToGFM(node)
	return strings.Repeat("#", level) + " " + text
}

// adfCodeBlockToGFM converts an ADF codeBlock to a fenced code block.
func adfCodeBlockToGFM(node map[string]any) string {
	lang := ""
	if attrs, ok := node["attrs"].(map[string]any); ok {
		if l, ok := attrs["language"].(string); ok {
			lang = l
		}
	}

	var code string
	if content, ok := node["content"].([]any); ok {
		for _, item := range content {
			if textNode, ok := item.(map[string]any); ok {
				if textNode["type"] == "text" {
					if t, ok := textNode["text"].(string); ok {
						code += t
					}
				}
			}
		}
	}

	return fmt.Sprintf("```%s\n%s\n```", lang, code)
}

// adfBulletListToGFM converts an ADF bulletList to markdown.
func adfBulletListToGFM(node map[string]any, skipped *skippedCollector) string {
	content, ok := node["content"].([]any)
	if !ok {
		return ""
	}

	items := make([]string, 0, len(content))
	for _, item := range content {
		listItem, ok := item.(map[string]any)
		if !ok || listItem["type"] != "listItem" {
			continue
		}
		itemText := adfListItemToGFM(listItem, skipped)
		items = append(items, "- "+itemText)
	}

	return strings.Join(items, "\n")
}

// adfOrderedListToGFM converts an ADF orderedList to markdown.
func adfOrderedListToGFM(node map[string]any, skipped *skippedCollector) string {
	content, ok := node["content"].([]any)
	if !ok {
		return ""
	}

	items := make([]string, 0, len(content))
	for i, item := range content {
		listItem, ok := item.(map[string]any)
		if !ok || listItem["type"] != "listItem" {
			continue
		}
		itemText := adfListItemToGFM(listItem, skipped)
		items = append(items, fmt.Sprintf("%d. %s", i+1, itemText))
	}

	return strings.Join(items, "\n")
}

// adfListItemToGFM extracts the text content from a list item.
func adfListItemToGFM(node map[string]any, skipped *skippedCollector) string {
	content, ok := node["content"].([]any)
	if !ok || len(content) == 0 {
		return ""
	}

	// List items typically contain paragraphs or nested lists
	parts := make([]string, 0, len(content))
	for _, item := range content {
		child, ok := item.(map[string]any)
		if !ok {
			continue
		}
		part := adfNodeToGFM(child, "", skipped)
		if part != "" {
			parts = append(parts, part)
		}
	}

	return strings.Join(parts, " ")
}

// adfBlockquoteToGFM converts an ADF blockquote to markdown.
func adfBlockquoteToGFM(node map[string]any, skipped *skippedCollector) string {
	content, ok := node["content"].([]any)
	if !ok {
		return ""
	}

	var lines []string
	for _, item := range content {
		child, ok := item.(map[string]any)
		if !ok {
			continue
		}
		text := adfNodeToGFM(child, "", skipped)
		// Prefix each line with >
		for _, line := range strings.Split(text, "\n") {
			lines = append(lines, "> "+line)
		}
	}

	return strings.Join(lines, "\n")
}

// adfInlineToGFM converts inline content to markdown.
func adfInlineToGFM(node map[string]any) string {
	content, ok := node["content"].([]any)
	if !ok {
		return ""
	}

	var result strings.Builder
	for _, item := range content {
		textNode, ok := item.(map[string]any)
		if !ok {
			continue
		}

		if textNode["type"] == "hardBreak" {
			result.WriteString("\n")
			continue
		}

		if textNode["type"] != "text" {
			continue
		}

		text, _ := textNode["text"].(string)
		marks, hasMarks := textNode["marks"].([]any)

		if !hasMarks || len(marks) == 0 {
			result.WriteString(text)
			continue
		}

		// Apply marks
		result.WriteString(applyMarks(text, marks))
	}

	return result.String()
}

// applyMarks wraps text with the appropriate markdown syntax for its marks.
func applyMarks(text string, marks []any) string {
	var hasStrong, hasEm, hasCode bool
	var linkHref string

	for _, mark := range marks {
		markMap, ok := mark.(map[string]any)
		if !ok {
			continue
		}
		markType, _ := markMap["type"].(string)
		switch markType {
		case "strong":
			hasStrong = true
		case "em":
			hasEm = true
		case "code":
			hasCode = true
		case "link":
			if attrs, ok := markMap["attrs"].(map[string]any); ok {
				if href, ok := attrs["href"].(string); ok {
					linkHref = href
				}
			}
		}
	}

	// Apply marks in specific order.
	// If code is present, skip em/strong since markdown doesn't support emphasis inside backticks.
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
	}
	if linkHref != "" {
		result = "[" + result + "](" + linkHref + ")"
	}

	return result
}
