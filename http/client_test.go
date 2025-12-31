package http_test

import (
	"context"
	"fmt"
	"io"
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

func TestNewClient(t *testing.T) {
	t.Parallel()

	t.Run("reads credentials from netrc", func(t *testing.T) {
		t.Parallel()

		// Create a temporary netrc file
		tmpDir := t.TempDir()
		netrcPath := filepath.Join(tmpDir, "netrc")
		netrcContent := `machine test.atlassian.net
  login user@example.com
  password api-token-123
`
		require.NoError(t, os.WriteFile(netrcPath, []byte(netrcContent), 0o600))

		client, err := jirahttp.NewClient("https://test.atlassian.net", jirahttp.WithNetrcPath(netrcPath))
		require.NoError(t, err)
		assert.NotNil(t, client)
	})

	t.Run("returns error when netrc file not found", func(t *testing.T) {
		t.Parallel()

		_, err := jirahttp.NewClient("https://test.atlassian.net", jirahttp.WithNetrcPath("/nonexistent/netrc"))
		assert.Error(t, err)
	})

	t.Run("returns error when machine not found in netrc", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		netrcPath := filepath.Join(tmpDir, "netrc")
		netrcContent := `machine other.atlassian.net
  login user@example.com
  password token
`
		require.NoError(t, os.WriteFile(netrcPath, []byte(netrcContent), 0o600))

		_, err := jirahttp.NewClient("https://test.atlassian.net", jirahttp.WithNetrcPath(netrcPath))
		assert.Error(t, err)
	})

	t.Run("returns error when login is empty in netrc", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		netrcPath := filepath.Join(tmpDir, "netrc")
		netrcContent := `machine test.atlassian.net
  password token
`
		require.NoError(t, os.WriteFile(netrcPath, []byte(netrcContent), 0o600))

		_, err := jirahttp.NewClient("https://test.atlassian.net", jirahttp.WithNetrcPath(netrcPath))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "login")
	})

	t.Run("returns error when password is empty in netrc", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		netrcPath := filepath.Join(tmpDir, "netrc")
		netrcContent := `machine test.atlassian.net
  login user@example.com
`
		require.NoError(t, os.WriteFile(netrcPath, []byte(netrcContent), 0o600))

		_, err := jirahttp.NewClient("https://test.atlassian.net", jirahttp.WithNetrcPath(netrcPath))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "password")
	})
}

