package http

import (
	"fmt"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
)

// GFMToADF converts GitHub-flavored markdown to Atlassian Document Format (ADF).
// The result can be used directly in Jira API requests for description and comment fields.
func GFMToADF(markdown string) map[string]any {
	md := goldmark.New(
		goldmark.WithExtensions(extension.GFM),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
		),
	)

	reader := text.NewReader([]byte(markdown))
	doc := md.Parser().Parse(reader)

	content := convertNode(doc, []byte(markdown))
	if content == nil {
		content = []any{}
	}

	return map[string]any{
		"type":    "doc",
		"version": 1,
		"content": content,
	}
}

// convertNode recursively converts goldmark AST nodes to ADF nodes.
func convertNode(node ast.Node, source []byte) []any {
	var content []any

	for child := node.FirstChild(); child != nil; child = child.NextSibling() {
		adfNode := nodeToADF(child, source)
		if adfNode != nil {
			content = append(content, adfNode)
		}
	}

	return content
}

// nodeToADF converts a single goldmark AST node to an ADF node.
func nodeToADF(node ast.Node, source []byte) map[string]any {
	switch n := node.(type) {
	case *ast.Paragraph:
		return convertParagraph(n, source)
	case *ast.TextBlock:
		return convertTextBlock(n, source)
	case *ast.Heading:
		return convertHeading(n, source)
	case *ast.FencedCodeBlock:
		return convertFencedCodeBlock(n, source)
	case *ast.List:
		return convertList(n, source)
	case *ast.Blockquote:
		return convertBlockquote(n, source)
	default:
		return nil
	}
}

// convertParagraph converts a goldmark paragraph to an ADF paragraph.
func convertParagraph(node *ast.Paragraph, source []byte) map[string]any {
	content := convertInlineContent(node, source)
	if len(content) == 0 {
		return nil
	}
	return map[string]any{
		"type":    "paragraph",
		"content": content,
	}
}

// convertTextBlock converts a goldmark text block (used in tight lists) to an ADF paragraph.
func convertTextBlock(node *ast.TextBlock, source []byte) map[string]any {
	content := convertInlineContent(node, source)
	if len(content) == 0 {
		return nil
	}
	return map[string]any{
		"type":    "paragraph",
		"content": content,
	}
}

// convertHeading converts a goldmark heading to an ADF heading.
func convertHeading(node *ast.Heading, source []byte) map[string]any {
	content := convertInlineContent(node, source)
	return map[string]any{
		"type": "heading",
		"attrs": map[string]any{
			"level": node.Level,
		},
		"content": content,
	}
}

// convertFencedCodeBlock converts a goldmark fenced code block to an ADF codeBlock.
func convertFencedCodeBlock(node *ast.FencedCodeBlock, source []byte) map[string]any {
	var codeText string
	lines := node.Lines()
	for i := range lines.Len() {
		line := lines.At(i)
		codeText += string(line.Value(source))
	}
	// Remove trailing newline
	if len(codeText) > 0 && codeText[len(codeText)-1] == '\n' {
		codeText = codeText[:len(codeText)-1]
	}

	result := map[string]any{
		"type": "codeBlock",
		"content": []any{
			map[string]any{
				"type": "text",
				"text": codeText,
			},
		},
	}

	lang := string(node.Language(source))
	if lang != "" {
		result["attrs"] = map[string]any{
			"language": lang,
		}
	}

	return result
}

// convertList converts a goldmark list to an ADF bulletList or orderedList.
func convertList(node *ast.List, source []byte) map[string]any {
	listType := "bulletList"
	if node.IsOrdered() {
		listType = "orderedList"
	}

	var items []any
	for child := node.FirstChild(); child != nil; child = child.NextSibling() {
		if listItem, ok := child.(*ast.ListItem); ok {
			items = append(items, convertListItem(listItem, source))
		}
	}

	return map[string]any{
		"type":    listType,
		"content": items,
	}
}

// convertListItem converts a goldmark list item to an ADF listItem.
func convertListItem(node *ast.ListItem, source []byte) map[string]any {
	content := convertNode(node, source)
	return map[string]any{
		"type":    "listItem",
		"content": content,
	}
}

