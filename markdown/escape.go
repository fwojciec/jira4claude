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
// so trailing punctuation is included or trimmed exactly as on write. A span
// with a scheme that a character reference follows ("&#10;", "&#32;",
// "&#92;"), with only punctuation Linkify trimmed in between, is
// written in angle brackets, since Linkify would take the reference in.

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
	// textAfter: the next inline node is text written with no delimiter in
	// between, so an autolink at the end of this text could run into it.
	textAfter bool
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
// output. Code text is verbatim, and link text equal to its href is left as
// is so that it is still read out as a bare URL. The node is siblings[i],
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
			if _, bare := bareURL(node); bare {
				return node.Text // written as the bare URL, which Linkify reads
			}
			// Any other link is written in brackets, so its label is
			// escaped, even when it is the href.
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
		textAfter: i+1 < len(siblings) && siblings[i+1].Type == "text" &&
			!hasRenderedMark(siblings[i+1].Marks),
		lineEnd:     !hasRenderedMark(node.Marks) && onlyHardBreaksFrom(siblings, i+1),
		breakAfter:  !hasRenderedMark(node.Marks) && i+1 < len(siblings) && siblings[i+1].Type == "hardBreak",
		encodeFirst: edges.first,
		encodeLast:  edges.last,

		breakAutolinks: edges.breakLinks,
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
	// The output is checked and returned as it is written, with a backslash
	// before a hard break already a reference.
	write := func(keepAutolinks bool) (string, bool) {
		out, kept := escapeTextWith(s, ctx, keepAutolinks)
		return withBreakBackslash(out, ctx), kept
	}
	if ctx.breakAutolinks {
		out, _ := write(false)
		return out
	}
	out, keptAutolink := write(true)
	// A character reference written for a blank, a line feed or a backslash
	// before a hard break can also be taken into an autolink that ends right
	// before it, so output with one is checked too.
	if (keptAutolink || strings.Contains(out, "&#")) && !readsBackAs(out, s) {
		// The verbatim autolink span did not survive in its emitted
		// surroundings (Linkify ended it elsewhere, or an entity at its edge
		// was decoded), or it holds ">", so angle brackets cannot close
		// it. Plain escaping is always symmetric, so use that.
		out, _ = write(false)
		if !readsBackAs(out, s) {
			// Linkify still reads the span, now running on into a
			// character reference after it, so the span is kept from
			// being linked.
			ctx.breakAutolinks = true
			out, _ = write(false)
		}
	}
	return out
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
		mid.lineEnd, mid.breakAfter, mid.tildeAfter, mid.textAfter = false, false, false, true
	}
	out := head + escapeText(s, mid)
	if tail != "" && !mid.breakAutolinks && linkifyCrosses(out+tail, len(out)-1, ctx.linkifyAtStart) {
		// An autolink reaching the reference would run on into it
		// ("https://x.test/&#32;" is all one URL to Linkify), or stop at it
		// with its last character cut off ("https://x.tes&#116;").
		mid.breakAutolinks = true
		out = head + escapeText(s, mid)
	}
	return out + tail
}

// linkifyTriggers are the characters after which goldmark's Linkify may be
// tried, taken generously: a blank, "(", an emphasis or strike delimiter
// character, or the end of a tag, link or code span.
const linkifyTriggers = " \t\n(*_~>)]`"

// linkifyCrosses reports whether goldmark's Linkify, run over md, reads an
// autolink that starts before boundary and ends after it. Linkify is tried
// at the start of md when atStart is set, and after linkifyTriggers. Erring
// towards a crossing only breaks an autolink that would not have formed.
func linkifyCrosses(md string, boundary int, atStart bool) bool {
	for _, span := range linkifySpans(md[:boundary], md, atStart, nil) {
		if span[1] > boundary {
			return true
		}
	}
	return false
}

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

// lenAt returns autolinkLen(s[p:]); p must not be less than in the last call.
func (a *autolinkScanner) lenAt(p int) int {
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
		// Linkify's URL patterns need "://" after the scheme. A parse here
		// costs the rest of the word, but only where a URL starts.
		return autolinkLen(word)
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
	// A "www." URL written in angle brackets carries the "http://" that
	// Linkify gives the bare form, so the two compare equal.
	norm := func(s string) string {
		return strings.ReplaceAll(strings.Trim(s, " \t"), "http://www.", "www.")
	}
	return norm(got.String()) == norm(want)
}

