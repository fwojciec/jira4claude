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
//     underline, thematic break), the "." or ")" of an ordered list
//     marker, and the first "-" of a line shaped like a table
//     delimiter row ("|-", ":-:", "---|---").
//   - In a heading only: a "#" run that ends the text after a blank (an ATX
//     closing sequence).
//
// A line feed in text is written as "&#10;" (raw, it would be a soft line
// break, which reads back as a space), and a backslash before it, or before
// trailing blanks written as character references, is escaped.
//
// Everything else is written as is. A hardBreak between text nodes is
// written as a CommonMark hard break, "\" and a newline (see
// adfInlineLineToGFM), so the next line starts a line of the same paragraph.
//
// Text never forms a link (#161): where goldmark's Linkify would read an
// autolink in it (a bare "http://", "https://", "ftp://" or "www." URL, or
// an email address), the first character of the span is written as a
// numeric character reference, so Linkify does not start there (break mode,
// textContext.breakAutolinks). Linkify is tried at a line head, after a blank
// or "(", and right after another inline construct such as a formatting
// delimiter; break mode errs towards trying it (linkifyTriggers). A link is
// written bare, as its URL verbatim, only where Linkify reads it back
// exactly (see the top of inline_marks.go); escapeTextNode leaves such link
// text as it is.
//
// Link destinations are written by linkDestination (inline_marks.go); the
// write side decodes them as it decodes text.

// decodeText resolves backslash escapes and entity references in Markdown
// text. A backslash before anything other than ASCII punctuation is kept, and
// an escaped "&" is never read as the start of an entity. A referenced
// character follows the same rule as raw input (see normalize.go): "&#13;"
// gives a line feed and "&#1;" a space.
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
	return normalizeText(b.String())
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

// unwritableTextDetail explains the degraded issue reported when Jira text
// holds a character that the write side would change (see writableText).
const unwritableTextDetail = "a carriage return, control character or invalid UTF-8 in text cannot be written in Markdown; " +
	"it is written as a line feed, a space or U+FFFD, as the write side would read it"