func TestClient_Do(t *testing.T) {
	t.Parallel()

	t.Run("adds Basic auth header to requests", func(t *testing.T) {
		t.Parallel()

		var receivedAuth string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedAuth = r.Header.Get("Authorization")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"ok": true}`))
		}))
		defer server.Close()

		client := newTestClient(t, server.URL, "user@example.com", "api-token")

		req, err := http.NewRequestWithContext(context.Background(), "GET", "/rest/api/3/myself", nil)
		require.NoError(t, err)

		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		// Basic auth: base64("user@example.com:api-token")
		assert.Equal(t, "Basic dXNlckBleGFtcGxlLmNvbTphcGktdG9rZW4=", receivedAuth)
	})

	t.Run("prepends base URL to relative paths", func(t *testing.T) {
		t.Parallel()

		var receivedPath string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedPath = r.URL.Path
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		client := newTestClient(t, server.URL, "user", "pass")

		req, err := http.NewRequestWithContext(context.Background(), "GET", "/rest/api/3/issue/TEST-1", nil)
		require.NoError(t, err)

		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, "/rest/api/3/issue/TEST-1", receivedPath)
	})

	t.Run("sets Accept header to application/json", func(t *testing.T) {
		t.Parallel()

		var receivedAccept string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedAccept = r.Header.Get("Accept")
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		client := newTestClient(t, server.URL, "user", "pass")

		req, err := http.NewRequestWithContext(context.Background(), "GET", "/rest/api/3/myself", nil)
		require.NoError(t, err)

		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, "application/json", receivedAccept)
	})
}

func TestParseErrorResponse(t *testing.T) {
	t.Parallel()

	t.Run("parses errorMessages array", func(t *testing.T) {
		t.Parallel()

		body := `{"errorMessages": ["Summary is required", "Project is required"], "errors": {}}`
		err := jirahttp.ParseErrorResponse(http.StatusBadRequest, []byte(body))

		require.Error(t, err)
		assert.Contains(t, err.Error(), "Summary is required")
		assert.Contains(t, err.Error(), "Project is required")
	})

	t.Run("parses errors map", func(t *testing.T) {
		t.Parallel()

		body := `{"errorMessages": [], "errors": {"project": "Project 'XYZ' does not exist", "summary": "Field required"}}`
		err := jirahttp.ParseErrorResponse(http.StatusBadRequest, []byte(body))

		require.Error(t, err)
		assert.Contains(t, err.Error(), "Project 'XYZ' does not exist")
		assert.Contains(t, err.Error(), "Field required")
	})

	t.Run("parses combined errorMessages and errors", func(t *testing.T) {
		t.Parallel()

		body := `{"errorMessages": ["General error"], "errors": {"field": "Field error"}}`
		err := jirahttp.ParseErrorResponse(http.StatusBadRequest, []byte(body))

		require.Error(t, err)
		assert.Contains(t, err.Error(), "General error")
		assert.Contains(t, err.Error(), "Field error")
	})

	t.Run("returns nil for empty error response", func(t *testing.T) {
		t.Parallel()

		body := `{"errorMessages": [], "errors": {}}`
		err := jirahttp.ParseErrorResponse(http.StatusBadRequest, []byte(body))

		assert.NoError(t, err)
	})

	t.Run("returns error for invalid JSON", func(t *testing.T) {
		t.Parallel()

		body := `not valid json`
		err := jirahttp.ParseErrorResponse(http.StatusBadRequest, []byte(body))

		assert.Error(t, err)
	})

	t.Run("maps 400 to EValidation", func(t *testing.T) {
		t.Parallel()

		body := `{"errorMessages": ["Bad request"], "errors": {}}`
		err := jirahttp.ParseErrorResponse(http.StatusBadRequest, []byte(body))

		assert.Equal(t, jira4claude.EValidation, jira4claude.ErrorCode(err))
	})

	t.Run("maps 401 to EUnauthorized", func(t *testing.T) {
		t.Parallel()

		body := `{"errorMessages": ["Not authenticated"], "errors": {}}`
		err := jirahttp.ParseErrorResponse(http.StatusUnauthorized, []byte(body))

		assert.Equal(t, jira4claude.EUnauthorized, jira4claude.ErrorCode(err))
	})

	t.Run("maps 403 to EForbidden", func(t *testing.T) {
		t.Parallel()

		body := `{"errorMessages": ["Access denied"], "errors": {}}`
		err := jirahttp.ParseErrorResponse(http.StatusForbidden, []byte(body))

		assert.Equal(t, jira4claude.EForbidden, jira4claude.ErrorCode(err))
	})

	t.Run("maps 404 to ENotFound", func(t *testing.T) {
		t.Parallel()

		body := `{"errorMessages": ["Issue not found"], "errors": {}}`
		err := jirahttp.ParseErrorResponse(http.StatusNotFound, []byte(body))

		assert.Equal(t, jira4claude.ENotFound, jira4claude.ErrorCode(err))
	})

	t.Run("maps 409 to EConflict", func(t *testing.T) {
		t.Parallel()

		body := `{"errorMessages": ["Conflict"], "errors": {}}`
		err := jirahttp.ParseErrorResponse(http.StatusConflict, []byte(body))

		assert.Equal(t, jira4claude.EConflict, jira4claude.ErrorCode(err))
	})

	t.Run("maps 429 to ERateLimit", func(t *testing.T) {
		t.Parallel()

		body := `{"errorMessages": ["Too many requests"], "errors": {}}`
		err := jirahttp.ParseErrorResponse(http.StatusTooManyRequests, []byte(body))

		assert.Equal(t, jira4claude.ERateLimit, jira4claude.ErrorCode(err))
	})

	t.Run("maps 5xx to EInternal", func(t *testing.T) {
		t.Parallel()

		body := `{"errorMessages": ["Server error"], "errors": {}}`
		err := jirahttp.ParseErrorResponse(http.StatusInternalServerError, []byte(body))

		assert.Equal(t, jira4claude.EInternal, jira4claude.ErrorCode(err))
	})
}

func TestClient_NewJSONRequest(t *testing.T) {
	t.Parallel()

	t.Run("creates request with JSON body and content-type header", func(t *testing.T) {
		t.Parallel()

		var receivedBody []byte
		var receivedContentType string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedContentType = r.Header.Get("Content-Type")
			receivedBody, _ = io.ReadAll(r.Body)
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		client := newTestClient(t, server.URL, "user", "pass")
		body := map[string]string{"key": "value"}
		req, err := client.NewJSONRequest(context.Background(), http.MethodPost, "/test", body)
		require.NoError(t, err)

		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, "application/json", receivedContentType)
		assert.JSONEq(t, `{"key": "value"}`, string(receivedBody))
	})

	t.Run("returns error on marshal failure", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		client := newTestClient(t, server.URL, "user", "pass")
		// Channels cannot be marshaled to JSON
		body := map[string]any{"ch": make(chan int)}
		_, err := client.NewJSONRequest(context.Background(), http.MethodPost, "/test", body)

		require.Error(t, err)
		assert.Equal(t, jira4claude.EInternal, jira4claude.ErrorCode(err))
		assert.Contains(t, err.Error(), "marshal")
	})
}

func TestClient_DoRequest(t *testing.T) {
	t.Parallel()

	t.Run("returns body on expected status", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"key": "TEST-1"}`))
		}))
		defer server.Close()

		client := newTestClient(t, server.URL, "user", "pass")
		req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "/test", nil)
		require.NoError(t, err)

		body, err := client.DoRequest(req, http.StatusOK)
		require.NoError(t, err)
		assert.JSONEq(t, `{"key": "TEST-1"}`, string(body))
	})

	t.Run("returns empty body on 204 No Content", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}))
		defer server.Close()

		client := newTestClient(t, server.URL, "user", "pass")
		req, err := http.NewRequestWithContext(context.Background(), http.MethodDelete, "/test", nil)
		require.NoError(t, err)

		body, err := client.DoRequest(req, http.StatusNoContent)
		require.NoError(t, err)
		assert.Empty(t, body)
	})

	t.Run("returns error on HTTP failure", func(t *testing.T) {
		t.Parallel()

		// Use an invalid URL to force connection error
		client := newTestClient(t, "http://localhost:1", "user", "pass")
		req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "/test", nil)
		require.NoError(t, err)

		_, err = client.DoRequest(req, http.StatusOK)
		require.Error(t, err)
		assert.Equal(t, jira4claude.EInternal, jira4claude.ErrorCode(err))
	})

	t.Run("returns parsed API error on wrong status", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"errorMessages": ["Issue not found"]}`))
		}))
		defer server.Close()

		client := newTestClient(t, server.URL, "user", "pass")
		req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "/test", nil)
		require.NoError(t, err)

		_, err = client.DoRequest(req, http.StatusOK)
		require.Error(t, err)
		assert.Equal(t, jira4claude.ENotFound, jira4claude.ErrorCode(err))
		assert.Contains(t, err.Error(), "Issue not found")
	})

	t.Run("returns generic error on wrong status without API error", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`not json`))
		}))
		defer server.Close()

		client := newTestClient(t, server.URL, "user", "pass")
		req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "/test", nil)
		require.NoError(t, err)

		_, err = client.DoRequest(req, http.StatusOK)
		require.Error(t, err)
		assert.Equal(t, jira4claude.EInternal, jira4claude.ErrorCode(err))
	})
}

// newTestClient creates a Client with credentials for testing.
func newTestClient(t *testing.T, baseURL, username, password string) *jirahttp.Client {
	t.Helper()

	// Create temp netrc with test server credentials
	tmpDir := t.TempDir()
	netrcPath := filepath.Join(tmpDir, "netrc")

	// Extract host from baseURL
	u, err := url.Parse(baseURL)
	require.NoError(t, err)

	netrcContent := fmt.Sprintf("machine %s\n  login %s\n  password %s\n", u.Host, username, password)
	require.NoError(t, os.WriteFile(netrcPath, []byte(netrcContent), 0o600))

	client, err := jirahttp.NewClient(baseURL, jirahttp.WithNetrcPath(netrcPath))
	require.NoError(t, err)

	return client
}
