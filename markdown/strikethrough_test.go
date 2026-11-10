package markdown_test

import (
	"testing"

	jira4claude "github.com/fwojciec/jira4claude"
	"github.com/fwojciec/jira4claude/markdown"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToADF_SingleTildeIsLiteral(t *testing.T) {
	t.Parallel()

	for _, input := range []string{
		"5~10 and 20~30",
		"a ~b~ c",
		"~a~",
		"x ~~~a~~~ y",
		"x ~~a~ y",
		"x ~a~~ y",
	} {
		t.Run(input, func(t *testing.T) {
			t.Parallel()

			converter := markdown.New()
			result, warnings := converter.ToADF(input)

			assert.Empty(t, warnings)
			requireValidADF(t, result)
			require.Len(t, result.Content, 1)
			paragraph := result.Content[0]
			require.Equal(t, "paragraph", paragraph.Type)

			var text string
			for _, node := range paragraph.Content {
				assert.Equal(t, "text", node.Type)
				assert.Empty(t, node.Marks, "node %q must not carry marks", node.Text)
				text += node.Text
			}
			assert.Equal(t, input, text)
		})
	}
}

func TestToADF_DoubleTildeStrikes(t *testing.T) {
	t.Parallel()

	converter := markdown.New()
	result, warnings := converter.ToADF("~~gone~~")

	expected := &jira4claude.ADFNode{
		Type:    "doc",
		Version: 1,
		Content: []jira4claude.ADFNode{
			{
				Type: "paragraph",
				Content: []jira4claude.ADFNode{
					{
						Type:  "text",
						Text:  "gone",
						Marks: []jira4claude.ADFMark{{Type: "strike"}},
					},
				},
			},
		},
	}

	assert.Empty(t, warnings)
	requireValidADF(t, result)
	assert.Equal(t, expected, result)
}

func TestRoundTrip_Tildes(t *testing.T) {
	t.Parallel()

	for _, input := range []string{
		"5~10 and 20~30",
		"a ~b~ c",
		"~~gone~~",
		"keep ~~this~~ struck, but 5~10 is a range",
	} {
		t.Run(input, func(t *testing.T) {
			t.Parallel()

			converter := markdown.New()
			adfDoc, w1 := converter.ToADF(input)
			assert.Empty(t, w1)
			requireValidADF(t, adfDoc)

			result, w2 := converter.ToMarkdown(adfDoc)
			assert.Empty(t, w2)
			assert.Equal(t, input, result)
		})
	}
}

func TestRoundTrip_PlainTextTildeFromJira(t *testing.T) {
	t.Parallel()

	// A text node from Jira holding lone tildes, or a backslash before a
	// tilde, must come back from ADF -> Markdown -> ADF unchanged and without
	// gaining a strike mark. Literal "~~" in Jira text is tracked separately
	// (GitHub issue 153).
	strong := []jira4claude.ADFMark{{Type: "strong"}}
	for _, tc := range []struct {
		name  string
		text  string
		marks []jira4claude.ADFMark
	}{
		{"lone tildes", "5~10 and 20~30", nil},
		{"tilde pair around word", "a ~b~ c", nil},
		{"lone tildes in strong", "5~10 and 20~30", strong},
		{"triple tildes", "x ~~~a~~~ y", nil},
		{"backslash before tilde in path", `C:\~user`, nil},
		{"backslash before spaced tilde", `a \~ b`, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			original := &jira4claude.ADFNode{
				Type:    "doc",
				Version: 1,
				Content: []jira4claude.ADFNode{
					{
						Type:    "paragraph",
						Content: []jira4claude.ADFNode{{Type: "text", Text: tc.text, Marks: tc.marks}},
					},
				},
			}

			converter := markdown.New()
			md, w1 := converter.ToMarkdown(original)
			assert.Empty(t, w1)

			result, w2 := converter.ToADF(md)
			assert.Empty(t, w2)
			requireValidADF(t, result)
			assert.Equal(t, original, result)
		})
	}
}
