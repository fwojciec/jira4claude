package json_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/fwojciec/jira4claude"
	jsonpkg "github.com/fwojciec/jira4claude/json"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPrinter_Issue(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	p := jsonpkg.NewPrinter(&out)

	view := jira4claude.IssueView{
		Key:     "TEST-123",
		Summary: "Test issue",
		Status:  "Open",
		Type:    "Task",
		Created: "2024-01-01T12:00:00Z",
		Updated: "2024-01-02T12:00:00Z",
	}

	p.Issue(view)

	var result map[string]any
	err := json.Unmarshal(out.Bytes(), &result)
	require.NoError(t, err)
	assert.Equal(t, "TEST-123", result["key"])
	assert.Equal(t, "Test issue", result["summary"])
	assert.Equal(t, "Open", result["status"])
}

func TestPrinter_Issue_WithAssignee(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	p := jsonpkg.NewPrinter(&out)

	view := jira4claude.IssueView{
		Key:      "TEST-123",
		Summary:  "Test issue",
		Status:   "Open",
		Assignee: "John Doe",
		Created:  "2024-01-01T12:00:00Z",
		Updated:  "2024-01-02T12:00:00Z",
	}

	p.Issue(view)

	var result map[string]any
	err := json.Unmarshal(out.Bytes(), &result)
	require.NoError(t, err)
	assert.Equal(t, "John Doe", result["assignee"])
}

func TestPrinter_Issue_WithReporter(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	p := jsonpkg.NewPrinter(&out)

	view := jira4claude.IssueView{
		Key:      "TEST-123",
		Summary:  "Test issue",
		Status:   "Open",
		Reporter: "Jane Smith",
		Created:  "2024-01-01T12:00:00Z",
		Updated:  "2024-01-02T12:00:00Z",
	}

	p.Issue(view)

	var result map[string]any
	err := json.Unmarshal(out.Bytes(), &result)
	require.NoError(t, err)
	assert.Equal(t, "Jane Smith", result["reporter"])
}

func TestPrinter_Issue_WithRelatedIssues(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	p := jsonpkg.NewPrinter(&out)

	view := jira4claude.IssueView{
		Key:     "TEST-123",
		Summary: "Test issue",
		Status:  "Open",
		Created: "2024-01-01T12:00:00Z",
		Updated: "2024-01-02T12:00:00Z",
		RelatedIssues: []jira4claude.RelatedIssueView{
			{
				Relationship: "blocks",
				Key:          "TEST-456",
				Type:         "Task",
				Status:       "To Do",
				Summary:      "Blocked issue",
			},
		},
	}

	p.Issue(view)

	var result map[string]any
	err := json.Unmarshal(out.Bytes(), &result)
	require.NoError(t, err)
	related, ok := result["relatedIssues"].([]any)
	require.True(t, ok, "relatedIssues should be an array")
	require.Len(t, related, 1)

	rel := related[0].(map[string]any)
	assert.Equal(t, "blocks", rel["relationship"])
	assert.Equal(t, "TEST-456", rel["key"])
	assert.Equal(t, "Task", rel["type"])
}

func TestPrinter_Issue_EmptyRelatedIssuesIsArray(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	p := jsonpkg.NewPrinter(&out)

	view := jira4claude.IssueView{
		Key:           "TEST-123",
		Summary:       "Test issue",
		Status:        "Open",
		Type:          "Task",
		Created:       "2024-01-01T12:00:00Z",
		Updated:       "2024-01-02T12:00:00Z",
		RelatedIssues: []jira4claude.RelatedIssueView{}, // Explicitly empty
	}

	p.Issue(view)

	var result map[string]any
	err := json.Unmarshal(out.Bytes(), &result)
	require.NoError(t, err)

	// relatedIssues should always be present as an array, never omitted
	related, exists := result["relatedIssues"]
	require.True(t, exists, "relatedIssues field must always be present")

	arr, ok := related.([]any)
	require.True(t, ok, "relatedIssues must be an array, not null")
	assert.Empty(t, arr, "relatedIssues should be an empty array")
}

