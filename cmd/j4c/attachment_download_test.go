package main_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/fwojciec/jira4claude"
	"github.com/fwojciec/jira4claude/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// downloader is an attachment service holding the given attachments, whose
// content is "content of ID". It records the IDs downloaded.
type downloader struct {
	mu          sync.Mutex
	attachments []*jira4claude.Attachment
	downloaded  []string
	// fail, when set, makes the download of that ID fail with failErr.
	fail    string
	failErr error
}

func (d *downloader) service(t *testing.T) *mock.AttachmentService {
	t.Helper()
	return &mock.AttachmentService{
		ListFn: func(_ context.Context, key string) ([]*jira4claude.Attachment, error) {
			assert.Equal(t, "TEST-1", key)
			return d.attachments, nil
		},
		DownloadFn: func(_ context.Context, id string, w io.Writer) (int64, error) {
			d.mu.Lock()
			d.downloaded = append(d.downloaded, id)
			d.mu.Unlock()
			if id == d.fail {
				n, _ := io.WriteString(w, "part")
				return int64(n), d.failErr
			}
			n, err := io.WriteString(w, "content of "+id)
			return int64(n), err
		},
	}
}

func (d *downloader) ids() []string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]string(nil), d.downloaded...)
}

// noDownload is an attachment service whose Download fails the test.
func noDownload(t *testing.T, attachments []*jira4claude.Attachment) *mock.AttachmentService {
	t.Helper()
	return &mock.AttachmentService{
		ListFn: func(context.Context, string) ([]*jira4claude.Attachment, error) { return attachments, nil },
		DownloadFn: func(context.Context, string, io.Writer) (int64, error) {
			t.Error("Download must not be called")
			return 0, errors.New("unexpected Download call")
		},
	}
}

// sized returns an attachment whose size matches the downloader's content.
func sized(id, filename string) *jira4claude.Attachment {
	return &jira4claude.Attachment{ID: id, Filename: filename, Size: int64(len("content of " + id))}
}

func dirEntries(t *testing.T, dir string) []string {
	t.Helper()
	des, err := os.ReadDir(dir)
	require.NoError(t, err)
	names := make([]string, 0, len(des))
	for _, de := range des {
		names = append(names, de.Name())
	}
	return names
}

func fileContent(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(data)
}

