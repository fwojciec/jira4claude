package markdown_test

import (
	"encoding/json"
	"os"
	"sort"
	"testing"

	jira4claude "github.com/fwojciec/jira4claude"
	"github.com/fwojciec/jira4claude/markdown"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRoundTrip(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		markdown string
	}{
		{"plain text", "Hello, world!"},
		{"bold text", "This is **bold** text."},
		{"italic text", "This is *italic* text."},
		{"inline code", "Use `fmt.Println` function."},
		{"code block", "```go\nfmt.Println(\"hello\")\n```"},
		{"heading", "## My Heading"},
		{"bullet list", "- Item 1\n- Item 2"},
		{"ordered list", "1. First\n2. Second"},
		{"link", "Visit [Google](https://google.com) for more."},
		{"blockquote", "> This is a quote."},
		{"multiple paragraphs", "First paragraph.\n\nSecond paragraph."},
		{"combined bold and italic", "This is ***bold and italic*** text."},
		{"complex document", `# Main Heading

This is a paragraph with **bold** and *italic* text.

## Subheading

- First item
- Second item

1. Numbered one
2. Numbered two

> A blockquote

` + "```go\nfunc main() {}\n```"},
		{"bare URL autolink", "Visit https://example.com for details."},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			converter := markdown.New()

			// Markdown -> ADF -> Markdown
			adfDoc, warnings := converter.ToADF(tc.markdown)
			assert.Empty(t, warnings)
			requireValidADF(t, adfDoc)

			result, warnings := converter.ToMarkdown(adfDoc)
			assert.Empty(t, warnings)

			assertMarkdownEqual(t, tc.markdown, result)
		})
	}

	t.Run("table round-trip", func(t *testing.T) {
		t.Parallel()

		input := "| Name | Age |\n| --- | --- |\n| Alice | 30 |"

		converter := markdown.New()
		adfDoc, warnings := converter.ToADF(input)
		assert.Empty(t, warnings)
		requireValidADF(t, adfDoc)

		result, warnings := converter.ToMarkdown(adfDoc)
		assert.Empty(t, warnings)

		assertMarkdownEqual(t, input, result)
	})

	t.Run("panel round-trip NOTE", func(t *testing.T) {
		t.Parallel()

		input := "> [!NOTE]\n> Panel content"

		converter := markdown.New()
		adfDoc, warnings := converter.ToADF(input)
		assert.Empty(t, warnings)
		requireValidADF(t, adfDoc)

		result, warnings := converter.ToMarkdown(adfDoc)
		assert.Empty(t, warnings)

		assertMarkdownEqual(t, input, result)
	})

	t.Run("panel round-trip all types", func(t *testing.T) {
		t.Parallel()

		alerts := []string{"NOTE", "WARNING", "CAUTION", "TIP", "IMPORTANT"}
		converter := markdown.New()
		for _, alert := range alerts {
			t.Run(alert, func(t *testing.T) {
				t.Parallel()

				input := "> [!" + alert + "]\n> Content here"
				adfDoc, w1 := converter.ToADF(input)
				assert.Empty(t, w1)
				requireValidADF(t, adfDoc)
				result, w2 := converter.ToMarkdown(adfDoc)
				assert.Empty(t, w2)
				assertMarkdownEqual(t, input, result)
			})
		}
	})

	t.Run("panel round-trip with multiple paragraphs", func(t *testing.T) {
		t.Parallel()

		input := "> [!WARNING]\n> First paragraph\n>\n> Second paragraph"

		converter := markdown.New()
		adfDoc, warnings := converter.ToADF(input)
		assert.Empty(t, warnings)
		requireValidADF(t, adfDoc)

		result, warnings := converter.ToMarkdown(adfDoc)
		assert.Empty(t, warnings)

		assertMarkdownEqual(t, input, result)
	})

	t.Run("task list round-trip", func(t *testing.T) {
		t.Parallel()

		input := "- [ ] Buy milk\n- [x] Walk dog"

		converter := markdown.New()
		adfDoc, warnings := converter.ToADF(input)
		assert.Empty(t, warnings)
		requireValidADF(t, adfDoc)

		result, warnings := converter.ToMarkdown(adfDoc)
		assert.Empty(t, warnings)

		assertMarkdownEqual(t, input, result)
	})
}