func TestPrinter_Issue_NilRelatedIssuesBecomesArray(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	p := jsonpkg.NewPrinter(&out)

	view := jira4claude.IssueView{
		Key:     "TEST-123",
		Summary: "Test issue",
		Status:  "Open",
		Type:    "Task",
		Created: "2024-01-01T12:00:00Z",
		Updated: "2024-01-02T12:00:00Z",
		// RelatedIssues intentionally NOT set (nil)
	}

	p.Issue(view)

	var result map[string]any
	err := json.Unmarshal(out.Bytes(), &result)
	require.NoError(t, err)

	// relatedIssues should always be present as an array, even when nil
	related, exists := result["relatedIssues"]
	require.True(t, exists, "relatedIssues field must always be present")

	arr, ok := related.([]any)
	require.True(t, ok, "relatedIssues must be an array, not null")
	assert.Empty(t, arr, "relatedIssues should be an empty array")
}

func TestPrinter_Issue_WithComments(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	p := jsonpkg.NewPrinter(&out)

	view := jira4claude.IssueView{
		Key:     "TEST-123",
		Summary: "Test issue",
		Status:  "Open",
		Created: "2024-01-01T12:00:00Z",
		Updated: "2024-01-02T12:00:00Z",
		Comments: []jira4claude.CommentView{
			{
				ID:      "10001",
				Author:  "John Doe",
				Body:    "First comment",
				Created: "2024-01-15T10:30:00Z",
			},
			{
				ID:      "10002",
				Author:  "",
				Body:    "Second comment",
				Created: "2024-01-16T14:00:00Z",
			},
		},
	}

	p.Issue(view)

	var result map[string]any
	err := json.Unmarshal(out.Bytes(), &result)
	require.NoError(t, err)

	comments, ok := result["comments"].([]any)
	require.True(t, ok, "comments should be an array")
	require.Len(t, comments, 2)

	comment1 := comments[0].(map[string]any)
	assert.Equal(t, "10001", comment1["id"])
	assert.Equal(t, "First comment", comment1["body"])
	assert.Equal(t, "John Doe", comment1["author"])

	comment2 := comments[1].(map[string]any)
	assert.Equal(t, "10002", comment2["id"])
	assert.Equal(t, "Second comment", comment2["body"])
}

func TestPrinter_Issues(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	p := jsonpkg.NewPrinter(&out)

	items := []jira4claude.IssueListItem{
		{Key: "TEST-1", Summary: "First", Status: "Open"},
		{Key: "TEST-2", Summary: "Second", Status: "Done"},
	}

	p.Issues(jira4claude.IssueListView{Issues: items})

	var result struct {
		Issues    []map[string]any `json:"issues"`
		Truncated *bool            `json:"truncated"`
	}
	err := json.Unmarshal(out.Bytes(), &result)
	require.NoError(t, err)
	assert.Len(t, result.Issues, 2)
	assert.Equal(t, "TEST-1", result.Issues[0]["key"])
	assert.Equal(t, "TEST-2", result.Issues[1]["key"])
	require.NotNil(t, result.Truncated, "truncated must always be present")
	assert.False(t, *result.Truncated)
}

func TestPrinter_Issues_TypeAndAssignee(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	p := jsonpkg.NewPrinter(&out)

	p.Issues(jira4claude.IssueListView{Issues: []jira4claude.IssueListItem{
		{Key: "TEST-1", Status: "Open", Type: "Bug", Priority: "Blocker", Assignee: "Jane Doe", Summary: "First"},
		{Key: "TEST-2", Status: "Open", Type: "Epic", Summary: "Second"},
	}})

	assert.JSONEq(t, `{"issues":[
		{"key":"TEST-1","status":"Open","type":"Bug","priority":"Blocker","assignee":"Jane Doe","summary":"First"},
		{"key":"TEST-2","status":"Open","type":"Epic","summary":"Second"}
	],"truncated":false}`, out.String())
}

func TestPrinter_Issues_Truncated(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	p := jsonpkg.NewPrinter(&out)

	p.Issues(jira4claude.IssueListView{
		Issues:    []jira4claude.IssueListItem{{Key: "TEST-1", Summary: "First", Status: "Open"}},
		Truncated: true,
	})

	var result map[string]any
	err := json.Unmarshal(out.Bytes(), &result)
	require.NoError(t, err)
	assert.Equal(t, true, result["truncated"])
	assert.Len(t, result["issues"], 1)
}

func TestPrinter_Issues_Empty(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	p := jsonpkg.NewPrinter(&out)

	p.Issues(jira4claude.IssueListView{})

	var result map[string]json.RawMessage
	err := json.Unmarshal(out.Bytes(), &result)
	require.NoError(t, err)
	assert.JSONEq(t, `[]`, string(result["issues"]), "empty list must be [] not null")
	assert.JSONEq(t, `false`, string(result["truncated"]))
}

