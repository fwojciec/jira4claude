package fs

import "errors"

// WithoutHardLinks makes d behave as on a file system without hard links,
// so that Save takes its fallback placement path.
func WithoutHardLinks(d *Dir) {
	d.link = func(string, string) error { return errors.ErrUnsupported }
}

// FailRename makes every rename in d fail with err.
func FailRename(d *Dir, err error) {
	d.rename = func(string, string) error { return err }
}
