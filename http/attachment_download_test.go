package http_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strconv"
	"sync/atomic"
	"testing"

	"github.com/fwojciec/jira4claude"
	jirahttp "github.com/fwojciec/jira4claude/http"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mediaServer stands in for the media host a Jira download redirects to. It
// records whether any request carried credentials or cookies.
type mediaServer struct {
	*httptest.Server
	requests   atomic.Int32
	sawAuth    atomic.Bool
	sawCookie  atomic.Bool
	lastAccept atomic.Value
}

func newMediaServer(t *testing.T, body string) *mediaServer {
	t.Helper()
	ms := &mediaServer{}
	ms.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ms.requests.Add(1)
		if r.Header.Get("Authorization") != "" {
			ms.sawAuth.Store(true)
		}
		if r.Header.Get("Cookie") != "" {
			ms.sawCookie.Store(true)
		}
		ms.lastAccept.Store(r.Header.Get("Accept"))
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(ms.Close)
	return ms
}

// redirectingJira answers every request with a 303 to location.
func redirectingJira(t *testing.T, location func() string, onRequest func(r *http.Request)) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if onRequest != nil {
			onRequest(r)
		}
		http.SetCookie(w, &http.Cookie{Name: "session", Value: "jira-session", Path: "/"})
		w.Header().Set("Location", location())
		w.WriteHeader(http.StatusSeeOther)
	}))
	t.Cleanup(server.Close)
	return server
}

