package markdown_test

import (
	"encoding/json"
	"testing"

	"github.com/fwojciec/jira4claude"
	"github.com/fwojciec/jira4claude/markdown"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// emptyPara builds an ADF paragraph with no content, the placeholder an empty
// list item holds.
func emptyPara() jira4claude.ADFNode {
	return jira4claude.ADFNode{Type: "paragraph"}
}

// infoPanel builds an ADF info panel holding the given block children.
func infoPanel(t *testing.T, children ...jira4claude.ADFNode) jira4claude.ADFNode {
	t.Helper()
	attrs, err := json.Marshal(map[string]any{"panelType": "info"})
	require.NoError(t, err)
	return jira4claude.ADFNode{Type: "panel", Attrs: attrs, Content: children}
}

// TestToMarkdown_AdjacentLists covers sibling lists of the same kind. They
// are separate ADF nodes, but Markdown merges adjacent lists that use the
// same marker, so each list uses a marker its neighbour does not: bullets
// alternate between - and *, ordered delimiters between . and ) (#158).
func TestToMarkdown_AdjacentLists(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		doc  func(t *testing.T) *jira4claude.ADFNode
		want string
	}{
		{
			name: "two bullet lists",
			doc: func(t *testing.T) *jira4claude.ADFNode {
				t.Helper()
				return doc(bullets(item(para("a"))), bullets(item(para("b"))))
			},
			want: "- a\n\n* b",
		},
		{
			name: "four bullet lists alternate markers",
			doc: func(t *testing.T) *jira4claude.ADFNode {
				t.Helper()
				return doc(bullets(item(para("a"))), bullets(item(para("b"))),
					bullets(item(para("c"))), bullets(item(para("d"))))
			},
			want: "- a\n\n* b\n\n- c\n\n* d",
		},
		{
			name: "bullet list after a task list",
			doc: func(t *testing.T) *jira4claude.ADFNode {
				t.Helper()
				return doc(taskList(taskItem(t, "DONE", txt("a"))), bullets(item(para("b"))))
			},
			want: "- [x] a\n\n* b",
		},
		{
			name: "task list after a bullet list",
			doc: func(t *testing.T) *jira4claude.ADFNode {
				t.Helper()
				return doc(bullets(item(para("a"))), taskList(taskItem(t, "TODO", txt("b"))))
			},
			want: "- a\n\n* [ ] b",
		},
		{
			name: "two task lists",
			doc: func(t *testing.T) *jira4claude.ADFNode {
				t.Helper()
				return doc(taskList(taskItem(t, "TODO", txt("a"))), taskList(taskItem(t, "TODO", txt("b"))))
			},
			want: "- [ ] a\n\n* [ ] b",
		},
		{
			name: "two ordered lists",
			doc: func(t *testing.T) *jira4claude.ADFNode {
				t.Helper()
				return doc(numbered(t, 1, item(para("a"))), numbered(t, 5, item(para("b"))))
			},
			want: "1. a\n\n5) b",
		},
		{
			name: "three ordered lists",
			doc: func(t *testing.T) *jira4claude.ADFNode {
				t.Helper()
				return doc(numbered(t, 1, item(para("a"))), numbered(t, 1, item(para("b"))),
					numbered(t, 1, item(para("c"))))
			},
			want: "1. a\n\n1) b\n\n1. c",
		},
		{
			name: "lists of different kinds keep their default markers",
			doc: func(t *testing.T) *jira4claude.ADFNode {
				t.Helper()
				return doc(bullets(item(para("a"))), numbered(t, 1, item(para("b"))), bullets(item(para("c"))))
			},
			want: "- a\n\n1. b\n\n- c",
		},
		{
			name: "lists separated by a paragraph keep their default markers",
			doc: func(t *testing.T) *jira4claude.ADFNode {
				t.Helper()
				return doc(bullets(item(para("a"))), para("p"), bullets(item(para("b"))))
			},
			want: "- a\n\np\n\n- b",
		},
		{
			name: "two bullet lists in a list item",
			doc: func(t *testing.T) *jira4claude.ADFNode {
				t.Helper()
				return doc(bullets(item(para("a"), bullets(item(para("b"))), bullets(item(para("c"))))))
			},
			want: "- a\n\n  - b\n\n  * c",
		},
		{
			name: "two ordered lists in an ordered item",
			doc: func(t *testing.T) *jira4claude.ADFNode {
				t.Helper()
				return doc(numbered(t, 1, item(para("a"), numbered(t, 1, item(para("b"))), numbered(t, 1, item(para("c"))))))
			},
			want: "1. a\n\n   1. b\n\n   1) c",
		},
		{
			name: "two bullet lists in a blockquote",
			doc: func(t *testing.T) *jira4claude.ADFNode {
				t.Helper()
				return doc(quote(bullets(item(para("a"))), bullets(item(para("b")))))
			},
			want: "> - a\n>\n> * b",
		},
		{
			name: "two bullet lists in a panel",
			doc: func(t *testing.T) *jira4claude.ADFNode {
				t.Helper()
				return doc(infoPanel(t, bullets(item(para("a"))), bullets(item(para("b")))))
			},
			want: "> [!NOTE]\n> - a\n>\n> * b",
		},
		{
			name: "two bullet lists in an expand",
			doc: func(t *testing.T) *jira4claude.ADFNode {
				t.Helper()
				return doc(expandNode(t, "T", bullets(item(para("a"))), bullets(item(para("b")))))
			},
			want: "<details><summary>T</summary>\n\n- a\n\n* b\n\n</details>",
		},
		{
			name: "two nested task lists after a task item",
			doc: func(t *testing.T) *jira4claude.ADFNode {
				t.Helper()
				return doc(taskList(
					taskItem(t, "TODO", txt("a")),
					taskList(taskItem(t, "TODO", txt("b"))),
					taskList(taskItem(t, "TODO", txt("c"))),
				))
			},
			want: "- [ ] a\n  - [ ] b\n  * [ ] c",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			md := assertRoundTrip(t, tt.doc(t))
			assert.Equal(t, tt.want, md)
		})
	}

	t.Run("a block that renders empty does not separate two lists", func(t *testing.T) {
		t.Parallel()

		md, issues := markdown.New().ToMarkdown(doc(
			bullets(item(para("a"))),
			jira4claude.ADFNode{Type: "extension"},
			bullets(item(para("b"))),
		))

		assert.Equal(t, []jira4claude.ConversionIssue{droppedIssue("extension")}, issues)
		assert.Equal(t, "- a\n\n* b", md)
	})
}

