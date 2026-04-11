package markdown_test

import (
	"encoding/json"
	"os"
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

			result, warnings := converter.ToMarkdown(adfDoc)
			assert.Empty(t, warnings)

			assert.Equal(t, tc.markdown, result)
		})
	}

	t.Run("table round-trip", func(t *testing.T) {
		t.Parallel()

		input := "| Name | Age |\n| --- | --- |\n| Alice | 30 |"

		converter := markdown.New()
		adfDoc, warnings := converter.ToADF(input)
		assert.Empty(t, warnings)

		result, warnings := converter.ToMarkdown(adfDoc)
		assert.Empty(t, warnings)

		assert.Equal(t, input, result)
	})

	t.Run("panel round-trip NOTE", func(t *testing.T) {
		t.Parallel()

		input := "> [!NOTE]\n> Panel content"

		converter := markdown.New()
		adfDoc, warnings := converter.ToADF(input)
		assert.Empty(t, warnings)

		result, warnings := converter.ToMarkdown(adfDoc)
		assert.Empty(t, warnings)

		assert.Equal(t, input, result)
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
				result, w2 := converter.ToMarkdown(adfDoc)
				assert.Empty(t, w2)
				assert.Equal(t, input, result)
			})
		}
	})

	t.Run("panel round-trip with multiple paragraphs", func(t *testing.T) {
		t.Parallel()

		input := "> [!WARNING]\n> First paragraph\n>\n> Second paragraph"

		converter := markdown.New()
		adfDoc, warnings := converter.ToADF(input)
		assert.Empty(t, warnings)

		result, warnings := converter.ToMarkdown(adfDoc)
		assert.Empty(t, warnings)

		assert.Equal(t, input, result)
	})

	t.Run("task list round-trip", func(t *testing.T) {
		t.Parallel()

		input := "- [ ] Buy milk\n- [x] Walk dog"

		converter := markdown.New()
		adfDoc, warnings := converter.ToADF(input)
		assert.Empty(t, warnings)

		result, warnings := converter.ToMarkdown(adfDoc)
		assert.Empty(t, warnings)

		assert.Equal(t, input, result)
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
	assert.Equal(t, &expected, result)
}

func TestGoldenFile_TaskListMDToADF(t *testing.T) {
	t.Parallel()

	mdBytes, err := os.ReadFile("testdata/tasklist.md")
	require.NoError(t, err)

	converter := markdown.New()
	result, warnings := converter.ToADF(string(mdBytes))

	assert.Empty(t, warnings)
	require.Equal(t, "doc", result.Type)
	require.Len(t, result.Content, 1)

	taskList := result.Content[0]
	assert.Equal(t, "taskList", taskList.Type)
	require.Len(t, taskList.Content, 3)

	// Verify structure matches golden file (ignoring dynamic localId)
	expectedStates := []string{"TODO", "DONE", "TODO"}
	expectedTexts := []string{"Buy milk", "Walk dog", "Write code"}

	for i, item := range taskList.Content {
		assert.Equal(t, "taskItem", item.Type)

		var attrs map[string]any
		require.NoError(t, json.Unmarshal(item.Attrs, &attrs))
		assert.Equal(t, expectedStates[i], attrs["state"])
		assert.NotEmpty(t, attrs["localId"])

		require.Len(t, item.Content, 1)
		para := item.Content[0]
		assert.Equal(t, "paragraph", para.Type)
		require.Len(t, para.Content, 1)
		assert.Equal(t, expectedTexts[i], para.Content[0].Text)
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
