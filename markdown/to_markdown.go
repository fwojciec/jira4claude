package markdown

import (
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/fwojciec/jira4claude"
)

// toMarkdown converts an Atlassian Document Format (ADF) document to GitHub-flavored markdown.
// This is useful for displaying Jira issue content in a readable format.
// Returns a ConversionIssue for any element that was skipped during conversion.
// Protected nodes are written as tokens (see protect.go).
func toMarkdown(adfDoc *jira4claude.ADFNode) (string, []jira4claude.ConversionIssue) {
	if adfDoc == nil {
		return "", nil
	}

	if len(adfDoc.Content) == 0 {
		return "", nil
	}

	md, skipped := readDocument(adfDoc)
	return md, skipped.issues()
}

// adfNodeToGFM converts a single ADF node to markdown. A node it does not
// know is protected: it is written as a token on a line of its own.
func adfNodeToGFM(node *jira4claude.ADFNode, skipped *skippedCollector) string {
	container := skipped.blockContainer()
	skipped.enterBlock(node.Type)
	defer skipped.leaveBlock()

	switch node.Type {
	case "paragraph":
		return adfInlineToGFM(node, skipped)
	case "heading":
		return adfHeadingToGFM(node, skipped)
	case "codeBlock":
		return adfCodeBlockToGFM(node, skipped)
	case "bulletList":
		return adfBulletListToGFM(node, skipped)
	case "orderedList":
		return adfOrderedListToGFM(node, skipped)
	case "blockquote":
		return adfBlockquoteToGFM(node, skipped)
	case "panel":
		return adfPanelToGFM(node, skipped)
	case "expand", "nestedExpand":
		return adfExpandToGFM(node, skipped)
	case "rule":
		return "---"
	case "table":
		return adfTableToGFM(node, skipped)
	case "taskList":
		return adfTaskListToGFM(node, skipped)
	case "decisionList":
		return adfDecisionListToGFM(node, skipped)
	case "layoutSection", "layoutColumn":
		return adfFlattenToGFM(node, skipped)
	case "hardBreak":
		return "\n"
	case "text":
		// Jira sometimes places bare "text" nodes at block level (e.g., inside listItem
		// without a wrapping paragraph). Reuse the inline rendering path so any marks
		// on the text node (strong/em/code/link) are preserved.
		wrapper := &jira4claude.ADFNode{
			Content: []jira4claude.ADFNode{*node},
		}
		return adfInlineToGFM(wrapper, skipped)
	default:
		// mediaSingle, mediaGroup, a bare media, extensions, and any type
		// this reader does not know.
		return skipped.protectBlock(node, container)
	}
}

// adfHeadingToGFM converts an ADF heading to markdown.
func adfHeadingToGFM(node *jira4claude.ADFNode, skipped *skippedCollector) string {
	level := 1
	if node.Attrs != nil {
		var attrs map[string]any
		if err := json.Unmarshal(node.Attrs, &attrs); err == nil {
			if l, ok := attrs["level"].(float64); ok {
				level = int(l)
			}
		}
	}

	text := escapeHeadingClose(adfInlineLineToGFM(node, skipped, true))
	return strings.Repeat("#", level) + " " + text
}

// adfCodeBlockToGFM converts an ADF codeBlock to a fenced code block.
func adfCodeBlockToGFM(node *jira4claude.ADFNode, skipped *skippedCollector) string {
	lang := ""
	if node.Attrs != nil {
		var attrs map[string]any
		if err := json.Unmarshal(node.Attrs, &attrs); err == nil {
			if l, ok := attrs["language"].(string); ok {
				lang = l
			}
		}
	}

	// The info string's first word is the language, so a language with
	// whitespace in it cannot be written whole.
	if word := infoFirstWord(lang); word != lang {
		if skipped != nil {
			skipped.degrade("codeBlock", "a code block language containing whitespace cannot be written in Markdown; only its first word is kept")
		}
		lang = word
	}

	var code string
	for _, child := range node.Content {
		if child.Type == "text" {
			code += child.Text
		}
	}
	if text, changed := writableText(code); changed {
		code = text
		if skipped != nil {
			skipped.degrade("codeBlock", unwritableTextDetail)
		}
	}

	// The info string of a backtick fence cannot contain a backtick, so such
	// a language takes a tilde fence. The fence is longer than any run of
	// its character that starts a line of the code, so no line closes it.
	fenceChar := "`"
	if strings.Contains(lang, "`") {
		fenceChar = "~"
	}
	fence := strings.Repeat(fenceChar, max(3, longestLineStartRun(code, fenceChar[0])+1))
	info := lang
	if strings.HasPrefix(info, fenceChar) {
		// Written directly after the fence it would lengthen the fence.
		info = " " + info
	}
	if code == "" {
		return fence + info + "\n" + fence
	}
	return fence + info + "\n" + code + "\n" + fence
}

