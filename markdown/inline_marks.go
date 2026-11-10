package markdown

import (
	"encoding/json"
	"slices"
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
// As a backstop, a line of text and hard breaks that holds delimited text is
// parsed back; if the text reads back the same but its marks do not, the
// text is reported as degraded.

// Links on read (#161).
//
// A link is written one of two ways:
//
//   - Bare, as its URL verbatim, which goldmark's Linkify reads back: only
//     when its text is its href, it has no other mark Markdown writes, and
//     the line as written, parsed as the write side parses it, reads back
//     with that link exactly (same text, same href, starting and ending
//     where the node does) and with the line's text unchanged. Linkify must
//     start there (a line head, a blank, an unescaped "*", "_", "~" or "(",
//     or right after another inline construct), must end there (not taking
//     in a letter, a backtick or a reference after it, nor dropping a
//     trailing "*", "_", "~", "." or ")"), and must give the same href
//     (a "www." URL gains "http://"; "0@0" or "A0:" is no autolink at all).
//     renderLinks checks this by parsing the line, and brackets the links
//     that fail; the analytic repairs in renderTextItems (autolink crossings,
//     delimiter flanking) bracket most of them before that.
//   - Otherwise in brackets: "[text](destination)", the text escaped as link
//     text and the destination written by linkDestination, which every
//     writable href has. An href holding a character Markdown cannot carry
//     (a control character, a carriage return) is written as the write side
//     would read it and reported as degraded.
//
// Text that is not a link is kept from forming one (break mode, escape.go),
// and plain text ending in "!" before a bracketed link escapes the "!", which
// would make the link an image. Two neighbouring links with the same written
// marks are merged into one by emittingInline, as the write side merges
// them (consolidateTextNodes).

// textEdges says which edge characters of a text node are written as
// character references.
type textEdges struct {
	first, last bool
	// breakLinks: for text that is not a link, keep Linkify from starting
	// in it (see breakAutolinks); for a link whose text is its href, write
	// it in brackets rather than bare.
	breakLinks bool
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

// bareLinkRounds bounds how often renderLinks brackets the bare links that
// do not read back and tries again; after that every bare link on the line
// is bracketed.
const bareLinkRounds = 3

// renderLinks returns the Markdown of each text item in items, the emitting
// inline nodes of one line, as renderTextItems does, with every link written
// bare only where it reads back (the rule at the top of this file). Issues
// are reported for the output returned.
func renderLinks(items []jira4claude.ADFNode, singleLine bool, skipped *skippedCollector) []string {
	bracket := make([]bool, len(items))
	for round := 0; ; round++ {
		texts, issues := renderTextItems(items, bracket)
		misread := misreadBareLinks(items, texts, joinInline(items, texts, singleLine, nil), singleLine)
		if len(misread) == 0 {
			issues.report(skipped)
			return texts
		}
		if round == bareLinkRounds {
			misread = bareLinks(items, texts)
		}
		for _, i := range misread {
			bracket[i] = true
		}
		if round == bareLinkRounds {
			texts, issues = renderTextItems(items, bracket)
			issues.report(skipped)
			return texts
		}
	}
}

// bareLinks returns the items written as their bare URL.
func bareLinks(items []jira4claude.ADFNode, out []string) []int {
	var bare []int
	for i := range items {
		if _, ok := bareURL(&items[i]); ok && out[i] == items[i].Text {
			bare = append(bare, i)
		}
	}
	return bare
}

// misreadBareLinks parses md, the Markdown of items with their text items
// written as out, as the write side does, and returns the items written as
// their bare URL that do not read back as themselves: where the links read
// back (each character's href, or none) differ from items within a bare
// link or next to it. If the text itself reads back differently, or a
// difference lies away from every bare link, all of them are returned.
func misreadBareLinks(items []jira4claude.ADFNode, out []string, md string, singleLine bool) []int {
	bare := bareLinks(items, out)
	if len(bare) == 0 {
		return nil
	}
	want, starts := linkedText(items, singleLine)
	source := []byte(md)
	doc := newMarkdownParser().Parse(text.NewReader(source))
	para := doc.FirstChild()
	if para == nil || para.NextSibling() != nil || para.Kind() != ast.KindParagraph {
		return bare
	}
	got, _ := linkedText(convertInlineContent(para, source, newSkippedCollector()), false)
	if got.text != want.text {
		return bare
	}
	end := func(i int) int { return starts[i] + len(items[i].Text) }
	var misread []int
	k := 0 // bare[k:] are the bare links that do not end before p
	for p := range want.hrefs {
		if want.hrefs[p] == got.hrefs[p] {
			continue
		}
		// The bare links whose text, or the character either side of it,
		// holds the difference.
		for k < len(bare) && end(bare[k]) < p {
			k++
		}
		found := false
		for j := k; j < len(bare) && starts[bare[j]] <= p+1; j++ {
			misread = append(misread, bare[j])
			found = true
		}
		if !found {
			return bare
		}
	}
	return slices.Compact(misread)
}

// linkText is inline content reduced to what misreadBareLinks compares: its
// text, and per byte of it, the href of the link it is in ("\x00" for none).
type linkText struct {
	text  string
	hrefs []string
}

// linkedText returns nodes as linkText: text as it reads, a hard break as a
// line feed (a space when singleLine, nothing at the end), a protected-node
// token as the text it parses as, an inline card as a link to its URL. It
// also returns where each node's text starts.
func linkedText(nodes []jira4claude.ADFNode, singleLine bool) (linkText, []int) {
	var b strings.Builder
	var hrefs []string
	starts := make([]int, len(nodes))
	add := func(s, href string) {
		b.WriteString(s)
		for range len(s) {
			hrefs = append(hrefs, href)
		}
	}
	for i := range nodes {
		n := &nodes[i]
		starts[i] = b.Len()
		switch n.Type {
		case "text":
			href := "\x00"
			for _, m := range n.Marks {
				if m.Type == "link" {
					href = linkHref(m)
				}
			}
			add(n.Text, href)
		case "hardBreak":
			switch {
			case onlyHardBreaksFrom(nodes, i):
			case singleLine:
				add(" ", "\x00")
			default:
				add("\n", "\x00")
			}
		case tokenItemType:
			add(decodeText([]byte(n.Text)), "\x00")
		case "inlineCard":
			url, _ := writableText(inlineCardURL(n))
			add(url, url)
		}
	}
	return linkText{text: b.String(), hrefs: hrefs}, starts
}

// textIssues are the issues renderTextItems found, reported by report once
// its output is the one used.
type textIssues struct {
	conflict bool
	// unclosable: an autolink runs across a node edge and no repair
	// removes it; it is reported with its own detail.
	unclosable bool
}

func (t textIssues) report(skipped *skippedCollector) {
	if skipped == nil {
		return
	}
	if t.conflict {
		skipped.degrade("text", markedTextDetail)
	}
	if t.unclosable {
		skipped.degrade("text", unclosableURLDetail)
	}
}

// renderTextItems returns the Markdown of each text item in items, the
// emitting inline nodes of one line (see emittingInline), applying the rules
// at the top of this file. Entries for other nodes are "". bracket marks
// links whose text is their href that must be written in brackets, not bare.
func renderTextItems(items []jira4claude.ADFNode, bracket []bool) ([]string, textIssues) {
	out := make([]string, len(items))
	edges := make([]textEdges, len(items))
	delims := make([]byte, len(items))
	var issues textIssues
	stuck := make([]bool, len(items))
	render := func(i int) {
		if href, ok := bareURL(&items[i]); ok {
			if edges[i].breakLinks {
				out[i] = bracketedURL(href)
			} else {
				out[i] = items[i].Text // read back by Linkify
			}
			return
		}
		out[i] = escapeTextNode(items, i, isLineStart(items, i), edges[i])
		if len(items[i].Marks) > 0 {
			out[i] = applyMarks(out[i], items[i].Marks, delims[i])
		}
	}

	// Text that is not a link never forms one: Linkify is kept from starting
	// anywhere in it.
	for i := range items {
		edges[i].breakLinks = items[i].Type == "text" && !hasMark(items[i].Marks, "link") || bracket[i]
	}

	// Marked text first: its Markdown depends only on its own text and the
	// delimiter chosen after its left neighbour.
	for i := range items {
		node := &items[i]
		if node.Type != "text" || isPlainText(items, i) {
			continue
		}
		if usesDelimiters(node.Marks) {
			inner := innerEdges(node)
			edges[i].first, edges[i].last = inner.first, inner.last
		}
		render(i)
		// The delimiter switches only where the two runs meet: not when a
		// link's brackets or a tag stand between them.
		if usesDelimiters(node.Marks) && strings.HasPrefix(out[i], "*") &&
			i > 0 && items[i-1].Type == "text" && strings.HasSuffix(out[i-1], "*") {
			delims[i] = '_'
			render(i)
		}
	}

	// Then, until nothing changes: what each delimiter run needs from its
	// neighbours, plain text with the edges asked for, and autolinks. No
	// autolink may run across the edge of a node's text: into its closing
	// delimiters or the next node, or back from an earlier node into this
	// one (an email's local part takes "*" and "~"). A crossing is repaired
	// by keeping autolinks from forming in the node before it, then in the
	// node after it, then by encoding the last character of the text
	// before it. Each repair can change what a delimiter run sees, so the
	// passes repeat; a crossing no repair removes is degraded.
	for range 4*len(items) + 1 {
		before := slices.Clone(edges)
		for i := range items {
			if items[i].Type != "text" || isPlainText(items, i) {
				continue
			}
			if punct, noTilde := delimiterNeeds(out[i], false); punct || noTilde {
				issues.conflict = !fitDelimiter(items, out, edges, i-1, true, punct, noTilde) || issues.conflict
			}
			if punct, noTilde := delimiterNeeds(out[i], true); punct || noTilde {
				issues.conflict = !fitDelimiter(items, out, edges, i+1, false, punct, noTilde) || issues.conflict
			}
		}
		for i := range items {
			if isPlainText(items, i) {
				render(i)
			}
		}

		// A crossing no repair removes is reported once and left.
		crossings := slices.DeleteFunc(autolinkCrossings(items, out), func(i int) bool { return stuck[i] })
		if len(crossings) == 0 && slices.Equal(before, edges) {
			return out, issues
		}
		// Every crossing found is repaired in this pass, so a line with many
		// takes a few passes, not one per crossing.
		for _, i := range crossings {
			switch {
			case !edges[i].breakLinks:
				edges[i].breakLinks = true
				render(i)
			case i+1 < len(items) && items[i+1].Type == "text" && !edges[i+1].breakLinks:
				edges[i+1].breakLinks = true
				render(i + 1)
			case !edges[i].last && !hasCodeMark(items[i].Marks):
				edges[i].last = true
				render(i)
			default:
				stuck[i] = true
				issues.unclosable = true
			}
		}
	}
	// The passes did not settle: an autolink still crosses a node edge, or
	// delimiter fixes kept changing each other.
	issues.conflict = issues.conflict || !issues.unclosable
	return out, issues
}

// autolinkCrossings returns, in order, the text nodes, items[i], where
// Linkify would read an autolink across the end of its text: one that starts
// before its closing delimiters (or before its end, when it has none) and
// ends after them, within the run of text nodes it belongs to.
func autolinkCrossings(items []jira4claude.ADFNode, out []string) []int {
	var crossings []int
	for start := 0; start < len(items); start++ {
		if items[start].Type != "text" {
			continue
		}
		var md strings.Builder
		var ends []int     // per node of the run: the end of its text in md
		var noStart []bool // per byte of md: inside a code span or link text
		stop := start
		for ; stop < len(items) && items[stop].Type == "text"; stop++ {
			end := md.Len() + len(out[stop])
			if usesDelimiters(items[stop].Marks) {
				end = md.Len() + len(strings.TrimRight(out[stop], "*_~"))
			}
			ends = append(ends, end)
			closed := hasCodeMark(items[stop].Marks) || hasMark(items[stop].Marks, "link") && strings.HasPrefix(out[stop], "[")
			for range len(out[stop]) {
				noStart = append(noStart, closed)
			}
			md.WriteString(out[stop])
		}
		// Linkify does not start inside a code span or link text, but a link
		// that starts before one can run on into it. The spans are found
		// once per run; the node ends rise, so one pass over both finds,
		// for each end, the furthest any span starting before it reaches.
		canStart := func(p int) bool { return !noStart[p] }
		spans := linkifySpans(md.String(), md.String(), isLineStart(items, start), canStart)
		next, reach := 0, 0
		for k, end := range ends {
			for ; next < len(spans) && spans[next][0] < end; next++ {
				reach = max(reach, spans[next][1])
			}
			if !hasCodeMark(items[start+k].Marks) && reach > end {
				crossings = append(crossings, start+k)
			}
		}
		start = stop
	}
	return crossings
}

// bracketedURL writes a link whose text is its href in brackets rather than
// as a bare URL.
func bracketedURL(href string) string {
	return "[" + escapeText(href, textContext{inLink: true}) + "](" + linkDestination(href) + ")"
}

// linkDestination writes href as a CommonMark link destination that the
// write side, which resolves backslash escapes and character references in
// a destination as in text, reads back as href. href must be writable (see
// writableText); writableHref makes it so.
//
// The destination is href as it stands, with "\", "<" and ">" escaped, an
// "&" that would start a character reference escaped, and a line feed or tab
// written as a character reference. It is put in angle brackets when it is
// empty, holds a blank, or has parentheses that are not balanced, which a
// bare destination cannot hold ("[a](<b c>)", "[a](<x(>)"). An ordinary URL,
// with balanced parentheses or none, is written as it is.
func linkDestination(href string) string {
	var b strings.Builder
	angle := href == ""
	depth := 0
	for i := 0; i < len(href); i++ {
		switch c := href[i]; c {
		case ' ':
			angle = true
			b.WriteByte(c)
		case '\n', '\t':
			b.WriteString(charRef(rune(c)))
		case '\\', '<', '>':
			b.WriteByte('\\')
			b.WriteByte(c)
		case '&':
			if _, _, ok := entityAt([]byte(href), i); ok {
				b.WriteByte('\\')
			}
			b.WriteByte(c)
		case '(', ')':
			if c == '(' {
				depth++
			} else {
				depth--
			}
			angle = angle || depth < 0
			b.WriteByte(c)
		default:
			b.WriteByte(c)
		}
	}
	if angle || depth != 0 {
		return "<" + b.String() + ">"
	}
	return b.String()
}

// hrefDetail explains the degraded issue reported when a link's href holds a
// character the write side would change.
const hrefDetail = "a carriage return, control character or invalid UTF-8 in a link's address cannot be written " +
	"in Markdown; it is written as a line feed, a space or U+FFFD, as the write side would read it"

// writableHrefs returns marks with each link's href made writable (see
// writableText), and whether any changed. marks itself is not modified.
func writableHrefs(marks []jira4claude.ADFMark) ([]jira4claude.ADFMark, bool) {
	changed := false
	for k, m := range marks {
		if m.Type != "link" {
			continue
		}
		href, hrefChanged := writableText(linkHref(m))
		if !hrefChanged {
			continue
		}
		if !changed {
			marks = slices.Clone(marks)
			changed = true
		}
		var attrs map[string]any
		_ = json.Unmarshal(m.Attrs, &attrs)
		if attrs == nil {
			attrs = map[string]any{}
		}
		attrs["href"] = href
		marks[k].Attrs, _ = json.Marshal(attrs)
	}
	return marks, changed
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
		edges[j].breakLinks = true
		out[j] = bracketedURL(href)
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
