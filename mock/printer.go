package mock

import "github.com/fwojciec/jira4claude"

// Compile-time interface verification.
var _ jira4claude.Printer = (*Printer)(nil)

// Printer is a mock implementation of jira4claude.Printer.
// Each method delegates to its corresponding function field if set.
// Unlike IssueService, methods with nil function fields do not panic;
// they only record the call for assertion.
type Printer struct {
	IssueFn       func(issue *jira4claude.Issue)
	IssuesFn      func(issues []*jira4claude.Issue)
	CommentFn     func(comment *jira4claude.Comment)
	TransitionsFn func(key string, ts []*jira4claude.Transition)
	LinksFn       func(key string, links []*jira4claude.IssueLink)
	SuccessFn     func(msg string, keys ...string)
	WarningFn     func(msg string)
	ErrorFn       func(err error)

	// Captured calls for assertions
	IssueCalls       []*jira4claude.Issue
	IssuesCalls      [][]*jira4claude.Issue
	CommentCalls     []*jira4claude.Comment
	TransitionsCalls []struct {
		Key         string
		Transitions []*jira4claude.Transition
	}
	LinksCalls []struct {
		Key   string
		Links []*jira4claude.IssueLink
	}
	SuccessCalls []struct {
		Msg  string
		Keys []string
	}
	WarningCalls []string
	ErrorCalls   []error
}

func (p *Printer) Issue(issue *jira4claude.Issue) {
	p.IssueCalls = append(p.IssueCalls, issue)
	if p.IssueFn != nil {
		p.IssueFn(issue)
	}
}

func (p *Printer) Issues(issues []*jira4claude.Issue) {
	p.IssuesCalls = append(p.IssuesCalls, issues)
	if p.IssuesFn != nil {
		p.IssuesFn(issues)
	}
}

func (p *Printer) Comment(comment *jira4claude.Comment) {
	p.CommentCalls = append(p.CommentCalls, comment)
	if p.CommentFn != nil {
		p.CommentFn(comment)
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

func (p *Printer) Links(key string, links []*jira4claude.IssueLink) {
	p.LinksCalls = append(p.LinksCalls, struct {
		Key   string
		Links []*jira4claude.IssueLink
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