func TestPrinter_Transitions(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	p := jsonpkg.NewPrinter(&out)

	transitions := []*jira4claude.Transition{
		{ID: "1", Name: "Start", To: "In Progress"},
		{ID: "2", Name: "Odd"},
	}

	p.Transitions("TEST-123", transitions)

	assert.JSONEq(t, `[
		{"id": "1", "name": "Start", "to": "In Progress"},
		{"id": "2", "name": "Odd", "to": null}
	]`, out.String(), "to is null when Jira names no target")
}

func TestPrinter_Links(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	p := jsonpkg.NewPrinter(&out)

	links := []jira4claude.RelatedIssueView{
		{
			Relationship: "blocks",
			Key:          "TEST-456",
			Type:         "Task",
			Status:       "To Do",
			Summary:      "Blocked issue",
		},
	}

	p.Links("TEST-123", links)

	var result []map[string]any
	err := json.Unmarshal(out.Bytes(), &result)
	require.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, "blocks", result[0]["relationship"])
	assert.Equal(t, "TEST-456", result[0]["key"])
}

func TestPrinter_Links_WithInwardIssue(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	p := jsonpkg.NewPrinter(&out)

	links := []jira4claude.RelatedIssueView{
		{
			Relationship: "is blocked by",
			Key:          "TEST-789",
			Type:         "Bug",
			Status:       "In Progress",
			Summary:      "Blocking issue",
		},
	}

	p.Links("TEST-123", links)

	var result []map[string]any
	err := json.Unmarshal(out.Bytes(), &result)
	require.NoError(t, err)
	require.Len(t, result, 1)

	link := result[0]
	assert.Equal(t, "is blocked by", link["relationship"])
	assert.Equal(t, "TEST-789", link["key"])
	assert.Equal(t, "Bug", link["type"])
	assert.Equal(t, "In Progress", link["status"])
}

func assertJSONLines(t *testing.T, got string, want ...string) {
	t.Helper()
	require.True(t, strings.HasSuffix(got, "\n"), "output must end in a newline: %q", got)
	lines := strings.Split(strings.TrimSuffix(got, "\n"), "\n")
	require.Len(t, lines, len(want), "output: %q", got)
	for i, line := range lines {
		assert.True(t, strings.HasPrefix(line, "{") && strings.HasSuffix(line, "}"), "line %d: %q", i, line)
		assert.JSONEq(t, want[i], line)
	}
}

func TestPrinter_Error(t *testing.T) {
	t.Parallel()

	t.Run("writes one JSON line to stderr and nothing to stdout", func(t *testing.T) {
		t.Parallel()

		var out, errOut bytes.Buffer
		p := jsonpkg.NewPrinterWithIO(&out, &errOut)

		p.Error(errors.New("something went wrong"))

		assert.Empty(t, out.String(), "errors must not go to stdout")
		assertJSONLines(t, errOut.String(),
			`{"level":"error","error":true,"code":"internal","message":"something went wrong"}`)
	})

	t.Run("carries the error code", func(t *testing.T) {
		t.Parallel()

		var out, errOut bytes.Buffer
		p := jsonpkg.NewPrinterWithIO(&out, &errOut)

		p.Error(&jira4claude.Error{Code: jira4claude.ENotFound, Message: "Issue not found"})

		assert.Empty(t, out.String())
		assertJSONLines(t, errOut.String(),
			`{"level":"error","error":true,"code":"not_found","message":"Issue not found"}`)
	})

	t.Run("a multi-line message stays on one line", func(t *testing.T) {
		t.Parallel()

		var out, errOut bytes.Buffer
		p := jsonpkg.NewPrinterWithIO(&out, &errOut)

		p.Error(errors.New("first\nsecond"))

		assertJSONLines(t, errOut.String(),
			`{"level":"error","error":true,"code":"internal","message":"first\nsecond"}`)
	})

	t.Run("angle brackets are not escaped", func(t *testing.T) {
		t.Parallel()

		var out, errOut bytes.Buffer
		p := jsonpkg.NewPrinterWithIO(&out, &errOut)

		p.Error(&jira4claude.Error{Code: jira4claude.EValidation, Message: `expected "<key>"`})

		assert.Contains(t, errOut.String(), `"<key>`)
		assertJSONLines(t, errOut.String(),
			`{"level":"error","error":true,"code":"validation","message":"expected \"<key>\""}`)
	})

	t.Run("NewPrinter discards errors", func(t *testing.T) {
		t.Parallel()

		var out bytes.Buffer
		p := jsonpkg.NewPrinter(&out)

		p.Error(errors.New("something went wrong"))

		assert.Empty(t, out.String())
	})
}

