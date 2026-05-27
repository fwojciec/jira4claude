package http_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sort"
	"testing"

	"github.com/fwojciec/jira4claude"
	jirahttp "github.com/fwojciec/jira4claude/http"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createMetaFixture wraps a fields-map body in the legacy createmeta envelope.
func createMetaFixture(fields string) string {
	return fmt.Sprintf(`{"projects":[{"issuetypes":[{"fields":%s}]}]}`, fields)
}

// editMetaFixture wraps a fields-map body in the editmeta envelope.
func editMetaFixture(fields string) string {
	return fmt.Sprintf(`{"fields":%s}`, fields)
}

// findField returns the field with the given id (test helper).
func findField(t *testing.T, fields []*jira4claude.IssueField, id string) *jira4claude.IssueField {
	t.Helper()
	for _, f := range fields {
		if f.ID == id {
			return f
		}
	}
	t.Fatalf("field %q not found", id)
	return nil
}

func TestIssueService_GetCreateFields(t *testing.T) {
	t.Parallel()

	t.Run("calls createmeta with escaped project and issuetype query params", func(t *testing.T) {
		t.Parallel()

		var receivedURL *url.URL
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedURL = r.URL
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(createMetaFixture(`{}`)))
		}))
		defer server.Close()

		client := newTestClient(t, server.URL, "user", "pass")
		svc := jirahttp.NewIssueService(client)

		_, err := svc.GetCreateFields(context.Background(), "INT", "Bug Report")
		require.NoError(t, err)
		require.NotNil(t, receivedURL)

		assert.Equal(t, "/rest/api/3/issue/createmeta", receivedURL.Path)
		q := receivedURL.Query()
		assert.Equal(t, "INT", q.Get("projectKeys"))
		assert.Equal(t, "Bug Report", q.Get("issuetypeNames"))
		assert.Equal(t, "projects.issuetypes.fields", q.Get("expand"))
	})

	t.Run("parses legacy createmeta nested shape", func(t *testing.T) {
		t.Parallel()

		body := createMetaFixture(`{
			"summary": {"name":"Summary","required":true,"schema":{"type":"string"}},
			"customfield_10010": {"name":"Story Points","required":false,"schema":{"type":"number","custom":"com.atlassian.jira.plugin.system.customfieldtypes:float"}}
		}`)
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(body))
		}))
		defer server.Close()

		client := newTestClient(t, server.URL, "user", "pass")
		svc := jirahttp.NewIssueService(client)

		fields, err := svc.GetCreateFields(context.Background(), "INT", "Task")
		require.NoError(t, err)
		require.Len(t, fields, 2)

		// Order is map-iteration-dependent. Sort for stable assertions.
		sort.Slice(fields, func(i, j int) bool { return fields[i].ID < fields[j].ID })
		assert.Equal(t, "customfield_10010", fields[0].ID)
		assert.Equal(t, "summary", fields[1].ID)
	})

	t.Run("returns ENotFound and quotes project key on empty projects", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"projects":[]}`))
		}))
		defer server.Close()

		client := newTestClient(t, server.URL, "user", "pass")
		svc := jirahttp.NewIssueService(client)

		_, err := svc.GetCreateFields(context.Background(), "NOPE", "Task")
		require.Error(t, err)
		assert.Equal(t, jira4claude.ENotFound, jira4claude.ErrorCode(err))
		assert.Contains(t, err.Error(), `"NOPE"`)
	})

	t.Run("returns ENotFound and quotes both keys on empty issuetypes", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"projects":[{"issuetypes":[]}]}`))
		}))
		defer server.Close()

		client := newTestClient(t, server.URL, "user", "pass")
		svc := jirahttp.NewIssueService(client)

		_, err := svc.GetCreateFields(context.Background(), "INT", "Mystery")
		require.Error(t, err)
		assert.Equal(t, jira4claude.ENotFound, jira4claude.ErrorCode(err))
		assert.Contains(t, err.Error(), `"Mystery"`)
		assert.Contains(t, err.Error(), `"INT"`)
	})

	t.Run("required flag maps through", func(t *testing.T) {
		t.Parallel()

		body := createMetaFixture(`{
			"a": {"name":"A","required":true,"schema":{"type":"string"}},
			"b": {"name":"B","required":false,"schema":{"type":"string"}}
		}`)
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(body))
		}))
		defer server.Close()

		client := newTestClient(t, server.URL, "user", "pass")
		svc := jirahttp.NewIssueService(client)

		fields, err := svc.GetCreateFields(context.Background(), "INT", "Task")
		require.NoError(t, err)

		assert.True(t, findField(t, fields, "a").Required)
		assert.False(t, findField(t, fields, "b").Required)
	})

	t.Run("field id is taken from map key even when entry body has no id", func(t *testing.T) {
		t.Parallel()

		// Entry body deliberately omits any id-like attribute.
		body := createMetaFixture(`{
			"customfield_99999": {"name":"Mystery","required":false,"schema":{"type":"string"}}
		}`)
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(body))
		}))
		defer server.Close()

		client := newTestClient(t, server.URL, "user", "pass")
		svc := jirahttp.NewIssueService(client)

		fields, err := svc.GetCreateFields(context.Background(), "INT", "Task")
		require.NoError(t, err)
		require.Len(t, fields, 1)
		assert.Equal(t, "customfield_99999", fields[0].ID)
	})

	t.Run("schema types and example generation", func(t *testing.T) {
		t.Parallel()

		// One representative entry per row of the table.
		body := createMetaFixture(`{
			"summary": {"name":"Summary","required":true,"schema":{"type":"string","custom":"com.atlassian.jira.plugin.system.customfieldtypes:textfield"}},
			"customfield_textarea": {"name":"Description Custom","required":false,"schema":{"type":"string","custom":"com.atlassian.jira.plugin.system.customfieldtypes:textarea"}},
			"customfield_number": {"name":"Story Points","required":false,"schema":{"type":"number","custom":"com.atlassian.jira.plugin.system.customfieldtypes:float"}},
			"duedate": {"name":"Due Date","required":false,"schema":{"type":"date"}},
			"customfield_datetime": {"name":"Custom DT","required":false,"schema":{"type":"datetime","custom":"com.atlassian.jira.plugin.system.customfieldtypes:datetime"}},
			"customfield_option": {"name":"Urgency","required":true,"schema":{"type":"option","custom":"com.atlassian.jira.plugin.system.customfieldtypes:select"},"allowedValues":[{"id":"1","value":"High"},{"id":"2","value":"Low"}]},
			"customfield_array_option": {"name":"Areas","required":true,"schema":{"type":"array","items":"option","custom":"com.atlassian.jira.plugin.system.customfieldtypes:multiselect"},"allowedValues":[{"id":"10","value":"Integrations"},{"id":"11","value":"API"}]},
			"labels": {"name":"Labels","required":false,"schema":{"type":"array","items":"string"}},
			"assignee": {"name":"Assignee","required":false,"schema":{"type":"user"}},
			"priority": {"name":"Priority","required":false,"schema":{"type":"priority"}},
			"customfield_unknown": {"name":"Mystery","required":false,"schema":{"type":"sprintnonsense"}}
		}`)
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(body))
		}))
		defer server.Close()

		client := newTestClient(t, server.URL, "user", "pass")
		svc := jirahttp.NewIssueService(client)

		fields, err := svc.GetCreateFields(context.Background(), "INT", "Task")
		require.NoError(t, err)

		// string + textfield -> "..."
		assert.JSONEq(t, `"..."`, string(findField(t, fields, "summary").Example))
		// string + textarea -> ADF
		assert.JSONEq(t,
			`{"type":"doc","version":1,"content":[{"type":"paragraph","content":[{"type":"text","text":"..."}]}]}`,
			string(findField(t, fields, "customfield_textarea").Example),
		)
		// number -> 0
		assert.JSONEq(t, `0`, string(findField(t, fields, "customfield_number").Example))
		// date
		assert.JSONEq(t, `"2026-05-27"`, string(findField(t, fields, "duedate").Example))
		// datetime
		assert.JSONEq(t, `"2026-05-27T12:00:00.000+0000"`, string(findField(t, fields, "customfield_datetime").Example))
		// option with allowedValues -> first allowed value
		assert.JSONEq(t, `{"value":"High"}`, string(findField(t, fields, "customfield_option").Example))
		// array<option> with allowedValues -> first allowed value
		assert.JSONEq(t, `[{"value":"Integrations"}]`, string(findField(t, fields, "customfield_array_option").Example))
		// array<string>
		assert.JSONEq(t, `["..."]`, string(findField(t, fields, "labels").Example))
		// user
		assert.JSONEq(t, `{"accountId":"..."}`, string(findField(t, fields, "assignee").Example))
		// priority
		assert.JSONEq(t, `{"name":"High"}`, string(findField(t, fields, "priority").Example))
		// unknown -> nil (omitted on JSON projection)
		assert.Nil(t, findField(t, fields, "customfield_unknown").Example)
	})

	t.Run("option without allowedValues falls back to placeholder example", func(t *testing.T) {
		t.Parallel()

		body := createMetaFixture(`{
			"customfield_opt": {"name":"Opt","required":false,"schema":{"type":"option"}}
		}`)
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(body))
		}))
		defer server.Close()

		client := newTestClient(t, server.URL, "user", "pass")
		svc := jirahttp.NewIssueService(client)

		fields, err := svc.GetCreateFields(context.Background(), "INT", "Task")
		require.NoError(t, err)
		assert.JSONEq(t, `{"value":"..."}`, string(findField(t, fields, "customfield_opt").Example))
	})

	t.Run("array<option> without allowedValues falls back to placeholder example", func(t *testing.T) {
		t.Parallel()

		body := createMetaFixture(`{
			"customfield_multi": {"name":"Multi","required":false,"schema":{"type":"array","items":"option"}}
		}`)
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(body))
		}))
		defer server.Close()

		client := newTestClient(t, server.URL, "user", "pass")
		svc := jirahttp.NewIssueService(client)

		fields, err := svc.GetCreateFields(context.Background(), "INT", "Task")
		require.NoError(t, err)
		assert.JSONEq(t, `[{"value":"..."}]`, string(findField(t, fields, "customfield_multi").Example))
	})

	t.Run("allowedValues shape variants", func(t *testing.T) {
		t.Parallel()

		body := createMetaFixture(`{
			"f_primitive": {"name":"Primitive","required":false,"schema":{"type":"option"},"allowedValues":["Foo","Bar"]},
			"f_value_only": {"name":"ValueOnly","required":false,"schema":{"type":"option"},"allowedValues":[{"value":"Foo"}]},
			"f_name_only": {"name":"NameOnly","required":false,"schema":{"type":"priority"},"allowedValues":[{"name":"High"}]},
			"f_id_value": {"name":"IDValue","required":false,"schema":{"type":"option"},"allowedValues":[{"id":"10500","value":"High"}]},
			"f_id_name": {"name":"IDName","required":false,"schema":{"type":"option"},"allowedValues":[{"id":"10000","name":"frontend"}]},
			"f_missing": {"name":"Missing","required":false,"schema":{"type":"option"}},
			"f_null": {"name":"Null","required":false,"schema":{"type":"option"},"allowedValues":null},
			"f_cascading": {"name":"Cascading","required":false,"schema":{"type":"option"},"allowedValues":[{"id":"1","value":"Parent","children":[{"id":"2","value":"Child"}]}]},
			"f_null_element": {"name":"NullElement","required":false,"schema":{"type":"option"},"allowedValues":[null,{"id":"7","value":"Real"}]}
		}`)
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(body))
		}))
		defer server.Close()

		client := newTestClient(t, server.URL, "user", "pass")
		svc := jirahttp.NewIssueService(client)

		fields, err := svc.GetCreateFields(context.Background(), "INT", "Task")
		require.NoError(t, err)

		assert.Equal(t,
			[]jira4claude.FieldAllowedValue{{Value: "Foo"}, {Value: "Bar"}},
			findField(t, fields, "f_primitive").AllowedValues,
		)
		assert.Equal(t,
			[]jira4claude.FieldAllowedValue{{Value: "Foo"}},
			findField(t, fields, "f_value_only").AllowedValues,
		)
		assert.Equal(t,
			[]jira4claude.FieldAllowedValue{{Value: "High"}},
			findField(t, fields, "f_name_only").AllowedValues,
		)
		assert.Equal(t,
			[]jira4claude.FieldAllowedValue{{ID: "10500", Value: "High"}},
			findField(t, fields, "f_id_value").AllowedValues,
		)
		assert.Equal(t,
			[]jira4claude.FieldAllowedValue{{ID: "10000", Value: "frontend"}},
			findField(t, fields, "f_id_name").AllowedValues,
		)
		assert.Empty(t, findField(t, fields, "f_missing").AllowedValues)
		assert.Empty(t, findField(t, fields, "f_null").AllowedValues)

		// Cascading: keep the top-level value, ignore nested children.
		assert.Equal(t,
			[]jira4claude.FieldAllowedValue{{ID: "1", Value: "Parent"}},
			findField(t, fields, "f_cascading").AllowedValues,
		)

		// Null elements inside the array are skipped rather than producing empty entries.
		assert.Equal(t,
			[]jira4claude.FieldAllowedValue{{ID: "7", Value: "Real"}},
			findField(t, fields, "f_null_element").AllowedValues,
		)
	})

	t.Run("option example uses first allowed value", func(t *testing.T) {
		t.Parallel()

		body := createMetaFixture(`{
			"customfield_urg": {"name":"Urgency","required":true,"schema":{"type":"option"},"allowedValues":[{"id":"10500","value":"Critical"},{"id":"10501","value":"Low"}]}
		}`)
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(body))
		}))
		defer server.Close()

		client := newTestClient(t, server.URL, "user", "pass")
		svc := jirahttp.NewIssueService(client)

		fields, err := svc.GetCreateFields(context.Background(), "INT", "Task")
		require.NoError(t, err)

		f := findField(t, fields, "customfield_urg")
		require.NotNil(t, f.Example)

		var got map[string]string
		require.NoError(t, json.Unmarshal(f.Example, &got))
		assert.Equal(t, "Critical", got["value"])
	})
}

