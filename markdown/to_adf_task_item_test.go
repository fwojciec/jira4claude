package markdown_test

import (
	"encoding/json"
	"testing"

	"github.com/fwojciec/jira4claude"
	"github.com/fwojciec/jira4claude/markdown"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// taskTextIssue is the degraded issue reported when a block of node type is
// not allowed inside a taskItem and its text is kept in the item after a
// line break.
func taskTextIssue(node string) jira4claude.ConversionIssue {
	return jira4claude.ConversionIssue{
		Kind:   jira4claude.ConversionDegraded,
		Node:   node,
		Detail: node + " is not allowed inside taskItem; its text kept in the task item after a line break",
	}
}

// droppedIssue is the issue reported when content of node type is omitted.
func droppedIssue(node string) jira4claude.ConversionIssue {
	return jira4claude.ConversionIssue{
		Kind:   jira4claude.ConversionDropped,
		Node:   node,
		Detail: "skipped unsupported node type '" + node + "'",
	}
}

// txt builds a plain ADF text node.
func txt(text string) jira4claude.ADFNode {
	return jira4claude.ADFNode{Type: "text", Text: text}
}

// hardBreak builds an ADF hardBreak node.
func hardBreak() jira4claude.ADFNode {
	return jira4claude.ADFNode{Type: "hardBreak"}
}

// taskItem builds an ADF taskItem with state and inline content. The
// localId is left out; compare against output passed through stripLocalIDs.
func taskItem(t *testing.T, state string, content ...jira4claude.ADFNode) jira4claude.ADFNode {
	t.Helper()
	attrs, err := json.Marshal(map[string]any{"state": state})
	require.NoError(t, err)
	return jira4claude.ADFNode{Type: "taskItem", Attrs: attrs, Content: content}
}

// taskList builds an ADF taskList with no attrs; compare against output
// passed through stripLocalIDs.
func taskList(items ...jira4claude.ADFNode) jira4claude.ADFNode {
	return jira4claude.ADFNode{Type: "taskList", Content: items}
}

// stripLocalIDs returns a copy of node with the random localId removed from
// every taskList and taskItem, so output can be compared exactly.
func stripLocalIDs(t *testing.T, node jira4claude.ADFNode) jira4claude.ADFNode {
	t.Helper()
	switch node.Type {
	case "taskList":
		node.Attrs = nil
	case "taskItem":
		var attrs struct {
			State string `json:"state"`
		}
		require.NoError(t, json.Unmarshal(node.Attrs, &attrs))
		stripped, err := json.Marshal(map[string]any{"state": attrs.State})
		require.NoError(t, err)
		node.Attrs = stripped
	}
	if node.Content != nil {
		content := make([]jira4claude.ADFNode, len(node.Content))
		for i, child := range node.Content {
			content[i] = stripLocalIDs(t, child)
		}
		node.Content = content
	}
	return node
}

func TestToADF_TaskItemBlockContent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		markdown string
		expected func(t *testing.T) *jira4claude.ADFNode
		issues   []jira4claude.ConversionIssue
	}{
		{
			name:     "plain unchecked item converts as before",
			markdown: "- [ ] task",
			expected: func(t *testing.T) *jira4claude.ADFNode {
				t.Helper()
				return doc(taskList(taskItem(t, "TODO", txt("task"))))
			},
		},
		{
			name:     "plain checked item converts as before",
			markdown: "- [x] done",
			expected: func(t *testing.T) *jira4claude.ADFNode {
				t.Helper()
				return doc(taskList(taskItem(t, "DONE", txt("done"))))
			},
		},
		{
			name:     "soft line break inside an item stays a space",
			markdown: "- [ ] one\n  two",
			expected: func(t *testing.T) *jira4claude.ADFNode {
				t.Helper()
				return doc(taskList(taskItem(t, "TODO", txt("one two"))))
			},
		},
		{
			name:     "second paragraph is kept after a hard break",
			markdown: "- [ ] task\n\n  second paragraph\n",
			expected: func(t *testing.T) *jira4claude.ADFNode {
				t.Helper()
				return doc(taskList(taskItem(t, "TODO", txt("task"), hardBreak(), txt("second paragraph"))))
			},
			issues: []jira4claude.ConversionIssue{taskTextIssue("paragraph")},
		},
		{
			name:     "nested bullets are kept after hard breaks",
			markdown: "- [ ] task\n  - first bullet\n  - second bullet\n",
			expected: func(t *testing.T) *jira4claude.ADFNode {
				t.Helper()
				return doc(taskList(taskItem(t, "TODO",
					txt("task"), hardBreak(), txt("first bullet"), hardBreak(), txt("second bullet"))))
			},
			issues: []jira4claude.ConversionIssue{taskTextIssue("bulletList")},
		},
		{
			name:     "nested ordered list is kept after hard breaks",
			markdown: "- [ ] task\n  1. first\n  2. second\n",
			expected: func(t *testing.T) *jira4claude.ADFNode {
				t.Helper()
				return doc(taskList(taskItem(t, "TODO",
					txt("task"), hardBreak(), txt("first"), hardBreak(), txt("second"))))
			},
			issues: []jira4claude.ConversionIssue{taskTextIssue("orderedList")},
		},
		{
			name:     "paragraphs and sub-bullets inside a nested bullet are all separated",
			markdown: "- [ ] task\n  - one\n\n    one more\n\n    - deep\n",
			expected: func(t *testing.T) *jira4claude.ADFNode {
				t.Helper()
				return doc(taskList(taskItem(t, "TODO",
					txt("task"), hardBreak(), txt("one"), hardBreak(), txt("one more"), hardBreak(), txt("deep"))))
			},
			issues: []jira4claude.ConversionIssue{taskTextIssue("bulletList")},
		},
		{
			name:     "heading is kept after a hard break",
			markdown: "- [ ] task\n\n  # Heading\n",
			expected: func(t *testing.T) *jira4claude.ADFNode {
				t.Helper()
				return doc(taskList(taskItem(t, "TODO", txt("task"), hardBreak(), txt("Heading"))))
			},
			issues: []jira4claude.ConversionIssue{taskTextIssue("heading")},
		},
		{
			name:     "blockquote is kept after a hard break",
			markdown: "- [ ] task\n\n  > quoted\n",
			expected: func(t *testing.T) *jira4claude.ADFNode {
				t.Helper()
				return doc(taskList(taskItem(t, "TODO", txt("task"), hardBreak(), txt("quoted"))))
			},
			issues: []jira4claude.ConversionIssue{taskTextIssue("blockquote")},
		},
		{
			name:     "marks in a second paragraph are kept",
			markdown: "- [ ] task\n\n  **bold**\n",
			expected: func(t *testing.T) *jira4claude.ADFNode {
				t.Helper()
				bold := jira4claude.ADFNode{Type: "text", Text: "bold", Marks: []jira4claude.ADFMark{{Type: "strong"}}}
				return doc(taskList(taskItem(t, "TODO", txt("task"), hardBreak(), bold)))
			},
			issues: []jira4claude.ConversionIssue{taskTextIssue("paragraph")},
		},
		{
			name:     "fenced code block is reported dropped",
			markdown: "- [ ] task\n\n  ```\n  secret code\n  ```\n",
			expected: func(t *testing.T) *jira4claude.ADFNode {
				t.Helper()
				return doc(taskList(taskItem(t, "TODO", txt("task"))))
			},
			issues: []jira4claude.ConversionIssue{droppedIssue("codeBlock")},
		},
		{
			name:     "code block inside a nested bullet is reported dropped",
			markdown: "- [ ] task\n  - bullet\n\n    ```\n    secret code\n    ```\n",
			expected: func(t *testing.T) *jira4claude.ADFNode {
				t.Helper()
				return doc(taskList(taskItem(t, "TODO", txt("task"), hardBreak(), txt("bullet"))))
			},
			issues: []jira4claude.ConversionIssue{taskTextIssue("bulletList"), droppedIssue("codeBlock")},
		},
		{
			name:     "table is reported dropped",
			markdown: "- [ ] task\n\n  | A |\n  | --- |\n  | b |\n",
			expected: func(t *testing.T) *jira4claude.ADFNode {
				t.Helper()
				return doc(taskList(taskItem(t, "TODO", txt("task"))))
			},
			issues: []jira4claude.ConversionIssue{droppedIssue("table")},
		},
		{
			name:     "thematic break is reported dropped",
			markdown: "- [ ] task\n\n  ***\n",
			expected: func(t *testing.T) *jira4claude.ADFNode {
				t.Helper()
				return doc(taskList(taskItem(t, "TODO", txt("task"))))
			},
			issues: []jira4claude.ConversionIssue{droppedIssue("rule")},
		},
		{
			name:     "item of a task list that starts with a code block is reported dropped",
			markdown: "- [ ] task\n- ```\n  secret code\n  ```\n",
			expected: func(t *testing.T) *jira4claude.ADFNode {
				t.Helper()
				return doc(taskList(taskItem(t, "TODO", txt("task")), taskItem(t, "TODO")))
			},
			issues: []jira4claude.ConversionIssue{droppedIssue("codeBlock")},
		},
		{
			name:     "nested task list becomes a sibling task list after its item",
			markdown: "- [ ] parent\n  - [x] child one\n  - [ ] child two\n- [ ] next\n",
			expected: func(t *testing.T) *jira4claude.ADFNode {
				t.Helper()
				return doc(taskList(
					taskItem(t, "TODO", txt("parent")),
					taskList(taskItem(t, "DONE", txt("child one")), taskItem(t, "TODO", txt("child two"))),
					taskItem(t, "TODO", txt("next")),
				))
			},
		},
		{
			name:     "text after a nested task list is kept in the item and reported",
			markdown: "- [ ] parent\n  - [ ] child\n\n  after\n",
			expected: func(t *testing.T) *jira4claude.ADFNode {
				t.Helper()
				return doc(taskList(
					taskItem(t, "TODO", txt("parent"), hardBreak(), txt("after")),
					taskList(taskItem(t, "TODO", txt("child"))),
				))
			},
			issues: []jira4claude.ConversionIssue{taskTextIssue("paragraph")},
		},
		{
			// A task list inside a bullet list item is constrained by the
			// listItem rules before it reaches the task item, which drop its
			// text; the write is refused either way.
			name:     "task list below a nested bullet is reported dropped",
			markdown: "- [ ] task\n  - bullet\n    - [x] deep task\n",
			expected: func(t *testing.T) *jira4claude.ADFNode {
				t.Helper()
				return doc(taskList(taskItem(t, "TODO", txt("task"), hardBreak(), txt("bullet"))))
			},
			issues: []jira4claude.ConversionIssue{
				taskTextIssue("bulletList"),
				unwrappedIssue("taskList", "listItem"),
				droppedIssue("text"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result, issues := markdown.New().ToADF(tt.markdown)

			requireValidADF(t, result)
			got := stripLocalIDs(t, *result)
			assert.Equal(t, tt.expected(t), &got)
			assert.Equal(t, tt.issues, issues)
		})
	}
}
