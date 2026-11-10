package http_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/fwojciec/jira4claude"
	jirahttp "github.com/fwojciec/jira4claude/http"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeNetrc writes content to a netrc file of the test's own and returns
// its path.
func writeNetrc(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "netrc")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	return path
}

// authServer answers 200 with an empty JSON object and records the Basic
// credentials of the last request.
func authServer(t *testing.T) (*httptest.Server, func() (string, string, bool)) {
	t.Helper()
	got := make(chan [2]string, 16)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if ok {
			got <- [2]string{user, pass}
		}
		_, _ = w.Write([]byte(`{}`))
	}))
	t.Cleanup(server.Close)
	return server, func() (string, string, bool) {
		select {
		case c := <-got:
			return c[0], c[1], true
		default:
			return "", "", false
		}
	}
}

func getOnce(t *testing.T, client *jirahttp.Client) {
	t.Helper()
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "/rest/api/3/myself", nil)
	require.NoError(t, err)
	_, err = client.DoRequest(req, http.StatusOK)
	require.NoError(t, err)
}

// jir-6ov5: with a port in the server URL, the netrc entry is looked up as
// host:port, then as the bare host name, as curl and git match it.
func TestNewClient_NetrcWithPort(t *testing.T) {
	t.Parallel()

	t.Run("a bare host name entry serves a server URL with a port", func(t *testing.T) {
		t.Parallel()
		server, creds := authServer(t)
		u, err := url.Parse(server.URL)
		require.NoError(t, err)
		require.NotEmpty(t, u.Port())
		netrc := writeNetrc(t, "machine "+u.Hostname()+" login bare@example.com password bare-token\n")

		client, err := jirahttp.NewClient(server.URL, jirahttp.WithNetrcPath(netrc),
			jirahttp.WithHTTPClient(&http.Client{Transport: privateTransport(t)}))
		require.NoError(t, err)
		getOnce(t, client)

		user, pass, ok := creds()
		require.True(t, ok, "the request carried no credentials")
		assert.Equal(t, "bare@example.com", user)
		assert.Equal(t, "bare-token", pass)
	})

	t.Run("a host:port entry wins over the bare host name, wherever it is in the file", func(t *testing.T) {
		t.Parallel()
		server, creds := authServer(t)
		u, err := url.Parse(server.URL)
		require.NoError(t, err)
		netrc := writeNetrc(t,
			"machine "+u.Hostname()+" login bare@example.com password bare-token\n"+
				"machine "+u.Host+" login port@example.com password port-token\n")

		client, err := jirahttp.NewClient(server.URL, jirahttp.WithNetrcPath(netrc),
			jirahttp.WithHTTPClient(&http.Client{Transport: privateTransport(t)}))
		require.NoError(t, err)
		getOnce(t, client)

		user, pass, ok := creds()
		require.True(t, ok, "the request carried no credentials")
		assert.Equal(t, "port@example.com", user)
		assert.Equal(t, "port-token", pass)
	})

	t.Run("an entry for another port is not used", func(t *testing.T) {
		t.Parallel()
		netrc := writeNetrc(t, "machine jira.example.com:9443 login a password b\n")

		_, err := jirahttp.NewClient("https://jira.example.com:8443", jirahttp.WithNetrcPath(netrc))

		require.Error(t, err)
		assert.Equal(t, jira4claude.EUnauthorized, jira4claude.ErrorCode(err))
		assert.Equal(t, "no netrc entry for jira.example.com:8443 or jira.example.com in "+netrc, err.Error())
	})

	t.Run("without a port the message names the host alone", func(t *testing.T) {
		t.Parallel()
		netrc := writeNetrc(t, "machine other.example.com login a password b\n")

		_, err := jirahttp.NewClient("https://jira.example.com", jirahttp.WithNetrcPath(netrc))

		require.Error(t, err)
		assert.Equal(t, "no netrc entry for jira.example.com in "+netrc, err.Error())
	})

	t.Run("a missing password names the entry that matched", func(t *testing.T) {
		t.Parallel()
		netrc := writeNetrc(t, "machine jira.example.com login a\n")

		_, err := jirahttp.NewClient("https://jira.example.com:8443", jirahttp.WithNetrcPath(netrc))

		require.Error(t, err)
		assert.Equal(t, jira4claude.EUnauthorized, jira4claude.ErrorCode(err))
		assert.Equal(t, "netrc entry for jira.example.com: password is required", err.Error())
	})

	t.Run("credentials from a bare host name entry are not sent to another port of that host", func(t *testing.T) {
		t.Parallel()
		// The media server is on 127.0.0.1 too, on another port: the bare
		// entry matches its host name, but the redirect leaves the
		// configured origin, so it gets no credentials.
		media := newMediaServer(t, "file bytes")
		jira := redirectingJira(t, func() string { return media.URL + "/blob?sig=s" }, nil)
		u, err := url.Parse(jira.URL)
		require.NoError(t, err)
		mu, err := url.Parse(media.URL)
		require.NoError(t, err)
		require.Equal(t, u.Hostname(), mu.Hostname())
		require.NotEqual(t, u.Port(), mu.Port())
		netrc := writeNetrc(t, "machine "+u.Hostname()+" login bare@example.com password bare-token\n")
		client, err := jirahttp.NewClient(jira.URL, jirahttp.WithNetrcPath(netrc),
			jirahttp.WithHTTPClient(&http.Client{Transport: privateTransport(t)}))
		require.NoError(t, err)

		var buf bytes.Buffer
		_, err = jirahttp.NewAttachmentService(client).Download(context.Background(), "10000", &buf)

		require.NoError(t, err)
		assert.Equal(t, "file bytes", buf.String())
		assert.Equal(t, int32(1), media.requests.Load())
		assert.False(t, media.sawAuth.Load(), "the media host on another port got credentials")
	})
}
