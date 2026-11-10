package markdown_test

import (
	"encoding/json"
	"os"
	"regexp"
	"strings"
	"testing"

	jira4claude "github.com/fwojciec/jira4claude"
	"github.com/fwojciec/jira4claude/markdown"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Protected nodes (#149): media, mediaSingle, mediaGroup, mediaInline,
// mention and node types the reader does not know render as numbered tokens,
// and Reattach puts the original nodes back in place of the tokens.

func loadADF(t *testing.T, path string) (*jira4claude.ADFNode, []byte) {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	var doc jira4claude.ADFNode
	require.NoError(t, json.Unmarshal(data, &doc))
	return &doc, data
}

func rawNode(t *testing.T, s string) jira4claude.ADFNode {
	t.Helper()
	var n jira4claude.ADFNode
	require.NoError(t, json.Unmarshal([]byte(s), &n))
	return n
}

func docOf(blocks ...jira4claude.ADFNode) *jira4claude.ADFNode {
	return &jira4claude.ADFNode{Type: "doc", Version: 1, Content: blocks}
}

func paraOf(inline ...jira4claude.ADFNode) jira4claude.ADFNode {
	return jira4claude.ADFNode{Type: "paragraph", Content: inline}
}

func plainText(s string) jira4claude.ADFNode {
	return jira4claude.ADFNode{Type: "text", Text: s}
}

// readEditWrite reads doc as Markdown, applies edit to it, converts the
// result back and re-attaches against doc.
func readEditWrite(t *testing.T, doc *jira4claude.ADFNode, edit func(string) string) (string, *jira4claude.ADFNode, []jira4claude.ConversionIssue) {
	t.Helper()
	conv := markdown.New()
	md, _ := conv.ToMarkdown(doc)
	edited := md
	if edit != nil {
		edited = edit(md)
	}
	adf, writeIssues := conv.ToADF(edited)
	require.Empty(t, jira4claude.DroppedIssues(writeIssues), "markdown was %q", edited)
	got, issues := conv.Reattach(doc, adf)
	require.NotNil(t, got)
	return md, got, issues
}

func jsonOf(t *testing.T, v any) string {
	t.Helper()
	data, err := json.Marshal(v)
	require.NoError(t, err)
	return string(data)
}

// tokenRE matches any token the reader writes, for counting in tests.
func tokenRE() *regexp.Regexp {
	return regexp.MustCompile(`\{\{!*[A-Za-z][A-Za-z0-9]*:[0-9]+(?: [^{}\n]*)?\}\}`)
}

// mention returns a mention node with the given id and display name.
func mention(id, name string) jira4claude.ADFNode {
	return jira4claude.ADFNode{Type: "mention", Attrs: json.RawMessage(`{"id":"` + id + `","text":"@` + name + `","accessLevel":""}`)}
}

func TestToMarkdown_ProtectedNodesRenderAsDistinctTokens(t *testing.T) {
	t.Parallel()

	tests := []struct {
		fixture string
		want    []string
	}{
		{"testdata/media.adf.json", []string{
			"{{mediaSingle:1 error dialog}}",
			"{{mediaGroup:2 server.log, client.log}}",
			"{{mediaInline:3 report.pdf}}",
		}},
		{"testdata/inline_nodes.adf.json", []string{"{{mention:1 John Smith}}"}},
	}
	for _, tt := range tests {
		t.Run(tt.fixture, func(t *testing.T) {
			t.Parallel()
			doc, _ := loadADF(t, tt.fixture)
			md, issues := markdown.New().ToMarkdown(doc)
			assert.Empty(t, jira4claude.DroppedIssues(issues))
			assert.Equal(t, tt.want, tokenRE().FindAllString(md, -1))
		})
	}

	t.Run("block tokens stand on their own line, inline tokens sit in the text", func(t *testing.T) {
		t.Parallel()
		doc, _ := loadADF(t, "testdata/media.adf.json")
		md, _ := markdown.New().ToMarkdown(doc)
		lines := strings.Split(md, "\n")
		assert.Contains(t, lines, "{{mediaSingle:1 error dialog}}")
		assert.Contains(t, lines, "{{mediaGroup:2 server.log, client.log}}")
		assert.Contains(t, lines, "Please review {{mediaInline:3 report.pdf}} before the meeting.")
	})

	t.Run("two identical mentions get different tokens", func(t *testing.T) {
		t.Parallel()
		doc := docOf(paraOf(plainText("a "), mention("u1", "Jo"), plainText(" b "), mention("u1", "Jo")))
		md, issues := markdown.New().ToMarkdown(doc)
		assert.Empty(t, issues)
		assert.Equal(t, "a {{mention:1 Jo}} b {{mention:2 Jo}}", md)
	})

	t.Run("unknown block and inline nodes are tokens, not dropped", func(t *testing.T) {
		t.Parallel()
		doc := docOf(
			rawNode(t, `{"type":"extension","attrs":{"extensionType":"com.atlassian.confluence.macro.core","extensionKey":"toc"}}`),
			paraOf(plainText("x "), rawNode(t, `{"type":"inlineExtension","attrs":{"extensionType":"com.atlassian.jira","extensionKey":"jira-issue"}}`)),
		)
		md, issues := markdown.New().ToMarkdown(doc)
		assert.Empty(t, issues)
		assert.Equal(t, "{{extension:1 toc}}\n\nx {{inlineExtension:2 jira-issue}}", md)
	})
}

func TestReattach_FixtureIdentity(t *testing.T) {
	t.Parallel()

	t.Run("media.adf.json comes back identical", func(t *testing.T) {
		t.Parallel()
		doc, raw := loadADF(t, "testdata/media.adf.json")
		_, got, issues := readEditWrite(t, doc, nil)
		assert.Empty(t, issues)
		assert.JSONEq(t, string(raw), jsonOf(t, got))
	})

	t.Run("protected nodes of inline_nodes.adf.json come back identical", func(t *testing.T) {
		t.Parallel()
		doc, _ := loadADF(t, "testdata/inline_nodes.adf.json")
		_, got, issues := readEditWrite(t, doc, nil)
		assert.Empty(t, issues)
		want := protectedNodes(doc)
		require.Len(t, want, 1)
		assert.Equal(t, jsonOf(t, want), jsonOf(t, protectedNodes(got)))
		assert.Equal(t, jsonOf(t, doc.Content[0]), jsonOf(t, got.Content[0]), "the mention's paragraph")
	})
}

func TestReattach_RestoresByteForByte(t *testing.T) {
	t.Parallel()

	// Every top-level key the ADF schema allows on a node (type, attrs,
	// content, marks, text, version) is an ADFNode field, and attrs are kept
	// raw, so a node read from Jira is written back as it was read.
	const media = `{"type":"mediaSingle","attrs":{"layout":"wrap-left","width":66.67,"widthType":"percentage"},"content":[` +
		`{"type":"media","attrs":{"id":"0c0f","type":"file","collection":"contentId-1","width":1.50,"height":2e3,` +
		`"alt":"a <b> & c","occurrenceKey":"k","__fileName":"shot.png","__fileSize":1024,"__fileMimeType":"image/png",` +
		`"__contextId":null,"__external":false,"localId":"l1"},` +
		`"marks":[{"type":"border","attrs":{"size":2,"color":"#091e4224"}}]},` +
		`{"type":"caption","content":[{"type":"text","text":"cap"}]}]}`
	const ment = `{"type":"mention","attrs":{"id":"5b10ac8d82e05b22cc7d4ef5","text":"@Jane Doe","accessLevel":"CONTAINER","userType":"DEFAULT","localId":"m1"}}`

	doc := docOf(paraOf(plainText("see")), rawNode(t, media), paraOf(plainText("hi "), rawNode(t, ment)))
	_, got, issues := readEditWrite(t, doc, nil)
	assert.Empty(t, issues)
	require.Len(t, got.Content, 3)
	assert.Equal(t, jsonOf(t, doc.Content[1]), jsonOf(t, got.Content[1]))
	assert.JSONEq(t, media, jsonOf(t, got.Content[1]))
	require.Len(t, got.Content[2].Content, 2)
	assert.JSONEq(t, ment, jsonOf(t, got.Content[2].Content[1]))
	assert.Equal(t, string(doc.Content[1].Content[0].Attrs), string(got.Content[1].Content[0].Attrs), "attrs bytes are kept raw")
}

func TestReattach_IdenticalMentions(t *testing.T) {
	t.Parallel()

	doc := docOf(paraOf(plainText("a "), mention("u1", "Jo"), plainText(" b "), mention("u1", "Jo"), plainText(" c")))

	t.Run("re-attach to their own positions after edits around them", func(t *testing.T) {
		t.Parallel()
		_, got, issues := readEditWrite(t, doc, func(md string) string {
			return strings.Replace(strings.Replace(md, "a ", "first ", 1), " c", " end", 1)
		})
		assert.Empty(t, issues)
		want := docOf(paraOf(plainText("first "), mention("u1", "Jo"), plainText(" b "), mention("u1", "Jo"), plainText(" end")))
		assert.Equal(t, jsonOf(t, want), jsonOf(t, got))
	})

	t.Run("removing one keeps the other in place and reports the removed one", func(t *testing.T) {
		t.Parallel()
		_, got, issues := readEditWrite(t, doc, func(md string) string {
			return strings.Replace(md, " b {{mention:2 Jo}}", "", 1)
		})
		want := docOf(paraOf(plainText("a "), mention("u1", "Jo"), plainText(" c")))
		assert.Equal(t, jsonOf(t, want), jsonOf(t, got))
		require.Len(t, issues, 1)
		assert.Equal(t, jira4claude.ConversionDropped, issues[0].Kind)
		assert.Equal(t, "{{mention:2 Jo}}", issues[0].Node)
	})
}

func TestReattach_SwappedTokensSwapNodes(t *testing.T) {
	t.Parallel()

	doc := docOf(paraOf(plainText("a "), mention("u1", "Ann"), plainText(" b "), mention("u2", "Bob")))
	_, got, issues := readEditWrite(t, doc, func(md string) string {
		assert.Equal(t, "a {{mention:1 Ann}} b {{mention:2 Bob}}", md)
		return "a {{mention:2 Bob}} b {{mention:1 Ann}}"
	})
	assert.Empty(t, issues)
	want := docOf(paraOf(plainText("a "), mention("u2", "Bob"), plainText(" b "), mention("u1", "Ann")))
	assert.Equal(t, jsonOf(t, want), jsonOf(t, got))
}

func TestReattach_MissingTokenIsDropped(t *testing.T) {
	t.Parallel()

	doc, _ := loadADF(t, "testdata/media.adf.json")
	_, got, issues := readEditWrite(t, doc, func(md string) string {
		md = strings.Replace(md, "{{mediaGroup:2 server.log, client.log}}\n\n", "", 1)
		return strings.Replace(md, "{{mediaInline:3 report.pdf}} ", "", 1)
	})
	require.Len(t, issues, 2)
	for _, i := range issues {
		assert.Equal(t, jira4claude.ConversionDropped, i.Kind)
	}
	assert.Equal(t, "{{mediaGroup:2 server.log, client.log}}", issues[0].Node)
	assert.Contains(t, issues[0].Detail, "mediaGroup")
	assert.Equal(t, "{{mediaInline:3 report.pdf}}", issues[1].Node)
	assert.Equal(t, "mediaSingle", got.Content[1].Type, "the remaining token is restored")

	err := jira4claude.DroppedContentError("description", issues)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "{{mediaGroup:2 server.log, client.log}}")
	assert.Contains(t, err.Error(), "{{mediaInline:3 report.pdf}}")
}

func TestReattach_LiteralTokenTextDoesNotReattach(t *testing.T) {
	t.Parallel()

	t.Run("beside real protected nodes", func(t *testing.T) {
		t.Parallel()
		doc := docOf(
			paraOf(plainText("literal {{mention:1 x}} and {{!mention:1 x}} and {{mention:2 Ann}}")),
			paraOf(mention("u1", "Ann")),
			jira4claude.ADFNode{Type: "codeBlock", Content: []jira4claude.ADFNode{plainText("{{!!mention:1 Ann}}")}},
		)
		md, got, issues := readEditWrite(t, doc, nil)
		assert.Empty(t, issues)
		assert.Equal(t, jsonOf(t, doc), jsonOf(t, got), "markdown was %q", md)
		assert.Contains(t, md, "{{!!mention:1 Ann}}\n```", "the code block is left alone")
		assert.Contains(t, md, "\n\n{{!!mention:1 Ann}}\n\n", "the real token is salted past the literal ones")
	})

	t.Run("without protected nodes", func(t *testing.T) {
		t.Parallel()
		doc := docOf(paraOf(plainText("see {{mention:1 x}} and {{mediaSingle:2}}")))
		md, issues := markdown.New().ToMarkdown(doc)
		assert.Empty(t, issues)
		_, got, issues := readEditWrite(t, doc, nil)
		assert.Empty(t, issues)
		assert.Equal(t, jsonOf(t, doc), jsonOf(t, got), "markdown was %q", md)
	})

	t.Run("a token made from adjacent text nodes", func(t *testing.T) {
		t.Parallel()
		doc := docOf(paraOf(plainText("{{"), plainText("mention:1 x}}"), plainText(" "), mention("u1", "Ann")))
		md, got, issues := readEditWrite(t, doc, nil)
		assert.Empty(t, issues)
		want := docOf(paraOf(plainText("{{mention:1 x}} "), mention("u1", "Ann")))
		assert.Equal(t, jsonOf(t, want), jsonOf(t, got), "markdown was %q", md)
	})

	t.Run("a token split across marks", func(t *testing.T) {
		t.Parallel()
		doc := docOf(paraOf(
			jira4claude.ADFNode{Type: "text", Text: "{", Marks: []jira4claude.ADFMark{{Type: "strong"}}},
			plainText("{mention:1 Ann}}"),
		))
		md, got, issues := readEditWrite(t, doc, nil)
		assert.Empty(t, issues, "markdown was %q", md)
		assert.Equal(t, jsonOf(t, doc), jsonOf(t, got), "markdown was %q", md)
	})

	t.Run("a token inside code is not recognised", func(t *testing.T) {
		t.Parallel()
		doc := docOf(paraOf(mention("u1", "Ann")))
		_, got, issues := readEditWrite(t, doc, func(md string) string {
			return "`" + md + "`"
		})
		require.Len(t, issues, 1)
		assert.Equal(t, "{{mention:1 Ann}}", issues[0].Node)
		assert.Empty(t, protectedNodes(got))
	})
}

func TestReattach_UnknownNodes(t *testing.T) {
	t.Parallel()

	ext := `{"type":"extension","attrs":{"extensionType":"com.atlassian.confluence.macro.core","extensionKey":"toc","parameters":{"macroParams":{"maxLevel":{"value":"2"}}}}}`
	bodied := `{"type":"bodiedExtension","attrs":{"extensionType":"t","extensionKey":"k"},"content":[{"type":"paragraph","content":[{"type":"text","text":"inner"}]}]}`
	inlineExt := `{"type":"inlineExtension","attrs":{"extensionType":"com.atlassian.jira","extensionKey":"jira-issue","parameters":{"key":"X-1"}}}`
	future := `{"type":"futureBlock","attrs":{"x":1},"content":[{"type":"text","text":"?"}]}`
	futureInline := `{"type":"futureInline","attrs":{"y":[1,2]}}`

	doc := docOf(
		rawNode(t, ext),
		paraOf(plainText("see "), rawNode(t, inlineExt), plainText(" and "), rawNode(t, futureInline)),
		rawNode(t, bodied),
		rawNode(t, future),
	)
	md, got, issues := readEditWrite(t, doc, nil)
	assert.Empty(t, issues)
	assert.Equal(t, jsonOf(t, doc), jsonOf(t, got), "markdown was %q", md)
	assert.Equal(t, "{{extension:1 toc}}\n\nsee {{inlineExtension:2 jira-issue}} and {{futureInline:3}}\n\n{{bodiedExtension:4 k}}\n\n{{futureBlock:5}}", md)
}

func TestReattach_TokenProblems(t *testing.T) {
	t.Parallel()

	doc := docOf(paraOf(plainText("hi "), mention("u1", "Ann")), rawNode(t,
		`{"type":"mediaSingle","attrs":{"layout":"center"},"content":[{"type":"media","attrs":{"id":"m","type":"file","collection":"c","alt":"pic"}}]}`))

	t.Run("a number the original does not have", func(t *testing.T) {
		t.Parallel()
		_, got, issues := readEditWrite(t, doc, func(md string) string { return md + "\n\nand {{mention:7 Zed}}" })
		require.Len(t, issues, 1)
		assert.Equal(t, jira4claude.ConversionDropped, issues[0].Kind)
		assert.Equal(t, "{{mention:7 Zed}}", issues[0].Node)
		assert.Equal(t, "and {{mention:7 Zed}}", got.Content[2].Content[0].Text, "left as text")
	})

	t.Run("a token used twice", func(t *testing.T) {
		t.Parallel()
		_, got, issues := readEditWrite(t, doc, func(md string) string {
			return strings.Replace(md, "hi {{mention:1 Ann}}", "hi {{mention:1 Ann}} and {{mention:1 Ann}}", 1)
		})
		require.Len(t, issues, 1)
		assert.Equal(t, jira4claude.ConversionDropped, issues[0].Kind)
		assert.Equal(t, "{{mention:1 Ann}}", issues[0].Node)
		assert.Contains(t, issues[0].Detail, "more than once")
		require.Len(t, got.Content[0].Content, 3)
		assert.Equal(t, "mention", got.Content[0].Content[1].Type, "the first one is restored")
		assert.Equal(t, " and {{mention:1 Ann}}", got.Content[0].Content[2].Text, "the second is left as text")
	})

	t.Run("the label is not trusted, only the kind and number", func(t *testing.T) {
		t.Parallel()
		_, got, issues := readEditWrite(t, doc, func(md string) string {
			md = strings.Replace(md, "{{mention:1 Ann}}", "{{mention:1 someone else}}", 1)
			return strings.Replace(md, "{{mediaSingle:2 pic}}", "{{mediaSingle:2}}", 1)
		})
		assert.Empty(t, issues)
		assert.Equal(t, jsonOf(t, doc), jsonOf(t, got))
	})

	t.Run("a token whose kind does not match its number", func(t *testing.T) {
		t.Parallel()
		_, got, issues := readEditWrite(t, doc, func(md string) string {
			return strings.Replace(md, "{{mention:1 Ann}}", "{{mediaInline:1 Ann}}", 1)
		})
		require.Len(t, issues, 2)
		assert.Equal(t, "{{mediaInline:1 Ann}}", issues[0].Node)
		assert.Contains(t, issues[0].Detail, "mention")
		assert.Equal(t, "{{mention:1 Ann}}", issues[1].Node, "the node is also missing")
		assert.Equal(t, "hi {{mediaInline:1 Ann}}", got.Content[0].Content[0].Text, "left as text")
	})
}

func TestReattach_OriginalChangedSinceRead(t *testing.T) {
	t.Parallel()

	// The description was read with a mention as node 1; before the write,
	// someone replaced it with an inline attachment, so number 1 is now a
	// different node type. The stale token must not re-attach.
	read := docOf(paraOf(plainText("hi "), mention("u1", "Ann")))
	current := docOf(paraOf(plainText("hi "), rawNode(t,
		`{"type":"mediaInline","attrs":{"id":"m","type":"file","collection":"c","__fileName":"a.pdf"}}`)))

	conv := markdown.New()
	md, _ := conv.ToMarkdown(read)
	require.Equal(t, "hi {{mention:1 Ann}}", md)
	written, _ := conv.ToADF(md + " edited")
	got, issues := conv.Reattach(current, written)

	require.Len(t, issues, 2)
	assert.Equal(t, jira4claude.ConversionDropped, issues[0].Kind)
	assert.Equal(t, "{{mention:1 Ann}}", issues[0].Node)
	assert.Contains(t, issues[0].Detail, "mediaInline")
	assert.Equal(t, "{{mediaInline:1 a.pdf}}", issues[1].Node)
	assert.Empty(t, protectedNodes(got))
}

func TestReattach_MisplacedChildren(t *testing.T) {
	t.Parallel()

	single := `{"type":"mediaSingle","attrs":{"layout":"center"},"content":[{"type":"media","attrs":{"id":"m","type":"file","collection":"c","alt":"pic"}}]}`
	blockCard := `{"type":"blockCard","attrs":{"url":"https://example.com/x"}}`
	item := func(blocks ...jira4claude.ADFNode) jira4claude.ADFNode {
		return jira4claude.ADFNode{Type: "listItem", Content: blocks}
	}
	cell := func(blocks ...jira4claude.ADFNode) jira4claude.ADFNode {
		return jira4claude.ADFNode{Type: "tableCell", Content: blocks}
	}
	row := func(cells ...jira4claude.ADFNode) jira4claude.ADFNode {
		return jira4claude.ADFNode{Type: "tableRow", Content: cells}
	}

	// Children a container does not expect are protected as a whole: they
	// read as a token in an item or cell of their own and are restored there.
	restored := []struct {
		name string
		doc  *jira4claude.ADFNode
	}{
		{"media directly in a bullet list", docOf(jira4claude.ADFNode{Type: "bulletList", Content: []jira4claude.ADFNode{
			item(paraOf(plainText("a"))), rawNode(t, single), item(paraOf(plainText("b"))),
		}})},
		{"paragraph with a mention directly in an ordered list", docOf(jira4claude.ADFNode{Type: "orderedList", Content: []jira4claude.ADFNode{
			item(paraOf(plainText("a"))), paraOf(plainText("x "), mention("u1", "Ann")),
		}})},
		{"media directly in a table", docOf(jira4claude.ADFNode{Type: "table", Content: []jira4claude.ADFNode{
			row(cell(paraOf(plainText("a")))), rawNode(t, single),
		}})},
		{"media directly in a table row", docOf(jira4claude.ADFNode{Type: "table", Content: []jira4claude.ADFNode{
			row(cell(paraOf(plainText("a"))), rawNode(t, single)),
		}})},
		{"media directly in a decision list", docOf(jira4claude.ADFNode{Type: "decisionList", Content: []jira4claude.ADFNode{
			{Type: "decisionItem", Content: []jira4claude.ADFNode{plainText("a")}},
			rawNode(t, single),
		}})},
		{"unknown block in a decision item", docOf(jira4claude.ADFNode{Type: "decisionList", Content: []jira4claude.ADFNode{
			{Type: "decisionItem", Content: []jira4claude.ADFNode{plainText("x "), mention("u1", "Ann")}},
			{Type: "decisionItem", Content: []jira4claude.ADFNode{rawNode(t, blockCard)}},
		}})},
	}
	for _, tt := range restored {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			md, readIssues := markdown.New().ToMarkdown(tt.doc)
			assert.Empty(t, jira4claude.DroppedIssues(readIssues), "markdown was %q", md)
			_, got, issues := readEditWrite(t, tt.doc, nil)
			assert.Empty(t, issues, "markdown was %q", md)
			assert.NotEmpty(t, protectedNodes(tt.doc))
			assert.Equal(t, jsonOf(t, protectedNodes(tt.doc)), jsonOf(t, protectedNodes(got)), "markdown was %q", md)
			if validateADF(tt.doc) == nil {
				requireValidADF(t, got)
			}
		})
	}

	// In a task or decision list a misplaced child has no Markdown form that
	// keeps the list what it is, so the read reports it.
	reported := []struct {
		name string
		doc  *jira4claude.ADFNode
		node string
	}{
		{"paragraph directly in a task list", docOf(jira4claude.ADFNode{Type: "taskList", Content: []jira4claude.ADFNode{
			{Type: "taskItem", Attrs: json.RawMessage(`{"localId":"i","state":"TODO"}`), Content: []jira4claude.ADFNode{plainText("a")}},
			paraOf(mention("u1", "Ann")),
		}}), "paragraph"},
		{"media nested in a task list", docOf(jira4claude.ADFNode{Type: "taskList", Content: []jira4claude.ADFNode{
			{Type: "taskItem", Attrs: json.RawMessage(`{"localId":"i","state":"TODO"}`), Content: []jira4claude.ADFNode{plainText("a")}},
			{Type: "taskList", Content: []jira4claude.ADFNode{rawNode(t, single)}},
		}}), "mediaSingle"},
	}
	for _, tt := range reported {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			md, issues := markdown.New().ToMarkdown(tt.doc)
			dropped := jira4claude.DroppedIssues(issues)
			require.Len(t, dropped, 1, "markdown was %q", md)
			assert.Equal(t, tt.node, dropped[0].Node)
		})
		t.Run(tt.name+" is reported again by the write that would remove it", func(t *testing.T) {
			t.Parallel()
			_, _, issues := readEditWrite(t, tt.doc, nil)
			dropped := jira4claude.DroppedIssues(issues)
			require.Len(t, dropped, 1)
			assert.Equal(t, tt.node, dropped[0].Node)
			assert.True(t, dropped[0].Existing)
			assert.Contains(t, dropped[0].Detail, "would remove it")

			err := jira4claude.DroppedContentError("description", issues)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "--append-description")
			assert.NotContains(t, err.Error(), "remove it")
			assert.NotContains(t, err.Error(), "put each token back")
		})
	}
}

