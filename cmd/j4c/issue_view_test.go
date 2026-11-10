package main_test

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/fwojciec/jira4claude"
	"github.com/fwojciec/jira4claude/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// issue view KEY... --field NAME tests (jir-hodd). They drive main.Run with
// the real Markdown converter and printers.

// viewIssue returns a fully populated issue with the given key.
func viewIssue(key string) *jira4claude.Issue {
	return &jira4claude.Issue{
		Key:      key,
		Project:  "TEST",
		Summary:  "Fix the login",
		Status:   "In Progress",
		Type:     "Bug",
		Priority: "High",
		Assignee: &jira4claude.User{DisplayName: "Jane Doe"},
		Reporter: &jira4claude.User{DisplayName: "John Roe"},
		Labels:   []string{"backend", "urgent"},
		Description: &jira4claude.ADFNode{Type: "doc", Version: 1, Content: []jira4claude.ADFNode{
			{Type: "paragraph", Content: []jira4claude.ADFNode{{Type: "text", Text: "Steps to reproduce."}}},
		}},
		Comments: []*jira4claude.Comment{{
			ID:      "10001",
			Author:  &jira4claude.User{DisplayName: "John Roe"},
			Body:    &jira4claude.ADFNode{Type: "doc", Version: 1, Content: []jira4claude.ADFNode{{Type: "paragraph", Content: []jira4claude.ADFNode{{Type: "text", Text: "Looks good"}}}}},
			Created: time.Date(2026, 11, 1, 9, 0, 0, 0, time.UTC),
		}},
		Parent:  &jira4claude.LinkedIssue{Key: "TEST-100", Summary: "Auth", Status: "To Do", Type: "Epic"},
		Created: time.Date(2026, 11, 1, 8, 0, 0, 0, time.UTC),
		Updated: time.Date(2026, 11, 9, 10, 15, 30, 123_000_000, time.UTC),
		ReadCustomFields: map[string]jira4claude.CustomFieldValue{
			"customfield_10016": {Name: "Story Points", Value: json.RawMessage(`5`)},
			"customfield_10020": {Name: "Team", Value: json.RawMessage(`{"id": "1", "name": "Core"}`)},
		},
	}
}

// bareIssue returns TEST-1 with no optional fields set.
func bareIssue() *jira4claude.Issue {
	return &jira4claude.Issue{Key: "TEST-1", Summary: "Bare", Status: "To Do", Type: "Task"}
}

// issueStore serves the issues it holds by key and fails the others with the
// error set for the key (not found by default).
func issueStore(issues map[string]*jira4claude.Issue, errs map[string]error) *mock.IssueService {
	return &mock.IssueService{
		GetFn: func(ctx context.Context, key string) (*jira4claude.Issue, error) {
			if err, ok := errs[key]; ok {
				return nil, err
			}
			if issue, ok := issues[key]; ok {
				return issue, nil
			}
			return nil, &jira4claude.Error{Code: jira4claude.ENotFound, Message: "issue not found"}
		},
	}
}

func twoIssues() map[string]*jira4claude.Issue {
	second := viewIssue("TEST-2")
	second.Summary = "Second"
	second.Status = "Done"
	return map[string]*jira4claude.Issue{"TEST-1": viewIssue("TEST-1"), "TEST-2": second}
}

// stderrLines decodes stderr as one JSON object per line.
func stderrLines(t *testing.T, stderr string) []map[string]any {
	t.Helper()
	raw := strings.Split(strings.TrimSuffix(stderr, "\n"), "\n")
	lines := make([]map[string]any, 0, len(raw))
	for _, line := range raw {
		var got map[string]any
		require.NoError(t, json.Unmarshal([]byte(line), &got), "line: %q", line)
		lines = append(lines, got)
	}
	return lines
}

// assertJSONInOrder asserts that got is the JSON document want with its keys
// in the same order, which assert.JSONEq alone does not check.
func assertJSONInOrder(t *testing.T, want, got string) {
	t.Helper()
	assert.JSONEq(t, want, got)
	assert.Equal(t, compactJSON(t, want), compactJSON(t, got), "key order")
}

func compactJSON(t *testing.T, s string) string {
	t.Helper()
	var buf bytes.Buffer
	require.NoError(t, json.Compact(&buf, []byte(s)), "JSON: %q", s)
	return buf.String()
}