// convertBlockquote converts a goldmark blockquote to an ADF blockquote.
func convertBlockquote(node *ast.Blockquote, source []byte) map[string]any {
	content := convertNode(node, source)
	return map[string]any{
		"type":    "blockquote",
		"content": content,
	}
}

// convertInlineContent converts the inline content of a block node to ADF text nodes.
func convertInlineContent(node ast.Node, source []byte) []any {
	var content []any
	for child := node.FirstChild(); child != nil; child = child.NextSibling() {
		inlineNodes := convertInlineNode(child, source, nil)
		content = append(content, inlineNodes...)
	}
	return consolidateTextNodes(content)
}

// consolidateTextNodes merges adjacent text nodes with identical marks.
func consolidateTextNodes(nodes []any) []any {
	if len(nodes) == 0 {
		return nodes
	}

	var result []any
	for _, node := range nodes {
		nodeMap, ok := node.(map[string]any)
		if !ok {
			result = append(result, node)
			continue
		}

		if len(result) == 0 {
			result = append(result, node)
			continue
		}

		lastMap, ok := result[len(result)-1].(map[string]any)
		if !ok {
			result = append(result, node)
			continue
		}

		// Both must be text nodes
		if lastMap["type"] != "text" || nodeMap["type"] != "text" {
			result = append(result, node)
			continue
		}

		// Compare marks
		if !marksEqual(lastMap["marks"], nodeMap["marks"]) {
			result = append(result, node)
			continue
		}

		// Merge the text
		lastText, _ := lastMap["text"].(string)
		nodeText, _ := nodeMap["text"].(string)
		lastMap["text"] = lastText + nodeText
	}

	return result
}

// marksEqual compares two marks slices for equality.
func marksEqual(a, b any) bool {
	aSlice, aOk := a.([]any)
	bSlice, bOk := b.([]any)

	// Both nil or both missing
	if !aOk && !bOk {
		return true
	}
	// One is nil, other is not
	if !aOk || !bOk {
		return false
	}
	// Different lengths
	if len(aSlice) != len(bSlice) {
		return false
	}
	// Compare each mark
	for i := range aSlice {
		aMap, aMapOk := aSlice[i].(map[string]any)
		bMap, bMapOk := bSlice[i].(map[string]any)
		if !aMapOk || !bMapOk {
			return false
		}
		if !mapEqual(aMap, bMap) {
			return false
		}
	}
	return true
}

// mapEqual compares two maps for equality (shallow comparison for mark comparison).
func mapEqual(a, b map[string]any) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		bv, ok := b[k]
		if !ok {
			return false
		}
		// Handle nested maps (for attrs)
		aMap, aIsMap := v.(map[string]any)
		bMap, bIsMap := bv.(map[string]any)
		if aIsMap && bIsMap {
			if !mapEqual(aMap, bMap) {
				return false
			}
			continue
		}
		if v != bv {
			return false
		}
	}
	return true
}

// convertInlineNode converts inline nodes (text, emphasis, etc.) to ADF text nodes.
func convertInlineNode(node ast.Node, source []byte, marks []map[string]any) []any {
	switch n := node.(type) {
	case *ast.Text:
		text := string(n.Segment.Value(source))
		if text == "" {
			return nil
		}
		result := map[string]any{
			"type": "text",
			"text": text,
		}
		if len(marks) > 0 {
			marksCopy := make([]any, len(marks))
			for i, m := range marks {
				marksCopy[i] = m
			}
			result["marks"] = marksCopy
		}
		return []any{result}

	case *ast.Emphasis:
		markType := "em"
		if n.Level == 2 {
			markType = "strong"
		}
		newMark := map[string]any{"type": markType}
		newMarks := append(marks, newMark)
		var content []any
		for child := n.FirstChild(); child != nil; child = child.NextSibling() {
			content = append(content, convertInlineNode(child, source, newMarks)...)
		}
		return content

	case *ast.CodeSpan:
		// Extract code span text from child text segments
		var codeText string
		for child := n.FirstChild(); child != nil; child = child.NextSibling() {
			if textNode, ok := child.(*ast.Text); ok {
				codeText += string(textNode.Segment.Value(source))
			}
		}
		newMark := map[string]any{"type": "code"}
		result := map[string]any{
			"type": "text",
			"text": codeText,
		}
		if len(marks) > 0 {
			allMarks := make([]any, len(marks)+1)
			for i, m := range marks {
				allMarks[i] = m
			}
			allMarks[len(marks)] = newMark
			result["marks"] = allMarks
		} else {
			result["marks"] = []any{newMark}
		}
		return []any{result}

	case *ast.Link:
		newMark := map[string]any{
			"type": "link",
			"attrs": map[string]any{
				"href": string(n.Destination),
			},
		}
		newMarks := append(marks, newMark)
		var content []any
		for child := n.FirstChild(); child != nil; child = child.NextSibling() {
			content = append(content, convertInlineNode(child, source, newMarks)...)
		}
		return content

	default:
		// For other inline nodes, recursively process children
		var content []any
		for child := node.FirstChild(); child != nil; child = child.NextSibling() {
			content = append(content, convertInlineNode(child, source, marks)...)
		}
		return content
	}
}

