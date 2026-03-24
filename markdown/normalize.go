package markdown

import (
	"strings"
	"unicode"
)

// NormalizeUnicode makes text safe for the Jira API by replacing non-ASCII
// symbols and punctuation with spaces. Letters, numbers, and combining marks
// (accented characters, CJK, Cyrillic, etc.) are preserved.
func NormalizeUnicode(s string) string {
	if !hasNonASCII(s) {
		return s
	}

	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch {
		case r < 128:
			b.WriteRune(r)
		case unicode.IsLetter(r), unicode.IsNumber(r), unicode.IsMark(r):
			b.WriteRune(r)
		default:
			b.WriteRune(' ')
		}
	}
	return b.String()
}

// hasNonASCII returns true if the string contains any non-ASCII bytes.
// This avoids allocating a strings.Builder for the common case of
// ASCII-only text.
func hasNonASCII(s string) bool {
	for i := range len(s) {
		if s[i] >= 128 {
			return true
		}
	}
	return false
}
