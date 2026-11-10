package main

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"reflect"
	"strings"
	"unicode/utf8"

	"github.com/alecthomas/kong"
	"github.com/fwojciec/jira4claude"
)

// maxInputBytes bounds the text read from a file or standard input. It is a
// guard against the wrong file or an endless stream, far above any text Jira
// accepts.
const maxInputBytes = 1 << 20

// stdinPath is the file flag value that means standard input.
const stdinPath = "-"

// utf8BOM is the byte order mark some editors put at the start of a UTF-8 file.
const utf8BOM = "\uFEFF"

// textInput is a text given inline (--description) or by a file flag
// (--description-file). Kong keeps the two apart (xor), so at most one is set.
type textInput struct {
	inlineFlag string  // the inline flag's name, e.g. "description"
	inline     *string // its value, nil when not given
	fileFlag   string  // the file flag's name, e.g. "description-file"
	path       *string // its value, nil when not given
}

// read returns the text and the flag it came from, as an error message names
// it ("--description" or "--description-file PATH"); text is nil when neither
// flag was given. A file is read as described in readFile; inline text must
// be valid UTF-8, as a file must.
func (in textInput) read(stdin io.Reader) (text *string, source string, err error) {
	if in.path == nil {
		if in.inline != nil {
			if err := checkUTF8("--"+in.inlineFlag, *in.inline); err != nil {
				return nil, "", err
			}
		}
		return in.inline, "--" + in.inlineFlag, nil
	}
	s, err := readFile(in.fileFlag, *in.path, stdin)
	if err != nil {
		return nil, "", err
	}
	source = "--" + in.fileFlag + " " + *in.path
	if *in.path == stdinPath {
		source += " (standard input)"
	}
	return &s, source, nil
}

// readFile reads the text of the file flag named flag: the file at path, or
// stdin for "-". A leading UTF-8 byte order mark and exactly one trailing
// newline (LF or CRLF) are removed, the newline that ends the last line of a
// text file and that issue view --field adds; anything else is the text, for
// the converter. Bytes that are not UTF-8, more than maxInputBytes, a
// directory or a file that cannot be read are validation errors naming the
// path.
func readFile(flag, path string, stdin io.Reader) (string, error) {
	name := "--" + flag + " " + path
	var data []byte
	var err error
	switch path {
	case "":
		return "", invalid("--" + flag + " needs a file path, or - for standard input")
	case stdinPath:
		name += " (standard input)"
		if stdin == nil {
			stdin = strings.NewReader("")
		}
		data, err = readBounded(stdin)
		if err != nil {
			return "", invalid(fmt.Sprintf("cannot read %s: %v", name, err))
		}
	default:
		data, err = readPath(path)
		if err != nil {
			var pathErr *fs.PathError
			if errors.As(err, &pathErr) {
				err = pathErr.Err
			}
			if errors.Is(err, errIsDir) {
				return "", invalid(name + " is a directory")
			}
			return "", invalid(fmt.Sprintf("cannot read %s: %v", name, err))
		}
	}
	if len(data) > maxInputBytes {
		return "", invalid(name + " is larger than 1 MiB")
	}
	text := strings.TrimPrefix(string(data), utf8BOM)
	if !utf8.ValidString(text) {
		// The offset counts from the start of the file, byte order mark
		// included.
		return "", checkUTF8(name, string(data))
	}
	if s, ok := strings.CutSuffix(text, "\r\n"); ok {
		return s, nil
	}
	return strings.TrimSuffix(text, "\n"), nil
}

// errIsDir reports a path that names a directory.
var errIsDir = errors.New("is a directory") //nolint:gochecknoglobals // sentinel error

// readPath reads the file at path, refusing a directory, through readBounded.
func readPath(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close() //nolint:errcheck // read only; a close error loses nothing
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if info.IsDir() {
		return nil, errIsDir
	}
	return readBounded(f)
}

// readBounded reads r up to one byte past maxInputBytes, so that a caller can
// tell an oversized input without reading all of it.
func readBounded(r io.Reader) ([]byte, error) {
	return io.ReadAll(io.LimitReader(r, maxInputBytes+1))
}

// checkUTF8 refuses text that is not valid UTF-8, naming where it came from
// (a flag, or a flag and its path) and the offset of the first bad byte.
// Text sent to Jira as JSON would otherwise have each bad byte replaced by
// U+FFFD without a word.
func checkUTF8(name, text string) error {
	if utf8.ValidString(text) {
		return nil
	}
	return invalid(fmt.Sprintf("%s is not valid UTF-8 (first invalid byte at offset %d)", name, invalidOffset(text)))
}

// rawText decodes the value of a flag whose text goes to Jira (--summary,
// --description, --append-description, --body) as the bytes given, into a
// string or *string field. kong's own string mapper passes every value
// through JSON, which replaces bytes that are not UTF-8 by U+FFFD; the
// command then could not refuse them, as checkUTF8 does for a file.
func rawText() kong.MapperFunc {
	return func(ctx *kong.DecodeContext, target reflect.Value) error {
		token, err := ctx.Scan.PopValue("text")
		if err != nil {
			return err
		}
		s, ok := token.Value.(string)
		if !ok {
			return fmt.Errorf("expected text but got %v (%T)", token.Value, token.Value)
		}
		if target.Kind() == reflect.Pointer {
			v := reflect.New(target.Type().Elem())
			v.Elem().SetString(s)
			target.Set(v)
			return nil
		}
		target.SetString(s)
		return nil
	}
}

// textFlags returns the kong options that decode the text flags of cli with
// rawText.
func textFlags(cli *CLI) []kong.Option {
	fields := []any{
		&cli.Issue.Create.Summary, &cli.Issue.Create.Description,
		&cli.Issue.Update.Summary, &cli.Issue.Update.Description, &cli.Issue.Update.AppendDescription,
		&cli.Issue.Comment.Body,
	}
	opts := make([]kong.Option, 0, len(fields))
	for _, f := range fields {
		opts = append(opts, kong.ValueMapper(f, rawText()))
	}
	return opts
}

// invalidOffset returns the offset in s of the first byte that does not start
// a valid UTF-8 sequence.
func invalidOffset(s string) int {
	for offset, r := range s {
		if r == utf8.RuneError {
			if _, size := utf8.DecodeRuneInString(s[offset:]); size == 1 {
				return offset
			}
		}
	}
	return len(s)
}

// invalid returns a validation error with message.
func invalid(message string) error {
	return &jira4claude.Error{Code: jira4claude.EValidation, Message: message}
}
