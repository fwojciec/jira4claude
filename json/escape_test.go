package json_test

import (
	"bytes"
	"testing"

	"github.com/fwojciec/jira4claude"
	jsonpkg "github.com/fwojciec/jira4claude/json"
	"github.com/stretchr/testify/assert"
)

// stdout JSON keeps &, < and > as they are, as the stderr lines do: the
// text is read by people as well as parsed.
func TestPrinter_StdoutDoesNotEscapeHTML(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	p := jsonpkg.NewPrinter(&out)

	p.Issue(jira4claude.IssueView{Key: "TEST-1", Summary: "Fix <b> & <i>"})

	assert.Contains(t, out.String(), `"summary": "Fix <b> & <i>"`)
	assert.NotContains(t, out.String(), `\u00`)

	out.Reset()
	p.IssueSelection(jira4claude.IssueSelectionView{Fields: []string{"summary"}, Issues: []jira4claude.SelectedIssueView{
		{Key: "TEST-1", Fields: []jira4claude.ViewFieldValue{{Name: "summary", Value: "a < b && c > d"}}},
	}})
	assert.Contains(t, out.String(), `"summary": "a < b && c > d"`)

	out.Reset()
	p.Mutation(jira4claude.MutationResult{Key: "TEST-1", Action: jira4claude.ActionUpdated, Changes: map[string]jira4claude.Change{
		"summary": {From: "<old>", To: "R&D"},
	}})
	assert.Contains(t, out.String(), `"from": "<old>"`)
	assert.Contains(t, out.String(), `"to": "R&D"`)
}
