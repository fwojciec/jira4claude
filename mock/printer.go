package mock

import "github.com/fwojciec/jira4claude"

// Compile-time interface verification.
var _ jira4claude.Printer = (*Printer)(nil)

// Printer is a mock implementation of jira4claude.Printer.
// Each method delegates to its corresponding function field if set.
// Unlike IssueService, methods with nil function fields do not panic;
// they only record the call for assertion.
type Printer struct {
	IssueFn       func(view jira4claude.IssueView)
	IssuesFn      func(views []jira4claude.IssueView)
	CommentFn     func(view jira4claude.CommentView)
	TransitionsFn func(key string, ts []*jira4claude.Transition)
	LinksFn       func(key string, links []jira4claude.RelatedIssueView)
	SuccessFn     func(msg string, keys ...string)
	WarningFn     func(msg string)
	ErrorFn       func(err error)

	// Captured calls for assertions
	IssueCalls       []jira4claude.IssueView
	IssuesCalls      [][]jira4claude.IssueView
	CommentCalls     []jira4claude.CommentView
	TransitionsCalls []struct {
		Key         string
		Transitions []*jira4claude.Transition
	}
	LinksCalls []struct {
		Key   string
		Links []jira4claude.RelatedIssueView
	}
	SuccessCalls []struct {
		Msg  string
		Keys []string
	}
	WarningCalls []string
	ErrorCalls   []error
}

func (p *Printer) Issue(view jira4claude.IssueView) {
	p.IssueCalls = append(p.IssueCalls, view)
	if p.IssueFn != nil {
		p.IssueFn(view)
	}
}

func (p *Printer) Issues(views []jira4claude.IssueView) {
	p.IssuesCalls = append(p.IssuesCalls, views)
	if p.IssuesFn != nil {
		p.IssuesFn(views)
	}
}

func (p *Printer) Comment(view jira4claude.CommentView) {
	p.CommentCalls = append(p.CommentCalls, view)
	if p.CommentFn != nil {
		p.CommentFn(view)
	}
}

func (p *Printer) Transitions(key string, ts []*jira4claude.Transition) {
	p.TransitionsCalls = append(p.TransitionsCalls, struct {
		Key         string
		Transitions []*jira4claude.Transition
	}{key, ts})
	if p.TransitionsFn != nil {
		p.TransitionsFn(key, ts)
	}
}

func (p *Printer) Links(key string, links []jira4claude.RelatedIssueView) {
	p.LinksCalls = append(p.LinksCalls, struct {
		Key   string
		Links []jira4claude.RelatedIssueView
	}{key, links})
	if p.LinksFn != nil {
		p.LinksFn(key, links)
	}
}

func (p *Printer) Success(msg string, keys ...string) {
	p.SuccessCalls = append(p.SuccessCalls, struct {
		Msg  string
		Keys []string
	}{msg, keys})
	if p.SuccessFn != nil {
		p.SuccessFn(msg, keys...)
	}
}

func (p *Printer) Warning(msg string) {
	p.WarningCalls = append(p.WarningCalls, msg)
	if p.WarningFn != nil {
		p.WarningFn(msg)
	}
}

func (p *Printer) Error(err error) {
	p.ErrorCalls = append(p.ErrorCalls, err)
	if p.ErrorFn != nil {
		p.ErrorFn(err)
	}
}
