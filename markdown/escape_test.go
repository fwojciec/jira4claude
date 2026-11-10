package markdown_test

import (
	"encoding/json"
	"strings"
	"testing"

	jira4claude "github.com/fwojciec/jira4claude"
	"github.com/fwojciec/jira4claude/markdown"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
		"pass <word> as *b* &amp; [x]",
		"<div>",
		"a <b>bold</b> c",
		"</p> closes",
		"<!-- comment -->",
		"<?php echo ?>",
		"<!DOCTYPE html>",
		"<![CDATA[x]]>",
		"<https://example.com>",
		"mail <a@b.co> now",
		"<1@b.co>",
		"a < b and x<5 and <- and <3",
		"ends with <",
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

func TestRoundTrip_BareURLKeepsWholeAutoLink(t *testing.T) {
	t.Parallel()

	// A bare URL in plain Jira text gains a link mark on write (Linkify). The
	// read side must not escape inside it, or the link would stop at the
	// first backslash. The linked range is the one Linkify gives the
	// unescaped text, and every character of the text is kept.
	link := func(url string) jira4claude.ADFNode {
		attrs, err := json.Marshal(map[string]any{"href": url})
		require.NoError(t, err)
		return jira4claude.ADFNode{
			Type:  "text",
			Text:  url,
			Marks: []jira4claude.ADFMark{{Type: "link", Attrs: attrs}},
		}
	}
	plain := func(text string) jira4claude.ADFNode {
		return jira4claude.ADFNode{Type: "text", Text: text}
	}

	tests := []struct {
		name     string
		text     string
		expected []jira4claude.ADFNode
	}{
		{
			name:     "brackets in query",
			text:     "https://example.com/?filter[x]=1",
			expected: []jira4claude.ADFNode{link("https://example.com/?filter[x]=1")},
		},
		{
			name:     "underscores and double tildes in path",
			text:     "see https://example.com/a_b_~~c~~/d now",
			expected: []jira4claude.ADFNode{plain("see "), link("https://example.com/a_b_~~c~~/d"), plain(" now")},
		},
		{
			// Linkify stops at "*", so the link ends there; the rest stays
			// plain text instead of becoming emphasis.
			name:     "asterisks in path",
			text:     "https://example.com/a_b*c*d",
			expected: []jira4claude.ADFNode{link("https://example.com/a_b"), plain("*c*d")},
		},
		{
			name:     "followed by a full stop",
			text:     "Read https://example.com/docs_v2.",
			expected: []jira4claude.ADFNode{plain("Read "), link("https://example.com/docs_v2"), plain(".")},
		},
		{
			name:     "followed by a closing parenthesis",
			text:     "(see https://example.com/a_b)",
			expected: []jira4claude.ADFNode{plain("(see "), link("https://example.com/a_b"), plain(")")},
		},
		{
			name:     "in the middle of a sentence",
			text:     "go to https://example.com/a_b?q=1&r=[2] for *details*",
			expected: []jira4claude.ADFNode{plain("go to "), link("https://example.com/a_b?q=1&r=[2]"), plain(" for *details*")},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			converter := markdown.New()
			md, w1 := converter.ToMarkdown(paragraphDoc(plain(tt.text)))
			assert.Empty(t, w1)

			result, w2 := converter.ToADF(md)
			assert.Empty(t, w2)
			requireValidADF(t, result)
			assert.Equal(t, paragraphDoc(tt.expected...), result, "markdown was %q", md)
		})
	}
}

