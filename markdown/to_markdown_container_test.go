package markdown_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/fwojciec/jira4claude"
	"github.com/fwojciec/jira4claude/markdown"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// item builds an ADF listItem holding the given block children.
func item(children ...jira4claude.ADFNode) jira4claude.ADFNode {
	return jira4claude.ADFNode{Type: "listItem", Content: children}
}

// bullets builds an ADF bulletList of the given list items.
func bullets(items ...jira4claude.ADFNode) jira4claude.ADFNode {
	return jira4claude.ADFNode{Type: "bulletList", Content: items}
}

// numbered builds an ADF orderedList starting at start, with the order attr
// set only when start is not 1, as the write side does.
func numbered(t *testing.T, start int, items ...jira4claude.ADFNode) jira4claude.ADFNode {
	t.Helper()
	list := jira4claude.ADFNode{Type: "orderedList", Content: items}
	if start != 1 {
		attrs, err := json.Marshal(map[string]any{"order": start})
		require.NoError(t, err)
		list.Attrs = attrs
	}
	return list
}

// quote builds an ADF blockquote holding the given block children.
func quote(children ...jira4claude.ADFNode) jira4claude.ADFNode {
	return jira4claude.ADFNode{Type: "blockquote", Content: children}
}

// code builds an ADF codeBlock with no language.
func code(text string) jira4claude.ADFNode {
	return jira4claude.ADFNode{Type: "codeBlock", Content: []jira4claude.ADFNode{txt(text)}}
}

// withLocalIDs returns a copy of node with a localId on every taskList and
// taskItem, so hand-built task lists validate against the ADF schema.
func withLocalIDs(t *testing.T, node jira4claude.ADFNode) jira4claude.ADFNode {
	t.Helper()
	switch node.Type {
	case "taskList":
		attrs, err := json.Marshal(map[string]any{"localId": "list"})
		require.NoError(t, err)
		node.Attrs = attrs
	case "taskItem":
		var attrs map[string]any
		require.NoError(t, json.Unmarshal(node.Attrs, &attrs))
		attrs["localId"] = "item"
		withID, err := json.Marshal(attrs)
		require.NoError(t, err)
		node.Attrs = withID
	}
	if node.Content != nil {
		content := make([]jira4claude.ADFNode, len(node.Content))
		for i, child := range node.Content {
			content[i] = withLocalIDs(t, child)
		}
		node.Content = content
	}
	return node
}

// assertRoundTrip renders original to Markdown, converts that back to ADF,
// and asserts the result is valid and identical to original, ignoring task
// list localIds. It returns the intermediate Markdown.
func assertRoundTrip(t *testing.T, original *jira4claude.ADFNode) string {
	t.Helper()
	input := withLocalIDs(t, *original)
	requireValidADF(t, &input)

	converter := markdown.New()
	md, mdIssues := converter.ToMarkdown(&input)
	require.Empty(t, mdIssues)

	back, issues := converter.ToADF(md)
	assert.Empty(t, issues, "markdown read back: %q", md)
	requireValidADF(t, back)
	assert.Equal(t, stripLocalIDs(t, input), stripLocalIDs(t, *back), "markdown read back: %q", md)
	return md
}

