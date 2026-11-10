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
	"github.com/yuin/goldmark/util"
)

// skippedCollector tracks content that a conversion could not carry over.
// Each unique (kind, node type) pair produces one jira4claude.ConversionIssue:
// add records content omitted from the output (dropped), degrade records
// content that is kept but changed form (degraded).
//
// On the write side the collector also carries the conversion's localId
// source, because it is the one value already threaded through every
// conversion function.
type skippedCollector struct {
	types map[issueKey]jira4claude.ConversionIssue

	localID  func(n int) string // write side only: source of localId values
	localIDs int                // number of localId values generated so far

	tokens *tokenRegistry // read side only: protected-node tokens (protect.go)
}

// issueKey identifies one recorded issue, so a node type can be both dropped
// and degraded in the same conversion without one hiding the other.
type issueKey struct {
	kind string
	node string
}

func newSkippedCollector() *skippedCollector {
	return &skippedCollector{types: make(map[issueKey]jira4claude.ConversionIssue)}
}

// add records nodeType as dropped: its content is not in the output.
func (s *skippedCollector) add(nodeType string) {
	s.record(jira4claude.ConversionIssue{
		Kind:   jira4claude.ConversionDropped,
		Node:   nodeType,
		Detail: fmt.Sprintf("skipped unsupported node type '%s'", nodeType),
	})
}

// degrade records nodeType as degraded: its content is in the output, but in
// a different form. detail says what changed.
func (s *skippedCollector) degrade(nodeType, detail string) {
	s.record(jira4claude.ConversionIssue{
		Kind:   jira4claude.ConversionDegraded,
		Node:   nodeType,
		Detail: detail,
	})
}

// nextLocalID returns the next localId of this conversion.
func (s *skippedCollector) nextLocalID() string {
	id := s.localID(s.localIDs)
	s.localIDs++
	return id
}

func (s *skippedCollector) record(issue jira4claude.ConversionIssue) {
	s.types[issueKey{kind: issue.Kind, node: issue.Node}] = issue
}

// issues returns one ConversionIssue per recorded (kind, node type) pair,
// sorted by node and then kind for deterministic output. Returns nil if
// nothing was recorded.
func (s *skippedCollector) issues() []jira4claude.ConversionIssue {
	if len(s.types) == 0 {
		return nil
	}
	issues := make([]jira4claude.ConversionIssue, 0, len(s.types))
	for _, issue := range s.types {
		issues = append(issues, issue)
	}
	sort.Slice(issues, func(i, j int) bool {
		if issues[i].Node != issues[j].Node {
			return issues[i].Node < issues[j].Node
		}
		return issues[i].Kind < issues[j].Kind
	})
	return issues
}

// newMarkdownParser returns the goldmark parser used on write. The read side
// also uses it to check its own output.
func newMarkdownParser() parser.Parser {
	return goldmark.New(
		// GFM minus its strikethrough, which also strikes on a single tilde.
		goldmark.WithExtensions(extension.Linkify, extension.Table, extension.TaskList),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
			parser.WithInlineParsers(util.Prioritized(newStrikethroughParser(), 500)),
		),
	).Parser()
}

// toADF converts GitHub-flavored markdown to Atlassian Document Format (ADF).
// The result can be used directly in Jira API requests for description and comment fields.
// Returns a ConversionIssue for any content that was dropped or degraded.
// Always returns a non-nil *ADFNode.
// localID supplies task list and task item localId values; nil means random.
func toADF(markdown string, localID func(n int) string) (*jira4claude.ADFNode, []jira4claude.ConversionIssue) {
	markdown = NormalizeUnicode(markdown)

	reader := text.NewReader([]byte(markdown))
	doc := newMarkdownParser().Parse(reader)

	skipped := newSkippedCollector()
	skipped.localID = localID
	if skipped.localID == nil {
		skipped.localID = randomLocalID
	}
	content := convertNode(doc, []byte(markdown), skipped)
	if content == nil {
		content = []jira4claude.ADFNode{}
	}

	return &jira4claude.ADFNode{
		Type:    "doc",
		Version: 1,
		Content: content,
	}, skipped.issues()
}

