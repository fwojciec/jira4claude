package http_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fwojciec/jira4claude"
	jirahttp "github.com/fwojciec/jira4claude/http"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// uploadResponseJSON is one entry of the addAttachment response, in the shape
// of the example in Atlassian's REST v3 OpenAPI document: a string id and
// created in epoch milliseconds.
const uploadResponseJSON = `[{
	"self": "https://example.atlassian.net/rest/api/3/attachments/10001",
	"id": "10001",
	"filename": "notes.txt",
	"author": {"accountId": "5b10a2844c20165700ede21g", "displayName": "Mia Krystof"},
	"created": 1651316514000,
	"size": 11,
	"mimeType": "text/plain",
	"content": "https://example.atlassian.net/rest/api/3/attachment/content/10001"
}]`

// memFile is an AttachmentFile over content that counts its opens.
func memFile(name, content string, opens *atomic.Int32) jira4claude.AttachmentFile {
	return jira4claude.AttachmentFile{
		Name: name,
		Size: int64(len(content)),
		Open: func() (io.ReadCloser, error) {
			if opens != nil {
				opens.Add(1)
			}
			return io.NopCloser(strings.NewReader(content)), nil
		},
	}
}

// receivedUpload is what an upload test server saw in one request.
type receivedUpload struct {
	method, path, rawPath string
	header                http.Header
	contentLength         int64
	transferEncoding      []string
	user, pass            string
	body                  []byte
}

// uploadServer answers each request with the next of statuses (the last one
// repeats), with body for a 200, and records every request.
type uploadServer struct {
	*httptest.Server
	mu       sync.Mutex
	received []receivedUpload
}

func newUploadServer(t *testing.T, body string, statuses ...int) *uploadServer {
	t.Helper()
	us := &uploadServer{}
	us.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, err := io.ReadAll(r.Body)
		assert.NoError(t, err)
		user, pass, _ := r.BasicAuth()
		us.mu.Lock()
		n := len(us.received)
		us.received = append(us.received, receivedUpload{
			method: r.Method, path: r.URL.Path, rawPath: r.URL.EscapedPath(),
			header: r.Header.Clone(), contentLength: r.ContentLength, transferEncoding: r.TransferEncoding,
			user: user, pass: pass, body: data,
		})
		us.mu.Unlock()
		status := statuses[min(n, len(statuses)-1)]
		if status == http.StatusOK {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(body))
			return
		}
		w.WriteHeader(status)
		if status >= 500 {
			_, _ = w.Write([]byte(`{"errorMessages":["Server error"]}`))
		}
	}))
	t.Cleanup(us.Close)
	return us
}

func (us *uploadServer) requests() []receivedUpload {
	us.mu.Lock()
	defer us.mu.Unlock()
	return append([]receivedUpload(nil), us.received...)
}

// formPart is one part of a multipart/form-data body.
type formPart struct {
	formName, fileName, contentType string
	content                         []byte
}

// parseForm reads a multipart/form-data body with the boundary of
// contentType.
func parseForm(t *testing.T, contentType string, body []byte) []formPart {
	t.Helper()
	mediaType, params, err := mime.ParseMediaType(contentType)
	require.NoError(t, err)
	require.Equal(t, "multipart/form-data", mediaType)
	require.NotEmpty(t, params["boundary"])
	mr := multipart.NewReader(bytes.NewReader(body), params["boundary"])
	var parts []formPart
	for {
		p, err := mr.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		require.NoError(t, err)
		content, err := io.ReadAll(p)
		require.NoError(t, err)
		parts = append(parts, formPart{
			formName: p.FormName(), fileName: p.FileName(),
			contentType: p.Header.Get("Content-Type"), content: content,
		})
	}
	return parts
}

