package markdown

import (
	"encoding/json"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/fwojciec/jira4claude"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// This file holds the two halves of plain-text escaping. They must stay
// symmetric: any text Jira holds must survive escapeText followed by a
// goldmark parse and decodeText unchanged.
//
// Write side (decodeText): CommonMark backslash escapes and entity references
// in text are decoded, in one left-to-right pass, the same way goldmark's own
// HTML renderer resolves them (renderer/html defaultWriter.Write). Code spans
// and code blocks are never decoded.
//
// Read side (escapeText): escape minimally, only a character that would
// otherwise be parsed as formatting or be consumed by decoding:
//
//   - "\" before ASCII punctuation, a newline, or the end of the text node
//     (where the next node's Markdown may begin with punctuation).
//   - "`" always, and "[" always (link, image, task box, alert, reference).
//     "]" only inside link text.
//   - "&" that starts something shaped like an entity reference
//     ("&name;", "&#123;", "&#x1F;"), or a prefix of one that runs to the end
//     of the text node ("&", "&am", "&#", "&#x1f"), since the next node's
//     text may complete it.
//   - "<" that could open HTML (an inline tag, closing tag, comment,
//     processing instruction, declaration or CDATA, or an HTML block) or an
//     autolink: followed by a letter, "/", "!" or "?", by an email address
//     and ">", or by the end of the text node. "a < b" and "x<5" stay bare.
//   - A run of "*" unless both neighbours are known whitespace; a run of "_"
//     unless both neighbours are known whitespace or both are letters or
//     digits (snake_case). The start and end of a text node, and the start
//     of a line, count as unknown, because the neighbour there may be a
//     delimiter emitted for another node.
//   - A run of exactly two "~" (only "~~" strikes, see strikethrough.go);
//     at the start of a line, any run of three or more (a code fence); and a
//     run at a text node's edge when the neighbouring text node's adjacent
//     edge is also "~", since the two meet in the Markdown. "~5" with no
//     tilde next to it stays bare.
//   - At the start of a line only: "#" that would open a heading, ">", a
//     "-" or "+" list marker, a line made of only "-" or "=" (setext
//     underline, thematic break), and the "." or ")" of an ordered list
//     marker.
//   - In a heading only: a "#" run that ends the text after a blank (an ATX
//     closing sequence).
//
// Everything else is written as is. A hardBreak between text nodes is
// written as a CommonMark hard break, "\" and a newline (see
// adfInlineLineToGFM), so the next line starts a line of the same paragraph.
//
// One exception: a span that goldmark's Linkify would turn into an autolink
// (a bare "http://", "https://", "ftp://" or "www." URL, or an email
// address) is written verbatim, because Linkify reads it raw and a backslash
// would end the link early. Linkify is tried at a line head, after a blank
// or "(", and right after another inline construct such as a formatting
// delimiter, so marked text is covered too. Because the span is measured on
// the raw text, not on what surrounds it in the output, the exception checks
// itself: the escaped node is parsed back, and if its text differs, the node
// is escaped again without the exception. A span that reaches the end of the
// node is not kept when plain text follows in the next node. The span is
// measured with Linkify's own parser,
// so trailing punctuation is included or trimmed exactly as on write.

// decodeText resolves backslash escapes and entity references in Markdown
// text. A backslash before anything other than ASCII punctuation is kept, and
// an escaped "&" is never read as the start of an entity.
func decodeText(src []byte) string {
	var b strings.Builder
	n := 0
	for i := 0; i < len(src); i++ {
		c := src[i]
		if c == '\\' && i+1 < len(src) && util.IsPunct(src[i+1]) {
			b.Write(src[n:i])
			b.WriteByte(src[i+1])
			i++
			n = i + 1
			continue
		}
		if c == '&' {
			if decoded, end, ok := entityAt(src, i); ok {
				b.Write(src[n:i])
				b.WriteString(decoded)
				n = end
				i = end - 1
			}
		}
	}
	b.Write(src[n:])
	return b.String()
}

// entityAt reports whether src[i:] starts with an entity or numeric
// character reference, returning its characters and the index just past the
// terminating ";". It follows goldmark's own resolution rules.
func entityAt(src []byte, i int) (string, int, bool) {
	limit := len(src)
	next := i + 1
	if next < limit && src[next] == '#' {
		start := next + 1
		base, maxDigits, pred := 10, 7, util.IsNumeric
		if start < limit && (src[start] == 'x' || src[start] == 'X') {
			start++
			base, maxDigits, pred = 16, 6, util.IsHexDecimal
		}
		end, ok := util.ReadWhile(src, [2]int{start, limit}, pred)
		if !ok || end >= limit || src[end] != ';' || end-start > maxDigits {
			return "", 0, false
		}
		v, _ := strconv.ParseUint(string(src[start:end]), base, 32)
		return string(util.ToValidRune(rune(v))), end + 1, true
	}
	end, ok := util.ReadWhile(src, [2]int{next, limit}, util.IsAlphaNumeric)
	if !ok || end >= limit || src[end] != ';' {
		return "", 0, false
	}
	entity, ok := util.LookUpHTML5EntityByName(string(src[next:end]))
	if !ok {
		return "", 0, false
	}
	return string(entity.Characters), end + 1, true
}

// looksLikeEntity reports whether s, which starts with "&", is shaped like an
// entity or numeric character reference. Unlike entityAt it does not check the
// name, so the read side errs towards escaping.
func looksLikeEntity(s string) bool {
	i := 1
	if i < len(s) && s[i] == '#' {
		i++
		if i < len(s) && (s[i] == 'x' || s[i] == 'X') {
			i++
		}
	}
	start := i
	for i < len(s) && util.IsAlphaNumeric(s[i]) {
		i++
	}
	// Reaching the end of the text node means the next node could supply
	// the rest of the reference, so a prefix counts as well.
	return i == len(s) || (i > start && s[i] == ';')
}

// textContext says where a text node's Markdown lands.
type textContext struct {
	// lineStart: the text begins a line of Markdown.
	lineStart bool
	// inLink: the text becomes link text.
	inLink bool
	// bareStart: nothing at all precedes the text on its line, not even a
	// mark delimiter.
	bareStart bool
	// linkifyAtStart: goldmark's Linkify is tried at the first character,
	// because a line head or another inline construct (a delimiter, a tag)
	// comes right before it.
	linkifyAtStart bool
	// tildeBefore, tildeAfter: the neighbouring text node ends (or starts)
	// with "~" right against this one, so an edge tilde here would join its
	// run in the Markdown.
	tildeBefore, tildeAfter bool
	// textAfter: the next inline node is text written with no delimiter in
	// between, so an autolink at the end of this text could run into it.
	textAfter bool
	// lineEnd: the text is unmarked and its end is the end of the inline
	// content, where a paragraph, heading or cell loses trailing blanks.
	lineEnd bool
}

// tildeEdge reports whether nodes[k] is a text node whose Markdown starts
// (atStart) or ends with a literal "~". Code text is wrapped in backticks, so
// its tildes never meet a neighbour's.
func tildeEdge(nodes []jira4claude.ADFNode, k int, atStart bool) bool {
	if k < 0 || k >= len(nodes) {
		return false
	}
	n := nodes[k]
	if n.Type != "text" || n.Text == "" {
		return false
	}
	if hasCodeMark(n.Marks) {
		return false
	}
	if atStart {
		return n.Text[0] == '~'
	}
	return n.Text[len(n.Text)-1] == '~'
}

// hasCodeMark reports whether marks include code.
func hasCodeMark(marks []jira4claude.ADFMark) bool {
	for _, m := range marks {
		if m.Type == "code" {
			return true
		}
	}
	return false
}

// renderedMarks are the marks applyMarks writes as Markdown or HTML syntax.
func hasRenderedMark(marks []jira4claude.ADFMark) bool {
	for _, m := range marks {
		switch m.Type {
		case "strong", "em", "strike", "code", "underline", "subsup", "link":
			return true
		}
	}
	return false
}

// escapeTextNode returns the text of an ADF text node escaped for Markdown
// output. Code text is verbatim, and link text equal to its href is left as
// is so that it is still read out as a bare URL. The node is siblings[i],
// where siblings holds only nodes that emit output (see emittingInline), so
// its neighbours there are the ones that meet its edges in the Markdown.
func escapeTextNode(siblings []jira4claude.ADFNode, i int, lineStart bool) string {
	node := &siblings[i]
	afterConstruct := i > 0 && (siblings[i-1].Type == "inlineCard" ||
		siblings[i-1].Type == "text" && hasRenderedMark(siblings[i-1].Marks))
	inLink := false
	for _, mark := range node.Marks {
		switch mark.Type {
		case "code":
			return node.Text
		case "link":
			if linkHref(mark) == node.Text {
				return node.Text
			}
			inLink = true
		}
	}
	return escapeText(node.Text, textContext{
		lineStart:      lineStart,
		inLink:         inLink,
		bareStart:      lineStart && len(node.Marks) == 0,
		linkifyAtStart: lineStart || afterConstruct || hasRenderedMark(node.Marks),
		tildeBefore:    tildeEdge(siblings, i-1, false),
		tildeAfter:     tildeEdge(siblings, i+1, true),
		textAfter: i+1 < len(siblings) && siblings[i+1].Type == "text" &&
			!hasRenderedMark(siblings[i+1].Marks),
		lineEnd: len(node.Marks) == 0 && onlyHardBreaksFrom(siblings, i+1),
	})
}

// linkHref returns the href of a link mark, or "" if it has none.
func linkHref(mark jira4claude.ADFMark) string {
	var attrs struct {
		Href string `json:"href"`
	}
	if err := json.Unmarshal(mark.Attrs, &attrs); err != nil {
		return ""
	}
	return attrs.Href
}

// escapeText escapes plain text held by Jira so that it reads back as the
// same text; ctx says where it lands. The rule is described at the top of
// this file.
func escapeText(s string, ctx textContext) string {
	out, keptAutolink := escapeTextWith(s, ctx, true)
	if keptAutolink && !readsBackAs(out, s) {
		// The verbatim autolink span did not survive in its emitted
		// surroundings (Linkify ended it elsewhere, or an entity at its edge
		// was decoded). Plain escaping is always symmetric, so use that.
		out, _ = escapeTextWith(s, ctx, false)
	}
	return out
}

// readsBackAs reports whether md, parsed on its own as the write side parses
// it, yields exactly the text want: decoded text plus raw autolink labels.
// Blanks at either end are ignored, since a paragraph trims them.
func readsBackAs(md, want string) bool {
	source := []byte(md)
	doc := newMarkdownParser().Parse(text.NewReader(source))
	para := doc.FirstChild()
	if para == nil || para.NextSibling() != nil || para.Kind() != ast.KindParagraph {
		return false
	}
	var got strings.Builder
	for n := para.FirstChild(); n != nil; n = n.NextSibling() {
		switch n := n.(type) {
		case *ast.Text:
			if n.HardLineBreak() {
				return false
			}
			if n.IsRaw() {
				got.Write(n.Segment.Value(source))
			} else {
				got.WriteString(decodeText(n.Segment.Value(source)))
			}
			if n.SoftLineBreak() {
				got.WriteByte('\n')
			}
		case *ast.AutoLink:
			got.Write(n.Label(source))
		default:
			return false
		}
	}
	return strings.Trim(got.String(), " \t") == strings.Trim(want, " \t")
}

// escapeTextWith escapes s; with keepAutolinks, a span Linkify will read as
// an autolink is written verbatim. It reports whether any span was.
func escapeTextWith(s string, ctx textContext, keepAutolinks bool) (string, bool) {
	kept := false
	var b strings.Builder
	atLineStart := ctx.lineStart
	afterEncodedBlank := false
	for i := 0; i < len(s); {
		c := s[i]
		if c == '\n' {
			b.WriteByte(c)
			i++
			atLineStart = true
			continue
		}
		// The content loses its trailing blanks on parse too, so blanks that
		// end unmarked text at the end of the content are written as
		// character references as well. Marked text there is left to
		// applyMarks.
		if ctx.lineEnd && (c == ' ' || c == '\t') {
			j := i
			for j < len(s) && (s[j] == ' ' || s[j] == '\t') {
				j++
			}
			if j == len(s) {
				for ; i < j; i++ {
					b.WriteString("&#" + strconv.Itoa(int(s[i])) + ";")
				}
				afterEncodedBlank = true
				continue
			}
		}
		// A paragraph line loses its leading blanks on parse, so they are
		// written as character references, which decodeText turns back.
		// Marked text is left to applyMarks, which moves its blanks outside
		// the delimiters.
		if atLineStart && (ctx.bareStart || i > 0) && (c == ' ' || c == '\t') {
			b.WriteString("&#" + strconv.Itoa(int(c)) + ";")
			i++
			afterEncodedBlank = true
			continue
		}
		if keepAutolinks && !ctx.inLink && !afterEncodedBlank && linkifyMayStart(s, i, ctx.linkifyAtStart) {
			// A span that reaches the end of the node could run on into
			// the next node's text, which this check cannot see.
			if n := autolinkLen(s[i:]); n > 0 && (i+n < len(s) || !ctx.textAfter) {
				kept = true
				b.WriteString(s[i : i+n])
				i += n
				atLineStart = false
				continue
			}
		}
		afterEncodedBlank = false
		if atLineStart {
			atLineStart = false
			if n := escapeLineStart(&b, s, i); n > 0 {
				i += n
				continue
			}
			// "*", "_" and "~" runs at the start of a line are handled below
			// with an unknown left neighbour.
			if c == '*' || c == '_' || c == '~' {
				i += escapeRun(&b, s, i, true, ctx)
				continue
			}
		}
		switch c {
		case '\\':
			if i+1 == len(s) || s[i+1] == '\n' || util.IsPunct(s[i+1]) {
				b.WriteByte('\\')
			}
		case '`', '[':
			b.WriteByte('\\')
		case ']':
			if ctx.inLink {
				b.WriteByte('\\')
			}
		case '&':
			if i+1 == len(s) || looksLikeEntity(s[i:]) {
				b.WriteByte('\\')
			}
		case '<':
			if mayOpenHTML(s[i:]) {
				b.WriteByte('\\')
			}
		case '*', '_', '~':
			i += escapeRun(&b, s, i, false, ctx)
			continue
		}
		b.WriteByte(c)
		i++
	}
	return b.String(), kept
}

// mayOpenHTML reports whether s, which starts with "<", could be parsed as
// HTML or an autolink. goldmark's inline raw HTML parser (parser/raw_html.go)
// and HTML block parser (parser/html_block.go) need a letter, "/", "!" or "?"
// after the "<"; its autolink parser (parser/auto_link.go) needs a URL, which
// starts with a letter, or an email address closed by ">". A "<" at the end of
// the text node is escaped too, since the next node may supply the rest.
func mayOpenHTML(s string) bool {
	if len(s) == 1 {
		return true
	}
	switch c := s[1]; {
	case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c == '/', c == '!', c == '?':
		return true
	}
	stop := util.FindEmailIndex([]byte(s[1:]))
	return stop >= 0 && 1+stop < len(s) && s[1+stop] == '>'
}

// linkifyMayStart reports whether goldmark's Linkify parser is tried at s[i]
// when the escaped text is parsed: at the head of a line, right after another
// inline construct (goldmark restarts its scan there), or after a blank or
// "(". Its other triggers ("*", "_", "~") may be escaped, so they are left
// out. At the start of the node, atStart decides.
func linkifyMayStart(s string, i int, atStart bool) bool {
	if i == 0 {
		return atStart
	}
	switch s[i-1] {
	case ' ', '\t', '\n', '(':
		return true
	}
	return false
}

// autolinkLen returns the length of the autolink that goldmark's Linkify
// parser reads at the start of s, or 0 if there is none.
func autolinkLen(s string) int {
	if nl := strings.IndexByte(s, '\n'); nl >= 0 {
		s = s[:nl]
	}
	word := s
	if sp := strings.IndexAny(word, " \t"); sp >= 0 {
		word = word[:sp]
	}
	if !strings.HasPrefix(s, "http:") && !strings.HasPrefix(s, "https:") &&
		!strings.HasPrefix(s, "ftp:") && !strings.HasPrefix(s, "www.") &&
		!strings.Contains(word, "@") {
		return 0
	}
	source := []byte(s)
	node := extension.NewLinkifyParser().Parse(ast.NewParagraph(), text.NewReader(source), parser.NewContext())
	link, ok := node.(*ast.AutoLink)
	if !ok {
		return 0
	}
	return len(link.Label(source))
}

// escapeHeadingClose escapes a run of "#" that ends heading text after a
// blank, which an ATX heading would otherwise strip as its closing sequence.
func escapeHeadingClose(text string) string {
	body := strings.TrimRight(text, "#")
	if body == text || body == "" || !strings.HasSuffix(body, " ") && !strings.HasSuffix(body, "\t") {
		return text
	}
	return body + `\` + text[len(body):]
}

// escapeLineStart writes the escaped form of a block marker that begins at
// s[i], the first non-blank character of a line, and returns the number of
// bytes consumed. It returns 0 when nothing at s[i] opens a block.
func escapeLineStart(b *strings.Builder, s string, i int) int {
	line := s[i:]
	if nl := strings.IndexByte(line, '\n'); nl >= 0 {
		line = line[:nl]
	}
	followedByBlank := func(n int) bool {
		return n == len(line) || line[n] == ' ' || line[n] == '\t'
	}
	onlyOf := func(c byte) bool {
		return strings.Trim(line, string(c)+" \t") == ""
	}

	switch c := line[0]; {
	case c == '#':
		run := 1
		for run < len(line) && line[run] == '#' {
			run++
		}
		if !followedByBlank(run) {
			return 0
		}
	case c == '>':
	case c == '-' || c == '+':
		if !followedByBlank(1) && (c != '-' || !onlyOf('-')) {
			return 0
		}
	case c == '=':
		if !onlyOf('=') {
			return 0
		}
	case c >= '0' && c <= '9':
		digits := 1
		for digits < len(line) && line[digits] >= '0' && line[digits] <= '9' {
			digits++
		}
		if digits > 9 || digits == len(line) || (line[digits] != '.' && line[digits] != ')') || !followedByBlank(digits+1) {
			return 0
		}
		b.WriteString(line[:digits])
		b.WriteByte('\\')
		b.WriteByte(line[digits])
		return digits + 1
	default:
		return 0
	}
	b.WriteByte('\\')
	b.WriteByte(line[0])
	return 1
}

// escapeRun writes the run of "*", "_" or "~" that starts at s[i], escaping
// every character in it when the run could open or close formatting, and
// returns the length of the run. lineStart marks a run that begins a line.
func escapeRun(b *strings.Builder, s string, i int, lineStart bool, ctx textContext) int {
	c := s[i]
	j := i
	for j < len(s) && s[j] == c {
		j++
	}
	run := j - i

	var escape bool
	switch c {
	case '~':
		// A run at a node edge also joins a neighbour's edge tilde.
		escape = run == 2 || (lineStart && run >= 3) ||
			(i == 0 && ctx.tildeBefore) || (j == len(s) && ctx.tildeAfter)
	default:
		before, beforeKnown := rune(0), i > 0 && !lineStart
		if beforeKnown {
			before, _ = utf8.DecodeLastRuneInString(s[:i])
		}
		after, afterKnown := rune(0), j < len(s)
		if afterKnown {
			after, _ = utf8.DecodeRuneInString(s[j:])
		}
		bothSpace := beforeKnown && afterKnown && unicode.IsSpace(before) && unicode.IsSpace(after)
		bothWord := beforeKnown && afterKnown && isWordRune(before) && isWordRune(after)
		escape = !bothSpace && (c != '_' || !bothWord)
	}

	for k := 0; k < run; k++ {
		if escape {
			b.WriteByte('\\')
		}
		b.WriteByte(c)
	}
	return run
}

// isWordRune reports whether r is a letter or digit, so that an underscore
// between two of them cannot open or close emphasis.
func isWordRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r)
}