const viewIssuePage = "# TEST-1: Fix the login\n\n" +
	"**Type:** Bug\n" +
	"**Status:** In Progress\n" +
	"**Priority:** High\n" +
	"**Assignee:** Jane Doe\n" +
	"**Reporter:** John Roe\n" +
	"**Parent:** TEST-100\n" +
	"**Labels:** backend, urgent\n" +
	"\nSteps to reproduce.\n" +
	"\n## Custom Fields\n\n" +
	"- **Story Points:** 5\n" +
	"- **Team:** {\"id\":\"1\",\"name\":\"Core\"}\n" +
	"\n## Comments\n\n" +
	"**John Roe** (2026-11-01 09:00, id 10001):\nLooks good\n" +
	"\n[View in Jira](https://test.atlassian.net/browse/TEST-1)\n"

const viewIssueJSON = `{
  "key": "TEST-1",
  "project": "TEST",
  "summary": "Fix the login",
  "description": "Steps to reproduce.",
  "status": "In Progress",
  "type": "Bug",
  "priority": "High",
  "assignee": "Jane Doe",
  "reporter": "John Roe",
  "labels": [
    "backend",
    "urgent"
  ],
  "relatedIssues": [
    {
      "relationship": "parent",
      "key": "TEST-100",
      "type": "Epic",
      "status": "To Do",
      "summary": "Auth"
    }
  ],
  "comments": [
    {
      "id": "10001",
      "author": "John Roe",
      "body": "Looks good",
      "created": "2026-11-01T09:00:00Z"
    }
  ],
  "created": "2026-11-01T08:00:00Z",
  "updated": "2026-11-09T10:15:30.123Z",
  "url": "https://test.atlassian.net/browse/TEST-1",
  "customFields": {
    "customfield_10016": {
      "name": "Story Points",
      "value": 5
    },
    "customfield_10020": {
      "name": "Team",
      "value": {
        "id": "1",
        "name": "Core"
      }
    }
  }
}
`

func TestRun_IssueViewOneKeyUnchanged(t *testing.T) {
	t.Parallel()

	t.Run("text output is the full page", func(t *testing.T) {
		t.Parallel()

		res := runCLI(t, testDeps(issueGetter(viewIssue("TEST-1"), nil), &mock.UserService{}), "issue", "view", "TEST-1")

		assert.Equal(t, 0, res.code)
		assert.Equal(t, viewIssuePage, res.stdout)
		assert.Empty(t, res.stderr)
	})

	t.Run("JSON output is one object", func(t *testing.T) {
		t.Parallel()

		res := runCLI(t, testDeps(issueGetter(viewIssue("TEST-1"), nil), &mock.UserService{}), "--json", "issue", "view", "TEST-1")

		assert.Equal(t, 0, res.code)
		assertJSONInOrder(t, viewIssueJSON, res.stdout)
		assert.Empty(t, res.stderr)
	})
}

