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

// TestRestore_KnownLimit pins what the matching cannot see: an edit that
// removes a node and adds another of the same type in the same container
// gives the new node the removed one's attributes, without a report.
func TestRestore_KnownLimit(t *testing.T) {
	t.Parallel()

	doc := docOf(rawNode(t, customPanel), paraOf(plainText("text")))
	_, got, issues := readEditWrite(t, doc, func(string) string { return "text\n\n> [!NOTE]\n> Other" })
	assert.Empty(t, issues)
	require.Len(t, got.Content, 2)
	assert.JSONEq(t, `{"panelType":"custom","panelColor":"#ABF5D1"}`, string(got.Content[1].Attrs),
		"the new panel takes the removed panel's attributes")
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