func TestRoundTrip_BareURLInMarkedText(t *testing.T) {
	t.Parallel()

	// Linkify also runs right after a formatting delimiter, so a URL at the
	// start of marked text is linked whole on write; escaping inside it would
	// cut the link short.
	const url = "https://example.com/?filter[x]=1"
	linkMark := jira4claude.ADFMark{Type: "link", Attrs: []byte(`{"href":"` + url + `"}`)}

	for _, mark := range []string{"strong", "em", "strike"} {
		t.Run(mark, func(t *testing.T) {
			t.Parallel()

			m := jira4claude.ADFMark{Type: mark}
			converter := markdown.New()

			for _, tc := range []struct {
				name     string
				inline   []jira4claude.ADFNode
				expected []jira4claude.ADFNode
			}{
				{
					name:     "whole node",
					inline:   []jira4claude.ADFNode{{Type: "text", Text: url, Marks: []jira4claude.ADFMark{m}}},
					expected: []jira4claude.ADFNode{{Type: "text", Text: url, Marks: []jira4claude.ADFMark{m, linkMark}}},
				},
				{
					name: "after plain text",
					inline: []jira4claude.ADFNode{
						{Type: "text", Text: "see "},
						{Type: "text", Text: url + " now", Marks: []jira4claude.ADFMark{m}},
					},
					expected: []jira4claude.ADFNode{
						{Type: "text", Text: "see "},
						{Type: "text", Text: url, Marks: []jira4claude.ADFMark{m, linkMark}},
						{Type: "text", Text: " now", Marks: []jira4claude.ADFMark{m}},
					},
				},
				{
					name: "plain URL after marked text",
					inline: []jira4claude.ADFNode{
						{Type: "text", Text: "see", Marks: []jira4claude.ADFMark{m}},
						{Type: "text", Text: url},
					},
					expected: []jira4claude.ADFNode{
						{Type: "text", Text: "see", Marks: []jira4claude.ADFMark{m}},
						{Type: "text", Text: url, Marks: []jira4claude.ADFMark{linkMark}},
					},
				},
			} {
				md, w1 := converter.ToMarkdown(paragraphDoc(tc.inline...))
				assert.Empty(t, w1)

				result, w2 := converter.ToADF(md)
				assert.Empty(t, w2)
				requireValidADF(t, result)
				assert.Equal(t, paragraphDoc(tc.expected...), result, "%s: markdown was %q", tc.name, md)
			}
		})
	}
}

func TestRoundTrip_EntityAcrossTextNodes(t *testing.T) {
	t.Parallel()

	// textColor is not rendered in Markdown, so the two nodes are written
	// next to each other with nothing between them. The mark itself is a
	// known loss; the text must survive.
	color := []jira4claude.ADFMark{{Type: "textColor", Attrs: []byte(`{"color":"#ff0000"}`)}}
	strong := []jira4claude.ADFMark{{Type: "strong"}}

	for _, tc := range []struct{ first, second string }{
		{"&am", "p;"},
		{"a &", "amp;"},
		{"&", "#35;"},
		{"&#", "35;"},
		{"&#3", "5;"},
		{"&#x", "41;"},
		{"&#x4", "1;"},
		{"x &copy", ";"},
	} {
		for _, marks := range []struct {
			name   string
			second []jira4claude.ADFMark
		}{{"unrendered mark", color}, {"no marks", nil}, {"strong", strong}} {
			if marks.name == "strong" && tc.second == ";" {
				// "**;**" right after a letter is not a valid opening
				// delimiter: the known adjacent-marks gap, not an entity one.
				continue
			}
			t.Run(tc.first+"+"+tc.second+"/"+marks.name, func(t *testing.T) {
				t.Parallel()

				original := paragraphDoc(
					jira4claude.ADFNode{Type: "text", Text: tc.first},
					jira4claude.ADFNode{Type: "text", Text: tc.second, Marks: marks.second},
				)

				converter := markdown.New()
				md, _ := converter.ToMarkdown(original)
				result, _ := converter.ToADF(md)

				var text string
				for _, n := range result.Content[0].Content {
					text += n.Text
				}
				assert.Equal(t, tc.first+tc.second, text, "markdown was %q", md)
			})
		}
	}
}

