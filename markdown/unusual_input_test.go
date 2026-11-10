package markdown_test

import (
	"encoding/json"
	"testing"

	jira4claude "github.com/fwojciec/jira4claude"
	"github.com/fwojciec/jira4claude/markdown"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Line endings, control characters and invalid UTF-8 (#162).
//
// Write side: a carriage return, alone or before a line feed, is a line
// ending and becomes a line feed before parsing (CommonMark 2.1). Any other
// control character except tab becomes a space, and invalid UTF-8 becomes
// U+FFFD. The same rule applies to a character that arrives through a numeric
// character reference, so "&#13;" gives a line feed in the text and "&#1;" a
// space. A line feed from a reference is text, not a line ending.
//
// Read side: a line feed in plain text is written as "&#10;", which reads back
// as the same text. A carriage return, another control character or invalid
// UTF-8 in Jira text has no form that reads back the same, so it is written as
// what the write side would make of it (a line feed, a space, U+FFFD) and
// reported as degraded. Code text has no escapes, so there a line feed is
// written as a space, as in a code span, and a code block keeps it.

func TestToADF_LineEndings(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		markdown string
		same     string
	}{
		{"lone carriage return", "a\rb", "a\nb"},
		{"carriage return before a heading", "\r#\r0", "\n#\n0"},
		{"CRLF", "a\r\nb", "a\nb"},
		{"carriage return in a fenced code block", "```\na\rb\n```", "```\na\nb\n```"},
		{"carriage return in a code span", "`a\rb`", "`a\nb`"},
		{"carriage return in a list", "- a\r- b", "- a\n- b"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			conv := markdown.New()
			got, gotIssues := conv.ToADF(tt.markdown)
			want, wantIssues := conv.ToADF(tt.same)

			assert.Equal(t, want, got)
			assert.Equal(t, wantIssues, gotIssues)
		})
	}
}

func TestToADF_CharacterReferencesFollowTheRawRule(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		markdown string
		text     string
	}{
		{"line feed", "a&#10;b", "a\nb"},
		{"carriage return", "a&#13;b", "a\nb"},
		{"hex carriage return", "a&#Xd;b", "a\nb"},
		{"CRLF", "a&#13;&#10;b", "a\nb"},
		{"C0 control", "a&#1;b", "a b"},
		{"C1 control", "a&#x85;b", "a b"},
		{"delete", "a&#127;b", "a b"},
		{"tab is kept", "a&#9;b", "a\tb"},
		{"NUL", "a&#0;b", "a�b"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			adf, issues := markdown.New().ToADF(tt.markdown)

			assert.Empty(t, issues)
			assert.Equal(t, paragraphDoc(jira4claude.ADFNode{Type: "text", Text: tt.text}), adf)
		})
	}
}

func TestToADF_InvalidUTF8(t *testing.T) {
	t.Parallel()

	t.Run("becomes U+FFFD", func(t *testing.T) {
		t.Parallel()

		adf, issues := markdown.New().ToADF("a\x81b")

		assert.Empty(t, issues)
		assert.Equal(t, paragraphDoc(jira4claude.ADFNode{Type: "text", Text: "a�b"}), adf)
	})

	t.Run("is replaced before parsing", func(t *testing.T) {
		t.Parallel()

		conv := markdown.New()
		got, _ := conv.ToADF("*0 []()\x81*")
		want, _ := conv.ToADF("*0 []()�*")

		assert.Equal(t, want, got)
	})
}

func TestNormalizeUnicode_ReplacesInvalidUTF8(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "a�b", markdown.NormalizeUnicode("a\x81b"))
	assert.Equal(t, "a��b", markdown.NormalizeUnicode("a\xc3\x28b"[:2]+"\xffb"))
}

// A line feed in Jira text is written as a character reference, which reads
// back as the same text.
func TestRoundTrip_LineFeedInText(t *testing.T) {
	t.Parallel()

	strong := []jira4claude.ADFMark{{Type: "strong"}}
	tests := []struct {
		name   string
		inline []jira4claude.ADFNode
		want   string
	}{
		{"between words", []jira4claude.ADFNode{txt("a\nb")}, "a&#10;b"},
		{"before a block marker", []jira4claude.ADFNode{txt("a\n# b")}, "a&#10;# b"},
		{"at the end", []jira4claude.ADFNode{txt("a\n")}, "a&#10;"},
		{"in strong text", []jira4claude.ADFNode{{Type: "text", Text: "a\nb", Marks: strong}}, "**a&#10;b**"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			conv := markdown.New()
			original := paragraphDoc(tt.inline...)
			md, readIssues := conv.ToMarkdown(original)
			back, writeIssues := conv.ToADF(md)

			assert.Equal(t, tt.want, md)
			assert.Empty(t, readIssues)
			assert.Empty(t, writeIssues)
			assert.Equal(t, original, back)
		})
	}
}