func TestRun_IssueViewOneField(t *testing.T) {
	t.Parallel()

	tests := []struct {
		field string
		want  string
	}{
		{"key", "TEST-1\n"},
		{"project", "TEST\n"},
		{"summary", "Fix the login\n"},
		{"description", "Steps to reproduce.\n"},
		{"status", "In Progress\n"},
		{"type", "Bug\n"},
		{"priority", "High\n"},
		{"assignee", "Jane Doe\n"},
		{"reporter", "John Roe\n"},
		{"labels", "backend\nurgent\n"},
		{"relatedIssues", `[{"relationship":"parent","key":"TEST-100","type":"Epic","status":"To Do","summary":"Auth"}]` + "\n"},
		{"comments", `[{"id":"10001","author":"John Roe","body":"Looks good","created":"2026-11-01T09:00:00Z"}]` + "\n"},
		{"created", "2026-11-01T08:00:00Z\n"},
		{"updated", "2026-11-09T10:15:30.123Z\n"},
		{"url", "https://test.atlassian.net/browse/TEST-1\n"},
		{"customfield_10016", "5\n"},
		{"customfield_10020", `{"id":"1","name":"Core"}` + "\n"},
		{"customfield_99999", "null\n"},
		{"Description", "Steps to reproduce.\n"},
		{"RELATEDISSUES", `[{"relationship":"parent","key":"TEST-100","type":"Epic","status":"To Do","summary":"Auth"}]` + "\n"},
	}
	for _, tt := range tests {
		t.Run("text prints the raw "+tt.field, func(t *testing.T) {
			t.Parallel()

			res := runCLI(t, testDeps(issueGetter(viewIssue("TEST-1"), nil), &mock.UserService{}),
				"issue", "view", "TEST-1", "--field", tt.field)

			assert.Equal(t, 0, res.code)
			assert.Equal(t, tt.want, res.stdout)
			assert.Empty(t, res.stderr)
		})
	}

	empty := []string{"description", "priority", "assignee", "reporter", "labels"}
	for _, field := range empty {
		t.Run("text prints nothing for an empty "+field, func(t *testing.T) {
			t.Parallel()

			res := runCLI(t, testDeps(issueGetter(bareIssue(), nil), &mock.UserService{}),
				"issue", "view", "TEST-1", "--field", field)

			assert.Equal(t, 0, res.code)
			assert.Empty(t, res.stdout)
			assert.Empty(t, res.stderr)
		})
	}

	t.Run("text prints a string custom field as JSON, quoted, as --field-json takes it", func(t *testing.T) {
		t.Parallel()

		issue := bareIssue()
		issue.ReadCustomFields = map[string]jira4claude.CustomFieldValue{
			"customfield_10030": {Name: "Severity", Value: json.RawMessage(`"High"`)},
		}

		res := runCLI(t, testDeps(issueGetter(issue, nil), &mock.UserService{}),
			"issue", "view", "TEST-1", "--field", "customfield_10030")

		assert.Equal(t, 0, res.code)
		assert.Equal(t, "\"High\"\n", res.stdout)
	})

	t.Run("text prints empty lists as JSON", func(t *testing.T) {
		t.Parallel()

		res := runCLI(t, testDeps(issueGetter(bareIssue(), nil), &mock.UserService{}),
			"issue", "view", "TEST-1", "--field", "comments")

		assert.Equal(t, 0, res.code)
		assert.Equal(t, "[]\n", res.stdout)
	})

	t.Run("a multi-line description ends in exactly one newline", func(t *testing.T) {
		t.Parallel()

		issue := bareIssue()
		issue.Description = &jira4claude.ADFNode{Type: "doc", Version: 1, Content: []jira4claude.ADFNode{
			{Type: "paragraph", Content: []jira4claude.ADFNode{{Type: "text", Text: "One."}}},
			{Type: "codeBlock", Content: []jira4claude.ADFNode{{Type: "text", Text: "x := 1\n"}}},
		}}

		res := runCLI(t, testDeps(issueGetter(issue, nil), &mock.UserService{}),
			"issue", "view", "TEST-1", "--field", "description")

		assert.Equal(t, 0, res.code)
		// The converter keeps the code's trailing newline inside the fence;
		// the value then ends in the fence and one newline.
		assert.Equal(t, "One.\n\n```\nx := 1\n\n```\n", res.stdout)
	})

	t.Run("JSON is an object of the key and the field", func(t *testing.T) {
		t.Parallel()

		res := runCLI(t, testDeps(issueGetter(viewIssue("TEST-1"), nil), &mock.UserService{}),
			"--json", "issue", "view", "TEST-1", "--field", "description")

		assert.Equal(t, 0, res.code)
		assertJSONInOrder(t, "{\n  \"key\": \"TEST-1\",\n  \"description\": \"Steps to reproduce.\"\n}\n", res.stdout)
		assert.Empty(t, res.stderr)
	})

	t.Run("JSON keeps empty values and an absent custom field as null", func(t *testing.T) {
		t.Parallel()

		res := runCLI(t, testDeps(issueGetter(bareIssue(), nil), &mock.UserService{}),
			"--json", "issue", "view", "TEST-1", "--field", "assignee,labels,comments,customfield_10016")

		assert.Equal(t, 0, res.code)
		assertJSONInOrder(t, `{
  "key": "TEST-1",
  "assignee": "",
  "labels": [],
  "comments": [],
  "customfield_10016": null
}
`, res.stdout)
	})

	t.Run("JSON custom field is the raw value", func(t *testing.T) {
		t.Parallel()

		res := runCLI(t, testDeps(issueGetter(viewIssue("TEST-1"), nil), &mock.UserService{}),
			"--json", "issue", "view", "TEST-1", "--field", "customfield_10020")

		assert.Equal(t, 0, res.code)
		var got map[string]any
		require.NoError(t, json.Unmarshal([]byte(res.stdout), &got))
		assert.Equal(t, map[string]any{"key": "TEST-1", "customfield_10020": map[string]any{"id": "1", "name": "Core"}}, got)
	})

	t.Run("conversion warnings still go to stderr", func(t *testing.T) {
		t.Parallel()

		res := runCLI(t, testDeps(issueGetter(degradedIssue(), nil), &mock.UserService{}),
			"issue", "view", "TEST-1", "--field", "description")

		assert.Equal(t, 0, res.code)
		assert.Equal(t, "```go\nx := 1\n```\n", res.stdout)
		assert.True(t, strings.HasPrefix(res.stderr, "[warn] degraded codeBlock"), "stderr: %q", res.stderr)
	})

	t.Run("conversion warnings are JSON lines on stderr under --json", func(t *testing.T) {
		t.Parallel()

		res := runCLI(t, testDeps(issueGetter(degradedIssue(), nil), &mock.UserService{}),
			"--json", "issue", "view", "TEST-1", "--field", "description")

		assert.Equal(t, 0, res.code)
		lines := stderrLines(t, res.stderr)
		require.Len(t, lines, 1)
		assert.Equal(t, "warning", lines[0]["level"])
		assert.Equal(t, "codeBlock", lines[0]["node"])
	})
}

