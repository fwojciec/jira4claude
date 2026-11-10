package markdown

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// The write side normalises unusual input with one rule, whether a character
// is typed raw or arrives through a numeric character reference (#162):
//
//   - A carriage return, alone or before a line feed, is a line ending and
//     becomes a line feed (CommonMark 2.1). Raw, this happens before parsing;
//     from a reference, the line feed is text, as any referenced character is.
//   - Any other control character except tab becomes a space
//     (NormalizeUnicode).
//   - Invalid UTF-8 becomes U+FFFD, as encoding/json would make it anyway.
//
// None of this is reported: control characters are not content Jira can hold
// safely, and the other two change no character a user could see.

// normalizeLineEndings turns every CRLF and lone CR into LF.
func normalizeLineEndings(s string) string {
	if !strings.Contains(s, "\r") {
		return s
	}
	return strings.ReplaceAll(strings.ReplaceAll(s, "\r\n", "\n"), "\r", "\n")
}

// normalizeText applies the write-side rule to s: to the Markdown source
// before parsing, and again to decoded text, for characters that came from
// character references.
func normalizeText(s string) string {
	return NormalizeUnicode(normalizeLineEndings(s))
}

// NormalizeUnicode makes text safe for the Jira API by replacing control
// characters with spaces and invalid UTF-8 with U+FFFD. Tab, newline, and
// carriage return are preserved because they carry structural meaning in
// markdown source. All printable Unicode — including symbols, punctuation,
// smart quotes, em-dashes, arrows, and CJK / Cyrillic / accented letters —
// is preserved.
//
// The ADF JSON schema imposes no character restrictions on text-node
// content beyond minLength: 1, so an aggressive blocklist over-strips.
func NormalizeUnicode(s string) string {
	if !hasControlChars(s) && utf8.ValidString(s) {
		return s
	}

	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if isPreservedControl(r) {
			b.WriteRune(r)
			continue
		}
		if unicode.IsControl(r) {
			b.WriteRune(' ')
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// isPreservedControl returns true for control characters that carry useful
// structural meaning in markdown source and must not be stripped.
func isPreservedControl(r rune) bool {
	return r == '\t' || r == '\n' || r == '\r'
}

// hasControlChars returns true if the string contains any control character
// that NormalizeUnicode would replace. This avoids allocating a builder for
// the common case of text containing only printable characters.
func hasControlChars(s string) bool {
	for _, r := range s {
		if isPreservedControl(r) {
			continue
		}
		if unicode.IsControl(r) {
			return true
		}
	}
	return false
}