// convertNode recursively converts goldmark AST nodes to ADF nodes.
// It performs stateful sibling-level detection for <details> HTML blocks,
// collecting sibling nodes between <details> and </details> into expand nodes.
func convertNode(node ast.Node, source []byte, skipped *skippedCollector) []jira4claude.ADFNode {
	var content []jira4claude.ADFNode

	for child := node.FirstChild(); child != nil; child = child.NextSibling() {
		// Check for <details> opening HTMLBlock
		if htmlBlock, ok := child.(*ast.HTMLBlock); ok {
			if opener, found := detectDetailsOpen(htmlBlock, source); found {
				// Collect siblings until </details>. Text that shares the
				// opening HTMLBlock comes first in the body.
				bodyContent, next, closed := collectExpandContent(child.NextSibling(), source, skipped)
				if opener.text != "" {
					bodyContent = append([]jira4claude.ADFNode{textParagraph(opener.text)}, bodyContent...)
				}
				if closed {
					attrs, _ := json.Marshal(map[string]any{"title": opener.title})
					if len(bodyContent) == 0 {
						// ADF requires expand content; an empty body holds an empty paragraph.
						bodyContent = []jira4claude.ADFNode{{Type: "paragraph"}}
					}
					content = append(content, jira4claude.ADFNode{
						Type:    "expand",
						Attrs:   attrs,
						Content: bodyContent,
					})
					child = next
					if child == nil {
						break
					}
					continue
				}
				// No closing </details> — emit body content directly and warn
				skipped.add("HTMLBlock")
				content = append(content, bodyContent...)
				break // All remaining siblings were consumed
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
		return convertParagraph(n, source, skipped)
	case *ast.TextBlock:
		return convertTextBlock(n, source, skipped)
	case *ast.Heading:
		return convertHeading(n, source, skipped)
	case *ast.FencedCodeBlock:
		return convertFencedCodeBlock(n, source), true
	case *ast.CodeBlock:
		return convertIndentedCodeBlock(n, source), true
	case *ast.List:
		return convertList(n, source, skipped), true
	case *ast.Blockquote:
		// A blockquote whose children were all dropped would be emitted
		// empty, which the ADF schema rejects; the drops are already
		// reported, so the empty container is left out.
		quote := convertBlockquote(n, source, skipped)
		return quote, quote.Type != "blockquote" || len(quote.Content) > 0
	case *ast.ThematicBreak:
		return jira4claude.ADFNode{Type: "rule"}, true
	case *extast.Table:
		return convertTable(n, source, skipped), true
	case *ast.HTMLBlock:
		return convertHTMLBlock(n, source, skipped)
	default:
		// Record the skipped node type
		typeName := reflect.TypeOf(node).Elem().Name()
		skipped.add(typeName)
		return jira4claude.ADFNode{}, false
	}
}

// convertParagraph converts a goldmark paragraph to an ADF paragraph.
func convertParagraph(node *ast.Paragraph, source []byte, skipped *skippedCollector) (jira4claude.ADFNode, bool) {
	content := convertInlineContent(node, source, skipped)
	if len(content) == 0 {
		return jira4claude.ADFNode{}, false
	}
	return jira4claude.ADFNode{
		Type:    "paragraph",
		Content: content,
	}, true
}

// convertTextBlock converts a goldmark text block (used in tight lists) to an ADF paragraph.
func convertTextBlock(node *ast.TextBlock, source []byte, skipped *skippedCollector) (jira4claude.ADFNode, bool) {
	content := convertInlineContent(node, source, skipped)
	if len(content) == 0 {
		return jira4claude.ADFNode{}, false
	}
	return jira4claude.ADFNode{
		Type:    "paragraph",
		Content: content,
	}, true
}

// convertHeading converts a goldmark heading to an ADF heading.
func convertHeading(node *ast.Heading, source []byte, skipped *skippedCollector) (jira4claude.ADFNode, bool) {
	content := convertInlineContent(node, source, skipped)
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

	result := jira4claude.ADFNode{Type: "codeBlock"}
	if codeText != "" { // ADF rejects an empty text node
		result.Content = []jira4claude.ADFNode{{Type: "text", Text: codeText}}
	}

	lang := infoLanguage(node.Language(source))
	if lang != "" {
		attrs, _ := json.Marshal(map[string]any{"language": lang})
		result.Attrs = attrs
	}

	return result
}

// infoLanguage returns the language of a fenced code block: the first word
// of its info string. goldmark's Language splits the info string at a space
// only, so a tab after the first word would stay in the language.
func infoLanguage(lang []byte) string {
	s := string(lang)
	if i := strings.IndexAny(s, " \t"); i >= 0 {
		s = s[:i]
	}
	return s
}

// convertIndentedCodeBlock converts a goldmark indented code block to an ADF
// codeBlock. Indented code carries no info string, so no language is set.
func convertIndentedCodeBlock(node *ast.CodeBlock, source []byte) jira4claude.ADFNode {
	var b strings.Builder
	lines := node.Lines()
	for i := range lines.Len() {
		line := lines.At(i)
		b.Write(line.Value(source))
	}
	return jira4claude.ADFNode{
		Type: "codeBlock",
		Content: []jira4claude.ADFNode{
			{Type: "text", Text: strings.TrimSuffix(b.String(), "\n")},
		},
	}
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

	result := jira4claude.ADFNode{
		Type:    listType,
		Content: items,
	}

	if node.IsOrdered() && node.Start != 1 {
		attrs, _ := json.Marshal(map[string]any{"order": node.Start})
		result.Attrs = attrs
	}

	return result
}

// convertListItem converts a goldmark list item to an ADF listItem.
func convertListItem(node *ast.ListItem, source []byte, skipped *skippedCollector) jira4claude.ADFNode {
	content := convertNode(node, source, skipped)
	content = constrainChildren(content, "listItem", isListItemAllowedChild, skipped)
	if len(content) == 0 || content[0].Type == "bulletList" || content[0].Type == "orderedList" {
		// ADF requires a listItem to open with a paragraph (or code or media),
		// so an empty item, or one that opens with a list, gets an empty one.
		content = append([]jira4claude.ADFNode{{Type: "paragraph"}}, content...)
	}
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
		if len(content) == 0 {
			// ADF requires panel content; an empty panel holds an empty paragraph.
			content = []jira4claude.ADFNode{{Type: "paragraph"}}
		}
		attrs, _ := json.Marshal(map[string]any{"panelType": panelType})
		return jira4claude.ADFNode{
			Type:    "panel",
			Attrs:   attrs,
			Content: content,
		}
	}

	content := convertNode(node, source, skipped)
	content = constrainChildren(content, "blockquote", isBlockquoteAllowedChild, skipped)
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

// isBlockquoteAllowedChild returns true if the ADF node type is allowed inside a blockquote.
// Per Jira Cloud ADF docs: paragraph, bulletList, orderedList, codeBlock, mediaGroup, mediaSingle.
func isBlockquoteAllowedChild(nodeType string) bool {
	switch nodeType {
	case "paragraph", "bulletList", "orderedList", "codeBlock", "mediaGroup", "mediaSingle":
		return true
	default:
		return false
	}
}

// isListItemAllowedChild returns true if the ADF node type is allowed inside a listItem.
// Per Jira Cloud ADF docs: paragraph, bulletList, orderedList, codeBlock, mediaSingle, mediaGroup.
func isListItemAllowedChild(nodeType string) bool {
	switch nodeType {
	case "paragraph", "bulletList", "orderedList", "codeBlock", "mediaSingle", "mediaGroup":
		return true
	default:
		return false
	}
}

// constrainChildren filters ADF children to only the types allowed inside
// container (an ADF node type such as "listItem", used in issue details).
// Headings are downgraded to paragraphs (same inline content, different type).
// Other disallowed containers have their children extracted and recursively
// constrained; an expand's title is kept as a plain paragraph before its body.
// Each such change keeps the content but not its form, so it is reported as
// degraded. Only the outermost unwrapped container is reported, so a
// flattened table gives one issue rather than one per row and cell type.
// Disallowed leaf nodes are skipped and reported as dropped.
func constrainChildren(children []jira4claude.ADFNode, container string, allowed func(string) bool, skipped *skippedCollector) []jira4claude.ADFNode {
	return constrainChildrenAt(children, container, allowed, skipped, true)
}

// constrainChildrenAt is constrainChildren; reportUnwrap is false inside a
// container that is already being unwrapped and reported.
func constrainChildrenAt(children []jira4claude.ADFNode, container string, allowed func(string) bool, skipped *skippedCollector, reportUnwrap bool) []jira4claude.ADFNode {
	var result []jira4claude.ADFNode
	for _, child := range children {
		if allowed(child.Type) {
			result = append(result, child)
			continue
		}
		// Downgrade heading → paragraph (preserve inline content)
		if child.Type == "heading" {
			skipped.degrade(child.Type, child.Type+" is not allowed inside "+container+"; downgraded to a paragraph")
			result = append(result, jira4claude.ADFNode{
				Type:    "paragraph",
				Content: child.Content,
			})
			continue
		}
		// Extract content from disallowed containers (blockquote, panel, etc.)
		// A container holding only its empty placeholder paragraph counts as
		// empty.
		if len(child.Content) > 1 || (len(child.Content) == 1 && !isEmptyParagraph(child.Content[0])) {
			detail := child.Type + " is not allowed inside " + container + "; unwrapped into its content"
			if child.Type == "expand" {
				detail += ", its title kept as a paragraph"
				if title := expandTitle(child); title != "" {
					result = append(result, textParagraph(title))
				}
			}
			if reportUnwrap {
				skipped.degrade(child.Type, detail)
			}
			result = append(result, constrainChildrenAt(child.Content, container, allowed, skipped, false)...)
			continue
		}
		skipped.add(child.Type)
	}
	return result
}

// isEmptyParagraph reports whether node is a paragraph with no content, the
// placeholder an empty table cell, panel, expand or list item holds.
func isEmptyParagraph(node jira4claude.ADFNode) bool {
	return node.Type == "paragraph" && len(node.Content) == 0
}

// expandTitle returns the trimmed title attribute of an expand node, or ""
// if it has none.
func expandTitle(node jira4claude.ADFNode) string {
	var attrs struct {
		Title string `json:"title"`
	}
	if len(node.Attrs) == 0 || json.Unmarshal(node.Attrs, &attrs) != nil {
		return ""
	}
	return strings.TrimSpace(attrs.Title)
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
				panelPara := convertPanelFirstParagraph(para, source, skipped)
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
func convertPanelFirstParagraph(para *ast.Paragraph, source []byte, skipped *skippedCollector) *jira4claude.ADFNode {
	// Skip "[", "!ALERT_NAME", "]"
	first := para.FirstChild()
	for range 3 {
		if first != nil {
			first = first.NextSibling()
		}
	}

	inlineContent := consolidateTextNodes(convertInlineRun(first, source, nil, skipped))
	if len(inlineContent) == 0 {
		return nil
	}

	return &jira4claude.ADFNode{
		Type:    "paragraph",
		Content: inlineContent,
	}
}

// htmlBlockText returns the trimmed source text of an HTMLBlock node, and
// false if node is not an HTMLBlock or has no lines.
func htmlBlockText(node ast.Node, source []byte) (string, bool) {
	htmlBlock, ok := node.(*ast.HTMLBlock)
	if !ok {
		return "", false
	}
	lines := htmlBlock.Lines()
	if lines.Len() == 0 {
		return "", false
	}
	var text strings.Builder
	for i := range lines.Len() {
		line := lines.At(i)
		text.Write(line.Value(source))
	}
	return strings.TrimSpace(text.String()), true
}

// convertHTMLBlock keeps a raw HTML block that is not part of a <details>
// construct as a paragraph of literal text, so nothing is lost and nothing is
// interpreted. Each source line becomes a text node, with one hard break
// between lines (a blank line inside the block is two consecutive hard
// breaks), so the line structure survives and reads back as the same block.
// A block that mentions <details> or </details> reached here only because the
// details handling could not pair it; it stays dropped so a write refuses.
func convertHTMLBlock(node *ast.HTMLBlock, source []byte, skipped *skippedCollector) (jira4claude.ADFNode, bool) {
	lines := htmlBlockLines(node, source)
	if isDetailsConstruct(lines) {
		skipped.add("HTMLBlock")
		return jira4claude.ADFNode{}, false
	}
	// Trim blank lines at either end so the paragraph never starts or ends
	// with a hard break.
	for len(lines) > 0 && strings.TrimSpace(lines[0]) == "" {
		lines = lines[1:]
	}
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	if len(lines) == 0 {
		return jira4claude.ADFNode{}, false
	}
	var content []jira4claude.ADFNode
	for i, line := range lines {
		if i > 0 {
			content = append(content, jira4claude.ADFNode{Type: "hardBreak"})
		}
		if line != "" {
			content = append(content, jira4claude.ADFNode{Type: "text", Text: line})
		}
	}
	return jira4claude.ADFNode{Type: "paragraph", Content: content}, true
}

// htmlBlockLines returns every source line of an HTMLBlock, including the
// closing line goldmark stores apart from the others, without line endings.
func htmlBlockLines(node *ast.HTMLBlock, source []byte) []string {
	segments := node.Lines()
	lines := make([]string, 0, segments.Len()+1)
	for i := range segments.Len() {
		segment := segments.At(i)
		lines = append(lines, string(segment.Value(source)))
	}
	if node.HasClosure() {
		lines = append(lines, string(node.ClosureLine.Value(source)))
	}
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, "\r\n")
	}
	return lines
}

// isDetailsConstruct reports whether the lines of an HTMLBlock contain a
// <details> opening tag or a </details> closing tag, in any letter case.
func isDetailsConstruct(lines []string) bool {
	text := strings.ToLower(strings.Join(lines, "\n"))
	if strings.Contains(text, "</details") {
		return true
	}
	for rest := text; ; {
		i := strings.Index(rest, "<details")
		if i == -1 {
			return false
		}
		rest = rest[i+len("<details"):]
		if rest == "" || strings.ContainsRune("> \t\n/", rune(rest[0])) {
			return true
		}
	}
}

// detailsOpener is an HTMLBlock that opens a <details> element.
type detailsOpener struct {
	plain      bool   // the tag is exactly <details>, with no attributes
	hasSummary bool   // a <summary>...</summary> element follows the tag
	title      string // the summary text, as written
	text       string // any other text in the block, joined into one line
}

// parseDetailsOpener reports whether node is an HTMLBlock that opens a
// <details> element, with or without attributes or a summary, and splits it
// into its summary and the rest of its text. goldmark keeps every line up to
// the next blank line in the same HTMLBlock, so the block can also hold body
// text; that text is returned so it is not lost. A block that also contains
// </details> is not an opener: the caller's fallback reports it as dropped.
func parseDetailsOpener(node ast.Node, source []byte) (detailsOpener, bool) {
	text, ok := htmlBlockText(node, source)
	if !ok || strings.Contains(text, "</details>") {
		return detailsOpener{}, false
	}
	rest, found := strings.CutPrefix(text, "<details")
	if !found || rest == "" || !strings.ContainsRune("> \t\r\n", rune(rest[0])) {
		return detailsOpener{}, false
	}
	tagEnd := strings.IndexByte(rest, '>')
	if tagEnd == -1 {
		return detailsOpener{}, false
	}
	opener := detailsOpener{plain: tagEnd == 0}
	rest = rest[tagEnd+1:]

	summaryStart := strings.Index(rest, "<summary>")
	summaryEnd := strings.Index(rest, "</summary>")
	if summaryStart != -1 && summaryEnd > summaryStart {
		opener.hasSummary = true
		opener.title = rest[summaryStart+len("<summary>") : summaryEnd]
		rest = rest[:summaryStart] + "\n" + rest[summaryEnd+len("</summary>"):]
	}
	opener.text = strings.Join(strings.Fields(rest), " ")
	return opener, true
}

// textParagraph builds a paragraph holding text as a single plain text node.
func textParagraph(text string) jira4claude.ADFNode {
	return jira4claude.ADFNode{
		Type:    "paragraph",
		Content: []jira4claude.ADFNode{{Type: "text", Text: text}},
	}
}

// detectDetailsOpen checks if a node is an HTMLBlock that starts with
// <details><summary>...</summary>. Only such a block opens an expand.
func detectDetailsOpen(node ast.Node, source []byte) (detailsOpener, bool) {
	opener, ok := parseDetailsOpener(node, source)
	if !ok || !opener.plain || !opener.hasSummary {
		return detailsOpener{}, false
	}
	return opener, true
}

// isDetailsClose checks if an HTMLBlock is a </details> closing tag.
func isDetailsClose(node ast.Node, source []byte) bool {
	text, ok := htmlBlockText(node, source)
	return ok && text == "</details>"
}

// nestedDetailsDetail explains the degraded issue reported when a nested
// <details> block is flattened into its enclosing expand.
const nestedDetailsDetail = "nested <details> flattened into the enclosing expand " +
	"(ADF cannot nest expand); its summary is kept as a paragraph"

// collectExpandContent collects sibling nodes starting from 'start', the node
// after an opening <details>, until the matching </details>. It returns the
// collected body content, the matching </details> node (so the caller can
// advance past it), and whether that closing tag was found. If it was not,
// the caller should emit the body content directly (not wrapped in expand)
// and warn.
//
// The scan tracks nesting depth explicitly: depth is the number of open
// <details> elements, starting at 1 for the block being collected. ADF cannot
// nest expand nodes, so a nested <details> is flattened: its open and close
// tags are consumed, its summary (if any) becomes a plain paragraph at that
// position, followed by any other text from its opening HTMLBlock; its body
// joins the enclosing body, and the flattening is reported as degraded.
func collectExpandContent(start ast.Node, source []byte, skipped *skippedCollector) ([]jira4claude.ADFNode, ast.Node, bool) {
	var bodyContent []jira4claude.ADFNode
	depth := 1

	for current := start; current != nil; current = current.NextSibling() {
		switch {
		case isDetailsClose(current, source):
			depth--
			if depth == 0 {
				return bodyContent, current, true
			}
		default:
			if opener, ok := parseDetailsOpener(current, source); ok {
				depth++
				skipped.degrade("details", nestedDetailsDetail)
				if title := strings.TrimSpace(opener.title); title != "" {
					bodyContent = append(bodyContent, textParagraph(title))
				}
				if opener.text != "" {
					bodyContent = append(bodyContent, textParagraph(opener.text))
				}
				continue
			}
			adfNode, ok := nodeToADF(current, source, skipped)
			if ok {
				bodyContent = append(bodyContent, adfNode)
			}
		}
	}

	return bodyContent, nil, false
}

// convertInlineContent converts the inline content of a block node to ADF text nodes.
// Inline HTML tags (<u>, <sub>, <sup>) are resolved by a markTracker.
func convertInlineContent(node ast.Node, source []byte, skipped *skippedCollector) []jira4claude.ADFNode {
	return consolidateTextNodes(convertInlineRun(node.FirstChild(), source, nil, skipped))
}

// rawHTMLContent extracts the text content from a goldmark RawHTML inline node.
func rawHTMLContent(rh *ast.RawHTML, source []byte) string {
	var sb strings.Builder
	for i := 0; i < rh.Segments.Len(); i++ {
		seg := rh.Segments.At(i)
		sb.Write(seg.Value(source))
	}
	return sb.String()
}

// openingHTMLTagToMark maps an opening inline HTML tag to an ADF mark.
// Returns the mark and true if the tag is recognized, otherwise false.
func openingHTMLTagToMark(tag string) (jira4claude.ADFMark, bool) {
	switch tag {
	case "<u>":
		return jira4claude.ADFMark{Type: "underline"}, true
	case "<sub>":
		attrs, _ := json.Marshal(map[string]any{"type": "sub"})
		return jira4claude.ADFMark{Type: "subsup", Attrs: attrs}, true
	case "<sup>":
		attrs, _ := json.Marshal(map[string]any{"type": "sup"})
		return jira4claude.ADFMark{Type: "subsup", Attrs: attrs}, true
	default:
		return jira4claude.ADFMark{}, false
	}
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

// textNodeWithMarks creates an ADF text node with the given text and marks,
// in canonical form (see canonicalMarks). Nested marks of one type with
// different attributes cannot all apply: the innermost is kept and the text
// is reported as degraded.
func textNodeWithMarks(text string, marks []jira4claude.ADFMark, skipped *skippedCollector) jira4claude.ADFNode {
	canonical, conflict := canonicalMarks(marks)
	if conflict && skipped != nil {
		skipped.degrade("text", conflictingMarksDetail)
	}
	return jira4claude.ADFNode{
		Type:  "text",
		Text:  text,
		Marks: canonical,
	}
}

// convertChildren recursively converts all children of a node with the given marks.
// Inline HTML tags (<u>, <sub>, <sup>) are resolved by a markTracker.
func convertChildren(node ast.Node, source []byte, marks []jira4claude.ADFMark, skipped *skippedCollector) []jira4claude.ADFNode {
	return convertInlineRun(node.FirstChild(), source, marks, skipped)
}

// convertInlineNode converts inline nodes (text, emphasis, etc.) to ADF text nodes.
func convertInlineNode(node ast.Node, source []byte, marks []jira4claude.ADFMark, skipped *skippedCollector) []jira4claude.ADFNode {
	switch n := node.(type) {
	case *ast.Text:
		var text string
		if n.IsRaw() {
			text = string(n.Segment.Value(source))
		} else {
			text = decodeText(n.Segment.Value(source))
		}
		// A soft line break renders as a space; without it the words on
		// either side of the line ending would be joined.
		if n.SoftLineBreak() {
			text += " "
		}
		var nodes []jira4claude.ADFNode
		if text != "" {
			nodes = append(nodes, textNodeWithMarks(text, marks, skipped))
		}
		// The break flag can sit on an empty Text node, as when the break
		// directly follows emphasis, a link or inline code; it must be kept
		// even though there is no text to emit.
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
		return convertChildren(n, source, newMarks, skipped)

	case *ast.CodeSpan:
		var codeText string
		for child := n.FirstChild(); child != nil; child = child.NextSibling() {
			if textNode, ok := child.(*ast.Text); ok {
				// A line ending inside a code span is a space (CommonMark
				// 6.1); goldmark leaves it at the end of the line's segment.
				value := string(textNode.Segment.Value(source))
				if strings.HasSuffix(value, "\n") {
					value = strings.TrimSuffix(value, "\n") + " "
				}
				codeText += value
			}
		}
		// ADF's code mark is exclusive: per the schema, a text node carrying
		// the code mark may only also carry link (or annotation). Inherited
		// strong/em/strike/underline/subsup marks would produce an invalid doc
		// that Jira rejects with INVALID_INPUT, so drop them here. Sibling
		// text nodes around the code span retain the parent marks, so visual
		// formatting is preserved everywhere except the code span itself.
		newMarks := append(filterCodeCompatibleMarks(marks), jira4claude.ADFMark{Type: "code"})
		return []jira4claude.ADFNode{textNodeWithMarks(codeText, newMarks, skipped)}

	case *ast.Link:
		attrs, _ := json.Marshal(map[string]any{"href": string(n.Destination)})
		newMark := jira4claude.ADFMark{
			Type:  "link",
			Attrs: attrs,
		}
		return convertChildren(n, source, append(cloneMarks(marks), newMark), skipped)

	case *ast.AutoLink:
		url := string(n.URL(source))
		attrs, _ := json.Marshal(map[string]any{"href": url})
		newMark := jira4claude.ADFMark{
			Type:  "link",
			Attrs: attrs,
		}
		return []jira4claude.ADFNode{textNodeWithMarks(url, append(cloneMarks(marks), newMark), skipped)}

	case *extast.Strikethrough:
		newMarks := append(cloneMarks(marks), jira4claude.ADFMark{Type: "strike"})
		return convertChildren(n, source, newMarks, skipped)

	case *ast.Image:
		if skipped != nil {
			skipped.add("Image")
		}
		return nil

	default:
		return convertChildren(node, source, marks, skipped)
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
func convertTaskList(node *ast.List, source []byte, skipped *skippedCollector) jira4claude.ADFNode {
	var items []jira4claude.ADFNode
	for child := node.FirstChild(); child != nil; child = child.NextSibling() {
		listItem, ok := child.(*ast.ListItem)
		if !ok {
			continue
		}
		item, nested := convertTaskItem(listItem, source, skipped)
		items = append(items, item)
		items = append(items, nested...)
	}
	attrs, _ := json.Marshal(map[string]any{
		"localId": skipped.nextLocalID(),
	})
	return jira4claude.ADFNode{
		Type:    "taskList",
		Attrs:   attrs,
		Content: items,
	}
}

// convertTaskItem converts a goldmark list item with a TaskCheckBox to an ADF
// taskItem. It also returns the task lists nested directly in the item, which
// ADF places as sibling taskList nodes after the item in the parent taskList.
func convertTaskItem(node *ast.ListItem, source []byte, skipped *skippedCollector) (jira4claude.ADFNode, []jira4claude.ADFNode) {
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
		"localId": skipped.nextLocalID(),
		"state":   state,
	})

	// Convert content, skipping the TaskCheckBox node itself
	content, nested := convertTaskItemContent(node, source, skipped)

	return jira4claude.ADFNode{
		Type:    "taskItem",
		Attrs:   attrs,
		Content: content,
	}, nested
}

// convertTaskItemContent converts the content of a task list item, skipping
// the checkbox. A taskItem holds inline content only, so:
//   - the item's first paragraph (the one with the checkbox) is its text, as is;
//   - a task list nested directly in the item is returned separately, to be
//     placed after the item as a sibling taskList (the native ADF form);
//   - any other block goes through taskItemInline, which keeps text-like
//     blocks after a hard break (reported as degraded) and reports the rest
//     as dropped.
func convertTaskItemContent(node *ast.ListItem, source []byte, skipped *skippedCollector) ([]jira4claude.ADFNode, []jira4claude.ADFNode) {
	var content, nested []jira4claude.ADFNode
	for block := node.FirstChild(); block != nil; block = block.NextSibling() {
		if block == node.FirstChild() {
			switch block.(type) {
			case *ast.Paragraph, *ast.TextBlock:
				// convertInlineRun skips the TaskCheckBox node.
				content = consolidateTextNodes(convertInlineRun(block.FirstChild(), source, nil, skipped))
				continue
			}
		}
		adfNode, ok := nodeToADF(block, source, skipped)
		if !ok {
			continue
		}
		if adfNode.Type == "taskList" {
			nested = append(nested, adfNode)
			continue
		}
		inline := taskItemInline(adfNode, skipped)
		if len(inline) == 0 {
			continue
		}
		skipped.degrade(adfNode.Type, adfNode.Type+" is not allowed inside taskItem; its text kept in the task item after a line break")
		content = appendAfterBreak(content, inline)
	}
	return content, nested
}

// taskItemInline flattens a block node into inline content for a taskItem.
// Paragraphs and headings give their inline content; text containers (lists,
// list items, blockquotes, panels) give their children's content separated
// by hard breaks. Any other node (code block, table, rule, a task list below
// the item's top level, and so on) cannot be kept faithfully and is reported
// as dropped. Kept nodes are not reported here: the caller reports the
// outermost block.
func taskItemInline(node jira4claude.ADFNode, skipped *skippedCollector) []jira4claude.ADFNode {
	switch node.Type {
	case "paragraph", "heading":
		return node.Content
	case "bulletList", "orderedList", "listItem", "blockquote", "panel":
		var result []jira4claude.ADFNode
		for _, child := range node.Content {
			result = appendAfterBreak(result, taskItemInline(child, skipped))
		}
		return result
	default:
		skipped.add(node.Type)
		return nil
	}
}

// appendAfterBreak appends inline to content, separated by a hardBreak when
// both are non-empty.
func appendAfterBreak(content, inline []jira4claude.ADFNode) []jira4claude.ADFNode {
	if len(inline) == 0 {
		return content
	}
	if len(content) > 0 {
		content = append(content, jira4claude.ADFNode{Type: "hardBreak"})
	}
	return append(content, inline...)
}

// randomLocalID creates a random UUID-like identifier for ADF nodes that
// require localId. It is the default localId source; n is ignored.
func randomLocalID(int) string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// convertTable converts a goldmark GFM table to an ADF table node.
// The goldmark AST has Table → TableHeader (first child) → TableCell,
// then TableRow children → TableCell. ADF uses tableRow for both,
// with tableHeader vs tableCell for cell types.
func convertTable(node *extast.Table, source []byte, skipped *skippedCollector) jira4claude.ADFNode {
	var rows []jira4claude.ADFNode

	for child := node.FirstChild(); child != nil; child = child.NextSibling() {
		switch row := child.(type) {
		case *extast.TableHeader:
			rows = append(rows, convertTableRow(row, source, true, skipped))
		case *extast.TableRow:
			rows = append(rows, convertTableRow(row, source, false, skipped))
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
func convertTableRow(node ast.Node, source []byte, isHeader bool, skipped *skippedCollector) jira4claude.ADFNode {
	var cells []jira4claude.ADFNode

	for child := node.FirstChild(); child != nil; child = child.NextSibling() {
		if cell, ok := child.(*extast.TableCell); ok {
			cellType := "tableCell"
			if isHeader {
				cellType = "tableHeader"
			}
			// ADF requires cell content, so an empty cell holds an empty paragraph.
			content := convertInlineContent(cell, source, skipped)
			cells = append(cells, jira4claude.ADFNode{
				Type:    cellType,
				Content: []jira4claude.ADFNode{{Type: "paragraph", Content: content}},
			})
		}
	}

	return jira4claude.ADFNode{
		Type:    "tableRow",
		Content: cells,
	}
}

// filterCodeCompatibleMarks returns the subset of marks that may co-exist with
// the ADF code mark. Per Atlassian's ADF spec the code mark is exclusive — only
// link may be applied to the same text node alongside code.
func filterCodeCompatibleMarks(marks []jira4claude.ADFMark) []jira4claude.ADFMark {
	if len(marks) == 0 {
		return nil
	}
	var result []jira4claude.ADFMark
	for _, m := range marks {
		if m.Type == "link" {
			result = append(result, m)
		}
	}
	return result
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