func TestRun_IssueViewSeveralFields(t *testing.T) {
	t.Parallel()

	t.Run("text prints a header line before each value", func(t *testing.T) {
		t.Parallel()

		res := runCLI(t, testDeps(issueGetter(viewIssue("TEST-1"), nil), &mock.UserService{}),
			"issue", "view", "TEST-1", "--field", "summary", "--field", "labels", "--field", "assignee,description")

		assert.Equal(t, 0, res.code)
		assert.Equal(t, "==> TEST-1 summary <==\nFix the login\n"+
			"==> TEST-1 labels <==\nbackend\nurgent\n"+
			"==> TEST-1 assignee <==\nJane Doe\n"+
			"==> TEST-1 description <==\nSteps to reproduce.\n", res.stdout)
		assert.Empty(t, res.stderr)
	})

	t.Run("text prints only the header for an empty value", func(t *testing.T) {
		t.Parallel()

		res := runCLI(t, testDeps(issueGetter(bareIssue(), nil), &mock.UserService{}),
			"issue", "view", "TEST-1", "--field", "assignee,status")

		assert.Equal(t, 0, res.code)
		assert.Equal(t, "==> TEST-1 assignee <==\n==> TEST-1 status <==\nTo Do\n", res.stdout)
	})

	t.Run("a value is never escaped, even a line that reads as a header", func(t *testing.T) {
		t.Parallel()

		// The header format is for reading and simple splitting; --json is the
		// form that no value can break.
		issue := bareIssue()
		issue.Description = &jira4claude.ADFNode{Type: "doc", Version: 1, Content: []jira4claude.ADFNode{
			{Type: "codeBlock", Content: []jira4claude.ADFNode{{Type: "text", Text: "==> TEST-1 status <=="}}},
		}}

		res := runCLI(t, testDeps(issueGetter(issue, nil), &mock.UserService{}),
			"issue", "view", "TEST-1", "--field", "description,status")

		assert.Equal(t, 0, res.code)
		assert.Equal(t, "==> TEST-1 description <==\n```\n==> TEST-1 status <==\n```\n==> TEST-1 status <==\nTo Do\n", res.stdout)
	})

	t.Run("a repeated field is printed once", func(t *testing.T) {
		t.Parallel()

		res := runCLI(t, testDeps(issueGetter(viewIssue("TEST-1"), nil), &mock.UserService{}),
			"issue", "view", "TEST-1", "--field", "status", "--field", "Status")

		assert.Equal(t, 0, res.code)
		assert.Equal(t, "In Progress\n", res.stdout)
	})

	t.Run("JSON is one object with the key first and fields in the order asked", func(t *testing.T) {
		t.Parallel()

		res := runCLI(t, testDeps(issueGetter(viewIssue("TEST-1"), nil), &mock.UserService{}),
			"--json", "issue", "view", "TEST-1", "--field", "status,labels,key")

		assert.Equal(t, 0, res.code)
		assertJSONInOrder(t, `{
  "key": "TEST-1",
  "status": "In Progress",
  "labels": [
    "backend",
    "urgent"
  ]
}
`, res.stdout)
	})
}

