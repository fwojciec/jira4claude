package fs_test

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fwojciec/jira4claude"
	"github.com/fwojciec/jira4claude/fs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// openDir opens dir for saving and closes it when the test ends.
func openDir(t *testing.T, dir string, overwrite bool) *fs.Dir {
	t.Helper()
	d, err := fs.OpenDir(dir, overwrite)
	require.NoError(t, err)
	t.Cleanup(func() { _ = d.Close() })
	return d
}

// content returns a fill function writing s.
func content(s string) func(io.Writer) error {
	return func(w io.Writer) error {
		_, err := io.WriteString(w, s)
		return err
	}
}

// entries returns the names in dir.
func entries(t *testing.T, dir string) []string {
	t.Helper()
	des, err := os.ReadDir(dir)
	require.NoError(t, err)
	names := make([]string, 0, len(des))
	for _, de := range des {
		names = append(names, de.Name())
	}
	return names
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(data)
}

func TestOpenDir(t *testing.T) {
	t.Parallel()

	t.Run("a missing directory is a validation error and is not created", func(t *testing.T) {
		t.Parallel()

		missing := filepath.Join(t.TempDir(), "missing")

		_, err := fs.OpenDir(missing, false)

		require.Error(t, err)
		assert.Equal(t, jira4claude.EValidation, jira4claude.ErrorCode(err))
		assert.Contains(t, err.Error(), "does not exist")
		assert.NoDirExists(t, missing)
	})

	t.Run("a file is a validation error", func(t *testing.T) {
		t.Parallel()

		file := filepath.Join(t.TempDir(), "file")
		require.NoError(t, os.WriteFile(file, nil, 0o600))

		_, err := fs.OpenDir(file, false)

		require.Error(t, err)
		assert.Equal(t, jira4claude.EValidation, jira4claude.ErrorCode(err))
		assert.Contains(t, err.Error(), "not a directory")
	})

	t.Run("an empty path is a validation error", func(t *testing.T) {
		t.Parallel()

		_, err := fs.OpenDir("", false)

		require.Error(t, err)
		assert.Equal(t, jira4claude.EValidation, jira4claude.ErrorCode(err))
	})
}