func TestRoundTrip_TildesAcrossTextNodes(t *testing.T) {
	t.Parallel()

	// Tildes at the edges of adjacent text nodes meet in the Markdown, so a
	// "~~" can form across nodes and strike the text between two such pairs.
	color := []jira4claude.ADFMark{{Type: "textColor", Attrs: []byte(`{"color":"#ff0000"}`)}}

	for _, texts := range [][]string{
		{"a~", "~b~", "~c"},
		{"a~", "~b"},
		{"x ~", "~y~", "~ z"},
	} {
		for _, marked := range []struct {
			name  string
			marks []jira4claude.ADFMark
		}{{"unrendered mark", color}, {"no marks", nil}} {
			t.Run(strings.Join(texts, "+")+"/"+marked.name, func(t *testing.T) {
				t.Parallel()

				var inline []jira4claude.ADFNode
				for k, s := range texts {
					n := jira4claude.ADFNode{Type: "text", Text: s}
					if k%2 == 1 {
						n.Marks = marked.marks
					}
					inline = append(inline, n)
				}

				converter := markdown.New()
				md, _ := converter.ToMarkdown(paragraphDoc(inline...))
				result, _ := converter.ToADF(md)

				var text string
				for _, n := range result.Content[0].Content {
					text += n.Text
					for _, m := range n.Marks {
						assert.NotEqual(t, "strike", m.Type, "markdown was %q", md)
					}
				}
				assert.Equal(t, strings.Join(texts, ""), text, "markdown was %q", md)
			})
		}
	}

	t.Run("lone tilde at a node start stays bare", func(t *testing.T) {
		t.Parallel()

		converter := markdown.New()
		md, _ := converter.ToMarkdown(paragraphDoc(
			jira4claude.ADFNode{Type: "text", Text: "about "},
			jira4claude.ADFNode{Type: "text", Text: "~5 minutes", Marks: color},
		))
		assert.Equal(t, "about ~5 minutes", md)

		md, _ = converter.ToMarkdown(paragraphDoc(jira4claude.ADFNode{Type: "text", Text: "~5"}))
		assert.Equal(t, "~5", md)
	})
}

func TestToADF_AutolinkKeepsEntitiesRaw(t *testing.T) {
	t.Parallel()

	// On write, a Linkify autolink takes its text and href from the raw
	// source: entities inside it are not decoded in either.
	const url = "https://example.com/?a=1&amp;b=2"
	converter := markdown.New()
	result, warnings := converter.ToADF("see " + url + " now")

	attrs, err := json.Marshal(map[string]any{"href": url})
	require.NoError(t, err)
	assert.Empty(t, warnings)
	assert.Equal(t, paragraphDoc(
		jira4claude.ADFNode{Type: "text", Text: "see "},
		jira4claude.ADFNode{Type: "text", Text: url, Marks: []jira4claude.ADFMark{{Type: "link", Attrs: attrs}}},
		jira4claude.ADFNode{Type: "text", Text: " now"},
	), result)
}

func TestRoundTrip_BareURLWithEntities(t *testing.T) {
	t.Parallel()

	color := []jira4claude.ADFMark{{Type: "textColor", Attrs: []byte(`{"color":"#ff0000"}`)}}
	plain := func(s string) jira4claude.ADFNode { return jira4claude.ADFNode{Type: "text", Text: s} }

	type tc struct {
		name   string
		inline []jira4claude.ADFNode
	}
	tests := make([]tc, 0, 64)
	tests = append(tests, []tc{
		{"trailing entity then underscore", []jira4claude.ADFNode{plain("https://example.com/?x=&amp;_")}},
		{"entity completed by the next node", []jira4claude.ADFNode{plain("https://example.com/?x=&am"), plain("p;")}},
		{"entity completed by the next node with unrendered mark", []jira4claude.ADFNode{
			plain("https://example.com/?x=&am"), {Type: "text", Text: "p;", Marks: color},
		}},
	}...)
	urls := []string{
		"https://example.com/?a=1&amp;b=2&lt;c=&#35;d",
		"https://example.com/?a=&amp;",
		"https://example.com/?a=&lt;",
		"https://example.com/?a=&#35;",
		"https://example.com/?a=&#x41;",
	}
	for _, url := range urls {
		tests = append(tests, tc{url, []jira4claude.ADFNode{plain(url)}})
		for _, after := range []string{"_", "*", ")", ".", "_x", "*x*"} {
			tests = append(tests, tc{url + after, []jira4claude.ADFNode{plain("(see " + url + after + " now")}})
		}
		tests = append(tests,
			tc{url + " + node", []jira4claude.ADFNode{plain("see " + url), plain("_x")}},
			tc{url + " + marked node", []jira4claude.ADFNode{plain("see " + url), {Type: "text", Text: "x;", Marks: color}}},
		)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			converter := markdown.New()
			md, _ := converter.ToMarkdown(paragraphDoc(tt.inline...))
			result, _ := converter.ToADF(md)

			var want, got string
			for _, n := range tt.inline {
				want += n.Text
			}
			for _, n := range result.Content[0].Content {
				got += n.Text
			}
			assert.Equal(t, want, got, "markdown was %q", md)
		})
	}
}

