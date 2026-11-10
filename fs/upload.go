package fs

import (
	"errors"
	"fmt"
	"io"
	iofs "io/fs"
	"os"
	"path/filepath"
	"unicode"
	"unicode/utf8"

	"github.com/fwojciec/jira4claude"
)

// Uploads is a set of local files opened for upload. The files stay open
// from the checks until Close, so what is sent is the content of the files
// that were checked, even if a path is replaced meanwhile.
type Uploads struct {
	// Files are the files to send, in the order given. Each one's Open
	// reads the held file from its first byte, up to the size it had when
	// it was checked.
	Files []jira4claude.AttachmentFile
	// Paths are the paths as given, in the same order.
	Paths []string

	files []*os.File
}

// OpenUploads checks and opens the files at paths for upload. Every path
// must name an existing, readable regular file; a symlink is followed, as
// any program reading a file the user named does, and the file is sent under
// the base name of the path given (the link's name, not the target's). An
// empty file is accepted. A missing or unreadable file, a directory or other
// non-regular file (a named pipe is refused before it is opened, so the
// check cannot block), a path given twice or two paths naming the same file,
// and a base name holding a control character or invalid UTF-8 are
// validation errors naming the path. Nothing is left open after an error.
func OpenUploads(paths []string) (*Uploads, error) {
	u := &Uploads{}
	infos := make([]iofs.FileInfo, 0, len(paths))
	for _, path := range paths {
		f, info, err := openUpload(path)
		if err == nil {
			err = sameAsEarlier(path, info, u.Paths, infos)
			if err != nil {
				// Error ignored: the file was only read.
				_ = f.Close()
			}
		}
		if err != nil {
			// Error ignored: the files were only read.
			_ = u.Close()
			return nil, err
		}
		u.files = append(u.files, f)
		u.Paths = append(u.Paths, path)
		infos = append(infos, info)
		size := info.Size()
		u.Files = append(u.Files, jira4claude.AttachmentFile{
			Name: filepath.Base(path),
			Size: size,
			Open: func() (io.ReadCloser, error) {
				return io.NopCloser(io.NewSectionReader(f, 0, size)), nil
			},
		})
	}
	return u, nil
}

// Close closes the files. It returns the first error.
func (u *Uploads) Close() error {
	var first error
	for _, f := range u.files {
		if err := f.Close(); err != nil && first == nil {
			first = err
		}
	}
	u.files = nil
	return first
}

// openUpload checks one path and opens it.
func openUpload(path string) (*os.File, iofs.FileInfo, error) {
	if path == "" {
		return nil, nil, invalid("an empty path is not a file")
	}
	if name := filepath.Base(path); !printable(name) {
		return nil, nil, invalid(fmt.Sprintf("the file name of %q holds a control character or invalid UTF-8; rename the file to upload it", path))
	}

	// Stat before opening: opening a named pipe would block.
	info, err := os.Stat(path)
	if err != nil {
		return nil, nil, statError(path, err)
	}
	if err := regular(path, info); err != nil {
		return nil, nil, err
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, nil, statError(path, err)
	}
	opened, err := f.Stat()
	if err == nil && !os.SameFile(info, opened) {
		err = invalid(fmt.Sprintf("%q changed while it was being checked", path))
	} else if err == nil {
		err = regular(path, opened)
	} else {
		err = statError(path, err)
	}
	if err != nil {
		// Error ignored: the file was only opened.
		_ = f.Close()
		return nil, nil, err
	}
	return f, opened, nil
}

// sameAsEarlier refuses a file that an earlier path already names.
func sameAsEarlier(path string, info iofs.FileInfo, paths []string, infos []iofs.FileInfo) error {
	for i, earlier := range infos {
		switch {
		case paths[i] == path:
			return invalid(fmt.Sprintf("%q is given twice", path))
		case os.SameFile(earlier, info):
			return invalid(fmt.Sprintf("%q is the same file as %q", path, paths[i]))
		}
	}
	return nil
}

// regular refuses anything but a regular file.
func regular(path string, info iofs.FileInfo) error {
	switch {
	case info.IsDir():
		return invalid(fmt.Sprintf("%q is a directory, not a file", path))
	case !info.Mode().IsRegular():
		return invalid(fmt.Sprintf("%q is not a regular file", path))
	}
	return nil
}

// statError reports a failure to stat or open path, with the system's reason
// but without repeating the path.
func statError(path string, err error) error {
	if errors.Is(err, iofs.ErrNotExist) {
		return invalid(fmt.Sprintf("no such file: %q", path))
	}
	var pathErr *iofs.PathError
	if errors.As(err, &pathErr) {
		err = pathErr.Err
	}
	return &jira4claude.Error{Code: jira4claude.EValidation, Message: fmt.Sprintf("cannot read %q", path), Inner: err}
}

// printable reports whether name is valid UTF-8 without control or invisible
// formatting characters, so that it is sent and reported exactly.
func printable(name string) bool {
	if !utf8.ValidString(name) {
		return false
	}
	for _, r := range name {
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
			return false
		}
	}
	return true
}

func invalid(msg string) error {
	return &jira4claude.Error{Code: jira4claude.EValidation, Message: msg}
}