func TestDir_Save(t *testing.T) {
	t.Parallel()

	t.Run("writes the content under the name and returns the absolute path", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		d := openDir(t, dir, false)

		path, n, err := d.Save("report.pdf", 5, content("hello"))

		require.NoError(t, err)
		assert.Equal(t, filepath.Join(dir, "report.pdf"), path)
		assert.True(t, filepath.IsAbs(path))
		assert.Equal(t, int64(5), n)
		assert.Equal(t, "hello", readFile(t, path))
		assert.Equal(t, []string{"report.pdf"}, entries(t, dir), "no temporary file is left")
	})

	t.Run("a relative directory gives an absolute path", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		wd, err := os.Getwd()
		require.NoError(t, err)
		rel, err := filepath.Rel(wd, dir)
		require.NoError(t, err)
		d := openDir(t, rel, false)

		path, _, err := d.Save("a.txt", 0, content("x"))

		require.NoError(t, err)
		assert.Equal(t, filepath.Join(dir, "a.txt"), path)
	})

	t.Run("an unknown size (0) is not checked", func(t *testing.T) {
		t.Parallel()

		d := openDir(t, t.TempDir(), false)

		_, n, err := d.Save("a.txt", 0, content("abc"))

		require.NoError(t, err)
		assert.Equal(t, int64(3), n)
	})

	t.Run("an existing file is refused before downloading and left unchanged", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		existing := filepath.Join(dir, "a.txt")
		require.NoError(t, os.WriteFile(existing, []byte("original"), 0o600))
		d := openDir(t, dir, false)
		called := false

		_, _, err := d.Save("a.txt", 0, func(w io.Writer) error {
			called = true
			return content("new")(w)
		})

		require.Error(t, err)
		assert.Equal(t, jira4claude.EValidation, jira4claude.ErrorCode(err))
		assert.Contains(t, err.Error(), "--overwrite")
		assert.False(t, called, "nothing is downloaded for a file that exists")
		assert.Equal(t, "original", readFile(t, existing))
		assert.Equal(t, []string{"a.txt"}, entries(t, dir))
	})

	t.Run("a file that appears during the download is not replaced", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		existing := filepath.Join(dir, "a.txt")
		d := openDir(t, dir, false)

		_, _, err := d.Save("a.txt", 0, func(w io.Writer) error {
			require.NoError(t, os.WriteFile(existing, []byte("raced"), 0o600))
			return content("new")(w)
		})

		require.Error(t, err)
		assert.Equal(t, jira4claude.EValidation, jira4claude.ErrorCode(err))
		assert.Equal(t, "raced", readFile(t, existing))
		assert.Equal(t, []string{"a.txt"}, entries(t, dir), "the temporary file is removed")
	})

	t.Run("a symlink at the name is not followed", func(t *testing.T) {
		t.Parallel()

		for _, overwrite := range []bool{false, true} {
			dir := t.TempDir()
			outside := filepath.Join(t.TempDir(), "victim")
			require.NoError(t, os.WriteFile(outside, []byte("precious"), 0o600))
			require.NoError(t, os.Symlink(outside, filepath.Join(dir, "a.txt")))
			d := openDir(t, dir, overwrite)

			_, _, err := d.Save("a.txt", 0, content("evil"))

			require.Error(t, err, "overwrite=%v", overwrite)
			assert.Equal(t, jira4claude.EValidation, jira4claude.ErrorCode(err))
			assert.Equal(t, "precious", readFile(t, outside), "overwrite=%v", overwrite)
			target, err := os.Readlink(filepath.Join(dir, "a.txt"))
			require.NoError(t, err)
			assert.Equal(t, outside, target, "the symlink itself stays")
		}
	})

	t.Run("a dangling symlink at the name is not followed", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		outside := filepath.Join(t.TempDir(), "created-by-attack")
		require.NoError(t, os.Symlink(outside, filepath.Join(dir, "a.txt")))
		d := openDir(t, dir, false)

		_, _, err := d.Save("a.txt", 0, content("evil"))

		require.Error(t, err)
		assert.NoFileExists(t, outside)
	})

	t.Run("a symlink placed during the download is not followed", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		outside := filepath.Join(t.TempDir(), "victim")
		require.NoError(t, os.WriteFile(outside, []byte("precious"), 0o600))
		d := openDir(t, dir, false)

		_, _, err := d.Save("a.txt", 0, func(w io.Writer) error {
			require.NoError(t, os.Symlink(outside, filepath.Join(dir, "a.txt")))
			return content("evil")(w)
		})

		require.Error(t, err)
		assert.Equal(t, "precious", readFile(t, outside))
	})

	t.Run("--overwrite replaces a regular file", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		existing := filepath.Join(dir, "a.txt")
		require.NoError(t, os.WriteFile(existing, []byte("original"), 0o600))
		d := openDir(t, dir, true)

		path, _, err := d.Save("a.txt", 0, content("new"))

		require.NoError(t, err)
		assert.Equal(t, existing, path)
		assert.Equal(t, "new", readFile(t, existing))
		assert.Equal(t, []string{"a.txt"}, entries(t, dir))
	})

	t.Run("--overwrite refuses a directory", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		require.NoError(t, os.Mkdir(filepath.Join(dir, "a.txt"), 0o700))
		d := openDir(t, dir, true)

		_, _, err := d.Save("a.txt", 0, content("new"))

		require.Error(t, err)
		assert.Equal(t, jira4claude.EValidation, jira4claude.ErrorCode(err))
		assert.DirExists(t, filepath.Join(dir, "a.txt"))
	})

	t.Run("--overwrite on a missing file saves it", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		d := openDir(t, dir, true)

		path, _, err := d.Save("a.txt", 0, content("new"))

		require.NoError(t, err)
		assert.Equal(t, "new", readFile(t, path))
	})

	t.Run("a failed download leaves no file", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		d := openDir(t, dir, false)
		boom := errors.New("connection reset")

		_, n, err := d.Save("a.txt", 0, func(w io.Writer) error {
			_, _ = io.WriteString(w, "part")
			return boom
		})

		require.ErrorIs(t, err, boom)
		assert.Equal(t, int64(4), n)
		assert.Empty(t, entries(t, dir), "neither the final nor the temporary file remains")
	})

	t.Run("a failed download under --overwrite keeps the old file", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		existing := filepath.Join(dir, "a.txt")
		require.NoError(t, os.WriteFile(existing, []byte("original"), 0o600))
		d := openDir(t, dir, true)

		_, _, err := d.Save("a.txt", 0, func(w io.Writer) error {
			_, _ = io.WriteString(w, "part")
			return errors.New("connection reset")
		})

		require.Error(t, err)
		assert.Equal(t, "original", readFile(t, existing))
		assert.Equal(t, []string{"a.txt"}, entries(t, dir))
	})

	t.Run("a size other than the reported one is an error and leaves no file", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		d := openDir(t, dir, false)

		_, n, err := d.Save("a.txt", 10, content("short"))

		require.Error(t, err)
		assert.Equal(t, int64(5), n)
		assert.Contains(t, err.Error(), "5 bytes")
		assert.Contains(t, err.Error(), "10")
		assert.Empty(t, entries(t, dir))
	})

	t.Run("a name that is not a plain base name is refused and nothing is written", func(t *testing.T) {
		t.Parallel()

		for _, name := range []string{"", ".", "..", "../a.txt", "a/b.txt", "/abs", `..\x`, "a\x00b", strings.Repeat("a", 256)} {
			parent := t.TempDir()
			dir := filepath.Join(parent, "dir")
			require.NoError(t, os.Mkdir(dir, 0o700))
			d := openDir(t, dir, true)
			called := false

			_, _, err := d.Save(name, 0, func(w io.Writer) error {
				called = true
				return content("evil")(w)
			})

			require.Error(t, err, "name %q", name)
			assert.False(t, called, "name %q", name)
			assert.Empty(t, entries(t, dir), "name %q", name)
			assert.Equal(t, []string{"dir"}, entries(t, parent), "name %q", name)
		}
	})
}
