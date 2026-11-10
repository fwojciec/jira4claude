package markdown_test

import (
	"testing"

	"github.com/fwojciec/jira4claude"
	"github.com/fwojciec/jira4claude/markdown"
	"github.com/stretchr/testify/assert"
)

// unwrappedIssue is the degraded issue reported when a node is not allowed
// inside container and is unwrapped into its children.
func unwrappedIssue(node, container string) jira4claude.ConversionIssue {
	return jira4claude.ConversionIssue{
		Kind:   jira4claude.ConversionDegraded,
		Node:   node,
		Detail: node + " is not allowed inside " + container + "; unwrapped into its content",
	}
}

// unwrappedExpandIssue is the degraded issue reported when an expand is
// unwrapped and its title kept as a paragraph.
func unwrappedExpandIssue(container string) jira4claude.ConversionIssue {
	return jira4claude.ConversionIssue{
		Kind:   jira4claude.ConversionDegraded,
		Node:   "expand",
		Detail: "expand is not allowed inside " + container + "; unwrapped into its content, its title kept as a paragraph",
	}
}

// downgradedHeadingIssue is the degraded issue reported when a heading is
// downgraded to a paragraph inside container.
func downgradedHeadingIssue(container string) jira4claude.ConversionIssue {
	return jira4claude.ConversionIssue{
		Kind:   jira4claude.ConversionDegraded,
		Node:   "heading",
		Detail: "heading is not allowed inside " + container + "; downgraded to a paragraph",
	}
}

// listOf wraps items in a bullet list of single-item list items.
func listOf(items ...[]jira4claude.ADFNode) jira4claude.ADFNode {
	list := jira4claude.ADFNode{Type: "bulletList"}
	for _, content := range items {
		list.Content = append(list.Content, jira4claude.ADFNode{Type: "listItem", Content: content})
	}
	return list
}

func doc(content ...jira4claude.ADFNode) *jira4claude.ADFNode {
	return &jira4claude.ADFNode{Type: "doc", Version: 1, Content: content}
}

func TestToADF_DetailsInsideContainers(t *testing.T) {
	t.Parallel()

	t.Run("details inside a list item keeps its title", func(t *testing.T) {
		t.Parallel()

		input := "- <details><summary>Title</summary>\n\n  body\n\n  </details>"

		result, issues := markdown.New().ToADF(input)

		requireValidADF(t, result)
		expected := doc(listOf([]jira4claude.ADFNode{para("Title"), para("body")}))
		assert.Equal(t, expected, result)
		assert.Equal(t, []jira4claude.ConversionIssue{unwrappedExpandIssue("listItem")}, issues)
	})

	t.Run("details inside a blockquote keeps its title", func(t *testing.T) {
		t.Parallel()

		input := "> <details><summary>Title</summary>\n>\n> body\n>\n> </details>"

		result, issues := markdown.New().ToADF(input)

		requireValidADF(t, result)
		expected := doc(jira4claude.ADFNode{
			Type:    "blockquote",
			Content: []jira4claude.ADFNode{para("Title"), para("body")},
		})
		assert.Equal(t, expected, result)
		assert.Equal(t, []jira4claude.ConversionIssue{unwrappedExpandIssue("blockquote")}, issues)
	})

	t.Run("nested details inside a list item keep both titles", func(t *testing.T) {
		t.Parallel()

		input := "- <details><summary>Outer</summary>\n\n  body\n\n" +
			"  <details><summary>Inner</summary>\n\n  inner\n\n  </details>\n\n" +
			"  tail\n\n  </details>"

		result, issues := markdown.New().ToADF(input)

		requireValidADF(t, result)
		expected := doc(listOf([]jira4claude.ADFNode{
			para("Outer"), para("body"), para("Inner"), para("inner"), para("tail"),
		}))
		assert.Equal(t, expected, result)
		assert.Equal(t, []jira4claude.ConversionIssue{
			nestedDetailsIssue(),
			unwrappedExpandIssue("listItem"),
		}, issues)
	})

	t.Run("nested details inside a blockquote keep both titles", func(t *testing.T) {
		t.Parallel()

		input := "> <details><summary>Outer</summary>\n>\n> body\n>\n" +
			"> <details><summary>Inner</summary>\n>\n> inner\n>\n> </details>\n>\n" +
			"> tail\n>\n> </details>"

		result, issues := markdown.New().ToADF(input)

		requireValidADF(t, result)
		expected := doc(jira4claude.ADFNode{
			Type: "blockquote",
			Content: []jira4claude.ADFNode{
				para("Outer"), para("body"), para("Inner"), para("inner"), para("tail"),
			},
		})
		assert.Equal(t, expected, result)
		assert.Equal(t, []jira4claude.ConversionIssue{
			nestedDetailsIssue(),
			unwrappedExpandIssue("blockquote"),
		}, issues)
	})

	t.Run("an expand with an empty title adds no paragraph", func(t *testing.T) {
		t.Parallel()

		input := "- <details><summary></summary>\n\n  body\n\n  </details>"

		result, issues := markdown.New().ToADF(input)

		requireValidADF(t, result)
		expected := doc(listOf([]jira4claude.ADFNode{para("body")}))
		assert.Equal(t, expected, result)
		assert.Equal(t, []jira4claude.ConversionIssue{unwrappedExpandIssue("listItem")}, issues)
	})
}

func TestToADF_ConstrainedChildrenReportDegraded(t *testing.T) {
	t.Parallel()

	t.Run("a heading inside a list item is downgraded and reported", func(t *testing.T) {
		t.Parallel()

		result, issues := markdown.New().ToADF("- ## Heading")

		requireValidADF(t, result)
		expected := doc(listOf([]jira4claude.ADFNode{para("Heading")}))
		assert.Equal(t, expected, result)
		assert.Equal(t, []jira4claude.ConversionIssue{downgradedHeadingIssue("listItem")}, issues)
	})

	t.Run("a blockquote inside a list item is unwrapped and reported", func(t *testing.T) {
		t.Parallel()

		result, issues := markdown.New().ToADF("- > quoted")

		requireValidADF(t, result)
		expected := doc(listOf([]jira4claude.ADFNode{para("quoted")}))
		assert.Equal(t, expected, result)
		assert.Equal(t, []jira4claude.ConversionIssue{unwrappedIssue("blockquote", "listItem")}, issues)
	})
}
