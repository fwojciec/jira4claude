package main_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/fwojciec/jira4claude"
	"github.com/fwojciec/jira4claude/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// deleter is an attachment service holding sampleAttachments()[0] (ID
// 10000) that records the calls made to it.
type deleter struct {
	mu    sync.Mutex
	calls []string
	// getErr and deleteErr make Get and Delete fail.
	getErr, deleteErr error
}

func (d *deleter) service() *mock.AttachmentService {
	return &mock.AttachmentService{
		GetFn: func(_ context.Context, id string) (*jira4claude.Attachment, error) {
			d.record("get " + id)
			if d.getErr != nil {
				return nil, d.getErr
			}
			return sampleAttachments()[0], nil
		},
		DeleteFn: func(_ context.Context, id string) error {
			d.record("delete " + id)
			return d.deleteErr
		},
	}
}

func (d *deleter) record(call string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.calls = append(d.calls, call)
}

func (d *deleter) recorded() []string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]string(nil), d.calls...)
}

func TestRun_AttachmentDelete(t *testing.T) {
	t.Parallel()

	t.Run("reads the attachment, deletes it and prints it as attachment list does", func(t *testing.T) {
		t.Parallel()

		d := &deleter{}

		res := runCLI(t, attachmentDeps(t, d.service()), "attachment", "delete", "10000")

		assert.Equal(t, 0, res.code, "stderr: %q", res.stderr)
		assert.Equal(t, "10000\t23123\timage/jpeg\tpicture.jpg\n", res.stdout)
		assert.Empty(t, res.stderr)
		assert.Equal(t, []string{"get 10000", "delete 10000"}, d.recorded())
	})

	t.Run("--json prints one result naming the deleted attachment and no issue", func(t *testing.T) {
		t.Parallel()

		res := runCLI(t, attachmentDeps(t, (&deleter{}).service()), "--json", "attachment", "delete", "10000")

		assert.Equal(t, 0, res.code, "stderr: %q", res.stderr)
		assert.JSONEq(t, `{
			"action": "attachment_deleted",
			"changes": {},
			"attachments": [{
				"id": "10000", "filename": "picture.jpg", "mimeType": "image/jpeg", "size": 23123,
				"author": "Mia Krystof", "created": "2022-10-06T07:32:47Z"
			}]
		}`, res.stdout)
		assert.Empty(t, res.stderr)
	})

	t.Run("an attachment that cannot be read is not deleted", func(t *testing.T) {
		t.Parallel()

		d := &deleter{getErr: &jira4claude.Error{Code: jira4claude.ENotFound, Message: "Not Found"}}

		res := runCLI(t, attachmentDeps(t, d.service()), "attachment", "delete", "10000")

		assert.Equal(t, 4, res.code)
		assert.Empty(t, res.stdout)
		assert.Equal(t, "[error] Not Found\n", res.stderr)
		assert.Equal(t, []string{"get 10000"}, d.recorded())
	})

	t.Run("a refused delete prints nothing on stdout", func(t *testing.T) {
		t.Parallel()

		d := &deleter{deleteErr: &jira4claude.Error{Code: jira4claude.EForbidden, Message: "Forbidden"}}

		res := runCLI(t, attachmentDeps(t, d.service()), "attachment", "delete", "10000")

		assert.Equal(t, 3, res.code)
		assert.Empty(t, res.stdout)
		assert.Equal(t, "[error] Forbidden\n", res.stderr)
	})

	t.Run("a delete answered not found after the read is success, with a warning", func(t *testing.T) {
		t.Parallel()

		d := &deleter{deleteErr: &jira4claude.Error{Code: jira4claude.ENotFound, Message: "Not Found"}}

		res := runCLI(t, attachmentDeps(t, d.service()), "attachment", "delete", "10000")

		assert.Equal(t, 0, res.code)
		assert.Equal(t, "10000\t23123\timage/jpeg\tpicture.jpg\n", res.stdout)
		assert.Equal(t,
			"[warn] attachment 10000 (\"picture.jpg\") was read, but the delete answered not found: "+
				"it was already gone, deleted by a retried attempt of this delete or by someone else meanwhile\n",
			res.stderr)
		assert.Equal(t, []string{"get 10000", "delete 10000"}, d.recorded())
	})

	t.Run("in --json the already-gone warning is a JSON line on stderr", func(t *testing.T) {
		t.Parallel()

		d := &deleter{deleteErr: &jira4claude.Error{Code: jira4claude.ENotFound, Message: "Not Found"}}

		res := runCLI(t, attachmentDeps(t, d.service()), "--json", "attachment", "delete", "10000")

		assert.Equal(t, 0, res.code)
		assertOneJSONValue(t, res.stdout)
		lines := stderrLines(t, res.stderr)
		require.Len(t, lines, 1)
		assert.Equal(t, "warning", lines[0]["level"])
	})

	t.Run("another delete failure is passed on as it is", func(t *testing.T) {
		t.Parallel()

		d := &deleter{deleteErr: errors.New("connection reset")}

		res := runCLI(t, attachmentDeps(t, d.service()), "attachment", "delete", "10000")

		assert.Equal(t, 7, res.code)
		assert.Equal(t, "[error] connection reset\n", res.stderr)
	})

	t.Run("help says plainly that it deletes, permanently and without asking", func(t *testing.T) {
		t.Parallel()

		res := runCLI(t, unreachableDeps(t), "attachment", "delete", "--help")

		assert.Equal(t, 0, res.code)
		assert.Contains(t, res.stdout, "Delete an attachment permanently")
		assert.Contains(t, res.stdout, "no confirmation")
		assert.Contains(t, res.stdout, "already gone")
	})

	t.Run("no ID is a usage error", func(t *testing.T) {
		t.Parallel()

		res := runCLI(t, unreachableDeps(t), "attachment", "delete")

		assert.Equal(t, 1, res.code)
		assert.Empty(t, res.stdout)
	})
}