func TestRun_IssueViewSeveralKeys(t *testing.T) {
	t.Parallel()

	t.Run("text without fields prints the pages in the order asked, a blank line apart", func(t *testing.T) {
		t.Parallel()

		svc := issueStore(twoIssues(), nil)
		one := runCLI(t, testDeps(svc, &mock.UserService{}), "issue", "view", "TEST-1")
		two := runCLI(t, testDeps(svc, &mock.UserService{}), "issue", "view", "TEST-2")

		res := runCLI(t, testDeps(svc, &mock.UserService{}), "issue", "view", "TEST-2", "TEST-1")

		assert.Equal(t, 0, res.code)
		assert.Equal(t, two.stdout+"\n"+one.stdout, res.stdout)
		assert.Empty(t, res.stderr)
	})

	t.Run("JSON without fields is an array of full views in the order asked", func(t *testing.T) {
		t.Parallel()

		res := runCLI(t, testDeps(issueStore(twoIssues(), nil), &mock.UserService{}), "--json", "issue", "view", "TEST-2", "TEST-1")

		assert.Equal(t, 0, res.code)
		var got []map[string]any
		require.NoError(t, json.Unmarshal([]byte(res.stdout), &got))
		require.Len(t, got, 2)
		assert.Equal(t, "TEST-2", got[0]["key"])
		assert.Equal(t, "Second", got[0]["summary"])
		assert.Equal(t, "TEST-1", got[1]["key"])
		assert.Equal(t, "Steps to reproduce.", got[1]["description"])
		assert.Empty(t, res.stderr)
	})

	t.Run("text with one field prints a header per key", func(t *testing.T) {
		t.Parallel()

		res := runCLI(t, testDeps(issueStore(twoIssues(), nil), &mock.UserService{}),
			"issue", "view", "TEST-2", "TEST-1", "--field", "status")

		assert.Equal(t, 0, res.code)
		assert.Equal(t, "==> TEST-2 status <==\nDone\n==> TEST-1 status <==\nIn Progress\n", res.stdout)
	})

	t.Run("text with several fields prints a header per key and field", func(t *testing.T) {
		t.Parallel()

		res := runCLI(t, testDeps(issueStore(twoIssues(), nil), &mock.UserService{}),
			"issue", "view", "TEST-1", "TEST-2", "--field", "status,summary")

		assert.Equal(t, 0, res.code)
		assert.Equal(t, "==> TEST-1 status <==\nIn Progress\n==> TEST-1 summary <==\nFix the login\n"+
			"==> TEST-2 status <==\nDone\n==> TEST-2 summary <==\nSecond\n", res.stdout)
	})

	t.Run("JSON with fields is an array of objects that carry their key", func(t *testing.T) {
		t.Parallel()

		res := runCLI(t, testDeps(issueStore(twoIssues(), nil), &mock.UserService{}),
			"--json", "issue", "view", "TEST-2", "TEST-1", "--field", "status")

		assert.Equal(t, 0, res.code)
		assertJSONInOrder(t, `[
  {
    "key": "TEST-2",
    "status": "Done"
  },
  {
    "key": "TEST-1",
    "status": "In Progress"
  }
]
`, res.stdout)
	})
}

func TestRun_IssueViewUnknownField(t *testing.T) {
	t.Parallel()

	for _, field := range []string{"descripton", "customfield_", "customfield_12a", "Story Points", ""} {
		t.Run("text: "+field, func(t *testing.T) {
			t.Parallel()

			// Zero-value mocks panic on any call: the error comes before any request.
			res := runCLI(t, testDeps(&mock.IssueService{}, &mock.UserService{}),
				"issue", "view", "TEST-1", "TEST-2", "--field", "summary", "--field", field)

			assert.Equal(t, 1, res.code)
			assert.Empty(t, res.stdout)
			assert.Contains(t, res.stderr, `[error] unknown field "`+field+`"`)
			assert.Contains(t, res.stderr, "key, project, summary, description, status, type, priority, assignee, reporter, labels, relatedIssues, comments, attachments, created, updated, url")
			assert.Contains(t, res.stderr, "customfield_")
		})
	}

	t.Run("JSON: one error line", func(t *testing.T) {
		t.Parallel()

		res := runCLI(t, testDeps(&mock.IssueService{}, &mock.UserService{}),
			"--json", "issue", "view", "TEST-1", "--field", "descripton")

		assert.Equal(t, 1, res.code)
		assert.Empty(t, res.stdout)
		lines := stderrLines(t, res.stderr)
		require.Len(t, lines, 1)
		assert.Equal(t, jira4claude.EValidation, lines[0]["code"])
		assert.Contains(t, lines[0]["message"], `unknown field "descripton"`)
	})
}

