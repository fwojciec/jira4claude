package markdown

import (
	"strings"

	"github.com/fwojciec/jira4claude"
	"github.com/yuin/goldmark/ast"
	extast "github.com/yuin/goldmark/extension/ast"
)

// tagAction is what a markTracker does with one raw inline HTML tag.
type tagAction int

const (
	// tagLiteral keeps the tag in the output as literal text. It is the
	// default, so a tag the pre-scan did not pair is never deleted.
	tagLiteral tagAction = iota
	// tagOpen pushes the tag's mark: a recognised opening tag whose matching
	// closing tag follows in the same run.
	tagOpen
	// tagClose pops the mark of the tagOpen it was paired with.
	tagClose
)

// markTracker turns inline HTML tags (<u>, <sub>, <sup>) into ADF marks
// across one run of sibling inline nodes (the children of one paragraph,
// emphasis, link and so on).
//
// A tag only changes the marks if it belongs to a well-formed pair inside the
// run. The pairs are resolved by a pre-scan, because whether an opening tag
// is ever closed depends on siblings not yet converted. The transitions are:
//
//   - a recognised opening tag is pushed onto the pre-scan stack as a
//     candidate;
//   - a closing tag pairs with the innermost candidate of the same tag: the
//     opening tag becomes tagOpen and the closing tag tagClose, and the
//     candidates opened after it are dropped from the stack and stay
//     tagLiteral (left open inside the pair, they are unclosed or
//     mis-nested);
//   - a closing tag with no candidate of the same tag (stray) stays
//     tagLiteral;
//   - a candidate never closed in the run stays tagLiteral;
//   - an unrecognised tag stays tagLiteral.
//
// Because only properly nested pairs are promoted, replaying the actions in
// order always pops the mark that was pushed last.
type markTracker struct {
	actions map[*ast.RawHTML]tagAction
	stack   []jira4claude.ADFMark
}

// newMarkTracker pre-scans the run of siblings that starts at first.
func newMarkTracker(first ast.Node, source []byte) *markTracker {
	t := &markTracker{actions: make(map[*ast.RawHTML]tagAction)}

	type candidate struct {
		node *ast.RawHTML
		tag  string
	}
	var open []candidate
	for n := first; n != nil; n = n.NextSibling() {
		rh, ok := n.(*ast.RawHTML)
		if !ok {
			continue
		}
		tag := rawHTMLContent(rh, source)
		if _, ok := openingHTMLTagToMark(tag); ok {
			open = append(open, candidate{node: rh, tag: tag})
			continue
		}
		for i := len(open) - 1; i >= 0; i-- {
			if closesTag(tag, open[i].tag) {
				// Candidates above i stay literal: they would have to
				// close inside this pair, and the pair is now closed.
				t.actions[open[i].node] = tagOpen
				t.actions[rh] = tagClose
				open = open[:i]
				break
			}
		}
	}
	return t
}

// closesTag reports whether closing is the closing form of the opening tag.
func closesTag(closing, opening string) bool {
	return strings.HasPrefix(closing, "</") && "<"+closing[2:] == opening
}

// visit applies the action resolved for rh and returns it.
func (t *markTracker) visit(rh *ast.RawHTML, source []byte) tagAction {
	action := t.actions[rh]
	switch action {
	case tagOpen:
		mark, _ := openingHTMLTagToMark(rawHTMLContent(rh, source))
		t.stack = append(t.stack, mark)
	case tagClose:
		t.stack = t.stack[:len(t.stack)-1]
	case tagLiteral:
	}
	return action
}

// marks returns parent followed by the marks of the currently open tags.
// It returns parent itself when no tag is open, so nil stays nil.
func (t *markTracker) marks(parent []jira4claude.ADFMark) []jira4claude.ADFMark {
	if len(t.stack) == 0 {
		return parent
	}
	return append(cloneMarks(parent), t.stack...)
}

// literalHTML is the text a tag is kept as. A tag that spans lines keeps a
// space for each line ending, as a soft line break does.
func literalHTML(rh *ast.RawHTML, source []byte) string {
	text := strings.ReplaceAll(rawHTMLContent(rh, source), "\r\n", "\n")
	return strings.ReplaceAll(text, "\n", " ")
}

// convertInlineRun converts the run of sibling inline nodes that starts at
// first, applying marks plus the marks of the run's paired HTML tags. Tags
// that are not part of a pair are kept as literal text.
func convertInlineRun(first ast.Node, source []byte, marks []jira4claude.ADFMark, skipped *skippedCollector) []jira4claude.ADFNode {
	var content []jira4claude.ADFNode
	tracker := newMarkTracker(first, source)

	for child := first; child != nil; child = child.NextSibling() {
		if _, ok := child.(*extast.TaskCheckBox); ok {
			continue // carried by the taskItem's state attribute
		}
		if rh, ok := child.(*ast.RawHTML); ok {
			if tracker.visit(rh, source) == tagLiteral {
				content = append(content, textNodeWithMarks(literalHTML(rh, source), tracker.marks(marks)))
			}
			continue
		}
		content = append(content, convertInlineNode(child, source, tracker.marks(marks), skipped)...)
	}
	return content
}
