package json

import (
	"encoding/json"
	"io"

	"github.com/fwojciec/jira4claude"
)

// Printer outputs JSON for machine parsing: data on stdout, and errors and
// warnings on stderr as one JSON object per line.
type Printer struct {
	out io.Writer
	err io.Writer
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

// IssueViews prints the issues of a several-key view as a JSON array of issue
// objects, in the order asked; [] when none could be read.
func (p *Printer) IssueViews(views []jira4claude.IssueView) {
	if views == nil {
		views = []jira4claude.IssueView{}
	}
	p.encode(views)
}

// IssueSelection prints the fields issue view --field selected. Each issue is
// an object holding "key" and then the selected fields in the order asked.
// One key gives that object; several keys give an array of them, in the order
// asked ([] when none could be read).
func (p *Printer) IssueSelection(view jira4claude.IssueSelectionView) {
	if !view.Several && len(view.Issues) == 1 {
		p.encode(view.Issues[0])
		return
	}
	issues := view.Issues
	if issues == nil {
		issues = []jira4claude.SelectedIssueView{}
	}
	p.encode(issues)
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

// Transitions prints transitions as a JSON array of {id, name, to}, where to
// is the status the transition leads to, or null when Jira names none.
func (p *Printer) Transitions(_ string, ts []*jira4claude.Transition) {
	result := make([]map[string]any, len(ts))
	for i, t := range ts {
		var to any
		if t.To != "" {
			to = t.To
		}
		result[i] = map[string]any{"id": t.ID, "name": t.Name, "to": to}
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

// SavedAttachments prints the saved files as a JSON array of {id, filename,
// path, size}, [] (not null) when empty, also when every file failed, so
// that stdout always parses.
func (p *Printer) SavedAttachments(_ string, saved []jira4claude.SavedAttachmentView, _ int) {
	if saved == nil {
		saved = []jira4claude.SavedAttachmentView{}
	}
	p.encode(saved)
}

// Mutation prints the result of a write as one JSON object. changes is
// always an object, {} when the command reports none.
func (p *Printer) Mutation(result jira4claude.MutationResult) {
	if result.Changes == nil {
		result.Changes = map[string]jira4claude.Change{}
	}
	p.encode(result)
}

// Initialized prints what init wrote as one JSON object.
func (p *Printer) Initialized(result jira4claude.InitResult) {
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
// carries the ConversionIssue fields (kind, node, token, detail, existing).
type warningLine struct {
	Level    string `json:"level"`
	Message  string `json:"message"`
	Kind     string `json:"kind,omitempty"`
	Node     string `json:"node,omitempty"`
	Token    string `json:"token,omitempty"`
	Detail   string `json:"detail,omitempty"`
	Existing bool   `json:"existing,omitempty"`
	// Key names the issue a warning is about (issue view).
	Key string `json:"key,omitempty"`
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
// that also carries the issue's kind, node, token, detail and existing fields.
func (p *Printer) ConversionIssue(issue jira4claude.ConversionIssue) {
	p.diagnose(warningLine{
		Level:    levelWarning,
		Message:  issue.String(),
		Kind:     issue.Kind,
		Node:     issue.Node,
		Token:    issue.Token,
		Detail:   issue.Detail,
		Existing: issue.Existing,
	})
}

// ViewConversionIssue prints a conversion issue met reading an issue for
// issue view: the ConversionIssue line plus the issue's key, whether one key
// or several were asked.
func (p *Printer) ViewConversionIssue(issue jira4claude.ViewConversionIssue) {
	p.diagnose(warningLine{
		Level:    levelWarning,
		Message:  issue.String(),
		Kind:     issue.Kind,
		Node:     issue.Node,
		Token:    issue.Token,
		Detail:   issue.Detail,
		Existing: issue.Existing,
		Key:      issue.Key,
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