func TestPrinter_Issue_WithURL(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	p := jsonpkg.NewPrinter(&out)

	view := jira4claude.IssueView{
		Key:     "TEST-123",
		Summary: "Test issue",
		Status:  "Open",
		Type:    "Task",
		Created: "2024-01-01T12:00:00Z",
		Updated: "2024-01-02T12:00:00Z",
		URL:     "https://example.atlassian.net/browse/TEST-123",
	}

	p.Issue(view)

	var result map[string]any
	err := json.Unmarshal(out.Bytes(), &result)
	require.NoError(t, err)
	assert.Equal(t, "https://example.atlassian.net/browse/TEST-123", result["url"])
}

func TestPrinter_Issue_NoURLWhenEmpty(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	p := jsonpkg.NewPrinter(&out)

	view := jira4claude.IssueView{
		Key:     "TEST-123",
		Summary: "Test issue",
		Status:  "Open",
		Type:    "Task",
		Created: "2024-01-01T12:00:00Z",
		Updated: "2024-01-02T12:00:00Z",
		// URL not set
	}

	p.Issue(view)

	var result map[string]any
	err := json.Unmarshal(out.Bytes(), &result)
	require.NoError(t, err)
	_, hasURL := result["url"]
	assert.False(t, hasURL, "url should not be present when empty")
}

func TestPrinter_Comment(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	p := jsonpkg.NewPrinter(&out)

	view := jira4claude.CommentView{
		ID:      "10001",
		Author:  "John Doe",
		Body:    "This is a test comment",
		Created: "2024-01-15T10:30:00Z",
	}

	p.Comment(view)

	var result map[string]any
	err := json.Unmarshal(out.Bytes(), &result)
	require.NoError(t, err)
	assert.Equal(t, "10001", result["id"])
	assert.Equal(t, "This is a test comment", result["body"])
	assert.Equal(t, "John Doe", result["author"])
}

func TestPrinter_Comment_WithoutAuthor(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	p := jsonpkg.NewPrinter(&out)

	view := jira4claude.CommentView{
		ID:      "10002",
		Author:  "",
		Body:    "Comment without author",
		Created: "2024-01-15T10:30:00Z",
	}

	p.Comment(view)

	var result map[string]any
	err := json.Unmarshal(out.Bytes(), &result)
	require.NoError(t, err)
	assert.Equal(t, "10002", result["id"])
	assert.Equal(t, "Comment without author", result["body"])
	// Author is empty string; without omitempty it appears as "author": ""
	assert.Empty(t, result["author"])
}

func TestPrinter_Warning(t *testing.T) {
	t.Parallel()

	t.Run("writes one JSON line to stderr and nothing to stdout", func(t *testing.T) {
		t.Parallel()

		var out, errOut bytes.Buffer
		p := jsonpkg.NewPrinterWithIO(&out, &errOut)

		p.Warning("showing the first 2 issues; more match")

		assert.Empty(t, out.String(), "warnings must not go to stdout")
		assertJSONLines(t, errOut.String(),
			`{"level":"warning","message":"showing the first 2 issues; more match"}`)
	})

	t.Run("each warning is its own line", func(t *testing.T) {
		t.Parallel()

		var out, errOut bytes.Buffer
		p := jsonpkg.NewPrinterWithIO(&out, &errOut)

		p.Warning("first warning")
		p.Warning("second warning")

		assertJSONLines(t, errOut.String(),
			`{"level":"warning","message":"first warning"}`,
			`{"level":"warning","message":"second warning"}`)
	})
}

