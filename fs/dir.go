// Package fs saves downloaded files to the local file system.
//
// It is the file-system dependency of the application, kept out of the root
// package as the HTTP client is: the root decides which name a file gets
// (jira4claude.SafeFilename), this package decides how it is written safely.
package fs

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	iofs "io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/fwojciec/jira4claude"
)

// maxNameBytes is the longest file name most file systems accept.
const maxNameBytes = 255

// Dir is a directory that files are saved into. Every operation goes through
// an os.Root, so no name can reach outside the directory, whether through
// "..", an absolute path or a symlink.
type Dir struct {
	root      *os.Root
	path      string // absolute
	overwrite bool

	// saved holds the files placed through this Dir, so that --overwrite
	// never replaces one of them, even under another name that the file
	// system treats as equal (case or Unicode normalisation on macOS).
	saved []iofs.FileInfo

	// link and rename are root.Link and root.Rename; tests replace them to
	// take the path for file systems without hard links (export_test.go).
	link   func(oldname, newname string) error
	rename func(oldname, newname string) error
}

// OpenDir opens an existing directory for saving. It never creates the
// directory: a missing path or a path that is not a directory is a
// validation error. With overwrite, Save may replace an existing regular
// file; without it, an existing name is a validation error.
func OpenDir(path string, overwrite bool) (*Dir, error) {
	if path == "" {
		return nil, &jira4claude.Error{Code: jira4claude.EValidation, Message: "a download directory is required"}
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, &jira4claude.Error{Code: jira4claude.EValidation, Message: "invalid download directory " + path, Inner: err}
	}
	info, err := os.Stat(abs)
	switch {
	case errors.Is(err, iofs.ErrNotExist):
		return nil, &jira4claude.Error{Code: jira4claude.EValidation, Message: "download directory does not exist: " + path}
	case err != nil:
		return nil, &jira4claude.Error{Code: jira4claude.EValidation, Message: "cannot use download directory " + path, Inner: err}
	case !info.IsDir():
		return nil, &jira4claude.Error{Code: jira4claude.EValidation, Message: "download directory is not a directory: " + path}
	}
	root, err := os.OpenRoot(abs)
	if err != nil {
		return nil, &jira4claude.Error{Code: jira4claude.EValidation, Message: "cannot open download directory " + path, Inner: err}
	}
	return &Dir{root: root, path: abs, overwrite: overwrite, link: root.Link, rename: root.Rename}, nil
}

// Close releases the directory.
func (d *Dir) Close() error {
	return d.root.Close()
}

// Save writes a file named name in the directory and returns its absolute
// path and the number of bytes written. fill writes the content; it runs
// against a temporary file in the directory, which is moved to name only
// when fill succeeds and the byte count matches size, so a failed or
// interrupted download never leaves a partial file under name. size is the
// expected byte count; 0 or less means unknown and is not checked.
//
// name must be a plain base name (as jira4claude.SafeFilename returns).
// Without overwrite, an existing name of any kind is a validation error, both
// before fill runs and atomically when the file is put in place, so a file
// or symlink created meanwhile is not replaced. With overwrite, only a
// regular file is replaced; a symlink, directory or other entry is refused.
func (d *Dir) Save(name string, size int64, fill func(io.Writer) error) (path string, n int64, err error) {
	if err := validName(name); err != nil {
		return "", 0, err
	}
	if err := d.check(name); err != nil {
		return "", 0, err
	}

	tmp, f, err := d.createTemp()
	if err != nil {
		return "", 0, err
	}
	defer func() {
		// Error ignored: after a successful link the temporary name is
		// redundant; after a failure, the failure is what matters.
		_ = d.root.Remove(tmp)
	}()

	cw := &countingWriter{w: f}
	fillErr := fill(cw)
	closeErr := f.Close()
	switch {
	case fillErr != nil:
		return "", cw.n, fillErr
	case closeErr != nil:
		return "", cw.n, &jira4claude.Error{Code: jira4claude.EInternal, Message: "failed to write " + name, Inner: closeErr}
	case size > 0 && cw.n != size:
		return "", cw.n, &jira4claude.Error{
			Code:    jira4claude.EInternal,
			Message: fmt.Sprintf("downloaded %d bytes for %s, but Jira reports %d", cw.n, name, size),
		}
	}

	if err := d.place(tmp, name); err != nil {
		return "", cw.n, err
	}
	d.remember(name)
	return filepath.Join(d.path, name), cw.n, nil
}