func TestGoldenFile_PanelMDToADF(t *testing.T) {
	t.Parallel()

	mdBytes, err := os.ReadFile("testdata/panel.md")
	require.NoError(t, err)

	expectedBytes, err := os.ReadFile("testdata/panel.adf.json")
	require.NoError(t, err)

	var expected jira4claude.ADFNode
	require.NoError(t, json.Unmarshal(expectedBytes, &expected))

	converter := markdown.New()
	result, warnings := converter.ToADF(string(mdBytes))

	assert.Empty(t, warnings)
	requireValidADF(t, result)
	assert.Equal(t, &expected, result)
}

func TestGoldenFile_PanelADFToMD(t *testing.T) {
	t.Parallel()

	adfBytes, err := os.ReadFile("testdata/panel.adf.json")
	require.NoError(t, err)

	var adfDoc jira4claude.ADFNode
	require.NoError(t, json.Unmarshal(adfBytes, &adfDoc))

	expectedMD, err := os.ReadFile("testdata/panel.md")
	require.NoError(t, err)

	converter := markdown.New()
	result, warnings := converter.ToMarkdown(&adfDoc)

	assert.Empty(t, warnings)
	assert.Equal(t, string(expectedMD), result)
}

func TestGoldenFile_TableMDToADF(t *testing.T) {
	t.Parallel()

	mdBytes, err := os.ReadFile("testdata/table.md")
	require.NoError(t, err)

	expectedBytes, err := os.ReadFile("testdata/table.adf.json")
	require.NoError(t, err)

	var expected jira4claude.ADFNode
	require.NoError(t, json.Unmarshal(expectedBytes, &expected))

	converter := markdown.New()
	result, warnings := converter.ToADF(string(mdBytes))

	assert.Empty(t, warnings)
	requireValidADF(t, result)
	assert.Equal(t, &expected, result)
}

func TestGoldenFile_TaskListMDToADF(t *testing.T) {
	t.Parallel()

	mdBytes, err := os.ReadFile("testdata/tasklist.md")
	require.NoError(t, err)

	converter := markdown.New()
	result, warnings := converter.ToADF(string(mdBytes))

	assert.Empty(t, warnings)
	requireValidADF(t, result)
	require.Equal(t, "doc", result.Type)
	require.Len(t, result.Content, 1)

	taskList := result.Content[0]
	assert.Equal(t, "taskList", taskList.Type)
	require.Len(t, taskList.Content, 3)

	// Verify taskList has localId attr
	var taskListAttrs map[string]any
	require.NoError(t, json.Unmarshal(taskList.Attrs, &taskListAttrs))
	assert.NotEmpty(t, taskListAttrs["localId"])

	// Verify structure matches golden file (ignoring dynamic localId)
	expectedStates := []string{"TODO", "DONE", "TODO"}
	expectedTexts := []string{"Buy milk", "Walk dog", "Write code"}

	for i, item := range taskList.Content {
		assert.Equal(t, "taskItem", item.Type)

		var attrs map[string]any
		require.NoError(t, json.Unmarshal(item.Attrs, &attrs))
		assert.Equal(t, expectedStates[i], attrs["state"])
		assert.NotEmpty(t, attrs["localId"])

		// taskItem content is inline nodes directly (not wrapped in paragraph)
		require.Len(t, item.Content, 1)
		assert.Equal(t, "text", item.Content[0].Type)
		assert.Equal(t, expectedTexts[i], item.Content[0].Text)
	}
}

func TestGoldenFile_TableADFToMD(t *testing.T) {
	t.Parallel()

	adfBytes, err := os.ReadFile("testdata/table.adf.json")
	require.NoError(t, err)

	var adfDoc jira4claude.ADFNode
	require.NoError(t, json.Unmarshal(adfBytes, &adfDoc))

	expectedMD, err := os.ReadFile("testdata/table.md")
	require.NoError(t, err)

	converter := markdown.New()
	result, warnings := converter.ToMarkdown(&adfDoc)

	assert.Empty(t, warnings)
	assert.Equal(t, string(expectedMD), result)
}

func TestGoldenFile_TaskListADFToMD(t *testing.T) {
	t.Parallel()

	adfBytes, err := os.ReadFile("testdata/tasklist.adf.json")
	require.NoError(t, err)

	var adfDoc jira4claude.ADFNode
	require.NoError(t, json.Unmarshal(adfBytes, &adfDoc))

	expectedMD, err := os.ReadFile("testdata/tasklist.md")
	require.NoError(t, err)

	converter := markdown.New()
	result, warnings := converter.ToMarkdown(&adfDoc)

	assert.Empty(t, warnings)
	assert.Equal(t, string(expectedMD), result)
}

