package markdown

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"

	"github.com/fwojciec/jira4claude"
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
// Always returns a non-nil *ADFNode.
func toADF(markdown string) (*jira4claude.ADFNode, []string) {
	markdown = NormalizeUnicode(markdown)

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
		content = []jira4claude.ADFNode{}
	}

	return &jira4claude.ADFNode{
		Type:    "doc",
		Version: 1,
		Content: content,
	}, skipped.warnings()
}

// convertNode recursively converts goldmark AST nodes to ADF nodes.
func convertNode(node ast.Node, source []byte, skipped *skippedCollector) []jira4claude.ADFNode {
	var content []jira4claude.ADFNode

	for child := node.FirstChild(); child != nil; child = child.NextSibling() {
		adfNode, ok := nodeToADF(child, source, skipped)
		if ok {
			content = append(content, adfNode)
		}
	}

	return content
}

// nodeToADF converts a single goldmark AST node to an ADF node.
func nodeToADF(node ast.Node, source []byte, skipped *skippedCollector) (jira4claude.ADFNode, bool) {
	switch n := node.(type) {
	case *ast.Paragraph:
		return convertParagraph(n, source)
	case *ast.TextBlock:
		return convertTextBlock(n, source)
	case *ast.Heading:
		return convertHeading(n, source)
	case *ast.FencedCodeBlock:
		return convertFencedCodeBlock(n, source), true
	case *ast.List:
		return convertList(n, source, skipped), true
	case *ast.Blockquote:
		return convertBlockquote(n, source, skipped), true
	default:
		// Record the skipped node type
		typeName := reflect.TypeOf(node).Elem().Name()
		skipped.add(typeName)
		return jira4claude.ADFNode{}, false
	}
}

// convertParagraph converts a goldmark paragraph to an ADF paragraph.
func convertParagraph(node *ast.Paragraph, source []byte) (jira4claude.ADFNode, bool) {
	content := convertInlineContent(node, source)
	if len(content) == 0 {
		return jira4claude.ADFNode{}, false
	}
	return jira4claude.ADFNode{
		Type:    "paragraph",
		Content: content,
	}, true
}

// convertTextBlock converts a goldmark text block (used in tight lists) to an ADF paragraph.
func convertTextBlock(node *ast.TextBlock, source []byte) (jira4claude.ADFNode, bool) {
	content := convertInlineContent(node, source)
	if len(content) == 0 {
		return jira4claude.ADFNode{}, false
	}
	return jira4claude.ADFNode{
		Type:    "paragraph",
		Content: content,
	}, true
}

// convertHeading converts a goldmark heading to an ADF heading.
func convertHeading(node *ast.Heading, source []byte) (jira4claude.ADFNode, bool) {
	content := convertInlineContent(node, source)
	if len(content) == 0 {
		return jira4claude.ADFNode{}, false
	}
	attrs, _ := json.Marshal(map[string]any{"level": node.Level})
	return jira4claude.ADFNode{
		Type:    "heading",
		Attrs:   attrs,
		Content: content,
	}, true
}

// convertFencedCodeBlock converts a goldmark fenced code block to an ADF codeBlock.
func convertFencedCodeBlock(node *ast.FencedCodeBlock, source []byte) jira4claude.ADFNode {
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

	result := jira4claude.ADFNode{
		Type: "codeBlock",
		Content: []jira4claude.ADFNode{
			{
				Type: "text",
				Text: codeText,
			},
		},
	}

	lang := string(node.Language(source))
	if lang != "" {
		attrs, _ := json.Marshal(map[string]any{"language": lang})
		result.Attrs = attrs
	}

	return result
}

// convertList converts a goldmark list to an ADF bulletList or orderedList.
func convertList(node *ast.List, source []byte, skipped *skippedCollector) jira4claude.ADFNode {
	listType := "bulletList"
	if node.IsOrdered() {
		listType = "orderedList"
	}

	var items []jira4claude.ADFNode
	for child := node.FirstChild(); child != nil; child = child.NextSibling() {
		if listItem, ok := child.(*ast.ListItem); ok {
			items = append(items, convertListItem(listItem, source, skipped))
		}
	}

	return jira4claude.ADFNode{
		Type:    listType,
		Content: items,
	}
}

// convertListItem converts a goldmark list item to an ADF listItem.
func convertListItem(node *ast.ListItem, source []byte, skipped *skippedCollector) jira4claude.ADFNode {
	content := convertNode(node, source, skipped)
	return jira4claude.ADFNode{
		Type:    "listItem",
		Content: content,
	}
}

