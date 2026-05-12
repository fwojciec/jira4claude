package markdown

import (
	"strings"
	"unicode"
)

// NormalizeUnicode makes text safe for the Jira API by replacing control
// characters with spaces. Tab, newline, and carriage return are preserved
// because they carry structural meaning in markdown source. All printable
// Unicode — including symbols, punctuation, smart quotes, em-dashes,
// arrows, and CJK / Cyrillic / accented letters — is preserved.
//
// The ADF JSON schema imposes no character restrictions on text-node
// content beyond minLength: 1, so an aggressive blocklist over-strips.
func NormalizeUnicode(s string) string {
	if !hasControlChars(s) {
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