func TestPrinter_ConversionIssue(t *testing.T) {
	t.Parallel()

	t.Run("writes a JSON warning with kind, node and detail to stderr", func(t *testing.T) {
		t.Parallel()

		var out, errOut bytes.Buffer
		p := jsonpkg.NewPrinterWithIO(&out, &errOut)

		p.ConversionIssue(jira4claude.ConversionIssue{
			Kind:   jira4claude.ConversionDropped,
			Node:   "Image",
			Detail: "skipped unsupported node type 'Image'",
		})

		assert.Empty(t, out.String(), "conversion issues must not go to stdout")
		assertJSONLines(t, errOut.String(),
			`{"level":"warning","message":"dropped Image: skipped unsupported node type 'Image'",`+
				`"kind":"dropped","node":"Image","detail":"skipped unsupported node type 'Image'"}`)
	})

	t.Run("a token issue also carries the token", func(t *testing.T) {
		t.Parallel()

		var out, errOut bytes.Buffer
		p := jsonpkg.NewPrinterWithIO(&out, &errOut)

		p.ConversionIssue(jira4claude.ConversionIssue{
			Kind:     jira4claude.ConversionDropped,
			Node:     "{{mention:2 Jane Doe}}",
			Token:    "{{mention:2 Jane Doe}}",
			Detail:   "the token is not in the new text",
			Existing: true,
		})

		assertJSONLines(t, errOut.String(),
			`{"level":"warning","message":"dropped {{mention:2 Jane Doe}}: the token is not in the new text",`+
				`"kind":"dropped","node":"{{mention:2 Jane Doe}}","token":"{{mention:2 Jane Doe}}",`+
				`"detail":"the token is not in the new text","existing":true}`)
	})

	t.Run("writes degraded issues", func(t *testing.T) {
		t.Parallel()

		var out, errOut bytes.Buffer
		p := jsonpkg.NewPrinterWithIO(&out, &errOut)

		p.ConversionIssue(jira4claude.ConversionIssue{
			Kind:   jira4claude.ConversionDegraded,
			Node:   "emoji",
			Detail: "rendered as text",
		})

		assertJSONLines(t, errOut.String(),
			`{"level":"warning","message":"degraded emoji: rendered as text",`+
				`"kind":"degraded","node":"emoji","detail":"rendered as text"}`)
	})

	t.Run("marks an issue about existing content", func(t *testing.T) {
		t.Parallel()

		var out, errOut bytes.Buffer
		p := jsonpkg.NewPrinterWithIO(&out, &errOut)

		p.ConversionIssue(jira4claude.ConversionIssue{
			Kind:     jira4claude.ConversionDropped,
			Node:     "media",
			Detail:   "removed",
			Existing: true,
		})

		assertJSONLines(t, errOut.String(),
			`{"level":"warning","message":"dropped media: removed",`+
				`"kind":"dropped","node":"media","detail":"removed","existing":true}`)
	})
}

// decodeFieldsArray decodes the {source, scope, omitted, fields} wrapper
// emitted by Printer.Fields and returns the fields array for inspection.
func decodeFieldsArray(t *testing.T, b []byte) []map[string]any {
	t.Helper()
	var wrapper struct {
		Fields []map[string]any `json:"fields"`
	}
	require.NoError(t, json.Unmarshal(b, &wrapper))
	return wrapper.Fields
}

