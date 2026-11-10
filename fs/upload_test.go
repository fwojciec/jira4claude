package fs_test

import (
	"io"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/fwojciec/jira4claude"
	"github.com/fwojciec/jira4claude/fs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeFile writes content to name in dir and returns its path.
func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	return path
}

// readAll reads what file.Open returns.
func readAll(t *testing.T, file jira4claude.AttachmentFile) string {
	t.Helper()
	rc, err := file.Open()
	require.NoError(t, err)
	defer func() { _ = rc.Close() }()
	data, err := io.ReadAll(rc)
	require.NoError(t, err)
	return string(data)
}

// openUploads opens paths, which must pass the checks, and closes them when
// the test ends.
func openUploads(t *testing.T, paths ...string) *fs.Uploads {
	t.Helper()
	u, err := fs.OpenUploads(paths)
	require.NoError(t, err)
	t.Cleanup(func() { _ = u.Close() })
	return u
}

// assertRefused checks that OpenUploads refuses paths with a validation error
// whose message is msg.
func assertRefused(t *testing.T, msg string, paths ...string) {
	t.Helper()
	u, err := fs.OpenUploads(paths)
	require.Error(t, err)
	assert.Nil(t, u)
	assert.Equal(t, jira4claude.EValidation, jira4claude.ErrorCode(err))
	assert.Equal(t, msg, err.Error())
}