// writableText returns s as the write side would read it back if it were
// written raw: a carriage return, alone or before a line feed, becomes a line
// feed, any other control character except tab and line feed becomes a space,
// and invalid UTF-8 becomes U+FFFD (the rule in normalize.go). No escape can
// carry these characters, so the caller reports a change. It reports whether
// s changed.
func writableText(s string) (string, bool) {
	clean := normalizeText(s)
	return clean, clean != s
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
	// lineEnd: the text is unmarked and its end is the end of the inline
	// content, where a paragraph, heading or cell loses trailing blanks.
	lineEnd bool
	// breakAfter: the text is unmarked and a hardBreak follows it, so a
	// backslash that ends it is written as "&#92;" (see withBreakBackslash).
	breakAfter bool
	// encodeFirst, encodeLast: write the first (or last) character as a
	// numeric character reference, because it meets a mark delimiter that
	// it would stop from opening or closing, or join (see inline_marks.go).
	encodeFirst, encodeLast bool
	// breakAutolinks: where Linkify would start an autolink, write its first
	// character as a numeric character reference, so the span stays plain
	// text. Used where a span would run on into a character reference or a
	// delimiter written after it (see inline_marks.go).
	breakAutolinks bool
	// linkAfter: a link or inline card may be written right after the text
	// in brackets, so a "!" that ends it would make the link an image.
	linkAfter bool
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

// hasRenderedMark reports whether marks include one that applyMarks writes
// as Markdown or HTML syntax.
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
// output. Code text is verbatim. The node is siblings[i],
// where siblings holds only nodes that emit output (see emittingInline), so
// its neighbours there are the ones that meet its edges in the Markdown.
func escapeTextNode(siblings []jira4claude.ADFNode, i int, lineStart bool, edges textEdges) string {
	node := &siblings[i]
	afterConstruct := i > 0 && (siblings[i-1].Type == "inlineCard" ||
		siblings[i-1].Type == "text" && hasRenderedMark(siblings[i-1].Marks))
	inLink := false
	for _, mark := range node.Marks {
		switch mark.Type {
		case "code":
			return node.Text
		case "link":
			// A link written as its bare URL never gets here (see
			// renderTextItems); any other is written in brackets, so its
			// label is escaped, even when it is the href.
			inLink = true
		}
	}
	return escapeText(node.Text, textContext{
		lineStart:      lineStart,
		inLink:         inLink,
		bareStart:      lineStart && !hasRenderedMark(node.Marks),
		linkifyAtStart: lineStart || afterConstruct || hasRenderedMark(node.Marks),
		tildeBefore:    tildeEdge(siblings, i-1, false),
		tildeAfter:     tildeEdge(siblings, i+1, true),
		lineEnd:        !hasRenderedMark(node.Marks) && onlyHardBreaksFrom(siblings, i+1),
		breakAfter:     !hasRenderedMark(node.Marks) && i+1 < len(siblings) && siblings[i+1].Type == "hardBreak",
		encodeFirst:    edges.first,
		encodeLast:     edges.last,

		breakAutolinks: edges.breakLinks,
		linkAfter: i+1 < len(siblings) && (siblings[i+1].Type == "inlineCard" ||
			siblings[i+1].Type == "text" && hasMark(siblings[i+1].Marks, "link")),
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
	if ctx.encodeFirst || ctx.encodeLast {
		return escapeTextEncodingEdges(s, ctx)
	}
	return withBreakBackslash(escapeTextWith(s, ctx), ctx)
}

// unclosableURLDetail explains the degraded issue reported when Linkify
// would read an autolink on past the end of its node's text and no repair
// in renderTextItems stops it.
const unclosableURLDetail = "a URL that Linkify would read on past its text (into a character reference written for " +
	"a line feed, blank or backslash, or into the next node) has no form here that stops it; " +
	"it reads back taking in what follows"

// escapeTextEncodingEdges escapes s with its first or last character, as
// ctx says, written as a character reference. The rest is escaped as text
// that begins (or ends) right after (or before) that reference: not at a line
// start or end, and not next to a tilde.
func escapeTextEncodingEdges(s string, ctx textContext) string {
	var head, tail string
	if ctx.encodeFirst && s != "" {
		r, n := utf8.DecodeRuneInString(s)
		head, s = charRef(r), s[n:]
	}
	if ctx.encodeLast && s != "" {
		r, n := utf8.DecodeLastRuneInString(s)
		tail, s = charRef(r), s[:len(s)-n]
	}
	if s == "" {
		return head + tail
	}
	mid := ctx
	mid.encodeFirst, mid.encodeLast = false, false
	if head != "" {
		mid.lineStart, mid.bareStart, mid.linkifyAtStart, mid.tildeBefore = false, false, false, false
	}
	if tail != "" {
		mid.lineEnd, mid.breakAfter, mid.tildeAfter, mid.linkAfter = false, false, false, false
	}
	return head + escapeText(s, mid) + tail
}

// linkifyTriggers are the characters after which goldmark's Linkify may be
// tried, taken generously: a blank, "(", an emphasis or strike delimiter
// character, or the end of a tag, link or code span.
const linkifyTriggers = " \t\n(*_~>)]`"

// linkifySpans returns the [start, end) of each autolink Linkify may read in
// md that starts within scan (a prefix of md), in order. Linkify is tried at
// the start of md when atStart is set, after linkifyTriggers, right after
// another autolink, and only where canStart, if not nil, allows.
//
// Like Linkify, it does not look for another autolink inside one it has
// read; with autolinkScanner, the work is linear in len(md).
func linkifySpans(scan, md string, atStart bool, canStart func(p int) bool) [][2]int {
	var spans [][2]int
	links := autolinkScanner{s: md}
	afterLink := -1 // Linkify is tried again right after an autolink
	for p := 0; p < len(scan); p++ {
		if p != afterLink && (p == 0 && !atStart || p > 0 && strings.IndexByte(linkifyTriggers, md[p-1]) < 0) {
			continue
		}
		if canStart != nil && !canStart(p) {
			continue
		}
		if n := links.lenAt(p); n > 0 {
			spans = append(spans, [2]int{p, p + n})
			afterLink = p + n
			p = afterLink - 1
		}
	}
	return spans
}

// autolinkScanner is autolinkLen over positions of one string, asked in
// increasing order, without a parse per position: a word's end is found
// once, and an email is parsed once per "@" (where it ends does not depend
// on where in its local part Linkify starts).
type autolinkScanner struct {
	s       string
	started bool
	wordEnd int // the end of the word holding the last position asked
	// at is the next "@" in the word, or wordEnd if there is none;
	// localStart, the first position the local part before it can start
	// at; emailEnd, where an email through it ends (0: not parsed yet,
	// -1: none).
	at, localStart, emailEnd int
	local                    [256]int8 // per byte: 1 an email local-part byte, -1 not, 0 unknown
}

// urlDecidedWithin bounds how much of a word after a URL start Linkify's URL
// patterns need to decide whether a URL starts there: a scheme or "www.",
// at most 256 host characters, a dot and a letter. Where the URL ends can
// take the whole word.
const urlDecidedWithin = 512

// lenAt returns autolinkLen(s[p:]); p must not be less than in the last call.
func (a *autolinkScanner) lenAt(p int) int {
	return a.scan(p, true)
}

// startsAt reports whether lenAt(p) > 0, without measuring a URL to its end:
// break mode asks at every position Linkify may start, and a word full of
// URL starts would otherwise be parsed to its end at each.
func (a *autolinkScanner) startsAt(p int) bool {
	return a.scan(p, false) > 0
}

// scan is lenAt; without full, a URL is parsed only as far as decides
// whether it starts, and its length is not exact.
func (a *autolinkScanner) scan(p int, full bool) int {
	if !a.started || p > a.wordEnd {
		a.started = true
		a.wordEnd = len(a.s)
		if sp := strings.IndexAny(a.s[p:], " \t\n"); sp >= 0 {
			a.wordEnd = p + sp
		}
		a.at = -1
	}
	word := a.s[p:a.wordEnd]
	if word == "" || strings.IndexByte(" *_~(", word[0]) >= 0 {
		return 0
	}
	if strings.HasPrefix(word, "http://") || strings.HasPrefix(word, "https://") ||
		strings.HasPrefix(word, "ftp://") || strings.HasPrefix(word, "www.") {
		// Linkify's URL patterns need "://" after the scheme. Whether a URL
		// starts is decided within urlDecidedWithin; only a URL that does is
		// parsed to its end, and only when its length is asked for, so a
		// word full of URL starts is not parsed again at each.
		if len(word) <= urlDecidedWithin {
			return autolinkLen(word)
		}
		if n := autolinkLen(word[:urlDecidedWithin]); n > 0 {
			if full {
				return autolinkLen(word)
			}
			return n
		}
		// No URL starts here. Linkify tries an email address next, which a
		// word starting with a scheme cannot be (no local part takes ":"),
		// but a "www." word can: that is the email scan below.
		if !strings.HasPrefix(word, "www.") {
			return 0
		}
	}
	if a.at < p {
		a.at = a.wordEnd
		if k := strings.IndexByte(word, '@'); k >= 0 {
			a.at = p + k
		}
		a.localStart = a.at
		for a.localStart > 0 && a.isLocal(a.s[a.localStart-1]) {
			a.localStart--
		}
		a.emailEnd = 0
	}
	if a.at == a.wordEnd || p < a.localStart || util.IsPunct(word[0]) {
		return 0
	}
	if a.emailEnd == 0 {
		a.emailEnd = -1
		// The email cannot run past the next "@", so only up to there is
		// parsed.
		email := word
		if k := strings.IndexByte(a.s[a.at+1:a.wordEnd], '@'); k >= 0 {
			email = a.s[p : a.at+1+k]
		}
		if n := autolinkLen(email); n > 0 {
			a.emailEnd = p + n
		}
	}
	if a.emailEnd < 0 {
		return 0
	}
	return a.emailEnd - p
}

// isLocal reports whether Linkify takes c into an email's local part.
func (a *autolinkScanner) isLocal(c byte) bool {
	if a.local[c] == 0 {
		a.local[c] = -1
		if c != '@' && util.FindEmailIndex([]byte{c, '@', 'a', '.', 'c', 'o'}) == 6 {
			a.local[c] = 1
		}
	}
	return a.local[c] == 1
}

// charRef returns the decimal numeric character reference for r.
func charRef(r rune) string {
	return "&#" + strconv.Itoa(int(r)) + ";"
}

// escapeTextWith escapes s as escapeText describes, before the hard-break
// fix-up of withBreakBackslash.
func escapeTextWith(s string, ctx textContext) string {
	var b strings.Builder
	links := autolinkScanner{s: s}
	atLineStart := ctx.lineStart
	afterEncodedBlank := false
	// brokeAt is the end of the last character break mode wrote as a
	// reference.
	brokeAt := -1
	// breaksAt reports whether break mode writes s[p] as a reference: Linkify
	// may start there, erring towards it (every character Linkify may start
	// after counts, escaped or not), and would read an autolink.
	breaksAt := func(p int) bool {
		return ctx.breakAutolinks && !ctx.inLink && p < len(s) &&
			(p == 0 && ctx.linkifyAtStart || p > 0 && strings.IndexByte(linkifyTriggers, s[p-1]) >= 0) &&
			links.startsAt(p)
	}
	for i := 0; i < len(s); {
		c := s[i]
		if c == '\n' {
			// A line feed in text is not a line ending: written raw it would
			// be a soft line break, which reads back as a space.
			b.WriteString("&#10;")
			i++
			afterEncodedBlank = true
			continue
		}
		// The content loses its trailing blanks on parse too, so blanks that
		// end unmarked text at the end of the content are written as
		// character references as well. Marked text ends before its
		// delimiters; see inline_marks.go.
		if ctx.lineEnd && (c == ' ' || c == '\t') {
			j := i
			for j < len(s) && (s[j] == ' ' || s[j] == '\t') {
				j++
			}
			if j == len(s) {
				for ; i < j; i++ {
					b.WriteString(charRef(rune(s[i])))
				}
				afterEncodedBlank = true
				continue
			}
		}
		// A paragraph line loses its leading blanks on parse, so they are
		// written as character references, which decodeText turns back.
		// Marked text begins after its delimiters; a blank at its edge is
		// encoded by the edge rules in inline_marks.go.
		if atLineStart && (ctx.bareStart || i > 0) && (c == ' ' || c == '\t') {
			b.WriteString(charRef(rune(c)))
			i++
			afterEncodedBlank = true
			continue
		}
		if !afterEncodedBlank && breaksAt(i) {
			r, n := utf8.DecodeRuneInString(s[i:])
			b.WriteString(charRef(r))
			i += n
			brokeAt = i
			atLineStart = false
			continue
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
				i += escapeRun(&b, s, i, true, ctx, false, false)
				continue
			}
		}
		switch c {
		case '\\':
			// A line feed and trailing blanks at the line end are written as
			// character references, which start with "&".
			if i+1 == len(s) || s[i+1] == '\n' || util.IsPunct(s[i+1]) ||
				ctx.lineEnd && strings.Trim(s[i+1:], " \t") == "" {
				b.WriteByte('\\')
			}
		case '!':
			if i+1 == len(s) && ctx.linkAfter {
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
			// A neighbour that break mode writes as a reference flanks the
			// run as punctuation does.
			j := i
			for j < len(s) && s[j] == c {
				j++
			}
			i += escapeRun(&b, s, i, false, ctx, brokeAt == i, breaksAt(j))
			continue
		}
		b.WriteByte(c)
		i++
	}
	return b.String()
}

// withBreakBackslash returns out as it is written before a hard break: a
// final escaped backslash becomes "&#92;", since goldmark does not take
// "\\\" before a newline as an escaped backslash and a hard break.
func withBreakBackslash(out string, ctx textContext) string {
	if ctx.breakAfter && strings.HasSuffix(out, `\\`) {
		return out[:len(out)-2] + "&#92;"
	}
	return out
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

// autolinkLen returns the length of the autolink that goldmark's Linkify
// parser reads at the start of s, or 0 if there is none.
func autolinkLen(s string) int {
	if s == "" || strings.IndexByte(" *_~(", s[0]) >= 0 {
		// Linkify takes a leading trigger character as the character before
		// the link, so a link read here would start one byte later.
		return 0
	}
	// An autolink ends at a blank or line ending, so only the word at the
	// start of s is parsed (which keeps a scan over a long line linear).
	if sp := strings.IndexAny(s, " \t\n"); sp >= 0 {
		s = s[:sp]
	}
	if !strings.HasPrefix(s, "http:") && !strings.HasPrefix(s, "https:") &&
		!strings.HasPrefix(s, "ftp:") && !strings.HasPrefix(s, "www.") &&
		!strings.Contains(s, "@") {
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

	// A line of only "-", ":", "|" and blanks, with a "-", is a table
	// delimiter row, which makes the line before it a table header. Any
	// backslash on the line keeps it from being one. The first "-" takes it,
	// not a "|": a table cell escapes every "|" again (adfTableCellToGFM).
	if dash := strings.IndexByte(line, '-'); dash >= 0 && strings.Trim(line, "-:| \t") == "" {
		b.WriteString(line[:dash])
		b.WriteString(`\-`)
		return dash + 1
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
//
// beforeRef and afterRef say that the character before (after) the run is
// written as a character reference, which flanks the run as punctuation.
func escapeRun(b *strings.Builder, s string, i int, lineStart bool, ctx textContext, beforeRef, afterRef bool) int {
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
		bothSpace := beforeKnown && afterKnown && writtenAsSpace(before) && writtenAsSpace(after)
		// snake_case is relied on only when neither neighbour is written as
		// a reference.
		bothWord := beforeKnown && afterKnown && !beforeRef && !afterRef && isWordRune(before) && isWordRune(after)
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

// writtenAsSpace reports whether r is whitespace that escapeText writes as
// itself. A line feed is written as "&#10;", which flanks a delimiter run as
// punctuation does.
func writtenAsSpace(r rune) bool {
	return r != '\n' && unicode.IsSpace(r)
}

// isWordRune reports whether r is a letter or digit, so that an underscore
// between two of them cannot open or close emphasis.
func isWordRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r)
}
