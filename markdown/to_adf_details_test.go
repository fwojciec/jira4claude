package markdown_test

import (
	"encoding/json"
	"testing"

	"github.com/fwojciec/jira4claude"
	"github.com/fwojciec/jira4claude/markdown"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// para builds an ADF paragraph holding a single plain text node.
func para(text string) jira4claude.ADFNode {
	return jira4claude.ADFNode{
		Type:    "paragraph",
		Content: []jira4claude.ADFNode{{Type: "text", Text: text}},
	}
}

// expandNode builds an ADF expand node with the given title and body.
func expandNode(t *testing.T, title string, body ...jira4claude.ADFNode) jira4claude.ADFNode {
	t.Helper()
	attrs, err := json.Marshal(map[string]any{"title": title})
	require.NoError(t, err)
	return jira4claude.ADFNode{Type: "expand", Attrs: attrs, Content: body}
}

// nestedDetailsIssue is the degraded issue reported when a nested <details>
// block is flattened into its enclosing expand.
func nestedDetailsIssue() jira4claude.ConversionIssue {
	return jira4claude.ConversionIssue{
		Kind:   jira4claude.ConversionDegraded,
		Node:   "details",
		Detail: "nested <details> flattened into the enclosing expand (ADF cannot nest expand); its summary is kept as a paragraph",
	}
}

func TestToADF_NestedDetails(t *testing.T) {
	t.Parallel()

	t.Run("flattens the inner block into one outer expand", func(t *testing.T) {
		t.Parallel()

		input := "<details><summary>Outer</summary>\n\nbody\n\n" +
			"<details><summary>Inner</summary>\n\ninner\n\n</details>\n\n" +
			"tail\n\n</details>\n\nafter"

		result, issues := markdown.New().ToADF(input)

		requireValidADF(t, result)
		expected := &jira4claude.ADFNode{
			Type:    "doc",
			Version: 1,
			Content: []jira4claude.ADFNode{
				expandNode(t, "Outer", para("body"), para("Inner"), para("inner"), para("tail")),
				para("after"),
			},
		}
		assert.Equal(t, expected, result)
		assert.Equal(t, []jira4claude.ConversionIssue{nestedDetailsIssue()}, issues)
	})

	t.Run("reports one degraded issue for several nested blocks at any depth", func(t *testing.T) {
		t.Parallel()

		input := "<details><summary>A</summary>\n\n" +
			"<details><summary>B</summary>\n\n" +
			"<details><summary>C</summary>\n\nc\n\n</details>\n\n" +
			"b\n\n</details>\n\n" +
			"<details><summary>D</summary>\n\nd\n\n</details>\n\n" +
			"a\n\n</details>"

		result, issues := markdown.New().ToADF(input)

		requireValidADF(t, result)
		expected := &jira4claude.ADFNode{
			Type:    "doc",
			Version: 1,
			Content: []jira4claude.ADFNode{
				expandNode(t, "A",
					para("B"), para("C"), para("c"), para("b"),
					para("D"), para("d"),
					para("a"),
				),
			},
		}
		assert.Equal(t, expected, result)
		assert.Equal(t, []jira4claude.ConversionIssue{nestedDetailsIssue()}, issues)
	})

	t.Run("an inner block without a summary still nests", func(t *testing.T) {
		t.Parallel()

		input := "<details><summary>Outer</summary>\n\n" +
			"<details>\n\ninner\n\n</details>\n\n" +
			"tail\n\n</details>\n\nafter"

		result, issues := markdown.New().ToADF(input)

		requireValidADF(t, result)
		expected := &jira4claude.ADFNode{
			Type:    "doc",
			Version: 1,
			Content: []jira4claude.ADFNode{
				expandNode(t, "Outer", para("inner"), para("tail")),
				para("after"),
			},
		}
		assert.Equal(t, expected, result)
		assert.Equal(t, []jira4claude.ConversionIssue{nestedDetailsIssue()}, issues)
	})

	t.Run("sibling blocks are not nested and report nothing", func(t *testing.T) {
		t.Parallel()

		input := "<details><summary>One</summary>\n\none\n\n</details>\n\n" +
			"<details><summary>Two</summary>\n\ntwo\n\n</details>"

		result, issues := markdown.New().ToADF(input)

		requireValidADF(t, result)
		expected := &jira4claude.ADFNode{
			Type:    "doc",
			Version: 1,
			Content: []jira4claude.ADFNode{
				expandNode(t, "One", para("one")),
				expandNode(t, "Two", para("two")),
			},
		}
		assert.Equal(t, expected, result)
		assert.Empty(t, issues)
	})
}

func TestToADF_UnclosedDetails(t *testing.T) {
	t.Parallel()

	t.Run("an unclosed block emits its body unwrapped and reports the HTML block", func(t *testing.T) {
		t.Parallel()

		result, issues := markdown.New().ToADF("before\n\n<details><summary>Title</summary>\n\nBody text\n\nMore text")

		requireValidADF(t, result)
		expected := &jira4claude.ADFNode{
			Type:    "doc",
			Version: 1,
			Content: []jira4claude.ADFNode{para("before"), para("Body text"), para("More text")},
		}
		assert.Equal(t, expected, result)
		require.Len(t, issues, 1)
		assert.Equal(t, jira4claude.ConversionDropped, issues[0].Kind)
		assert.Equal(t, "HTMLBlock", issues[0].Node)
	})

	t.Run("an outer block left open around a closed inner block is unclosed", func(t *testing.T) {
		t.Parallel()

		input := "<details><summary>Outer</summary>\n\nbody\n\n" +
			"<details><summary>Inner</summary>\n\ninner\n\n</details>\n\ntail"

		result, issues := markdown.New().ToADF(input)

		requireValidADF(t, result)
		expected := &jira4claude.ADFNode{
			Type:    "doc",
			Version: 1,
			Content: []jira4claude.ADFNode{para("body"), para("Inner"), para("inner"), para("tail")},
		}
		assert.Equal(t, expected, result)
		assert.Equal(t, []jira4claude.ConversionIssue{
			{
				Kind:   jira4claude.ConversionDropped,
				Node:   "HTMLBlock",
				Detail: "skipped unsupported node type 'HTMLBlock'",
			},
			nestedDetailsIssue(),
		}, issues)
	})
}
