package http_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/fwojciec/jira4claude"
	jirahttp "github.com/fwojciec/jira4claude/http"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIssueService_Create(t *testing.T) {
	t.Parallel()

	t.Run("uses pre-converted ADF when description is ADF JSON", func(t *testing.T) {
		t.Parallel()

		var receivedRequest map[string]any
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewDecoder(r.Body).Decode(&receivedRequest)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"key": "TEST-1"}`))
		}))
		defer server.Close()

		client := newTestClient(t, server.URL, "user@example.com", "api-token")
		svc := jirahttp.NewIssueService(client)

		// ADF document (now passed directly as map[string]any)
		adfDoc := jira4claude.ADF{
			"type":    "doc",
			"version": 1,
			"content": []any{
				map[string]any{
					"type": "paragraph",
					"content": []any{
						map[string]any{
							"type": "text",
							"text": "bold",
							"marks": []any{
								map[string]any{"type": "strong"},
							},
						},
					},
				},
			},
		}

		issue := &jira4claude.Issue{
			Project:     "TEST",
			Summary:     "Test issue",
			Description: adfDoc,
			Type:        "Task",
		}

		_, err := svc.Create(context.Background(), issue)

		require.NoError(t, err)

		// Verify the ADF was passed through as-is, not re-converted
		fields := receivedRequest["fields"].(map[string]any)
		desc := fields["description"].(map[string]any)
		assert.Equal(t, "doc", desc["type"])
		content := desc["content"].([]any)
		require.Len(t, content, 1)
		paragraph := content[0].(map[string]any)
		paragraphContent := paragraph["content"].([]any)
		require.Len(t, paragraphContent, 1)
		textNode := paragraphContent[0].(map[string]any)
		assert.Equal(t, "bold", textNode["text"])
		marks := textNode["marks"].([]any)
		require.Len(t, marks, 1)
		assert.Equal(t, "strong", marks[0].(map[string]any)["type"])
	})

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
			Project: "TEST",
			Summary: "Test issue",
			Description: jira4claude.ADF{"type": "doc", "version": 1, "content": []any{
				map[string]any{"type": "paragraph", "content": []any{
					map[string]any{"type": "text", "text": "This is a test description"},
				}},
			}},
			Type: "Task",
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

	t.Run("creates subtask with parent field", func(t *testing.T) {
		t.Parallel()

		var receivedRequest map[string]any
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewDecoder(r.Body).Decode(&receivedRequest)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"key": "TEST-3"}`))
		}))
		defer server.Close()

		client := newTestClient(t, server.URL, "user@example.com", "api-token")
		svc := jirahttp.NewIssueService(client)

		issue := &jira4claude.Issue{
			Project: "TEST",
			Summary: "Subtask issue",
			Type:    "Sub-task",
			Parent:  &jira4claude.LinkedIssue{Key: "TEST-1"},
		}

		result, err := svc.Create(context.Background(), issue)

		require.NoError(t, err)
		assert.Equal(t, "TEST-3", result.Key)
		require.NotNil(t, result.Parent)
		assert.Equal(t, "TEST-1", result.Parent.Key)

		// Verify parent field is sent in request
		fields := receivedRequest["fields"].(map[string]any)
		parent := fields["parent"].(map[string]any)
		assert.Equal(t, "TEST-1", parent["key"])
	})

	t.Run("creates issue without parent when not set", func(t *testing.T) {
		t.Parallel()

		var receivedRequest map[string]any
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewDecoder(r.Body).Decode(&receivedRequest)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"key": "TEST-4"}`))
		}))
		defer server.Close()

		client := newTestClient(t, server.URL, "user@example.com", "api-token")
		svc := jirahttp.NewIssueService(client)

		issue := &jira4claude.Issue{
			Project: "TEST",
			Summary: "Regular issue",
			Type:    "Task",
		}

		_, err := svc.Create(context.Background(), issue)

		require.NoError(t, err)

		// Verify parent field is NOT sent in request
		fields := receivedRequest["fields"].(map[string]any)
		_, hasParent := fields["parent"]
		assert.False(t, hasParent)
	})
}

func TestIssueService_Get(t *testing.T) {
	t.Parallel()

	t.Run("escapes special characters in key", func(t *testing.T) {
		t.Parallel()

		var receivedRawPath string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// RawPath contains the encoded path when it differs from Path
			receivedRawPath = r.URL.RawPath
			if receivedRawPath == "" {
				receivedRawPath = r.URL.Path
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"key": "TEST/1",
				"fields": {
					"project": {"key": "TEST"},
					"summary": "Test issue",
					"status": {"name": "To Do"},
					"issuetype": {"name": "Task"}
				}
			}`))
		}))
		defer server.Close()

		client := newTestClient(t, server.URL, "user@example.com", "api-token")
		svc := jirahttp.NewIssueService(client)

		_, err := svc.Get(context.Background(), "TEST/1")

		require.NoError(t, err)
		// The slash should be escaped as %2F
		assert.Equal(t, "/rest/api/3/issue/TEST%2F1", receivedRawPath)
	})

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
		// Description is now ADF (map[string]any)
		assert.Equal(t, "doc", issue.Description["type"])
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

	t.Run("returns subtask with parent as LinkedIssue", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet || r.URL.Path != "/rest/api/3/issue/TEST-2" {
				w.WriteHeader(http.StatusNotFound)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"key": "TEST-2",
				"fields": {
					"project": {"key": "TEST"},
					"summary": "Subtask issue",
					"status": {"name": "To Do"},
					"issuetype": {"name": "Sub-task"},
					"parent": {
						"key": "TEST-1",
						"fields": {
							"summary": "Parent issue",
							"status": {"name": "In Progress"},
							"issuetype": {"name": "Epic"}
						}
					}
				}
			}`))
		}))
		defer server.Close()

		client := newTestClient(t, server.URL, "user@example.com", "api-token")
		svc := jirahttp.NewIssueService(client)

		issue, err := svc.Get(context.Background(), "TEST-2")

		require.NoError(t, err)
		assert.Equal(t, "TEST-2", issue.Key)
		assert.Equal(t, "Sub-task", issue.Type)
		require.NotNil(t, issue.Parent)
		assert.Equal(t, "TEST-1", issue.Parent.Key)
		assert.Equal(t, "Parent issue", issue.Parent.Summary)
		assert.Equal(t, "In Progress", issue.Parent.Status)
		assert.Equal(t, "Epic", issue.Parent.Type)
	})

	t.Run("returns issue without parent when not a subtask", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet || r.URL.Path != "/rest/api/3/issue/TEST-3" {
				w.WriteHeader(http.StatusNotFound)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"key": "TEST-3",
				"fields": {
					"project": {"key": "TEST"},
					"summary": "Regular task",
					"status": {"name": "To Do"},
					"issuetype": {"name": "Task"}
				}
			}`))
		}))
		defer server.Close()

		client := newTestClient(t, server.URL, "user@example.com", "api-token")
		svc := jirahttp.NewIssueService(client)

		issue, err := svc.Get(context.Background(), "TEST-3")

		require.NoError(t, err)
		assert.Equal(t, "TEST-3", issue.Key)
		assert.Nil(t, issue.Parent)
	})

	t.Run("returns issue with links", func(t *testing.T) {
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
					"status": {"name": "To Do"},
					"issuetype": {"name": "Task"},
					"issuelinks": [
						{
							"id": "10001",
							"type": {"name": "Blocks", "inward": "is blocked by", "outward": "blocks"},
							"outwardIssue": {
								"key": "TEST-2",
								"fields": {
									"summary": "Blocked issue",
									"status": {"name": "In Progress"},
									"issuetype": {"name": "Bug"}
								}
							}
						},
						{
							"id": "10002",
							"type": {"name": "Blocks", "inward": "is blocked by", "outward": "blocks"},
							"inwardIssue": {
								"key": "TEST-3",
								"fields": {
									"summary": "Blocking issue",
									"status": {"name": "Done"},
									"issuetype": {"name": "Task"}
								}
							}
						}
					]
				}
			}`))
		}))
		defer server.Close()

		client := newTestClient(t, server.URL, "user@example.com", "api-token")
		svc := jirahttp.NewIssueService(client)

		issue, err := svc.Get(context.Background(), "TEST-1")

		require.NoError(t, err)
		require.Len(t, issue.Links, 2)

		// First link: outward (TEST-1 blocks TEST-2)
		assert.Equal(t, "10001", issue.Links[0].ID)
		assert.Equal(t, "Blocks", issue.Links[0].Type.Name)
		assert.Equal(t, "is blocked by", issue.Links[0].Type.Inward)
		assert.Equal(t, "blocks", issue.Links[0].Type.Outward)
		require.NotNil(t, issue.Links[0].OutwardIssue)
		assert.Equal(t, "TEST-2", issue.Links[0].OutwardIssue.Key)
		assert.Equal(t, "Blocked issue", issue.Links[0].OutwardIssue.Summary)
		assert.Equal(t, "In Progress", issue.Links[0].OutwardIssue.Status)
		assert.Equal(t, "Bug", issue.Links[0].OutwardIssue.Type)
		assert.Nil(t, issue.Links[0].InwardIssue)

		// Second link: inward (TEST-3 blocks TEST-1)
		assert.Equal(t, "10002", issue.Links[1].ID)
		require.NotNil(t, issue.Links[1].InwardIssue)
		assert.Equal(t, "TEST-3", issue.Links[1].InwardIssue.Key)
		assert.Equal(t, "Blocking issue", issue.Links[1].InwardIssue.Summary)
		assert.Equal(t, "Done", issue.Links[1].InwardIssue.Status)
		assert.Equal(t, "Task", issue.Links[1].InwardIssue.Type)
		assert.Nil(t, issue.Links[1].OutwardIssue)
	})

	t.Run("returns issue with comments", func(t *testing.T) {
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
					"status": {"name": "To Do"},
					"issuetype": {"name": "Task"},
					"comment": {
						"comments": [
							{
								"id": "10001",
								"author": {
									"accountId": "user123",
									"displayName": "John Doe",
									"emailAddress": "john@example.com"
								},
								"body": {
									"type": "doc",
									"version": 1,
									"content": [{"type": "paragraph", "content": [{"type": "text", "text": "First comment"}]}]
								},
								"created": "2024-01-15T10:30:00.000+0000"
							},
							{
								"id": "10002",
								"author": {
									"accountId": "user456",
									"displayName": "Jane Smith",
									"emailAddress": "jane@example.com"
								},
								"body": {
									"type": "doc",
									"version": 1,
									"content": [{"type": "paragraph", "content": [{"type": "text", "text": "Second comment"}]}]
								},
								"created": "2024-01-16T14:20:00.000+0000"
							}
						],
						"total": 2
					}
				}
			}`))
		}))
		defer server.Close()

		client := newTestClient(t, server.URL, "user@example.com", "api-token")
		svc := jirahttp.NewIssueService(client)

		issue, err := svc.Get(context.Background(), "TEST-1")

		require.NoError(t, err)
		require.Len(t, issue.Comments, 2)

		// First comment - Body is now ADF (map[string]any)
		assert.Equal(t, "10001", issue.Comments[0].ID)
		require.NotNil(t, issue.Comments[0].Author)
		assert.Equal(t, "user123", issue.Comments[0].Author.AccountID)
		assert.Equal(t, "John Doe", issue.Comments[0].Author.DisplayName)
		assert.Equal(t, "doc", issue.Comments[0].Body["type"])
		assert.False(t, issue.Comments[0].Created.IsZero())

		// Second comment - Body is now ADF (map[string]any)
		assert.Equal(t, "10002", issue.Comments[1].ID)
		require.NotNil(t, issue.Comments[1].Author)
		assert.Equal(t, "user456", issue.Comments[1].Author.AccountID)
		assert.Equal(t, "Jane Smith", issue.Comments[1].Author.DisplayName)
		assert.Equal(t, "doc", issue.Comments[1].Body["type"])
		assert.False(t, issue.Comments[1].Created.IsZero())
	})

	t.Run("returns issue without comments when none exist", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"key": "TEST-1",
				"fields": {
					"project": {"key": "TEST"},
					"summary": "Test issue",
					"status": {"name": "To Do"},
					"issuetype": {"name": "Task"}
				}
			}`))
		}))
		defer server.Close()

		client := newTestClient(t, server.URL, "user@example.com", "api-token")
		svc := jirahttp.NewIssueService(client)

		issue, err := svc.Get(context.Background(), "TEST-1")

		require.NoError(t, err)
		assert.Nil(t, issue.Comments)
	})

	t.Run("returns comment body as GFM with ADF preserved", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"key": "TEST-1",
				"fields": {
					"project": {"key": "TEST"},
					"summary": "Test issue",
					"status": {"name": "To Do"},
					"issuetype": {"name": "Task"},
					"comment": {
						"comments": [
							{
								"id": "10001",
								"body": {
									"type": "doc",
									"version": 1,
									"content": [{"type": "paragraph", "content": [{"type": "text", "text": "Comment with ", "marks": []}, {"type": "text", "text": "bold", "marks": [{"type": "strong"}]}]}]
								},
								"created": "2024-01-15T10:30:00.000+0000"
							}
						],
						"total": 1
					}
				}
			}`))
		}))
		defer server.Close()

		client := newTestClient(t, server.URL, "user@example.com", "api-token")
		svc := jirahttp.NewIssueService(client)

		issue, err := svc.Get(context.Background(), "TEST-1")

		require.NoError(t, err)
		require.Len(t, issue.Comments, 1)
		// Body is now ADF directly
		require.NotNil(t, issue.Comments[0].Body)
		assert.Equal(t, "doc", issue.Comments[0].Body["type"])
	})

	t.Run("returns parent issue with subtasks", func(t *testing.T) {
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
					"summary": "Parent issue",
					"status": {"name": "In Progress"},
					"issuetype": {"name": "Task"},
					"subtasks": [
						{
							"key": "TEST-2",
							"fields": {
								"summary": "First subtask",
								"status": {"name": "Done"},
								"issuetype": {"name": "Sub-task"}
							}
						},
						{
							"key": "TEST-3",
							"fields": {
								"summary": "Second subtask",
								"status": {"name": "To Do"},
								"issuetype": {"name": "Sub-task"}
							}
						}
					]
				}
			}`))
		}))
		defer server.Close()

		client := newTestClient(t, server.URL, "user@example.com", "api-token")
		svc := jirahttp.NewIssueService(client)

		issue, err := svc.Get(context.Background(), "TEST-1")

		require.NoError(t, err)
		require.Len(t, issue.Subtasks, 2)

		// First subtask
		assert.Equal(t, "TEST-2", issue.Subtasks[0].Key)
		assert.Equal(t, "First subtask", issue.Subtasks[0].Summary)
		assert.Equal(t, "Done", issue.Subtasks[0].Status)
		assert.Equal(t, "Sub-task", issue.Subtasks[0].Type)

		// Second subtask
		assert.Equal(t, "TEST-3", issue.Subtasks[1].Key)
		assert.Equal(t, "Second subtask", issue.Subtasks[1].Summary)
		assert.Equal(t, "To Do", issue.Subtasks[1].Status)
		assert.Equal(t, "Sub-task", issue.Subtasks[1].Type)
	})

	t.Run("returns issue without subtasks when none exist", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"key": "TEST-1",
				"fields": {
					"project": {"key": "TEST"},
					"summary": "Issue without subtasks",
					"status": {"name": "To Do"},
					"issuetype": {"name": "Task"},
					"subtasks": []
				}
			}`))
		}))
		defer server.Close()

		client := newTestClient(t, server.URL, "user@example.com", "api-token")
		svc := jirahttp.NewIssueService(client)

		issue, err := svc.Get(context.Background(), "TEST-1")

		require.NoError(t, err)
		assert.Nil(t, issue.Subtasks)
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

	t.Run("includes parent in JQL filter", func(t *testing.T) {
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
			Parent:  "TEST-1",
		})

		require.NoError(t, err)
		assert.Contains(t, receivedJQL, "project = \"TEST\"")
		assert.Contains(t, receivedJQL, "parent = \"TEST-1\"")
	})

	t.Run("includes assignee in JQL filter", func(t *testing.T) {
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
			Project:  "TEST",
			Assignee: "john.doe",
		})

		require.NoError(t, err)
		assert.Contains(t, receivedJQL, "project = \"TEST\"")
		assert.Contains(t, receivedJQL, "assignee = \"john.doe\"")
	})

	t.Run("combines all filter fields in JQL", func(t *testing.T) {
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
			Project:  "TEST",
			Status:   "In Progress",
			Assignee: "john.doe",
			Parent:   "TEST-1",
			Labels:   []string{"bug", "urgent"},
		})

		require.NoError(t, err)
		assert.Contains(t, receivedJQL, "project = \"TEST\"")
		assert.Contains(t, receivedJQL, "status = \"In Progress\"")
		assert.Contains(t, receivedJQL, "assignee = \"john.doe\"")
		assert.Contains(t, receivedJQL, "parent = \"TEST-1\"")
		assert.Contains(t, receivedJQL, "labels = \"bug\"")
		assert.Contains(t, receivedJQL, "labels = \"urgent\"")
		// Verify clauses are joined with AND
		assert.Contains(t, receivedJQL, " AND ")
	})

	t.Run("sends empty JQL when no filters provided", func(t *testing.T) {
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

		_, err := svc.List(context.Background(), jira4claude.IssueFilter{})

		require.NoError(t, err)
		assert.Empty(t, receivedJQL)
	})

	t.Run("omits maxResults when limit is zero", func(t *testing.T) {
		t.Parallel()

		var receivedMaxResults string
		var hasMaxResults bool
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedMaxResults = r.URL.Query().Get("maxResults")
			hasMaxResults = r.URL.Query().Has("maxResults")
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"issues": []}`))
		}))
		defer server.Close()

		client := newTestClient(t, server.URL, "user@example.com", "api-token")
		svc := jirahttp.NewIssueService(client)

		_, err := svc.List(context.Background(), jira4claude.IssueFilter{
			Project: "TEST",
			Limit:   0,
		})

		require.NoError(t, err)
		assert.False(t, hasMaxResults, "maxResults should not be present when Limit is 0")
		assert.Empty(t, receivedMaxResults)
	})
}

func TestIssueService_Update(t *testing.T) {
	t.Parallel()

	t.Run("passes ADF description directly to API", func(t *testing.T) {
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

		// ADF document (now passed directly as map[string]any)
		adfDoc := jira4claude.ADF{
			"type":    "doc",
			"version": 1,
			"content": []any{
				map[string]any{
					"type": "paragraph",
					"content": []any{
						map[string]any{
							"type": "text",
							"text": "italic",
							"marks": []any{
								map[string]any{"type": "em"},
							},
						},
					},
				},
			},
		}

		_, err := svc.Update(context.Background(), "TEST-1", jira4claude.IssueUpdate{
			Description: &adfDoc,
		})

		require.NoError(t, err)

		// Verify the ADF was passed through as-is
		fields := receivedRequest["fields"].(map[string]any)
		desc := fields["description"].(map[string]any)
		assert.Equal(t, "doc", desc["type"])
		content := desc["content"].([]any)
		require.Len(t, content, 1)
		paragraph := content[0].(map[string]any)
		paragraphContent := paragraph["content"].([]any)
		require.Len(t, paragraphContent, 1)
		textNode := paragraphContent[0].(map[string]any)
		assert.Equal(t, "italic", textNode["text"])
		marks := textNode["marks"].([]any)
		require.Len(t, marks, 1)
		assert.Equal(t, "em", marks[0].(map[string]any)["type"])
	})

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
		newDescription := jira4claude.ADF{"type": "doc", "version": 1, "content": []any{
			map[string]any{"type": "paragraph", "content": []any{
				map[string]any{"type": "text", "text": "Updated description"},
			}},
		}}
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

	t.Run("passes ADF body directly to API", func(t *testing.T) {
		t.Parallel()

		var receivedRequest map[string]any
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewDecoder(r.Body).Decode(&receivedRequest)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{
				"id": "10001",
				"author": {"accountId": "123", "displayName": "Test"},
				"body": {"type": "doc", "version": 1, "content": []},
				"created": "2024-01-15T10:30:00.000+0000"
			}`))
		}))
		defer server.Close()

		client := newTestClient(t, server.URL, "user@example.com", "api-token")
		svc := jirahttp.NewIssueService(client)

		// ADF document (now passed directly as map[string]any)
		adfDoc := jira4claude.ADF{
			"type":    "doc",
			"version": 1,
			"content": []any{
				map[string]any{
					"type": "paragraph",
					"content": []any{
						map[string]any{
							"type": "text",
							"text": "code",
							"marks": []any{
								map[string]any{"type": "code"},
							},
						},
					},
				},
			},
		}

		_, err := svc.AddComment(context.Background(), "TEST-1", adfDoc)

		require.NoError(t, err)

		// Verify the ADF was passed through as-is
		body := receivedRequest["body"].(map[string]any)
		assert.Equal(t, "doc", body["type"])
		content := body["content"].([]any)
		require.Len(t, content, 1)
		paragraph := content[0].(map[string]any)
		paragraphContent := paragraph["content"].([]any)
		require.Len(t, paragraphContent, 1)
		textNode := paragraphContent[0].(map[string]any)
		assert.Equal(t, "code", textNode["text"])
		marks := textNode["marks"].([]any)
		require.Len(t, marks, 1)
		assert.Equal(t, "code", marks[0].(map[string]any)["type"])
	})

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

		adfDoc := jira4claude.ADF{"type": "doc", "version": 1, "content": []any{
			map[string]any{"type": "paragraph", "content": []any{
				map[string]any{"type": "text", "text": "This is a comment"},
			}},
		}}
		comment, err := svc.AddComment(context.Background(), "TEST-1", adfDoc)

		require.NoError(t, err)
		assert.Equal(t, "10001", comment.ID)
		assert.Equal(t, "123", comment.Author.AccountID)
		assert.Equal(t, "John Doe", comment.Author.DisplayName)
		// Body is now ADF (map[string]any), not a string
		assert.Equal(t, "doc", comment.Body["type"])
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

		adfDoc := jira4claude.ADF{"type": "doc", "version": 1, "content": []any{
			map[string]any{"type": "paragraph", "content": []any{
				map[string]any{"type": "text", "text": "Comment text"},
			}},
		}}
		_, err := svc.AddComment(context.Background(), "NOTFOUND-1", adfDoc)

		require.Error(t, err)
		assert.Equal(t, jira4claude.ENotFound, jira4claude.ErrorCode(err))
	})

	t.Run("handles multiline comment text with paragraph breaks", func(t *testing.T) {
		t.Parallel()

		var receivedRequest map[string]any
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewDecoder(r.Body).Decode(&receivedRequest)

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			// Response uses paragraph nodes for paragraph breaks
			_, _ = w.Write([]byte(`{
				"id": "10002",
				"body": {
					"type": "doc",
					"version": 1,
					"content": [
						{"type": "paragraph", "content": [{"type": "text", "text": "Paragraph 1"}]},
						{"type": "paragraph", "content": [{"type": "text", "text": "Paragraph 2"}]},
						{"type": "paragraph", "content": [{"type": "text", "text": "Paragraph 3"}]}
					]
				},
				"created": "2024-01-15T10:30:00.000+0000"
			}`))
		}))
		defer server.Close()

		client := newTestClient(t, server.URL, "user@example.com", "api-token")
		svc := jirahttp.NewIssueService(client)

		// ADF with multiple paragraphs
		adfDoc := jira4claude.ADF{
			"type":    "doc",
			"version": 1,
			"content": []any{
				map[string]any{"type": "paragraph", "content": []any{
					map[string]any{"type": "text", "text": "Paragraph 1"},
				}},
				map[string]any{"type": "paragraph", "content": []any{
					map[string]any{"type": "text", "text": "Paragraph 2"},
				}},
				map[string]any{"type": "paragraph", "content": []any{
					map[string]any{"type": "text", "text": "Paragraph 3"},
				}},
			},
		}
		comment, err := svc.AddComment(context.Background(), "TEST-1", adfDoc)

		require.NoError(t, err)
		assert.Equal(t, "10002", comment.ID)
		// Body is now ADF (map[string]any), not a string
		assert.Equal(t, "doc", comment.Body["type"])

		// Verify request body has ADF format with paragraph nodes
		body := receivedRequest["body"].(map[string]any)
		assert.Equal(t, "doc", body["type"])
		content := body["content"].([]any)
		require.Len(t, content, 3) // Three paragraphs
		assert.Equal(t, "paragraph", content[0].(map[string]any)["type"])
		assert.Equal(t, "paragraph", content[1].(map[string]any)["type"])
		assert.Equal(t, "paragraph", content[2].(map[string]any)["type"])
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

