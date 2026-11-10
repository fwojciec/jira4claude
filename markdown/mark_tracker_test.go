package markdown_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/fwojciec/jira4claude"
	"github.com/fwojciec/jira4claude/markdown"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// span is a text node flattened for comparison: its text and the names of its
// marks in order. A subsup mark is named after its type ("sub" or "sup").
type span struct {
	Text  string
	Marks []string
}

// inlineSpans flattens every text node of doc, in document order, into spans.
func inlineSpans(t *testing.T, doc *jira4claude.ADFNode) []span {
	t.Helper()

	var spans []span
	var walk func(n jira4claude.ADFNode)
	walk = func(n jira4claude.ADFNode) {
		if n.Type == "text" {
			s := span{Text: n.Text}
			for _, m := range n.Marks {
				s.Marks = append(s.Marks, markName(t, m))
			}
			spans = append(spans, s)
		}
		for _, c := range n.Content {
			walk(c)
		}
	}
	walk(*doc)
	return spans
}

func markName(t *testing.T, m jira4claude.ADFMark) string {
	t.Helper()

	if m.Type != "subsup" {
		return m.Type
	}
	var attrs struct {
		Type string `json:"type"`
	}
	require.NoError(t, json.Unmarshal(m.Attrs, &attrs))
	return attrs.Type
}

// TestMarkTracker_Transitions covers every transition of the inline HTML mark
// tracker, one row per transition. A recognised opening tag (<u>, <sub>,
// <sup>) becomes a mark only when a matching closing tag follows in the same
// inline container and the pair nests properly; every other tag is kept as
// literal text, so nothing is ever deleted and nothing is reported as lost.
func TestMarkTracker_Transitions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		markdown string
		want     []span
	}{
		{
			name:     "open then matching close applies underline",
			markdown: "a <u>b</u> c",
			want:     []span{{Text: "a "}, {Text: "b", Marks: []string{"underline"}}, {Text: " c"}},
		},
		{
			name:     "open then matching close applies sub",
			markdown: "H<sub>2</sub>O",
			want:     []span{{Text: "H"}, {Text: "2", Marks: []string{"sub"}}, {Text: "O"}},
		},
		{
			name:     "open then matching close applies sup",
			markdown: "x<sup>2</sup>",
			want:     []span{{Text: "x"}, {Text: "2", Marks: []string{"sup"}}},
		},
		{
			name:     "properly nested pairs stack their marks",
			markdown: "<u>a<sub>b</sub>c</u>",
			want: []span{
				{Text: "a", Marks: []string{"underline"}},
				{Text: "b", Marks: []string{"underline", "sub"}},
				{Text: "c", Marks: []string{"underline"}},
			},
		},
		{
			name:     "sequential pairs reopen the mark",
			markdown: "<u>a</u> <u>b</u>",
			want: []span{
				{Text: "a", Marks: []string{"underline"}},
				{Text: " "},
				{Text: "b", Marks: []string{"underline"}},
			},
		},
		{
			name:     "unrecognised tag is literal",
			markdown: "pass <word> as the argument",
			want:     []span{{Text: "pass <word> as the argument"}},
		},
		{
			name:     "unrecognised closing tag is literal",
			markdown: "a </word> b",
			want:     []span{{Text: "a </word> b"}},
		},
		{
			name:     "stray close with nothing open is literal",
			markdown: "stray </u> close here",
			want:     []span{{Text: "stray </u> close here"}},
		},
		{
			name:     "unclosed open is literal and applies no mark",
			markdown: "x <u>a never closed",
			want:     []span{{Text: "x <u>a never closed"}},
		},
		{
			name:     "close before open leaves both literal",
			markdown: "a </u>b<u> c",
			want:     []span{{Text: "a </u>b<u> c"}},
		},
		{
			name:     "close of a different tag inside a pair is literal",
			markdown: "<u>a</sub>b</u>",
			want:     []span{{Text: "a</sub>b", Marks: []string{"underline"}}},
		},
		{
			name:     "unrecognised tag inside a pair carries the pair's mark",
			markdown: "<u>a <word> b</u>",
			want:     []span{{Text: "a <word> b", Marks: []string{"underline"}}},
		},
		{
			name:     "unclosed inner open inside a pair is literal",
			markdown: "<u>a <sub>b</u>",
			want:     []span{{Text: "a <sub>b", Marks: []string{"underline"}}},
		},
		{
			name:     "unclosed outer open around a pair is literal",
			markdown: "<u>a <sub>b</sub>",
			want:     []span{{Text: "<u>a "}, {Text: "b", Marks: []string{"sub"}}},
		},
		{
			// Mis-nested: the close of the outer tag arrives while the inner
			// tag is still open. The first close pairs with its open; the
			// inner tag, left open inside that pair, and its late close
			// become literal text.
			name:     "mis-nested pair keeps the first closed mark and the rest as literal",
			markdown: "a <u>b <sub>c</u> d</sub> e",
			want: []span{
				{Text: "a "},
				{Text: "b <sub>c", Marks: []string{"underline"}},
				{Text: " d</sub> e"},
			},
		},
		{
			name:     "close pairs with the innermost open of the same tag",
			markdown: "<u>a <u>b</u> c",
			want:     []span{{Text: "<u>a "}, {Text: "b", Marks: []string{"underline"}}, {Text: " c"}},
		},
		{
			name:     "pair split across an emphasis boundary is literal",
			markdown: "<u>a *b</u> c*",
			want:     []span{{Text: "<u>a "}, {Text: "b</u> c", Marks: []string{"em"}}},
		},
		{
			name:     "pair around emphasis applies both marks",
			markdown: "<u>*a*</u>",
			want:     []span{{Text: "a", Marks: []string{"underline", "em"}}},
		},
		{
			name:     "pair inside emphasis applies both marks",
			markdown: "*<u>a</u>*",
			want:     []span{{Text: "a", Marks: []string{"em", "underline"}}},
		},
		{
			name:     "pair inside link applies both marks",
			markdown: "[<u>a</u> b](https://x.test)",
			want: []span{
				{Text: "a", Marks: []string{"link", "underline"}},
				{Text: " b", Marks: []string{"link"}},
			},
		},
		{
			name:     "recognised tag with attributes is literal",
			markdown: "<u class=\"x\">a</u>",
			want:     []span{{Text: "<u class=\"x\">a</u>"}},
		},
		{
			name:     "void tag is literal",
			markdown: "a<br>b",
			want:     []span{{Text: "a<br>b"}},
		},
		{
			name:     "inline comment is literal",
			markdown: "a <!-- c --> b",
			want:     []span{{Text: "a <!-- c --> b"}},
		},
		{
			name:     "tag spanning a line break is literal with a space",
			markdown: "x <a\nhref=\"y\"> z",
			want:     []span{{Text: "x <a href=\"y\"> z"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			doc, issues := markdown.New().ToADF(tt.markdown)
			requireValidADF(t, doc)
			assert.Empty(t, issues)
			assert.Equal(t, tt.want, inlineSpans(t, doc))
		})
	}
}