func TestGoldenFile_ExpandMDToADF(t *testing.T) {
	t.Parallel()

	mdBytes, err := os.ReadFile("testdata/expand.md")
	require.NoError(t, err)

	expectedBytes, err := os.ReadFile("testdata/expand.adf.json")
	require.NoError(t, err)

	var expected jira4claude.ADFNode
	require.NoError(t, json.Unmarshal(expectedBytes, &expected))

	converter := markdown.New()
	result, warnings := converter.ToADF(string(mdBytes))

	assert.Empty(t, warnings)
	requireValidADF(t, result)
	assert.Equal(t, &expected, result)
}

func TestGoldenFile_ExpandADFToMD(t *testing.T) {
	t.Parallel()

	adfBytes, err := os.ReadFile("testdata/expand.adf.json")
	require.NoError(t, err)

	var adfDoc jira4claude.ADFNode
	require.NoError(t, json.Unmarshal(adfBytes, &adfDoc))

	expectedMD, err := os.ReadFile("testdata/expand.md")
	require.NoError(t, err)

	converter := markdown.New()
	result, warnings := converter.ToMarkdown(&adfDoc)

	assert.Empty(t, warnings)
	assert.Equal(t, string(expectedMD), result)
}

func TestGoldenFile_MediaADFToMD(t *testing.T) {
	t.Parallel()

	adfBytes, err := os.ReadFile("testdata/media.adf.json")
	require.NoError(t, err)

	var adfDoc jira4claude.ADFNode
	require.NoError(t, json.Unmarshal(adfBytes, &adfDoc))

	expectedMD, err := os.ReadFile("testdata/media.md")
	require.NoError(t, err)

	converter := markdown.New()
	result, warnings := converter.ToMarkdown(&adfDoc)

	assert.Empty(t, warnings)
	assert.Equal(t, string(expectedMD), result)
}

func TestGoldenFile_InlineNodesADFToMD(t *testing.T) {
	t.Parallel()

	adfBytes, err := os.ReadFile("testdata/inline_nodes.adf.json")
	require.NoError(t, err)

	var adfDoc jira4claude.ADFNode
	require.NoError(t, json.Unmarshal(adfBytes, &adfDoc))

	expectedMD, err := os.ReadFile("testdata/inline_nodes.md")
	require.NoError(t, err)

	converter := markdown.New()
	result, warnings := converter.ToMarkdown(&adfDoc)

	assert.Empty(t, warnings)
	assert.Equal(t, string(expectedMD), result)
}

func TestRoundTrip_Expand(t *testing.T) {
	t.Parallel()

	input := "<details><summary>Click to expand</summary>\n\nExpanded content here.\n\n</details>"

	converter := markdown.New()
	adfDoc, w1 := converter.ToADF(input)
	assert.Empty(t, w1)
	requireValidADF(t, adfDoc)

	result, w2 := converter.ToMarkdown(adfDoc)
	assert.Empty(t, w2)

	assertMarkdownEqual(t, input, result)
}

func TestGoldenFile_MarksMDToADF(t *testing.T) {
	t.Parallel()

	mdBytes, err := os.ReadFile("testdata/marks.md")
	require.NoError(t, err)

	expectedBytes, err := os.ReadFile("testdata/marks.adf.json")
	require.NoError(t, err)

	var expected jira4claude.ADFNode
	require.NoError(t, json.Unmarshal(expectedBytes, &expected))

	converter := markdown.New()
	result, warnings := converter.ToADF(string(mdBytes))

	assert.Empty(t, warnings)
	requireValidADF(t, result)
	assert.Equal(t, &expected, result)
}

func TestGoldenFile_MarksADFToMD(t *testing.T) {
	t.Parallel()

	adfBytes, err := os.ReadFile("testdata/marks.adf.json")
	require.NoError(t, err)

	var adfDoc jira4claude.ADFNode
	require.NoError(t, json.Unmarshal(adfBytes, &adfDoc))

	expectedMD, err := os.ReadFile("testdata/marks.md")
	require.NoError(t, err)

	converter := markdown.New()
	result, warnings := converter.ToMarkdown(&adfDoc)

	assert.Empty(t, warnings)
	assert.Equal(t, string(expectedMD), result)
}