func TestRun_IssueViewPartialFailure(t *testing.T) {
	t.Parallel()

	forbidden := &jira4claude.Error{Code: jira4claude.EForbidden, Message: "no permission"}

	t.Run("text prints the others and reports the failure on stderr", func(t *testing.T) {
		t.Parallel()

		res := runCLI(t, testDeps(issueStore(twoIssues(), nil), &mock.UserService{}),
			"issue", "view", "TEST-1", "TEST-9", "TEST-2", "--field", "status")

		assert.Equal(t, 4, res.code)
		assert.Equal(t, "==> TEST-1 status <==\nIn Progress\n==> TEST-2 status <==\nDone\n", res.stdout)
		assert.Equal(t, "[error] TEST-9: issue not found\n[error] could not read 1 of 3 issues: TEST-9\n", res.stderr)
	})

	t.Run("text without fields prints the other pages", func(t *testing.T) {
		t.Parallel()

		res := runCLI(t, testDeps(issueStore(twoIssues(), nil), &mock.UserService{}),
			"issue", "view", "TEST-9", "TEST-1")

		assert.Equal(t, 4, res.code)
		assert.Equal(t, viewIssuePage, res.stdout)
		assert.Contains(t, res.stderr, "[error] TEST-9: issue not found\n")
	})

	t.Run("JSON prints the others as an array and one error line per failure", func(t *testing.T) {
		t.Parallel()

		res := runCLI(t, testDeps(issueStore(twoIssues(), nil), &mock.UserService{}),
			"--json", "issue", "view", "TEST-1", "TEST-9", "TEST-2")

		assert.Equal(t, 4, res.code)
		var got []map[string]any
		require.NoError(t, json.Unmarshal([]byte(res.stdout), &got))
		require.Len(t, got, 2)
		assert.Equal(t, "TEST-1", got[0]["key"])
		assert.Equal(t, "TEST-2", got[1]["key"])
		lines := stderrLines(t, res.stderr)
		require.Len(t, lines, 2)
		assert.Equal(t, map[string]any{"level": "error", "error": true, "code": jira4claude.ENotFound, "message": "TEST-9: issue not found"}, lines[0])
		assert.Equal(t, map[string]any{"level": "error", "error": true, "code": jira4claude.ENotFound, "message": "could not read 1 of 3 issues: TEST-9"}, lines[1])
	})

	t.Run("differing failures exit with the code of the first in the order asked", func(t *testing.T) {
		t.Parallel()

		svc := issueStore(twoIssues(), map[string]error{"TEST-8": forbidden})

		res := runCLI(t, testDeps(svc, &mock.UserService{}), "issue", "view", "TEST-1", "TEST-8", "TEST-9", "--field", "key")
		assert.Equal(t, 3, res.code)
		assert.Contains(t, res.stderr, "[error] TEST-8: no permission\n[error] TEST-9: issue not found\n")
		assert.Contains(t, res.stderr, "could not read 2 of 3 issues: TEST-8, TEST-9")

		res = runCLI(t, testDeps(svc, &mock.UserService{}), "issue", "view", "TEST-1", "TEST-9", "TEST-8", "--field", "key")
		assert.Equal(t, 4, res.code)
	})

	t.Run("JSON prints an empty array when every key fails", func(t *testing.T) {
		t.Parallel()

		res := runCLI(t, testDeps(issueStore(nil, nil), &mock.UserService{}),
			"--json", "issue", "view", "TEST-8", "TEST-9", "--field", "status")

		assert.Equal(t, 4, res.code)
		assert.Equal(t, "[]\n", res.stdout)
		assert.Len(t, stderrLines(t, res.stderr), 3)
	})

	t.Run("text prints nothing when every key fails", func(t *testing.T) {
		t.Parallel()

		res := runCLI(t, testDeps(issueStore(nil, nil), &mock.UserService{}), "issue", "view", "TEST-8", "TEST-9")

		assert.Equal(t, 4, res.code)
		assert.Empty(t, res.stdout)
	})

	t.Run("one key that fails is reported as before", func(t *testing.T) {
		t.Parallel()

		res := runCLI(t, testDeps(issueStore(nil, nil), &mock.UserService{}), "issue", "view", "TEST-9", "--field", "status")

		assert.Equal(t, 4, res.code)
		assert.Empty(t, res.stdout)
		assert.Equal(t, "[error] issue not found\n", res.stderr)
	})
}

func attachedIssue() *jira4claude.Issue {
	issue := bareIssue()
	issue.Attachments = []*jira4claude.Attachment{
		{ID: "10100", Filename: "shot one.png", MimeType: "image/png", Size: 23145},
		{ID: "10101", Filename: "log.txt", MimeType: "text/plain", Size: 12},
	}
	return issue
}