// TestMarkTracker_EveryInlineContainer runs the same unbalanced and balanced
// inputs through every container that converts a run of inline nodes.
func TestMarkTracker_EveryInlineContainer(t *testing.T) {
	t.Parallel()

	containers := []struct {
		name string
		wrap func(inline string) string
	}{
		{name: "paragraph", wrap: func(s string) string { return s }},
		{name: "heading", wrap: func(s string) string { return "# " + s }},
		{name: "bullet list item", wrap: func(s string) string { return "- " + s }},
		{name: "blockquote", wrap: func(s string) string { return "> " + s }},
		{name: "panel first paragraph", wrap: func(s string) string { return "> [!NOTE]\n> " + s }},
		{name: "panel later paragraph", wrap: func(s string) string { return "> [!NOTE]\n> intro\n>\n> " + s }},
		{name: "task item", wrap: func(s string) string { return "- [ ] " + s }},
		{name: "table cell", wrap: func(s string) string { return "| h |\n| - |\n| " + s + " |" }},
		{name: "strong emphasis", wrap: func(s string) string { return "**" + s + "**" }},
		{name: "link text", wrap: func(s string) string { return "[" + s + "](https://x.test)" }},
	}

	// text is the visible text of the input; underlined is the only part of
	// it that may carry the underline mark.
	unbalanced := []struct {
		markdown   string
		text       string
		underlined string
	}{
		{markdown: "pass <word> as the argument", text: "pass <word> as the argument"},
		{markdown: "stray </u> close", text: "stray </u> close"},
		{markdown: "x <u>a never closed", text: "x <u>a never closed"},
		{markdown: "a <u>b <sub>c</u> d</sub> e", text: "a b <sub>c d</sub> e", underlined: "b <sub>c"},
	}

	balanced := []struct {
		markdown string
		text     string
		mark     string
	}{
		{markdown: "a <u>b</u> c", text: "b", mark: "underline"},
		{markdown: "a <sub>b</sub> c", text: "b", mark: "sub"},
		{markdown: "a <sup>b</sup> c", text: "b", mark: "sup"},
	}

	for _, c := range containers {
		for _, u := range unbalanced {
			t.Run(c.name+"/unbalanced/"+u.markdown, func(t *testing.T) {
				t.Parallel()

				doc, issues := markdown.New().ToADF(c.wrap(u.markdown))
				requireValidADF(t, doc)
				assert.Empty(t, issues)

				var text strings.Builder
				for _, s := range inlineSpans(t, doc) {
					text.WriteString(s.Text)
					if s.Text != u.underlined {
						assert.NotContains(t, s.Marks, "underline", "no underline expected on %q", s.Text)
					}
				}
				assert.Contains(t, text.String(), u.text)
			})
		}
		for _, b := range balanced {
			t.Run(c.name+"/balanced/"+b.markdown, func(t *testing.T) {
				t.Parallel()

				doc, issues := markdown.New().ToADF(c.wrap(b.markdown))
				requireValidADF(t, doc)
				assert.Empty(t, issues)

				var found bool
				for _, s := range inlineSpans(t, doc) {
					if s.Text == b.text {
						found = true
						assert.Contains(t, s.Marks, b.mark)
					}
					if s.Text != b.text {
						assert.NotContains(t, s.Marks, b.mark, "mark leaked onto %q", s.Text)
					}
				}
				assert.True(t, found, "no text node %q", b.text)
			})
		}
	}
}

// TestRoundTrip_LiteralInlineTag guards against literal tag text read back
// from Jira being dropped or reinterpreted on the next write.
func TestRoundTrip_LiteralInlineTag(t *testing.T) {
	t.Parallel()

	for _, text := range []string{
		"pass <word> as the argument",
		"stray </u> close",
		"x <u>a never closed",
	} {
		t.Run(text, func(t *testing.T) {
			t.Parallel()

			original := &jira4claude.ADFNode{
				Type:    "doc",
				Version: 1,
				Content: []jira4claude.ADFNode{{
					Type:    "paragraph",
					Content: []jira4claude.ADFNode{{Type: "text", Text: text}},
				}},
			}

			converter := markdown.New()
			md, readIssues := converter.ToMarkdown(original)
			assert.Empty(t, readIssues)

			written, writeIssues := converter.ToADF(md)
			assert.Empty(t, writeIssues)
			assert.Equal(t, original, written)
		})
	}
}