// infoFirstWord returns the first word of a fenced code block info string,
// which is what the write side takes as the language. The words are
// separated by the blanks goldmark trims and splits on; a line ending cannot
// be in an info string at all.
func infoFirstWord(info string) string {
	const blanks = " \t\n\r"
	info = strings.TrimLeft(info, blanks)
	if i := strings.IndexAny(info, blanks); i >= 0 {
		info = info[:i]
	}
	return info
}

// longestLineStartRun returns the length of the longest run of c that
// starts a line of text, after any indentation.
func longestLineStartRun(text string, c byte) int {
	longest := 0
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimLeft(line, " \t")
		n := 0
		for n < len(line) && line[n] == c {
			n++
		}
		longest = max(longest, n)
	}
	return longest
}

// codeSpan writes text as a CommonMark code span. The delimiter is a
// backtick run longer than any run inside the text, so none closes the span
// early. The parser strips one space from each end when the text both starts
// and ends with a space or a line ending and is not all blanks, and a
// backtick at either edge would join the delimiter, so in those cases the
// text is padded with one space on each side. A line ending has no form in
// a code span (it reads as a space) and is written as a space; the caller
// reports it.
func codeSpan(text string) string {
	text = strings.ReplaceAll(text, "\n", " ")
	longest, run := 0, 0
	for i := range len(text) {
		if text[i] == '`' {
			run++
			longest = max(longest, run)
		} else {
			run = 0
		}
	}
	delim := strings.Repeat("`", longest+1)

	blank := strings.Trim(text, " \t\r") == ""
	spaceEdges := strings.HasPrefix(text, " ") && strings.HasSuffix(text, " ") && !blank
	if spaceEdges || strings.HasPrefix(text, "`") || strings.HasSuffix(text, "`") {
		text = " " + text + " "
	}
	return delim + text + delim
}

// Markdown merges adjacent lists that use the same marker into one list, but
// starts a new list when the bullet character or the ordered delimiter
// changes. Sibling lists are separate ADF nodes, so each list takes the first
// of these markers that the list emitted right before it does not use:
// bulletList, taskList and decisionList all render as bullets and form one
// family; orderedList forms the other. A list with no list neighbour takes
// the first marker, so adjacent lists alternate between the first two.
const (
	bulletMarkers     = "-*+"
	orderedDelimiters = ".)"
)

// pickMarker returns the first marker in candidates that is not in avoid.
func pickMarker(candidates string, avoid ...string) string {
	for _, c := range candidates {
		if marker := string(c); !slices.Contains(avoid, marker) {
			return marker
		}
	}
	return candidates[:1]
}

// siblingBlocks renders a container's child blocks one at a time, choosing
// list markers so that no list uses the marker of the list emitted right
// before it, which would merge the two on read. Blocks that render empty do
// not separate their neighbours, so they leave the choice unchanged.
type siblingBlocks struct {
	skipped *skippedCollector
	// prev is the marker of the last emitted block when it is a list, else "".
	prev string
	// avoid is a marker the first emitted block must not use when it is a
	// list: the parent item's marker, when the list goes on the parent's
	// marker line ("- - -" would be a thematic break).
	avoid string
}