func TestRun_AttachmentDownload(t *testing.T) {
	t.Parallel()

	t.Run("--all saves every attachment and prints one path per line", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		d := &downloader{attachments: []*jira4claude.Attachment{sized("10000", "picture.jpg"), sized("10001", "debug log.txt")}}

		res := runCLI(t, attachmentDeps(t, d.service(t)), "attachment", "download", "TEST-1", "--all", "--dir", dir)

		assert.Equal(t, 0, res.code, "stderr: %q", res.stderr)
		assert.Equal(t, filepath.Join(dir, "picture.jpg")+"\n"+filepath.Join(dir, "debug log.txt")+"\n", res.stdout)
		assert.Empty(t, res.stderr)
		assert.Equal(t, "content of 10000", fileContent(t, filepath.Join(dir, "picture.jpg")))
		assert.Equal(t, "content of 10001", fileContent(t, filepath.Join(dir, "debug log.txt")))
		assert.ElementsMatch(t, []string{"picture.jpg", "debug log.txt"}, dirEntries(t, dir))
	})

	t.Run("--json prints an array of id, filename, path and size", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		d := &downloader{attachments: []*jira4claude.Attachment{sized("10000", "../picture.jpg")}}

		res := runCLI(t, attachmentDeps(t, d.service(t)), "--json", "attachment", "download", "TEST-1", "--all", "--dir", dir)

		assert.Equal(t, 0, res.code, "stderr: %q", res.stderr)
		var got []map[string]any
		require.NoError(t, json.Unmarshal([]byte(res.stdout), &got), "stdout: %q", res.stdout)
		assert.Equal(t, []map[string]any{{
			"id":       "10000",
			"filename": "../picture.jpg",
			"path":     filepath.Join(dir, "picture.jpg"),
			"size":     float64(len("content of 10000")),
		}}, got)
		assert.Empty(t, res.stderr)
	})

	t.Run("--id saves only that attachment", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		d := &downloader{attachments: []*jira4claude.Attachment{sized("10000", "a.txt"), sized("10001", "b.txt")}}

		res := runCLI(t, attachmentDeps(t, d.service(t)), "attachment", "download", "TEST-1", "--id", "10001", "--dir", dir)

		assert.Equal(t, 0, res.code, "stderr: %q", res.stderr)
		assert.Equal(t, filepath.Join(dir, "b.txt")+"\n", res.stdout)
		assert.Equal(t, []string{"10001"}, d.ids())
		assert.Equal(t, []string{"b.txt"}, dirEntries(t, dir))
	})

	t.Run("an --id that is not on the issue is not found and nothing is downloaded", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		svc := noDownload(t, []*jira4claude.Attachment{sized("10000", "a.txt")})

		res := runCLI(t, attachmentDeps(t, svc), "attachment", "download", "TEST-1", "--id", "99999", "--dir", dir)

		assert.Equal(t, 4, res.code)
		assert.Empty(t, res.stdout)
		assert.Equal(t, "[error] attachment 99999 is not on TEST-1\n", res.stderr)
		assert.Empty(t, dirEntries(t, dir))
	})

	t.Run("--name matches the exact filename Jira reports", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		d := &downloader{attachments: []*jira4claude.Attachment{sized("10000", "a\tb.txt"), sized("10001", "c.txt")}}

		res := runCLI(t, attachmentDeps(t, d.service(t)), "attachment", "download", "TEST-1", "--name", "a\tb.txt", "--dir", dir)

		assert.Equal(t, 0, res.code, "stderr: %q", res.stderr)
		assert.Equal(t, filepath.Join(dir, "a_b.txt")+"\n", res.stdout)
		assert.Equal(t, []string{"10000"}, d.ids())
	})

	t.Run("--name does not match the escaped form attachment list prints", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		svc := noDownload(t, []*jira4claude.Attachment{sized("10000", "a\tb.txt")})

		res := runCLI(t, attachmentDeps(t, svc), "attachment", "download", "TEST-1", "--name", `a\tb.txt`, "--dir", dir)

		assert.Equal(t, 4, res.code)
		assert.Empty(t, res.stdout)
		assert.Equal(t, "[error] no attachment of TEST-1 is named \"a\\\\tb.txt\"\n", res.stderr)
	})

	t.Run("--name matching several attachments is an error listing their IDs", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		svc := noDownload(t, []*jira4claude.Attachment{sized("10000", "a.txt"), sized("10001", "b.txt"), sized("10002", "a.txt")})

		res := runCLI(t, attachmentDeps(t, svc), "attachment", "download", "TEST-1", "--name", "a.txt", "--dir", dir)

		assert.Equal(t, 1, res.code)
		assert.Empty(t, res.stdout)
		assert.Equal(t, "[error] 2 attachments of TEST-1 are named \"a.txt\" (10000, 10002); use --id\n", res.stderr)
		assert.Empty(t, dirEntries(t, dir))
	})

	t.Run("hostile filenames are saved inside --dir under their base name", func(t *testing.T) {
		t.Parallel()

		parent := t.TempDir()
		dir := filepath.Join(parent, "dl")
		require.NoError(t, os.Mkdir(dir, 0o700))
		d := &downloader{attachments: []*jira4claude.Attachment{
			sized("1", "../../etc/passwd"),
			sized("2", "/abs/path"),
			sized("3", `..\win.txt`),
			sized("4", ".."),
			sized("5", ""),
			sized("6", "a\x00b\nc"),
			sized("7", "sub/"),
		}}

		res := runCLI(t, attachmentDeps(t, d.service(t)), "attachment", "download", "TEST-1", "--all", "--dir", dir)

		assert.Equal(t, 0, res.code, "stderr: %q", res.stderr)
		assert.ElementsMatch(t, []string{"passwd", "path", "win.txt", "attachment-4", "attachment-5", "a_b_c", "attachment-7"}, dirEntries(t, dir))
		assert.Equal(t, []string{"dl"}, dirEntries(t, parent), "nothing is written outside --dir")
		lines := strings.Split(strings.TrimSuffix(res.stdout, "\n"), "\n")
		assert.Len(t, lines, 7)
		for _, line := range lines {
			assert.Equal(t, dir, filepath.Dir(line))
		}
	})

	t.Run("two attachments with the same name are both saved, the second with its ID", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		d := &downloader{attachments: []*jira4claude.Attachment{sized("10000", "report.pdf"), sized("10001", "old/report.pdf")}}

		res := runCLI(t, attachmentDeps(t, d.service(t)), "attachment", "download", "TEST-1", "--all", "--dir", dir)

		assert.Equal(t, 0, res.code, "stderr: %q", res.stderr)
		assert.Equal(t, filepath.Join(dir, "report.pdf")+"\n"+filepath.Join(dir, "report-10001.pdf")+"\n", res.stdout)
		assert.Equal(t, "content of 10000", fileContent(t, filepath.Join(dir, "report.pdf")))
		assert.Equal(t, "content of 10001", fileContent(t, filepath.Join(dir, "report-10001.pdf")))
	})

	t.Run("an existing file is not overwritten without --overwrite", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		existing := filepath.Join(dir, "a.txt")
		require.NoError(t, os.WriteFile(existing, []byte("mine"), 0o600))
		svc := noDownload(t, []*jira4claude.Attachment{sized("10000", "a.txt")})

		res := runCLI(t, attachmentDeps(t, svc), "attachment", "download", "TEST-1", "--id", "10000", "--dir", dir)

		assert.Equal(t, 1, res.code)
		assert.Empty(t, res.stdout)
		assert.Equal(t, "[error] attachment 10000 (\"a.txt\"): "+existing+" already exists; use --overwrite to replace it\n", res.stderr)
		assert.Equal(t, "mine", fileContent(t, existing))
	})

	t.Run("--overwrite replaces an existing file", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		existing := filepath.Join(dir, "a.txt")
		require.NoError(t, os.WriteFile(existing, []byte("mine"), 0o600))
		d := &downloader{attachments: []*jira4claude.Attachment{sized("10000", "a.txt")}}

		res := runCLI(t, attachmentDeps(t, d.service(t)), "attachment", "download", "TEST-1", "--id", "10000", "--dir", dir, "--overwrite")

		assert.Equal(t, 0, res.code, "stderr: %q", res.stderr)
		assert.Equal(t, "content of 10000", fileContent(t, existing))
	})

	t.Run("--all stops at a failure, keeps and prints what was saved, and exits non-zero", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		d := &downloader{
			attachments: []*jira4claude.Attachment{sized("1", "a.txt"), sized("2", "b.txt"), sized("3", "c.txt")},
			fail:        "2",
			failErr:     &jira4claude.Error{Code: jira4claude.EForbidden, Message: "no access"},
		}

		res := runCLI(t, attachmentDeps(t, d.service(t)), "attachment", "download", "TEST-1", "--all", "--dir", dir)

		assert.Equal(t, 3, res.code)
		assert.Equal(t, filepath.Join(dir, "a.txt")+"\n", res.stdout)
		assert.Equal(t, "[error] attachment 2 (\"b.txt\"): no access\n", res.stderr)
		assert.Equal(t, []string{"a.txt"}, dirEntries(t, dir), "no partial or temporary file is left")
		assert.Equal(t, []string{"1", "2"}, d.ids())
	})

	t.Run("--all failure part-way in --json mode keeps stdout one array", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		d := &downloader{
			attachments: []*jira4claude.Attachment{sized("1", "a.txt"), sized("2", "b.txt")},
			fail:        "2",
			failErr:     errors.New("connection reset"),
		}

		res := runCLI(t, attachmentDeps(t, d.service(t)), "--json", "attachment", "download", "TEST-1", "--all", "--dir", dir)

		assert.Equal(t, 7, res.code)
		var got []map[string]any
		require.NoError(t, json.Unmarshal([]byte(res.stdout), &got), "stdout: %q", res.stdout)
		require.Len(t, got, 1)
		assert.Equal(t, "1", got[0]["id"])
		assertJSONError(t, res.stderr, jira4claude.EInternal, "attachment 2 (\"b.txt\"): connection reset")
	})

	t.Run("a size other than the one Jira reports is an error and no file is kept", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		d := &downloader{attachments: []*jira4claude.Attachment{{ID: "10000", Filename: "a.txt", Size: 999}}}

		res := runCLI(t, attachmentDeps(t, d.service(t)), "attachment", "download", "TEST-1", "--all", "--dir", dir)

		assert.Equal(t, 7, res.code)
		assert.Empty(t, res.stdout)
		assert.Contains(t, res.stderr, "but Jira reports 999")
		assert.Empty(t, dirEntries(t, dir))
	})

	t.Run("a name clash the ID cannot resolve fails before downloading", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		svc := noDownload(t, []*jira4claude.Attachment{sized("1", "a.txt"), sized("2", "a.txt"), sized("3", "a-2.txt")})

		res := runCLI(t, attachmentDeps(t, svc), "attachment", "download", "TEST-1", "--all", "--dir", dir)

		assert.Equal(t, 5, res.code)
		assert.Empty(t, res.stdout)
		assert.Contains(t, res.stderr, "attachments 2 and 3 would be saved under the same name")
		assert.Empty(t, dirEntries(t, dir))
	})
}

