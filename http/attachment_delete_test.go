package http_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/fwojciec/jira4claude"
	jirahttp "github.com/fwojciec/jira4claude/http"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// attachmentMetadataJSON is the getAttachment response example of
// Atlassian's REST v3 OpenAPI document: a numeric id and Jira's timestamp
// form.
const attachmentMetadataJSON = `{
	"author": {"accountId": "5b10a2844c20165700ede21g", "displayName": "Mia Krystof"},
	"content": "https://your-domain.atlassian.net/jira/rest/api/3/attachment/content/10000",
	"created": "2022-10-06T07:32:47.000+0000",
	"filename": "picture.jpg",
	"id": 10000,
	"mimeType": "image/jpeg",
	"self": "https://your-domain.atlassian.net/rest/api/3/attachments/10000",
	"size": 23123
}`

// statusServer answers each request with the next of statuses (the last one
// repeats) and body, and records each request's method and path.
type statusServer struct {
	*httptest.Server
	mu       sync.Mutex
	requests []string
}

func newStatusServer(t *testing.T, body string, statuses ...int) *statusServer {
	t.Helper()
	ss := &statusServer{}
	ss.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ss.mu.Lock()
		n := len(ss.requests)
		ss.requests = append(ss.requests, r.Method+" "+r.URL.Path)
		ss.mu.Unlock()
		status := statuses[min(n, len(statuses)-1)]
		w.WriteHeader(status)
		switch {
		case status == http.StatusOK:
			_, _ = w.Write([]byte(body))
		case status >= 400:
			_, _ = w.Write([]byte(`{"errorMessages":["` + http.StatusText(status) + `"]}`))
		}
	}))
	t.Cleanup(ss.Close)
	return ss
}

func (ss *statusServer) seen() []string {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	return append([]string(nil), ss.requests...)
}

func TestAttachmentService_Get(t *testing.T) {
	t.Parallel()

	t.Run("reads the attachment's metadata", func(t *testing.T) {
		t.Parallel()

		server := newStatusServer(t, attachmentMetadataJSON, http.StatusOK)
		svc := jirahttp.NewAttachmentService(newTestClient(t, server.URL, "user", "pass"))

		got, err := svc.Get(context.Background(), "10000")

		require.NoError(t, err)
		assert.Equal(t, []string{"GET /rest/api/3/attachment/10000"}, server.seen())
		assert.Equal(t, "10000", got.ID)
		assert.Equal(t, "picture.jpg", got.Filename)
		assert.Equal(t, "image/jpeg", got.MimeType)
		assert.Equal(t, int64(23123), got.Size)
		require.NotNil(t, got.Author)
		assert.Equal(t, "Mia Krystof", got.Author.DisplayName)
		assert.True(t, time.Date(2022, 10, 6, 7, 32, 47, 0, time.UTC).Equal(got.Created), "created: %v", got.Created)
	})

	t.Run("a missing attachment is not found", func(t *testing.T) {
		t.Parallel()

		server := newStatusServer(t, "", http.StatusNotFound)
		svc := jirahttp.NewAttachmentService(newTestClient(t, server.URL, "user", "pass"))

		_, err := svc.Get(context.Background(), "10000")

		assert.Equal(t, jira4claude.ENotFound, jira4claude.ErrorCode(err))
	})

	t.Run("an ID that is not a plain token is refused without a request", func(t *testing.T) {
		t.Parallel()

		server := newStatusServer(t, attachmentMetadataJSON, http.StatusOK)
		svc := jirahttp.NewAttachmentService(newTestClient(t, server.URL, "user", "pass"))

		for _, id := range []string{"", "../10000", "10000?x=1", "1 0", "10000/"} {
			_, err := svc.Get(context.Background(), id)
			assert.Equal(t, jira4claude.EValidation, jira4claude.ErrorCode(err), "id %q", id)
		}
		assert.Empty(t, server.seen())
	})
}

func TestAttachmentService_Delete(t *testing.T) {
	t.Parallel()

	t.Run("deletes the attachment, answered with 204", func(t *testing.T) {
		t.Parallel()

		server := newStatusServer(t, "", http.StatusNoContent)
		svc := jirahttp.NewAttachmentService(newTestClient(t, server.URL, "user", "pass"))

		err := svc.Delete(context.Background(), "10000")

		require.NoError(t, err)
		assert.Equal(t, []string{"DELETE /rest/api/3/attachment/10000"}, server.seen())
	})

	for _, tc := range []struct {
		status int
		code   string
	}{
		{http.StatusNotFound, jira4claude.ENotFound},
		{http.StatusForbidden, jira4claude.EForbidden},
	} {
		t.Run(http.StatusText(tc.status)+" is returned with its code and not retried", func(t *testing.T) {
			t.Parallel()

			server := newStatusServer(t, "", tc.status, http.StatusNoContent)
			svc := jirahttp.NewAttachmentService(newTestClient(t, server.URL, "user", "pass", jirahttp.WithRetryBaseDelay(0)))

			err := svc.Delete(context.Background(), "10000")

			require.Error(t, err)
			assert.Equal(t, tc.code, jira4claude.ErrorCode(err))
			assert.Equal(t, http.StatusText(tc.status), err.Error())
			assert.Len(t, server.seen(), 1)
		})
	}

	// DELETE is idempotent, so the existing policy (design 1.5) retries it on
	// 429, 5xx and connection errors.
	for _, status := range []int{http.StatusTooManyRequests, http.StatusInternalServerError, http.StatusServiceUnavailable} {
		t.Run(http.StatusText(status)+" is retried as any DELETE is", func(t *testing.T) {
			t.Parallel()

			server := newStatusServer(t, "", status, http.StatusNoContent)
			svc := jirahttp.NewAttachmentService(newTestClient(t, server.URL, "user", "pass", jirahttp.WithRetryBaseDelay(0)))

			err := svc.Delete(context.Background(), "10000")

			require.NoError(t, err)
			assert.Len(t, server.seen(), 2)
		})
	}

	t.Run("an ID that is not a plain token is refused without a request", func(t *testing.T) {
		t.Parallel()

		server := newStatusServer(t, "", http.StatusNoContent)
		svc := jirahttp.NewAttachmentService(newTestClient(t, server.URL, "user", "pass"))

		for _, id := range []string{"", "../10000", "10000?x=1", "1 0", "%2e%2e"} {
			err := svc.Delete(context.Background(), id)
			assert.Equal(t, jira4claude.EValidation, jira4claude.ErrorCode(err), "id %q", id)
		}
		assert.Empty(t, server.seen())
	})
}
