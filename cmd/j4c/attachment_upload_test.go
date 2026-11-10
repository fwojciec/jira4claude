package main_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"
	"testing"

	"github.com/fwojciec/jira4claude"
	"github.com/fwojciec/jira4claude/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// sentFile is one file an uploader received.
type sentFile struct {
	key, name, content string
	size               int64
}

// uploader is an attachment service whose Upload gives each file the next ID
// from 10001 and records what it was sent.
type uploader struct {
	mu   sync.Mutex
	sent []sentFile
	// fails makes the upload of each file name in it fail with the error
	// given.
	fails map[string]error
}

func (u *uploader) service(t *testing.T) *mock.AttachmentService {
	t.Helper()
	return &mock.AttachmentService{
		UploadFn: func(_ context.Context, key string, file jira4claude.AttachmentFile) (*jira4claude.Attachment, error) {
			rc, err := file.Open()
			require.NoError(t, err)
			data, err := io.ReadAll(rc)
			require.NoError(t, err)
			require.NoError(t, rc.Close())

			u.mu.Lock()
			defer u.mu.Unlock()
			u.sent = append(u.sent, sentFile{key: key, name: file.Name, content: string(data), size: file.Size})
			if err, ok := u.fails[file.Name]; ok {
				return nil, err
			}
			return &jira4claude.Attachment{
				ID:       strconv.Itoa(10000 + len(u.sent)),
				Filename: file.Name,
				MimeType: "text/plain",
				Size:     file.Size,
			}, nil
		},
	}
}

func (u *uploader) names() []string {
	u.mu.Lock()
	defer u.mu.Unlock()
	names := make([]string, 0, len(u.sent))
	for _, s := range u.sent {
		names = append(names, s.name)
	}
	return names
}

// noUpload is an attachment service whose Upload fails the test.
func noUpload(t *testing.T) *mock.AttachmentService {
	t.Helper()
	return &mock.AttachmentService{
		UploadFn: func(context.Context, string, jira4claude.AttachmentFile) (*jira4claude.Attachment, error) {
			t.Error("Upload must not be called")
			return nil, errors.New("unexpected Upload call")
		},
	}
}

// uploadFixture is the absolute path of testdata/upload/hello.txt, found from
// this source file rather than the working directory, which a parallel test
// in this package changes (see goldenDir). It is for tables built without a
// *testing.T.
func uploadFixture() string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		panic("cannot locate the test source")
	}
	return filepath.Join(filepath.Dir(file), "testdata", "upload", "hello.txt")
}

// localFile writes content to name in dir and returns its path.
func localFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	return path
}

