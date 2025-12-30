package http_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fwojciec/jira4claude"
	jirahttp "github.com/fwojciec/jira4claude/http"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIssueService_Create(t *testing.T) {
	t.Parallel()

	t.Run("creates issue and returns it with key", func(t *testing.T) {
		t.Parallel()

		var receivedRequest map[string]any
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost || r.URL.Path != "/rest/api/3/issue" {
				w.WriteHeader(http.StatusNotFound)
				return
			}

			// Capture the request body
			err := json.NewDecoder(r.Body).Decode(&receivedRequest)
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			// Return created issue response
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{
				"id": "10001",
				"key": "TEST-1",
				"self": "https://test.atlassian.net/rest/api/3/issue/10001"
			}`))
		}))
		defer server.Close()

		client := newTestClient(t, server.URL, "user@example.com", "api-token")
		svc := jirahttp.NewIssueService(client)

		issue := &jira4claude.Issue{
			Project:     "TEST",
			Summary:     "Test issue",
			Description: "This is a test description",
			Type:        "Task",
		}

		result, err := svc.Create(context.Background(), issue)

		require.NoError(t, err)
		assert.Equal(t, "TEST-1", result.Key)

		// Verify request structure
		fields := receivedRequest["fields"].(map[string]any)
		assert.Equal(t, map[string]any{"key": "TEST"}, fields["project"])
		assert.Equal(t, "Test issue", fields["summary"])
		assert.Equal(t, map[string]any{"name": "Task"}, fields["issuetype"])

		// Description should be in ADF format
		desc := fields["description"].(map[string]any)
		assert.Equal(t, "doc", desc["type"])
	})

	t.Run("creates issue without description", func(t *testing.T) {
		t.Parallel()

		var receivedRequest map[string]any
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewDecoder(r.Body).Decode(&receivedRequest)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"key": "TEST-2"}`))
		}))
		defer server.Close()

		client := newTestClient(t, server.URL, "user@example.com", "api-token")
		svc := jirahttp.NewIssueService(client)

		issue := &jira4claude.Issue{
			Project: "TEST",
			Summary: "No description issue",
			Type:    "Task",
		}

		result, err := svc.Create(context.Background(), issue)

		require.NoError(t, err)
		assert.Equal(t, "TEST-2", result.Key)

		// Verify description field is not sent
		fields := receivedRequest["fields"].(map[string]any)
		_, hasDescription := fields["description"]
		assert.False(t, hasDescription)
	})

	t.Run("returns error on API failure", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"errorMessages": ["Project is required"], "errors": {}}`))
		}))
		defer server.Close()

		client := newTestClient(t, server.URL, "user@example.com", "api-token")
		svc := jirahttp.NewIssueService(client)

		_, err := svc.Create(context.Background(), &jira4claude.Issue{})

		require.Error(t, err)
		assert.Equal(t, jira4claude.EValidation, jira4claude.ErrorCode(err))
	})
}

func TestIssueService_Get(t *testing.T) {
	t.Parallel()

	t.Run("retrieves issue by key", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet || r.URL.Path != "/rest/api/3/issue/TEST-1" {
				w.WriteHeader(http.StatusNotFound)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"key": "TEST-1",
				"fields": {
					"project": {"key": "TEST"},
					"summary": "Test issue",
					"description": {
						"type": "doc",
						"version": 1,
						"content": [{"type": "paragraph", "content": [{"type": "text", "text": "Description here"}]}]
					},
					"status": {"name": "To Do"},
					"issuetype": {"name": "Task"},
					"priority": {"name": "Medium"},
					"assignee": {"accountId": "123", "displayName": "John Doe", "emailAddress": "john@example.com"},
					"reporter": {"accountId": "456", "displayName": "Jane Smith", "emailAddress": "jane@example.com"},
					"labels": ["bug", "urgent"],
					"created": "2024-01-15T10:30:00.000+0000",
					"updated": "2024-01-16T14:20:00.000+0000"
				}
			}`))
		}))
		defer server.Close()

		client := newTestClient(t, server.URL, "user@example.com", "api-token")
		svc := jirahttp.NewIssueService(client)

		issue, err := svc.Get(context.Background(), "TEST-1")

		require.NoError(t, err)
		assert.Equal(t, "TEST-1", issue.Key)
		assert.Equal(t, "TEST", issue.Project)
		assert.Equal(t, "Test issue", issue.Summary)
		assert.Equal(t, "Description here", issue.Description)
		assert.Equal(t, "To Do", issue.Status)
		assert.Equal(t, "Task", issue.Type)
		assert.Equal(t, "Medium", issue.Priority)
		assert.NotNil(t, issue.Assignee)
		assert.Equal(t, "123", issue.Assignee.AccountID)
		assert.Equal(t, "John Doe", issue.Assignee.DisplayName)
		assert.NotNil(t, issue.Reporter)
		assert.Equal(t, []string{"bug", "urgent"}, issue.Labels)
	})

	t.Run("returns not found error for missing issue", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"errorMessages": ["Issue does not exist"], "errors": {}}`))
		}))
		defer server.Close()

		client := newTestClient(t, server.URL, "user@example.com", "api-token")
		svc := jirahttp.NewIssueService(client)

		_, err := svc.Get(context.Background(), "NOTFOUND-1")

		require.Error(t, err)
		assert.Equal(t, jira4claude.ENotFound, jira4claude.ErrorCode(err))
	})
}

func TestIssueService_List(t *testing.T) {
	t.Parallel()

	t.Run("lists issues with filter fields", func(t *testing.T) {
		t.Parallel()

		var receivedJQL string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet || r.URL.Path != "/rest/api/3/search/jql" {
				w.WriteHeader(http.StatusNotFound)
				return
			}

			receivedJQL = r.URL.Query().Get("jql")

			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"issues": [
					{
						"key": "TEST-1",
						"fields": {
							"project": {"key": "TEST"},
							"summary": "First issue",
							"status": {"name": "To Do"},
							"issuetype": {"name": "Task"}
						}
					},
					{
						"key": "TEST-2",
						"fields": {
							"project": {"key": "TEST"},
							"summary": "Second issue",
							"status": {"name": "In Progress"},
							"issuetype": {"name": "Bug"}
						}
					}
				]
			}`))
		}))
		defer server.Close()

		client := newTestClient(t, server.URL, "user@example.com", "api-token")
		svc := jirahttp.NewIssueService(client)

		issues, err := svc.List(context.Background(), jira4claude.IssueFilter{
			Project: "TEST",
			Status:  "To Do",
		})

		require.NoError(t, err)
		require.Len(t, issues, 2)
		assert.Equal(t, "TEST-1", issues[0].Key)
		assert.Equal(t, "TEST-2", issues[1].Key)
		assert.Contains(t, receivedJQL, "project = \"TEST\"")
		assert.Contains(t, receivedJQL, "status = \"To Do\"")
	})

	t.Run("uses raw JQL when provided", func(t *testing.T) {
		t.Parallel()

		var receivedJQL string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedJQL = r.URL.Query().Get("jql")
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"issues": []}`))
		}))
		defer server.Close()

		client := newTestClient(t, server.URL, "user@example.com", "api-token")
		svc := jirahttp.NewIssueService(client)

		_, err := svc.List(context.Background(), jira4claude.IssueFilter{
			JQL:     "project = CUSTOM AND assignee = currentUser()",
			Project: "IGNORED",
		})

		require.NoError(t, err)
		assert.Equal(t, "project = CUSTOM AND assignee = currentUser()", receivedJQL)
	})

	t.Run("respects limit parameter", func(t *testing.T) {
		t.Parallel()

		var receivedMaxResults string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedMaxResults = r.URL.Query().Get("maxResults")
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"issues": []}`))
		}))
		defer server.Close()

		client := newTestClient(t, server.URL, "user@example.com", "api-token")
		svc := jirahttp.NewIssueService(client)

		_, err := svc.List(context.Background(), jira4claude.IssueFilter{
			Project: "TEST",
			Limit:   25,
		})

		require.NoError(t, err)
		assert.Equal(t, "25", receivedMaxResults)
	})

	t.Run("includes labels in JQL filter", func(t *testing.T) {
		t.Parallel()

		var receivedJQL string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedJQL = r.URL.Query().Get("jql")
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"issues": []}`))
		}))
		defer server.Close()

		client := newTestClient(t, server.URL, "user@example.com", "api-token")
		svc := jirahttp.NewIssueService(client)

		_, err := svc.List(context.Background(), jira4claude.IssueFilter{
			Project: "TEST",
			Labels:  []string{"bug", "urgent"},
		})

		require.NoError(t, err)
		assert.Contains(t, receivedJQL, "project = \"TEST\"")
		assert.Contains(t, receivedJQL, "labels = \"bug\"")
		assert.Contains(t, receivedJQL, "labels = \"urgent\"")
	})
}

func TestIssueService_Update(t *testing.T) {
	t.Parallel()

	t.Run("updates issue fields", func(t *testing.T) {
		t.Parallel()

		var receivedRequest map[string]any
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodPut && r.URL.Path == "/rest/api/3/issue/TEST-1" {
				_ = json.NewDecoder(r.Body).Decode(&receivedRequest)
				w.WriteHeader(http.StatusNoContent)
				return
			}

			if r.Method == http.MethodGet && r.URL.Path == "/rest/api/3/issue/TEST-1" {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{
					"key": "TEST-1",
					"fields": {
						"project": {"key": "TEST"},
						"summary": "Updated summary",
						"description": {"type": "doc", "version": 1, "content": [{"type": "paragraph", "content": [{"type": "text", "text": "Updated description"}]}]},
						"status": {"name": "To Do"},
						"issuetype": {"name": "Task"},
						"priority": {"name": "High"}
					}
				}`))
				return
			}

			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		client := newTestClient(t, server.URL, "user@example.com", "api-token")
		svc := jirahttp.NewIssueService(client)

		newSummary := "Updated summary"
		newDescription := "Updated description"
		newPriority := "High"
		result, err := svc.Update(context.Background(), "TEST-1", jira4claude.IssueUpdate{
			Summary:     &newSummary,
			Description: &newDescription,
			Priority:    &newPriority,
		})

		require.NoError(t, err)
		assert.Equal(t, "Updated summary", result.Summary)
		assert.Equal(t, "High", result.Priority)

		// Verify request structure
		fields := receivedRequest["fields"].(map[string]any)
		assert.Equal(t, "Updated summary", fields["summary"])
		assert.Equal(t, "High", fields["priority"].(map[string]any)["name"])
	})

	t.Run("unassigns issue when assignee is empty string", func(t *testing.T) {
		t.Parallel()

		var receivedRequest map[string]any
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodPut {
				_ = json.NewDecoder(r.Body).Decode(&receivedRequest)
				w.WriteHeader(http.StatusNoContent)
				return
			}
			if r.Method == http.MethodGet {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"key": "TEST-1", "fields": {"project": {"key": "TEST"}, "summary": "Test", "status": {"name": "To Do"}, "issuetype": {"name": "Task"}}}`))
				return
			}
			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		client := newTestClient(t, server.URL, "user@example.com", "api-token")
		svc := jirahttp.NewIssueService(client)

		emptyAssignee := ""
		_, err := svc.Update(context.Background(), "TEST-1", jira4claude.IssueUpdate{
			Assignee: &emptyAssignee,
		})

		require.NoError(t, err)
		fields := receivedRequest["fields"].(map[string]any)
		assert.Nil(t, fields["assignee"])
	})

	t.Run("updates labels", func(t *testing.T) {
		t.Parallel()

		var receivedRequest map[string]any
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodPut {
				_ = json.NewDecoder(r.Body).Decode(&receivedRequest)
				w.WriteHeader(http.StatusNoContent)
				return
			}
			if r.Method == http.MethodGet {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"key": "TEST-1", "fields": {"project": {"key": "TEST"}, "summary": "Test", "status": {"name": "To Do"}, "issuetype": {"name": "Task"}, "labels": ["new-label"]}}`))
				return
			}
			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		client := newTestClient(t, server.URL, "user@example.com", "api-token")
		svc := jirahttp.NewIssueService(client)

		newLabels := []string{"new-label"}
		result, err := svc.Update(context.Background(), "TEST-1", jira4claude.IssueUpdate{
			Labels: &newLabels,
		})

		require.NoError(t, err)
		assert.Equal(t, []string{"new-label"}, result.Labels)

		fields := receivedRequest["fields"].(map[string]any)
		assert.Equal(t, []any{"new-label"}, fields["labels"])
	})

	t.Run("returns error on not found", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"errorMessages": ["Issue does not exist"], "errors": {}}`))
		}))
		defer server.Close()

		client := newTestClient(t, server.URL, "user@example.com", "api-token")
		svc := jirahttp.NewIssueService(client)

		newSummary := "New summary"
		_, err := svc.Update(context.Background(), "NOTFOUND-1", jira4claude.IssueUpdate{
			Summary: &newSummary,
		})

		require.Error(t, err)
		assert.Equal(t, jira4claude.ENotFound, jira4claude.ErrorCode(err))
	})
}

func TestIssueService_AddComment(t *testing.T) {
	t.Parallel()

	t.Run("adds comment and returns it", func(t *testing.T) {
		t.Parallel()

		var receivedRequest map[string]any
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost || r.URL.Path != "/rest/api/3/issue/TEST-1/comment" {
				w.WriteHeader(http.StatusNotFound)
				return
			}

			_ = json.NewDecoder(r.Body).Decode(&receivedRequest)

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{
				"id": "10001",
				"author": {
					"accountId": "123",
					"displayName": "John Doe",
					"emailAddress": "john@example.com"
				},
				"body": {
					"type": "doc",
					"version": 1,
					"content": [{"type": "paragraph", "content": [{"type": "text", "text": "This is a comment"}]}]
				},
				"created": "2024-01-15T10:30:00.000+0000"
			}`))
		}))
		defer server.Close()

		client := newTestClient(t, server.URL, "user@example.com", "api-token")
		svc := jirahttp.NewIssueService(client)

		comment, err := svc.AddComment(context.Background(), "TEST-1", "This is a comment")

		require.NoError(t, err)
		assert.Equal(t, "10001", comment.ID)
		assert.Equal(t, "123", comment.Author.AccountID)
		assert.Equal(t, "John Doe", comment.Author.DisplayName)
		assert.Equal(t, "This is a comment", comment.Body)
		assert.False(t, comment.Created.IsZero())

		// Verify request body is in ADF format
		body := receivedRequest["body"].(map[string]any)
		assert.Equal(t, "doc", body["type"])
	})

	t.Run("returns error when issue not found", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"errorMessages": ["Issue does not exist"], "errors": {}}`))
		}))
		defer server.Close()

		client := newTestClient(t, server.URL, "user@example.com", "api-token")
		svc := jirahttp.NewIssueService(client)

		_, err := svc.AddComment(context.Background(), "NOTFOUND-1", "Comment text")

		require.Error(t, err)
		assert.Equal(t, jira4claude.ENotFound, jira4claude.ErrorCode(err))
	})

	t.Run("handles multiline comment text", func(t *testing.T) {
		t.Parallel()

		var receivedRequest map[string]any
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewDecoder(r.Body).Decode(&receivedRequest)

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{
				"id": "10002",
				"body": {
					"type": "doc",
					"version": 1,
					"content": [
						{"type": "paragraph", "content": [{"type": "text", "text": "Line 1\nLine 2\nLine 3"}]}
					]
				},
				"created": "2024-01-15T10:30:00.000+0000"
			}`))
		}))
		defer server.Close()

		client := newTestClient(t, server.URL, "user@example.com", "api-token")
		svc := jirahttp.NewIssueService(client)

		comment, err := svc.AddComment(context.Background(), "TEST-1", "Line 1\nLine 2\nLine 3")

		require.NoError(t, err)
		assert.Equal(t, "10002", comment.ID)
		assert.Equal(t, "Line 1\nLine 2\nLine 3", comment.Body)

		// Verify request body has ADF format
		body := receivedRequest["body"].(map[string]any)
		assert.Equal(t, "doc", body["type"])
		content := body["content"].([]any)
		require.Len(t, content, 1)
		paragraph := content[0].(map[string]any)
		paragraphContent := paragraph["content"].([]any)
		textNode := paragraphContent[0].(map[string]any)
		assert.Equal(t, "Line 1\nLine 2\nLine 3", textNode["text"])
	})
}

