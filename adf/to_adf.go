package adf

import (
	"fmt"
	"reflect"
	"sort"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
)

// skippedCollector tracks node types that were skipped during conversion.
// Each unique node type generates one warning.
type skippedCollector struct {
	types map[string]struct{}
}

func newSkippedCollector() *skippedCollector {
	return &skippedCollector{types: make(map[string]struct{})}
}

func (s *skippedCollector) add(nodeType string) {
	s.types[nodeType] = struct{}{}
}

// warnings returns a slice of warning messages for each skipped node type.
// Warnings are sorted alphabetically by node type for deterministic output.
// Returns nil if no nodes were skipped.
func (s *skippedCollector) warnings() []string {
	if len(s.types) == 0 {
		return nil
	}
	types := make([]string, 0, len(s.types))
	for t := range s.types {
		types = append(types, t)
	}
	sort.Strings(types)
	warnings := make([]string, len(types))
	for i, t := range types {
		warnings[i] = fmt.Sprintf("skipped unsupported node type '%s'", t)
	}
	return warnings
}

// toADF converts GitHub-flavored markdown to Atlassian Document Format (ADF).
// The result can be used directly in Jira API requests for description and comment fields.
// Returns warnings for any elements that were skipped during conversion.
func toADF(markdown string) (map[string]any, []string) {
	md := goldmark.New(
		goldmark.WithExtensions(extension.GFM),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
		),
	)

	reader := text.NewReader([]byte(markdown))
	doc := md.Parser().Parse(reader)

	skipped := newSkippedCollector()
	content := convertNode(doc, []byte(markdown), skipped)
	if content == nil {
		content = []any{}
	}

	return map[string]any{
		"type":    "doc",
		"version": 1,
		"content": content,
	}, skipped.warnings()
}

// convertNode recursively converts goldmark AST nodes to ADF nodes.
func convertNode(node ast.Node, source []byte, skipped *skippedCollector) []any {
	var content []any

	for child := node.FirstChild(); child != nil; child = child.NextSibling() {
		adfNode := nodeToADF(child, source, skipped)
		if adfNode != nil {
			content = append(content, adfNode)
		}
	}

	return content
}

// nodeToADF converts a single goldmark AST node to an ADF node.
func nodeToADF(node ast.Node, source []byte, skipped *skippedCollector) map[string]any {
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
		return convertList(n, source, skipped)
	case *ast.Blockquote:
		return convertBlockquote(n, source, skipped)
	default:
		// Record the skipped node type
		typeName := reflect.TypeOf(node).Elem().Name()
		skipped.add(typeName)
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
	if len(content) == 0 {
		return nil
	}
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
func convertList(node *ast.List, source []byte, skipped *skippedCollector) map[string]any {
	listType := "bulletList"
	if node.IsOrdered() {
		listType = "orderedList"
	}

	var items []any
	for child := node.FirstChild(); child != nil; child = child.NextSibling() {
		if listItem, ok := child.(*ast.ListItem); ok {
			items = append(items, convertListItem(listItem, source, skipped))
		}
	}

	return map[string]any{
		"type":    listType,
		"content": items,
	}
}

// convertListItem converts a goldmark list item to an ADF listItem.
func convertListItem(node *ast.ListItem, source []byte, skipped *skippedCollector) map[string]any {
	content := convertNode(node, source, skipped)
	return map[string]any{
		"type":    "listItem",
		"content": content,
	}
}

// convertBlockquote converts a goldmark blockquote to an ADF blockquote.
func convertBlockquote(node *ast.Blockquote, source []byte, skipped *skippedCollector) map[string]any {
	content := convertNode(node, source, skipped)
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

// textNodeWithMarks creates an ADF text node with the given text and marks.
func textNodeWithMarks(text string, marks []map[string]any) map[string]any {
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
	return result
}

// convertChildren recursively converts all children of a node with the given marks.
func convertChildren(node ast.Node, source []byte, marks []map[string]any) []any {
	var content []any
	for child := node.FirstChild(); child != nil; child = child.NextSibling() {
		content = append(content, convertInlineNode(child, source, marks)...)
	}
	return content
}

// convertInlineNode converts inline nodes (text, emphasis, etc.) to ADF text nodes.
func convertInlineNode(node ast.Node, source []byte, marks []map[string]any) []any {
	switch n := node.(type) {
	case *ast.Text:
		text := string(n.Segment.Value(source))
		if text == "" {
			return nil
		}
		return []any{textNodeWithMarks(text, marks)}

	case *ast.Emphasis:
		markType := "em"
		if n.Level == 2 {
			markType = "strong"
		}
		newMarks := append(marks, map[string]any{"type": markType})
		return convertChildren(n, source, newMarks)

	case *ast.CodeSpan:
		var codeText string
		for child := n.FirstChild(); child != nil; child = child.NextSibling() {
			if textNode, ok := child.(*ast.Text); ok {
				codeText += string(textNode.Segment.Value(source))
			}
		}
		newMarks := append(marks, map[string]any{"type": "code"})
		return []any{textNodeWithMarks(codeText, newMarks)}

	case *ast.Link:
		newMark := map[string]any{
			"type": "link",
			"attrs": map[string]any{
				"href": string(n.Destination),
			},
		}
		return convertChildren(n, source, append(marks, newMark))

	default:
		return convertChildren(node, source, marks)
	}
}
