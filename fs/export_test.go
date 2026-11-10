package fs

import (
	"errors"
	"io"
)

// WithoutHardLinks makes d behave as on a file system without hard links,
// so that Save takes its fallback placement path.
func WithoutHardLinks(d *Dir) {
	d.link = func(string, string) error { return errors.ErrUnsupported }
}

// AfterReserve makes the fallback path call f right after it has created
// the reservation under the final name.
func AfterReserve(d *Dir, f func()) {
	d.afterReserve = f
}

// FailCopy makes the fallback path's copy into the reservation fail with
// err after writing a few bytes.
func FailCopy(d *Dir, err error) {
	d.copy = func(dst io.Writer, _ io.Reader) (int64, error) {
		n, _ := io.WriteString(dst, "par")
		return int64(n), err
	}
}
