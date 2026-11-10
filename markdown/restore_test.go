package markdown_test

import (
	"encoding/json"
	"strings"
	"testing"

	jira4claude "github.com/fwojciec/jira4claude"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Restoring what Markdown cannot carry (jir-h44x): where the edit leaves a
// node as the reader showed it, the original comes back; where it changed
// it, what the edit could not have expressed is taken from the original when
// the node can be matched, and reported when it cannot.

const (
	customPanel = `{"type":"panel","attrs":{"panelType":"custom","panelColor":"#ABF5D1"},"content":[{"type":"paragraph","content":[{"type":"text","text":"Keep"}]}]}`
	centered    = `{"type":"paragraph","marks":[{"type":"alignment","attrs":{"align":"center"}}],"content":[{"type":"text","text":"Same"}]}`
)

func TestRestore_AttributesOfAnEditedNode(t *testing.T) {
	t.Parallel()

	t.Run("a custom panel whose text is edited keeps its type and colour", func(t *testing.T) {
		t.Parallel()
		doc := docOf(rawNode(t, customPanel))
		_, got, issues := readEditWrite(t, doc, func(md string) string { return strings.Replace(md, "Keep", "Kept", 1) })
		assert.Empty(t, issues)
		want := docOf(rawNode(t, strings.Replace(customPanel, "Keep", "Kept", 1)))
		assert.JSONEq(t, jsonOf(t, want), jsonOf(t, got))
	})

	t.Run("a value the edit changes is the edit's", func(t *testing.T) {
		t.Parallel()
		doc := docOf(
			rawNode(t, `{"type":"heading","attrs":{"level":2,"localId":"h"},"content":[{"type":"text","text":"Title"}]}`),
			rawNode(t, `{"type":"orderedList","attrs":{"order":3},"content":[{"type":"listItem","content":[{"type":"paragraph","content":[{"type":"text","text":"x"}]}]}]}`),
		)
		_, got, issues := readEditWrite(t, doc, func(md string) string {
			return strings.Replace(strings.Replace(md, "## Title", "### Title", 1), "3. x", "7. x", 1)
		})
		assert.Empty(t, issues)
		assert.JSONEq(t, `{"level":3,"localId":"h"}`, string(got.Content[0].Attrs), "the level is the edit's, the localId the original's")
		assert.JSONEq(t, `{"order":7}`, string(got.Content[1].Attrs))
	})

	t.Run("a panel type the edit changes is the edit's; the colour, which Markdown cannot show, stays", func(t *testing.T) {
		t.Parallel()
		doc := docOf(rawNode(t, customPanel))
		_, got, issues := readEditWrite(t, doc, func(md string) string { return strings.Replace(md, "[!NOTE]", "[!WARNING]", 1) })
		assert.JSONEq(t, `{"panelType":"warning","panelColor":"#ABF5D1"}`, string(got.Content[0].Attrs))
		require.Len(t, issues, 1, "the custom type is gone")
		assert.Equal(t, "panel", issues[0].Node)
		assert.Equal(t, jira4claude.ConversionDegraded, issues[0].Kind)
	})

	t.Run("task items keep their localIds", func(t *testing.T) {
		t.Parallel()
		doc, _ := loadADF(t, "testdata/tasklist.adf.json")
		_, got, issues := readEditWrite(t, doc, func(md string) string { return strings.Replace(md, "[ ]", "[x]", 1) })
		assert.Empty(t, issues)
		assert.Equal(t, jsonOf(t, collectAttr(doc, "localId")), jsonOf(t, collectAttr(got, "localId")))
	})
}

func TestRestore_Refusals(t *testing.T) {
	t.Parallel()

	t.Run("a panel inserted before another does not take its attributes", func(t *testing.T) {
		t.Parallel()
		doc := docOf(rawNode(t, customPanel))
		_, got, issues := readEditWrite(t, doc, func(md string) string { return "> [!NOTE]\n> New\n\n" + md })
		assert.Empty(t, issues)
		require.Len(t, got.Content, 2)
		assert.JSONEq(t, `{"panelType":"info"}`, string(got.Content[0].Attrs), "the new panel")
		assert.Equal(t, jsonOf(t, rawNode(t, customPanel)), jsonOf(t, got.Content[1]), "the original panel, unchanged")
	})

	t.Run("a copy of an unchanged paragraph makes the copies ambiguous: neither is matched, and the loss is reported", func(t *testing.T) {
		t.Parallel()
		doc := docOf(rawNode(t, centered))
		_, got, issues := readEditWrite(t, doc, func(md string) string { return md + "\n\n" + md })
		require.Len(t, got.Content, 2)
		for _, p := range got.Content {
			assert.Empty(t, p.Marks)
		}
		require.Len(t, issues, 1)
		assert.Equal(t, "alignment", issues[0].Node)
		assert.Equal(t, jira4claude.ConversionDegraded, issues[0].Kind)
		assert.Contains(t, issues[0].Detail, "1 mark")
	})

	t.Run("an edited paragraph among more paragraphs than before is not matched by position", func(t *testing.T) {
		t.Parallel()
		doc := docOf(rawNode(t, centered), paraOf(plainText("other")))
		_, got, issues := readEditWrite(t, doc, func(md string) string {
			return strings.Replace(md, "Same", "Changed", 1) + "\n\nadded"
		})
		require.Len(t, got.Content, 3)
		assert.Empty(t, got.Content[0].Marks)
		require.Len(t, issues, 1)
		assert.Equal(t, "alignment", issues[0].Node)
	})

	t.Run("table geometry stays out of a table whose shape the edit changed", func(t *testing.T) {
		t.Parallel()
		doc := docOf(rawNode(t, `{"type":"table","attrs":{"layout":"wide"},"content":[`+
			`{"type":"tableRow","content":[{"type":"tableHeader","attrs":{"colwidth":[200]},"content":[{"type":"paragraph","content":[{"type":"text","text":"H"}]}]}]},`+
			`{"type":"tableRow","content":[{"type":"tableCell","attrs":{"colwidth":[200]},"content":[{"type":"paragraph","content":[{"type":"text","text":"a"}]}]}]}]}`))
		_, got, issues := readEditWrite(t, doc, func(md string) string { return md + "\n| b |" })
		requireValidADF(t, got)
		table := got.Content[0]
		assert.JSONEq(t, `{"layout":"wide"}`, string(table.Attrs), "the table's own attributes are kept")
		require.Len(t, table.Content, 3)
		for _, row := range table.Content {
			assert.Nil(t, row.Content[0].Attrs, "no cell keeps its width")
		}
		require.NotEmpty(t, issues)
		assert.Equal(t, "tableCell", issues[0].Node)
		assert.Equal(t, "tableHeader", issues[1].Node)
	})
}

func TestRestore_ReadAsNothing(t *testing.T) {
	t.Parallel()

	doc := docOf(paraOf(plainText("one")), paraOf(), paraOf(plainText("two")))

	t.Run("an empty paragraph goes back next to a neighbour that came back", func(t *testing.T) {
		t.Parallel()
		_, got, issues := readEditWrite(t, doc, func(md string) string { return strings.Replace(md, "two", "three", 1) })
		assert.Empty(t, issues)
		assert.Equal(t, jsonOf(t, docOf(paraOf(plainText("one")), paraOf(), paraOf(plainText("three")))), jsonOf(t, got))
	})

	t.Run("with both neighbours edited it is left out, and reported", func(t *testing.T) {
		t.Parallel()
		_, got, issues := readEditWrite(t, doc, func(md string) string {
			return strings.Replace(strings.Replace(md, "two", "three", 1), "one", "zero", 1)
		})
		assert.Len(t, got.Content, 2)
		require.Len(t, issues, 1)
		assert.Equal(t, "paragraph", issues[0].Node)
		assert.Equal(t, jira4claude.ConversionDegraded, issues[0].Kind)
	})
}

func TestRestore_ReadLossOfAnEditedBlockIsCounted(t *testing.T) {
	t.Parallel()

	code := func(s string) jira4claude.ADFNode {
		return jira4claude.ADFNode{Type: "text", Text: s, Marks: []jira4claude.ADFMark{{Type: "code"}}}
	}
	edit := func(md string) string { return strings.Replace(md, "Alpha", "Omega", 1) }

	t.Run("every place in the edited block", func(t *testing.T) {
		t.Parallel()
		doc := docOf(paraOf(plainText("Alpha "), code("a\nb"), plainText(" and "), code("c\nd")))
		_, _, issues := readEditWrite(t, doc, edit)
		require.Len(t, issues, 1)
		assert.Equal(t, jira4claude.ConversionDegraded, issues[0].Kind)
		assert.True(t, issues[0].Existing)
		assert.Contains(t, issues[0].Detail, "(2 places)", issues[0].Detail)
	})

	t.Run("not the places in a block that comes back whole", func(t *testing.T) {
		t.Parallel()
		doc := docOf(paraOf(plainText("Alpha "), code("a\nb")), paraOf(plainText("Beta "), code("c\nd")))
		_, got, issues := readEditWrite(t, doc, edit)
		require.Len(t, issues, 1)
		assert.Contains(t, issues[0].Detail, "(1 place)", issues[0].Detail)
		require.Len(t, got.Content, 2)
		assert.Equal(t, jsonOf(t, doc.Content[1]), jsonOf(t, got.Content[1]))
	})
}

// TestRestore_SwappedTokensOfNodesThatDifferOnlyInLocalID checks that the
// localId of a protected node counts when the edit is compared with the
// read: it came from the original through the token, not from the writer.
func TestRestore_SwappedTokensOfNodesThatDifferOnlyInLocalID(t *testing.T) {
	t.Parallel()

	withID := func(n jira4claude.ADFNode, id string) jira4claude.ADFNode {
		var attrs map[string]any
		require.NoError(t, json.Unmarshal(n.Attrs, &attrs))
		attrs["localId"] = id
		data, err := json.Marshal(attrs)
		require.NoError(t, err)
		n.Attrs = data
		return n
	}
	swap := func(a, b string) func(string) string {
		return func(md string) string {
			md = strings.Replace(md, a, "\x00", 1)
			md = strings.Replace(md, b, a, 1)
			return strings.Replace(md, "\x00", b, 1)
		}
	}
	m1, m2 := withID(mention("u1", "Jo"), "id-1"), withID(mention("u1", "Jo"), "id-2")
	s1 := rawNode(t, `{"type":"status","attrs":{"text":"DONE","color":"green","localId":"s-1"}}`)
	s2 := rawNode(t, `{"type":"status","attrs":{"text":"DONE","color":"green","localId":"s-2"}}`)
	e1 := rawNode(t, `{"type":"extension","attrs":{"extensionType":"com.x","extensionKey":"k","localId":"e-1"}}`)
	e2 := rawNode(t, `{"type":"extension","attrs":{"extensionType":"com.x","extensionKey":"k","localId":"e-2"}}`)

	tests := []struct {
		name      string
		doc       *jira4claude.ADFNode
		a, b      string
		want      []jira4claude.ADFNode
		unchanged bool
	}{
		{
			name: "mentions in one paragraph",
			doc:  docOf(paraOf(m1, plainText(" and "), m2)),
			a:    "{{mention:1 Jo}}", b: "{{mention:2 Jo}}",
			want: []jira4claude.ADFNode{m2, m1},
		},
		{
			name: "statuses in two paragraphs",
			doc:  docOf(paraOf(s1), paraOf(s2)),
			a:    "{{status:1 DONE}}", b: "{{status:2 DONE}}",
			want: []jira4claude.ADFNode{s2, s1},
		},
		{
			name: "extension blocks",
			doc:  docOf(e1, paraOf(plainText("text")), e2),
			a:    "{{extension:1", b: "{{extension:2",
			want: []jira4claude.ADFNode{e2, e1},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			md, got, issues := readEditWrite(t, tt.doc, swap(tt.a, tt.b))
			require.Contains(t, md, tt.a, "the token form this test swaps")
			require.Contains(t, md, tt.b, "the token form this test swaps")
			assert.Empty(t, issues)
			assert.Equal(t, jsonOf(t, tt.want), jsonOf(t, protectedNodes(got)))
		})
	}
}