func TestPrinter_Fields(t *testing.T) {
	t.Parallel()

	t.Run("output is a wrapper object with scope, omitted, and fields", func(t *testing.T) {
		t.Parallel()
		var out bytes.Buffer
		p := jsonpkg.NewPrinter(&out)

		view := jira4claude.IssueFieldsView{
			Source:  "INT / Bug",
			Scope:   jira4claude.FieldScopeDefault,
			Omitted: 14,
			Fields: []*jira4claude.IssueField{
				{ID: "summary", Name: "Summary", Required: true, SchemaType: "string"},
			},
		}
		p.Fields(view)

		var result struct {
			Source  string           `json:"source"`
			Scope   string           `json:"scope"`
			Omitted int              `json:"omitted"`
			Fields  []map[string]any `json:"fields"`
		}
		err := json.Unmarshal(out.Bytes(), &result)
		require.NoError(t, err, "output should decode as a JSON object")
		assert.Equal(t, "INT / Bug", result.Source)
		assert.Equal(t, "default", result.Scope)
		assert.Equal(t, 14, result.Omitted)

		require.Len(t, result.Fields, 1)
		assert.Equal(t, "summary", result.Fields[0]["id"])
	})

	t.Run("empty fields emit [] not null", func(t *testing.T) {
		t.Parallel()
		var out bytes.Buffer
		p := jsonpkg.NewPrinter(&out)

		p.Fields(jira4claude.IssueFieldsView{Source: "INT / Bug", Scope: jira4claude.FieldScopeFiltered})

		var result map[string]any
		err := json.Unmarshal(out.Bytes(), &result)
		require.NoError(t, err)
		fields, ok := result["fields"].([]any)
		require.True(t, ok, "fields should be an array, not null")
		assert.Empty(t, fields)
	})

	t.Run("emits lowercase JSON keys for required base fields", func(t *testing.T) {
		t.Parallel()
		var out bytes.Buffer
		p := jsonpkg.NewPrinter(&out)

		view := jira4claude.IssueFieldsView{
			Source: "INT / Bug",
			Fields: []*jira4claude.IssueField{
				{ID: "customfield_10801", Name: "Urgency / Risk", Required: true, SchemaType: "option"},
			},
		}
		p.Fields(view)

		result := decodeFieldsArray(t, out.Bytes())
		require.Len(t, result, 1)
		assert.Equal(t, "customfield_10801", result[0]["id"])
		assert.Equal(t, "Urgency / Risk", result[0]["name"])
		assert.Equal(t, true, result[0]["required"])
		assert.Equal(t, "option", result[0]["schemaType"])
	})

	t.Run("omitempty drops allowedValues and example when absent", func(t *testing.T) {
		t.Parallel()
		var out bytes.Buffer
		p := jsonpkg.NewPrinter(&out)

		view := jira4claude.IssueFieldsView{
			Source: "INT / Bug",
			Fields: []*jira4claude.IssueField{
				{ID: "summary", Name: "Summary", Required: true, SchemaType: "string"},
			},
		}
		p.Fields(view)

		result := decodeFieldsArray(t, out.Bytes())
		require.Len(t, result, 1)
		_, hasAllowed := result[0]["allowedValues"]
		_, hasExample := result[0]["example"]
		_, hasItems := result[0]["schemaItems"]
		_, hasCustom := result[0]["schemaCustom"]
		assert.False(t, hasAllowed, "allowedValues should be omitted when empty")
		assert.False(t, hasExample, "example should be omitted when nil")
		assert.False(t, hasItems, "schemaItems should be omitted when empty")
		assert.False(t, hasCustom, "schemaCustom should be omitted when empty")
	})

	t.Run("example is emitted as a JSON value, not a string", func(t *testing.T) {
		t.Parallel()
		var out bytes.Buffer
		p := jsonpkg.NewPrinter(&out)

		exampleRaw, err := json.Marshal(map[string]any{"value": "High"})
		require.NoError(t, err)

		view := jira4claude.IssueFieldsView{
			Source: "INT / Bug",
			Fields: []*jira4claude.IssueField{
				{
					ID: "customfield_10801", Name: "Urgency / Risk", Required: true, SchemaType: "option",
					Example: json.RawMessage(exampleRaw),
				},
			},
		}
		p.Fields(view)

		result := decodeFieldsArray(t, out.Bytes())
		require.Len(t, result, 1)
		ex, ok := result[0]["example"].(map[string]any)
		require.True(t, ok, "example should decode as a JSON object, not a string")
		assert.Equal(t, "High", ex["value"])
	})

	t.Run("allowedValues project as id+value entries", func(t *testing.T) {
		t.Parallel()
		var out bytes.Buffer
		p := jsonpkg.NewPrinter(&out)

		view := jira4claude.IssueFieldsView{
			Source: "INT / Bug",
			Fields: []*jira4claude.IssueField{
				{
					ID: "customfield_10801", Name: "Urgency / Risk", Required: true, SchemaType: "option",
					AllowedValues: []jira4claude.FieldAllowedValue{
						{ID: "10500", Value: "High"},
						{ID: "10501", Value: "Medium"},
					},
				},
			},
		}
		p.Fields(view)

		result := decodeFieldsArray(t, out.Bytes())
		require.Len(t, result, 1)
		allowed, ok := result[0]["allowedValues"].([]any)
		require.True(t, ok)
		require.Len(t, allowed, 2)

		first, ok := allowed[0].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "10500", first["id"])
		assert.Equal(t, "High", first["value"])

		second, ok := allowed[1].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "10501", second["id"])
		assert.Equal(t, "Medium", second["value"])
	})

	t.Run("FieldAllowedValue.id omitempty drops when empty", func(t *testing.T) {
		t.Parallel()
		var out bytes.Buffer
		p := jsonpkg.NewPrinter(&out)

		view := jira4claude.IssueFieldsView{
			Source: "INT / Bug",
			Fields: []*jira4claude.IssueField{
				{
					ID: "customfield_10838", Name: "Product Area(s)", Required: true,
					SchemaType: "array", SchemaItems: "option",
					AllowedValues: []jira4claude.FieldAllowedValue{
						{Value: "Integrations"}, // no ID -> should omit "id" key
					},
				},
			},
		}
		p.Fields(view)

		result := decodeFieldsArray(t, out.Bytes())
		require.Len(t, result, 1)

		// schemaItems should be present since SchemaType == "array"
		assert.Equal(t, "option", result[0]["schemaItems"])

		allowed, ok := result[0]["allowedValues"].([]any)
		require.True(t, ok)
		require.Len(t, allowed, 1)
		first, ok := allowed[0].(map[string]any)
		require.True(t, ok)
		_, hasID := first["id"]
		assert.False(t, hasID, "id should be omitted for primitive-string allowed values")
		assert.Equal(t, "Integrations", first["value"])
	})

	t.Run("nil fields list emits [] (not null) so consumers can always iterate", func(t *testing.T) {
		t.Parallel()
		var out bytes.Buffer
		p := jsonpkg.NewPrinter(&out)

		p.Fields(jira4claude.IssueFieldsView{Source: "INT / Bug", Fields: nil})

		// Byte-assert on the raw output: json.Unmarshal can't distinguish null
		// from [] here (both decode to a nil slice), so we pin the shape directly.
		assert.Contains(t, out.String(), `"fields": []`)
		assert.NotContains(t, out.String(), `"fields": null`)
	})
}