func TestAttachmentService_Download(t *testing.T) {
	t.Parallel()

	t.Run("streams the content endpoint of the configured host with credentials", func(t *testing.T) {
		t.Parallel()

		var gotPath, gotRedirect, gotAccept, gotMethod string
		var gotUser, gotPass string
		var gotOK bool
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotMethod = r.Method
			gotPath = r.URL.Path
			gotRedirect = r.URL.Query().Get("redirect")
			gotAccept = r.Header.Get("Accept")
			gotUser, gotPass, gotOK = r.BasicAuth()
			_, _ = w.Write([]byte("file bytes"))
		}))
		defer server.Close()
		client := newTestClient(t, server.URL, "user", "pass")
		svc := jirahttp.NewAttachmentService(client)

		var buf bytes.Buffer
		n, err := svc.Download(context.Background(), "10000", &buf)

		require.NoError(t, err)
		assert.Equal(t, int64(10), n)
		assert.Equal(t, "file bytes", buf.String())
		assert.Equal(t, http.MethodGet, gotMethod)
		assert.Equal(t, "/rest/api/3/attachment/content/10000", gotPath)
		assert.Equal(t, "false", gotRedirect)
		assert.NotEqual(t, "application/json", gotAccept)
		assert.True(t, gotOK)
		assert.Equal(t, "user", gotUser)
		assert.Equal(t, "pass", gotPass)
	})

	t.Run("an id that is not a plain token is refused without a request", func(t *testing.T) {
		t.Parallel()

		var requests atomic.Int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requests.Add(1)
		}))
		defer server.Close()
		svc := jirahttp.NewAttachmentService(newTestClient(t, server.URL, "user", "pass"))

		for _, id := range []string{"", "..", "../issue/TEST-1", "1/2", "1?x=y", "1#x", "1%2F"} {
			_, err := svc.Download(context.Background(), id, &bytes.Buffer{})

			require.Error(t, err, "id %q", id)
			assert.Equal(t, jira4claude.EValidation, jira4claude.ErrorCode(err), "id %q", id)
		}
		assert.Zero(t, requests.Load())
	})

	t.Run("a cross-host redirect is followed without credentials or cookies", func(t *testing.T) {
		t.Parallel()

		media := newMediaServer(t, "media bytes")
		var jiraSawAuth atomic.Bool
		jira := redirectingJira(t, func() string { return media.URL + "/file/abc?token=signed" }, func(r *http.Request) {
			_, _, ok := r.BasicAuth()
			jiraSawAuth.Store(ok)
		})

		// The jar holds a cookie for 127.0.0.1, which both test servers
		// share: cookies ignore ports, so only refusing them protects the
		// media host.
		jar, err := cookiejar.New(nil)
		require.NoError(t, err)
		jiraURL, err := url.Parse(jira.URL)
		require.NoError(t, err)
		jar.SetCookies(jiraURL, []*http.Cookie{{Name: "jar", Value: "secret", Path: "/"}})
		client := newTestClient(t, jira.URL, "user", "pass", jirahttp.WithHTTPClient(&http.Client{Jar: jar}))
		svc := jirahttp.NewAttachmentService(client)

		var buf bytes.Buffer
		n, err := svc.Download(context.Background(), "10000", &buf)

		require.NoError(t, err)
		assert.Equal(t, "media bytes", buf.String())
		assert.Equal(t, int64(11), n)
		assert.True(t, jiraSawAuth.Load(), "the configured host gets credentials")
		assert.Equal(t, int32(1), media.requests.Load())
		assert.False(t, media.sawAuth.Load(), "the media host must not get the Authorization header")
		assert.False(t, media.sawCookie.Load(), "the media host must not get cookies")
	})

	t.Run("a redirect within the configured host keeps credentials", func(t *testing.T) {
		t.Parallel()

		var authed atomic.Int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if _, _, ok := r.BasicAuth(); ok {
				authed.Add(1)
			}
			if r.URL.Path != "/final" {
				w.Header().Set("Location", "/final")
				w.WriteHeader(http.StatusFound)
				return
			}
			_, _ = w.Write([]byte("ok"))
		}))
		defer server.Close()
		svc := jirahttp.NewAttachmentService(newTestClient(t, server.URL, "user", "pass"))

		var buf bytes.Buffer
		_, err := svc.Download(context.Background(), "10000", &buf)

		require.NoError(t, err)
		assert.Equal(t, "ok", buf.String())
		assert.Equal(t, int32(2), authed.Load())
	})

	t.Run("credentials stay off after a hop leaves the configured host", func(t *testing.T) {
		t.Parallel()

		// Jira redirects to another host, which redirects back to a Jira
		// URL of its choosing.
		var finalAuth atomic.Bool
		var finalHits atomic.Int32
		var bouncerURL string
		jira := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/back" {
				finalHits.Add(1)
				_, _, ok := r.BasicAuth()
				finalAuth.Store(ok)
				_, _ = w.Write([]byte("back"))
				return
			}
			w.Header().Set("Location", bouncerURL+"/x")
			w.WriteHeader(http.StatusSeeOther)
		}))
		defer jira.Close()
		bouncer := redirectingJira(t, func() string { return jira.URL + "/back" }, nil)
		bouncerURL = bouncer.URL
		svc := jirahttp.NewAttachmentService(newTestClient(t, jira.URL, "user", "pass"))

		var buf bytes.Buffer
		_, err := svc.Download(context.Background(), "10000", &buf)

		require.NoError(t, err)
		assert.Equal(t, int32(1), finalHits.Load())
		assert.False(t, finalAuth.Load(), "a URL reached through another host gets no credentials")
	})

	t.Run("a redirect from HTTPS to plain HTTP is refused", func(t *testing.T) {
		t.Parallel()

		media := newMediaServer(t, "media bytes")
		jira := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Location", media.URL+"/file")
			w.WriteHeader(http.StatusSeeOther)
		}))
		defer jira.Close()
		client := newTestClient(t, jira.URL, "user", "pass", jirahttp.WithHTTPClient(jira.Client()))
		svc := jirahttp.NewAttachmentService(client)

		var buf bytes.Buffer
		_, err := svc.Download(context.Background(), "10000", &buf)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "non-HTTPS")
		assert.NotContains(t, err.Error(), "token", "signed redirect URLs are kept out of messages")
		assert.Zero(t, media.requests.Load())
		assert.Empty(t, buf.String())
	})

	t.Run("a redirect to a non-HTTP scheme is refused", func(t *testing.T) {
		t.Parallel()

		jira := redirectingJira(t, func() string { return "file:///etc/passwd" }, nil)
		svc := jirahttp.NewAttachmentService(newTestClient(t, jira.URL, "user", "pass"))

		_, err := svc.Download(context.Background(), "10000", &bytes.Buffer{})

		require.Error(t, err)
	})

	t.Run("redirects are capped", func(t *testing.T) {
		t.Parallel()

		var requests atomic.Int32
		var self string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			n := requests.Add(1)
			w.Header().Set("Location", self+"/loop/"+strconv.Itoa(int(n)))
			w.WriteHeader(http.StatusFound)
		}))
		defer server.Close()
		self = server.URL
		svc := jirahttp.NewAttachmentService(newTestClient(t, server.URL, "user", "pass", jirahttp.WithMaxRetries(3)))

		_, err := svc.Download(context.Background(), "10000", &bytes.Buffer{})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "too many redirects")
		assert.Equal(t, int32(6), requests.Load(), "the first request and 5 redirects, not retried")
	})

	t.Run("a 404 is a not-found error with nothing written", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"errorMessages":["The attachment does not exist."]}`))
		}))
		defer server.Close()
		svc := jirahttp.NewAttachmentService(newTestClient(t, server.URL, "user", "pass"))

		var buf bytes.Buffer
		_, err := svc.Download(context.Background(), "10000", &buf)

		require.Error(t, err)
		assert.Equal(t, jira4claude.ENotFound, jira4claude.ErrorCode(err))
		assert.Contains(t, err.Error(), "The attachment does not exist.")
		assert.Empty(t, buf.String())
	})

	t.Run("a 5xx before any byte is retried", func(t *testing.T) {
		t.Parallel()

		var requests atomic.Int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if requests.Add(1) == 1 {
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			_, _ = w.Write([]byte("ok"))
		}))
		defer server.Close()
		svc := jirahttp.NewAttachmentService(newTestClient(t, server.URL, "user", "pass", jirahttp.WithRetryBaseDelay(0)))

		var buf bytes.Buffer
		_, err := svc.Download(context.Background(), "10000", &buf)

		require.NoError(t, err)
		assert.Equal(t, "ok", buf.String())
		assert.Equal(t, int32(2), requests.Load())
	})

	t.Run("a body cut short after bytes were written is not retried", func(t *testing.T) {
		t.Parallel()

		var requests atomic.Int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requests.Add(1)
			w.Header().Set("Content-Length", "100")
			_, _ = w.Write([]byte("partial"))
			// Returning with fewer bytes than announced ends the connection.
		}))
		defer server.Close()
		svc := jirahttp.NewAttachmentService(newTestClient(t, server.URL, "user", "pass", jirahttp.WithRetryBaseDelay(0)))

		var buf bytes.Buffer
		n, err := svc.Download(context.Background(), "10000", &buf)

		require.Error(t, err)
		assert.Equal(t, int32(1), requests.Load())
		assert.Equal(t, int64(len("partial")), n)
		assert.Equal(t, "partial", buf.String())
	})
}