func TestIssueService_GetEditFields(t *testing.T) {
	t.Parallel()

	t.Run("calls editmeta with escaped issue key path", func(t *testing.T) {
		t.Parallel()

		var receivedPath string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedPath = r.URL.Path
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(editMetaFixture(`{}`)))
		}))
		defer server.Close()

		client := newTestClient(t, server.URL, "user", "pass")
		svc := jirahttp.NewIssueService(client)

		_, err := svc.GetEditFields(context.Background(), "INT-1118")
		require.NoError(t, err)
		assert.Equal(t, "/rest/api/3/issue/INT-1118/editmeta", receivedPath)
	})

	t.Run("parses editmeta top-level fields shape via the shared parser", func(t *testing.T) {
		t.Parallel()

		// Same kind of entry as createmeta — confirms the shared parser handles both.
		body := editMetaFixture(`{
			"summary": {"name":"Summary","required":true,"schema":{"type":"string"}},
			"customfield_10801": {"name":"Urgency","required":true,"schema":{"type":"option","custom":"com.atlassian.jira.plugin.system.customfieldtypes:select"},"allowedValues":[{"id":"10500","value":"High"}]}
		}`)
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(body))
		}))
		defer server.Close()

		client := newTestClient(t, server.URL, "user", "pass")
		svc := jirahttp.NewIssueService(client)

		fields, err := svc.GetEditFields(context.Background(), "INT-1118")
		require.NoError(t, err)
		require.Len(t, fields, 2)

		summary := findField(t, fields, "summary")
		assert.Equal(t, "Summary", summary.Name)
		assert.True(t, summary.Required)
		assert.Equal(t, "string", summary.SchemaType)
		assert.JSONEq(t, `"..."`, string(summary.Example))

		urgency := findField(t, fields, "customfield_10801")
		assert.Equal(t, "option", urgency.SchemaType)
		assert.Equal(t,
			[]jira4claude.FieldAllowedValue{{ID: "10500", Value: "High"}},
			urgency.AllowedValues,
		)
		assert.JSONEq(t, `{"value":"High"}`, string(urgency.Example))
	})

	t.Run("propagates upstream 404 as ENotFound", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"errorMessages":["Issue does not exist"]}`))
		}))
		defer server.Close()

		client := newTestClient(t, server.URL, "user", "pass")
		svc := jirahttp.NewIssueService(client)

		_, err := svc.GetEditFields(context.Background(), "NOPE-1")
		require.Error(t, err)
		assert.Equal(t, jira4claude.ENotFound, jira4claude.ErrorCode(err))
	})
}