// render returns the Markdown of node, or "" when it renders empty.
func (s *siblingBlocks) render(node *jira4claude.ADFNode) string {
	var part, marker string
	switch node.Type {
	case "bulletList", "taskList", "decisionList", "orderedList":
		// Lists rendered here skip adfNodeToGFM, so they record the block
		// they open themselves, for the tokens inside them.
		s.skipped.enterBlock(node.Type)
		defer s.skipped.leaveBlock()
	}
	switch node.Type {
	case "bulletList":
		marker = pickMarker(bulletMarkers, s.prev, s.avoid)
		part = adfBulletListMarkerToGFM(node, s.skipped, marker)
	case "taskList":
		marker = pickMarker(bulletMarkers, s.prev, s.avoid)
		part = adfTaskListMarkerToGFM(node, s.skipped, marker)
	case "decisionList":
		marker = pickMarker(bulletMarkers, s.prev, s.avoid)
		part = adfDecisionListMarkerToGFM(node, s.skipped, marker)
	case "orderedList":
		marker = pickMarker(orderedDelimiters, s.prev)
		part = adfOrderedListMarkerToGFM(node, s.skipped, marker)
	default:
		part = adfNodeToGFM(node, s.skipped)
	}
	s.emitted(part, marker)
	return part
}

// emitted records that part was rendered with marker ("" for a block that
// is not a list).
func (s *siblingBlocks) emitted(part, marker string) {
	if part == "" {
		return
	}
	s.prev = marker
	s.avoid = ""
}

// adfBulletListToGFM converts an ADF bulletList to markdown.
func adfBulletListToGFM(node *jira4claude.ADFNode, skipped *skippedCollector) string {
	return adfBulletListMarkerToGFM(node, skipped, bulletMarkers[:1])
}

// adfBulletListMarkerToGFM converts an ADF bulletList to markdown, using
// marker as the bullet character.
func adfBulletListMarkerToGFM(node *jira4claude.ADFNode, skipped *skippedCollector, marker string) string {
	items := make([]string, 0, len(node.Content))
	for i := range node.Content {
		child := &node.Content[i]
		if child.Type != "listItem" {
			items = append(items, listItemLine(marker, skipped.protectMisplaced(child, node.Type)))
			continue
		}
		items = append(items, listItemLine(marker, adfListItemToGFM(child, skipped, marker, len(marker)+1)))
	}

	return strings.Join(items, "\n")
}

// listItemLine puts an item's rendered content after its marker. An empty
// item is the bare marker, with no trailing space.
func listItemLine(marker, content string) string {
	if content == "" {
		return marker
	}
	return marker + " " + content
}

// adfOrderedListToGFM converts an ADF orderedList to markdown.
func adfOrderedListToGFM(node *jira4claude.ADFNode, skipped *skippedCollector) string {
	return adfOrderedListMarkerToGFM(node, skipped, orderedDelimiters[:1])
}

// adfOrderedListMarkerToGFM converts an ADF orderedList to markdown, using
// delimiter (. or )) after each item number.
func adfOrderedListMarkerToGFM(node *jira4claude.ADFNode, skipped *skippedCollector, delimiter string) string {
	startNum := 1
	if node.Attrs != nil {
		var attrs map[string]any
		if err := json.Unmarshal(node.Attrs, &attrs); err == nil {
			if order, ok := attrs["order"].(float64); ok {
				startNum = int(order)
			}
		}
	}

	items := make([]string, 0, len(node.Content))
	num := startNum - 1
	for i := range node.Content {
		child := &node.Content[i]
		num++
		marker := strconv.Itoa(num) + delimiter
		if child.Type != "listItem" {
			items = append(items, listItemLine(marker, skipped.protectMisplaced(child, node.Type)))
			continue
		}
		items = append(items, listItemLine(marker, adfListItemToGFM(child, skipped, marker, len(marker)+1)))
	}

	return strings.Join(items, "\n")
}

// emptyLeadDetail explains the degraded issue reported for a list item that
// opens with an empty paragraph followed by anything but a list.
const emptyLeadDetail = "an empty paragraph at the start of a list item, before content other than a list, " +
	"has no Markdown form: it is left out"

