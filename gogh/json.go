package gogh

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/fwojciec/jira4claude"
)

// JSONPrinter outputs JSON format to stdout for machine parsing.
type JSONPrinter struct {
	out       io.Writer
	err       io.Writer
	serverURL string
}

// SetServerURL sets the server URL for generating issue URLs.
func (p *JSONPrinter) SetServerURL(url string) {
	p.serverURL = url
}

// NewJSONPrinter creates a JSON printer that writes to out.
// Warnings are discarded since no stderr writer is provided.
func NewJSONPrinter(out io.Writer) *JSONPrinter {
	return &JSONPrinter{out: out, err: io.Discard}
}

// NewJSONPrinterWithIO creates a JSON printer with explicit stdout and stderr writers.
func NewJSONPrinterWithIO(out, err io.Writer) *JSONPrinter {
	return &JSONPrinter{out: out, err: err}
}

func (p *JSONPrinter) encode(v any) {
	enc := json.NewEncoder(p.out)
	enc.SetIndent("", "  ")
	// Error ignored: encoding known map structures should not fail.
	// If the writer fails, CLI output has no useful recovery path.
	_ = enc.Encode(v)
}

// Issue prints a single issue as JSON.
func (p *JSONPrinter) Issue(view jira4claude.IssueView) {
	p.encode(view)
}

// Issues prints multiple issues as JSON array.
func (p *JSONPrinter) Issues(views []jira4claude.IssueView) {
	p.encode(views)
}

// Comment prints a single comment as JSON.
func (p *JSONPrinter) Comment(view jira4claude.CommentView) {
	p.encode(view)
}

// Transitions prints transitions as JSON array.
func (p *JSONPrinter) Transitions(_ string, ts []*jira4claude.Transition) {
	result := make([]map[string]any, len(ts))
	for i, t := range ts {
		result[i] = map[string]any{"id": t.ID, "name": t.Name}
	}
	p.encode(result)
}

// Links prints links as JSON array.
func (p *JSONPrinter) Links(_ string, links []jira4claude.LinkView) {
	p.encode(links)
}

// Success prints a success message as JSON.
func (p *JSONPrinter) Success(msg string, keys ...string) {
	result := map[string]any{
		"success": true,
		"message": msg,
	}
	if len(keys) > 0 {
		result["keys"] = keys
		if p.serverURL != "" {
			urls := make([]string, len(keys))
			for i, k := range keys {
				urls[i] = p.serverURL + "/browse/" + k
			}
			result["urls"] = urls
		}
	}
	p.encode(result)
}

// Warning prints a warning message to stderr as plain text.
func (p *JSONPrinter) Warning(msg string) {
	fmt.Fprintln(p.err, "warning: "+msg)
}

// Error prints an error as JSON to stdout (for machine parsing).
func (p *JSONPrinter) Error(err error) {
	p.encode(map[string]any{
		"error":   true,
		"code":    jira4claude.ErrorCode(err),
		"message": jira4claude.ErrorMessage(err),
	})
}

// Verify interface compliance at compile time.
var _ jira4claude.Printer = (*JSONPrinter)(nil)