func TestIssueService_Delete(t *testing.T) {
	t.Parallel()

	t.Run("deletes issue successfully", func(t *testing.T) {
		t.Parallel()

		var deleteCalled bool
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodDelete && r.URL.Path == "/rest/api/3/issue/TEST-1" {
				deleteCalled = true
				w.WriteHeader(http.StatusNoContent)
				return
			}
			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		client := newTestClient(t, server.URL, "user@example.com", "api-token")
		svc := jirahttp.NewIssueService(client)

		err := svc.Delete(context.Background(), "TEST-1")

		require.NoError(t, err)
		assert.True(t, deleteCalled)
	})

	t.Run("returns error when issue not found", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"errorMessages": ["Issue does not exist"], "errors": {}}`))
		}))
		defer server.Close()

		client := newTestClient(t, server.URL, "user@example.com", "api-token")
		svc := jirahttp.NewIssueService(client)

		err := svc.Delete(context.Background(), "NOTFOUND-1")

		require.Error(t, err)
		assert.Equal(t, jira4claude.ENotFound, jira4claude.ErrorCode(err))
	})
}

func TestIssueService_Transitions(t *testing.T) {
	t.Parallel()

	t.Run("returns available transitions", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet || r.URL.Path != "/rest/api/3/issue/TEST-1/transitions" {
				w.WriteHeader(http.StatusNotFound)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"transitions": [
					{"id": "11", "name": "To Do"},
					{"id": "21", "name": "In Progress"},
					{"id": "31", "name": "Done"}
				]
			}`))
		}))
		defer server.Close()

		client := newTestClient(t, server.URL, "user@example.com", "api-token")
		svc := jirahttp.NewIssueService(client)

		transitions, err := svc.Transitions(context.Background(), "TEST-1")

		require.NoError(t, err)
		require.Len(t, transitions, 3)
		assert.Equal(t, "11", transitions[0].ID)
		assert.Equal(t, "To Do", transitions[0].Name)
		assert.Equal(t, "21", transitions[1].ID)
		assert.Equal(t, "In Progress", transitions[1].Name)
		assert.Equal(t, "31", transitions[2].ID)
		assert.Equal(t, "Done", transitions[2].Name)
	})

	t.Run("returns error when issue not found", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"errorMessages": ["Issue does not exist"], "errors": {}}`))
		}))
		defer server.Close()

		client := newTestClient(t, server.URL, "user@example.com", "api-token")
		svc := jirahttp.NewIssueService(client)

		_, err := svc.Transitions(context.Background(), "NOTFOUND-1")

		require.Error(t, err)
		assert.Equal(t, jira4claude.ENotFound, jira4claude.ErrorCode(err))
	})

	t.Run("returns empty list when no transitions available", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"transitions": []}`))
		}))
		defer server.Close()

		client := newTestClient(t, server.URL, "user@example.com", "api-token")
		svc := jirahttp.NewIssueService(client)

		transitions, err := svc.Transitions(context.Background(), "TEST-1")

		require.NoError(t, err)
		assert.Empty(t, transitions)
	})
}

