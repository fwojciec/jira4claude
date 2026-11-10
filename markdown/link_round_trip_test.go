package markdown_test

import (
	"encoding/json"
	"testing"

	jira4claude "github.com/fwojciec/jira4claude"
	"github.com/fwojciec/jira4claude/markdown"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Links (#161). The rule for when a link is written bare, in brackets or
// reported is at the top of inline_marks.go.

// hrefLink builds a text node marked as a link to href, and with marks after
// it (in canonical order).
func hrefLink(text, href string, marks ...string) jira4claude.ADFNode {
	attrs, err := json.Marshal(map[string]any{"href": href})
	if err != nil {
		panic(err)
	}
	n := jira4claude.ADFNode{Type: "text", Text: text, Marks: []jira4claude.ADFMark{{Type: "link", Attrs: attrs}}}
	for _, m := range marks {
		n.Marks = append(n.Marks, jira4claude.ADFMark{Type: m})
	}
	return n
}

// A link destination is CommonMark: backslash escapes and character
// references in it are resolved on write, as in text.
func TestToADF_LinkDestinationIsDecoded(t *testing.T) {
	t.Parallel()

	tests := []struct {
		md   string
		href string
	}{
		{`[a](b\)c)`, "b)c"},
		{`[a](<b c>)`, "b c"},
		{`[a](a&amp;b)`, "a&b"},
		{`[a](<a\>b\<c>)`, "a>b<c"},
		{`[a](a\\b)`, `a\b`},
		{`[a](a\b)`, `a\b`},
		{`[a](a&#10;b)`, "a\nb"},
		{`[a](b%20c)`, "b%20c"},
		{"[a]()", ""},
		{"[a](<>)", ""},
		{"[a]\n\n[a]: b\\_c&amp;d", "b_c&d"},
	}
	for _, tt := range tests {
		t.Run(tt.md, func(t *testing.T) {
			t.Parallel()

			got, issues := markdown.New().ToADF(tt.md)
			assert.Empty(t, issues)
			assert.Equal(t, paragraphDoc(hrefLink("a", tt.href)), got)
		})
	}
}

// An autolink keeps its raw text and href, which CommonMark does not decode.
func TestToADF_AutolinkHrefIsRaw(t *testing.T) {
	t.Parallel()

	got, issues := markdown.New().ToADF(`<https://x.test/a\_b&amp;c>`)
	assert.Empty(t, issues)
	assert.Equal(t, paragraphDoc(hrefLink(`https://x.test/a\_b&amp;c`, `https://x.test/a\_b&amp;c`)), got)
}

// A link with no text has nothing to carry its href: it is reported.
func TestToADF_LinkWithoutTextIsReported(t *testing.T) {
	t.Parallel()

	for _, md := range []string{"[]()", "a [](https://x.test) b"} {
		t.Run(md, func(t *testing.T) {
			t.Parallel()

			_, issues := markdown.New().ToADF(md)
			assert.Contains(t, issues, droppedIssue("link"))
		})
	}
}

// Every href, written as a link destination, reads back as itself.
func TestRoundTrip_LinkDestinationExhaustive(t *testing.T) {
	t.Parallel()

	alphabet := []string{`\`, "<", ">", "(", ")", "&", ";", "#", "a", " ", "\t", "\n", "*", "[", "]", "`"}
	hrefs := make([]string, 0, 5000)
	hrefs = append(hrefs, "")
	for _, a := range alphabet {
		hrefs = append(hrefs, a)
		for _, b := range alphabet {
			hrefs = append(hrefs, a+b)
			for _, c := range alphabet {
				hrefs = append(hrefs, a+b+c)
			}
		}
	}
	hrefs = append(hrefs,
		"&amp;", "a&amp;b", "&#35;", "&#x41;", "&copy;", `a\&amp;`, "https://x.test/?a=1&amp;b=2",
		"https://x.test/a(b)c", "https://x.test/a(", "https://x.test/a)", "https://x.test/a b", `https://x.test/a\`,
	)

	conv := markdown.New()
	for _, href := range hrefs {
		// A link whose text is its href is written bare only where it reads
		// back as that link; otherwise in brackets, like any other.
		for _, text := range []string{"t", href} {
			if text == "" {
				continue
			}
			doc := paragraphDoc(txt("see "), hrefLink(text, href), txt(" after"))
			md, readIssues := conv.ToMarkdown(doc)
			back, writeIssues := conv.ToADF(md)
			if !assert.Empty(t, readIssues, "href %q text %q read as %q", href, text, md) ||
				!assert.Empty(t, writeIssues, "href %q text %q read as %q", href, text, md) ||
				!assert.Equal(t, doc, back, "href %q text %q read as %q", href, text, md) {
				return
			}
		}
	}
}

// An href holding a character Markdown cannot carry (a control character
// other than tab and line feed, or a carriage return) is
// written as the write side would read it, and reported.
func TestToMarkdown_LinkDestinationWithoutMarkdownForm(t *testing.T) {
	t.Parallel()

	// Invalid UTF-8 cannot reach an href: JSON gives it as U+FFFD.
	for _, href := range []string{"a\x01b", "a\rb", "a\x7fb", "a\u0085b"} {
		t.Run(href, func(t *testing.T) {
			t.Parallel()

			md, issues := markdown.New().ToMarkdown(paragraphDoc(hrefLink("t", href)))
			if assert.Len(t, issues, 1, "read as %q", md) {
				assert.Equal(t, jira4claude.ConversionDegraded, issues[0].Kind)
				assert.Equal(t, "link", issues[0].Node)
			}
			back, writeIssues := markdown.New().ToADF(md)
			assert.Empty(t, writeIssues)
			require.Len(t, back.Content, 1)
			require.Len(t, back.Content[0].Content, 1)
			assert.Equal(t, "t", back.Content[0].Content[0].Text)
		})
	}
}

// Ordinary links read exactly as before #161.
func TestToMarkdown_OrdinaryLinkForms(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		inline []jira4claude.ADFNode
		want   string
	}{
		{"link", []jira4claude.ADFNode{hrefLink("text", "https://example.com")}, "[text](https://example.com)"},
		{"link in a sentence", []jira4claude.ADFNode{txt("Visit "), hrefLink("Google", "https://google.com"), txt(" for more.")},
			"Visit [Google](https://google.com) for more."},
		{"balanced parentheses", []jira4claude.ADFNode{hrefLink("Foo", "https://en.wikipedia.org/wiki/Foo_(bar)")},
			"[Foo](https://en.wikipedia.org/wiki/Foo_(bar))"},
		{"query", []jira4claude.ADFNode{hrefLink("q", "https://x.test/?a=1&b=2")}, "[q](https://x.test/?a=1&b=2)"},
		{"bare URL in a sentence", []jira4claude.ADFNode{
			txt("see "), hrefLink("https://example.com/a_b", "https://example.com/a_b"), txt(" now"),
		}, "see https://example.com/a_b now"},
		{"bare URL before a full stop", []jira4claude.ADFNode{
			txt("Read "), hrefLink("https://example.com/docs", "https://example.com/docs"), txt("."),
		}, "Read https://example.com/docs."},
		{"bare URL in parentheses", []jira4claude.ADFNode{
			txt("(see "), hrefLink("https://example.com", "https://example.com"), txt(")"),
		}, "(see https://example.com)"},
		{"bare email", []jira4claude.ADFNode{txt("mail "), hrefLink("a@b.co", "a@b.co"), txt(" now")}, "mail a@b.co now"},
		{"empty href", []jira4claude.ADFNode{hrefLink("text", "")}, "[text](<>)"},
		{"href with a blank", []jira4claude.ADFNode{hrefLink("text", "a b")}, "[text](<a b>)"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			md, issues := markdown.New().ToMarkdown(paragraphDoc(tt.inline...))
			assert.Empty(t, issues)
			assert.Equal(t, tt.want, md)
			back, writeIssues := markdown.New().ToADF(md)
			assert.Empty(t, writeIssues)
			assert.Equal(t, paragraphDoc(tt.inline...), back)
		})
	}
}

// A link whose text is its href is written bare only where Linkify reads it
// back as exactly that link: the same text and href, starting and ending
// where the node does. Anywhere else it is written in brackets (#161).
func TestRoundTrip_LinkWhoseTextIsItsHref(t *testing.T) {
	t.Parallel()

	self := func(href string) jira4claude.ADFNode { return hrefLink(href, href) }
	tests := []struct {
		name   string
		inline []jira4claude.ADFNode
		bare   bool // the link is written as its bare URL
	}{
		{"URL in a sentence", []jira4claude.ADFNode{txt("see "), self("https://x.test/a"), txt(" now")}, true},
		{"email in a sentence", []jira4claude.ADFNode{txt("mail "), self("a@b.co"), txt(" now")}, true},
		{"non-URL href", []jira4claude.ADFNode{txt("a "), self("0")}, false},
		{"scheme only", []jira4claude.ADFNode{self("A0:")}, false},
		{"email without a dot", []jira4claude.ADFNode{self("0@0")}, false},
		{"www without a scheme", []jira4claude.ADFNode{self("www.x.test")}, false},
		{"after an escaped delimiter", []jira4claude.ADFNode{txt("*"), self("https://0.a")}, false},
		{"after a literal tag", []jira4claude.ADFNode{txt("<A0>"), self("https://0.a")}, false},
		{"letter before", []jira4claude.ADFNode{txt("a"), self("https://x.test")}, false},
		{"letter after", []jira4claude.ADFNode{self("https://x.test"), txt("a")}, false},
		{"trailing asterisk", []jira4claude.ADFNode{self("https://x.test/*")}, false},
		{"trailing underscore", []jira4claude.ADFNode{self("https://x.test/_")}, false},
		{"trailing tilde", []jira4claude.ADFNode{self("https://x.test/~")}, false},
		{"trailing full stop", []jira4claude.ADFNode{self("https://x.test/.")}, false},
		{"unbalanced parenthesis", []jira4claude.ADFNode{self("https://x.test/a)")}, false},
		{"balanced parentheses", []jira4claude.ADFNode{txt("see "), self("https://x.test/a(b)"), txt(" now")}, true},
		{"email before code", []jira4claude.ADFNode{self("a@b.co"), textNode("x", "code")}, false},
		{"URL before code", []jira4claude.ADFNode{self("https://x.co"), textNode("x", "code")}, false},
		{"URL after code", []jira4claude.ADFNode{textNode("x", "code"), self("https://x.co")}, true},
		{"URL holding a backtick before code", []jira4claude.ADFNode{
			txt("see "), self("https://x.co/a`b"), txt(" and "), textNode("c", "code"),
		}, true},
		{"after a blank that follows a backslash", []jira4claude.ADFNode{txt(`\ `), self("http://x.co/a"), txt("<a")}, false},
		{"exclamation mark before", []jira4claude.ADFNode{txt("a!"), self("https://x.test")}, false},
		{"two URLs after each other", []jira4claude.ADFNode{self("https://x.test"), self("https://y.test")}, false},
		{"after strong text", []jira4claude.ADFNode{textNode("a", "strong"), self("https://x.test")}, true},
		{"before strong text", []jira4claude.ADFNode{self("https://x.test"), textNode("a", "strong")}, false},
		{"after a hard break", []jira4claude.ADFNode{txt("a"), hardBreak(), self("https://x.test")}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			doc := paragraphDoc(tt.inline...)
			roundTripsExactly(t, doc)
			md, _ := markdown.New().ToMarkdown(doc)
			if tt.bare {
				assert.NotContains(t, md, "](", "read as %q", md)
			}
		})
	}
}

