package http_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fwojciec/jira4claude"
	jirahttp "github.com/fwojciec/jira4claude/http"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// attachmentFieldJSON is an issue's attachment field in the shape of the
// getIssue response example in Atlassian's REST v3 OpenAPI document: a
// numeric id and Jira's own timestamp form. The second entry has a string id,
// the shape of the Attachment schema.
const attachmentFieldJSON = `[
	{
		"self": "https://example.atlassian.net/rest/api/3/attachments/10000",
		"id": 10000,
		"filename": "picture.jpg",
		"author": {
			"accountId": "5b10a2844c20165700ede21g",
			"displayName": "Mia Krystof",
			"emailAddress": "mia@example.com"
		},
		"created": "2022-10-06T07:32:47.000+0000",
		"size": 23123,
		"mimeType": "image/jpeg",
		"content": "https://example.atlassian.net/rest/api/3/attachment/content/10000",
		"thumbnail": "https://example.atlassian.net/rest/api/3/attachment/thumbnail/10000"
	},
	{
		"id": "10001",
		"filename": "debug log.txt",
		"created": "2022-10-07T08:00:00.000+0000",
		"size": 2460,
		"mimeType": "text/plain",
		"content": "https://example.atlassian.net/rest/api/3/attachment/content/10001"
	}
]`

func assertParsedAttachments(t *testing.T, got []*jira4claude.Attachment) {
	t.Helper()
	require.Len(t, got, 2)

	assert.Equal(t, "10000", got[0].ID)
	assert.Equal(t, "picture.jpg", got[0].Filename)
	assert.Equal(t, "image/jpeg", got[0].MimeType)
	assert.Equal(t, int64(23123), got[0].Size)
	require.NotNil(t, got[0].Author)
	assert.Equal(t, "Mia Krystof", got[0].Author.DisplayName)
	assert.Equal(t, "5b10a2844c20165700ede21g", got[0].Author.AccountID)
	assert.True(t, time.Date(2022, 10, 6, 7, 32, 47, 0, time.UTC).Equal(got[0].Created), "created: %v", got[0].Created)

	assert.Equal(t, "10001", got[1].ID)
	assert.Equal(t, "debug log.txt", got[1].Filename)
	assert.Equal(t, "text/plain", got[1].MimeType)
	assert.Equal(t, int64(2460), got[1].Size)
	assert.Nil(t, got[1].Author)
}