func TestReattach_BlockTaskItem(t *testing.T) {
	t.Parallel()

	// A blockTaskItem (a task item holding paragraphs) is schema-valid: it
	// reads as a task item, with tokens for what it holds, and is written
	// back as a taskItem.
	doc := docOf(jira4claude.ADFNode{Type: "taskList", Attrs: json.RawMessage(`{"localId":"l"}`), Content: []jira4claude.ADFNode{
		{Type: "taskItem", Attrs: json.RawMessage(`{"localId":"a","state":"TODO"}`), Content: []jira4claude.ADFNode{plainText("first")}},
		{Type: "blockTaskItem", Attrs: json.RawMessage(`{"localId":"b","state":"DONE"}`), Content: []jira4claude.ADFNode{
			paraOf(plainText("ping "), mention("u1", "Ann")),
		}},
	}})

	md, readIssues := markdown.New().ToMarkdown(doc)

	assert.Empty(t, jira4claude.DroppedIssues(readIssues), "markdown was %q", md)
	assert.Contains(t, md, "- [x] ping {{mention:1 Ann}}")
	require.Len(t, readIssues, 1)
	assert.Equal(t, jira4claude.ConversionDegraded, readIssues[0].Kind)
	assert.Equal(t, "blockTaskItem", readIssues[0].Node)

	_, got, issues := readEditWrite(t, doc, nil)

	assert.Empty(t, issues)
	assert.Equal(t, jsonOf(t, protectedNodes(doc)), jsonOf(t, protectedNodes(got)))
	requireValidADF(t, doc)
	requireValidADF(t, got)
}

