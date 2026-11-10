package markdown

import (
	"encoding/json"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/fwojciec/jira4claude"
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
//     ("&name;", "&#123;", "&#x1F;"), or that ends the text node.
//   - A run of "*" unless both neighbours are known whitespace; a run of "_"
//     unless both neighbours are known whitespace or both are letters or
//     digits (snake_case). The start and end of a text node, and the start
//     of a line, count as unknown, because the neighbour there may be a
//     delimiter emitted for another node.
//   - A run of exactly two "~" (only "~~" strikes, see strikethrough.go);
//     at the start of a line, any run of three or more (a code fence).
//   - At the start of a line only: "#" that would open a heading, ">", a
//     "-" or "+" list marker, a line made of only "-" or "=" (setext
//     underline, thematic break), and the "." or ")" of an ordered list
//     marker.
//   - In a heading only: a "#" run that ends the text after a blank (an ATX
//     closing sequence).
//
// Everything else, including "<", is written as is.

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
	return i > start && i < len(s) && s[i] == ';'
}

// escapeTextNode returns the text of an ADF text node escaped for Markdown
// output. Code text is verbatim, and link text equal to its href is left as
// is so that it is still read out as a bare URL.
func escapeTextNode(node *jira4claude.ADFNode, lineStart bool) string {
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
	return escapeText(node.Text, lineStart, inLink)
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
// same text. lineStart is true when the text begins a line of Markdown;
// inLink is true when the text becomes link text. The rule is described at
// the top of this file.
func escapeText(s string, lineStart, inLink bool) string {
	var b strings.Builder
	atLineStart := lineStart
	for i := 0; i < len(s); {
		c := s[i]
		if c == '\n' {
			b.WriteByte(c)
			i++
			atLineStart = true
			continue
		}
		if atLineStart && c != ' ' && c != '\t' {
			atLineStart = false
			if n := escapeLineStart(&b, s, i); n > 0 {
				i += n
				continue
			}
			// "*", "_" and "~" runs at the start of a line are handled below
			// with an unknown left neighbour.
			if c == '*' || c == '_' || c == '~' {
				i += escapeRun(&b, s, i, true)
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
			if inLink {
				b.WriteByte('\\')
			}
		case '&':
			if i+1 == len(s) || looksLikeEntity(s[i:]) {
				b.WriteByte('\\')
			}
		case '*', '_', '~':
			i += escapeRun(&b, s, i, false)
			continue
		}
		b.WriteByte(c)
		i++
	}
	return b.String()
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
func escapeRun(b *strings.Builder, s string, i int, lineStart bool) int {
	c := s[i]
	j := i
	for j < len(s) && s[j] == c {
		j++
	}
	run := j - i

	var escape bool
	switch c {
	case '~':
		escape = run == 2 || (lineStart && run >= 3)
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