// TestRestore_UnchangedCellOfAReshapedTable checks that a paragraph put
// back in a table that gained a row is not reported again: its cell is
// not restored whole, but the paragraph is.
func TestRestore_UnchangedCellOfAReshapedTable(t *testing.T) {
	t.Parallel()

	card := `{"type":"inlineCard","attrs":{"data":{"@type":"Document","name":"x"}}}`
	cell := func(cellType, content string) string {
		return `{"type":"` + cellType + `","attrs":{},"content":[{"type":"paragraph","content":[` + content + `]}]}`
	}
	head, body := `{"type":"text","text":"Head"}`, `{"type":"text","text":"Body"}`
	withCard := `{"type":"text","text":"Card "},` + card
	for _, tt := range []struct{ name, head, body string }{
		{"in a tableCell", head, withCard},
		{"in a tableHeader", withCard, body},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			doc := docOf(rawNode(t, `{"type":"table","attrs":{},"content":[`+
				`{"type":"tableRow","content":[`+cell("tableHeader", tt.head)+`]},`+
				`{"type":"tableRow","content":[`+cell("tableCell", tt.body)+`]}]}`))

			t.Run("a row added", func(t *testing.T) {
				t.Parallel()
				_, got, issues := readEditWrite(t, doc, func(md string) string { return md + "\n| added |" })
				assert.Empty(t, jira4claude.DroppedIssues(issues))
				assert.Contains(t, jsonOf(t, got), `"@type":"Document"`, "the card stays")
				assert.Contains(t, jsonOf(t, got), "added")
			})

			t.Run("the paragraph holding the card edited", func(t *testing.T) {
				t.Parallel()
				_, _, issues := readEditWrite(t, doc, func(md string) string { return strings.Replace(md, "Card", "Kard", 1) })
				dropped := jira4claude.DroppedIssues(issues)
				require.Len(t, dropped, 1)
				assert.Equal(t, "inlineCard", dropped[0].Node)
				assert.True(t, dropped[0].Existing)
			})
		})
	}
}