// ADFToGFM converts an Atlassian Document Format (ADF) document to GitHub-flavored markdown.
// This is useful for displaying Jira issue content in a readable format.
func ADFToGFM(adf map[string]any) string {
	if adf == nil {
		return ""
	}

	content, ok := adf["content"].([]any)
	if !ok || len(content) == 0 {
		return ""
	}

	var parts []string
	for _, item := range content {
		node, ok := item.(map[string]any)
		if !ok {
			continue
		}
		part := adfNodeToGFM(node, "")
		if part != "" {
			parts = append(parts, part)
		}
	}

	return strings.Join(parts, "\n\n")
}

// adfNodeToGFM converts a single ADF node to markdown.
// The prefix is used for nested contexts like blockquotes.
func adfNodeToGFM(node map[string]any, prefix string) string {
	nodeType, _ := node["type"].(string)

	switch nodeType {
	case "paragraph":
		return prefix + adfInlineToGFM(node)
	case "heading":
		return adfHeadingToGFM(node)
	case "codeBlock":
		return adfCodeBlockToGFM(node)
	case "bulletList":
		return adfBulletListToGFM(node)
	case "orderedList":
		return adfOrderedListToGFM(node)
	case "blockquote":
		return adfBlockquoteToGFM(node)
	case "hardBreak":
		return "\n"
	default:
		// For unknown types, try to extract text content
		return adfInlineToGFM(node)
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
func adfBulletListToGFM(node map[string]any) string {
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
		itemText := adfListItemToGFM(listItem)
		items = append(items, "- "+itemText)
	}

	return strings.Join(items, "\n")
}

// adfOrderedListToGFM converts an ADF orderedList to markdown.
func adfOrderedListToGFM(node map[string]any) string {
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
		itemText := adfListItemToGFM(listItem)
		items = append(items, fmt.Sprintf("%d. %s", i+1, itemText))
	}

	return strings.Join(items, "\n")
}

// adfListItemToGFM extracts the text content from a list item.
func adfListItemToGFM(node map[string]any) string {
	content, ok := node["content"].([]any)
	if !ok || len(content) == 0 {
		return ""
	}

	// List items typically contain paragraphs
	parts := make([]string, 0, len(content))
	for _, item := range content {
		child, ok := item.(map[string]any)
		if !ok {
			continue
		}
		parts = append(parts, adfInlineToGFM(child))
	}

	return strings.Join(parts, " ")
}

// adfBlockquoteToGFM converts an ADF blockquote to markdown.
func adfBlockquoteToGFM(node map[string]any) string {
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
		text := adfNodeToGFM(child, "")
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

	// Apply in specific order: code first (innermost), then em, then strong, then link (outermost)
	result := text

	if hasCode {
		result = "`" + result + "`"
	}
	if hasEm && hasStrong {
		// Combined: use ***
		result = "***" + result + "***"
	} else {
		if hasEm {
			result = "*" + result + "*"
		}
		if hasStrong {
			result = "**" + result + "**"
		}
	}
	if linkHref != "" {
		result = "[" + result + "](" + linkHref + ")"
	}

	return result
}