func TestRun_AttachmentDownloadFlags(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		args   []string
		stderr string
	}{
		{"one of --id, --name and --all is required", []string{"--dir", "."}, "one of --id, --name or --all is required"},
		{"--id and --all exclude each other", []string{"--id", "1", "--all", "--dir", "."}, "can't be used together"},
		{"--id and --name exclude each other", []string{"--id", "1", "--name", "a", "--dir", "."}, "can't be used together"},
		{"--name and --all exclude each other", []string{"--name", "a", "--all", "--dir", "."}, "can't be used together"},
		{"--dir is required", []string{"--all"}, "--dir"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			args := append([]string{"attachment", "download", "TEST-1"}, tc.args...)
			res := runCLI(t, unreachableDeps(t), args...)

			assert.Equal(t, 1, res.code)
			assert.Empty(t, res.stdout)
			assert.Contains(t, res.stderr, tc.stderr)
		})
	}

	t.Run("a --dir that does not exist is a validation error, is not created, and Jira is not asked", func(t *testing.T) {
		t.Parallel()

		missing := filepath.Join(t.TempDir(), "missing", "deeper")
		deps := attachmentDeps(t, &mock.AttachmentService{
			ListFn: func(context.Context, string) ([]*jira4claude.Attachment, error) {
				t.Error("List must not be called")
				return nil, nil
			},
		})

		res := runCLI(t, deps, "attachment", "download", "TEST-1", "--all", "--dir", missing)

		assert.Equal(t, 1, res.code)
		assert.Empty(t, res.stdout)
		assert.Contains(t, res.stderr, "does not exist")
		assert.NoDirExists(t, filepath.Dir(missing))
	})

	t.Run("a --dir that is a file is a validation error", func(t *testing.T) {
		t.Parallel()

		file := filepath.Join(t.TempDir(), "file")
		require.NoError(t, os.WriteFile(file, nil, 0o600))

		res := runCLI(t, attachmentDeps(t, noDownload(t, nil)), "attachment", "download", "TEST-1", "--all", "--dir", file)

		assert.Equal(t, 1, res.code)
		assert.Contains(t, res.stderr, "not a directory")
	})
}
