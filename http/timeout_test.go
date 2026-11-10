package http_test

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fwojciec/jira4claude"
	jirahttp "github.com/fwojciec/jira4claude/http"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The timeouts of WithTimeout, with a short limit and handlers that never
// answer or stall: every request class returns within the limit, a slow but
// moving download is not cut, and a timed-out write says it may have been
// applied.

const testTimeout = 150 * time.Millisecond

// returnsWithin is generous: the race detector and a parallel suite slow
// everything, and the point is "not forever", not the exact time.
const returnsWithin = 3 * time.Second

// hangingServer answers nothing: each handler reads the request body, then
// waits until the client goes away or the test ends. It counts requests.
func hangingServer(t *testing.T) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var count atomic.Int32
	done := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count.Add(1)
		_, _ = io.Copy(io.Discard, r.Body)
		select {
		case <-r.Context().Done():
		case <-done:
		}
	}))
	// Cleanups run last first: done closes before Close waits on handlers.
	t.Cleanup(server.Close)
	t.Cleanup(func() { close(done) })
	return server, &count
}

func timeoutClient(t *testing.T, url string) *jirahttp.Client {
	t.Helper()
	return newTestClient(t, url, "user@example.com", "api-token",
		jirahttp.WithTimeout(testTimeout), jirahttp.WithRetryBaseDelay(time.Millisecond))
}

func TestTimeout_EveryRequestClassReturns(t *testing.T) {
	t.Parallel()

	summary := "S"
	tests := []struct {
		name  string
		write bool
		call  func(ctx context.Context, c *jirahttp.Client) error
	}{
		{"GET an issue", false, func(ctx context.Context, c *jirahttp.Client) error {
			_, err := jirahttp.NewIssueService(c).Get(ctx, "TEST-1")
			return err
		}},
		{"GET a search page", false, func(ctx context.Context, c *jirahttp.Client) error {
			_, _, err := jirahttp.NewIssueService(c).List(ctx, jira4claude.IssueFilter{Project: "TEST"})
			return err
		}},
		{"POST a comment", true, func(ctx context.Context, c *jirahttp.Client) error {
			_, err := jirahttp.NewIssueService(c).AddComment(ctx, "TEST-1", &jira4claude.ADFNode{Type: "doc", Version: 1})
			return err
		}},
		{"PUT an update", true, func(ctx context.Context, c *jirahttp.Client) error {
			_, err := jirahttp.NewIssueService(c).Update(ctx, "TEST-1", jira4claude.IssueUpdate{Summary: &summary})
			return err
		}},
		{"DELETE an attachment", true, func(ctx context.Context, c *jirahttp.Client) error {
			return jirahttp.NewAttachmentService(c).Delete(ctx, "10000")
		}},
		{"download", false, func(ctx context.Context, c *jirahttp.Client) error {
			_, err := jirahttp.NewAttachmentService(c).Download(ctx, "10000", io.Discard)
			return err
		}},
		{"upload", true, func(ctx context.Context, c *jirahttp.Client) error {
			_, err := jirahttp.NewAttachmentService(c).Upload(ctx, "TEST-1", jira4claude.AttachmentFile{
				Name: "a.txt", Size: 5,
				Open: func() (io.ReadCloser, error) { return io.NopCloser(strings.NewReader("hello")), nil },
			})
			return err
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			server, count := hangingServer(t)
			client := timeoutClient(t, server.URL)

			start := time.Now()
			err := tt.call(context.Background(), client)
			elapsed := time.Since(start)

			require.Error(t, err)
			assert.Less(t, elapsed, returnsWithin)
			assert.GreaterOrEqual(t, elapsed, testTimeout)
			assert.Equal(t, jira4claude.EInternal, jira4claude.ErrorCode(err))
			assert.Contains(t, err.Error(), "timed out")
			assert.True(t, jira4claude.IsTimeout(err), "marked as a timeout")
			assert.Contains(t, err.Error(), "--timeout")
			assert.Equal(t, int32(1), count.Load(), "a timed-out request is not retried")
			if tt.write {
				assert.Contains(t, err.Error(), "may have been applied")
			} else {
				assert.NotContains(t, err.Error(), "may have been applied")
			}
		})
	}
}