// adfListItemToGFM renders the content of a list item that follows a list
// marker width characters wide. The first block goes on the marker line; any
// further blocks follow after a blank line, so each stays its own block.
// Every line after the first is indented by the marker width, so all of the
// item's content, including lines after a line break inside a paragraph,
// stays inside the item. An item with a single block renders with no blank
// line.
//
// An item's empty paragraph renders as nothing. An item holding only one
// reads back from the bare marker, and one whose empty paragraph comes
// before a list reads back from the list on the marker line; that list takes
// a marker other than the item's (marker), so "- - -" does not form a
// thematic break. An empty paragraph before other content has no Markdown
// form and is reported.
//
// A block whose last line is a bare marker (an empty item) is followed by a
// single line break instead of a blank line: after an empty item, a blank
// line ends the enclosing item, and the following block would escape it.
func adfListItemToGFM(node *jira4claude.ADFNode, skipped *skippedCollector, marker string, width int) string {
	blocks := siblingBlocks{skipped: skipped}
	if content := node.Content; len(content) > 0 && isEmptyParagraph(content[0]) {
		blocks.avoid = marker
		if len(content) > 1 && content[1].Type != "bulletList" && content[1].Type != "orderedList" && skipped != nil {
			skipped.degrade("listItem", emptyLeadDetail)
		}
	}
	parts := blocks.renderAll(node.Content)

	var text strings.Builder
	for i, part := range parts {
		if i > 0 {
			if endsWithBareMarker(parts[i-1]) {
				text.WriteString("\n")
			} else {
				text.WriteString("\n\n")
			}
		}
		text.WriteString(part)
	}
	return indentContinuation(text.String(), strings.Repeat(" ", width))
}

// endsWithBareMarker reports whether the last line of text holds nothing but
// list markers, as an empty list item (or a run of items that open with
// lists, ending in an empty one) renders.
func endsWithBareMarker(text string) bool {
	fields := strings.Fields(text[strings.LastIndex(text, "\n")+1:])
	if len(fields) == 0 {
		return false
	}
	for _, f := range fields {
		if !isListMarker(f) {
			return false
		}
	}
	return true
}

// isListMarker reports whether s is a bullet character or an ordered list
// number (one to nine digits, as CommonMark allows) followed by its
// delimiter.
func isListMarker(s string) bool {
	if len(s) == 1 && strings.Contains(bulletMarkers, s) {
		return true
	}
	if len(s) < 2 || len(s) > 10 {
		return false
	}
	number, delimiter := s[:len(s)-1], s[len(s)-1:]
	return strings.Trim(number, "0123456789") == "" && strings.Contains(orderedDelimiters, delimiter)
}

// adfBlocksToGFM renders a container's children, one string per block,
// leaving out blocks that render empty. Consecutive inline nodes (which Jira
// sometimes places directly in a container) render together as one block.
func adfBlocksToGFM(children []jira4claude.ADFNode, skipped *skippedCollector) []string {
	blocks := siblingBlocks{skipped: skipped}
	return blocks.renderAll(children)
}

// renderAll renders children as adfBlocksToGFM describes, choosing list
// markers as render does.
func (s *siblingBlocks) renderAll(children []jira4claude.ADFNode) []string {
	var parts []string
	for i := 0; i < len(children); {
		var part string
		if isInlineNode(children[i].Type) {
			run := jira4claude.ADFNode{}
			for ; i < len(children) && isInlineNode(children[i].Type); i++ {
				run.Content = append(run.Content, children[i])
			}
			part = adfInlineToGFM(&run, s.skipped)
			s.emitted(part, "")
		} else {
			part = s.render(&children[i])
			i++
		}
		if part != "" {
			parts = append(parts, part)
		}
	}
	return parts
}

// isInlineNode reports whether an ADF node type is inline content.
func isInlineNode(nodeType string) bool {
	switch nodeType {
	case "text", "hardBreak", "mention", "emoji", "inlineCard", "status", "date",
		"mediaInline", "placeholder", "inlineExtension":
		return true
	default:
		return false
	}
}

// indentContinuation prefixes every line of text after the first with
// indent. Blank lines stay empty: they do not end a container.
func indentContinuation(text, indent string) string {
	lines := strings.Split(text, "\n")
	for i := 1; i < len(lines); i++ {
		if lines[i] != "" {
			lines[i] = indent + lines[i]
		}
	}
	return strings.Join(lines, "\n")
}

// quoteLines prefixes every line of text with "> ", or a bare ">" for a
// blank line.
func quoteLines(text string) string {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		if line == "" {
			lines[i] = ">"
		} else {
			lines[i] = "> " + line
		}
	}
	return strings.Join(lines, "\n")
}