// assertTaskAttrs checks that a task item written back keeps the attrs of the
// item ToADF made: the state of its checkbox and a localId. (Markdown carries
// no localId, so it is a new one, as for every task item written.)
func assertTaskAttrs(t *testing.T, state string, item jira4claude.ADFNode) {
	t.Helper()
	var attrs map[string]string
	require.NoError(t, json.Unmarshal(item.Attrs, &attrs))
	assert.Equal(t, state, attrs["state"])
	assert.NotEmpty(t, attrs["localId"])
	assert.Len(t, attrs, 2)
}

func TestReattach_BlockInTaskItem(t *testing.T) {
	t.Parallel()

	const ext = `{"type":"extension","attrs":{"extensionType":"com.atlassian.macro","extensionKey":"toc"}}`
	single := `{"type":"mediaSingle","attrs":{"layout":"center"},"content":[{"type":"media","attrs":{"id":"m","type":"file","collection":"c","alt":"pic"}}]}`
	taskList := func(items ...jira4claude.ADFNode) *jira4claude.ADFNode {
		return docOf(jira4claude.ADFNode{Type: "taskList", Attrs: json.RawMessage(`{"localId":"l"}`), Content: items})
	}
	blockItem := func(blocks ...jira4claude.ADFNode) jira4claude.ADFNode {
		return jira4claude.ADFNode{Type: "blockTaskItem", Attrs: json.RawMessage(`{"localId":"b","state":"DONE"}`), Content: blocks}
	}
	plainItem := func(inline ...jira4claude.ADFNode) jira4claude.ADFNode {
		return jira4claude.ADFNode{Type: "taskItem", Attrs: json.RawMessage(`{"localId":"a","state":"TODO"}`), Content: inline}
	}

	// A task item holds inline content only: an extension restored in it
	// makes it a blockTaskItem, its text going into paragraphs.
	for _, tt := range []struct {
		name string
		doc  *jira4claude.ADFNode
	}{
		{"an extension after a paragraph", taskList(blockItem(paraOf(plainText("intro")), rawNode(t, ext)))},
		{"an extension alone", taskList(blockItem(rawNode(t, ext)))},
		{"an extension between paragraphs, after a plain item", taskList(
			plainItem(plainText("first")),
			blockItem(paraOf(plainText("a")), rawNode(t, ext), paraOf(plainText("b"))),
		)},
	} {
		t.Run(tt.name+" comes back schema-valid", func(t *testing.T) {
			t.Parallel()
			requireValidADF(t, tt.doc)
			md, got, issues := readEditWrite(t, tt.doc, nil)
			assert.Empty(t, issues, "markdown was %q", md)
			requireValidADF(t, got)
			assert.Equal(t, jsonOf(t, protectedNodes(tt.doc)), jsonOf(t, protectedNodes(got)), "markdown was %q", md)
			items := got.Content[0].Content
			last := items[len(items)-1]
			assert.Equal(t, "blockTaskItem", last.Type)
			assertTaskAttrs(t, "DONE", last)
		})
	}

	t.Run("a block token typed into a plain task item", func(t *testing.T) {
		t.Parallel()
		doc := taskList(plainItem(plainText("first")), blockItem(rawNode(t, ext)))
		md, got, issues := readEditWrite(t, doc, func(md string) string {
			token := tokenRE().FindString(md)
			require.NotEmpty(t, token, "markdown was %q", md)
			return "- [ ] first " + token + " and more"
		})
		assert.Empty(t, issues, "markdown was %q", md)
		requireValidADF(t, got)
		item := got.Content[0].Content[0]
		assert.Equal(t, "blockTaskItem", item.Type)
		assertTaskAttrs(t, "TODO", item)
		want := []jira4claude.ADFNode{paraOf(plainText("first")), rawNode(t, ext), paraOf(plainText("and more"))}
		assert.Equal(t, jsonOf(t, want), jsonOf(t, item.Content))
	})

	t.Run("a node a blockTaskItem does not allow is reported and left as text", func(t *testing.T) {
		t.Parallel()
		// Legacy ADF: a task item holding paragraph-wrapped blocks.
		doc := taskList(plainItem(paraOf(plainText("x")), rawNode(t, single)))
		md, got, issues := readEditWrite(t, doc, nil)
		require.Len(t, issues, 1, "markdown was %q", md)
		assert.Equal(t, jira4claude.ConversionDropped, issues[0].Kind)
		assert.Contains(t, issues[0].Node, "mediaSingle:1")
		assert.Contains(t, issues[0].Detail, "task item")
		requireValidADF(t, got)
		assert.Contains(t, jsonOf(t, got), "{{mediaSingle:1")
	})
}