func TestTimeout_APICallIsBoundedAsAWhole(t *testing.T) {
	t.Parallel()

	// An API call's body that keeps trickling past the limit is cut: the
	// limit covers the whole answer, unlike a transfer's.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		for i := 0; i < 40; i++ {
			if _, err := w.Write([]byte(" ")); err != nil {
				return
			}
			w.(http.Flusher).Flush()
			select {
			case <-time.After(testTimeout / 5):
			case <-r.Context().Done():
				return
			}
		}
	}))
	defer server.Close()

	_, err := jirahttp.NewIssueService(timeoutClient(t, server.URL)).Get(context.Background(), "TEST-1")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "no answer from Jira within 150ms")
}

func TestTimeout_Download(t *testing.T) {
	t.Parallel()

	t.Run("a slow download that keeps moving is not cut", func(t *testing.T) {
		t.Parallel()
		// A limit far above the pause between chunks: this test must pass,
		// so a loaded machine that delays one chunk must not cut it.
		const limit = time.Second
		const redirectChunks = 25 // the redirect's body alone outlasts the limit
		const chunks = 15
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// The redirect's body trickles too: draining it is progress.
			count := chunks
			if r.URL.Path != "/blob" {
				count = redirectChunks
				w.Header().Set("Location", "/blob")
				w.WriteHeader(http.StatusSeeOther)
			}
			for i := 0; i < count; i++ {
				if _, err := w.Write([]byte("x")); err != nil {
					return
				}
				w.(http.Flusher).Flush()
				time.Sleep(limit / 20)
			}
		}))
		defer server.Close()
		client := newTestClient(t, server.URL, "user@example.com", "api-token",
			jirahttp.WithTimeout(limit), jirahttp.WithRetryBaseDelay(time.Millisecond))

		var buf bytes.Buffer
		start := time.Now()
		n, err := jirahttp.NewAttachmentService(client).Download(context.Background(), "10000", &buf)

		require.NoError(t, err)
		assert.Equal(t, int64(chunks), n)
		assert.Greater(t, time.Since(start), limit, "the download outlasted the limit")
	})

	t.Run("a download that stalls mid-body fails without a retry", func(t *testing.T) {
		t.Parallel()
		var count atomic.Int32
		done := make(chan struct{})
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			count.Add(1)
			w.Header().Set("Content-Length", "100")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("half"))
			w.(http.Flusher).Flush()
			select {
			case <-r.Context().Done():
			case <-done:
			}
		}))
		t.Cleanup(server.Close)
		t.Cleanup(func() { close(done) })

		var buf bytes.Buffer
		start := time.Now()
		n, err := jirahttp.NewAttachmentService(timeoutClient(t, server.URL)).Download(context.Background(), "10000", &buf)

		require.Error(t, err)
		assert.Less(t, time.Since(start), returnsWithin)
		assert.Equal(t, int64(4), n)
		assert.Equal(t, jira4claude.EInternal, jira4claude.ErrorCode(err))
		assert.Equal(t, "download timed out: nothing arrived for 150ms after 4 bytes (--timeout sets the limit)", err.Error())
		assert.Equal(t, int32(1), count.Load())
	})

	t.Run("a redirect hop that never answers fails within the limit", func(t *testing.T) {
		t.Parallel()
		media, _ := hangingServer(t)
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, media.URL+"/blob?sig=secret", http.StatusSeeOther)
		}))
		defer server.Close()

		start := time.Now()
		_, err := jirahttp.NewAttachmentService(timeoutClient(t, server.URL)).Download(context.Background(), "10000", io.Discard)

		require.Error(t, err)
		assert.Less(t, time.Since(start), returnsWithin)
		assert.Contains(t, err.Error(), "download timed out")
		assert.NotContains(t, err.Error(), "secret")
	})
}

func TestTimeout_Zero(t *testing.T) {
	t.Parallel()

	// No limit: an answer slower than testTimeout still arrives.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * testTimeout)
		_, _ = w.Write([]byte(`{"key":"TEST-1","fields":{}}`))
	}))
	defer server.Close()
	client := newTestClient(t, server.URL, "user@example.com", "api-token", jirahttp.WithTimeout(0))

	issue, err := jirahttp.NewIssueService(client).Get(context.Background(), "TEST-1")

	require.NoError(t, err)
	assert.Equal(t, "TEST-1", issue.Key)
}