// escapeTextWith escapes s; with keepAutolinks, a span Linkify will read as
// an autolink is written verbatim. It reports whether any span was.
func escapeTextWith(s string, ctx textContext, keepAutolinks bool) (string, bool) {
	kept := false
	var b strings.Builder
	links := autolinkScanner{s: s}
	atLineStart := ctx.lineStart
	afterEncodedBlank := false
	// emailEnd is where a kept email address ends (see backslashAfter).
	emailEnd := -1
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
		// Breaking errs the other way: every character Linkify may start
		// after counts, escaped or not.
		if ctx.breakAutolinks && !ctx.inLink && !afterEncodedBlank &&
			(i == 0 && ctx.linkifyAtStart || i > 0 && strings.IndexByte(linkifyTriggers, s[i-1]) >= 0) &&
			links.lenAt(i) > 0 {
			r, n := utf8.DecodeRuneInString(s[i:])
			b.WriteString(charRef(r))
			i += n
			atLineStart = false
			continue
		}
		if keepAutolinks && !ctx.inLink && !afterEncodedBlank && linkifyMayStart(s, i, ctx.linkifyAtStart, b.String()) {
			// A span that reaches the end of the node could run on into
			// the next node's text, which this check cannot see.
			if n := links.lenAt(i); n > 0 && (i+n < len(s) || !ctx.textAfter) {
				kept = true
				span := s[i : i+n]
				// goldmark's Linkify drops an escaping backslash right after
				// an autolink, so a backslash written escaped there is
				// lost: after an email it is written as a reference
				// (below), and a URL is closed as before a reference.
				backslashAfter := i+n < len(s) && s[i+n] == '\\' && (i+n+1 == len(s) || util.IsPunct(s[i+n+1]))
				// An email address has no scheme; a URL may hold "@" too.
				email := strings.Contains(span, "@") && !hasURLScheme(span) && !strings.HasPrefix(span, "www.")
				switch {
				// email comes first: it needs no scan of the rest of the text,
				// which would be quadratic over a run of addresses.
				case email || strings.Contains(span, ">") || !backslashAfter && !referenceBeforeBlank(s, i+n, ctx):
					// Linkify ends an email address at the "&". A URL
					// holding ">" has no closed form: escapeText finds it
					// does not read back and keeps it from being linked.
					b.WriteString(span)
				case hasURLScheme(span):
					// Linkify would take the "&", "#" and ";" of the
					// reference into the URL; angle brackets close it.
					b.WriteString("<" + span + ">")
				default:
					// A "www." URL: in angle brackets it needs the scheme
					// Linkify gives it anyway.
					b.WriteString("<http://" + span + ">")
				}
				i += n
				if email {
					emailEnd = i
				}
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
			if i == emailEnd && (i+1 == len(s) || util.IsPunct(s[i+1])) {
				b.WriteString("&#92;")
				i++
				continue
			}
			// A line feed and trailing blanks at the line end are written as
			// character references, which start with "&".
			if i+1 == len(s) || s[i+1] == '\n' || util.IsPunct(s[i+1]) ||
				ctx.lineEnd && strings.Trim(s[i+1:], " \t") == "" {
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

// referenceBeforeBlank reports whether, from s[i] up to the next blank or
// "<" (where Linkify stops for good), escapeTextWith writes a character as a
// character reference: a line feed, the blanks that end the content, or a
// backslash before a hard break. Characters Linkify left out of the URL at
// s[i] (trailing punctuation) are skipped, since a reference after them would
// pull them back in.
func referenceBeforeBlank(s string, i int, ctx textContext) bool {
	for ; i < len(s); i++ {
		switch s[i] {
		case '\n':
			return true
		case ' ', '\t':
			return ctx.lineEnd && strings.Trim(s[i:], " \t") == ""
		case '<':
			return false
		}
	}
	return ctx.breakAfter && strings.HasSuffix(s, `\`)
}

// hasURLScheme reports whether an autolink span starts with a scheme, so it
// can be written as a CommonMark autolink in angle brackets.
func hasURLScheme(span string) bool {
	return strings.HasPrefix(span, "http:") || strings.HasPrefix(span, "https:") || strings.HasPrefix(span, "ftp:")
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

// linkifyMayStart reports whether goldmark's Linkify parser is tried at s[i]
// when the escaped text is parsed: at the head of a line, right after another
// inline construct (goldmark restarts its scan there), or after a blank or
// "(". Its other triggers ("*", "_", "~") count only when out, the text
// written so far, ends with them unescaped. A tab is not a trigger: a span kept verbatim after one is not read as
// a link, and its "*" or "_" can pair with a delimiter after it. At the start
// of the node, atStart decides.
func linkifyMayStart(s string, i int, atStart bool, out string) bool {
	if i == 0 {
		return atStart
	}
	switch c := s[i-1]; c {
	case ' ', '\n', '(':
		return true
	case '*', '_', '~':
		return strings.HasSuffix(out, string(c)) && !strings.HasSuffix(out, `\`+string(c))
	}
	return false
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
		bothSpace := beforeKnown && afterKnown && writtenAsSpace(before) && writtenAsSpace(after)
		// Breaking autolinks may turn either neighbour into a character
		// reference, which is punctuation, so snake_case is not relied on.
		bothWord := !ctx.breakAutolinks && beforeKnown && afterKnown && isWordRune(before) && isWordRune(after)
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