// TestRestore_UnchangedBlockOfAnEditedContainer checks, for each container,
// that a block put back whole beside an edited sibling is not reported
// again: the read of every container enters the block's quiet scope. A
// table cell is covered by TestRestore_UnchangedCellOfAReshapedTable: two
// paragraphs of one cell read as one, so editing one edits both.
func TestRestore_UnchangedBlockOfAnEditedContainer(t *testing.T) {
	t.Parallel()

	card := `{"type":"inlineCard","attrs":{"data":{"@type":"Document","name":"x"}}}`
	withCard := `{"type":"paragraph","content":[{"type":"text","text":"Card "},` + card + `]}`
	other := `{"type":"paragraph","content":[{"type":"text","text":"Other"}]}`
	item := func(p string) string { return `{"type":"listItem","content":[` + p + `]}` }
	tests := []struct{ name, block string }{
		{"bulletList", `{"type":"bulletList","content":[` + item(withCard) + `,` + item(other) + `]}`},
		{"orderedList", `{"type":"orderedList","attrs":{"order":1},"content":[` + item(withCard) + `,` + item(other) + `]}`},
		{"listItem", `{"type":"bulletList","content":[{"type":"listItem","content":[` + withCard + `,` + other + `]}]}`},
		{"blockquote", `{"type":"blockquote","content":[` + withCard + `,` + other + `]}`},
		{"panel", `{"type":"panel","attrs":{"panelType":"info"},"content":[` + withCard + `,` + other + `]}`},
		{"expand", `{"type":"expand","attrs":{"title":"T"},"content":[` + withCard + `,` + other + `]}`},
		{"taskList", `{"type":"taskList","attrs":{"localId":""},"content":[` +
			`{"type":"taskItem","attrs":{"localId":"","state":"TODO"},"content":[{"type":"text","text":"Card "},` + card + `]},` +
			`{"type":"taskItem","attrs":{"localId":"","state":"TODO"},"content":[{"type":"text","text":"Other"}]}]}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			doc := docOf(rawNode(t, tt.block))
			md, got, issues := readEditWrite(t, doc, func(md string) string { return strings.Replace(md, "Other", "Changed", 1) })
			require.Contains(t, md, "Other")
			assert.Empty(t, jira4claude.DroppedIssues(issues), md)
			assert.Contains(t, jsonOf(t, got), `"@type":"Document"`, "the card stays")
			assert.Contains(t, jsonOf(t, got), "Changed")
		})
	}
}

func TestRestore_DeletedTokenOfIdenticalNodes(t *testing.T) {
	t.Parallel()

	// Two identical mentions: the one kept by its token does not hide the
	// loss of the other, whichever is deleted.
	doc := docOf(paraOf(plainText("a "), mention("u1", "Jo"), plainText(" b "), mention("u1", "Jo")))
	for _, deleted := range []string{"{{mention:1 Jo}}", "{{mention:2 Jo}}"} {
		t.Run(deleted, func(t *testing.T) {
			t.Parallel()
			_, got, issues := readEditWrite(t, doc, func(md string) string { return strings.Replace(md, deleted, "x", 1) })
			assert.Len(t, protectedNodes(got), 1)
			require.Len(t, issues, 1)
			assert.Equal(t, jira4claude.ConversionDropped, issues[0].Kind)
			assert.Equal(t, deleted, issues[0].Node)
			assert.True(t, issues[0].Existing)
		})
	}

	t.Run("an unchanged paragraph gives both back", func(t *testing.T) {
		t.Parallel()
		_, got, issues := readEditWrite(t, doc, func(md string) string { return md + "\n\nmore" })
		assert.Empty(t, issues)
		assert.Len(t, protectedNodes(got), 2)
	})
}

// TestRestore_SameKindReplacement checks that a block removed and another of
// the same type added are paired only where the blocks the edit left
// unchanged around them pin them to the same place (jir-wd4f): elsewhere the
// new block stands as written, and what the removed one held is reported.
func TestRestore_SameKindReplacement(t *testing.T) {
	t.Parallel()

	doc := docOf(rawNode(t, customPanel), paraOf(plainText("text")))

	t.Run("a panel added on the other side of an unchanged block does not take the removed panel's attributes", func(t *testing.T) {
		t.Parallel()
		_, got, issues := readEditWrite(t, doc, func(string) string { return "text\n\n> [!NOTE]\n> Other" })
		require.Len(t, got.Content, 2)
		assert.Equal(t, jsonOf(t, paraOf(plainText("text"))), jsonOf(t, got.Content[0]))
		assert.JSONEq(t, `{"panelType":"info"}`, string(got.Content[1].Attrs), "the new panel as written")
		require.Len(t, issues, 1, "the custom panel's type and colour are lost")
		assert.Equal(t, "panel", issues[0].Node)
		assert.Equal(t, jira4claude.ConversionDegraded, issues[0].Kind)
		assert.True(t, issues[0].Existing)
	})

	t.Run("a panel edited where it was keeps its attributes", func(t *testing.T) {
		t.Parallel()
		_, got, issues := readEditWrite(t, doc, func(md string) string { return strings.Replace(md, "Keep", "Kept", 1) })
		assert.Empty(t, issues)
		assert.JSONEq(t, `{"panelType":"custom","panelColor":"#ABF5D1"}`, string(got.Content[0].Attrs))
	})

	t.Run("a block moved past an unchanged one and edited is not paired", func(t *testing.T) {
		t.Parallel()
		doc := docOf(rawNode(t, centered), paraOf(plainText("middle")), paraOf(plainText("last")))
		_, got, issues := readEditWrite(t, doc, func(string) string { return "middle\n\nSome\n\nlast" })
		require.Len(t, got.Content, 3)
		assert.Empty(t, got.Content[1].Marks, "the moved, edited paragraph as written")
		require.Len(t, issues, 1)
		assert.Equal(t, "alignment", issues[0].Node)
	})

	t.Run("a panel replaced in place while another is added elsewhere is not paired", func(t *testing.T) {
		t.Parallel()
		_, got, issues := readEditWrite(t, doc, func(string) string { return "> [!NOTE]\n> Other\n\ntext\n\n> [!NOTE]\n> More" })
		require.Len(t, got.Content, 3)
		assert.JSONEq(t, `{"panelType":"info"}`, string(got.Content[0].Attrs))
		require.Len(t, issues, 1)
		assert.Equal(t, "panel", issues[0].Node)
	})

	// The limit left: a block replaced by another of the same type between
	// the same unchanged neighbours is taken for the block, edited.
	t.Run("known limit: a panel replaced in place takes the removed panel's attributes", func(t *testing.T) {
		t.Parallel()
		_, got, issues := readEditWrite(t, doc, func(string) string { return "> [!NOTE]\n> Other\n\ntext" })
		assert.Empty(t, issues)
		require.Len(t, got.Content, 2)
		assert.JSONEq(t, `{"panelType":"custom","panelColor":"#ABF5D1"}`, string(got.Content[0].Attrs))
	})
}

// TestRestore_LinkWithACardsURLAddedElsewhere checks that a plain link with
// an inline card's URL, added in another paragraph, leaves the paragraph
// holding the card unchanged (jir-wd4f): the card stays where it was, with
// the inline comment beside it, and the new link stays a link.
func TestRestore_LinkWithACardsURLAddedElsewhere(t *testing.T) {
	t.Parallel()

	original := rawNode(t, `{"type":"paragraph","content":[`+
		`{"type":"text","text":"Alpha","marks":[{"type":"annotation","attrs":{"id":"a-1","annotationType":"inlineComment"}}]},`+
		`{"type":"text","text":" "},{"type":"inlineCard","attrs":{"url":"https://a.test/x"}}]}`)
	doc := docOf(original)
	_, got, issues := readEditWrite(t, doc, func(md string) string {
		return md + "\n\nSee [https://a.test/x](https://a.test/x)"
	})
	assert.Empty(t, issues)
	require.Len(t, got.Content, 2)
	assert.Equal(t, jsonOf(t, original), jsonOf(t, got.Content[0]), "the paragraph as Jira held it")
	link := `{"type":"paragraph","content":[{"type":"text","text":"See "},` +
		`{"type":"text","text":"https://a.test/x","marks":[{"type":"link","attrs":{"href":"https://a.test/x"}}]}]}`
	assert.JSONEq(t, link, jsonOf(t, got.Content[1]), "the new link as written")
}

// TestRestore_SwappedTokensOfMisplacedKnownNodes checks that a node of a
// type the reader knows, written as a token because of where it sits, is
// compared and restored as its token carries it, localId included
// (jir-wd4f).
func TestRestore_SwappedTokensOfMisplacedKnownNodes(t *testing.T) {
	t.Parallel()

	p1 := rawNode(t, `{"type":"paragraph","attrs":{"localId":"p-1"},"content":[{"type":"text","text":"x"}]}`)
	p2 := rawNode(t, `{"type":"paragraph","attrs":{"localId":"p-2"},"content":[{"type":"text","text":"x"}]}`)

	t.Run("swapped, they come back swapped", func(t *testing.T) {
		t.Parallel()
		doc := docOf(jira4claude.ADFNode{Type: "bulletList", Content: []jira4claude.ADFNode{p1, p2}})
		md, got, issues := readEditWrite(t, doc, func(md string) string {
			md = strings.Replace(md, "{{paragraph:1}}", "\x00", 1)
			md = strings.Replace(md, "{{paragraph:2}}", "{{paragraph:1}}", 1)
			return strings.Replace(md, "\x00", "{{paragraph:2}}", 1)
		})
		require.Equal(t, "- {{paragraph:1}}\n- {{paragraph:2}}", md)
		assert.Empty(t, jira4claude.DroppedIssues(issues))
		assert.Equal(t, []string{"p-2", "p-1"}, collectAttr(got, "localId"))
	})

	t.Run("moved to where a paragraph was, it is not merged with that paragraph", func(t *testing.T) {
		t.Parallel()
		pa := rawNode(t, `{"type":"paragraph","attrs":{"localId":"p-a"},"marks":[{"type":"alignment","attrs":{"align":"center"}}],"content":[{"type":"text","text":"a"}]}`)
		doc := docOf(
			jira4claude.ADFNode{Type: "bulletList", Content: []jira4claude.ADFNode{{Type: "listItem", Content: []jira4claude.ADFNode{pa}}}},
			jira4claude.ADFNode{Type: "orderedList", Attrs: json.RawMessage(`{"order":1}`), Content: []jira4claude.ADFNode{p1}},
		)
		md, got, issues := readEditWrite(t, doc, func(string) string { return "- {{paragraph:1}}\n\n1. a" })
		require.Equal(t, "- a\n\n1. {{paragraph:1}}", md)
		assert.Empty(t, jira4claude.DroppedIssues(issues))
		require.Len(t, got.Content, 2)
		require.Len(t, got.Content[0].Content, 1)
		require.Len(t, got.Content[0].Content[0].Content, 1)
		assert.Equal(t, jsonOf(t, p1), jsonOf(t, got.Content[0].Content[0].Content[0]), "the paragraph as its token carries it")
	})
}

// TestRestore_EmptyParagraphLeadingAListItem checks that the warning about
// an empty paragraph at the start of a list item, which Markdown cannot
// show, follows that paragraph (jir-wd4f): it is not given when the
// paragraph is put back, and is when it is not.
func TestRestore_EmptyParagraphLeadingAListItem(t *testing.T) {
	t.Parallel()

	doc := docOf(jira4claude.ADFNode{Type: "bulletList", Content: []jira4claude.ADFNode{{Type: "listItem", Content: []jira4claude.ADFNode{
		paraOf(), paraOf(plainText("same")), paraOf(plainText("other")),
	}}}})

	t.Run("put back beside an unchanged paragraph, it is not reported", func(t *testing.T) {
		t.Parallel()
		_, got, issues := readEditWrite(t, doc, func(md string) string { return strings.Replace(md, "other", "changed", 1) })
		assert.Empty(t, issues)
		want := docOf(jira4claude.ADFNode{Type: "bulletList", Content: []jira4claude.ADFNode{{Type: "listItem", Content: []jira4claude.ADFNode{
			paraOf(), paraOf(plainText("same")), paraOf(plainText("changed")),
		}}}})
		assert.Equal(t, jsonOf(t, want), jsonOf(t, got))
	})

	t.Run("left out, it is reported", func(t *testing.T) {
		t.Parallel()
		_, got, issues := readEditWrite(t, doc, func(md string) string {
			return strings.Replace(strings.Replace(md, "other", "changed", 1), "same", "altered", 1)
		})
		assert.Len(t, got.Content[0].Content[0].Content, 2)
		require.NotEmpty(t, issues)
		for _, i := range issues {
			assert.Equal(t, jira4claude.ConversionDegraded, i.Kind)
		}
	})
}

// collectAttr returns the values of attribute key in doc, in document order.
func collectAttr(doc *jira4claude.ADFNode, key string) []string {
	var out []string
	var walk func(n *jira4claude.ADFNode)
	walk = func(n *jira4claude.ADFNode) {
		if n.Attrs != nil {
			var attrs map[string]any
			if json.Unmarshal(n.Attrs, &attrs) == nil {
				if v, ok := attrs[key].(string); ok {
					out = append(out, v)
				}
			}
		}
		for i := range n.Content {
			walk(&n.Content[i])
		}
	}
	walk(doc)
	return out
}
