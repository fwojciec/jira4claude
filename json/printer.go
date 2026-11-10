package json

import (
	"encoding/json"
	"io"

	"github.com/fwojciec/jira4claude"
)

// Printer outputs JSON for machine parsing: data on stdout, and errors and
// warnings on stderr as one JSON object per line.
type Printer struct {
	out       io.Writer
	err       io.Writer
	serverURL string
}

// SetServerURL sets the server URL for generating issue URLs.
func (p *Printer) SetServerURL(url string) {
	p.serverURL = url
}

// NewPrinter creates a JSON printer that writes data to out.
// Errors and warnings are discarded since no stderr writer is provided.
func NewPrinter(out io.Writer) *Printer {
	return &Printer{out: out, err: io.Discard}
}

// NewPrinterWithIO creates a JSON printer with explicit stdout and stderr writers.
func NewPrinterWithIO(out, err io.Writer) *Printer {
	return &Printer{out: out, err: err}
}

func (p *Printer) encode(v any) {
	enc := json.NewEncoder(p.out)
	enc.SetIndent("", "  ")
	// Error ignored: encoding known map structures should not fail.
	// If the writer fails, CLI output has no useful recovery path.
	_ = enc.Encode(v)
}

// Issue prints a single issue as JSON.
func (p *Printer) Issue(view jira4claude.IssueView) {
	p.encode(view)
}

// Issues prints issues as a JSON object: {issues, truncated}. truncated is
// always present and tells a consumer that more matching issues exist beyond
// those shown. The issues array is emitted as [] (not null) when empty.
func (p *Printer) Issues(view jira4claude.IssueListView) {
	if view.Issues == nil {
		view.Issues = []jira4claude.IssueListItem{}
	}
	p.encode(view)
}

// Comment prints a single comment as JSON.
func (p *Printer) Comment(view jira4claude.CommentView) {
	p.encode(view)
}

// Transitions prints transitions as JSON array.
func (p *Printer) Transitions(_ string, ts []*jira4claude.Transition) {
	result := make([]map[string]any, len(ts))
	for i, t := range ts {
		result[i] = map[string]any{"id": t.ID, "name": t.Name}
	}
	p.encode(result)
}

// Fields prints settable issue fields as a JSON object:
// {source, scope, omitted, fields}. The scope and omitted count tell a
// consumer which selection produced the list and how many settable fields are
// hidden behind --all / --filter. The fields array is emitted as [] (not null)
// when empty so consumers can always iterate without a nil check.
func (p *Printer) Fields(view jira4claude.IssueFieldsView) {
	if view.Fields == nil {
		view.Fields = []*jira4claude.IssueField{}
	}
	p.encode(view)
}

// Links prints links as JSON array.
func (p *Printer) Links(_ string, links []jira4claude.RelatedIssueView) {
	p.encode(links)
}

// Attachments prints attachments as a JSON array, [] (not null) when empty.
func (p *Printer) Attachments(_ string, attachments []jira4claude.AttachmentView) {
	if attachments == nil {
		attachments = []jira4claude.AttachmentView{}
	}
	p.encode(attachments)
}

// Success prints a success message as JSON.
func (p *Printer) Success(msg string, keys ...string) {
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

// Diagnostic levels on stderr. A script reads level to tell an error from a
// warning without knowing which other keys each one carries.
const (
	levelError   = "error"
	levelWarning = "warning"
)

// errorLine is the stderr shape of an error. The error, code and message keys
// are the ones errors carried when they were printed on stdout.
type errorLine struct {
	Level   string `json:"level"`
	Error   bool   `json:"error"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

// warningLine is the stderr shape of a warning. A conversion warning also
// carries the ConversionIssue fields (kind, node, detail, existing).
type warningLine struct {
	Level    string `json:"level"`
	Message  string `json:"message"`
	Kind     string `json:"kind,omitempty"`
	Node     string `json:"node,omitempty"`
	Detail   string `json:"detail,omitempty"`
	Existing bool   `json:"existing,omitempty"`
}

// diagnose writes v to stderr as one compact JSON object on its own line, so
// that a script can read stderr line by line.
func (p *Printer) diagnose(v any) {
	enc := json.NewEncoder(p.err)
	// Messages are read by people too: keep <, > and & as they are.
	enc.SetEscapeHTML(false)
	// Error ignored: as in encode, a failing writer has no recovery path.
	_ = enc.Encode(v)
}

// Warning prints a warning to stderr as one JSON line:
// {"level":"warning","message":...}.
func (p *Printer) Warning(msg string) {
	p.diagnose(warningLine{Level: levelWarning, Message: msg})
}

// ConversionIssue prints a conversion issue to stderr as one JSON warning line
// that also carries the issue's kind, node, detail and existing fields.
func (p *Printer) ConversionIssue(issue jira4claude.ConversionIssue) {
	p.diagnose(warningLine{
		Level:    levelWarning,
		Message:  issue.String(),
		Kind:     issue.Kind,
		Node:     issue.Node,
		Detail:   issue.Detail,
		Existing: issue.Existing,
	})
}

// Error prints an error to stderr as one JSON line:
// {"level":"error","error":true,"code":...,"message":...}. stdout carries data
// only.
func (p *Printer) Error(err error) {
	p.diagnose(errorLine{
		Level:   levelError,
		Error:   true,
		Code:    jira4claude.ErrorCode(err),
		Message: jira4claude.ErrorMessage(err),
	})
}

// Verify interface compliance at compile time.
var _ jira4claude.Printer = (*Printer)(nil)