func TestReattach_TokensNextToEmphasis(t *testing.T) {
	t.Parallel()

	bold := jira4claude.ADFNode{Type: "text", Text: "bold", Marks: []jira4claude.ADFMark{{Type: "strong"}}}
	em := jira4claude.ADFNode{Type: "text", Text: "em", Marks: []jira4claude.ADFMark{{Type: "em"}}}
	for _, tt := range []struct {
		name string
		doc  *jira4claude.ADFNode
	}{
		{"a mention directly after bold", docOf(paraOf(bold, mention("u1", "Ann"), plainText(" x")))},
		{"a mention directly before emphasis", docOf(paraOf(plainText("x "), mention("u1", "Ann"), em))},
		{"a mention between bold and emphasis", docOf(paraOf(bold, mention("u1", "Ann"), em))},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			md, got, issues := readEditWrite(t, tt.doc, nil)
			assert.Empty(t, issues, "markdown was %q", md)
			assert.Equal(t, jsonOf(t, tt.doc), jsonOf(t, got), "markdown was %q", md)
		})
	}
}

func TestFindTokens(t *testing.T) {
	t.Parallel()

	conv := markdown.New()

	t.Run("finds tokens of every salt, in order", func(t *testing.T) {
		t.Parallel()

		doc, _ := conv.ToADF("see {{!mention:1 Ann}} and\n\n{{mediaSingle:2 shot.png}}\n\n- {{!!!status:3}}")

		assert.Equal(t, []string{"{{!mention:1 Ann}}", "{{mediaSingle:2 shot.png}}", "{{!!!status:3}}"}, conv.FindTokens(doc))
	})

	t.Run("ignores code and text that is not a token", func(t *testing.T) {
		t.Parallel()

		doc, _ := conv.ToADF("`{{mention:1 Ann}}` and {{ not one }} and {mention:1}\n\n```\n{{mention:2 Bob}}\n```")

		assert.Empty(t, conv.FindTokens(doc))
	})

	t.Run("a nil document has none", func(t *testing.T) {
		t.Parallel()

		assert.Empty(t, conv.FindTokens(nil))
	})
}

