package json_test

import (
	"bytes"
	"encoding/json"
	"errors"
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

	p.Issues(items)

	var result []map[string]any
	err := json.Unmarshal(out.Bytes(), &result)
	require.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Equal(t, "TEST-1", result[0]["key"])
	assert.Equal(t, "TEST-2", result[1]["key"])
}

func TestPrinter_Issues_Empty(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	p := jsonpkg.NewPrinter(&out)

	p.Issues([]jira4claude.IssueListItem{})

	var result []map[string]any
	err := json.Unmarshal(out.Bytes(), &result)
	require.NoError(t, err)
	assert.Empty(t, result)
}

func TestPrinter_Transitions(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	p := jsonpkg.NewPrinter(&out)

	transitions := []*jira4claude.Transition{
		{ID: "1", Name: "In Progress"},
		{ID: "2", Name: "Done"},
	}

	p.Transitions("TEST-123", transitions)

	var result []map[string]any
	err := json.Unmarshal(out.Bytes(), &result)
	require.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Equal(t, "1", result[0]["id"])
	assert.Equal(t, "In Progress", result[0]["name"])
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

func TestPrinter_Success(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	p := jsonpkg.NewPrinter(&out)

	p.Success("Created issue", "TEST-123")

	var result map[string]any
	err := json.Unmarshal(out.Bytes(), &result)
	require.NoError(t, err)
	assert.Equal(t, true, result["success"])
	assert.Equal(t, "Created issue", result["message"])
	keys, ok := result["keys"].([]any)
	require.True(t, ok)
	assert.Contains(t, keys, "TEST-123")
}

func TestPrinter_Success_NoKeys(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	p := jsonpkg.NewPrinter(&out)

	p.Success("Operation complete")

	var result map[string]any
	err := json.Unmarshal(out.Bytes(), &result)
	require.NoError(t, err)
	assert.Equal(t, true, result["success"])
	assert.Equal(t, "Operation complete", result["message"])
	_, hasKeys := result["keys"]
	assert.False(t, hasKeys, "keys should not be present when no keys provided")
}

func TestPrinter_Error(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	p := jsonpkg.NewPrinter(&out)

	p.Error(errors.New("something went wrong"))

	var result map[string]any
	err := json.Unmarshal(out.Bytes(), &result)
	require.NoError(t, err)
	assert.Equal(t, true, result["error"])
	assert.Equal(t, "something went wrong", result["message"])
}

func TestPrinter_Error_WithCode(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	p := jsonpkg.NewPrinter(&out)

	appErr := &jira4claude.Error{
		Code:    jira4claude.ENotFound,
		Message: "Issue not found",
	}

	p.Error(appErr)

	var result map[string]any
	err := json.Unmarshal(out.Bytes(), &result)
	require.NoError(t, err)
	assert.Equal(t, true, result["error"])
	assert.Equal(t, jira4claude.ENotFound, result["code"])
	assert.Equal(t, "Issue not found", result["message"])
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

func TestPrinter_Success_WithServerURL(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	p := jsonpkg.NewPrinter(&out)
	p.SetServerURL("https://example.atlassian.net")

	p.Success("Created:", "TEST-123")

	var result map[string]any
	err := json.Unmarshal(out.Bytes(), &result)
	require.NoError(t, err)
	assert.Equal(t, true, result["success"])
	assert.Equal(t, "Created:", result["message"])
	urls, ok := result["urls"].([]any)
	require.True(t, ok, "urls should be an array")
	assert.Contains(t, urls, "https://example.atlassian.net/browse/TEST-123")
}

func TestPrinter_Success_NoURLsWhenNoKeys(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	p := jsonpkg.NewPrinter(&out)
	p.SetServerURL("https://example.atlassian.net")

	p.Success("Operation complete")

	var result map[string]any
	err := json.Unmarshal(out.Bytes(), &result)
	require.NoError(t, err)
	_, hasURLs := result["urls"]
	assert.False(t, hasURLs, "urls should not be present when no keys provided")
}

func TestPrinter_Success_ShowsMultipleURLsForMultipleKeys(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	p := jsonpkg.NewPrinter(&out)
	p.SetServerURL("https://example.atlassian.net")

	p.Success("Created:", "TEST-1", "TEST-2", "TEST-3")

	var result map[string]any
	err := json.Unmarshal(out.Bytes(), &result)
	require.NoError(t, err)
	urls, ok := result["urls"].([]any)
	require.True(t, ok, "urls should be an array")
	require.Len(t, urls, 3)
	assert.Equal(t, "https://example.atlassian.net/browse/TEST-1", urls[0])
	assert.Equal(t, "https://example.atlassian.net/browse/TEST-2", urls[1])
	assert.Equal(t, "https://example.atlassian.net/browse/TEST-3", urls[2])
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

func TestPrinter_Warning_WritesToStderr(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	p := jsonpkg.NewPrinterWithIO(&out, &errOut)

	p.Warning("unsupported element skipped")

	// Warning should go to stderr, not stdout
	assert.Empty(t, out.String(), "warnings should not go to stdout")
	assert.Contains(t, errOut.String(), "warning:")
	assert.Contains(t, errOut.String(), "unsupported element skipped")
}

func TestPrinter_Warning_NotInJSONFormat(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	p := jsonpkg.NewPrinterWithIO(&out, &errOut)

	p.Warning("unsupported element skipped")

	// Warning should be plain text, not JSON
	errOutput := errOut.String()
	assert.NotContains(t, errOutput, "{", "warnings should be plain text, not JSON")
}

func TestPrinter_Warning_MultipleWarnings(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	p := jsonpkg.NewPrinterWithIO(&out, &errOut)

	p.Warning("first warning")
	p.Warning("second warning")

	errOutput := errOut.String()
	assert.Contains(t, errOutput, "first warning")
	assert.Contains(t, errOutput, "second warning")
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