// TestToMarkdown_EmptyListItem covers list items with no content of their
// own: an empty item holds only an empty paragraph, and an item that opens
// with a list holds an empty paragraph before it. Each has a Markdown form
// that reads back as the same ADF (#158).
func TestToMarkdown_EmptyListItem(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		doc  func(t *testing.T) *jira4claude.ADFNode
		want string
	}{
		{
			name: "lone empty item",
			doc: func(t *testing.T) *jira4claude.ADFNode {
				t.Helper()
				return doc(bullets(item(emptyPara())))
			},
			want: "-",
		},
		{
			name: "empty item between items",
			doc: func(t *testing.T) *jira4claude.ADFNode {
				t.Helper()
				return doc(bullets(item(para("a")), item(emptyPara()), item(para("c"))))
			},
			want: "- a\n-\n- c",
		},
		{
			name: "empty ordered item",
			doc: func(t *testing.T) *jira4claude.ADFNode {
				t.Helper()
				return doc(numbered(t, 1, item(emptyPara()), item(para("b"))))
			},
			want: "1.\n2. b",
		},
		{
			name: "item opening with a list",
			doc: func(t *testing.T) *jira4claude.ADFNode {
				t.Helper()
				return doc(bullets(item(emptyPara(), bullets(item(para("a"))))))
			},
			want: "- * a",
		},
		{
			name: "three nested empty items are not a thematic break",
			doc: func(t *testing.T) *jira4claude.ADFNode {
				t.Helper()
				return doc(bullets(item(emptyPara(), bullets(item(emptyPara(), bullets(item(emptyPara())))))))
			},
			want: "- * -",
		},
		{
			name: "three nested empty items in an ordered item",
			doc: func(t *testing.T) *jira4claude.ADFNode {
				t.Helper()
				inner := bullets(item(emptyPara(), bullets(item(emptyPara()))))
				return doc(numbered(t, 1, item(emptyPara(), inner)))
			},
			want: "1. - *",
		},
		{
			name: "item opening with two lists",
			doc: func(t *testing.T) *jira4claude.ADFNode {
				t.Helper()
				return doc(bullets(item(emptyPara(), bullets(item(para("a"))), bullets(item(para("b"))))))
			},
			want: "- * a\n\n  - b",
		},
		{
			name: "empty nested item before parent content",
			doc: func(t *testing.T) *jira4claude.ADFNode {
				t.Helper()
				return doc(bullets(item(emptyPara(), numbered(t, 0, item(emptyPara())), para("0"))))
			},
			want: "- 0.\n  0",
		},
		{
			name: "empty nested item before a code block",
			doc: func(t *testing.T) *jira4claude.ADFNode {
				t.Helper()
				return doc(bullets(item(emptyPara(), bullets(item(emptyPara())), code("c"))))
			},
			want: "- *\n  ```\n  c\n  ```",
		},
		{
			name: "empty nested item after a paragraph, before more content",
			doc: func(t *testing.T) *jira4claude.ADFNode {
				t.Helper()
				return doc(bullets(item(para("a"), bullets(item(emptyPara())), para("x"))))
			},
			want: "- a\n\n  -\n  x",
		},
		{
			name: "empty item deep in nested lists, before parent content",
			doc: func(t *testing.T) *jira4claude.ADFNode {
				t.Helper()
				return doc(bullets(item(
					para("a"),
					bullets(item(para("b"), bullets(item(emptyPara())))),
					para("x"),
				)))
			},
			want: "- a\n\n  - b\n\n    -\n  x",
		},
		{
			name: "empty nested item before a sibling list",
			doc: func(t *testing.T) *jira4claude.ADFNode {
				t.Helper()
				return doc(bullets(item(para("a"), bullets(item(emptyPara())), bullets(item(para("b"))))))
			},
			want: "- a\n\n  -\n  * b",
		},
		{
			name: "empty item before a paragraph in a blockquote",
			doc: func(t *testing.T) *jira4claude.ADFNode {
				t.Helper()
				return doc(quote(bullets(item(emptyPara())), para("x")))
			},
			want: "> -\n>\n> x",
		},
		{
			name: "empty item before a paragraph in a panel",
			doc: func(t *testing.T) *jira4claude.ADFNode {
				t.Helper()
				return doc(infoPanel(t, bullets(item(emptyPara())), para("x")))
			},
			want: "> [!NOTE]\n>\n> -\n>\n> x",
		},
		{
			name: "empty item before a paragraph in an expand",
			doc: func(t *testing.T) *jira4claude.ADFNode {
				t.Helper()
				return doc(expandNode(t, "T", bullets(item(emptyPara())), para("x")))
			},
			want: "<details><summary>T</summary>\n\n-\n\nx\n\n</details>",
		},
		{
			name: "text like an ordered marker of ten digits is not an empty item",
			doc: func(t *testing.T) *jira4claude.ADFNode {
				t.Helper()
				return doc(bullets(item(para("0000000000."), para("0"))))
			},
			want: "- 0000000000.\n\n  0",
		},
		{
			name: "empty item before a top-level paragraph",
			doc: func(t *testing.T) *jira4claude.ADFNode {
				t.Helper()
				return doc(bullets(item(emptyPara())), para("x"))
			},
			want: "-\n\nx",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			md := assertRoundTrip(t, tt.doc(t))
			assert.Equal(t, tt.want, md)
		})
	}

	t.Run("a line of marker-like text after a hard break is not an empty item", func(t *testing.T) {
		t.Parallel()

		for _, marker := range []string{"-", "+", "*", "2.", "1)", "+ -", "1. 2)"} {
			broken := jira4claude.ADFNode{Type: "paragraph", Content: []jira4claude.ADFNode{txt("a"), hardBreak(), txt(marker)}}
			assertRoundTrip(t, doc(bullets(item(broken, para("x")))))
			assertRoundTrip(t, doc(bullets(item(para("a"), bullets(item(para("b")), item(broken)), para("x")))))
		}
	})

	t.Run("empty paragraph before other content has no Markdown form", func(t *testing.T) {
		t.Parallel()

		for _, tc := range []struct {
			content []jira4claude.ADFNode
			want    string
		}{
			{[]jira4claude.ADFNode{emptyPara(), para("x")}, "- x"},
			{[]jira4claude.ADFNode{emptyPara(), code("c")}, "- ```\n  c\n  ```"},
		} {
			md, issues := markdown.New().ToMarkdown(doc(bullets(item(tc.content...))))

			require.Len(t, issues, 1, tc.want)
			assert.Equal(t, jira4claude.ConversionDegraded, issues[0].Kind, tc.want)
			assert.Equal(t, "listItem", issues[0].Node, tc.want)
			assert.Equal(t, tc.want, md)
		}
	})
}