func TestReattach_MovedTokens(t *testing.T) {
	t.Parallel()

	single := `{"type":"mediaSingle","attrs":{"layout":"center"},"content":[{"type":"media","attrs":{"id":"m","type":"file","collection":"c","alt":"pic"}}]}`
	group := `{"type":"mediaGroup","content":[{"type":"media","attrs":{"id":"g","type":"file","collection":"c","__fileName":"a.log"}}]}`
	doc := docOf(paraOf(plainText("hi "), mention("u1", "Ann")), rawNode(t, single), rawNode(t, group))

	t.Run("a block token moved into a paragraph splits it where the container allows the node", func(t *testing.T) {
		t.Parallel()
		_, got, issues := readEditWrite(t, doc, func(md string) string {
			return "before {{mediaSingle:2 pic}} after {{mention:1 Ann}}\n\n{{mediaGroup:3 a.log}}"
		})
		assert.Empty(t, issues)
		want := docOf(paraOf(plainText("before")), rawNode(t, single), paraOf(plainText("after "), mention("u1", "Ann")), rawNode(t, group))
		assert.Equal(t, jsonOf(t, want), jsonOf(t, got))
	})

	t.Run("a block token moved into a list item, which allows it", func(t *testing.T) {
		t.Parallel()
		_, got, issues := readEditWrite(t, doc, func(md string) string {
			return "- item\n\n  {{mediaSingle:2 pic}}\n\n{{mention:1 Ann}}\n\n{{mediaGroup:3 a.log}}"
		})
		assert.Empty(t, issues)
		require.Equal(t, "bulletList", got.Content[0].Type)
		item := got.Content[0].Content[0]
		require.Len(t, item.Content, 2)
		assert.JSONEq(t, single, jsonOf(t, item.Content[1]))
	})

	t.Run("a block token where the container does not allow the node is reported and left as text", func(t *testing.T) {
		t.Parallel()
		_, got, issues := readEditWrite(t, doc, func(md string) string {
			return "# head {{mediaSingle:2 pic}}\n\n- {{mediaGroup:3 a.log}}\n\n{{mention:1 Ann}}"
		})
		require.Len(t, issues, 2)
		assert.Equal(t, "{{mediaSingle:2 pic}}", issues[0].Node)
		assert.Contains(t, issues[0].Detail, "heading")
		assert.Equal(t, "{{mediaGroup:3 a.log}}", issues[1].Node)
		assert.Contains(t, issues[1].Detail, "listItem")
		assert.Equal(t, "head {{mediaSingle:2 pic}}", got.Content[0].Content[0].Text)
	})

	t.Run("an inline token moved to its own line becomes a paragraph of its own", func(t *testing.T) {
		t.Parallel()
		_, got, issues := readEditWrite(t, doc, func(md string) string {
			return "hi\n\n{{mention:1 Ann}}\n\n{{mediaSingle:2 pic}}\n\n{{mediaGroup:3 a.log}}"
		})
		assert.Empty(t, issues)
		want := docOf(paraOf(plainText("hi")), paraOf(mention("u1", "Ann")), rawNode(t, single), rawNode(t, group))
		assert.Equal(t, jsonOf(t, want), jsonOf(t, got))
	})
}

