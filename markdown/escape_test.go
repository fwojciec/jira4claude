package markdown_test

import (
	"strings"
	"testing"

	jira4claude "github.com/fwojciec/jira4claude"
	"github.com/fwojciec/jira4claude/markdown"
	"github.com/stretchr/testify/assert"
)

// paragraphDoc wraps inline nodes in a single-paragraph ADF document.
func paragraphDoc(inline ...jira4claude.ADFNode) *jira4claude.ADFNode {
	return &jira4claude.ADFNode{
		Type:    "doc",
		Version: 1,
		Content: []jira4claude.ADFNode{{Type: "paragraph", Content: inline}},
	}
}

func TestToADF_DecodesEscapesAndEntities(t *testing.T) {
	t.Parallel()

	em := []jira4claude.ADFMark{{Type: "em"}}
	strong := []jira4claude.ADFMark{{Type: "strong"}}

	tests := []struct {
		name     string
		markdown string
		expected []jira4claude.ADFNode
	}{
		{
			name:     "backslash escapes",
			markdown: `2 \* 3 and snake\_case and \[x\]`,
			expected: []jira4claude.ADFNode{{Type: "text", Text: "2 * 3 and snake_case and [x]"}},
		},
		{
			name:     "named entity",
			markdown: "a &amp; b",
			expected: []jira4claude.ADFNode{{Type: "text", Text: "a & b"}},
		},
		{
			name:     "numeric entities",
			markdown: "&#35; &#x41; &copy;",
			expected: []jira4claude.ADFNode{{Type: "text", Text: "# A ©"}},
		},
		{
			name:     "escaped ampersand keeps the entity literal",
			markdown: `\&amp;`,
			expected: []jira4claude.ADFNode{{Type: "text", Text: "&amp;"}},
		},
		{
			name:     "entity is decoded once",
			markdown: "&amp;amp;",
			expected: []jira4claude.ADFNode{{Type: "text", Text: "&amp;"}},
		},
		{
			name:     "unknown entity stays literal",
			markdown: "a &foo; b",
			expected: []jira4claude.ADFNode{{Type: "text", Text: "a &foo; b"}},
		},
		{
			name:     "backslash before a letter stays",
			markdown: `C:\Users`,
			expected: []jira4claude.ADFNode{{Type: "text", Text: `C:\Users`}},
		},
		{
			name:     "escaped backslash before real emphasis",
			markdown: `\\*a*`,
			expected: []jira4claude.ADFNode{
				{Type: "text", Text: `\`},
				{Type: "text", Text: "a", Marks: em},
			},
		},
		{
			name:     "escape inside emphasis",
			markdown: `**a \* b**`,
			expected: []jira4claude.ADFNode{{Type: "text", Text: "a * b", Marks: strong}},
		},
		{
			name:     "escaped tilde",
			markdown: `5\~10`,
			expected: []jira4claude.ADFNode{{Type: "text", Text: "5~10"}},
		},
		{
			name:     "escaped double tilde is literal",
			markdown: `a \~\~b\~\~ c`,
			expected: []jira4claude.ADFNode{{Type: "text", Text: "a ~~b~~ c"}},
		},
		{
			name:     "escaped backslash before a real strike",
			markdown: `\\~~gone~~`,
			expected: []jira4claude.ADFNode{
				{Type: "text", Text: `\`},
				{Type: "text", Text: "gone", Marks: []jira4claude.ADFMark{{Type: "strike"}}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			converter := markdown.New()
			result, warnings := converter.ToADF(tt.markdown)

			assert.Empty(t, warnings)
			requireValidADF(t, result)
			assert.Equal(t, paragraphDoc(tt.expected...), result)
		})
	}
}

func TestToADF_CodeKeepsEscapesAndEntitiesVerbatim(t *testing.T) {
	t.Parallel()

	t.Run("inline code", func(t *testing.T) {
		t.Parallel()

		converter := markdown.New()
		result, warnings := converter.ToADF("`a \\* b &amp; c`")

		expected := paragraphDoc(jira4claude.ADFNode{
			Type:  "text",
			Text:  `a \* b &amp; c`,
			Marks: []jira4claude.ADFMark{{Type: "code"}},
		})

		assert.Empty(t, warnings)
		requireValidADF(t, result)
		assert.Equal(t, expected, result)
	})

	t.Run("fenced code", func(t *testing.T) {
		t.Parallel()

		converter := markdown.New()
		result, warnings := converter.ToADF("```\na \\* b &amp; c \\~\\~\n```")

		expected := &jira4claude.ADFNode{
			Type:    "doc",
			Version: 1,
			Content: []jira4claude.ADFNode{{
				Type:    "codeBlock",
				Content: []jira4claude.ADFNode{{Type: "text", Text: `a \* b &amp; c \~\~`}},
			}},
		}

		assert.Empty(t, warnings)
		requireValidADF(t, result)
		assert.Equal(t, expected, result)
	})
}

func TestToMarkdown_EscapesOnlyWhatWouldBeParsed(t *testing.T) {
	t.Parallel()

	tests := []struct {
		text     string
		expected string
	}{
		// Text that cannot be parsed as formatting is read out unchanged.
		{"2 * 3", "2 * 3"},
		{"snake_case and a_b_c", "snake_case and a_b_c"},
		{`C:\Users\me`, `C:\Users\me`},
		{"a & b", "a & b"},
		{"5~10 and 20~30", "5~10 and 20~30"},
		{"a # b", "a # b"},
		{"#12345", "#12345"},
		{"1.5 and -1 and +1", "1.5 and -1 and +1"},
		{"a - b + c > d", "a - b + c > d"},
		// Text that would gain formatting or lose characters is escaped.
		{"a *b* c", `a \*b\* c`},
		{"_b_", `\_b\_`},
		{"[x]", `\[x]`},
		{"[a](b)", `\[a](b)`},
		{"a `b` c", "a \\`b\\` c"},
		{"# x", `\# x`},
		{"> quote", `\> quote`},
		{"- item", `\- item`},
		{"1. item", `1\. item`},
		{"a &amp; b", `a \&amp; b`},
		{"&#35;", `\&#35;`},
		{`a\*b`, `a\\\*b`},
		{"a ~~b~~ c", `a \~\~b\~\~ c`},
		{`C:\~user`, `C:\\~user`},
		{`a \~~b~~ c`, `a \\\~\~b\~\~ c`},
		{`end\`, `end\\`},
	}

	for _, tt := range tests {
		t.Run(tt.text, func(t *testing.T) {
			t.Parallel()

			converter := markdown.New()
			result, warnings := converter.ToMarkdown(paragraphDoc(jira4claude.ADFNode{Type: "text", Text: tt.text}))

			assert.Empty(t, warnings)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestToMarkdown_CodeTextIsNotEscaped(t *testing.T) {
	t.Parallel()

	converter := markdown.New()
	doc := paragraphDoc(jira4claude.ADFNode{
		Type:  "text",
		Text:  `a *b* [x] &amp; \~`,
		Marks: []jira4claude.ADFMark{{Type: "code"}},
	})

	result, warnings := converter.ToMarkdown(doc)

	assert.Empty(t, warnings)
	assert.Equal(t, "`a *b* [x] &amp; \\~`", result)
}

func TestRoundTrip_PlainTextFromJira(t *testing.T) {
	t.Parallel()

	strong := []jira4claude.ADFMark{{Type: "strong"}}
	link := []jira4claude.ADFMark{{Type: "link", Attrs: []byte(`{"href":"https://example.com"}`)}}

	texts := []string{
		"2 * 3 and snake_case and [x]",
		"a *b* c",
		"_b_",
		"**not bold**",
		"[x] done",
		"[ ] todo",
		"[a](b)",
		"![a](b)",
		"[!NOTE]",
		"# x",
		"## x",
		"#12345",
		"> quote",
		"- item",
		"+ item",
		"* item",
		"1. item",
		"2) item",
		"---",
		"***",
		"___",
		"~~~",
		"```",
		"a `b` c",
		`2 \* 3`,
		`a\*b`,
		`a \_b\_`,
		`C:\Users\me`,
		`end\`,
		"a &amp; b",
		"&#35; and &#x41;",
		"a & b",
		"a ~~b~~ c",
		`C:\~user`,
		`a \~~b~~ c`,
		"5~10 and 20~30",
	}

	type context struct {
		name string
		wrap func(text jira4claude.ADFNode) *jira4claude.ADFNode
	}
	contexts := []context{
		{"paragraph", func(n jira4claude.ADFNode) *jira4claude.ADFNode { return paragraphDoc(n) }},
		{"bullet item", func(n jira4claude.ADFNode) *jira4claude.ADFNode {
			return &jira4claude.ADFNode{
				Type:    "doc",
				Version: 1,
				Content: []jira4claude.ADFNode{{
					Type: "bulletList",
					Content: []jira4claude.ADFNode{{
						Type:    "listItem",
						Content: []jira4claude.ADFNode{{Type: "paragraph", Content: []jira4claude.ADFNode{n}}},
					}},
				}},
			}
		}},
		{"blockquote", func(n jira4claude.ADFNode) *jira4claude.ADFNode {
			return &jira4claude.ADFNode{
				Type:    "doc",
				Version: 1,
				Content: []jira4claude.ADFNode{{
					Type:    "blockquote",
					Content: []jira4claude.ADFNode{{Type: "paragraph", Content: []jira4claude.ADFNode{n}}},
				}},
			}
		}},
	}

	for _, c := range contexts {
		for _, text := range texts {
			t.Run(c.name+"/"+text, func(t *testing.T) {
				t.Parallel()

				original := c.wrap(jira4claude.ADFNode{Type: "text", Text: text})

				converter := markdown.New()
				md, w1 := converter.ToMarkdown(original)
				assert.Empty(t, w1)

				result, w2 := converter.ToADF(md)
				assert.Empty(t, w2)
				requireValidADF(t, result)
				assert.Equal(t, original, result, "markdown was %q", md)
			})
		}
	}

	for _, text := range texts {
		for _, marked := range []struct {
			name  string
			marks []jira4claude.ADFMark
		}{{"strong", strong}, {"link", link}} {
			t.Run(marked.name+"/"+text, func(t *testing.T) {
				t.Parallel()

				original := paragraphDoc(jira4claude.ADFNode{Type: "text", Text: text, Marks: marked.marks})

				converter := markdown.New()
				md, w1 := converter.ToMarkdown(original)
				assert.Empty(t, w1)

				result, w2 := converter.ToADF(md)
				assert.Empty(t, w2)
				requireValidADF(t, result)
				assert.Equal(t, original, result, "markdown was %q", md)
			})
		}
	}
}

func TestRoundTrip_HeadingTextFromJira(t *testing.T) {
	t.Parallel()

	for _, text := range []string{"issue #", "a ##", "# x", "a *b* #", "C# and F#"} {
		t.Run(text, func(t *testing.T) {
			t.Parallel()

			original := &jira4claude.ADFNode{
				Type:    "doc",
				Version: 1,
				Content: []jira4claude.ADFNode{{
					Type:    "heading",
					Attrs:   []byte(`{"level":2}`),
					Content: []jira4claude.ADFNode{{Type: "text", Text: text}},
				}},
			}

			converter := markdown.New()
			md, w1 := converter.ToMarkdown(original)
			assert.Empty(t, w1)

			result, w2 := converter.ToADF(md)
			assert.Empty(t, w2)
			requireValidADF(t, result)
			assert.Equal(t, original, result, "markdown was %q", md)
		})
	}
}

func TestRoundTrip_PlainTextAfterHardBreakGainsNoBlock(t *testing.T) {
	t.Parallel()

	// A hardBreak is read out as a bare newline, which is written back as a
	// soft break (a space); that loss is separate from escaping. What must
	// hold is that text starting the new line does not become a block of its
	// own (heading, list, quote, fence) and keeps all its characters.
	for _, text := range []string{"# x", "> quote", "- item", "* item", "1. item", "---", "===", "~~~", "```"} {
		t.Run(text, func(t *testing.T) {
			t.Parallel()

			original := paragraphDoc(
				jira4claude.ADFNode{Type: "text", Text: "first"},
				jira4claude.ADFNode{Type: "hardBreak"},
				jira4claude.ADFNode{Type: "text", Text: text},
			)

			converter := markdown.New()
			md, w1 := converter.ToMarkdown(original)
			assert.Empty(t, w1)

			result, w2 := converter.ToADF(md)
			assert.Empty(t, w2)
			requireValidADF(t, result)
			assert.Equal(t, paragraphDoc(jira4claude.ADFNode{Type: "text", Text: "first " + text}), result, "markdown was %q", md)
		})
	}
}

// TestRoundTrip_PlainTextExhaustive enumerates every short string over the
// characters that carry meaning in Markdown text, and checks that each one,
// held by Jira as plain text, survives ADF -> Markdown -> ADF unchanged. It
// guards the symmetry of the read-side escaping and the write-side decoding.
func TestRoundTrip_PlainTextExhaustive(t *testing.T) {
	t.Parallel()

	alphabet := []string{"a", " ", "*", "_", "~", "[", "]", `\`, "&", ";", "#", "`", "-", ">"}
	const maxLen = 4

	converter := markdown.New()
	var failures []string
	var walk func(prefix string)
	walk = func(prefix string) {
		if prefix != "" && strings.TrimSpace(prefix) == prefix {
			original := paragraphDoc(jira4claude.ADFNode{Type: "text", Text: prefix})
			md, _ := converter.ToMarkdown(original)
			result, _ := converter.ToADF(md)
			if len(result.Content) != 1 || len(result.Content[0].Content) != 1 ||
				result.Content[0].Content[0].Text != prefix || len(result.Content[0].Content[0].Marks) != 0 {
				failures = append(failures, prefix+" -> "+md)
			}
		}
		if len(prefix) == maxLen {
			return
		}
		for _, c := range alphabet {
			walk(prefix + c)
		}
	}
	walk("")

	assert.Empty(t, failures)
}