func TestAttachmentService_Upload(t *testing.T) {
	t.Parallel()

	t.Run("posts one multipart file part with the no-check token and reads the new attachment", func(t *testing.T) {
		t.Parallel()

		server := newUploadServer(t, uploadResponseJSON, http.StatusOK)
		svc := jirahttp.NewAttachmentService(newTestClient(t, server.URL, "user@example.com", "api-token"))

		got, err := svc.Upload(context.Background(), "TEST-1", memFile("notes.txt", "hello world", nil))

		require.NoError(t, err)
		reqs := server.requests()
		require.Len(t, reqs, 1)
		r := reqs[0]
		assert.Equal(t, http.MethodPost, r.method)
		assert.Equal(t, "/rest/api/3/issue/TEST-1/attachments", r.path)
		assert.Equal(t, "no-check", r.header.Get("X-Atlassian-Token"))
		assert.Equal(t, "application/json", r.header.Get("Accept"))
		assert.Equal(t, "user@example.com", r.user)
		assert.Equal(t, "api-token", r.pass)
		assert.Equal(t, int64(len(r.body)), r.contentLength, "Content-Length is set to the exact body length")
		assert.Empty(t, r.transferEncoding, "the body is not sent chunked")

		parts := parseForm(t, r.header.Get("Content-Type"), r.body)
		require.Len(t, parts, 1)
		assert.Equal(t, "file", parts[0].formName)
		assert.Equal(t, "notes.txt", parts[0].fileName)
		assert.Equal(t, "application/octet-stream", parts[0].contentType)
		assert.Equal(t, "hello world", string(parts[0].content))

		require.NotNil(t, got)
		assert.Equal(t, "10001", got.ID)
		assert.Equal(t, "notes.txt", got.Filename)
		assert.Equal(t, "text/plain", got.MimeType)
		assert.Equal(t, int64(11), got.Size)
		require.NotNil(t, got.Author)
		assert.Equal(t, "Mia Krystof", got.Author.DisplayName)
		assert.True(t, time.UnixMilli(1651316514000).Equal(got.Created), "created: %v", got.Created)
	})

	t.Run("a numeric id in the response is read as a string", func(t *testing.T) {
		t.Parallel()

		server := newUploadServer(t, `[{"id":10002,"filename":"a.txt","size":1,"mimeType":"text/plain"}]`, http.StatusOK)
		svc := jirahttp.NewAttachmentService(newTestClient(t, server.URL, "user", "pass"))

		got, err := svc.Upload(context.Background(), "TEST-1", memFile("a.txt", "a", nil))

		require.NoError(t, err)
		assert.Equal(t, "10002", got.ID)
	})

	t.Run("escapes the key", func(t *testing.T) {
		t.Parallel()

		server := newUploadServer(t, uploadResponseJSON, http.StatusOK)
		svc := jirahttp.NewAttachmentService(newTestClient(t, server.URL, "user", "pass"))

		_, err := svc.Upload(context.Background(), "TEST/1", memFile("notes.txt", "hello world", nil))

		require.NoError(t, err)
		reqs := server.requests()
		require.Len(t, reqs, 1)
		assert.Equal(t, "/rest/api/3/issue/TEST%2F1/attachments", reqs[0].rawPath)
	})

	t.Run("a filename with quotes and non-ASCII letters reaches the server intact", func(t *testing.T) {
		t.Parallel()

		server := newUploadServer(t, uploadResponseJSON, http.StatusOK)
		svc := jirahttp.NewAttachmentService(newTestClient(t, server.URL, "user", "pass"))

		_, err := svc.Upload(context.Background(), "TEST-1", memFile(`say "zażółć".txt`, "x", nil))

		require.NoError(t, err)
		reqs := server.requests()
		require.Len(t, reqs, 1)
		parts := parseForm(t, reqs[0].header.Get("Content-Type"), reqs[0].body)
		require.Len(t, parts, 1)
		assert.Equal(t, `say "zażółć".txt`, parts[0].fileName)
	})

	t.Run("content holding the boundary of an earlier request is still sent whole", func(t *testing.T) {
		t.Parallel()

		server := newUploadServer(t, uploadResponseJSON, http.StatusOK)
		svc := jirahttp.NewAttachmentService(newTestClient(t, server.URL, "user", "pass"))
		_, err := svc.Upload(context.Background(), "TEST-1", memFile("a.txt", "a", nil))
		require.NoError(t, err)
		_, params, err := mime.ParseMediaType(server.requests()[0].header.Get("Content-Type"))
		require.NoError(t, err)
		content := "before\r\n--" + params["boundary"] + "--\r\nafter"

		_, err = svc.Upload(context.Background(), "TEST-1", memFile("b.txt", content, nil))

		require.NoError(t, err)
		second := server.requests()[1]
		parts := parseForm(t, second.header.Get("Content-Type"), second.body)
		require.Len(t, parts, 1)
		assert.Equal(t, content, string(parts[0].content))
	})

	t.Run("a 429 is retried with the whole file opened and sent again", func(t *testing.T) {
		t.Parallel()

		server := newUploadServer(t, uploadResponseJSON, http.StatusTooManyRequests, http.StatusOK)
		svc := jirahttp.NewAttachmentService(newTestClient(t, server.URL, "user", "pass", jirahttp.WithRetryBaseDelay(0)))
		var opens atomic.Int32

		got, err := svc.Upload(context.Background(), "TEST-1", memFile("notes.txt", "hello world", &opens))

		require.NoError(t, err)
		assert.Equal(t, "10001", got.ID)
		reqs := server.requests()
		require.Len(t, reqs, 2)
		assert.Equal(t, int32(2), opens.Load())
		for i, r := range reqs {
			assert.Equal(t, int64(len(r.body)), r.contentLength, "request %d", i)
			parts := parseForm(t, r.header.Get("Content-Type"), r.body)
			require.Len(t, parts, 1, "request %d", i)
			assert.Equal(t, "hello world", string(parts[0].content), "request %d", i)
		}
		assert.Equal(t, reqs[0].body, reqs[1].body, "the retry sends the same bytes, boundary included")
	})

	for _, status := range []int{http.StatusInternalServerError, http.StatusBadGateway, http.StatusServiceUnavailable} {
		t.Run(http.StatusText(status)+" is not retried and may have been applied", func(t *testing.T) {
			t.Parallel()

			server := newUploadServer(t, uploadResponseJSON, status, http.StatusOK)
			svc := jirahttp.NewAttachmentService(newTestClient(t, server.URL, "user", "pass", jirahttp.WithRetryBaseDelay(0)))

			got, err := svc.Upload(context.Background(), "TEST-1", memFile("notes.txt", "hello world", nil))

			require.Error(t, err)
			assert.Nil(t, got)
			assert.Len(t, server.requests(), 1)
			assert.Equal(t, jira4claude.EInternal, jira4claude.ErrorCode(err))
			assert.Contains(t, err.Error(), "may have been applied")
		})
	}

	t.Run("a connection error is not retried and may have been applied", func(t *testing.T) {
		t.Parallel()

		var attempts atomic.Int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			attempts.Add(1)
			_, _ = io.Copy(io.Discard, r.Body)
			hj, ok := w.(http.Hijacker)
			if !assert.True(t, ok) {
				return
			}
			conn, _, err := hj.Hijack()
			if assert.NoError(t, err) {
				_ = conn.Close()
			}
		}))
		defer server.Close()
		svc := jirahttp.NewAttachmentService(newTestClient(t, server.URL, "user", "pass", jirahttp.WithRetryBaseDelay(0)))

		_, err := svc.Upload(context.Background(), "TEST-1", memFile("notes.txt", "hello world", nil))

		require.Error(t, err)
		assert.Equal(t, int32(1), attempts.Load())
		assert.Contains(t, err.Error(), "may have been applied")
	})

	t.Run("a 413 is a validation error naming the limits, also with a body that is not JSON", func(t *testing.T) {
		t.Parallel()

		var attempts atomic.Int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			attempts.Add(1)
			_, _ = io.Copy(io.Discard, r.Body)
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(http.StatusRequestEntityTooLarge)
			_, _ = w.Write([]byte("<html><body>Request Entity Too Large</body></html>"))
		}))
		defer server.Close()
		svc := jirahttp.NewAttachmentService(newTestClient(t, server.URL, "user", "pass", jirahttp.WithRetryBaseDelay(0)))

		_, err := svc.Upload(context.Background(), "TEST-1", memFile("big.bin", "0123456789", nil))

		require.Error(t, err)
		assert.Equal(t, int32(1), attempts.Load())
		assert.Equal(t, jira4claude.EValidation, jira4claude.ErrorCode(err))
		assert.Equal(t,
			`Jira refused "big.bin" (10 bytes) as too large (HTTP 413): it exceeds the site's attachment size limit, or the issue has reached its attachment limit`,
			err.Error())
	})

	t.Run("a 413 with a Jira error body keeps Jira's message", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = io.Copy(io.Discard, r.Body)
			w.WriteHeader(http.StatusRequestEntityTooLarge)
			_, _ = w.Write([]byte(`{"errorMessages":["The file is too big"]}`))
		}))
		defer server.Close()
		svc := jirahttp.NewAttachmentService(newTestClient(t, server.URL, "user", "pass"))

		_, err := svc.Upload(context.Background(), "TEST-1", memFile("big.bin", "0123456789", nil))

		require.Error(t, err)
		assert.Equal(t, jira4claude.EValidation, jira4claude.ErrorCode(err))
		assert.Contains(t, err.Error(), "(HTTP 413)")
		assert.Contains(t, err.Error(), "The file is too big")
	})

	for _, tc := range []struct {
		status int
		code   string
	}{
		{http.StatusForbidden, jira4claude.EForbidden},
		{http.StatusNotFound, jira4claude.ENotFound},
	} {
		t.Run(http.StatusText(tc.status)+" is returned with its code and not retried", func(t *testing.T) {
			t.Parallel()

			server := newUploadServer(t, uploadResponseJSON, tc.status, http.StatusOK)
			svc := jirahttp.NewAttachmentService(newTestClient(t, server.URL, "user", "pass", jirahttp.WithRetryBaseDelay(0)))

			_, err := svc.Upload(context.Background(), "TEST-1", memFile("notes.txt", "hello world", nil))

			require.Error(t, err)
			assert.Equal(t, tc.code, jira4claude.ErrorCode(err))
			assert.Len(t, server.requests(), 1)
		})
	}

	for _, status := range []int{http.StatusTemporaryRedirect, http.StatusPermanentRedirect, http.StatusFound} {
		t.Run(http.StatusText(status)+" is not followed: the file goes nowhere else", func(t *testing.T) {
			t.Parallel()

			var elsewhere atomic.Int32
			other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				elsewhere.Add(1)
				_, _ = io.Copy(io.Discard, r.Body)
				_, _ = w.Write([]byte(uploadResponseJSON))
			}))
			defer other.Close()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = io.Copy(io.Discard, r.Body)
				http.Redirect(w, r, other.URL+"/upload?sig=secret", status)
			}))
			defer server.Close()
			svc := jirahttp.NewAttachmentService(newTestClient(t, server.URL, "user", "pass", jirahttp.WithRetryBaseDelay(0)))

			_, err := svc.Upload(context.Background(), "TEST-1", memFile("notes.txt", "hello world", nil))

			require.Error(t, err)
			assert.Equal(t, int32(0), elsewhere.Load())
			assert.NotContains(t, err.Error(), "secret")
			assert.Contains(t, err.Error(), "redirect")
		})
	}

	for _, body := range []string{`[]`, `[{"filename":"notes.txt","size":11}]`, `[{"id":"1"},{"id":"2"}]`, `{"id":"1"}`} {
		t.Run("a success whose body is not one attachment with an ID says the file was uploaded: "+body, func(t *testing.T) {
			t.Parallel()

			server := newUploadServer(t, body, http.StatusOK)
			svc := jirahttp.NewAttachmentService(newTestClient(t, server.URL, "user", "pass"))

			got, err := svc.Upload(context.Background(), "TEST-1", memFile("notes.txt", "hello world", nil))

			require.Error(t, err)
			assert.Nil(t, got)
			assert.Equal(t, jira4claude.EInternal, jira4claude.ErrorCode(err))
			assert.Contains(t, err.Error(), `Jira accepted "notes.txt" (HTTP 200)`)
			assert.Contains(t, err.Error(), "attachment list TEST-1")
		})
	}

	// jir-dqbz: the 200 says the file was attached, even if the body is
	// lost on the way.
	t.Run("a 200 whose body is cut short says the file was uploaded, not that it may have been", func(t *testing.T) {
		t.Parallel()

		var posts atomic.Int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			posts.Add(1)
			_, _ = io.Copy(io.Discard, r.Body)
			w.Header().Set("Content-Length", "100")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[{"id":`))
		}))
		defer server.Close()
		svc := jirahttp.NewAttachmentService(newTestClient(t, server.URL, "user", "pass"))

		_, err := svc.Upload(context.Background(), "TEST-1", memFile("notes.txt", "hello world", nil))

		require.Error(t, err)
		assert.Contains(t, err.Error(), `Jira accepted "notes.txt" (HTTP 200)`)
		assert.NotContains(t, err.Error(), "may have been applied")
		assert.Equal(t, int32(1), posts.Load())
	})

	t.Run("a file that cannot be opened fails before any request", func(t *testing.T) {
		t.Parallel()

		server := newUploadServer(t, uploadResponseJSON, http.StatusOK)
		svc := jirahttp.NewAttachmentService(newTestClient(t, server.URL, "user", "pass"))
		file := jira4claude.AttachmentFile{Name: "x", Size: 1, Open: func() (io.ReadCloser, error) {
			return nil, errors.New("permission denied")
		}}

		_, err := svc.Upload(context.Background(), "TEST-1", file)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "permission denied")
		assert.Empty(t, server.requests())
	})
}