func TestReattach_ContainersRoundTrip(t *testing.T) {
	t.Parallel()

	single := `{"type":"mediaSingle","attrs":{"layout":"center"},"content":[{"type":"media","attrs":{"id":"m","type":"file","collection":"c","alt":"pic"}}]}`
	blockCard := `{"type":"blockCard","attrs":{"url":"https://example.com/x"}}`
	cell := func(blocks ...jira4claude.ADFNode) jira4claude.ADFNode {
		return jira4claude.ADFNode{Type: "tableCell", Content: blocks}
	}
	header := func(blocks ...jira4claude.ADFNode) jira4claude.ADFNode {
		return jira4claude.ADFNode{Type: "tableHeader", Content: blocks}
	}
	row := func(cells ...jira4claude.ADFNode) jira4claude.ADFNode {
		return jira4claude.ADFNode{Type: "tableRow", Content: cells}
	}

	tests := []struct {
		name string
		doc  *jira4claude.ADFNode
		// invalid: the schema rejects the document (a blockCard is not
		// allowed in a list item or blockquote); it is restored as read.
		invalid bool
	}{
		{"table cell with text and an image", docOf(jira4claude.ADFNode{Type: "table", Content: []jira4claude.ADFNode{
			row(header(paraOf(plainText("h1"))), header(paraOf(plainText("h2")))),
			row(cell(paraOf(plainText("shot")), rawNode(t, single)), cell(paraOf(plainText("by "), mention("u1", "Ann")))),
		}}), false},
		{"list item", docOf(jira4claude.ADFNode{Type: "bulletList", Content: []jira4claude.ADFNode{
			{Type: "listItem", Content: []jira4claude.ADFNode{paraOf(plainText("x")), rawNode(t, single)}},
		}}), false},
		{"blockquote", docOf(jira4claude.ADFNode{Type: "blockquote", Content: []jira4claude.ADFNode{
			paraOf(plainText("x "), mention("u1", "Ann")), rawNode(t, single),
		}}), false},
		{"heading", docOf(jira4claude.ADFNode{Type: "heading", Attrs: json.RawMessage(`{"level":2}`), Content: []jira4claude.ADFNode{
			plainText("by "), mention("u1", "Ann"),
		}}), false},
		{"panel", docOf(jira4claude.ADFNode{Type: "panel", Attrs: json.RawMessage(`{"panelType":"info"}`), Content: []jira4claude.ADFNode{
			paraOf(plainText("x "), mention("u1", "Ann")), rawNode(t, single), rawNode(t, blockCard),
		}}), false},
		{"expand", docOf(jira4claude.ADFNode{Type: "expand", Attrs: json.RawMessage(`{"title":"t"}`), Content: []jira4claude.ADFNode{
			paraOf(plainText("x "), mention("u1", "Ann")), rawNode(t, single), rawNode(t, blockCard),
		}}), false},
		{"task item", docOf(jira4claude.ADFNode{Type: "taskList", Attrs: json.RawMessage(`{"localId":"l"}`), Content: []jira4claude.ADFNode{
			{Type: "taskItem", Attrs: json.RawMessage(`{"localId":"i","state":"TODO"}`), Content: []jira4claude.ADFNode{
				plainText("ask "), mention("u1", "Ann"),
			}},
		}}), false},
		{"unknown block in a list item, table cell and blockquote", docOf(
			jira4claude.ADFNode{Type: "bulletList", Content: []jira4claude.ADFNode{
				{Type: "listItem", Content: []jira4claude.ADFNode{paraOf(plainText("x")), rawNode(t, blockCard)}},
			}},
			jira4claude.ADFNode{Type: "table", Content: []jira4claude.ADFNode{
				row(header(paraOf(plainText("h")))),
				row(cell(paraOf(plainText("x")), rawNode(t, blockCard))),
			}},
			jira4claude.ADFNode{Type: "blockquote", Content: []jira4claude.ADFNode{paraOf(plainText("x")), rawNode(t, blockCard)}},
		), true},
		{"layout, which the reader flattens into the document", docOf(jira4claude.ADFNode{Type: "layoutSection", Content: []jira4claude.ADFNode{
			{Type: "layoutColumn", Content: []jira4claude.ADFNode{paraOf(plainText("x")), rawNode(t, blockCard), rawNode(t, single)}},
		}}), true},
		{"list items after an empty item, with a mention and an image", docOf(jira4claude.ADFNode{Type: "bulletList", Content: []jira4claude.ADFNode{
			{Type: "listItem", Content: []jira4claude.ADFNode{{Type: "paragraph", Content: []jira4claude.ADFNode{}}}},
			{Type: "listItem", Content: []jira4claude.ADFNode{paraOf(plainText("by "), mention("u1", "Ann")), rawNode(t, single)}},
			{Type: "listItem", Content: []jira4claude.ADFNode{rawNode(t, single)}},
		}}), false},
		{"image with a pipe in its alt text in a table cell", docOf(jira4claude.ADFNode{Type: "table", Content: []jira4claude.ADFNode{
			row(header(paraOf(plainText("h")))),
			row(cell(paraOf(plainText("x")), rawNode(t,
				`{"type":"mediaSingle","attrs":{"layout":"center"},"content":[{"type":"media","attrs":{"id":"m","type":"file","collection":"c","alt":"a | b \\| c"}}]}`))),
		}}), false},
		{"image directly under a panel header", docOf(jira4claude.ADFNode{Type: "panel", Attrs: json.RawMessage(`{"panelType":"info"}`), Content: []jira4claude.ADFNode{
			rawNode(t, single), paraOf(plainText("after")),
		}}), false},
		{"mention with a pipe in a table cell", docOf(jira4claude.ADFNode{Type: "table", Content: []jira4claude.ADFNode{
			row(header(paraOf(plainText("h")))),
			row(cell(paraOf(jira4claude.ADFNode{Type: "mention", Attrs: json.RawMessage(`{"id":"u1","text":"@A | B \\| C"}`)}))),
		}}), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if !tt.invalid {
				requireValidADF(t, tt.doc)
			}
			md, got, issues := readEditWrite(t, tt.doc, nil)
			assert.Empty(t, issues, "markdown was %q", md)
			assert.NotEmpty(t, protectedNodes(tt.doc))
			assert.Equal(t, jsonOf(t, protectedNodes(tt.doc)), jsonOf(t, protectedNodes(got)), "markdown was %q", md)
			if !tt.invalid {
				requireValidADF(t, got)
			}
		})
	}
}