// adfBlockquoteToGFM converts an ADF blockquote to markdown. Children are
// separated by a blank quoted line so each stays its own block, and every
// line carries the quote prefix.
func adfBlockquoteToGFM(node *jira4claude.ADFNode, skipped *skippedCollector) string {
	parts := adfBlocksToGFM(node.Content, skipped)
	if len(parts) == 0 {
		return ""
	}
	return quoteLines(strings.Join(parts, "\n\n"))
}

// panelTypeToAlert maps ADF panel types to GitHub alert names.
func panelTypeToAlert(panelType string) string {
	switch panelType {
	case "info":
		return "NOTE"
	case "warning":
		return "WARNING"
	case "error":
		return "CAUTION"
	case "success":
		return "TIP"
	case "note":
		return "IMPORTANT"
	default:
		return "NOTE"
	}
}

// adfPanelToGFM converts an ADF panel to a GitHub alert blockquote.
func adfPanelToGFM(node *jira4claude.ADFNode, skipped *skippedCollector) string {
	panelType := "info"
	if node.Attrs != nil {
		var attrs map[string]any
		if err := json.Unmarshal(node.Attrs, &attrs); err == nil {
			if pt, ok := attrs["panelType"].(string); ok {
				panelType = pt
			}
		}
	}

	header := "> [!" + panelTypeToAlert(panelType) + "]"
	parts := adfBlocksToGFM(node.Content, skipped)
	if len(parts) == 0 {
		return header
	}
	// Children are separated by a blank quoted line, as in a blockquote.
	body := quoteLines(strings.Join(parts, "\n\n"))
	if first, _, _ := strings.Cut(parts[0], "\n"); first == "-" {
		// A bare "-" (an empty list item) right under the header line would
		// be a setext underline, turning the header into a heading.
		return header + "\n>\n" + body
	}
	return header + "\n" + body
}