func TestPrinter_Attachments(t *testing.T) {
	t.Parallel()

	t.Run("prints the attachments as an array", func(t *testing.T) {
		t.Parallel()
		var out, errOut bytes.Buffer
		p := jsonpkg.NewPrinterWithIO(&out, &errOut)

		p.Attachments("TEST-1", []jira4claude.AttachmentView{
			{ID: "10000", Filename: "picture.jpg", MimeType: "image/jpeg", Size: 23123, Author: "Mia Krystof", Created: "2022-10-06T07:32:47Z"},
		})

		assert.JSONEq(t, `[{
			"id": "10000",
			"filename": "picture.jpg",
			"mimeType": "image/jpeg",
			"size": 23123,
			"author": "Mia Krystof",
			"created": "2022-10-06T07:32:47Z"
		}]`, out.String())
		assert.Empty(t, errOut.String())
	})

	t.Run("no attachments is an empty array, not null, and nothing on stderr", func(t *testing.T) {
		t.Parallel()
		var out, errOut bytes.Buffer
		p := jsonpkg.NewPrinterWithIO(&out, &errOut)

		p.Attachments("TEST-1", nil)

		assert.JSONEq(t, `[]`, out.String())
		assert.Empty(t, errOut.String())
	})
}

func TestPrinter_SavedAttachments(t *testing.T) {
	t.Parallel()

	t.Run("prints the saved files as an array", func(t *testing.T) {
		t.Parallel()
		var out, errOut bytes.Buffer
		p := jsonpkg.NewPrinterWithIO(&out, &errOut)

		p.SavedAttachments("TEST-1", []jira4claude.SavedAttachmentView{
			{ID: "10000", Filename: "../picture.jpg", Path: "/tmp/dl/picture.jpg", Size: 23123},
		}, 1)

		assert.JSONEq(t, `[{
			"id": "10000",
			"filename": "../picture.jpg",
			"path": "/tmp/dl/picture.jpg",
			"size": 23123
		}]`, out.String())
		assert.Empty(t, errOut.String())
	})

	t.Run("nothing saved is an empty array, not null, and nothing on stderr", func(t *testing.T) {
		t.Parallel()
		var out, errOut bytes.Buffer
		p := jsonpkg.NewPrinterWithIO(&out, &errOut)

		p.SavedAttachments("TEST-1", nil, 0)

		assert.JSONEq(t, `[]`, out.String())
		assert.Empty(t, errOut.String())
	})

	t.Run("nothing saved because every file failed is still an empty array", func(t *testing.T) {
		t.Parallel()
		var out, errOut bytes.Buffer
		p := jsonpkg.NewPrinterWithIO(&out, &errOut)

		p.SavedAttachments("TEST-1", nil, 2)

		assert.JSONEq(t, `[]`, out.String())
		assert.Empty(t, errOut.String())
	})
}