// TestToMarkdown_ContainerBlockRoundTrip asserts that block content inside
// list items, blockquotes, panels, and task lists keeps its boundaries when
// read as Markdown and written back: ADF to Markdown to ADF is identical.
func TestToMarkdown_ContainerBlockRoundTrip(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		doc  func(t *testing.T) *jira4claude.ADFNode
	}{
		{
			name: "bullet item with two paragraphs",
			doc: func(t *testing.T) *jira4claude.ADFNode {
				t.Helper()
				return doc(bullets(item(para("item"), para("second para")), item(para("next"))))
			},
		},
		{
			name: "ordered item with two paragraphs",
			doc: func(t *testing.T) *jira4claude.ADFNode {
				t.Helper()
				return doc(numbered(t, 1, item(para("item"), para("second para")), item(para("next"))))
			},
		},
		{
			name: "two-digit ordered item with two paragraphs",
			doc: func(t *testing.T) *jira4claude.ADFNode {
				t.Helper()
				return doc(numbered(t, 10, item(para("item"), para("second para")), item(para("next"))))
			},
		},
		{
			name: "two-digit ordered item with an indented code block",
			doc: func(t *testing.T) *jira4claude.ADFNode {
				t.Helper()
				return doc(numbered(t, 10, item(para("item"), code("x := 1\n\ny := 2"))))
			},
		},
		{
			name: "bullet item whose first child is a multi-line literal HTML paragraph",
			doc: func(t *testing.T) *jira4claude.ADFNode {
				t.Helper()
				return doc(bullets(item(literalLines("<div>", "hello", "</div>")), item(para("next"))))
			},
		},
		{
			name: "ordered item whose first child is a multi-line literal HTML paragraph",
			doc: func(t *testing.T) *jira4claude.ADFNode {
				t.Helper()
				return doc(numbered(t, 9, item(literalLines("<div>", "hello", "</div>")), item(para("next"))))
			},
		},
		{
			name: "blockquote with a paragraph, then a literal tag and bold-looking text",
			doc: func(t *testing.T) *jira4claude.ADFNode {
				t.Helper()
				return doc(quote(para("quote"), literalLines("<word>", "**text**")))
			},
		},
		{
			name: "blockquote with two paragraphs",
			doc: func(t *testing.T) *jira4claude.ADFNode {
				t.Helper()
				return doc(quote(para("one"), para("two")))
			},
		},
		{
			name: "list item containing a code block",
			doc: func(t *testing.T) *jira4claude.ADFNode {
				t.Helper()
				return doc(bullets(item(para("item"), code("a\n\nb")), item(para("next"))))
			},
		},
		{
			name: "list item containing a nested list",
			doc: func(t *testing.T) *jira4claude.ADFNode {
				t.Helper()
				return doc(bullets(
					item(para("parent"), numbered(t, 3, item(para("child")), item(para("child two")))),
					item(para("next")),
				))
			},
		},
		{
			name: "list item with a paragraph after a nested list",
			doc: func(t *testing.T) *jira4claude.ADFNode {
				t.Helper()
				return doc(bullets(item(para("parent"), bullets(item(para("child"))), para("after"))))
			},
		},
		{
			name: "blockquote containing a list with multi-paragraph items",
			doc: func(t *testing.T) *jira4claude.ADFNode {
				t.Helper()
				return doc(quote(para("intro"), bullets(item(para("a"), para("b")), item(para("c")))))
			},
		},
		{
			name: "task list containing a nested task list",
			doc: func(t *testing.T) *jira4claude.ADFNode {
				t.Helper()
				return doc(taskList(
					taskItem(t, "TODO", txt("parent")),
					taskList(taskItem(t, "DONE", txt("child one")), taskItem(t, "TODO", txt("child two"))),
					taskItem(t, "TODO", txt("next")),
				))
			},
		},
		{
			name: "task list nested two levels deep",
			doc: func(t *testing.T) *jira4claude.ADFNode {
				t.Helper()
				return doc(taskList(
					taskItem(t, "TODO", txt("a")),
					taskList(
						taskItem(t, "TODO", txt("b")),
						taskList(taskItem(t, "DONE", txt("c"))),
					),
				))
			},
		},
		{
			name: "panel with two paragraphs",
			doc: func(t *testing.T) *jira4claude.ADFNode {
				t.Helper()
				attrs, err := json.Marshal(map[string]any{"panelType": "info"})
				require.NoError(t, err)
				return doc(jira4claude.ADFNode{Type: "panel", Attrs: attrs, Content: []jira4claude.ADFNode{
					para("one"), para("two"),
				}})
			},
		},
		{
			name: "expand holding a list with multi-paragraph items",
			doc: func(t *testing.T) *jira4claude.ADFNode {
				t.Helper()
				return doc(expandNode(t, "T", bullets(item(para("a"), para("b")))))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assertRoundTrip(t, tt.doc(t))
		})
	}
}