func TestRoundTrip_Underline(t *testing.T) {
	t.Parallel()

	input := "This is <u>underlined</u> text."

	converter := markdown.New()
	adfDoc, w1 := converter.ToADF(input)
	assert.Empty(t, w1)
	requireValidADF(t, adfDoc)

	result, w2 := converter.ToMarkdown(adfDoc)
	assert.Empty(t, w2)

	assertMarkdownEqual(t, input, result)
}

func TestRoundTrip_Subscript(t *testing.T) {
	t.Parallel()

	input := "H<sub>2</sub>O"

	converter := markdown.New()
	adfDoc, w1 := converter.ToADF(input)
	assert.Empty(t, w1)
	requireValidADF(t, adfDoc)

	result, w2 := converter.ToMarkdown(adfDoc)
	assert.Empty(t, w2)

	assertMarkdownEqual(t, input, result)
}

func TestRoundTrip_Superscript(t *testing.T) {
	t.Parallel()

	input := "x<sup>2</sup>"

	converter := markdown.New()
	adfDoc, w1 := converter.ToADF(input)
	assert.Empty(t, w1)
	requireValidADF(t, adfDoc)

	result, w2 := converter.ToMarkdown(adfDoc)
	assert.Empty(t, w2)

	assertMarkdownEqual(t, input, result)
}

func TestMarkdownEquivalence(t *testing.T) {
	t.Parallel()

	t.Run("identical strings are equivalent", func(t *testing.T) {
		t.Parallel()
		assert.True(t, markdownEquivalent(t, "**bold** text", "**bold** text"))
	})

	t.Run("different content is not equivalent", func(t *testing.T) {
		t.Parallel()
		assert.False(t, markdownEquivalent(t, "**bold** text", "*italic* text"))
	})

	t.Run("reordered marks are equivalent", func(t *testing.T) {
		t.Parallel()
		assert.True(t, markdownEquivalent(t, "~~**text**~~", "**~~text~~**"))
	})

	t.Run("setext and atx headings are equivalent", func(t *testing.T) {
		t.Parallel()
		assert.True(t, markdownEquivalent(t, "# Heading", "Heading\n======"))
	})
}

// markdownEquivalent checks if two markdown strings are semantically equivalent
// by comparing their ADF representations with normalized mark ordering.
// Warnings from ToADF are compared to prevent false positives when content is dropped.
func markdownEquivalent(t *testing.T, a, b string) bool {
	t.Helper()
	conv := markdown.New()

	adfA, wA := conv.ToADF(a)
	adfB, wB := conv.ToADF(b)

	if !assert.ObjectsAreEqual(wA, wB) {
		return false
	}

	normalizeADF(adfA)
	normalizeADF(adfB)

	return assert.ObjectsAreEqual(adfA, adfB)
}

// assertMarkdownEqual asserts that two markdown strings are semantically
// equivalent by comparing their normalized ADF representations.
// Warnings from ToADF are checked to prevent false positives when content is dropped.
// Both ADF representations are validated against the embedded ADF JSON schema.
func assertMarkdownEqual(t *testing.T, expected, actual string) {
	t.Helper()
	conv := markdown.New()

	expectedADF, wExpected := conv.ToADF(expected)
	actualADF, wActual := conv.ToADF(actual)

	requireValidADF(t, expectedADF)
	requireValidADF(t, actualADF)

	assert.Equal(t, wExpected, wActual, "ToADF warnings differ between expected and actual markdown")

	normalizeADF(expectedADF)
	normalizeADF(actualADF)

	if !assert.ObjectsAreEqual(expectedADF, actualADF) {
		t.Errorf("markdown not semantically equivalent\nexpected: %s\nactual:   %s", expected, actual)
	}
}

// normalizeADF normalizes an ADF tree for comparison by sorting marks
// and stripping non-deterministic fields like localId.
func normalizeADF(node *jira4claude.ADFNode) {
	if node == nil {
		return
	}

	// Sort marks by type for order-independent comparison
	if len(node.Marks) > 1 {
		sort.Slice(node.Marks, func(i, j int) bool {
			return node.Marks[i].Type < node.Marks[j].Type
		})
	}

	// Strip non-deterministic attrs (localId in taskItem and taskList)
	if (node.Type == "taskItem" || node.Type == "taskList") && node.Attrs != nil {
		var attrs map[string]any
		if err := json.Unmarshal(node.Attrs, &attrs); err == nil {
			delete(attrs, "localId")
			node.Attrs, _ = json.Marshal(attrs)
		}
	}

	for i := range node.Content {
		normalizeADF(&node.Content[i])
	}
}
