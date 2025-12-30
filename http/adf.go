package http

import "strings"

// TextToADF converts plain text to Atlassian Document Format (ADF).
// The result can be used directly in Jira API requests for description and comment fields.
// Double newlines (\n\n) become separate paragraphs; single newlines (\n) become hardBreak nodes.
func TextToADF(text string) map[string]any {
	// Split by double newlines to get paragraphs
	paragraphs := strings.Split(text, "\n\n")

	content := make([]any, 0, len(paragraphs))
	for _, para := range paragraphs {
		// Skip empty paragraphs
		if para == "" {
			continue
		}

		paraContent := buildParagraphContent(para)
		content = append(content, map[string]any{
			"type":    "paragraph",
			"content": paraContent,
		})
	}

	// Handle empty text case
	if len(content) == 0 {
		content = []any{
			map[string]any{
				"type": "paragraph",
				"content": []any{
					map[string]any{
						"type": "text",
						"text": "",
					},
				},
			},
		}
	}

	return map[string]any{
		"type":    "doc",
		"version": 1,
		"content": content,
	}
}

// buildParagraphContent converts a paragraph string into ADF content nodes,
// using hardBreak nodes for single newlines.
func buildParagraphContent(para string) []any {
	lines := strings.Split(para, "\n")

	// Each line becomes a text node, with hardBreak nodes between them
	content := make([]any, 0, len(lines)*2-1)
	for i, line := range lines {
		if i > 0 {
			content = append(content, map[string]any{
				"type": "hardBreak",
			})
		}
		content = append(content, map[string]any{
			"type": "text",
			"text": line,
		})
	}

	return content
}

// ADFToText extracts plain text from an Atlassian Document Format (ADF) document.
// It recursively traverses the document structure and concatenates all text nodes.
// Paragraphs are separated by double newlines; hardBreak nodes become single newlines.
func ADFToText(adf map[string]any) string {
	if adf == nil {
		return ""
	}
	return adfToTextNode(adf)
}

// adfToTextNode recursively extracts text from an ADF node.
func adfToTextNode(node map[string]any) string {
	nodeType, _ := node["type"].(string)

	// Handle leaf nodes
	switch nodeType {
	case "text":
		text, _ := node["text"].(string)
		return text
	case "hardBreak":
		return "\n"
	}

	// Recursively process content array
	content, ok := node["content"].([]any)
	if !ok {
		return ""
	}

	var result string
	for i, item := range content {
		child, ok := item.(map[string]any)
		if !ok {
			continue
		}

		childType, _ := child["type"].(string)
		isBlockNode := childType == "paragraph"

		// Add separator before block nodes (except first)
		if isBlockNode && i > 0 {
			result += "\n\n"
		}

		result += adfToTextNode(child)
	}

	return result
}