// TestToMarkdown_ContainerContinuationLines asserts that every line after a
// line break inside a container child carries the container's indentation or
// quote prefix, whatever the line break renders as, so the text stays inside
// its container.
func TestToMarkdown_ContainerContinuationLines(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		doc    func(t *testing.T) *jira4claude.ADFNode
		prefix string
	}{
		{
			name: "bullet item",
			doc: func(t *testing.T) *jira4claude.ADFNode {
				t.Helper()
				return doc(bullets(item(literalLines("one", "two", "three"))))
			},
			prefix: "  ",
		},
		{
			name: "ordered item continuation matches the marker width",
			doc: func(t *testing.T) *jira4claude.ADFNode {
				t.Helper()
				return doc(numbered(t, 1, item(literalLines("one", "two"))))
			},
			prefix: "   ",
		},
		{
			name: "two-digit ordered item continuation matches the marker width",
			doc: func(t *testing.T) *jira4claude.ADFNode {
				t.Helper()
				return doc(numbered(t, 10, item(literalLines("one", "two"))))
			},
			prefix: "    ",
		},
		{
			name: "task item text with a hard break",
			doc: func(t *testing.T) *jira4claude.ADFNode {
				t.Helper()
				return doc(taskList(taskItem(t, "TODO", txt("task"), hardBreak(), txt("- [x] example"))))
			},
			prefix: "  ",
		},
		{
			name: "blockquote",
			doc: func(t *testing.T) *jira4claude.ADFNode {
				t.Helper()
				return doc(quote(literalLines("one", "two")))
			},
			prefix: ">",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			input := withLocalIDs(t, *tt.doc(t))
			md, issues := markdown.New().ToMarkdown(&input)
			require.Empty(t, issues)

			lines := strings.Split(md, "\n")
			require.Greater(t, len(lines), 1, "markdown: %q", md)
			for _, line := range lines[1:] {
				assert.True(t, strings.HasPrefix(line, tt.prefix), "line %q lacks prefix %q in %q", line, tt.prefix, md)
			}
		})
	}
}

// TestToMarkdown_TightListUnchanged guards the common case: a list of
// single-paragraph items renders with no blank lines between items.
func TestToMarkdown_TightListUnchanged(t *testing.T) {
	t.Parallel()

	t.Run("bullet list", func(t *testing.T) {
		t.Parallel()
		md, issues := markdown.New().ToMarkdown(doc(bullets(item(para("one")), item(para("two")), item(para("three")))))
		require.Empty(t, issues)
		assert.Equal(t, "- one\n- two\n- three", md)
	})

	t.Run("ordered list", func(t *testing.T) {
		t.Parallel()
		md, issues := markdown.New().ToMarkdown(doc(numbered(t, 9, item(para("nine")), item(para("ten")))))
		require.Empty(t, issues)
		assert.Equal(t, "9. nine\n10. ten", md)
	})

	t.Run("task list", func(t *testing.T) {
		t.Parallel()
		input := withLocalIDs(t, *doc(taskList(taskItem(t, "TODO", txt("one")), taskItem(t, "DONE", txt("two")))))
		md, issues := markdown.New().ToMarkdown(&input)
		require.Empty(t, issues)
		assert.Equal(t, "- [ ] one\n- [x] two", md)
	})
}

// TestToMarkdown_ContainerEdgeCases covers containers with nothing to nest
// under or nothing to render.
func TestToMarkdown_ContainerEdgeCases(t *testing.T) {
	t.Parallel()

	t.Run("empty blockquote renders nothing", func(t *testing.T) {
		t.Parallel()
		md, issues := markdown.New().ToMarkdown(doc(quote(), para("after")))
		require.Empty(t, issues)
		assert.Equal(t, "after", md)
	})

	t.Run("task list nested before any task item is rendered at its parent's level and reported", func(t *testing.T) {
		t.Parallel()
		md, issues := markdown.New().ToMarkdown(doc(taskList(
			taskList(taskItem(t, "TODO", txt("orphan"))),
			taskItem(t, "DONE", txt("item")),
		)))
		assert.Equal(t, "- [ ] orphan\n- [x] item", md)
		assert.Equal(t, []jira4claude.ConversionIssue{{
			Kind:   jira4claude.ConversionDegraded,
			Node:   "taskList",
			Detail: "taskList nested before any taskItem; rendered at its parent's level",
		}}, issues)
	})
}