func TestRun_IssueViewAttachmentsField(t *testing.T) {
	t.Parallel()

	t.Run("text prints the lines attachment list prints", func(t *testing.T) {
		t.Parallel()

		res := runCLI(t, testDeps(issueGetter(attachedIssue(), nil), &mock.UserService{}),
			"issue", "view", "TEST-1", "--field", "attachments")

		assert.Equal(t, 0, res.code)
		assert.Equal(t, "10100\t23145\timage/png\tshot one.png\n10101\t12\ttext/plain\tlog.txt\n", res.stdout)
		assert.Empty(t, res.stderr)
	})

	t.Run("JSON is the array attachment list prints", func(t *testing.T) {
		t.Parallel()

		res := runCLI(t, testDeps(issueGetter(attachedIssue(), nil), &mock.UserService{}),
			"--json", "issue", "view", "TEST-1", "--field", "attachments")

		assert.Equal(t, 0, res.code)
		assertJSONInOrder(t, `{"key":"TEST-1","attachments":[
			{"id":"10100","filename":"shot one.png","mimeType":"image/png","size":23145},
			{"id":"10101","filename":"log.txt","mimeType":"text/plain","size":12}]}`, res.stdout)
	})

	t.Run("none prints nothing in text", func(t *testing.T) {
		t.Parallel()

		res := runCLI(t, testDeps(issueGetter(bareIssue(), nil), &mock.UserService{}),
			"issue", "view", "TEST-1", "--field", "attachments")

		assert.Equal(t, 0, res.code)
		assert.Empty(t, res.stdout)
		assert.Empty(t, res.stderr)
	})

	t.Run("none is [] in JSON", func(t *testing.T) {
		t.Parallel()

		res := runCLI(t, testDeps(issueGetter(bareIssue(), nil), &mock.UserService{}),
			"--json", "issue", "view", "TEST-1", "--field", "attachments")

		assert.Equal(t, 0, res.code)
		assertJSONInOrder(t, `{"key":"TEST-1","attachments":[]}`, res.stdout)
	})
}

// degradedComment is a comment whose body reads back degraded, as the
// description of degradedIssue does.
func degradedComment() *jira4claude.Comment {
	return &jira4claude.Comment{ID: "1", Body: degradedIssue().Description}
}

func TestRun_IssueViewWarningsForSelectedFields(t *testing.T) {
	t.Parallel()

	degraded := func() *jira4claude.Issue {
		issue := degradedIssue()
		issue.Comments = []*jira4claude.Comment{degradedComment()}
		return issue
	}

	for _, mode := range []string{"text", "json"} {
		args := func(rest ...string) []string {
			if mode == "json" {
				return append([]string{"--json"}, rest...)
			}
			return rest
		}

		t.Run(mode+": a field that is not converted warns nothing", func(t *testing.T) {
			t.Parallel()

			res := runCLI(t, testDeps(issueGetter(degraded(), nil), &mock.UserService{}),
				args("issue", "view", "TEST-1", "--field", "status")...)

			assert.Equal(t, 0, res.code)
			assert.NotEmpty(t, res.stdout)
			assert.Empty(t, res.stderr)
		})

		t.Run(mode+": description warns for the description only", func(t *testing.T) {
			t.Parallel()

			res := runCLI(t, testDeps(issueGetter(degraded(), nil), &mock.UserService{}),
				args("issue", "view", "TEST-1", "--field", "description")...)

			assert.Equal(t, 0, res.code)
			assert.Equal(t, 1, strings.Count(res.stderr, "degraded codeBlock"), "stderr: %q", res.stderr)
		})

		t.Run(mode+": comments warn for the comments only", func(t *testing.T) {
			t.Parallel()

			res := runCLI(t, testDeps(issueGetter(degraded(), nil), &mock.UserService{}),
				args("issue", "view", "TEST-1", "--field", "comments")...)

			assert.Equal(t, 0, res.code)
			assert.Equal(t, 1, strings.Count(res.stderr, "degraded codeBlock"), "stderr: %q", res.stderr)
		})

		t.Run(mode+": no --field warns for both, as before", func(t *testing.T) {
			t.Parallel()

			res := runCLI(t, testDeps(issueGetter(degraded(), nil), &mock.UserService{}),
				args("issue", "view", "TEST-1")...)

			assert.Equal(t, 0, res.code)
			assert.Equal(t, 2, strings.Count(res.stderr, "degraded codeBlock"), "stderr: %q", res.stderr)
		})
	}
}

