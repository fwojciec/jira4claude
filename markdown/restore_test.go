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

func TestRestore_ProtectedNodeOfAnUnchangedPart(t *testing.T) {
	t.Parallel()

	// A token that does not read back would be lost by an edit, but an
	// unchanged rewrite gives its node back, so nothing is reported.
	doc := docOf(paraOf(plainText("a "), mention("u1", "Ann")), paraOf(plainText("b")))
	_, got, issues := readEditWrite(t, doc, func(md string) string { return strings.Replace(md, "b", "c", 1) })
	assert.Empty(t, issues)
	assert.Equal(t, jsonOf(t, docOf(paraOf(plainText("a "), mention("u1", "Ann")), paraOf(plainText("c")))), jsonOf(t, got))
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
