package markdown

import (
	"strings"
	"unicode/utf8"

	"github.com/fwojciec/jira4claude"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// Marked text on read (#159).
//
// Strong, em and strike are written as delimiter runs ("**", "*" or "_",
// "~~"), which goldmark only takes as opening or closing when the characters
// on either side of the run fit CommonMark's flanking rules. Link, underline
// and subsup are written as brackets and tags, which have no such rules. So
// the rules below are about delimiter runs only; they keep every text node's
// text and marks exact.
//
//   - Edge blanks stay inside the mark. A delimiter run cannot open before a
//     blank or close after one, so a blank (any Unicode space) at the edge of
//     delimited text is written as a character reference ("**a&#32;**").
//     Tags and links keep the blank as is ("<u>a </u>").
//   - A tilde at the edge of struck text is a character reference
//     ("~~a&#126;~~"): a "~" next to the "~~" would make a run of three, which
//     neither opens nor closes (see strikethrough.go). Plain text next to a
//     "~~" delimiter writes its adjacent tilde the same way.
//   - Flanking: when the character just inside a delimiter run is punctuation
//     (including the "&" or ";" of a reference, or another delimiter), the
//     character just outside must be whitespace or punctuation too. A letter
//     or digit there, from neighbouring plain text, is written as a character
//     reference ("**a.**&#97;"). "_" runs always need this, since "_" cannot
//     open or close inside a word.
//   - Adjacent delimited text: strike is innermost and em and strong are
//     outside it, so two neighbouring nodes' delimiters can only meet as "*"
//     against "*" ("~~" against "~~" would need two nodes with the same
//     written marks, which emittingInline merges). When a node would open
//     right after a "*" run, it uses "_" instead ("**a**_b_"), so the two
//     runs stay apart.
//   - A link written as its bare URL cannot have its edge encoded; when a
//     delimiter run needs punctuation there, the link is written in brackets
//     ("[https://x.test](https://x.test)**.a**").
//
// Plain text ending in a URL cannot have its last character encoded either:
// Linkify would end the link at the reference. There the text is reported as
// degraded. As a backstop, a line of text and hard breaks that holds
// delimited text is parsed back; if the text reads back the same but its
// marks do not, the text is reported as degraded.

// textEdges says which edge characters of a text node are written as
// character references.
type textEdges struct {
	first, last bool
}

// markedTextDetail explains the degraded issue for marked text that has no
// exact Markdown form where it stands.
const markedTextDetail = "bold, italic or strikethrough text right after a URL, " +
	"or in a shape with no exact Markdown form, may not read back with the same marks"

// usesDelimiters reports whether applyMarks writes marks as delimiter runs:
// strong, em or strike, without code (code is exclusive, see applyMarks).
func usesDelimiters(marks []jira4claude.ADFMark) bool {
	if hasCodeMark(marks) {
		return false
	}
	for _, m := range marks {
		switch m.Type {
		case "strong", "em", "strike":
			return true
		}
	}
	return false
}

func hasMark(marks []jira4claude.ADFMark, markType string) bool {
	for _, m := range marks {
		if m.Type == markType {
			return true
		}
	}
	return false
}

// isPlainText reports whether items[i] is text written with no syntax of its
// own, whose edge characters can therefore be encoded.
func isPlainText(items []jira4claude.ADFNode, i int) bool {
	return items[i].Type == "text" && !hasRenderedMark(items[i].Marks)
}

// isLineStart reports whether items[i] begins a line of Markdown.
func isLineStart(items []jira4claude.ADFNode, i int) bool {
	return i == 0 || items[i-1].Type == "hardBreak"
}

// innerEdges returns the edges of delimited text that are encoded whatever
// its neighbours: blanks, and tildes when struck.
func innerEdges(node *jira4claude.ADFNode) textEdges {
	first, _ := utf8.DecodeRuneInString(node.Text)
	last, _ := utf8.DecodeLastRuneInString(node.Text)
	struck := hasMark(node.Marks, "strike")
	return textEdges{
		first: util.IsSpaceRune(first) || struck && first == '~',
		last:  util.IsSpaceRune(last) || struck && last == '~',
	}
}

// delimiterNeeds reports what the delimiter run at one end of md (a node's
// Markdown) needs from the character outside it: punct, a whitespace or
// punctuation character; noTilde, anything but "~". atEnd picks the closing
// run. md that does not begin (end) with a delimiter run needs nothing.
func delimiterNeeds(md string, atEnd bool) (punct, noTilde bool) {
	if md == "" {
		return false, false
	}
	var c byte
	if atEnd {
		c = md[len(md)-1]
	} else {
		c = md[0]
	}
	if c != '*' && c != '_' && c != '~' {
		return false, false
	}
	var inside rune
	if atEnd {
		rest := strings.TrimRight(md, string(c))
		inside, _ = utf8.DecodeLastRuneInString(rest)
	} else {
		rest := strings.TrimLeft(md, string(c))
		inside, _ = utf8.DecodeRuneInString(rest)
	}
	return c == '_' || util.IsPunctRune(inside), c == '~'
}

// renderTextItems returns the Markdown of each text item in items, the
// emitting inline nodes of one line (see emittingInline), applying the rules
// at the top of this file. Entries for other nodes are "".
func renderTextItems(items []jira4claude.ADFNode, skipped *skippedCollector) []string {
	out := make([]string, len(items))
	edges := make([]textEdges, len(items))

	// Marked text first: its Markdown depends only on its own text and the
	// delimiter chosen after its left neighbour.
	for i := range items {
		node := &items[i]
		if node.Type != "text" || isPlainText(items, i) {
			continue
		}
		var delim byte
		if usesDelimiters(node.Marks) {
			edges[i] = innerEdges(node)
			delim = '*'
			if i > 0 && items[i-1].Type == "text" && strings.HasSuffix(out[i-1], "*") {
				delim = '_'
			}
		}
		out[i] = applyMarks(escapeTextNode(items, i, isLineStart(items, i), edges[i]), node.Marks, delim)
	}

	// Then what each delimiter run needs from its neighbours.
	conflict := false
	for i := range items {
		if items[i].Type != "text" || isPlainText(items, i) {
			continue
		}
		if punct, noTilde := delimiterNeeds(out[i], false); punct || noTilde {
			conflict = !fitDelimiter(items, out, edges, i-1, true, punct, noTilde) || conflict
		}
		if punct, noTilde := delimiterNeeds(out[i], true); punct || noTilde {
			conflict = !fitDelimiter(items, out, edges, i+1, false, punct, noTilde) || conflict
		}
	}
	if conflict && skipped != nil {
		skipped.degrade("text", markedTextDetail)
	}

	// Then plain text, with the edges its neighbours asked for.
	for i := range items {
		if !isPlainText(items, i) {
			continue
		}
		out[i] = escapeTextNode(items, i, isLineStart(items, i), edges[i])
		if len(items[i].Marks) > 0 {
			out[i] = applyMarks(out[i], items[i].Marks, 0)
		}
	}
	return out
}

// fitDelimiter makes the edge of items[j] that meets a delimiter run fit it
// (atEnd: the end of items[j] meets the run's start). Plain text gets its
// edge character encoded; any other node must already fit. It reports
// whether the edge fits.
func fitDelimiter(items []jira4claude.ADFNode, out []string, edges []textEdges, j int, atEnd, punct, noTilde bool) bool {
	if j < 0 || j >= len(items) || items[j].Type == "hardBreak" {
		return true // a line start, line end or hard break is whitespace or "\"
	}
	fits := func(r rune) bool {
		return (!punct || util.IsSpaceRune(r) || util.IsPunctRune(r)) && (!noTilde || r != '~')
	}
	if isPlainText(items, j) {
		var r rune
		if atEnd {
			r, _ = utf8.DecodeLastRuneInString(items[j].Text)
		} else {
			r, _ = utf8.DecodeRuneInString(items[j].Text)
		}
		if fits(r) {
			return true
		}
		if atEnd {
			// Linkify would end a URL at the reference, linking only part
			// of it, so the last character of a URL is left as is.
			if endsInAutolink(items[j].Text) {
				return false
			}
			edges[j].last = true
		} else {
			edges[j].first = true
		}
		return true
	}
	if items[j].Type != "text" {
		return true // inline nodes are written in brackets
	}
	if href, ok := bareURL(&items[j]); ok {
		// A link written as its bare URL is written in brackets instead.
		out[j] = "[" + escapeText(href, textContext{inLink: true}) + "](" + href + ")"
		return true
	}
	var r rune
	if atEnd {
		r, _ = utf8.DecodeLastRuneInString(out[j])
	} else {
		r, _ = utf8.DecodeRuneInString(out[j])
	}
	return fits(r)
}

// bareURL reports whether node is a link that applyMarks writes as its bare
// URL (its text is its href and it has no other mark that is written),
// returning the href.
func bareURL(node *jira4claude.ADFNode) (string, bool) {
	href := ""
	for _, m := range node.Marks {
		switch {
		case m.Type == "link":
			href = linkHref(m)
		case hasRenderedMark([]jira4claude.ADFMark{m}):
			return "", false
		}
	}
	return href, href != "" && href == node.Text
}

// endsInAutolink reports whether s ends in a span that goldmark's Linkify
// would read as an autolink.
func endsInAutolink(s string) bool {
	word := s[strings.LastIndexAny(s, " \t\n(")+1:]
	n := autolinkLen(word)
	return n > 0 && n == len(word)
}

// checkMarksReadBack is the backstop described at the top of this file. md
// is the Markdown of items, one line of inline content. If md, parsed as the
// write side parses it, gives the same text as items but different strong,
// em or strike marks, the text is reported as degraded.
func checkMarksReadBack(items []jira4claude.ADFNode, md string, singleLine bool, skipped *skippedCollector) {
	if skipped == nil {
		return
	}
	delimited := false
	for i := range items {
		switch items[i].Type {
		case "text":
			if strings.ContainsAny(items[i].Text, "\r\n") {
				return // a line ending in text is a soft break on write
			}
			delimited = delimited || usesDelimiters(items[i].Marks)
		case "hardBreak":
			if singleLine {
				return
			}
		default:
			return
		}
	}
	if !delimited {
		return
	}
	for len(items) > 0 && items[len(items)-1].Type == "hardBreak" {
		items = items[:len(items)-1] // left out on read
	}

	source := []byte(md)
	doc := newMarkdownParser().Parse(text.NewReader(source))
	para := doc.FirstChild()
	if para == nil || para.NextSibling() != nil || para.Kind() != ast.KindParagraph {
		return
	}
	got := markSpans(convertInlineContent(para, source, newSkippedCollector()))
	want := markSpans(items)
	if spansText(got) == spansText(want) && !spansEqual(got, want) {
		skipped.degrade("text", markedTextDetail)
	}
}

// markSpan is a text node, or a hard break, reduced to what the backstop
// compares: its text and its delimiter marks (strong, em, strike), in
// canonical order. Other marks have rules of their own, outside #159.
type markSpan struct {
	text  string
	marks []jira4claude.ADFMark
}

func markSpans(nodes []jira4claude.ADFNode) []markSpan {
	var spans []markSpan
	for _, n := range nodes {
		s := markSpan{text: "\n"}
		if n.Type == "text" {
			s.text = n.Text
			for _, m := range n.Marks {
				switch m.Type {
				case "strong", "em", "strike":
					s.marks = append(s.marks, m)
				}
			}
			s.marks, _ = canonicalMarks(s.marks)
		}
		if last := len(spans) - 1; last >= 0 && n.Type == "text" && spans[last].text != "\n" &&
			marksEqual(spans[last].marks, s.marks) {
			spans[last].text += s.text
			continue
		}
		spans = append(spans, s)
	}
	return spans
}

func spansText(spans []markSpan) string {
	var b strings.Builder
	for _, s := range spans {
		b.WriteString(s.text)
	}
	return b.String()
}

func spansEqual(a, b []markSpan) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].text != b[i].text || !marksEqual(a[i].marks, b[i].marks) {
			return false
		}
	}
	return true
}
