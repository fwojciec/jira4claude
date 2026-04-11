package markdown

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/fwojciec/jira4claude"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	extast "github.com/yuin/goldmark/extension/ast"
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
// It performs stateful sibling-level detection for <details> HTML blocks,
// collecting sibling nodes between <details> and </details> into expand nodes.
func convertNode(node ast.Node, source []byte, skipped *skippedCollector) []jira4claude.ADFNode {
	var content []jira4claude.ADFNode

	for child := node.FirstChild(); child != nil; child = child.NextSibling() {
		// Check for <details> opening HTMLBlock
		if htmlBlock, ok := child.(*ast.HTMLBlock); ok {
			if title, found := detectDetailsOpen(htmlBlock, source); found {
				// Collect siblings until </details>
				expandNode, next := collectExpandContent(child.NextSibling(), source, skipped, title)
				content = append(content, expandNode)
				child = next
				if child == nil {
					break
				}
				continue
			}
		}

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
	case *ast.ThematicBreak:
		return jira4claude.ADFNode{Type: "rule"}, true
	case *extast.Table:
		return convertTable(n, source, skipped), true
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

// convertList converts a goldmark list to an ADF bulletList, orderedList, or taskList.
// Task lists are detected by checking if any list item contains a TaskCheckBox.
func convertList(node *ast.List, source []byte, skipped *skippedCollector) jira4claude.ADFNode {
	if isTaskList(node) {
		return convertTaskList(node, source, skipped)
	}

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

// alertNameToPanelType maps GitHub alert names to ADF panel types.
func alertNameToPanelType(name string) (string, bool) {
	switch name {
	case "!NOTE":
		return "info", true
	case "!WARNING":
		return "warning", true
	case "!CAUTION":
		return "error", true
	case "!TIP":
		return "success", true
	case "!IMPORTANT":
		return "note", true
	default:
		return "", false
	}
}

// convertBlockquote converts a goldmark blockquote to an ADF blockquote or panel.
// If the first paragraph starts with a GitHub alert prefix (e.g., [!NOTE]),
// it emits a panel node instead of a blockquote.
func convertBlockquote(node *ast.Blockquote, source []byte, skipped *skippedCollector) jira4claude.ADFNode {
	if panelType, ok := detectAlertPrefix(node, source); ok {
		content := convertPanelContent(node, source, skipped)
		attrs, _ := json.Marshal(map[string]any{"panelType": panelType})
		return jira4claude.ADFNode{
			Type:    "panel",
			Attrs:   attrs,
			Content: content,
		}
	}

	content := convertNode(node, source, skipped)
	return jira4claude.ADFNode{
		Type:    "blockquote",
		Content: content,
	}
}

// detectAlertPrefix checks if the blockquote's first paragraph starts with a
// GitHub alert prefix like [!NOTE]. Goldmark parses "> [!NOTE]" as three
// separate text nodes: "[", "!NOTE", "]". Returns the ADF panel type and true if found.
func detectAlertPrefix(node *ast.Blockquote, source []byte) (string, bool) {
	firstChild := node.FirstChild()
	if firstChild == nil {
		return "", false
	}
	para, ok := firstChild.(*ast.Paragraph)
	if !ok {
		return "", false
	}

	// Collect the first 3 text nodes: should be "[", "!ALERT_NAME", "]"
	child := para.FirstChild()
	if child == nil {
		return "", false
	}
	t1, ok := child.(*ast.Text)
	if !ok || string(t1.Segment.Value(source)) != "[" {
		return "", false
	}

	child = child.NextSibling()
	if child == nil {
		return "", false
	}
	t2, ok := child.(*ast.Text)
	if !ok {
		return "", false
	}
	alertName := string(t2.Segment.Value(source))

	child = child.NextSibling()
	if child == nil {
		return "", false
	}
	t3, ok := child.(*ast.Text)
	if !ok || string(t3.Segment.Value(source)) != "]" {
		return "", false
	}

	return alertNameToPanelType(alertName)
}

// isPanelAllowedChild returns true if the ADF node type is allowed inside a panel.
// Per the ADF spec, panel can contain: paragraph, heading, bulletList, orderedList.
func isPanelAllowedChild(nodeType string) bool {
	switch nodeType {
	case "paragraph", "heading", "bulletList", "orderedList":
		return true
	default:
		return false
	}
}

// convertPanelContent converts the children of a blockquote that has been identified
// as a panel. It strips the alert prefix from the first paragraph's text and filters
// out child types not allowed in ADF panels.
func convertPanelContent(node *ast.Blockquote, source []byte, skipped *skippedCollector) []jira4claude.ADFNode {
	var content []jira4claude.ADFNode
	isFirst := true

	for child := node.FirstChild(); child != nil; child = child.NextSibling() {
		if isFirst {
			isFirst = false
			if para, ok := child.(*ast.Paragraph); ok {
				panelPara := convertPanelFirstParagraph(para, source)
				if panelPara != nil {
					content = append(content, *panelPara)
				}
				continue
			}
		}
		adfNode, ok := nodeToADF(child, source, skipped)
		if ok {
			if !isPanelAllowedChild(adfNode.Type) {
				skipped.add(adfNode.Type)
				continue
			}
			content = append(content, adfNode)
		}
	}

	return content
}

// convertPanelFirstParagraph converts the first paragraph of a panel blockquote,
// skipping the first 3 text nodes that form the alert prefix ("[", "!NOTE", "]").
func convertPanelFirstParagraph(para *ast.Paragraph, source []byte) *jira4claude.ADFNode {
	var inlineContent []jira4claude.ADFNode
	skipCount := 3 // Skip "[", "!ALERT_NAME", "]"

	for child := para.FirstChild(); child != nil; child = child.NextSibling() {
		if skipCount > 0 {
			skipCount--
			continue
		}
		inlineContent = append(inlineContent, convertInlineNode(child, source, nil)...)
	}

	inlineContent = consolidateTextNodes(inlineContent)
	if len(inlineContent) == 0 {
		return nil
	}

	return &jira4claude.ADFNode{
		Type:    "paragraph",
		Content: inlineContent,
	}
}

// detectDetailsOpen checks if an HTMLBlock starts with <details><summary>...</summary>
// and returns the extracted title if found.
func detectDetailsOpen(htmlBlock *ast.HTMLBlock, source []byte) (string, bool) {
	lines := htmlBlock.Lines()
	if lines.Len() == 0 {
		return "", false
	}

	var text string
	for i := range lines.Len() {
		line := lines.At(i)
		text += string(line.Value(source))
	}
	text = strings.TrimSpace(text)

	if !strings.HasPrefix(text, "<details>") {
		return "", false
	}

	// Extract title from <summary>...</summary>
	summaryStart := strings.Index(text, "<summary>")
	summaryEnd := strings.Index(text, "</summary>")
	if summaryStart == -1 || summaryEnd == -1 {
		return "", false
	}

	title := text[summaryStart+len("<summary>") : summaryEnd]
	return title, true
}

// isDetailsClose checks if an HTMLBlock is a </details> closing tag.
func isDetailsClose(node ast.Node, source []byte) bool {
	htmlBlock, ok := node.(*ast.HTMLBlock)
	if !ok {
		return false
	}
	lines := htmlBlock.Lines()
	if lines.Len() == 0 {
		return false
	}

	var text string
	for i := range lines.Len() {
		line := lines.At(i)
		text += string(line.Value(source))
	}
	return strings.TrimSpace(text) == "</details>"
}

// collectExpandContent collects sibling nodes starting from 'start' until a
// </details> HTMLBlock is encountered. Returns the assembled expand ADF node
// and the </details> node (so the caller can advance past it).
func collectExpandContent(start ast.Node, source []byte, skipped *skippedCollector, title string) (jira4claude.ADFNode, ast.Node) {
	var bodyContent []jira4claude.ADFNode

	var current ast.Node
	for current = start; current != nil; current = current.NextSibling() {
		if isDetailsClose(current, source) {
			break
		}
		adfNode, ok := nodeToADF(current, source, skipped)
		if ok {
			bodyContent = append(bodyContent, adfNode)
		}
	}

	attrs, _ := json.Marshal(map[string]any{"title": title})
	expandNode := jira4claude.ADFNode{
		Type:    "expand",
		Attrs:   attrs,
		Content: bodyContent,
	}

	return expandNode, current
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
		nodes := []jira4claude.ADFNode{textNodeWithMarks(text, marks)}
		if n.HardLineBreak() {
			nodes = append(nodes, jira4claude.ADFNode{Type: "hardBreak"})
		}
		return nodes

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

	case *extast.Strikethrough:
		newMarks := append(cloneMarks(marks), jira4claude.ADFMark{Type: "strike"})
		return convertChildren(n, source, newMarks)

	default:
		return convertChildren(node, source, marks)
	}
}

// isTaskList checks if a list contains task checkboxes.
func isTaskList(node *ast.List) bool {
	for child := node.FirstChild(); child != nil; child = child.NextSibling() {
		if listItem, ok := child.(*ast.ListItem); ok {
			for block := listItem.FirstChild(); block != nil; block = block.NextSibling() {
				for inline := block.FirstChild(); inline != nil; inline = inline.NextSibling() {
					if _, ok := inline.(*extast.TaskCheckBox); ok {
						return true
					}
				}
			}
		}
	}
	return false
}

// convertTaskList converts a goldmark list with task checkboxes to an ADF taskList.
func convertTaskList(node *ast.List, source []byte, _ *skippedCollector) jira4claude.ADFNode {
	var items []jira4claude.ADFNode
	for child := node.FirstChild(); child != nil; child = child.NextSibling() {
		listItem, ok := child.(*ast.ListItem)
		if !ok {
			continue
		}
		items = append(items, convertTaskItem(listItem, source))
	}
	return jira4claude.ADFNode{
		Type:    "taskList",
		Content: items,
	}
}

// convertTaskItem converts a goldmark list item with a TaskCheckBox to an ADF taskItem.
func convertTaskItem(node *ast.ListItem, source []byte) jira4claude.ADFNode {
	state := "TODO"
	// Look for TaskCheckBox in the list item's children
	for block := node.FirstChild(); block != nil; block = block.NextSibling() {
		for inline := block.FirstChild(); inline != nil; inline = inline.NextSibling() {
			if cb, ok := inline.(*extast.TaskCheckBox); ok {
				if cb.IsChecked {
					state = "DONE"
				}
				break
			}
		}
	}

	attrs, _ := json.Marshal(map[string]any{
		"localId": generateLocalID(),
		"state":   state,
	})

	// Convert content, skipping the TaskCheckBox node itself
	content := convertTaskItemContent(node, source)

	return jira4claude.ADFNode{
		Type:    "taskItem",
		Attrs:   attrs,
		Content: content,
	}
}

// convertTaskItemContent converts the content of a task list item, skipping the checkbox.
func convertTaskItemContent(node *ast.ListItem, source []byte) []jira4claude.ADFNode {
	var content []jira4claude.ADFNode
	for block := node.FirstChild(); block != nil; block = block.NextSibling() {
		// Convert inline content, skipping TaskCheckBox nodes
		var inlineContent []jira4claude.ADFNode
		for child := block.FirstChild(); child != nil; child = child.NextSibling() {
			if _, ok := child.(*extast.TaskCheckBox); ok {
				continue
			}
			inlineContent = append(inlineContent, convertInlineNode(child, source, nil)...)
		}
		inlineContent = consolidateTextNodes(inlineContent)
		if len(inlineContent) > 0 {
			content = append(content, jira4claude.ADFNode{
				Type:    "paragraph",
				Content: inlineContent,
			})
		}
	}
	return content
}

// generateLocalID creates a random UUID-like identifier for ADF nodes that require localId.
func generateLocalID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// convertTable converts a goldmark GFM table to an ADF table node.
// The goldmark AST has Table → TableHeader (first child) → TableCell,
// then TableRow children → TableCell. ADF uses tableRow for both,
// with tableHeader vs tableCell for cell types.
func convertTable(node *extast.Table, source []byte, _ *skippedCollector) jira4claude.ADFNode {
	var rows []jira4claude.ADFNode

	for child := node.FirstChild(); child != nil; child = child.NextSibling() {
		switch row := child.(type) {
		case *extast.TableHeader:
			rows = append(rows, convertTableRow(row, source, true))
		case *extast.TableRow:
			rows = append(rows, convertTableRow(row, source, false))
		}
	}

	return jira4claude.ADFNode{
		Type:    "table",
		Content: rows,
	}
}

// convertTableRow converts a goldmark table header or row to an ADF tableRow.
// Each child TableCell becomes either an ADF tableHeader or tableCell node
// depending on whether the row is a header row.
func convertTableRow(node ast.Node, source []byte, isHeader bool) jira4claude.ADFNode {
	var cells []jira4claude.ADFNode

	for child := node.FirstChild(); child != nil; child = child.NextSibling() {
		if cell, ok := child.(*extast.TableCell); ok {
			cellType := "tableCell"
			if isHeader {
				cellType = "tableHeader"
			}
			content := convertInlineContent(cell, source)
			var cellContent []jira4claude.ADFNode
			if len(content) > 0 {
				cellContent = []jira4claude.ADFNode{
					{
						Type:    "paragraph",
						Content: content,
					},
				}
			}
			cells = append(cells, jira4claude.ADFNode{
				Type:    cellType,
				Content: cellContent,
			})
		}
	}

	return jira4claude.ADFNode{
		Type:    "tableRow",
		Content: cells,
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
