package markdown_test

import (
	"encoding/json"
	"testing"

	"github.com/fwojciec/jira4claude"
	"github.com/fwojciec/jira4claude/markdown"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestToADF_MarkSet covers the marks of a written text node (#159): a mark
// type appears at most once, and marks are in one canonical order (link, em,
// strong, strike, subsup, underline, code), whatever the Markdown nesting.
func TestToADF_MarkSet(t *testing.T) {
	t.Parallel()

	link := jira4claude.ADFMark{Type: "link", Attrs: json.RawMessage(`{"href":"x"}`)}
	em := jira4claude.ADFMark{Type: "em"}
	strong := jira4claude.ADFMark{Type: "strong"}
	strike := jira4claude.ADFMark{Type: "strike"}
	underline := jira4claude.ADFMark{Type: "underline"}
	sup := jira4claude.ADFMark{Type: "subsup", Attrs: json.RawMessage(`{"type":"sup"}`)}

	cases := map[string]struct {
		md   string
		want []jira4claude.ADFMark
	}{
		"em inside em":                {md: "*_a_*", want: []jira4claude.ADFMark{em}},
		"strong inside strong":        {md: "**__a__**", want: []jira4claude.ADFMark{strong}},
		"underline inside underline":  {md: "<u><u>a</u></u>", want: []jira4claude.ADFMark{underline}},
		"underline across emphasis":   {md: "<u>*<u>a</u>*</u>", want: []jira4claude.ADFMark{em, underline}},
		"strong around em":            {md: "**_a_**", want: []jira4claude.ADFMark{em, strong}},
		"em around strong":            {md: "_**a**_", want: []jira4claude.ADFMark{em, strong}},
		"triple delimiter":            {md: "***a***", want: []jira4claude.ADFMark{em, strong}},
		"strike around strong":        {md: "~~**a**~~", want: []jira4claude.ADFMark{strong, strike}},
		"strong around strike":        {md: "**~~a~~**", want: []jira4claude.ADFMark{strong, strike}},
		"strong inside link":          {md: "[**a**](x)", want: []jira4claude.ADFMark{link, strong}},
		"link inside strong":          {md: "**[a](x)**", want: []jira4claude.ADFMark{link, strong}},
		"underline around sup":        {md: "<u><sup>a</sup></u>", want: []jira4claude.ADFMark{sup, underline}},
		"sup around underline":        {md: "<sup><u>a</u></sup>", want: []jira4claude.ADFMark{sup, underline}},
		"underline around strong":     {md: "<u>**a**</u>", want: []jira4claude.ADFMark{strong, underline}},
		"code in link in underline":   {md: "<u>[`a`](x)</u>", want: []jira4claude.ADFMark{link, {Type: "code"}}},
		"strong em strike underlined": {md: "<u>~~***a***~~</u>", want: []jira4claude.ADFMark{em, strong, strike, underline}},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			result, issues := markdown.New().ToADF(tc.md)

			requireValidADF(t, result)
			assert.Empty(t, issues)
			require.Len(t, result.Content, 1)
			require.Len(t, result.Content[0].Content, 1, "one text node")
			assert.Equal(t, tc.want, result.Content[0].Content[0].Marks)
		})
	}

	t.Run("strike inside strike", func(t *testing.T) {
		t.Parallel()

		result, issues := markdown.New().ToADF("~~a ~~b~~ c~~")

		requireValidADF(t, result)
		assert.Empty(t, issues)
		for _, n := range result.Content[0].Content {
			assert.LessOrEqual(t, countMarks(n.Marks, "strike"), 1, "node %q", n.Text)
		}
	})

	t.Run("em inside em around strike", func(t *testing.T) {
		t.Parallel()

		result, issues := markdown.New().ToADF("*a _~~b~~_*")

		requireValidADF(t, result)
		assert.Empty(t, issues)
		assert.Equal(t, []jira4claude.ADFNode{
			{Type: "text", Text: "a ", Marks: []jira4claude.ADFMark{em}},
			{Type: "text", Text: "b", Marks: []jira4claude.ADFMark{em, strike}},
		}, result.Content[0].Content)
	})
}

// TestToADF_ConflictingMarks covers nested marks of one type with different
// attributes: ADF allows one mark of a type on a text node, so the innermost
// is kept and the conversion reports the text as degraded (#159).
func TestToADF_ConflictingMarks(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		md   string
		want jira4claude.ADFMark
	}{
		"sup inside sub": {md: "<sub><sup>a</sup></sub>", want: jira4claude.ADFMark{Type: "subsup", Attrs: json.RawMessage(`{"type":"sup"}`)}},
		"sub inside sup": {md: "<sup><sub>a</sub></sup>", want: jira4claude.ADFMark{Type: "subsup", Attrs: json.RawMessage(`{"type":"sub"}`)}},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			result, issues := markdown.New().ToADF(tc.md)

			requireValidADF(t, result)
			require.Len(t, result.Content, 1)
			require.Len(t, result.Content[0].Content, 1)
			assert.Equal(t, []jira4claude.ADFMark{tc.want}, result.Content[0].Content[0].Marks)
			require.Len(t, issues, 1)
			assert.Equal(t, jira4claude.ConversionDegraded, issues[0].Kind)
			assert.Equal(t, "text", issues[0].Node)
		})
	}

	t.Run("same sub twice is not a conflict", func(t *testing.T) {
		t.Parallel()

		result, issues := markdown.New().ToADF("<sub><sub>a</sub></sub>")

		assert.Empty(t, issues)
		assert.Equal(t, []jira4claude.ADFMark{{Type: "subsup", Attrs: json.RawMessage(`{"type":"sub"}`)}},
			result.Content[0].Content[0].Marks)
	})
}

func countMarks(marks []jira4claude.ADFMark, markType string) int {
	n := 0
	for _, m := range marks {
		if m.Type == markType {
			n++
		}
	}
	return n
}