// convertBlockquote converts a goldmark blockquote to an ADF blockquote.
func convertBlockquote(node *ast.Blockquote, source []byte, skipped *skippedCollector) jira4claude.ADFNode {
	content := convertNode(node, source, skipped)
	return jira4claude.ADFNode{
		Type:    "blockquote",
		Content: content,
	}
}

// convertInlineContent converts the inline content of a block node to ADF text nodes.
func convertInlineContent(node ast.Node, source []byte) []jira4claude.ADFNode {
	var content []jira4claude.ADFNode
	for child := node.FirstChild(); child != nil; child = child.NextSibling() {
		inlineNodes := convertInlineNode(child, source, nil)
		content = append(content, inlineNodes...)
	}
	return consolidateTextNodes(content)
}

// consolidateTextNodes merges adjacent text nodes with identical marks.
func consolidateTextNodes(nodes []jira4claude.ADFNode) []jira4claude.ADFNode {
	if len(nodes) == 0 {
		return nodes
	}

	var result []jira4claude.ADFNode
	for _, node := range nodes {
		if len(result) == 0 {
			result = append(result, node)
			continue
		}

		last := &result[len(result)-1]

		// Both must be text nodes
		if last.Type != "text" || node.Type != "text" {
			result = append(result, node)
			continue
		}

		// Compare marks
		if !marksEqual(last.Marks, node.Marks) {
			result = append(result, node)
			continue
		}

		// Merge the text
		last.Text += node.Text
	}

	return result
}

// marksEqual compares two marks slices for equality.
func marksEqual(a, b []jira4claude.ADFMark) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Type != b[i].Type {
			return false
		}
		if string(a[i].Attrs) != string(b[i].Attrs) {
			return false
		}
	}
	return true
}

// textNodeWithMarks creates an ADF text node with the given text and marks.
func textNodeWithMarks(text string, marks []jira4claude.ADFMark) jira4claude.ADFNode {
	return jira4claude.ADFNode{
		Type:  "text",
		Text:  text,
		Marks: marks,
	}
}

// convertChildren recursively converts all children of a node with the given marks.
func convertChildren(node ast.Node, source []byte, marks []jira4claude.ADFMark) []jira4claude.ADFNode {
	var content []jira4claude.ADFNode
	for child := node.FirstChild(); child != nil; child = child.NextSibling() {
		content = append(content, convertInlineNode(child, source, marks)...)
	}
	return content
}

// convertInlineNode converts inline nodes (text, emphasis, etc.) to ADF text nodes.
func convertInlineNode(node ast.Node, source []byte, marks []jira4claude.ADFMark) []jira4claude.ADFNode {
	switch n := node.(type) {
	case *ast.Text:
		text := string(n.Segment.Value(source))
		if text == "" {
			return nil
		}
		return []jira4claude.ADFNode{textNodeWithMarks(text, marks)}

	case *ast.Emphasis:
		markType := "em"
		if n.Level == 2 {
			markType = "strong"
		}
		newMarks := append(cloneMarks(marks), jira4claude.ADFMark{Type: markType})
		return convertChildren(n, source, newMarks)

	case *ast.CodeSpan:
		var codeText string
		for child := n.FirstChild(); child != nil; child = child.NextSibling() {
			if textNode, ok := child.(*ast.Text); ok {
				codeText += string(textNode.Segment.Value(source))
			}
		}
		newMarks := append(cloneMarks(marks), jira4claude.ADFMark{Type: "code"})
		return []jira4claude.ADFNode{textNodeWithMarks(codeText, newMarks)}

	case *ast.Link:
		attrs, _ := json.Marshal(map[string]any{"href": string(n.Destination)})
		newMark := jira4claude.ADFMark{
			Type:  "link",
			Attrs: attrs,
		}
		return convertChildren(n, source, append(cloneMarks(marks), newMark))

	case *ast.AutoLink:
		url := string(n.URL(source))
		attrs, _ := json.Marshal(map[string]any{"href": url})
		newMark := jira4claude.ADFMark{
			Type:  "link",
			Attrs: attrs,
		}
		return []jira4claude.ADFNode{textNodeWithMarks(url, append(cloneMarks(marks), newMark))}

	default:
		return convertChildren(node, source, marks)
	}
}

// cloneMarks creates a shallow copy of a marks slice to avoid mutation via append.
func cloneMarks(marks []jira4claude.ADFMark) []jira4claude.ADFMark {
	if marks == nil {
		return nil
	}
	c := make([]jira4claude.ADFMark, len(marks))
	copy(c, marks)
	return c
}
