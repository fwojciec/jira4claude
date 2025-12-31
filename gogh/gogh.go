// Package gogh provides terminal-aware I/O using go-gh terminal detection.
package gogh

import (
	"io"
	"os"

	"github.com/cli/go-gh/v2/pkg/term"
)

// IO encapsulates terminal-aware I/O.
type IO struct {
	Out        io.Writer
	Err        io.Writer
	IsTerminal bool
	ServerURL  string
}

// NewIO creates IO with the given writers.
// Terminal detection is derived from out.
func NewIO(out, err io.Writer) *IO {
	return &IO{
		Out:        out,
		Err:        err,
		IsTerminal: isTerminal(out),
	}
}

// isTerminal checks if w is a terminal (file descriptor check).
func isTerminal(w io.Writer) bool {
	if f, ok := w.(*os.File); ok {
		return term.IsTerminal(f)
	}
	return false
}