func TestReattach_LabelsSurviveMarkdown(t *testing.T) {
	t.Parallel()

	labels := []string{
		"A &amp; B", "*x*", "a_b_ _c_", `a\*b`, "<b>bold</b>", "~~x~~", "[x](y)", "`c`",
		"me@2x.png", "www.example.com", "http://x.y/z", "{{mention:9 x}}", "line\nbreak", "# 1. - > |",
	}
	attrs := func(m map[string]any) json.RawMessage {
		b, err := json.Marshal(m)
		require.NoError(t, err)
		return b
	}
	for _, label := range labels {
		t.Run(label, func(t *testing.T) {
			t.Parallel()
			doc := docOf(
				paraOf(plainText("see "), jira4claude.ADFNode{Type: "mention", Attrs: attrs(map[string]any{"id": "1", "text": "@" + label})}, plainText(" now")),
				paraOf(plainText("file "), jira4claude.ADFNode{Type: "mediaInline", Attrs: attrs(map[string]any{"id": "1", "__fileName": label})}),
				jira4claude.ADFNode{Type: "mediaSingle", Content: []jira4claude.ADFNode{{
					Type: "media", Attrs: attrs(map[string]any{"id": "1", "type": "file", "__fileName": label}),
				}}},
			)
			md, got, issues := readEditWrite(t, doc, nil)
			assert.Empty(t, issues, "markdown was %q", md)
			assert.Equal(t, jsonOf(t, doc), jsonOf(t, got), "markdown was %q", md)
		})
	}
}