func TestIssueService_Transition(t *testing.T) {
	t.Parallel()

	t.Run("transitions issue to new status", func(t *testing.T) {
		t.Parallel()

		var receivedRequest map[string]any
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost || r.URL.Path != "/rest/api/3/issue/TEST-1/transitions" {
				w.WriteHeader(http.StatusNotFound)
				return
			}

			_ = json.NewDecoder(r.Body).Decode(&receivedRequest)
			w.WriteHeader(http.StatusNoContent)
		}))
		defer server.Close()

		client := newTestClient(t, server.URL, "user@example.com", "api-token")
		svc := jirahttp.NewIssueService(client)

		err := svc.Transition(context.Background(), "TEST-1", "21")

		require.NoError(t, err)

		// Verify request structure
		transition := receivedRequest["transition"].(map[string]any)
		assert.Equal(t, "21", transition["id"])
	})

	t.Run("returns error when issue not found", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"errorMessages": ["Issue does not exist"], "errors": {}}`))
		}))
		defer server.Close()

		client := newTestClient(t, server.URL, "user@example.com", "api-token")
		svc := jirahttp.NewIssueService(client)

		err := svc.Transition(context.Background(), "NOTFOUND-1", "21")

		require.Error(t, err)
		assert.Equal(t, jira4claude.ENotFound, jira4claude.ErrorCode(err))
	})

	t.Run("returns error for invalid transition", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"errorMessages": ["Transition is not valid for this issue"], "errors": {}}`))
		}))
		defer server.Close()

		client := newTestClient(t, server.URL, "user@example.com", "api-token")
		svc := jirahttp.NewIssueService(client)

		err := svc.Transition(context.Background(), "TEST-1", "999")

		require.Error(t, err)
		assert.Equal(t, jira4claude.EValidation, jira4claude.ErrorCode(err))
	})
}