func TestAttachmentService_List(t *testing.T) {
	t.Parallel()

	t.Run("reads the issue's attachment field only", func(t *testing.T) {
		t.Parallel()

		var gotFields, gotMethod, gotPath string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotMethod = r.Method
			gotPath = r.URL.Path
			gotFields = r.URL.Query().Get("fields")
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"key":"TEST-1","fields":{"attachment":` + attachmentFieldJSON + `}}`))
		}))
		defer server.Close()

		svc := jirahttp.NewAttachmentService(newTestClient(t, server.URL, "user@example.com", "api-token"))

		got, err := svc.List(context.Background(), "TEST-1")

		require.NoError(t, err)
		assert.Equal(t, http.MethodGet, gotMethod)
		assert.Equal(t, "/rest/api/3/issue/TEST-1", gotPath)
		assert.Equal(t, "attachment", gotFields)
		assertParsedAttachments(t, got)
	})

	t.Run("escapes the key", func(t *testing.T) {
		t.Parallel()

		var rawPath string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rawPath = r.URL.EscapedPath()
			_, _ = w.Write([]byte(`{"fields":{}}`))
		}))
		defer server.Close()

		svc := jirahttp.NewAttachmentService(newTestClient(t, server.URL, "user@example.com", "api-token"))

		_, err := svc.List(context.Background(), "TEST/1")

		require.NoError(t, err)
		assert.Equal(t, "/rest/api/3/issue/TEST%2F1", rawPath)
	})

	t.Run("an issue without attachments gives an empty list", func(t *testing.T) {
		t.Parallel()

		for _, body := range []string{`{"fields":{"attachment":[]}}`, `{"fields":{}}`, `{"fields":{"attachment":null}}`} {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte(body))
			}))

			svc := jirahttp.NewAttachmentService(newTestClient(t, server.URL, "user@example.com", "api-token"))
			got, err := svc.List(context.Background(), "TEST-1")
			server.Close()

			require.NoError(t, err, "body %s", body)
			assert.Empty(t, got, "body %s", body)
		}
	})

	t.Run("an unparseable created time is left zero, not an error", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"fields":{"attachment":[{"id":"1","filename":"a","created":"yesterday","size":1}]}}`))
		}))
		defer server.Close()

		svc := jirahttp.NewAttachmentService(newTestClient(t, server.URL, "user@example.com", "api-token"))

		got, err := svc.List(context.Background(), "TEST-1")

		require.NoError(t, err)
		require.Len(t, got, 1)
		assert.True(t, got[0].Created.IsZero())
	})

	t.Run("a created time in epoch milliseconds is parsed", func(t *testing.T) {
		t.Parallel()

		// The addAttachment response example in the same OpenAPI document
		// gives created as a number of milliseconds since the epoch.
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"fields":{"attachment":[{"id":"1","filename":"a","created":1651316514000,"size":1}]}}`))
		}))
		defer server.Close()

		svc := jirahttp.NewAttachmentService(newTestClient(t, server.URL, "user@example.com", "api-token"))

		got, err := svc.List(context.Background(), "TEST-1")

		require.NoError(t, err)
		require.Len(t, got, 1)
		assert.True(t, time.UnixMilli(1651316514000).Equal(got[0].Created), "created: %v", got[0].Created)
	})

	t.Run("an id that is neither string nor number is an internal error", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"fields":{"attachment":[{"id":{"x":1},"filename":"a"}]}}`))
		}))
		defer server.Close()

		svc := jirahttp.NewAttachmentService(newTestClient(t, server.URL, "user@example.com", "api-token"))

		_, err := svc.List(context.Background(), "TEST-1")

		require.Error(t, err)
		assert.Equal(t, jira4claude.EInternal, jira4claude.ErrorCode(err))
	})

	t.Run("maps a missing issue to not found", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"errorMessages":["Issue does not exist or you do not have permission to see it."]}`))
		}))
		defer server.Close()

		svc := jirahttp.NewAttachmentService(newTestClient(t, server.URL, "user@example.com", "api-token"))

		_, err := svc.List(context.Background(), "TEST-404")

		require.Error(t, err)
		assert.Equal(t, jira4claude.ENotFound, jira4claude.ErrorCode(err))
		assert.Contains(t, jira4claude.ErrorMessage(err), "Issue does not exist")
	})

	t.Run("retries a 5xx, as a GET", func(t *testing.T) {
		t.Parallel()

		var calls atomic.Int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if calls.Add(1) == 1 {
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			_, _ = w.Write([]byte(`{"fields":{"attachment":[{"id":"1","filename":"a","size":1}]}}`))
		}))
		defer server.Close()

		client := newTestClient(t, server.URL, "user@example.com", "api-token", jirahttp.WithRetryBaseDelay(time.Millisecond))
		svc := jirahttp.NewAttachmentService(client)

		got, err := svc.List(context.Background(), "TEST-1")

		require.NoError(t, err)
		assert.Len(t, got, 1)
		assert.Equal(t, int32(2), calls.Load())
	})
}

func TestIssueService_Get_Attachments(t *testing.T) {
	t.Parallel()

	t.Run("parses the attachment field from the same response", func(t *testing.T) {
		t.Parallel()

		var calls atomic.Int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			calls.Add(1)
			_, _ = w.Write([]byte(`{
				"key": "TEST-1",
				"fields": {
					"summary": "With files",
					"status": {"name": "To Do"},
					"issuetype": {"name": "Task"},
					"attachment": ` + attachmentFieldJSON + `
				}
			}`))
		}))
		defer server.Close()

		svc := jirahttp.NewIssueService(newTestClient(t, server.URL, "user@example.com", "api-token"))

		issue, err := svc.Get(context.Background(), "TEST-1")

		require.NoError(t, err)
		assert.Equal(t, "With files", issue.Summary)
		assertParsedAttachments(t, issue.Attachments)
		assert.Equal(t, int32(1), calls.Load(), "one request serves the issue and its attachments")
	})

	t.Run("an attachment id that is neither string nor number fails the read", func(t *testing.T) {
		t.Parallel()

		// Deliberate: an attachment the tool cannot identify is not shown with
		// an empty ID, which download and delete could not use.
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"key":"TEST-1","fields":{"attachment":[{"id":true,"filename":"a"}]}}`))
		}))
		defer server.Close()

		svc := jirahttp.NewIssueService(newTestClient(t, server.URL, "user@example.com", "api-token"))

		_, err := svc.Get(context.Background(), "TEST-1")

		require.Error(t, err)
		assert.Equal(t, jira4claude.EInternal, jira4claude.ErrorCode(err))
	})

	t.Run("an issue without the field has no attachments", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"key":"TEST-1","fields":{"summary":"Plain"}}`))
		}))
		defer server.Close()

		svc := jirahttp.NewIssueService(newTestClient(t, server.URL, "user@example.com", "api-token"))

		issue, err := svc.Get(context.Background(), "TEST-1")

		require.NoError(t, err)
		assert.Empty(t, issue.Attachments)
	})
}