func TestIssueService_Link(t *testing.T) {
	t.Parallel()

	t.Run("creates link between issues", func(t *testing.T) {
		t.Parallel()

		var receivedRequest map[string]any
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost || r.URL.Path != "/rest/api/3/issueLink" {
				w.WriteHeader(http.StatusNotFound)
				return
			}

			_ = json.NewDecoder(r.Body).Decode(&receivedRequest)
			w.WriteHeader(http.StatusCreated)
		}))
		defer server.Close()

		client := newTestClient(t, server.URL, "user@example.com", "api-token")
		svc := jirahttp.NewIssueService(client)

		err := svc.Link(context.Background(), "TEST-1", "Blocks", "TEST-2")

		require.NoError(t, err)

		// Verify request structure
		linkType := receivedRequest["type"].(map[string]any)
		assert.Equal(t, "Blocks", linkType["name"])

		inwardIssue := receivedRequest["inwardIssue"].(map[string]any)
		assert.Equal(t, "TEST-1", inwardIssue["key"])

		outwardIssue := receivedRequest["outwardIssue"].(map[string]any)
		assert.Equal(t, "TEST-2", outwardIssue["key"])
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

		err := svc.Link(context.Background(), "NOTFOUND-1", "Blocks", "TEST-2")

		require.Error(t, err)
		assert.Equal(t, jira4claude.ENotFound, jira4claude.ErrorCode(err))
	})

	t.Run("returns error for invalid link type", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"errorMessages": ["No issue link type with name 'Invalid' exists"], "errors": {}}`))
		}))
		defer server.Close()

		client := newTestClient(t, server.URL, "user@example.com", "api-token")
		svc := jirahttp.NewIssueService(client)

		err := svc.Link(context.Background(), "TEST-1", "Invalid", "TEST-2")

		require.Error(t, err)
		assert.Equal(t, jira4claude.EValidation, jira4claude.ErrorCode(err))
	})
}

func TestIssueService_Unlink(t *testing.T) {
	t.Parallel()

	t.Run("removes link between issues", func(t *testing.T) {
		t.Parallel()

		var deletedLinkID string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// First, the service fetches the issue to find the link
			if r.Method == http.MethodGet && r.URL.Path == "/rest/api/3/issue/TEST-1" {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{
					"key": "TEST-1",
					"fields": {
						"project": {"key": "TEST"},
						"summary": "Test issue",
						"status": {"name": "To Do"},
						"issuetype": {"name": "Task"},
						"issuelinks": [
							{
								"id": "10001",
								"type": {"name": "Blocks", "inward": "is blocked by", "outward": "blocks"},
								"outwardIssue": {
									"key": "TEST-2",
									"fields": {
										"summary": "Target issue",
										"status": {"name": "To Do"},
										"issuetype": {"name": "Task"}
									}
								}
							}
						]
					}
				}`))
				return
			}

			// Then, it deletes the link
			const linkPath = "/rest/api/3/issueLink/"
			if r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, linkPath) {
				deletedLinkID = strings.TrimPrefix(r.URL.Path, linkPath)
				w.WriteHeader(http.StatusNoContent)
				return
			}

			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		client := newTestClient(t, server.URL, "user@example.com", "api-token")
		svc := jirahttp.NewIssueService(client)

		err := svc.Unlink(context.Background(), "TEST-1", "TEST-2")

		require.NoError(t, err)
		assert.Equal(t, "10001", deletedLinkID)
	})

	t.Run("removes link when issues are in reverse order", func(t *testing.T) {
		t.Parallel()

		var deletedLinkID string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodGet && r.URL.Path == "/rest/api/3/issue/TEST-2" {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{
					"key": "TEST-2",
					"fields": {
						"project": {"key": "TEST"},
						"summary": "Test issue",
						"status": {"name": "To Do"},
						"issuetype": {"name": "Task"},
						"issuelinks": [
							{
								"id": "10002",
								"type": {"name": "Blocks", "inward": "is blocked by", "outward": "blocks"},
								"inwardIssue": {
									"key": "TEST-1",
									"fields": {
										"summary": "Source issue",
										"status": {"name": "To Do"},
										"issuetype": {"name": "Task"}
									}
								}
							}
						]
					}
				}`))
				return
			}

			const linkPath = "/rest/api/3/issueLink/"
			if r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, linkPath) {
				deletedLinkID = strings.TrimPrefix(r.URL.Path, linkPath)
				w.WriteHeader(http.StatusNoContent)
				return
			}

			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		client := newTestClient(t, server.URL, "user@example.com", "api-token")
		svc := jirahttp.NewIssueService(client)

		err := svc.Unlink(context.Background(), "TEST-2", "TEST-1")

		require.NoError(t, err)
		assert.Equal(t, "10002", deletedLinkID)
	})

	t.Run("returns error when no link exists", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodGet && r.URL.Path == "/rest/api/3/issue/TEST-1" {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{
					"key": "TEST-1",
					"fields": {
						"project": {"key": "TEST"},
						"summary": "Test issue",
						"status": {"name": "To Do"},
						"issuetype": {"name": "Task"},
						"issuelinks": []
					}
				}`))
				return
			}
			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		client := newTestClient(t, server.URL, "user@example.com", "api-token")
		svc := jirahttp.NewIssueService(client)

		err := svc.Unlink(context.Background(), "TEST-1", "TEST-2")

		require.Error(t, err)
		assert.Equal(t, jira4claude.ENotFound, jira4claude.ErrorCode(err))
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

		err := svc.Unlink(context.Background(), "NOTFOUND-1", "TEST-2")

		require.Error(t, err)
		assert.Equal(t, jira4claude.ENotFound, jira4claude.ErrorCode(err))
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