// Text from Jira that the write side would change is written as what it reads
// back as, and reported.
func TestToMarkdown_TextWithoutMarkdownForm(t *testing.T) {
	t.Parallel()

	codeMark := []jira4claude.ADFMark{{Type: "code"}}
	tests := []struct {
		name string
		doc  *jira4claude.ADFNode
		want string
	}{
		{"carriage return", paragraphDoc(txt("a\rb")), "a&#10;b"},
		{"CRLF", paragraphDoc(txt("a\r\nb")), "a&#10;b"},
		{"control character", paragraphDoc(txt("a\x01b")), "a b"},
		{"C1 control character", paragraphDoc(txt("a\u0085b")), "a b"},
		{"invalid UTF-8", paragraphDoc(txt("a\x81b")), "a�b"},
		{"carriage return in inline code", paragraphDoc(jira4claude.ADFNode{Type: "text", Text: "a\rb", Marks: codeMark}), "`a b`"},
		{"control character in inline code", paragraphDoc(jira4claude.ADFNode{Type: "text", Text: "a\x01b", Marks: codeMark}), "`a b`"},
		{"invalid UTF-8 in inline code", paragraphDoc(jira4claude.ADFNode{Type: "text", Text: "a\x81b", Marks: codeMark}), "`a�b`"},
		{"CRLF in a code block", codeBlockDoc("", "a\r\nb"), "```\na\nb\n```"},
		{"lone carriage return in a code block", codeBlockDoc("", "a\rb"), "```\na\nb\n```"},
		{"control character in a code block", codeBlockDoc("", "a\x01b"), "```\na b\n```"},
		{"invalid UTF-8 in a code block", codeBlockDoc("", "a\x81b"), "```\na�b\n```"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			conv := markdown.New()
			md, issues := conv.ToMarkdown(tt.doc)

			assert.Equal(t, tt.want, md)
			if assert.Len(t, issues, 1) {
				assert.Equal(t, jira4claude.ConversionDegraded, issues[0].Kind)
			}

			// What is written reads back as what the issue describes, and
			// that reads back unchanged.
			back, _ := conv.ToADF(md)
			again, againIssues := conv.ToMarkdown(back)
			assert.Equal(t, md, again)
			assert.Empty(t, againIssues)
		})
	}
}

