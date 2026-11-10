package markdown

import (
	"sort"

	"github.com/fwojciec/jira4claude"
)

// markRank gives the canonical order of the marks the converter writes. It
// follows the mark order of Atlassian's ADF (ProseMirror) schema, so the
// order matches what Jira itself returns. Marks written in one order compare
// equal (marksEqual) only when the order is fixed: Markdown nesting
// ("**_a_**" against "_**a**_") must not decide it.
func markRank(markType string) int {
	switch markType {
	case "link":
		return 0
	case "em":
		return 1
	case "strong":
		return 2
	case "strike":
		return 3
	case "subsup":
		return 4
	case "underline":
		return 5
	case "code":
		return 6
	default:
		return 7
	}
}

// conflictingMarksDetail explains the degraded issue for nested marks of one
// type with different attributes.
const conflictingMarksDetail = "nested marks of one type with different attributes (<sub> and <sup>, or a link in a link) " +
	"cannot all apply to ADF text: the innermost is kept"

// canonicalMarks returns marks with each mark type once and in canonical
// order (markRank). marks is in nesting order, outermost first, so when one
// type appears more than once the last (innermost) is kept; conflict reports
// that two of them had different attributes. The input is not modified, and
// nil or empty input returns it unchanged.
func canonicalMarks(marks []jira4claude.ADFMark) ([]jira4claude.ADFMark, bool) {
	if len(marks) == 0 {
		return marks, false
	}
	out := make([]jira4claude.ADFMark, 0, len(marks))
	conflict := false
	for _, m := range marks {
		replaced := false
		for k := range out {
			if out[k].Type == m.Type {
				if string(out[k].Attrs) != string(m.Attrs) {
					conflict = true
				}
				out[k] = m
				replaced = true
				break
			}
		}
		if !replaced {
			out = append(out, m)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		return markRank(out[i].Type) < markRank(out[j].Type)
	})
	return out, conflict
}

// sameMarkSet reports whether a and b hold the same marks in any order.
func sameMarkSet(a, b []jira4claude.ADFMark) bool {
	ca, _ := canonicalMarks(a)
	cb, _ := canonicalMarks(b)
	return marksEqual(ca, cb)
}
