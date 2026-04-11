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
// Returns warnings for any elements that were skipped during conversion.
func toMarkdown(adfDoc *jira4claude.ADFNode) (string, []string) {
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

	return strings.Join(parts, "\n\n"), skipped.warnings()
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

	text := adfInlineToGFM(node, skipped)
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
		itemText := adfListItemToGFM(child, skipped)
		items = append(items, "- "+itemText)
	}

	return strings.Join(items, "\n")
}

// adfOrderedListToGFM converts an ADF orderedList to markdown.
func adfOrderedListToGFM(node *jira4claude.ADFNode, skipped *skippedCollector) string {
	items := make([]string, 0, len(node.Content))
	num := 0
	for i := range node.Content {
		child := &node.Content[i]
		if child.Type != "listItem" {
			continue
		}
		num++
		itemText := adfListItemToGFM(child, skipped)
		items = append(items, fmt.Sprintf("%d. %s", num, itemText))
	}

	return strings.Join(items, "\n")
}

// adfListItemToGFM extracts the text content from a list item.
func adfListItemToGFM(node *jira4claude.ADFNode, skipped *skippedCollector) string {
	if len(node.Content) == 0 {
		return ""
	}

	// List items typically contain paragraphs or nested lists
	parts := make([]string, 0, len(node.Content))
	for i := range node.Content {
		child := &node.Content[i]
		part := adfNodeToGFM(child, skipped)
		if part != "" {
			parts = append(parts, part)
		}
	}

	return strings.Join(parts, " ")
}

// adfBlockquoteToGFM converts an ADF blockquote to markdown.
func adfBlockquoteToGFM(node *jira4claude.ADFNode, skipped *skippedCollector) string {
	var lines []string
	for i := range node.Content {
		child := &node.Content[i]
		text := adfNodeToGFM(child, skipped)
		// Prefix each line with >
		for _, line := range strings.Split(text, "\n") {
			lines = append(lines, "> "+line)
		}
	}

	return strings.Join(lines, "\n")
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

	alert := panelTypeToAlert(panelType)
	var lines []string
	lines = append(lines, "> [!"+alert+"]")

	for i := range node.Content {
		text := adfNodeToGFM(&node.Content[i], skipped)
		if i > 0 {
			// Blank blockquote line to separate paragraphs
			lines = append(lines, ">")
		}
		for _, line := range strings.Split(text, "\n") {
			lines = append(lines, "> "+line)
		}
	}

	return strings.Join(lines, "\n")
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
		part := adfNodeToGFM(child, skipped)
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
func adfTaskListToGFM(node *jira4claude.ADFNode, skipped *skippedCollector) string {
	var items []string
	for i := range node.Content {
		taskItem := &node.Content[i]
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

		text := adfListItemToGFM(taskItem, skipped)
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
		text := adfListItemToGFM(child, skipped)
		if text != "" {
			items = append(items, "- "+text)
		}
	}
	return strings.Join(items, "\n")
}

// adfInlineToGFM converts inline content to markdown.
func adfInlineToGFM(node *jira4claude.ADFNode, skipped *skippedCollector) string {
	if len(node.Content) == 0 {
		return ""
	}

	var result strings.Builder
	for i := range node.Content {
		child := &node.Content[i]
		switch child.Type {
		case "text":
			if len(child.Marks) == 0 {
				result.WriteString(child.Text)
			} else {
				result.WriteString(applyMarks(child.Text, child.Marks))
			}
		case "hardBreak":
			result.WriteString("\n")
		case "mediaInline":
			result.WriteString(adfMediaInlineToGFM(child))
		case "mention":
			result.WriteString(adfMentionToGFM(child))
		case "emoji":
			result.WriteString(adfEmojiToGFM(child))
		case "inlineCard":
			if text := adfInlineCardToGFM(child); text != "" {
				result.WriteString(text)
			} else if skipped != nil {
				skipped.add("inlineCard")
			}
		case "status":
			result.WriteString(adfStatusToGFM(child))
		case "date":
			result.WriteString(adfDateToGFM(child))
		case "placeholder":
			// Drop silently — placeholder nodes produce no output.
		case "inlineExtension":
			if skipped != nil {
				skipped.add("inlineExtension")
			}
		default:
			if skipped != nil {
				skipped.add(child.Type)
			}
		}
	}

	return result.String()
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

// adfInlineCardToGFM converts an ADF inlineCard node to a markdown link.
func adfInlineCardToGFM(node *jira4claude.ADFNode) string {
	if node.Attrs == nil {
		return ""
	}
	var attrs map[string]any
	if err := json.Unmarshal(node.Attrs, &attrs); err != nil {
		return ""
	}
	if url, ok := attrs["url"].(string); ok && url != "" {
		return fmt.Sprintf("[%s](%s)", url, url)
	}
	return ""
}

// adfStatusToGFM converts an ADF status node to bold text.
func adfStatusToGFM(node *jira4claude.ADFNode) string {
	if node.Attrs == nil {
		return ""
	}
	var attrs map[string]any
	if err := json.Unmarshal(node.Attrs, &attrs); err != nil {
		return ""
	}
	if text, ok := attrs["text"].(string); ok && text != "" {
		return "**" + text + "**"
	}
	return ""
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
		return fmt.Sprintf("![%s](%s)", alt, url)
	}
	if fileName != "" {
		return fmt.Sprintf("[image: %s]", fileName)
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

// adfMediaInlineToGFM converts an ADF mediaInline node to inline markdown text.
func adfMediaInlineToGFM(node *jira4claude.ADFNode) string {
	_, _, fileName := mediaAttrs(node)
	if fileName != "" {
		return fmt.Sprintf("[%s]", fileName)
	}
	return "[attachment]"
}