// A backslash right before a blank that is written as a character reference
// must not escape the reference.
func TestRoundTrip_BackslashBeforeEncodedBlank(t *testing.T) {
	t.Parallel()

	br := jira4claude.ADFNode{Type: "hardBreak"}
	tests := []struct {
		name   string
		inline []jira4claude.ADFNode
	}{
		{"trailing space after a hard break", []jira4claude.ADFNode{txt("<A>"), br, txt(`\ `)}},
		{"trailing space", []jira4claude.ADFNode{txt(`a\ `)}},
		{"trailing tab", []jira4claude.ADFNode{txt("a\\\t")}},
		{"trailing spaces", []jira4claude.ADFNode{txt(`a\  `)}},
		{"two backslashes", []jira4claude.ADFNode{txt(`a\\ `)}},
		{"leading space after a hard break", []jira4claude.ADFNode{txt(`a\`), br, txt(" b")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			conv := markdown.New()
			original := paragraphDoc(tt.inline...)
			md, readIssues := conv.ToMarkdown(original)
			back, writeIssues := conv.ToADF(md)

			assert.Empty(t, readIssues)
			assert.Empty(t, writeIssues)
			assert.Equal(t, original, back, "read as %q", md)
		})
	}
}

// A line that would be a table delimiter row, after a hard break, would turn
// the line before it into a table header.
func TestRoundTrip_DelimiterRowAfterHardBreak(t *testing.T) {
	t.Parallel()

	br := jira4claude.ADFNode{Type: "hardBreak"}
	for _, row := range []string{"|-", "-|-", "|-|", ":-", "-:", ":-:", "---|---", "| --- | :-: |", "-|"} {
		for _, before := range []string{"", "a", "a | b", `\`} {
			inline := []jira4claude.ADFNode{br, txt(row)}
			if before != "" {
				inline = append([]jira4claude.ADFNode{txt(before)}, inline...)
			}
			original := paragraphDoc(inline...)

			conv := markdown.New()
			md, readIssues := conv.ToMarkdown(original)
			back, writeIssues := conv.ToADF(md)

			assert.Empty(t, readIssues, "%q then %q", before, row)
			assert.Empty(t, writeIssues, "%q then %q", before, row)
			assert.Equal(t, original, back, "%q then %q read as %q", before, row, md)
		}
	}
}

// In a table cell, where the escape for a delimiter row meets the cell's own
// escaping of "|", the text still reads back unchanged.
func TestRoundTrip_DelimiterRowShapeInTableCell(t *testing.T) {
	t.Parallel()

	for _, text := range []string{"|-", "-|-", ":-", "| --- |"} {
		cell := func(typ string) jira4claude.ADFNode {
			return jira4claude.ADFNode{Type: typ, Content: []jira4claude.ADFNode{{
				Type: "paragraph", Content: []jira4claude.ADFNode{txt(text)},
			}}}
		}
		original := &jira4claude.ADFNode{Type: "doc", Version: 1, Content: []jira4claude.ADFNode{{
			Type: "table",
			Content: []jira4claude.ADFNode{
				{Type: "tableRow", Content: []jira4claude.ADFNode{cell("tableHeader")}},
				{Type: "tableRow", Content: []jira4claude.ADFNode{cell("tableCell")}},
			},
		}}}

		conv := markdown.New()
		md, readIssues := conv.ToMarkdown(original)
		back, writeIssues := conv.ToADF(md)

		assert.Empty(t, readIssues, "%q", text)
		assert.Empty(t, writeIssues, "%q", text)
		assert.Equal(t, original, back, "%q read as %q", text, md)
	}
}

// A link whose text is its URL is written bare only where Linkify reads it
// back whole. Linkify takes "&", "#" and ";" into a URL, so when a character
// written as a reference follows (a line feed, trailing blanks, or a
// backslash before a hard break), the link is written in brackets (#161).
// Before #161 these inputs were plain text, which gained a link on write,
// closed in angle brackets; plain text now stays plain
// (TestRoundTrip_TextThatLooksLikeALinkStaysText), so the links are the input.
func TestRoundTrip_BareURLBeforeCharacterReference(t *testing.T) {
	t.Parallel()

	link := func(url string) jira4claude.ADFNode {
		attrs, err := json.Marshal(map[string]any{"href": url})
		require.NoError(t, err)
		return jira4claude.ADFNode{Type: "text", Text: url, Marks: []jira4claude.ADFMark{{Type: "link", Attrs: attrs}}}
	}

	br := jira4claude.ADFNode{Type: "hardBreak"}
	tests := []struct {
		name   string
		inline []jira4claude.ADFNode
	}{
		{"line feed after a path", []jira4claude.ADFNode{txt("see "), link("https://x.com/a"), txt("\nb")}},
		{"line feed after a query", []jira4claude.ADFNode{link("https://x.com/a?q=1&r=[2]"), txt("\nb")}},
		{"trailing space", []jira4claude.ADFNode{txt("see "), link("https://x.com/a"), txt(" ")}},
		{"full stop then a line feed", []jira4claude.ADFNode{txt("("), link("https://x.com/p?q=1"), txt(".\n")}},
		{"backslash before a hard break", []jira4claude.ADFNode{link("https://x.com/a"), txt(`\`), br, txt("b")}},
		{"after a single tilde", []jira4claude.ADFNode{txt("~"), link("https://x.com/a"), txt("\n")}},
		{"email before a line feed", []jira4claude.ADFNode{link("a@b.co"), txt("\nx")}},
		{"email before a trailing space", []jira4claude.ADFNode{link("a@b.co"), txt(" ")}},
		{"email before a hard break", []jira4claude.ADFNode{link("a@b.co"), txt(`\`), br, txt("b")}},
		// Linkify gives a www. URL an http:// scheme in its text and href.
		{"www URL before a line feed", []jira4claude.ADFNode{link("http://www.example.com/path"), txt("\nnext")}},
		{"www URL before a trailing space", []jira4claude.ADFNode{txt("see "), link("http://www.x.com/a"), txt(" ")}},
		{"www URL before a hard break", []jira4claude.ADFNode{link("http://www.x.com/a"), txt(`\`), br, txt("b")}},
		{"www URL after punctuation before a line feed", []jira4claude.ADFNode{txt("("), link("http://www.x.com/a"), txt(".\n")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			conv := markdown.New()
			md, readIssues := conv.ToMarkdown(paragraphDoc(tt.inline...))
			back, writeIssues := conv.ToADF(md)

			assert.Empty(t, readIssues, "read as %q", md)
			assert.Empty(t, writeIssues)
			assert.Equal(t, paragraphDoc(tt.inline...), back, "read as %q", md)
		})
	}
}

// The character reference may come from the next node: Linkify reads the
// emitted line, not the nodes. A URL that ends one node, bare link text or
// plain text, is closed when Linkify would read on into the next node's
// Markdown: a character reference, or a link.
func TestRoundTrip_URLBeforeCharacterReferenceInNextNode(t *testing.T) {
	t.Parallel()

	link := func(text, href string) jira4claude.ADFNode {
		attrs, err := json.Marshal(map[string]any{"href": href})
		require.NoError(t, err)
		return jira4claude.ADFNode{Type: "text", Text: text, Marks: []jira4claude.ADFMark{{Type: "link", Attrs: attrs}}}
	}

	br := jira4claude.ADFNode{Type: "hardBreak"}
	tests := []struct {
		name     string
		inline   []jira4claude.ADFNode
		expected []jira4claude.ADFNode // nil: the inline nodes themselves
	}{
		{"bare link before a line feed", []jira4claude.ADFNode{link("https://x.com/a", "https://x.com/a"), txt("\nnext")}, nil},
		{"bare link after text before a line feed", []jira4claude.ADFNode{txt("see "), link("https://x.com/a", "https://x.com/a"), txt("\nnext")}, nil},
		{"bare link before trailing blanks", []jira4claude.ADFNode{txt("see "), link("https://x.com/a", "https://x.com/a"), txt(" \t")}, nil},
		{"bare link before a backslash and a hard break", []jira4claude.ADFNode{link("https://x.com/a", "https://x.com/a"), txt(`\`), br, txt("b")}, nil},
		{"bare link ending in punctuation before a line feed", []jira4claude.ADFNode{link("https://x.com/a", "https://x.com/a"), txt(".\nnext")}, nil},
		{"link label that is a URL before a line feed", []jira4claude.ADFNode{link("https://x.com/a", "https://example.com"), txt("\nnext")}, nil},
		{"link label ending in a line feed", []jira4claude.ADFNode{link("https://x.com/a\n", "https://example.com"), txt("next")}, nil},
		{"bare link before a link", []jira4claude.ADFNode{link("https://x.com/a", "https://x.com/a"), link("b", "https://example.com")}, nil},
		// Plain text that Linkify would read on into the next node is kept
		// from being linked at all (inline_marks.go), so it stays plain.
		{"plain URL before a link", []jira4claude.ADFNode{txt("see https://x.com/a"), link("b", "https://example.com")}, nil},
		{"www URL and punctuation before a link", []jira4claude.ADFNode{txt("www.x.com/a;"), link(";", "https://example.com")}, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			conv := markdown.New()
			md, readIssues := conv.ToMarkdown(paragraphDoc(tt.inline...))
			back, writeIssues := conv.ToADF(md)

			assert.Empty(t, readIssues, "read as %q", md)
			assert.Empty(t, writeIssues)
			expected := tt.expected
			if expected == nil {
				expected = tt.inline
			}
			assert.Equal(t, paragraphDoc(expected...), back, "read as %q", md)
		})
	}
}

// Inside an explicit link's label Linkify does not run, so a URL there next
// to a character reference reads back unchanged and is not reported.
func TestRoundTrip_URLBeforeCharacterReferenceInLinkLabel(t *testing.T) {
	t.Parallel()

	attrs, err := json.Marshal(map[string]any{"href": "https://example.com"})
	require.NoError(t, err)
	doc := paragraphDoc(jira4claude.ADFNode{Type: "text", Text: "https://x.com/a\nb", Marks: []jira4claude.ADFMark{{Type: "link", Attrs: attrs}}})

	conv := markdown.New()
	md, readIssues := conv.ToMarkdown(doc)
	back, writeIssues := conv.ToADF(md)

	assert.Empty(t, readIssues, "read as %q", md)
	assert.Empty(t, writeIssues)
	assert.Equal(t, doc, back, "read as %q", md)
}

// A URL holding ">" cannot be closed with angle brackets. When a character
// reference follows it, Linkify would take the reference in, so the URL is
// kept from being linked: its first character is written as a reference and
// it reads back as the plain text it was. Bare link text without a scheme
// is written as a bracketed link instead.
func TestRoundTrip_URLThatCannotBeClosed(t *testing.T) {
	t.Parallel()

	link := func(url string) jira4claude.ADFNode {
		attrs, err := json.Marshal(map[string]any{"href": url})
		require.NoError(t, err)
		return jira4claude.ADFNode{Type: "text", Text: url, Marks: []jira4claude.ADFMark{{Type: "link", Attrs: attrs}}}
	}

	br := jira4claude.ADFNode{Type: "hardBreak"}
	for _, inline := range [][]jira4claude.ADFNode{
		{txt("see https://x.com/a>b\nnext")},
		{txt("see https://x.com/a>b ")},
		{txt(`https://x.com/a>b\`), br, txt("next")},
		{link("www.x.com/a"), txt("\nnext")},
	} {
		conv := markdown.New()
		md, readIssues := conv.ToMarkdown(paragraphDoc(inline...))
		back, writeIssues := conv.ToADF(md)

		assert.Empty(t, readIssues, "read as %q", md)
		assert.Empty(t, writeIssues, "read as %q", md)
		assert.Equal(t, paragraphDoc(inline...), back, "read as %q", md)
	}
}

// A link written as its bare URL whose href holds ">" cannot be closed in
// angle brackets, so before a reference Linkify would take in, it is written
// in brackets, with ">" escaped in the destination (#161).
func TestRoundTrip_BareLinkThatCannotBeClosed(t *testing.T) {
	t.Parallel()

	attrs, err := json.Marshal(map[string]any{"href": "https://x.com/a>b"})
	require.NoError(t, err)
	link := jira4claude.ADFNode{Type: "text", Text: "https://x.com/a>b", Marks: []jira4claude.ADFMark{{Type: "link", Attrs: attrs}}}
	doc := paragraphDoc(link, txt("\nnext"))

	md, readIssues := markdown.New().ToMarkdown(doc)
	back, writeIssues := markdown.New().ToADF(md)

	assert.Empty(t, readIssues, "read as %q", md)
	assert.Empty(t, writeIssues, "read as %q", md)
	assert.Equal(t, doc, back, "read as %q", md)
}

// goldmark's Linkify drops an escaping backslash right after an autolink, so
// "a@b.co\\[" would read back as "a@b.co[" and let the "[" open a link. A
// link written bare is followed by a backslash only where that reads back
// (#161). Before #161 the inputs were the plain text the links came from;
// plain text now stays plain, so the links are the input.
func TestRoundTrip_BackslashAfterAutolink(t *testing.T) {
	t.Parallel()

	link := func(text, href string) jira4claude.ADFNode {
		attrs, err := json.Marshal(map[string]any{"href": href})
		require.NoError(t, err)
		return jira4claude.ADFNode{Type: "text", Text: text, Marks: []jira4claude.ADFMark{{Type: "link", Attrs: attrs}}}
	}
	tests := []struct {
		name   string
		inline []jira4claude.ADFNode
	}{
		{"email before a link", []jira4claude.ADFNode{txt("see "), link("a@b.co", "a@b.co"), txt(`\`), link(";", "https://example.com")}},
		{"email before punctuation", []jira4claude.ADFNode{txt("see "), link("a@b.co", "a@b.co"), txt(`\! x`)}},
		{"URL before punctuation", []jira4claude.ADFNode{txt("see "), link("https://x.com/b", "https://x.com/b"), txt(`\. x`)}},
		{"www URL holding @ before a link", []jira4claude.ADFNode{
			txt("see "), link("http://www.x.com/a@b.co", "http://www.x.com/a@b.co"), txt(`\`), link(";", "https://example.com"),
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			conv := markdown.New()
			md, readIssues := conv.ToMarkdown(paragraphDoc(tt.inline...))
			back, writeIssues := conv.ToADF(md)

			assert.Empty(t, readIssues, "read as %q", md)
			assert.Empty(t, writeIssues)
			assert.Equal(t, paragraphDoc(tt.inline...), back, "read as %q", md)
		})
	}
}

// A line feed at the edge of marked text follows the edge rule for a blank
// (inline_marks.go): it is written as a reference inside the delimiters, and
// a letter outside that would stop the run opening or closing is a reference
// too. A carriage return or control character there becomes a line feed or a
// space first (reported), then follows the same rule. Plain text ending in
// an email address next to a delimiter is kept from being linked.
func TestRoundTrip_UnusualCharacterAtMarkEdge(t *testing.T) {
	t.Parallel()

	strong := func(text string) jira4claude.ADFNode {
		return jira4claude.ADFNode{Type: "text", Text: text, Marks: []jira4claude.ADFMark{{Type: "strong"}}}
	}
	tests := []struct {
		name     string
		inline   []jira4claude.ADFNode
		expected []jira4claude.ADFNode // nil: the inline nodes themselves, with no issue
	}{
		{"line feed opening strong after a letter", []jira4claude.ADFNode{txt("x"), strong("\na")}, nil},
		{"line feed closing strong before a letter", []jira4claude.ADFNode{strong("a\n"), txt("x")}, nil},
		{"strong line feed between letters", []jira4claude.ADFNode{txt("x"), strong("\n"), txt("y")}, nil},
		{"email before strong punctuation", []jira4claude.ADFNode{txt("a@b.co"), strong(";")}, nil},
		{"carriage return opening strong", []jira4claude.ADFNode{txt("x"), strong("\ra")}, []jira4claude.ADFNode{txt("x"), strong("\na")}},
		{"CRLF opening strong", []jira4claude.ADFNode{txt("x"), strong("\r\na")}, []jira4claude.ADFNode{txt("x"), strong("\na")}},
		{"carriage return closing strong", []jira4claude.ADFNode{strong("a\r"), txt("x")}, []jira4claude.ADFNode{strong("a\n"), txt("x")}},
		{"control character opening strong", []jira4claude.ADFNode{txt("x"), strong("\x01a")}, []jira4claude.ADFNode{txt("x"), strong(" a")}},
		{"control character closing strong", []jira4claude.ADFNode{strong("a\x01"), txt("x")}, []jira4claude.ADFNode{strong("a "), txt("x")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			conv := markdown.New()
			md, readIssues := conv.ToMarkdown(paragraphDoc(tt.inline...))
			back, writeIssues := conv.ToADF(md)

			expected := tt.expected
			if expected == nil {
				expected = tt.inline
				assert.Empty(t, readIssues, "read as %q", md)
			} else if assert.Len(t, readIssues, 1, "read as %q", md) {
				assert.Equal(t, jira4claude.ConversionDegraded, readIssues[0].Kind)
			}
			assert.Empty(t, writeIssues)
			assert.Equal(t, paragraphDoc(expected...), back, "read as %q", md)
		})
	}
}

// A protected node's token label comes from Jira attrs, which may hold a
// carriage return, line feed, control character or invalid UTF-8. The token
// is not text: it is neither changed nor reported by the text rule, and it
// re-attaches to the node.
func TestRoundTrip_TokenLabelWithUnusualCharacters(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"Jo\rDoe", "Jo\nDoe", "Jo\r\nDoe", "Jo\x01Doe", "Jo\u0085Doe", "Jo\x81Doe"} {
		attrs, err := json.Marshal(map[string]any{"id": "u1", "text": "@" + name, "accessLevel": ""})
		require.NoError(t, err)
		mentionNode := jira4claude.ADFNode{Type: "mention", Attrs: attrs}
		doc := paragraphDoc(txt("a "), mentionNode, txt(" b"))

		conv := markdown.New()
		md, readIssues := conv.ToMarkdown(doc)
		assert.Empty(t, readIssues, "label %q read as %q", name, md)
		assert.NotContains(t, md, "\r", "label %q", name)
		assert.NotContains(t, md, "&#10;", "label %q", name)

		back, _ := conv.ToADF(md)
		got, issues := conv.Reattach(doc, back)
		assert.Empty(t, issues, "label %q read as %q", name, md)
		assert.Equal(t, doc, got, "label %q read as %q", name, md)
	}
}