func TestReattach_DoesNotModifyItsInputs(t *testing.T) {
	t.Parallel()

	doc := docOf(paraOf(plainText("a "), mention("u1", "Ann")), rawNode(t,
		`{"type":"mediaSingle","content":[{"type":"media","attrs":{"id":"m","type":"file","collection":"c"}}]}`))
	conv := markdown.New()
	md, _ := conv.ToMarkdown(doc)
	edited, _ := conv.ToADF("x " + md)
	before, original := jsonOf(t, edited), jsonOf(t, doc)

	got, issues := conv.Reattach(doc, edited)
	assert.Empty(t, issues)
	assert.Equal(t, before, jsonOf(t, edited))
	assert.Equal(t, original, jsonOf(t, doc))

	got.Content[1].Content[0].Attrs[2] = 'X'
	assert.Equal(t, original, jsonOf(t, doc), "restored nodes share no memory with the original")
}

func TestReattach_NilDocuments(t *testing.T) {
	t.Parallel()

	conv := markdown.New()
	adf, _ := conv.ToADF("plain")
	got, issues := conv.Reattach(nil, adf)
	assert.Empty(t, issues)
	assert.Equal(t, jsonOf(t, adf), jsonOf(t, got))

	got, issues = conv.Reattach(docOf(paraOf(mention("u1", "Ann"))), nil)
	require.NotNil(t, got)
	require.Len(t, issues, 1)
	assert.Equal(t, "{{mention:1 Ann}}", issues[0].Node)
}

// protectedNodes returns the protected nodes of doc in document order,
// without descending into them.
func protectedNodes(doc *jira4claude.ADFNode) []jira4claude.ADFNode {
	var out []jira4claude.ADFNode
	var walk func(n jira4claude.ADFNode)
	walk = func(n jira4claude.ADFNode) {
		switch n.Type {
		case "media", "mediaSingle", "mediaGroup", "mediaInline", "mention",
			"extension", "inlineExtension", "bodiedExtension", "multiBodiedExtension", "extensionFrame",
			"blockCard", "embedCard":
			out = append(out, n)
			return
		}
		for _, c := range n.Content {
			walk(c)
		}
	}
	if doc != nil {
		walk(*doc)
	}
	return out
}
