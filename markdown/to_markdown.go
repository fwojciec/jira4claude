package markdown

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/fwojciec/jira4claude"
)

// toMarkdown converts an Atlassian Document Format (ADF) document to GitHub-flavored markdown.
// This is useful for displaying Jira issue content in a readable format.
// Returns a ConversionIssue for any element that was skipped during conversion.
func toMarkdown(adfDoc *jira4claude.ADFNode) (string, []jira4claude.ConversionIssue) {
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

	return strings.Join(parts, "\n\n"), skipped.issues()
}

// adfNodeToGFM converts a single ADF node to markdown.
func adfNodeToGFM(node *jira4claude.ADFNode, skipped *skippedCollector) string {
	switch node.Type {
	case "paragraph":
		return adfInlineToGFM(node, skipped)
	case "heading":
		return adfHeadingToGFM(node, skipped)
	case "codeBlock":
		return adfCodeBlockToGFM(node)
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
	case "mediaSingle":
		return adfMediaSingleToGFM(node)
	case "mediaGroup":
		return adfMediaGroupToGFM(node)
	case "bodiedExtension", "extension", "multiBodiedExtension", "extensionFrame":
		skipped.add(node.Type)
		return ""
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
		// Record the skipped node type
		skipped.add(node.Type)
		return ""
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
		items = append(items, "- "+adfListItemToGFM(child, skipped, len("- ")))
	}

	return strings.Join(items, "\n")
}

// adfOrderedListToGFM converts an ADF orderedList to markdown.
func adfOrderedListToGFM(node *jira4claude.ADFNode, skipped *skippedCollector) string {
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
		if child.Type != "listItem" {
			continue
		}
		num++
		marker := strconv.Itoa(num) + ". "
		items = append(items, marker+adfListItemToGFM(child, skipped, len(marker)))
	}

	return strings.Join(items, "\n")
}

// adfListItemToGFM renders the content of a list item that follows a list
// marker width characters wide. The first block goes on the marker line; any
// further blocks follow after a blank line, so each stays its own block.
// Every line after the first is indented by the marker width, so all of the
// item's content, including lines after a line break inside a paragraph,
// stays inside the item. An item with a single block renders with no blank
// line.
func adfListItemToGFM(node *jira4claude.ADFNode, skipped *skippedCollector, width int) string {
	parts := adfBlocksToGFM(node.Content, skipped)
	return indentContinuation(strings.Join(parts, "\n\n"), strings.Repeat(" ", width))
}