// TestToADF_BlockquoteWithOnlyDroppedChild asserts that a blockquote whose
// only child is dropped on write is left out rather than emitted empty,
// which the ADF schema rejects.
func TestToADF_BlockquoteWithOnlyDroppedChild(t *testing.T) {
	t.Parallel()

	result, issues := markdown.New().ToADF("> <details><summary>T</summary>\n> body\n> </details>")

	requireValidADF(t, result)
	assert.Empty(t, result.Content)
	assert.Equal(t, []jira4claude.ConversionIssue{droppedIssue("HTMLBlock")}, issues)
}

// TestRoundTrip_CodeBlockBlankOnlyLineInContainer asserts that a code block
// line made only of blanks keeps them inside every container (#163). Inside a
// list item the Markdown parser drops a blank line's indentation along with
// its content, so the write side takes the blanks back from the source.
func TestRoundTrip_CodeBlockBlankOnlyLineInContainer(t *testing.T) {
	t.Parallel()

	containers := []struct {
		name string
		wrap func(t *testing.T, block jira4claude.ADFNode) *jira4claude.ADFNode
	}{
		{"bullet item", func(t *testing.T, block jira4claude.ADFNode) *jira4claude.ADFNode {
			t.Helper()
			return doc(bullets(item(para("item"), block)))
		}},
		{"bullet item opening with the code block", func(t *testing.T, block jira4claude.ADFNode) *jira4claude.ADFNode {
			t.Helper()
			return doc(bullets(item(block)))
		}},
		{"two-digit ordered item", func(t *testing.T, block jira4claude.ADFNode) *jira4claude.ADFNode {
			t.Helper()
			return doc(numbered(t, 10, item(para("item"), block)))
		}},
		{"nested bullet item", func(t *testing.T, block jira4claude.ADFNode) *jira4claude.ADFNode {
			t.Helper()
			return doc(bullets(item(para("outer"), bullets(item(para("inner"), block)))))
		}},
		{"bullet item in a blockquote", func(t *testing.T, block jira4claude.ADFNode) *jira4claude.ADFNode {
			t.Helper()
			return doc(quote(bullets(item(para("item"), block))))
		}},
		{"bullet item in an expand", func(t *testing.T, block jira4claude.ADFNode) *jira4claude.ADFNode {
			t.Helper()
			return doc(expandNode(t, "T", bullets(item(para("item"), block))))
		}},
		{"blockquote", func(t *testing.T, block jira4claude.ADFNode) *jira4claude.ADFNode {
			t.Helper()
			return doc(quote(block))
		}},
		{"expand", func(t *testing.T, block jira4claude.ADFNode) *jira4claude.ADFNode {
			t.Helper()
			return doc(expandNode(t, "T", block))
		}},
	}
	texts := []string{" ", "  ", "     ", "a\n \nb", "a\n   \n\n  \nb", "\t", "a\n \t \nb", "  \na"}

	for _, c := range containers {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			for _, text := range texts {
				original := c.wrap(t, code(text))
				conv := markdown.New()
				md, readIssues := conv.ToMarkdown(original)
				back, writeIssues := conv.ToADF(md)

				assert.Empty(t, readIssues, "code %q", text)
				assert.Empty(t, writeIssues, "code %q", text)
				assert.Equal(t, original, back, "code %q read as %q", text, md)
			}
		})
	}
}

