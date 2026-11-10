package http_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/fwojciec/jira4claude"
	jirahttp "github.com/fwojciec/jira4claude/http"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// jir-dqbz: a write Jira answered 2xx was applied, whatever the answer's
// body holds. When the client needs the body (the new issue's key, the new
// comment's ID) and cannot read it, the error says the write was applied
// (jira4claude.UnreadableAnswerError), so that nobody repeats it.

// answering serves every request with status and body, counting requests.
func answering(t *testing.T, status int, body string) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var n atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n.Add(1)
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)
	return server, &n
}

func TestIssueService_Create_AnswerUnreadable(t *testing.T) {
	t.Parallel()

	issue := &jira4claude.Issue{Project: "TEST", Type: "Task", Summary: "S"}

	for _, tc := range []struct {
		name string
		body string
	}{
		{"not JSON", `<html>created</html>`},
		{"no key", `{"id":"10001"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			server, posts := answering(t, http.StatusCreated, tc.body)
			svc := jirahttp.NewIssueService(newTestClient(t, server.URL, "user", "pass"))

			got, err := svc.Create(context.Background(), issue)

			require.Error(t, err)
			assert.Nil(t, got)
			var unreadable *jira4claude.UnreadableAnswerError
			require.ErrorAs(t, err, &unreadable)
			assert.Equal(t, jira4claude.EInternal, jira4claude.ErrorCode(err))
			assert.Contains(t, err.Error(), "the write was applied")
			assert.Equal(t, int32(1), posts.Load(), "not retried")
		})
	}

	t.Run("a body cut short after the 201 is an applied write, not retried", func(t *testing.T) {
		t.Parallel()
		var posts atomic.Int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			posts.Add(1)
			w.Header().Set("Content-Length", "100")
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"key":`))
			// Returning with fewer bytes than announced closes the
			// connection: the client's read fails.
		}))
		defer server.Close()
		svc := jirahttp.NewIssueService(newTestClient(t, server.URL, "user", "pass"))

		_, err := svc.Create(context.Background(), issue)

		var unreadable *jira4claude.UnreadableAnswerError
		require.ErrorAs(t, err, &unreadable)
		assert.NotContains(t, err.Error(), "may have been applied")
		assert.Equal(t, int32(1), posts.Load())
	})

	t.Run("another 2xx with a key is a success", func(t *testing.T) {
		t.Parallel()
		server, _ := answering(t, http.StatusOK, `{"id":"10001","key":"TEST-7"}`)
		svc := jirahttp.NewIssueService(newTestClient(t, server.URL, "user", "pass"))

		got, err := svc.Create(context.Background(), issue)

		require.NoError(t, err)
		assert.Equal(t, "TEST-7", got.Key)
	})

	t.Run("a refused create is not an applied write", func(t *testing.T) {
		t.Parallel()
		server, _ := answering(t, http.StatusBadRequest, `{"errors":{"summary":"required"}}`)
		svc := jirahttp.NewIssueService(newTestClient(t, server.URL, "user", "pass"))

		_, err := svc.Create(context.Background(), issue)

		var unreadable *jira4claude.UnreadableAnswerError
		assert.NotErrorAs(t, err, &unreadable)
		assert.Equal(t, jira4claude.EValidation, jira4claude.ErrorCode(err))
	})
}

func TestIssueService_AddComment_AnswerUnreadable(t *testing.T) {
	t.Parallel()

	body := &jira4claude.ADFNode{Type: "doc", Version: 1}

	for _, tc := range []struct {
		name string
		body string
	}{
		{"not JSON", `<html>ok</html>`},
		{"no id", `{"body":{"type":"doc","version":1}}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			server, posts := answering(t, http.StatusCreated, tc.body)
			svc := jirahttp.NewIssueService(newTestClient(t, server.URL, "user", "pass"))

			got, err := svc.AddComment(context.Background(), "TEST-1", body)

			require.Error(t, err)
			assert.Nil(t, got)
			var unreadable *jira4claude.UnreadableAnswerError
			require.ErrorAs(t, err, &unreadable)
			assert.Equal(t, jira4claude.EInternal, jira4claude.ErrorCode(err))
			assert.Equal(t, int32(1), posts.Load(), "not retried")
		})
	}
}

// The writes whose answer the client does not read succeed on any 2xx, and
// on a 2xx whose body cannot be read: nothing is unknown.
func TestIssueService_WritesWithoutAnswer_AnyTwoHundred(t *testing.T) {
	t.Parallel()

	writes := map[string]func(*jirahttp.IssueService) error{
		"link": func(s *jirahttp.IssueService) error {
			return s.Link(context.Background(), "TEST-1", "Blocks", "TEST-2")
		},
		"transition": func(s *jirahttp.IssueService) error {
			return s.Transition(context.Background(), "TEST-1", "11")
		},
		"assign": func(s *jirahttp.IssueService) error {
			return s.Assign(context.Background(), "TEST-1", "acc-1")
		},
		"delete comment": func(s *jirahttp.IssueService) error {
			return s.DeleteComment(context.Background(), "TEST-1", "10001")
		},
	}
	for name, write := range writes {
		t.Run(name+": 200 instead of the documented status", func(t *testing.T) {
			t.Parallel()
			server, n := answering(t, http.StatusOK, `{"ok":true}`)
			svc := jirahttp.NewIssueService(newTestClient(t, server.URL, "user", "pass"))

			require.NoError(t, write(svc))
			assert.Equal(t, int32(1), n.Load())
		})
		t.Run(name+": a 2xx whose body is cut short", func(t *testing.T) {
			t.Parallel()
			var n atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				n.Add(1)
				w.Header().Set("Content-Length", "100")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"ok":`))
			}))
			defer server.Close()
			svc := jirahttp.NewIssueService(newTestClient(t, server.URL, "user", "pass", jirahttp.WithMaxRetries(0)))

			require.NoError(t, write(svc))
		})
	}
}