// adfBlocksToGFM renders a container's children, one string per block,
// leaving out blocks that render empty. Consecutive inline nodes (which Jira
// sometimes places directly in a container) render together as one block.
func adfBlocksToGFM(children []jira4claude.ADFNode, skipped *skippedCollector) []string {
	var parts []string
	for i := 0; i < len(children); {
		var part string
		if isInlineNode(children[i].Type) {
			run := jira4claude.ADFNode{}
			for ; i < len(children) && isInlineNode(children[i].Type); i++ {
				run.Content = append(run.Content, children[i])
			}
			part = adfInlineToGFM(&run, skipped)
		} else {
			part = adfNodeToGFM(&children[i], skipped)
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
	return header + "\n" + quoteLines(strings.Join(parts, "\n\n"))
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

	var parts []string
	for i := range node.Content {
		part := adfNodeToGFM(&node.Content[i], skipped)
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
	var items []string
	for i := range node.Content {
		taskItem := &node.Content[i]
		if taskItem.Type == "taskList" {
			nested := adfTaskListToGFM(taskItem, skipped)
			if nested == "" {
				continue
			}
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

		// A taskItem holds inline content (schema-compliant) or, in legacy
		// ADF from Jira, paragraph-wrapped blocks; both render as list item
		// content under the "- " marker.
		text := adfListItemToGFM(taskItem, skipped, len("- "))
		items = append(items, "- "+checkbox+" "+text)
	}

	return strings.Join(items, "\n")
}

// adfFlattenToGFM flattens a container node's content sequentially, ignoring
// the container's own structure (e.g., layoutSection columns become sequential blocks).
func adfFlattenToGFM(node *jira4claude.ADFNode, skipped *skippedCollector) string {
	var parts []string
	for i := range node.Content {
		part := adfNodeToGFM(&node.Content[i], skipped)
		if part != "" {
			parts = append(parts, part)
		}
	}
	return strings.Join(parts, "\n\n")
}

// adfDecisionListToGFM converts an ADF decisionList to a plain bullet list.
func adfDecisionListToGFM(node *jira4claude.ADFNode, skipped *skippedCollector) string {
	var items []string
	for i := range node.Content {
		child := &node.Content[i]
		if child.Type != "decisionItem" {
			continue
		}
		text := adfListItemToGFM(child, skipped, len("- "))
		if text != "" {
			items = append(items, "- "+text)
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

	var result strings.Builder
	for i := range items {
		child := &items[i]
		switch child.Type {
		case "text":
			lineStart := i == 0 || items[i-1].Type == "hardBreak"
			text := escapeTextNode(items, i, lineStart)
			if len(child.Marks) == 0 {
				result.WriteString(text)
			} else {
				result.WriteString(applyMarks(text, child.Marks))
			}
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
				// goldmark does not take "\\\" before a newline as an
				// escaped backslash and a hard break, so a plain text that
				// ends in a backslash writes it as a character reference.
				if prev := i - 1; prev >= 0 && items[prev].Type == "text" &&
					len(items[prev].Marks) == 0 && strings.HasSuffix(items[prev].Text, `\`) {
					out := result.String()
					if strings.HasSuffix(out, `\\`) {
						result.Reset()
						result.WriteString(out[:len(out)-2] + "&#92;")
					}
				}
				result.WriteString("\\\n")
			}
		case "mediaInline":
			result.WriteString(adfMediaInlineToGFM(child, nextStartsWith(items, i)))
		case "inlineCard":
			result.WriteString(adfInlineCardToGFM(child))
		}
	}

	return result.String()
}

// emittingInline returns the inline nodes that produce output, in order.
// Nodes whose output is a plain label (mention, emoji, date, status) become
// text nodes holding that label, so it is escaped like any other text; a
// status keeps its bold. Nodes that emit nothing are left out, and the ones
// that are not supported are recorded as dropped.
func emittingInline(content []jira4claude.ADFNode, skipped *skippedCollector) []jira4claude.ADFNode {
	items := make([]jira4claude.ADFNode, 0, len(content))
	// label appends text, merging it into the previous item when that is
	// text with the same marks: written separately, the two would get
	// delimiters between them ("**a\***" + "**b**") that do not read back.
	// Label nodes merge too: they are written as plain text with no syntax
	// of their own, so merging changes nothing but the delimiters.
	label := func(text string, marks []jira4claude.ADFMark) {
		if text == "" {
			return
		}
		if last := len(items) - 1; last >= 0 && items[last].Type == "text" && marksEqual(items[last].Marks, marks) {
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
		case "hardBreak", "mediaInline":
			items = append(items, *child)
		case "mention":
			label(adfMentionToGFM(child), nil)
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
			if skipped != nil {
				skipped.add(child.Type)
			}
		}
	}
	return items
}

// nextStartsWith returns the first byte the node after items[i] emits, or 0.
func nextStartsWith(items []jira4claude.ADFNode, i int) byte {
	if i+1 >= len(items) {
		return 0
	}
	switch next := items[i+1]; next.Type {
	case "text":
		if hasRenderedMark(next.Marks) {
			return '*' // any delimiter; only "(" and ":" matter to callers
		}
		return next.Text[0]
	case "inlineCard":
		return '['
	case "mediaInline":
		return '['
	}
	return 0
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
// Whitespace expulsion: leading/trailing whitespace is moved outside all
// mark delimiters to avoid producing broken syntax like "**bold **".
func applyMarks(text string, marks []jira4claude.ADFMark) string {
	var hasStrong, hasEm, hasCode, hasStrike, hasUnderline bool
	var linkHref string
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

	// Expel leading/trailing whitespace so delimiters hug content.
	// If text is whitespace-only, skip all marks to avoid empty delimiters.
	trimmed := strings.Trim(text, " \t")
	if trimmed == "" {
		return text
	}
	leading := text[:len(text)-len(strings.TrimLeft(text, " \t"))]
	trailing := text[len(strings.TrimRight(text, " \t")):]
	result := trimmed

	// Apply marks innermost → outermost.
	// Code is exclusive: suppresses em/strong/strike.
	if hasCode {
		result = "`" + result + "`"
	} else {
		if hasStrike {
			result = "~~" + result + "~~"
		}
		if hasEm {
			result = "*" + result + "*"
		}
		if hasStrong {
			result = "**" + result + "**"
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
	if linkHref != "" && result != linkHref {
		result = "[" + result + "](" + linkHref + ")"
	}

	return leading + result + trailing
}

// adfMentionToGFM converts an ADF mention node to @DisplayName.
func adfMentionToGFM(node *jira4claude.ADFNode) string {
	if node.Attrs == nil {
		return ""
	}
	var attrs map[string]any
	if err := json.Unmarshal(node.Attrs, &attrs); err != nil {
		return ""
	}
	if text, ok := attrs["text"].(string); ok && text != "" {
		// Some Jira payloads include the @ prefix in text, some don't.
		return "@" + strings.TrimPrefix(text, "@")
	}
	if id, ok := attrs["id"].(string); ok && id != "" {
		return "@" + id
	}
	return ""
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
// side decodes, so it is escaped as link text.
func adfInlineCardToGFM(node *jira4claude.ADFNode) string {
	url := inlineCardURL(node)
	return "[" + escapeText(url, textContext{inLink: true}) + "](" + url + ")"
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

// mediaAttrs extracts common attributes from a media node's attrs JSON.
func mediaAttrs(node *jira4claude.ADFNode) (url, alt, fileName string) {
	if node.Attrs == nil {
		return "", "", ""
	}
	var attrs map[string]any
	if err := json.Unmarshal(node.Attrs, &attrs); err != nil {
		return "", "", ""
	}
	if u, ok := attrs["url"].(string); ok {
		url = u
	}
	if a, ok := attrs["alt"].(string); ok {
		alt = a
	}
	if f, ok := attrs["__fileName"].(string); ok {
		fileName = f
	}
	return url, alt, fileName
}

// adfMediaToGFM renders a single media node as markdown.
func adfMediaToGFM(node *jira4claude.ADFNode) string {
	url, alt, fileName := mediaAttrs(node)
	if url != "" {
		if alt == "" {
			alt = fileName
		}
		return fmt.Sprintf("![%s](%s)", escapeBracketed(alt), url)
	}
	if fileName != "" {
		return fmt.Sprintf("[image: %s]", escapeBracketed(fileName))
	}
	return "[image]"
}

// adfMediaSingleToGFM converts an ADF mediaSingle node to markdown.
func adfMediaSingleToGFM(node *jira4claude.ADFNode) string {
	for i := range node.Content {
		if node.Content[i].Type == "media" {
			return adfMediaToGFM(&node.Content[i])
		}
	}
	return "[image]"
}

// adfMediaGroupToGFM converts an ADF mediaGroup (multiple media nodes) to markdown.
func adfMediaGroupToGFM(node *jira4claude.ADFNode) string {
	var parts []string
	for i := range node.Content {
		if node.Content[i].Type == "media" {
			parts = append(parts, adfMediaToGFM(&node.Content[i]))
		}
	}
	return strings.Join(parts, "\n")
}

// escapeBracketed escapes a label written inside brackets the read side
// supplies, so it cannot close them early or turn into formatting.
func escapeBracketed(label string) string {
	return escapeText(label, textContext{inLink: true})
}

// adfMediaInlineToGFM converts an ADF mediaInline node to inline markdown
// text. next is the first character the following node emits: a "(" or ":"
// right after the closing bracket would turn the bracketed name into a link
// or a link reference definition, so the opening bracket is escaped then.
func adfMediaInlineToGFM(node *jira4claude.ADFNode, next byte) string {
	open := "["
	if next == '(' || next == ':' {
		open = `\[`
	}
	_, _, fileName := mediaAttrs(node)
	if fileName != "" {
		return open + escapeBracketed(fileName) + "]"
	}
	return open + "attachment]"
}