// A panel takes no code block on write, so its Markdown is checked directly:
// the code lines keep their blanks under the quote prefix.
func TestToMarkdown_CodeBlockBlankOnlyLineInPanel(t *testing.T) {
	t.Parallel()

	attrs, err := json.Marshal(map[string]any{"panelType": "info"})
	require.NoError(t, err)
	panel := doc(jira4claude.ADFNode{Type: "panel", Attrs: attrs, Content: []jira4claude.ADFNode{code("a\n  \nb")}})

	md, issues := markdown.New().ToMarkdown(panel)

	assert.Empty(t, issues)
	assert.Equal(t, "> [!NOTE]\n> ```\n> a\n>   \n> b\n> ```", md)
}

// TestToADF_CodeBlockBlankOnlyLineInListItem asserts that the blanks of a
// code line past the opening fence's column are kept inside a list item,
// where the Markdown parser drops a blank line whole (#163).
func TestToADF_CodeBlockBlankOnlyLineInListItem(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		markdown string
		code     string
	}{
		{"blanks past the fence column", "- item\n\n  ```\n     \n  ```", "   "},
		{"indentation only", "- item\n\n  ```\n  a\n  \n  b\n  ```", "a\n\nb"},
		{"less than the indentation", "- item\n\n  ```\n  a\n \n  b\n  ```", "a\n\nb"},
		{"no indentation", "- item\n\n  ```\n  a\n\n  b\n  ```", "a\n\nb"},
		{"fence on the marker line", "- ```\n   \n  ```", " "},
		{"indented fence", "- item\n\n    ```\n      \n    ```", "  "},
		{"tab across the fence column", "- ```\n\t \n  ```", "   "},
		{"list in a blockquote", "> - ```\n>     \n>   ```", "  "},
		{"quote marker indented on later lines", "> - ~~~\n >   \n >   x\n >   ~~~", "\nx"},
		{"quote marker indented further on later lines", ">  - ~~~\n  >      \n  >    ~~~", "  "},
		{"nested quotes", "> > - ~~~\n> >      \n> >   ~~~", "   "},
		{"nested quotes without spaces", ">> - ~~~\n>>     \n>>   ~~~", "  "},
		{"list in a quote in a list", "- > - ~~~\n  >      \n  >   ~~~", "   "},
		{"tab after the quote marker", "> - ~~~\n>\t   \n>   ~~~", "   "},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Only the code text is checked: some of these containers are
			// reshaped on write (a blockquote in a blockquote is unwrapped),
			// and that is reported.
			adf, _ := markdown.New().ToADF(tt.markdown)

			var found []string
			var walk func(n jira4claude.ADFNode)
			walk = func(n jira4claude.ADFNode) {
				if n.Type == "codeBlock" {
					text := ""
					for _, c := range n.Content {
						text += c.Text
					}
					found = append(found, text)
				}
				for _, c := range n.Content {
					walk(c)
				}
			}
			walk(*adf)
			assert.Equal(t, []string{tt.code}, found)
		})
	}
}

// A GFM table has as many columns as its header row and leaves out the cells
// beyond them, so a row wider than the header (under a header cell spanning
// two columns, say) widens the header with empty cells (jir-h44x).
func TestToMarkdown_TableRowWiderThanHeader(t *testing.T) {
	t.Parallel()

	doc := docOf(rawNode(t, `{"type":"table","content":[`+
		`{"type":"tableRow","content":[{"type":"tableHeader","attrs":{"colspan":2},"content":[{"type":"paragraph","content":[{"type":"text","text":"H"}]}]}]},`+
		`{"type":"tableRow","content":[{"type":"tableCell","content":[{"type":"paragraph","content":[{"type":"text","text":"a"}]}]},`+
		`{"type":"tableCell","content":[{"type":"paragraph","content":[{"type":"text","text":"b"}]}]}]}]}`))
	md, _ := markdown.New().ToMarkdown(doc)
	assert.Equal(t, "| H |  |\n| --- | --- |\n| a | b |", md)
	adf, _ := markdown.New().ToADF(md)
	require.Len(t, adf.Content, 1)
	assert.Contains(t, jsonOf(t, adf), `"text":"b"`)
}
