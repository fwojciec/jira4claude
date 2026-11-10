package markdown_test

import (
	"encoding/json"
	"testing"

	jira4claude "github.com/fwojciec/jira4claude"
	"github.com/fwojciec/jira4claude/markdown"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Code spans and code blocks round trip (#160).

// stringsOver returns every string over alphabet of length 1 to maxLen.
func stringsOver(alphabet string, maxLen int) []string {
	all := make([]string, 0, len(alphabet)*maxLen)
	level := []string{""}
	for range maxLen {
		next := make([]string, 0, len(alphabet)*len(level))
		for _, prefix := range level {
			for _, r := range alphabet {
				next = append(next, prefix+string(r))
			}
		}
		all = append(all, next...)
		level = next
	}
	return all
}

func codeText(text string) jira4claude.ADFNode {
	return jira4claude.ADFNode{Type: "text", Text: text, Marks: []jira4claude.ADFMark{{Type: "code"}}}
}

func codeBlockDoc(language, text string) *jira4claude.ADFNode {
	block := jira4claude.ADFNode{Type: "codeBlock"}
	if language != "" {
		attrs, _ := json.Marshal(map[string]any{"language": language})
		block.Attrs = attrs
	}
	if text != "" {
		block.Content = []jira4claude.ADFNode{{Type: "text", Text: text}}
	}
	return &jira4claude.ADFNode{Type: "doc", Version: 1, Content: []jira4claude.ADFNode{block}}
}

func TestToMarkdown_CodeSpanDelimiters(t *testing.T) {
	t.Parallel()

	tests := []struct {
		text string
		want string
	}{
		{"a", "`a`"},
		{"a`b", "``a`b``"},
		{"a``b`", "``` a``b` ```"},
		{"`", "`` ` ``"},
		{"``", "``` `` ```"},
		{"` a", "`` ` a ``"},
		{" a ", "`  a  `"},
		{" a", "` a`"},
		{"a ", "`a `"},
		{" ", "` `"},
		{"  ", "`  `"},
		{" \t ", "` \t `"},
	}

	for _, tt := range tests {
		t.Run(tt.text, func(t *testing.T) {
			t.Parallel()

			md, issues := markdown.New().ToMarkdown(paragraphDoc(codeText(tt.text)))

			assert.Empty(t, issues)
			assert.Equal(t, tt.want, md)
		})
	}
}

// TestRoundTrip_CodeSpanText reads every short code text from Jira and
// writes it back: the code text must come back unchanged, with no issue.
func TestRoundTrip_CodeSpanText(t *testing.T) {
	t.Parallel()

	for _, text := range stringsOver("`~ \ta", 4) {
		for _, around := range [][2]string{{"", ""}, {"x", "y"}, {"x ", " y"}} {
			doc := paragraphDoc(codeText(text))
			if around[0] != "" {
				doc = paragraphDoc(txt(around[0]), codeText(text), txt(around[1]))
			}
			conv := markdown.New()
			md, readIssues := conv.ToMarkdown(doc)
			back, writeIssues := conv.ToADF(md)

			assert.Empty(t, readIssues, "code %q", text)
			assert.Empty(t, writeIssues, "code %q", text)
			assert.Equal(t, doc, back, "code %q around %q read as %q", text, around, md)
		}
	}
}

// Two adjacent code texts with the same marks have no Markdown form as two
// spans (their backtick runs would meet), so they are written as one span;
// the text is unchanged.
func TestRoundTrip_AdjacentCodeTextsMerge(t *testing.T) {
	t.Parallel()

	conv := markdown.New()
	md, readIssues := conv.ToMarkdown(paragraphDoc(codeText("a "), codeText(" b")))
	back, writeIssues := conv.ToADF(md)

	assert.Empty(t, readIssues)
	assert.Empty(t, writeIssues)
	assert.Equal(t, "`a  b`", md)
	assert.Equal(t, paragraphDoc(codeText("a  b")), back)
}

// A line ending in code text from Jira cannot be written in a code span,
// where it reads as a space; it is written as a space and reported.
func TestToMarkdown_CodeTextWithLineEnding(t *testing.T) {
	t.Parallel()

	md, issues := markdown.New().ToMarkdown(paragraphDoc(codeText("a\nb")))

	assert.Equal(t, "`a b`", md)
	require.Len(t, issues, 1)
	assert.Equal(t, jira4claude.ConversionDegraded, issues[0].Kind)
	assert.Equal(t, "text", issues[0].Node)
}

func TestToADF_CodeSpanLineEndingIsSpace(t *testing.T) {
	t.Parallel()

	tests := []struct {
		md   string
		want *jira4claude.ADFNode
	}{
		{"`a\nb`", paragraphDoc(codeText("a b"))},
		{"`a\n b`", paragraphDoc(codeText("a b"))},
		{"`\n`", paragraphDoc(codeText(" "))},
		{"`` a\nb ``", paragraphDoc(codeText("a b"))},
	}

	for _, tt := range tests {
		t.Run(tt.md, func(t *testing.T) {
			t.Parallel()

			adf, issues := markdown.New().ToADF(tt.md)

			assert.Empty(t, issues)
			assert.Equal(t, tt.want, adf)
		})
	}
}

func TestToADF_CodeBlockLanguageIsFirstWord(t *testing.T) {
	t.Parallel()

	tests := []struct {
		md   string
		want string
	}{
		{"```go\n```", "go"},
		{"```go extra\n```", "go"},
		{"```go\textra\n```", "go"},
		{"~~~0\t 0", "0"},
		{"```  go  \n```", "go"},
	}

	for _, tt := range tests {
		t.Run(tt.md, func(t *testing.T) {
			t.Parallel()

			adf, issues := markdown.New().ToADF(tt.md)

			assert.Empty(t, issues)
			assert.Equal(t, codeBlockDoc(tt.want, ""), adf)
		})
	}
}

func TestToMarkdown_CodeBlockFences(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		language string
		text     string
		want     string
	}{
		{"plain", "go", "x", "```go\nx\n```"},
		{"empty", "", "", "```\n```"},
		{"empty with language", "go", "", "```go\n```"},
		{"ends with a blank line", "", "x\n", "```\nx\n\n```"},
		{"contains a fence", "", "```", "````\n```\n````"},
		{"contains an indented fence", "", "a\n  ````", "`````\na\n  ````\n`````"},
		{"backticks inside a line", "", "a ````", "```\na ````\n```"},
		{"language with a backtick", "a`b", "x", "~~~a`b\nx\n~~~"},
		{"language with a backtick and code with tildes", "`", "~~~~", "~~~~~`\n~~~~\n~~~~~"},
		{"language starting with a tilde", "~`", "x", "~~~ ~`\nx\n~~~"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			md, issues := markdown.New().ToMarkdown(codeBlockDoc(tt.language, tt.text))

			assert.Empty(t, issues)
			assert.Equal(t, tt.want, md)
		})
	}
}

