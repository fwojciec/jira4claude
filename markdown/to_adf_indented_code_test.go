package markdown_test

import (
	"testing"

	jira4claude "github.com/fwojciec/jira4claude"
	"github.com/fwojciec/jira4claude/markdown"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestToADF_IndentedCodeBlock covers indented (four-space) code blocks, which
// must convert to a codeBlock without a language instead of being dropped.
func TestToADF_IndentedCodeBlock(t *testing.T) {
	t.Parallel()

	codeBlock := func(text string) jira4claude.ADFNode {
		return jira4claude.ADFNode{
			Type:    "codeBlock",
			Content: []jira4claude.ADFNode{{Type: "text", Text: text}},
		}
	}

	t.Run("converts top-level indented code to codeBlock without language", func(t *testing.T) {
		t.Parallel()

		md := "Before\n\n    func main() {\n        fmt.Println(\"hi\")\n    }\n\nAfter"
		result, issues := markdown.New().ToADF(md)

		assert.Empty(t, issues)
		requireValidADF(t, result)
		require.Len(t, result.Content, 3)
		assert.Equal(t, codeBlock("func main() {\n    fmt.Println(\"hi\")\n}"), result.Content[1])
		assert.Nil(t, result.Content[1].Attrs, "indented code has no language attribute")
	})

	t.Run("keeps interior blank lines and drops trailing ones", func(t *testing.T) {
		t.Parallel()

		md := "    a\n\n    b\n\n\n"
		result, issues := markdown.New().ToADF(md)

		assert.Empty(t, issues)
		requireValidADF(t, result)
		require.Len(t, result.Content, 1)
		assert.Equal(t, codeBlock("a\n\nb"), result.Content[0])
	})

	t.Run("converts indented code inside a list item", func(t *testing.T) {
		t.Parallel()

		md := "- item\n\n      code line\n"
		result, issues := markdown.New().ToADF(md)

		assert.Empty(t, issues)
		requireValidADF(t, result)
		require.Len(t, result.Content, 1)
		list := result.Content[0]
		require.Equal(t, "bulletList", list.Type)
		require.Len(t, list.Content, 1)
		item := list.Content[0]
		require.Len(t, item.Content, 2)
		assert.Equal(t, codeBlock("code line"), item.Content[1])
	})

	t.Run("reads back as a fenced code block and rewrites to identical ADF", func(t *testing.T) {
		t.Parallel()

		converter := markdown.New()
		md := "Intro\n\n    x := 1\n    y := 2\n\n- item\n\n      nested()\n"

		first, issues := converter.ToADF(md)
		require.Empty(t, issues)
		requireValidADF(t, first)

		readBack, issues := converter.ToMarkdown(first)
		require.Empty(t, issues)
		assert.Contains(t, readBack, "```\nx := 1\ny := 2\n```")

		second, issues := converter.ToADF(readBack)
		assert.Empty(t, issues)
		requireValidADF(t, second)
		assert.Equal(t, first, second)
	})
}