// check refuses a name that Save must not write to.
func (d *Dir) check(name string) error {
	info, err := d.root.Lstat(name)
	if errors.Is(err, iofs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return &jira4claude.Error{Code: jira4claude.EInternal, Message: "cannot check " + d.display(name), Inner: err}
	}
	if !d.overwrite {
		return existsError(d.display(name))
	}
	if !info.Mode().IsRegular() {
		return &jira4claude.Error{
			Code:    jira4claude.EValidation,
			Message: d.display(name) + " exists and is not a regular file; refusing to replace it even with --overwrite",
		}
	}
	for _, s := range d.saved {
		if os.SameFile(s, info) {
			return &jira4claude.Error{
				Code:    jira4claude.EConflict,
				Message: d.display(name) + " was saved earlier in this run; refusing to replace it",
			}
		}
	}
	return nil
}

// remember records a file placed under name, for check.
func (d *Dir) remember(name string) {
	// Error ignored: the file was just placed; if it cannot be read back,
	// the only loss is this extra guard for it.
	if info, err := d.root.Lstat(name); err == nil {
		d.saved = append(d.saved, info)
	}
}

// place moves the temporary file to name. Without overwrite it never
// replaces anything: a hard link fails if the name exists in any form. With
// overwrite, the regular file that check accepted is replaced by a rename,
// which replaces a directory entry and never follows a symlink.
//
// On a file system without hard links (FAT, exFAT, some network file
// systems), the name is reserved by creating it with O_CREATE|O_EXCL, which
// fails on any existing entry and never follows a symlink, and the complete
// temporary file is then renamed over the reservation. The standard library
// has no portable rename that refuses an existing target (Linux
// renameat2(RENAME_NOREPLACE) and macOS renamex_np(RENAME_EXCL) need
// golang.org/x/sys), so the reservation cannot be avoided: an empty file
// exists under name between the two calls, with no network I/O in between;
// a process killed exactly there leaves that empty file. A failed rename
// removes the reservation, if it is still the file created here.
func (d *Dir) place(tmp, name string) error {
	if d.overwrite {
		if err := d.check(name); err != nil {
			return err
		}
		if err := d.rename(tmp, name); err != nil {
			return &jira4claude.Error{Code: jira4claude.EInternal, Message: "failed to save " + d.display(name), Inner: err}
		}
		return nil
	}

	err := d.link(tmp, name)
	if err == nil {
		return nil
	}
	if errors.Is(err, iofs.ErrExist) {
		return existsError(d.display(name))
	}

	reserved, err := d.root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if errors.Is(err, iofs.ErrExist) {
		return existsError(d.display(name))
	}
	if err != nil {
		return &jira4claude.Error{Code: jira4claude.EInternal, Message: "failed to save " + d.display(name), Inner: err}
	}
	reservation, statErr := reserved.Stat()
	// Error ignored: the file is empty and only reserves the name.
	_ = reserved.Close()
	if err := d.rename(tmp, name); err != nil {
		if current, err := d.root.Lstat(name); statErr == nil && err == nil && os.SameFile(reservation, current) {
			// Error ignored: the rename failure is what is reported.
			_ = d.root.Remove(name)
		}
		return &jira4claude.Error{Code: jira4claude.EInternal, Message: "failed to save " + d.display(name), Inner: err}
	}
	return nil
}

// createTemp creates a new temporary file in the directory, readable only by
// the user.
func (d *Dir) createTemp() (string, *os.File, error) {
	var lastErr error
	for range 10 {
		name := ".j4c-" + rand.Text() + ".part"
		f, err := d.root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if err == nil {
			return name, f, nil
		}
		lastErr = err
		if !errors.Is(err, iofs.ErrExist) {
			break
		}
	}
	return "", nil, &jira4claude.Error{Code: jira4claude.EInternal, Message: "cannot create a temporary file in " + d.path, Inner: lastErr}
}

// display is the path of name for messages.
func (d *Dir) display(name string) string {
	return filepath.Join(d.path, name)
}

func existsError(path string) error {
	return &jira4claude.Error{
		Code:    jira4claude.EValidation,
		Message: path + " already exists; use --overwrite to replace it",
	}
}

// validName refuses anything but a plain base name. Names come from
// jira4claude.SafeFilename, so a refusal here is a programming error; the
// check keeps the directory boundary independent of that function.
func validName(name string) error {
	if name == "" || name == "." || name == ".." || len(name) > maxNameBytes ||
		strings.ContainsAny(name, "/\\\x00") || filepath.Base(name) != name {
		return &jira4claude.Error{Code: jira4claude.EInternal, Message: fmt.Sprintf("refusing unsafe file name %q", name)}
	}
	return nil
}

// countingWriter counts the bytes written through it.
type countingWriter struct {
	w io.Writer
	n int64
}

func (c *countingWriter) Write(p []byte) (int, error) {
	n, err := c.w.Write(p)
	c.n += int64(n)
	return n, err
}