func TestIssueService_Assign(t *testing.T) {
	t.Parallel()

	t.Run("assigns issue to user", func(t *testing.T) {
		t.Parallel()

		var receivedRequest map[string]any
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPut || r.URL.Path != "/rest/api/3/issue/TEST-1/assignee" {
				w.WriteHeader(http.StatusNotFound)
				return
			}

			_ = json.NewDecoder(r.Body).Decode(&receivedRequest)
			w.WriteHeader(http.StatusNoContent)
		}))
		defer server.Close()

		client := newTestClient(t, server.URL, "user@example.com", "api-token")
		svc := jirahttp.NewIssueService(client)

		err := svc.Assign(context.Background(), "TEST-1", "abc123")

		require.NoError(t, err)
		assert.Equal(t, "abc123", receivedRequest["accountId"])
	})

	t.Run("unassigns issue when accountID is empty", func(t *testing.T) {
		t.Parallel()

		var receivedRequest map[string]any
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPut || r.URL.Path != "/rest/api/3/issue/TEST-1/assignee" {
				w.WriteHeader(http.StatusNotFound)
				return
			}

			_ = json.NewDecoder(r.Body).Decode(&receivedRequest)
			w.WriteHeader(http.StatusNoContent)
		}))
		defer server.Close()

		client := newTestClient(t, server.URL, "user@example.com", "api-token")
		svc := jirahttp.NewIssueService(client)

		err := svc.Assign(context.Background(), "TEST-1", "")

		require.NoError(t, err)
		assert.Nil(t, receivedRequest["accountId"])
	})

	t.Run("returns error when issue not found", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"errorMessages": ["Issue does not exist"], "errors": {}}`))
		}))
		defer server.Close()

		client := newTestClient(t, server.URL, "user@example.com", "api-token")
		svc := jirahttp.NewIssueService(client)

		err := svc.Assign(context.Background(), "NOTFOUND-1", "abc123")

		require.Error(t, err)
		assert.Equal(t, jira4claude.ENotFound, jira4claude.ErrorCode(err))
	})
}
