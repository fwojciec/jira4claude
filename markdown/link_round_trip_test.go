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

// hrefLink builds a text node marked as a link to href.
func hrefLink(text, href string) jira4claude.ADFNode {
	attrs, err := json.Marshal(map[string]any{"href": href})
	if err != nil {
		panic(err)
	}
	return jira4claude.ADFNode{Type: "text", Text: text, Marks: []jira4claude.ADFMark{{Type: "link", Attrs: attrs}}}
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
		for _, text := range []string{"t"} {
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
