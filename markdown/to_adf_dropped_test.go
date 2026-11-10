package markdown_test

import (
	"testing"

	"github.com/fwojciec/jira4claude"
	"github.com/fwojciec/jira4claude/markdown"
	"github.com/stretchr/testify/assert"
)

// TestToADF_ImageIsDroppedEverywhere guards the write refusal: a Markdown
// image must be reported as dropped wherever it appears, or a write that
// loses it would go through unrefused.
func TestToADF_ImageIsDroppedEverywhere(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"paragraph":  "![a](http://x/y.png)",
		"heading":    "# Title ![a](http://x/y.png)",
		"list item":  "- item ![a](http://x/y.png)",
		"task item":  "- [ ] task ![a](http://x/y.png)",
		"table cell": "| A |\n| --- |\n| ![a](http://x/y.png) |",
		"blockquote": "> quote ![a](http://x/y.png)",
		"panel":      "> [!NOTE]\n> note ![a](http://x/y.png)",
		"link":       "[![a](http://x/y.png)](http://example.com)",
		"emphasis":   "**bold ![a](http://x/y.png)**",
		"details":    "<details><summary>T</summary>\n\n![a](http://x/y.png)\n\n</details>",
	}

	for name, md := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			_, issues := markdown.New().ToADF(md)

			assert.Contains(t, jira4claude.DroppedIssues(issues), jira4claude.ConversionIssue{
				Kind:   jira4claude.ConversionDropped,
				Node:   "Image",
				Detail: "skipped unsupported node type 'Image'",
			})
		})
	}
}