// Many short links and neighbours, combined: each link reads back exactly,
// whichever form it is written in.
func TestRoundTrip_LinkNeighbours(t *testing.T) {
	t.Parallel()

	befores := []jira4claude.ADFNode{
		txt("a"), txt(" "), txt("*"), txt("("), txt(`\ `), txt("<A0>"), txt("x."), txt("!"), txt("~"), txt("_"),
		textNode("c", "code"), textNode("s", "strong"), textNode("e", "em"), textNode("u", "underline"),
		hrefLink("l", "https://l.test"),
	}
	links := []jira4claude.ADFNode{
		hrefLink("https://x.test", "https://x.test"), hrefLink("a@b.co", "a@b.co"), hrefLink("www.x.test", "www.x.test"),
		hrefLink("http://www.x.test", "http://www.x.test"), hrefLink("https://x.test/*", "https://x.test/*"),
		hrefLink("t", "https://x.test"), hrefLink("x", "x"),
	}
	afters := []jira4claude.ADFNode{
		txt("a"), txt(" "), txt("."), txt(")"), txt("*"), txt("\n"), txt("<"), txt("_a"), txt("&amp;"), txt(`\`),
		textNode("c", "code"), textNode("s", "strong"), textNode("e", "em"), hrefLink("l", "https://l.test"),
	}
	conv := markdown.New()
	for _, before := range befores {
		for _, link := range links {
			for _, after := range afters {
				for _, inline := range [][]jira4claude.ADFNode{
					{before, link}, {link, after}, {before, link, after},
				} {
					doc := paragraphDoc(inline...)
					md, readIssues := conv.ToMarkdown(doc)
					back, writeIssues := conv.ToADF(md)
					if !assert.Empty(t, readIssues, "read as %q", md) ||
						!assert.Empty(t, writeIssues, "read as %q", md) ||
						!assert.Equal(t, mustJSON(doc), mustJSON(back), "read as %q", md) {
						return
					}
				}
			}
		}
	}
}

// A link whose text is one character a task box takes (" ", "x", "X"), at
// the start of a list item, would read back as a task box: "- [X](a)". Its
// text is written as a character reference instead.
func TestRoundTrip_LinkThatLooksLikeATaskBox(t *testing.T) {
	t.Parallel()

	for _, text := range []string{" ", "\t", "x", "X"} {
		card, err := json.Marshal(map[string]any{"url": text})
		require.NoError(t, err)
		for _, inline := range [][]jira4claude.ADFNode{
			{hrefLink(text, "a"), txt("b")},
			{hrefLink(text, ""), hrefLink("0", "", "em")},
			{hrefLink(text, text)},
			{{Type: "inlineCard", Attrs: card}, txt(" b")},
		} {
			t.Run(text, func(t *testing.T) {
				t.Parallel()

				doc := &jira4claude.ADFNode{Type: "doc", Version: 1, Content: []jira4claude.ADFNode{{
					Type: "bulletList",
					Content: []jira4claude.ADFNode{{
						Type:    "listItem",
						Content: []jira4claude.ADFNode{{Type: "paragraph", Content: inline}},
					}},
				}}}
				md, readIssues := markdown.New().ToMarkdown(doc)
				back, writeIssues := markdown.New().ToADF(md)
				assert.Empty(t, readIssues, "read as %q", md)
				assert.Empty(t, writeIssues, "read as %q", md)
				require.Len(t, back.Content, 1, "read as %q", md)
				assert.Equal(t, "bulletList", back.Content[0].Type, "read as %q", md)
				if inline[0].Type != "inlineCard" {
					assert.Equal(t, mustJSON(doc), mustJSON(back), "read as %q", md)
				}
			})
		}
	}
}

// An ordinary bare URL stays bare next to other inline nodes and inside
// containers: a line is parsed back whole, and a difference anywhere on it
// would bracket every bare link on it.
func TestToMarkdown_BareLinkInContext(t *testing.T) {
	t.Parallel()

	url := hrefLink("https://x.test/a", "https://x.test/a")
	para := func(inline ...jira4claude.ADFNode) jira4claude.ADFNode {
		return jira4claude.ADFNode{Type: "paragraph", Content: inline}
	}
	sentence := para(txt("see "), url, txt(" now"))
	mention := jira4claude.ADFNode{Type: "mention", Attrs: json.RawMessage(`{"id":"1","text":"@Ann *x*"}`)}
	emoji := jira4claude.ADFNode{Type: "emoji", Attrs: json.RawMessage(`{"shortName":":smile:","text":"😄"}`)}
	date := jira4claude.ADFNode{Type: "date", Attrs: json.RawMessage(`{"timestamp":"0"}`)}
	status := jira4claude.ADFNode{Type: "status", Attrs: json.RawMessage(`{"text":"DONE","color":"green"}`)}
	cell := func(typ string, content ...jira4claude.ADFNode) jira4claude.ADFNode {
		return jira4claude.ADFNode{Type: typ, Content: content}
	}

	tests := map[string]jira4claude.ADFNode{
		"after a mention":         para(mention, txt(" see "), url, txt(" now")),
		"before a mention":        para(txt("see "), url, txt(" "), mention),
		"next to an emoji":        para(emoji, txt(" "), url, txt(" "), emoji),
		"next to a date":          para(date, txt(" "), url),
		"next to a status":        para(status, txt(" "), url),
		"after a mid-line break":  para(txt("a"), hardBreak(), txt("see "), url, txt(" now")),
		"before a mid-line break": para(txt("see "), url, hardBreak(), txt("next")),
		"in a list item":          cell("bulletList", cell("listItem", sentence)),
		"in a blockquote":         cell("blockquote", sentence),
		"in a panel":              {Type: "panel", Attrs: json.RawMessage(`{"panelType":"info"}`), Content: []jira4claude.ADFNode{sentence}},
		"in a table cell": cell("table",
			cell("tableRow", cell("tableHeader", para(txt("h")))),
			cell("tableRow", cell("tableCell", sentence))),
		"in a heading": {Type: "heading", Attrs: json.RawMessage(`{"level":2}`), Content: sentence.Content},
	}
	for name, block := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			doc := &jira4claude.ADFNode{Type: "doc", Version: 1, Content: []jira4claude.ADFNode{block}}
			md, _ := markdown.New().ToMarkdown(doc)
			assert.Contains(t, md, "https://x.test/a", "read as %q", md)
			assert.NotContains(t, md, "](https://x.test/a)", "read as %q", md)
		})
	}
}

// An email in text that is emphasised with "_" (forced by a neighbour that
// ends in "*") stays text, and a linked one stays a link (#161, cause 9a).
func TestRoundTrip_EmailInUnderscoreEmphasis(t *testing.T) {
	t.Parallel()

	for name, inline := range map[string][]jira4claude.ADFNode{
		"plain":           {txt("_a@b.co x_")},
		"em after strong": {textNode("s", "strong"), textNode("a@b.co x", "em")},
		"linked em after strong": {
			textNode("s", "strong"), hrefLink("a@b.co", "a@b.co", "em"), textNode(" x", "em"),
		},
		"plain email after em": {textNode("e", "em"), txt("_a@b.co x_")},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			roundTripsExactly(t, paragraphDoc(inline...))
		})
	}
}

// roundTripsExactly asserts that doc reads and writes back unchanged, with
// no issue on either side.
func roundTripsExactly(t *testing.T, doc *jira4claude.ADFNode) {
	t.Helper()

	md, readIssues := markdown.New().ToMarkdown(doc)
	back, writeIssues := markdown.New().ToADF(md)
	assert.Empty(t, readIssues, "read as %q", md)
	assert.Empty(t, writeIssues, "read as %q", md)
	assert.Equal(t, doc, back, "read as %q", md)
}

// Text that is not a link stays text: where Linkify would read a URL or an
// email address in it, the read keeps it from forming a link (the break mode
// in escape.go), so a link appears only where ADF has one (#161).
func TestRoundTrip_TextThatLooksLikeALinkStaysText(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		inline []jira4claude.ADFNode
	}{
		{"URL in a sentence", []jira4claude.ADFNode{txt("see https://x.test/a_b now")}},
		{"URL at a line start", []jira4claude.ADFNode{txt("http://0.a0")}},
		{"www URL", []jira4claude.ADFNode{txt("see www.x.test now")}},
		{"email", []jira4claude.ADFNode{txt("mail a@b.co now")}},
		{"email with punctuation", []jira4claude.ADFNode{txt("0!@0.0")}},
		// Linkify takes no email right before "_", but the "_" is written
		// escaped, and "\" does not stop it.
		{"email before an underscore", []jira4claude.ADFNode{txt("0@0.0_")}},
		{"email before an underscore in a sentence", []jira4claude.ADFNode{txt("mail a@b.co_ now")}},
		{"URL after a parenthesis", []jira4claude.ADFNode{txt("(https://0.a")}},
		{"URL after an unmatched bracket", []jira4claude.ADFNode{txt("[ http://x.com")}},
		{"URL after a hard break", []jira4claude.ADFNode{txt("a"), hardBreak(), txt("https://x.test")}},
		{"strong URL", []jira4claude.ADFNode{textNode("https://x.test/a", "strong")}},
		{"em URL after text", []jira4claude.ADFNode{txt("see "), textNode("https://x.test/a now", "em")}},
		{"struck email", []jira4claude.ADFNode{textNode("a@b.co", "strike")}},
		{"underlined URL", []jira4claude.ADFNode{textNode("https://x.test", "underline")}},
		{"URL after a link", []jira4claude.ADFNode{hrefLink("a", "https://y.test"), txt("https://x.test")}},
		{"URL after code", []jira4claude.ADFNode{textNode("c", "code"), txt("https://x.test")}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			roundTripsExactly(t, paragraphDoc(tt.inline...))
		})
	}
}