func TestOpenUploads(t *testing.T) {
	t.Parallel()

	t.Run("each file is sent under its base name with its size and content, in order", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		a := writeFile(t, dir, "a.txt", "alpha")
		require.NoError(t, os.Mkdir(filepath.Join(dir, "sub"), 0o700))
		b := writeFile(t, filepath.Join(dir, "sub"), "b log.txt", "bravo!")

		u := openUploads(t, a, b)

		require.Len(t, u.Files, 2)
		assert.Equal(t, []string{a, b}, u.Paths)
		assert.Equal(t, "a.txt", u.Files[0].Name)
		assert.Equal(t, int64(5), u.Files[0].Size)
		assert.Equal(t, "alpha", readAll(t, u.Files[0]))
		assert.Equal(t, "b log.txt", u.Files[1].Name)
		assert.Equal(t, int64(6), u.Files[1].Size)
		assert.Equal(t, "bravo!", readAll(t, u.Files[1]))
	})

	t.Run("Open gives the whole content again on every call", func(t *testing.T) {
		t.Parallel()

		u := openUploads(t, writeFile(t, t.TempDir(), "a.txt", "alpha"))

		assert.Equal(t, "alpha", readAll(t, u.Files[0]))
		assert.Equal(t, "alpha", readAll(t, u.Files[0]))
	})

	t.Run("the content is that of the file checked, cut to the size checked", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		path := writeFile(t, dir, "a.txt", "alpha")
		u := openUploads(t, path)

		// Replacing the path afterwards does not change what is sent; growing
		// the file does not either.
		require.NoError(t, os.Rename(path, filepath.Join(dir, "old.txt")))
		writeFile(t, dir, "a.txt", "something else")
		f, err := os.OpenFile(filepath.Join(dir, "old.txt"), os.O_APPEND|os.O_WRONLY, 0)
		require.NoError(t, err)
		_, err = f.WriteString(" and more")
		require.NoError(t, err)
		require.NoError(t, f.Close())

		assert.Equal(t, "alpha", readAll(t, u.Files[0]))
	})

	t.Run("an empty file is accepted", func(t *testing.T) {
		t.Parallel()

		u := openUploads(t, writeFile(t, t.TempDir(), "empty.txt", ""))

		assert.Equal(t, int64(0), u.Files[0].Size)
		assert.Empty(t, readAll(t, u.Files[0]))
	})

	t.Run("a symlink to a file is followed and sent under the link's name", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		target := writeFile(t, dir, "target.txt", "alpha")
		link := filepath.Join(dir, "link.txt")
		require.NoError(t, os.Symlink(target, link))

		u := openUploads(t, link)

		assert.Equal(t, "link.txt", u.Files[0].Name)
		assert.Equal(t, "alpha", readAll(t, u.Files[0]))
	})

	t.Run("two files with the same base name in different directories are both accepted", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		require.NoError(t, os.Mkdir(filepath.Join(dir, "x"), 0o700))
		require.NoError(t, os.Mkdir(filepath.Join(dir, "y"), 0o700))
		a := writeFile(t, filepath.Join(dir, "x"), "log.txt", "one")
		b := writeFile(t, filepath.Join(dir, "y"), "log.txt", "two")

		u := openUploads(t, a, b)

		assert.Equal(t, "log.txt", u.Files[0].Name)
		assert.Equal(t, "log.txt", u.Files[1].Name)
	})

	t.Run("a missing file is refused", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		ok := writeFile(t, dir, "a.txt", "alpha")
		missing := filepath.Join(dir, "missing.txt")

		assertRefused(t, `no such file: "`+missing+`"`, ok, missing)
	})

	t.Run("a dangling symlink is refused as missing", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		link := filepath.Join(dir, "link.txt")
		require.NoError(t, os.Symlink(filepath.Join(dir, "gone.txt"), link))

		assertRefused(t, `no such file: "`+link+`"`, link)
	})

	t.Run("a directory is refused, also through a symlink", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		sub := filepath.Join(dir, "sub")
		require.NoError(t, os.Mkdir(sub, 0o700))
		link := filepath.Join(dir, "link")
		require.NoError(t, os.Symlink(sub, link))

		assertRefused(t, `"`+sub+`" is a directory, not a file`, sub)
		assertRefused(t, `"`+link+`" is a directory, not a file`, link)
	})

	t.Run("a named pipe is refused without blocking", func(t *testing.T) {
		t.Parallel()

		fifo := filepath.Join(t.TempDir(), "pipe")
		require.NoError(t, syscall.Mkfifo(fifo, 0o600))

		assertRefused(t, `"`+fifo+`" is not a regular file`, fifo)
	})

	t.Run("an unreadable file is refused", func(t *testing.T) {
		t.Parallel()

		if os.Geteuid() == 0 {
			t.Skip("root reads a file without read permission")
		}
		path := writeFile(t, t.TempDir(), "secret.txt", "alpha")
		require.NoError(t, os.Chmod(path, 0o000))

		assertRefused(t, `cannot read "`+path+`": permission denied`, path)
	})

	t.Run("the same path given twice is refused", func(t *testing.T) {
		t.Parallel()

		path := writeFile(t, t.TempDir(), "a.txt", "alpha")

		assertRefused(t, `"`+path+`" is given twice`, path, path)
	})

	t.Run("two paths naming the same file are refused", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		path := writeFile(t, dir, "a.txt", "alpha")
		link := filepath.Join(dir, "link.txt")
		require.NoError(t, os.Symlink(path, link))
		dotted := dir + "/./a.txt"

		assertRefused(t, `"`+link+`" is the same file as "`+path+`"`, path, link)
		assertRefused(t, `"`+dotted+`" is the same file as "`+path+`"`, path, dotted)
	})

	t.Run("a base name holding a control character is refused", func(t *testing.T) {
		t.Parallel()

		path := writeFile(t, t.TempDir(), "a\nb.txt", "alpha")

		assertRefused(t, `the file name of "`+filepath.Dir(path)+`/a\nb.txt" holds a control character or invalid UTF-8; rename the file to upload it`, path)
	})

	t.Run("an empty path is refused", func(t *testing.T) {
		t.Parallel()

		assertRefused(t, "an empty path is not a file", "")
	})
}

func TestUploads_Close(t *testing.T) {
	t.Parallel()

	u, err := fs.OpenUploads([]string{writeFile(t, t.TempDir(), "a.txt", "alpha")})
	require.NoError(t, err)

	require.NoError(t, u.Close())

	rc, err := u.Files[0].Open()
	require.NoError(t, err, "Open only wraps the file")
	_, err = io.ReadAll(rc)
	assert.Error(t, err, "reading a closed file fails")
}