func TestPrinter_Mutation(t *testing.T) {
	t.Parallel()

	t.Run("prints the result as one JSON object on stdout", func(t *testing.T) {
		t.Parallel()
		var out, errOut bytes.Buffer
		p := jsonpkg.NewPrinterWithIO(&out, &errOut)

		p.Mutation(jira4claude.MutationResult{
			Key:    "TEST-1",
			URL:    "https://x.atlassian.net/browse/TEST-1",
			Action: jira4claude.ActionUpdated,
			Changes: map[string]jira4claude.Change{
				"summary":     {From: "Old", To: "New"},
				"assignee":    {From: nil, To: "Jane Doe"},
				"description": {Edit: jira4claude.EditAppended},
			},
			Removed: []string{"{{mention:1 Ann}}"},
			Updated: "2026-11-09T10:15:30.123Z",
		})

		assert.JSONEq(t, `{
			"key": "TEST-1",
			"url": "https://x.atlassian.net/browse/TEST-1",
			"action": "updated",
			"changes": {
				"summary": {"from": "Old", "to": "New"},
				"assignee": {"from": null, "to": "Jane Doe"},
				"description": {"edit": "appended"}
			},
			"removed": ["{{mention:1 Ann}}"],
			"updated": "2026-11-09T10:15:30.123Z"
		}`, out.String())
		assert.Empty(t, errOut.String())
	})

	t.Run("changes is an empty object, never null", func(t *testing.T) {
		t.Parallel()
		var out bytes.Buffer
		p := jsonpkg.NewPrinter(&out)

		p.Mutation(jira4claude.MutationResult{Key: "TEST-1", Action: jira4claude.ActionDeleted})

		assert.JSONEq(t, `{"key":"TEST-1","action":"deleted","changes":{}}`, out.String())
	})

	t.Run("a dry run carries dryRun and the text, an empty text included", func(t *testing.T) {
		t.Parallel()
		var out bytes.Buffer
		p := jsonpkg.NewPrinter(&out)
		empty := ""

		p.Mutation(jira4claude.MutationResult{
			Key: "TEST-1", Action: jira4claude.ActionWouldUpdate, DryRun: true, Text: &empty,
			Changes: map[string]jira4claude.Change{"description": {Edit: jira4claude.EditCleared}},
		})

		assert.JSONEq(t, `{"key":"TEST-1","action":"would_update","dryRun":true,"changes":{"description":{"edit":"cleared"}},"text":""}`, out.String())
	})

	t.Run("the dry run of a create names the project, not a key", func(t *testing.T) {
		t.Parallel()
		var out bytes.Buffer
		p := jsonpkg.NewPrinter(&out)
		text := "Some *text*"

		p.Mutation(jira4claude.MutationResult{Project: "TEST", Action: jira4claude.ActionWouldCreate, DryRun: true, Text: &text})

		assert.JSONEq(t, `{"project":"TEST","action":"would_create","dryRun":true,"changes":{},"text":"Some *text*"}`, out.String())
	})
}

func TestPrinter_Initialized(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	p := jsonpkg.NewPrinterWithIO(&out, &errOut)

	p.Initialized(jira4claude.InitResult{ConfigCreated: true, GitignoreAdded: true})

	assert.JSONEq(t, `{"configCreated":true,"gitignoreAdded":true,"gitignoreExists":false}`, out.String())
	assert.Empty(t, errOut.String())
}

func TestPrinter_ViewConversionIssue(t *testing.T) {
	t.Parallel()

	t.Run("a view warning carries the issue key with every conversion field", func(t *testing.T) {
		t.Parallel()

		var out, errOut bytes.Buffer
		p := jsonpkg.NewPrinterWithIO(&out, &errOut)

		p.ViewConversionIssue(jira4claude.ViewConversionIssue{
			ConversionIssue: jira4claude.ConversionIssue{
				Kind:     jira4claude.ConversionDropped,
				Node:     "{{mention:2 Jane Doe}}",
				Token:    "{{mention:2 Jane Doe}}",
				Detail:   "the token does not read back",
				Existing: true,
			},
			Key: "TEST-1",
		})

		assert.Empty(t, out.String())
		assertJSONLines(t, errOut.String(),
			`{"level":"warning","message":"dropped {{mention:2 Jane Doe}}: the token does not read back",`+
				`"kind":"dropped","node":"{{mention:2 Jane Doe}}","token":"{{mention:2 Jane Doe}}",`+
				`"detail":"the token does not read back","existing":true,"key":"TEST-1"}`)
	})
}