func TestRun_AttachmentUpload(t *testing.T) {
	t.Parallel()

	t.Run("several files are uploaded in order, one attachment line each", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		a := localFile(t, dir, "a.txt", "alpha")
		b := localFile(t, dir, "sub/b log.txt", "bravo!")
		u := &uploader{}

		res := runCLI(t, attachmentDeps(t, u.service(t)), "attachment", "upload", "TEST-1", a, b)

		assert.Equal(t, 0, res.code, "stderr: %q", res.stderr)
		assert.Equal(t, "10001\t5\ttext/plain\ta.txt\n10002\t6\ttext/plain\tb log.txt\n", res.stdout)
		assert.Empty(t, res.stderr)
		assert.Equal(t, []sentFile{
			{key: "TEST-1", name: "a.txt", content: "alpha", size: 5},
			{key: "TEST-1", name: "b log.txt", content: "bravo!", size: 6},
		}, u.sent)
	})

	t.Run("a path holding a comma is one file, not split", func(t *testing.T) {
		t.Parallel()

		u := &uploader{}

		res := runCLI(t, attachmentDeps(t, u.service(t)), "attachment", "upload", "TEST-1", localFile(t, t.TempDir(), "a,b.txt", "alpha"))

		assert.Equal(t, 0, res.code, "stderr: %q", res.stderr)
		assert.Equal(t, []string{"a,b.txt"}, u.names())
	})

	t.Run("--json prints one result listing the new attachments", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		u := &uploader{}

		res := runCLI(t, attachmentDeps(t, u.service(t)), "--json", "attachment", "upload", "TEST-1",
			localFile(t, dir, "a.txt", "alpha"), localFile(t, dir, "b.txt", "bravo!"))

		assert.Equal(t, 0, res.code, "stderr: %q", res.stderr)
		assert.JSONEq(t, `{
			"key": "TEST-1",
			"url": "https://test.atlassian.net/browse/TEST-1",
			"action": "attachment_added",
			"changes": {},
			"attachments": [
				{"id": "10001", "filename": "a.txt", "mimeType": "text/plain", "size": 5},
				{"id": "10002", "filename": "b.txt", "mimeType": "text/plain", "size": 6}
			]
		}`, res.stdout)
		assert.Empty(t, res.stderr)
	})

	t.Run("a file that is missing fails before any upload", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		missing := filepath.Join(dir, "missing.txt")

		res := runCLI(t, attachmentDeps(t, noUpload(t)), "attachment", "upload", "TEST-1", localFile(t, dir, "a.txt", "alpha"), missing)

		assert.Equal(t, 1, res.code)
		assert.Empty(t, res.stdout)
		assert.Equal(t, "[error] no such file: \""+missing+"\"\n", res.stderr)
	})

	t.Run("a directory fails before any upload", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()

		res := runCLI(t, attachmentDeps(t, noUpload(t)), "--json", "attachment", "upload", "TEST-1", dir)

		assert.Equal(t, 1, res.code)
		assert.Empty(t, res.stdout)
		assertJSONError(t, res.stderr, jira4claude.EValidation, `"`+dir+`" is a directory, not a file`)
	})

	t.Run("the same file twice fails before any upload", func(t *testing.T) {
		t.Parallel()

		a := localFile(t, t.TempDir(), "a.txt", "alpha")

		res := runCLI(t, attachmentDeps(t, noUpload(t)), "attachment", "upload", "TEST-1", a, a)

		assert.Equal(t, 1, res.code)
		assert.Empty(t, res.stdout)
		assert.Equal(t, "[error] \""+a+"\" is given twice\n", res.stderr)
	})

	t.Run("no file is a usage error", func(t *testing.T) {
		t.Parallel()

		res := runCLI(t, attachmentDeps(t, noUpload(t)), "attachment", "upload", "TEST-1")

		assert.Equal(t, 1, res.code)
		assert.Empty(t, res.stdout)
	})

	t.Run("two files with the same base name are both uploaded, with a warning", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		a := localFile(t, dir, "x/log.txt", "one")
		b := localFile(t, dir, "y/log.txt", "two")
		u := &uploader{}

		res := runCLI(t, attachmentDeps(t, u.service(t)), "attachment", "upload", "TEST-1", a, b)

		assert.Equal(t, 0, res.code, "stderr: %q", res.stderr)
		assert.Equal(t, "10001\t3\ttext/plain\tlog.txt\n10002\t3\ttext/plain\tlog.txt\n", res.stdout)
		assert.Equal(t, "[warn] \""+a+"\" and \""+b+"\" are both sent as \"log.txt\"; Jira keeps each as its own attachment with its own ID\n", res.stderr)
		assert.Equal(t, []string{"log.txt", "log.txt"}, u.names())
	})

	t.Run("help says attachment list shows what an interrupted run attached", func(t *testing.T) {
		t.Parallel()

		res := runCLI(t, unreachableDeps(t), "attachment", "upload", "--help")

		assert.Equal(t, 0, res.code)
		assert.Contains(t, res.stdout, "attachment list KEY")
		assert.Contains(t, res.stdout, "interrupted")
	})

	t.Run("a single file that fails is reported as its error alone", func(t *testing.T) {
		t.Parallel()

		u := &uploader{fails: map[string]error{"a.txt": &jira4claude.Error{
			Code:    jira4claude.EInternal,
			Message: "POST /rest/api/3/issue/TEST-1/attachments failed and was not retried; the request may have been applied, check before retrying",
		}}}

		res := runCLI(t, attachmentDeps(t, u.service(t)), "attachment", "upload", "TEST-1", localFile(t, t.TempDir(), "a.txt", "alpha"))

		assert.Equal(t, 7, res.code)
		assert.Empty(t, res.stdout)
		assert.Equal(t, "[error] POST /rest/api/3/issue/TEST-1/attachments failed and was not retried; the request may have been applied, check before retrying\n", res.stderr)
	})
}