func TestToADF_DecodesTextAroundLiteralInlineTag(t *testing.T) {
	t.Parallel()

	converter := markdown.New()
	result, warnings := converter.ToADF(`pass <word> as 2 \* 3 &amp; \[x\]`)

	assert.Empty(t, warnings)
	requireValidADF(t, result)
	assert.Equal(t, paragraphDoc(jira4claude.ADFNode{Type: "text", Text: "pass <word> as 2 * 3 & [x]"}), result)
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

	// A hardBreak is read out as a CommonMark hard break ("\" and a newline).
	// Text starting the new line must not become a block of its own
	// (heading, list, quote, fence, HTML) and must keep all its characters.
	for _, text := range []string{
		"# x", "> quote", "- item", "* item", "1. item", "---", "===", "~~~", "```",
		"<div>", "</div>", "<!-- c -->", "  indented", "\tTab",
	} {
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
			assert.Equal(t, original, result, "markdown was %q", md)
		})
	}
}

func TestRoundTrip_HardBreaks(t *testing.T) {
	t.Parallel()

	text := func(s string, marks ...jira4claude.ADFMark) jira4claude.ADFNode {
		return jira4claude.ADFNode{Type: "text", Text: s, Marks: marks}
	}
	br := jira4claude.ADFNode{Type: "hardBreak"}
	strong := jira4claude.ADFMark{Type: "strong"}
	em := jira4claude.ADFMark{Type: "em"}
	code := jira4claude.ADFMark{Type: "code"}

	tests := []struct {
		name     string
		inline   []jira4claude.ADFNode
		markdown string
	}{
		{"one", []jira4claude.ADFNode{text("a"), br, text("b")}, "a\\\nb"},
		{"two consecutive", []jira4claude.ADFNode{text("a"), br, br, text("b")}, "a\\\n\\\nb"},
		{"three consecutive", []jira4claude.ADFNode{text("a"), br, br, br, text("b")}, "a\\\n\\\n\\\nb"},
		{"leading", []jira4claude.ADFNode{br, text("b")}, "\\\nb"},
		{"after strong", []jira4claude.ADFNode{text("a", strong), br, text("b")}, "**a**\\\nb"},
		{"between marked", []jira4claude.ADFNode{text("a", strong), br, text("b", em)}, "**a**\\\n*b*"},
		{"after code", []jira4claude.ADFNode{text("a", code), br, text("b")}, "`a`\\\nb"},
		{"after a backslash", []jira4claude.ADFNode{text(`a\`), br, text("b")}, "a&#92;\\\nb"},
		{"after trailing spaces", []jira4claude.ADFNode{text("a  "), br, text("b")}, "a  \\\nb"},
		{"blank line held as a space", []jira4claude.ADFNode{text("a"), br, text(" "), br, text("b")}, "a\\\n&#32;\\\nb"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			original := paragraphDoc(tt.inline...)

			converter := markdown.New()
			md, w1 := converter.ToMarkdown(original)
			assert.Empty(t, w1)
			assert.Equal(t, tt.markdown, md)

			result, w2 := converter.ToADF(md)
			assert.Empty(t, w2)
			requireValidADF(t, result)
			assert.Equal(t, original, result, "markdown was %q", md)
		})
	}
}

func TestRoundTrip_HardBreaksInContainers(t *testing.T) {
	t.Parallel()

	br := jira4claude.ADFNode{Type: "hardBreak"}
	text := func(s string) jira4claude.ADFNode { return jira4claude.ADFNode{Type: "text", Text: s} }
	doc := func(block jira4claude.ADFNode) *jira4claude.ADFNode {
		return &jira4claude.ADFNode{Type: "doc", Version: 1, Content: []jira4claude.ADFNode{block}}
	}
	para := func(inline ...jira4claude.ADFNode) jira4claude.ADFNode {
		return jira4claude.ADFNode{Type: "paragraph", Content: inline}
	}

	for _, second := range []string{"b", "# x", "- item", "<div>"} {
		inline := []jira4claude.ADFNode{text("a"), br, text(second)}
		tests := map[string]*jira4claude.ADFNode{
			"blockquote": doc(jira4claude.ADFNode{Type: "blockquote", Content: []jira4claude.ADFNode{para(inline...)}}),
			"bullet item": doc(jira4claude.ADFNode{Type: "bulletList", Content: []jira4claude.ADFNode{
				{Type: "listItem", Content: []jira4claude.ADFNode{para(inline...)}},
			}}),
			"ordered item": doc(jira4claude.ADFNode{Type: "orderedList", Content: []jira4claude.ADFNode{
				{Type: "listItem", Content: []jira4claude.ADFNode{para(inline...)}},
			}}),
		}
		for name, original := range tests {
			t.Run(name+"/"+second, func(t *testing.T) {
				t.Parallel()

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

func TestRoundTrip_HardBreaksInListTaskAndQuote(t *testing.T) {
	t.Parallel()

	doc := func(block jira4claude.ADFNode) jira4claude.ADFNode {
		return jira4claude.ADFNode{Type: "doc", Version: 1, Content: []jira4claude.ADFNode{block}}
	}
	para := func(inline ...jira4claude.ADFNode) jira4claude.ADFNode {
		return jira4claude.ADFNode{Type: "paragraph", Content: inline}
	}

	tests := []struct {
		name     string
		original func(t *testing.T) jira4claude.ADFNode
	}{
		{"list item paragraph", func(t *testing.T) jira4claude.ADFNode {
			return doc(jira4claude.ADFNode{Type: "bulletList", Content: []jira4claude.ADFNode{
				{Type: "listItem", Content: []jira4claude.ADFNode{
					para(txt("first"), hardBreak(), txt("second")),
					para(txt("after")),
				}},
				{Type: "listItem", Content: []jira4claude.ADFNode{para(txt("next item"))}},
			}})
		}},
		{"task item with task-looking text after a break", func(t *testing.T) jira4claude.ADFNode {
			return doc(taskList(
				taskItem(t, "TODO", txt("task"), hardBreak(), txt("- [x] example")),
				taskItem(t, "DONE", txt("other")),
			))
		}},
		{"task item with two consecutive breaks", func(t *testing.T) jira4claude.ADFNode {
			return doc(taskList(taskItem(t, "TODO", txt("a"), hardBreak(), hardBreak(), txt("b"))))
		}},
		{"blockquote with quote-looking text after a break", func(t *testing.T) jira4claude.ADFNode {
			return doc(jira4claude.ADFNode{Type: "blockquote", Content: []jira4claude.ADFNode{
				para(txt("quoted"), hardBreak(), txt("> nested")),
			}})
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			original := tt.original(t)

			converter := markdown.New()
			md, w1 := converter.ToMarkdown(&original)
			assert.Empty(t, w1)

			result, w2 := converter.ToADF(md)
			assert.Empty(t, w2)
			requireValidADF(t, result)
			assert.Equal(t, original, stripLocalIDs(t, *result), "markdown was %q", md)
		})
	}
}

func TestToMarkdown_HardBreakWithoutMarkdownForm(t *testing.T) {
	t.Parallel()

	br := jira4claude.ADFNode{Type: "hardBreak"}
	text := func(s string) jira4claude.ADFNode { return jira4claude.ADFNode{Type: "text", Text: s} }
	degraded := jira4claude.ConversionIssue{
		Kind: jira4claude.ConversionDegraded,
		Node: "hardBreak",
		Detail: "a line break at the end of a paragraph, or inside a heading or table cell, " +
			"cannot be written in Markdown: it is left out at the end and read as a space elsewhere",
	}
	heading := func(inline ...jira4claude.ADFNode) *jira4claude.ADFNode {
		return &jira4claude.ADFNode{Type: "doc", Version: 1, Content: []jira4claude.ADFNode{
			{Type: "heading", Attrs: []byte(`{"level":2}`), Content: inline},
		}}
	}
	cell := func(inline ...jira4claude.ADFNode) *jira4claude.ADFNode {
		return &jira4claude.ADFNode{Type: "doc", Version: 1, Content: []jira4claude.ADFNode{{
			Type: "table",
			Content: []jira4claude.ADFNode{{
				Type: "tableRow",
				Content: []jira4claude.ADFNode{{
					Type:    "tableHeader",
					Content: []jira4claude.ADFNode{{Type: "paragraph", Content: inline}},
				}},
			}},
		}}}
	}

	tests := []struct {
		name     string
		doc      *jira4claude.ADFNode
		markdown string
		readBack *jira4claude.ADFNode
	}{
		{
			name:     "trailing in a paragraph",
			doc:      paragraphDoc(text("a"), br),
			markdown: "a",
			readBack: paragraphDoc(text("a")),
		},
		{
			name:     "two trailing in a paragraph",
			doc:      paragraphDoc(text("a"), br, br),
			markdown: "a",
			readBack: paragraphDoc(text("a")),
		},
		{
			name:     "inside a heading",
			doc:      heading(text("a"), br, text("b")),
			markdown: "## a b",
			readBack: heading(text("a b")),
		},
		{
			name:     "trailing in a heading",
			doc:      heading(text("a"), br),
			markdown: "## a",
			readBack: heading(text("a")),
		},
		{
			name:     "inside a table cell",
			doc:      cell(text("a"), br, text("b")),
			markdown: "| a b |\n| --- |",
			readBack: cell(text("a b")),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			converter := markdown.New()
			md, issues := converter.ToMarkdown(tt.doc)
			assert.Equal(t, tt.markdown, md)
			assert.Equal(t, []jira4claude.ConversionIssue{degraded}, issues)

			result, w := converter.ToADF(md)
			assert.Empty(t, w)
			assert.Equal(t, tt.readBack, result)
		})
	}
}

func TestToADF_EscapedAngleBracketNextToLiteralTag(t *testing.T) {
	t.Parallel()

	converter := markdown.New()
	result, warnings := converter.ToADF(`\<b> and <word> and \</i> and <u>x</u>`)

	assert.Empty(t, warnings)
	requireValidADF(t, result)
	assert.Equal(t, paragraphDoc(
		jira4claude.ADFNode{Type: "text", Text: "<b> and <word> and </i> and "},
		jira4claude.ADFNode{Type: "text", Text: "x", Marks: []jira4claude.ADFMark{{Type: "underline"}}},
	), result)
}

// TestRoundTrip_PlainTextExhaustive enumerates every short string over the
// characters that carry meaning in Markdown text, and checks that each one,
// held by Jira as plain text, survives ADF -> Markdown -> ADF unchanged. It
// guards the symmetry of the read-side escaping and the write-side decoding.
func TestRoundTrip_PlainTextExhaustive(t *testing.T) {
	t.Parallel()

	alphabet := []string{"a", " ", "*", "_", "~", "[", "]", `\`, "&", ";", "#", "`", "-", ">", "<", "/", "!"}
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