// TestRoundTrip_CodeBlockText reads every short code block from Jira, at top
// level and in a list item, a blockquote and an expand, and writes it back: it
// must come back unchanged, with no issue. A panel takes no code block on
// write, so there the Markdown must be the blockquote's under the alert line.
func TestRoundTrip_CodeBlockText(t *testing.T) {
	t.Parallel()

	panelAttrs, err := json.Marshal(map[string]any{"panelType": "info"})
	require.NoError(t, err)
	wrap := func(container jira4claude.ADFNode) *jira4claude.ADFNode {
		return &jira4claude.ADFNode{Type: "doc", Version: 1, Content: []jira4claude.ADFNode{container}}
	}

	languages := []string{"", "go", "a`b", "~", "~`", "`~"}
	for _, language := range languages {
		for _, text := range append([]string{""}, stringsOver("`~ \na", 4)...) {
			doc := codeBlockDoc(language, text)
			block := doc.Content[0]
			inList := wrap(jira4claude.ADFNode{
				Type: "bulletList",
				Content: []jira4claude.ADFNode{{
					Type:    "listItem",
					Content: []jira4claude.ADFNode{{Type: "paragraph", Content: []jira4claude.ADFNode{txt("item")}}, block},
				}},
			})
			inQuote := wrap(jira4claude.ADFNode{Type: "blockquote", Content: []jira4claude.ADFNode{block}})
			inExpand := wrap(expandNode(t, "T", block))
			for _, d := range []*jira4claude.ADFNode{doc, inList, inQuote, inExpand} {
				conv := markdown.New()
				md, readIssues := conv.ToMarkdown(d)
				back, writeIssues := conv.ToADF(md)

				assert.Empty(t, readIssues, "language %q code %q", language, text)
				assert.Empty(t, writeIssues, "language %q code %q", language, text)
				assert.Equal(t, d, back, "language %q code %q read as %q", language, text, md)
			}

			conv := markdown.New()
			quoteMD, _ := conv.ToMarkdown(inQuote)
			panelMD, panelIssues := conv.ToMarkdown(wrap(jira4claude.ADFNode{Type: "panel", Attrs: panelAttrs, Content: []jira4claude.ADFNode{block}}))
			assert.Empty(t, panelIssues, "language %q code %q", language, text)
			assert.Equal(t, "> [!NOTE]\n"+quoteMD, panelMD, "language %q code %q", language, text)
		}
	}
}

// A language from Jira with whitespace in it has no Markdown form: the info
// string's first word is the language. The first word is written and the
// change is reported.
func TestToMarkdown_CodeBlockLanguageWithWhitespace(t *testing.T) {
	t.Parallel()

	tests := []struct {
		language string
		want     string
	}{
		{"go extra", "```go\nx\n```"},
		{"go\textra", "```go\nx\n```"},
		{"go\nextra", "```go\nx\n```"},
		{" go", "```go\nx\n```"},
	}

	for _, tt := range tests {
		t.Run(tt.language, func(t *testing.T) {
			t.Parallel()

			md, issues := markdown.New().ToMarkdown(codeBlockDoc(tt.language, "x"))

			assert.Equal(t, tt.want, md)
			require.Len(t, issues, 1)
			assert.Equal(t, jira4claude.ConversionDegraded, issues[0].Kind)
			assert.Equal(t, "codeBlock", issues[0].Node)
		})
	}
}
