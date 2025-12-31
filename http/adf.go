package http

import (
	"encoding/json"
	"strings"
)

// textOrADF converts text to ADF, but if the text is already a valid ADF JSON
// document (starts with `{` and has a "type":"doc" field), it parses and returns
// that directly. This allows the CLI to pre-convert markdown to ADF and pass it
// through without double-conversion.
func textOrADF(text string) map[string]any {
	if adf := tryParseADF(text); adf != nil {
		return adf
	}
	return TextToADF(text)
}

// tryParseADF attempts to parse text as an ADF JSON document.
// Returns the parsed ADF if successful, nil otherwise.
func tryParseADF(text string) map[string]any {
	// Quick check: must start with { to be valid JSON object
	if len(text) == 0 || text[0] != '{' {
		return nil
	}

	var adf map[string]any
	if err := json.Unmarshal([]byte(text), &adf); err != nil {
		return nil
	}

	// Verify this is an ADF document (has "type": "doc")
	if docType, ok := adf["type"].(string); ok && docType == "doc" {
		return adf
	}

	return nil
}

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