func TestRun_IssueViewWarningsNameTheIssue(t *testing.T) {
	t.Parallel()

	// Keys asked in lower case: Jira returns them in upper case, and that
	// spelling is used in warnings, headers and objects alike.
	store := func() *mock.IssueService {
		second := degradedIssue()
		second.Key = "TEST-2"
		return issueStore(map[string]*jira4claude.Issue{"test-1": degradedIssue(), "test-2": second}, nil)
	}

	t.Run("text: several keys prefix the warning with the key", func(t *testing.T) {
		t.Parallel()

		res := runCLI(t, testDeps(store(), &mock.UserService{}), "issue", "view", "test-1", "test-2", "--field", "description")

		assert.Equal(t, 0, res.code)
		assert.Equal(t, "==> TEST-1 description <==\n```go\nx := 1\n```\n==> TEST-2 description <==\n```go\nx := 1\n```\n", res.stdout)
		lines := strings.Split(strings.TrimSuffix(res.stderr, "\n"), "\n")
		require.Len(t, lines, 2)
		assert.True(t, strings.HasPrefix(lines[0], "[warn] TEST-1: degraded codeBlock"), "stderr: %q", res.stderr)
		assert.True(t, strings.HasPrefix(lines[1], "[warn] TEST-2: degraded codeBlock"), "stderr: %q", res.stderr)
	})

	t.Run("text: one key keeps the warning as it was", func(t *testing.T) {
		t.Parallel()

		res := runCLI(t, testDeps(store(), &mock.UserService{}), "issue", "view", "test-1")

		assert.Equal(t, 0, res.code)
		assert.True(t, strings.HasPrefix(res.stderr, "[warn] degraded codeBlock"), "stderr: %q", res.stderr)
	})

	t.Run("JSON: every view warning carries the key, one key or several", func(t *testing.T) {
		t.Parallel()

		res := runCLI(t, testDeps(store(), &mock.UserService{}), "--json", "issue", "view", "test-1", "test-2")
		assert.Equal(t, 0, res.code)
		lines := stderrLines(t, res.stderr)
		require.Len(t, lines, 2)
		assert.Equal(t, "TEST-1", lines[0]["key"])
		assert.Equal(t, "TEST-2", lines[1]["key"])
		assert.Equal(t, "warning", lines[0]["level"])
		assert.Equal(t, "codeBlock", lines[0]["node"])
		assert.Contains(t, lines[0]["message"], "degraded codeBlock")

		res = runCLI(t, testDeps(store(), &mock.UserService{}), "--json", "issue", "view", "test-1", "--field", "description")
		assert.Equal(t, 0, res.code)
		lines = stderrLines(t, res.stderr)
		require.Len(t, lines, 1)
		assert.Equal(t, "TEST-1", lines[0]["key"])
	})
}

// tokenTailIssue is guardedIssue with its media moved to the end, so the
// description read ends in a token line.
func tokenTailIssue() *jira4claude.Issue {
	issue := guardedIssue()
	issue.Description = &jira4claude.ADFNode{Type: "doc", Version: 1, Content: []jira4claude.ADFNode{
		{Type: "paragraph", Content: []jira4claude.ADFNode{
			{Type: "text", Text: "Ping "},
			guardedMention(),
			{Type: "text", Text: " about it."},
		}},
		guardedMediaSingle(),
	}}
	return issue
}

func TestRun_IssueViewFieldRoundTrip(t *testing.T) {
	t.Parallel()

	t.Run("the description bytes re-attach the mention and media on update", func(t *testing.T) {
		t.Parallel()

		view := runCLI(t, testDeps(newGuardedService(tokenTailIssue()), &mock.UserService{}),
			"issue", "view", "TEST-1", "--field", "description")
		require.Equal(t, 0, view.code, "stderr: %s", view.stderr)
		require.Empty(t, view.stderr)
		lines := tokenLines(view.stdout)
		require.Len(t, lines, 2, "description: %q", view.stdout)
		assert.True(t, strings.HasSuffix(view.stdout, lines[1]+"\n"), "the token line ends the value: %q", view.stdout)
		assert.False(t, strings.HasSuffix(view.stdout, "\n\n"), "exactly one newline: %q", view.stdout)

		svc := newGuardedService(tokenTailIssue())
		res := runCLI(t, testDeps(svc, &mock.UserService{}), "issue", "update", "TEST-1", "--description="+view.stdout)

		require.Equal(t, 0, res.code, "stderr: %s", res.stderr)
		assert.Empty(t, res.stderr)
		doc := svc.Sent().Description.Document()
		media := findNodes(doc, "mediaSingle")
		require.Len(t, media, 1)
		assert.Equal(t, marshal(t, guardedMediaSingle()), marshal(t, media[0]))
		mentions := findNodes(doc, "mention")
		require.Len(t, mentions, 1)
		assert.Equal(t, marshal(t, guardedMention()), marshal(t, mentions[0]))
		assert.NotContains(t, marshal(t, doc), "{{")
	})

	t.Run("updated is accepted by --if-updated", func(t *testing.T) {
		t.Parallel()

		view := runCLI(t, testDeps(newGuardedService(guardedIssue()), &mock.UserService{}),
			"issue", "view", "TEST-1", "--field", "updated")
		require.Equal(t, 0, view.code)

		svc := newGuardedService(guardedIssue())
		// $(...) drops the trailing newline.
		res := runCLI(t, testDeps(svc, &mock.UserService{}),
			"issue", "update", "TEST-1", "--summary=New", "--if-updated="+strings.TrimSuffix(view.stdout, "\n"))

		require.Equal(t, 0, res.code, "stderr: %s", res.stderr)
		assert.Equal(t, []string{"get TEST-1", "update TEST-1"}, svc.Calls())
	})
}