// TestRun_AttachmentUploadPartialFailure follows
// TestRun_AttachmentDownloadPartialFailure: each failure is its own error
// line, a summary error follows, the files uploaded are printed, and the exit
// code is that of the first failure.
func TestRun_AttachmentUploadPartialFailure(t *testing.T) {
	t.Parallel()

	three := func(t *testing.T) []string {
		t.Helper()
		dir := t.TempDir()
		return []string{localFile(t, dir, "a.txt", "alpha"), localFile(t, dir, "b.txt", "bravo"), localFile(t, dir, "c.txt", "charlie")}
	}

	t.Run("a failure does not stop the other uploads", func(t *testing.T) {
		t.Parallel()

		paths := three(t)
		u := &uploader{fails: map[string]error{"b.txt": &jira4claude.Error{Code: jira4claude.EValidation, Message: "too large"}}}

		res := runCLI(t, attachmentDeps(t, u.service(t)), append([]string{"attachment", "upload", "TEST-1"}, paths...)...)

		assert.Equal(t, 1, res.code)
		assert.Equal(t, "10001\t5\ttext/plain\ta.txt\n10003\t7\ttext/plain\tc.txt\n", res.stdout)
		assert.Equal(t,
			"[error] file \""+paths[1]+"\": too large\n"+
				"[error] could not upload 1 of 3 files: \""+paths[1]+"\"\n",
			res.stderr)
		assert.Equal(t, []string{"a.txt", "b.txt", "c.txt"}, u.names())
	})

	t.Run("JSON prints the uploaded files as one result and one error line per failure", func(t *testing.T) {
		t.Parallel()

		paths := three(t)
		u := &uploader{fails: map[string]error{"b.txt": errors.New("connection reset")}}

		res := runCLI(t, attachmentDeps(t, u.service(t)), append([]string{"--json", "attachment", "upload", "TEST-1"}, paths...)...)

		assert.Equal(t, 7, res.code)
		var got struct {
			Attachments []struct {
				ID       string `json:"id"`
				Filename string `json:"filename"`
			} `json:"attachments"`
		}
		require.NoError(t, json.Unmarshal([]byte(res.stdout), &got), "stdout: %q", res.stdout)
		require.Len(t, got.Attachments, 2)
		assert.Equal(t, "10001", got.Attachments[0].ID)
		assert.Equal(t, "c.txt", got.Attachments[1].Filename)
		lines := stderrLines(t, res.stderr)
		require.Len(t, lines, 2)
		assert.Equal(t, map[string]any{"level": "error", "error": true, "code": jira4claude.EInternal,
			"message": "file \"" + paths[1] + "\": connection reset"}, lines[0])
		assert.Equal(t, map[string]any{"level": "error", "error": true, "code": jira4claude.EInternal,
			"message": "could not upload 1 of 3 files: \"" + paths[1] + "\""}, lines[1])
	})

	t.Run("differing failures exit with the code of the first", func(t *testing.T) {
		t.Parallel()

		paths := three(t)
		u := &uploader{fails: map[string]error{
			"a.txt": &jira4claude.Error{Code: jira4claude.EValidation, Message: "too large"},
			"c.txt": errors.New("connection reset"),
		}}

		res := runCLI(t, attachmentDeps(t, u.service(t)), append([]string{"attachment", "upload", "TEST-1"}, paths...)...)

		assert.Equal(t, 1, res.code)
		assert.Equal(t, "10002\t5\ttext/plain\tb.txt\n", res.stdout)
		assert.Equal(t,
			"[error] file \""+paths[0]+"\": too large\n"+
				"[error] file \""+paths[2]+"\": connection reset\n"+
				"[error] could not upload 2 of 3 files: \""+paths[0]+"\", \""+paths[2]+"\"\n",
			res.stderr)
	})

	for _, asJSON := range []bool{false, true} {
		t.Run("every file failing prints no attachment, json="+strconv.FormatBool(asJSON), func(t *testing.T) {
			t.Parallel()

			paths := three(t)[:2]
			u := &uploader{fails: map[string]error{"a.txt": errors.New("connection reset"), "b.txt": errors.New("connection reset")}}
			args := append([]string{"attachment", "upload", "TEST-1"}, paths...)
			if asJSON {
				args = append([]string{"--json"}, args...)
			}

			res := runCLI(t, attachmentDeps(t, u.service(t)), args...)

			assert.Equal(t, 7, res.code)
			if asJSON {
				// stdout still parses: the result, with no attachment.
				assert.JSONEq(t, `{"key":"TEST-1","url":"https://test.atlassian.net/browse/TEST-1",
					"action":"attachment_added","changes":{},"attachments":[]}`, res.stdout)
				assert.Len(t, stderrLines(t, res.stderr), 3)
			} else {
				assert.Empty(t, res.stdout, "text mode prints a line per attachment, so none")
				assert.Contains(t, res.stderr, "could not upload 2 of 2 files")
			}
		})
	}

	for _, tc := range []struct {
		code string
		exit int
	}{
		{jira4claude.EUnauthorized, 2},
		{jira4claude.EForbidden, 3},
		{jira4claude.ENotFound, 4},
		{jira4claude.ERateLimit, 6},
	} {
		t.Run(tc.code+" stops the run: the rest would fail the same way", func(t *testing.T) {
			t.Parallel()

			paths := three(t)
			u := &uploader{fails: map[string]error{"b.txt": &jira4claude.Error{Code: tc.code, Message: "denied"}}}

			res := runCLI(t, attachmentDeps(t, u.service(t)), append([]string{"attachment", "upload", "TEST-1"}, paths...)...)

			assert.Equal(t, tc.exit, res.code)
			assert.Equal(t, "10001\t5\ttext/plain\ta.txt\n", res.stdout)
			assert.Equal(t,
				"[error] file \""+paths[1]+"\": denied\n"+
					"[error] could not upload 2 of 3 files: \""+paths[1]+"\"; stopped after a "+tc.code+" error, not tried: \""+paths[2]+"\"\n",
				res.stderr)
			assert.Equal(t, []string{"a.txt", "b.txt"}, u.names())
		})
	}
}