// adfExpandToGFM converts an ADF expand or nestedExpand to an HTML details block.
func adfExpandToGFM(node *jira4claude.ADFNode, skipped *skippedCollector) string {
	title := ""
	if node.Attrs != nil {
		var attrs map[string]any
		if err := json.Unmarshal(node.Attrs, &attrs); err == nil {
			if t, ok := attrs["title"].(string); ok {
				title = t
			}
		}
	}

	blocks := siblingBlocks{skipped: skipped}
	var parts []string
	for i := range node.Content {
		part := blocks.render(&node.Content[i])
		if part != "" {
			parts = append(parts, part)
		}
	}

	body := strings.Join(parts, "\n\n")
	return fmt.Sprintf("<details><summary>%s</summary>\n\n%s\n\n</details>", title, body)
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
			// Kept as a row of one cell holding its token.
			token := strings.ReplaceAll(skipped.protectMisplaced(rowNode, node.Type), "|", "\\|")
			firstRendered = false
			maxCols = max(maxCols, 1)
			lines = append(lines, "| "+token+" |")
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
	if node.Type != "tableCell" && node.Type != "tableHeader" {
		// Kept as a cell holding its token.
		return strings.ReplaceAll(skipped.protectMisplaced(node, "tableRow"), "|", "\\|")
	}
	var parts []string
	for i := range node.Content {
		child := &node.Content[i]
		var part string
		if child.Type == "paragraph" {
			// A GFM table row is one line, so a hard break becomes a space.
			part = adfInlineLineToGFM(child, skipped, true)
		} else {
			part = adfNodeToGFM(child, skipped)
		}
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
// A taskList nested in the list (ADF places it after the item it belongs to)
// is rendered indented under the preceding item, so it reads back nested.
// Continuation lines of an item's text are indented so they stay in the item.
func adfTaskListToGFM(node *jira4claude.ADFNode, skipped *skippedCollector) string {
	return adfTaskListMarkerToGFM(node, skipped, bulletMarkers[:1])
}

// adfTaskListMarkerToGFM converts an ADF taskList as adfTaskListToGFM does,
// using marker as the bullet character. Task lists nested under the same
// item are adjacent lists there, so each takes a marker the one before it
// does not use.
func adfTaskListMarkerToGFM(node *jira4claude.ADFNode, skipped *skippedCollector, marker string) string {
	var items []string
	prevNested := "" // marker of the last list nested under the current item
	for i := range node.Content {
		taskItem := &node.Content[i]
		if taskItem.Type == "taskList" {
			nestedMarker := pickMarker(bulletMarkers, prevNested)
			nested := adfTaskListMarkerToGFM(taskItem, skipped, nestedMarker)
			if nested == "" {
				continue
			}
			prevNested = nestedMarker
			if len(items) == 0 {
				// Not valid ADF: no item to nest under, so the list is
				// rendered at its parent's level.
				skipped.degrade("taskList", "taskList nested before any taskItem; rendered at its parent's level")
				items = append(items, nested)
				continue
			}
			items[len(items)-1] += indentContinuation("\n"+nested, "  ")
			continue
		}
		if taskItem.Type == "blockTaskItem" {
			// Schema-valid: a task item holding paragraphs (or extensions),
			// read as a task item. The writer makes a taskItem of it.
			skipped.degrade("blockTaskItem", "a task item holding paragraphs is written back as a taskItem")
		} else if taskItem.Type != "taskItem" {
			// An item without a checkbox would not read back as part of
			// the task list.
			skipped.dropMisplaced(taskItem, node.Type)
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

		// A taskItem holds inline content (schema-compliant) or, in legacy
		// ADF from Jira, paragraph-wrapped blocks; both render as list item
		// content under the marker. The checkbox comes first on the marker
		// line, so no list there can join the marker in a thematic break.
		text := adfListItemToGFM(taskItem, skipped, "", len(marker)+1)
		items = append(items, marker+" "+checkbox+" "+text)
		prevNested = ""
	}

	return strings.Join(items, "\n")
}

// adfFlattenToGFM flattens a container node's content sequentially, ignoring
// the container's own structure (e.g., layoutSection columns become sequential blocks).
func adfFlattenToGFM(node *jira4claude.ADFNode, skipped *skippedCollector) string {
	blocks := siblingBlocks{skipped: skipped}
	var parts []string
	for i := range node.Content {
		part := blocks.render(&node.Content[i])
		if part != "" {
			parts = append(parts, part)
		}
	}
	return strings.Join(parts, "\n\n")
}

// adfDecisionListToGFM converts an ADF decisionList to a plain bullet list.
func adfDecisionListToGFM(node *jira4claude.ADFNode, skipped *skippedCollector) string {
	return adfDecisionListMarkerToGFM(node, skipped, bulletMarkers[:1])
}

// adfDecisionListMarkerToGFM converts an ADF decisionList as
// adfDecisionListToGFM does, using marker as the bullet character.
func adfDecisionListMarkerToGFM(node *jira4claude.ADFNode, skipped *skippedCollector, marker string) string {
	var items []string
	for i := range node.Content {
		child := &node.Content[i]
		var text string
		if child.Type == "decisionItem" {
			text = adfListItemToGFM(child, skipped, marker, len(marker)+1)
		} else {
			text = skipped.protectMisplaced(child, node.Type)
		}
		if text != "" {
			items = append(items, marker+" "+text)
		}
	}
	return strings.Join(items, "\n")
}

// adfInlineToGFM converts inline content to markdown.
func adfInlineToGFM(node *jira4claude.ADFNode, skipped *skippedCollector) string {
	return adfInlineLineToGFM(node, skipped, false)
}

// hardBreakDetail explains the degraded issue reported when a hardBreak has
// no Markdown form where it stands.
const hardBreakDetail = "a line break at the end of a paragraph, or inside a heading or table cell, " +
	"cannot be written in Markdown: it is left out at the end and read as a space elsewhere"

// adfInlineLineToGFM converts inline content to markdown. A hardBreak becomes
// a CommonMark hard break (a backslash before the newline). singleLine is for
// content that must stay on one line (a heading, a table cell): there a
// hardBreak becomes a space. A hardBreak with nothing but hardBreaks after it
// is left out, because a backslash at the end of a paragraph is literal text.
// Both changes are reported as degraded.
func adfInlineLineToGFM(node *jira4claude.ADFNode, skipped *skippedCollector, singleLine bool) string {
	if len(node.Content) == 0 {
		return ""
	}

	// Every escaping rule that looks at a neighbour sees this list, which
	// holds only nodes that emit something, with label nodes already turned
	// into the text they read as.
	items := emittingInline(node.Content, skipped)
	for i := range items {
		if items[i].Type != "text" {
			continue
		}
		if text, changed := writableText(items[i].Text); changed {
			items[i].Text = text
			if skipped != nil {
				skipped.degrade("text", unwritableTextDetail)
			}
		}
		if marks, changed := writableHrefs(items[i].Marks); changed {
			items[i].Marks = marks
			if skipped != nil {
				skipped.degrade("link", hrefDetail)
			}
		}
	}
	texts := renderTextItems(items, skipped)

	var result strings.Builder
	for i := range items {
		child := &items[i]
		switch child.Type {
		case "text":
			if skipped != nil && hasCodeMark(child.Marks) && strings.Contains(child.Text, "\n") {
				skipped.degrade("text", "a line break inside inline code cannot be written in a Markdown code span; it is read as a space")
			}
			result.WriteString(texts[i])
		case "hardBreak":
			switch {
			case onlyHardBreaksFrom(items, i):
				if skipped != nil {
					skipped.degrade("hardBreak", hardBreakDetail)
				}
			case singleLine:
				result.WriteString(" ")
				if skipped != nil {
					skipped.degrade("hardBreak", hardBreakDetail)
				}
			default:
				// A plain text that ends in a backslash writes it as a
				// character reference (see withBreakBackslash).
				result.WriteString("\\\n")
			}
		case tokenItemType:
			result.WriteString(child.Text)
		case "inlineCard":
			result.WriteString(adfInlineCardToGFM(child, skipped))
		}
	}

	checkMarksReadBack(items, result.String(), singleLine, skipped)
	return result.String()
}

// emittingInline returns the inline nodes that produce output, in order.
// Nodes whose output is a plain label (mention, emoji, date, status) become
// text nodes holding that label, so it is escaped like any other text; a
// status keeps its bold. Protected nodes (mention, mediaInline and inline
// nodes this reader does not know) become token items (tokenItemType), which
// hold their own Markdown and are never merged with text. Nodes that emit
// nothing are left out, and an inlineCard without a URL is recorded as
// dropped.
func emittingInline(content []jira4claude.ADFNode, skipped *skippedCollector) []jira4claude.ADFNode {
	items := make([]jira4claude.ADFNode, 0, len(content))
	// label appends text, merging it into the previous item when that is
	// text with the same written marks, in any order: written separately,
	// the two would get delimiters between them ("**a\***" + "**b**") that
	// do not read back. Marks Markdown does not carry (textColor) are left
	// out of the comparison, since they are not written either way.
	// Label nodes merge too: they are written as plain text with no syntax
	// of their own, so merging changes nothing but the delimiters. Code
	// merges too: two code spans written side by side would join their
	// backtick runs, and one span keeps the text exactly (see codeSpan).
	label := func(text string, marks []jira4claude.ADFMark) {
		if text == "" {
			return
		}
		if last := len(items) - 1; last >= 0 && items[last].Type == "text" &&
			sameWrittenMarks(items[last].Marks, marks) {
			items[last].Text += text
			return
		}
		items = append(items, jira4claude.ADFNode{Type: "text", Text: text, Marks: marks})
	}
	for i := range content {
		child := &content[i]
		switch child.Type {
		case "text":
			label(child.Text, child.Marks)
		case "hardBreak":
			items = append(items, *child)
		case "emoji":
			label(adfEmojiToGFM(child), nil)
		case "date":
			label(adfDateToGFM(child), nil)
		case "status":
			label(adfStatusToGFM(child), []jira4claude.ADFMark{{Type: "strong"}})
		case "inlineCard":
			if inlineCardURL(child) != "" {
				items = append(items, *child)
			} else if skipped != nil {
				skipped.add("inlineCard")
			}
		case "placeholder":
			// Drop silently — placeholder nodes produce no output.
		default:
			// mention, mediaInline, and any type this reader does not know.
			items = append(items, skipped.protectInline(child))
		}
	}
	return items
}

// onlyHardBreaksFrom reports whether nodes[i:] holds nothing but hardBreaks.
func onlyHardBreaksFrom(nodes []jira4claude.ADFNode, i int) bool {
	for _, n := range nodes[i:] {
		if n.Type != "hardBreak" {
			return false
		}
	}
	return true
}

// applyMarks wraps text with the appropriate markdown syntax for its marks.
//
// Mark nesting order (outermost → innermost):
//
//	link → subsup → underline → strong → em → strike → code
//
// Code is exclusive: when present it suppresses em, strong, and strike
// because Markdown doesn't support emphasis inside backticks.
// text is already escaped, with any edge blank or tilde that would stop a
// delimiter run from opening or closing encoded (see inline_marks.go); it is
// wrapped as is. delim is the emphasis character, '*' or '_' (0 means '*').
func applyMarks(text string, marks []jira4claude.ADFMark, delim byte) string {
	var hasStrong, hasEm, hasCode, hasStrike, hasUnderline, hasLink bool
	var href string
	var subsupType string // "sub" or "sup"

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
		case "underline":
			hasUnderline = true
		case "subsup":
			if mark.Attrs != nil {
				var attrs map[string]any
				if err := json.Unmarshal(mark.Attrs, &attrs); err == nil {
					if t, ok := attrs["type"].(string); ok {
						subsupType = t
					}
				}
			}
		case "link":
			hasLink = true
			href = linkHref(mark)
		}
	}

	if delim == 0 {
		delim = '*'
	}
	var result string

	// Apply marks innermost → outermost.
	// Code is exclusive: suppresses em/strong/strike. A code span keeps its
	// blanks, so code text is not trimmed.
	if hasCode {
		result = codeSpan(text)
	} else {
		result = text
		if hasStrike {
			result = "~~" + result + "~~"
		}
		if hasEm {
			result = string(delim) + result + string(delim)
		}
		if hasStrong {
			result = strings.Repeat(string(delim), 2) + result + strings.Repeat(string(delim), 2)
		}
	}
	if hasUnderline {
		result = "<u>" + result + "</u>"
	}
	switch subsupType {
	case "sub":
		result = "<sub>" + result + "</sub>"
	case "sup":
		result = "<sup>" + result + "</sup>"
	}
	if hasLink && result != href {
		result = "[" + result + "](" + linkDestination(href) + ")"
	}

	return result
}

// adfEmojiToGFM converts an ADF emoji node to its Unicode character or :shortName:.
func adfEmojiToGFM(node *jira4claude.ADFNode) string {
	if node.Attrs == nil {
		return ""
	}
	var attrs map[string]any
	if err := json.Unmarshal(node.Attrs, &attrs); err != nil {
		return ""
	}
	if text, ok := attrs["text"].(string); ok && text != "" {
		return text
	}
	if shortName, ok := attrs["shortName"].(string); ok && shortName != "" {
		return shortName
	}
	return ""
}

// inlineCardURL returns the url of an ADF inlineCard node, or "".
func inlineCardURL(node *jira4claude.ADFNode) string {
	if node.Attrs == nil {
		return ""
	}
	var attrs map[string]any
	if err := json.Unmarshal(node.Attrs, &attrs); err != nil {
		return ""
	}
	url, _ := attrs["url"].(string)
	return url
}

// adfInlineCardToGFM converts an ADF inlineCard node to a markdown link whose
// text is its URL. Inside the brackets the URL is link text, which the write
// side decodes, so it is escaped as link text; the destination is written by
// linkDestination. A URL the write side would change is written as it would
// read it, and reported.
func adfInlineCardToGFM(node *jira4claude.ADFNode, skipped *skippedCollector) string {
	url, changed := writableText(inlineCardURL(node))
	if changed && skipped != nil {
		skipped.degrade("inlineCard", hrefDetail)
	}
	return bracketedURL(url)
}

// adfStatusToGFM returns the label of an ADF status node, which is read out
// in bold.
func adfStatusToGFM(node *jira4claude.ADFNode) string {
	if node.Attrs == nil {
		return ""
	}
	var attrs map[string]any
	if err := json.Unmarshal(node.Attrs, &attrs); err != nil {
		return ""
	}
	text, _ := attrs["text"].(string)
	return text
}

// adfDateToGFM converts an ADF date node to a human-readable date string.
func adfDateToGFM(node *jira4claude.ADFNode) string {
	if node.Attrs == nil {
		return ""
	}
	var attrs map[string]any
	if err := json.Unmarshal(node.Attrs, &attrs); err != nil {
		return ""
	}
	if ts, ok := attrs["timestamp"].(string); ok && ts != "" {
		ms, err := strconv.ParseInt(ts, 10, 64)
		if err != nil {
			return ts
		}
		return time.UnixMilli(ms).UTC().Format("2006-01-02")
	}
	return ""
}
