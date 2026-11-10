package mock

import "github.com/fwojciec/jira4claude"

// Compile-time interface verification.
var _ jira4claude.Printer = (*Printer)(nil)

// Printer is a mock implementation of jira4claude.Printer.
// Each method delegates to its corresponding function field if set.
// Unlike IssueService, methods with nil function fields do not panic;
// they only record the call for assertion.
type Printer struct {
	IssueFn           func(view jira4claude.IssueView)
	IssueViewsFn      func(views []jira4claude.IssueView)
	IssueSelectionFn  func(view jira4claude.IssueSelectionView)
	IssuesFn          func(view jira4claude.IssueListView)
	CommentFn         func(view jira4claude.CommentView)
	TransitionsFn     func(key string, ts []*jira4claude.Transition)
	FieldsFn          func(view jira4claude.IssueFieldsView)
	LinksFn           func(key string, links []jira4claude.RelatedIssueView)
	AttachmentsFn     func(key string, attachments []jira4claude.AttachmentView)
	MutationFn        func(result jira4claude.MutationResult)
	InitializedFn     func(result jira4claude.InitResult)
	WarningFn         func(msg string)
	ConversionIssueFn func(issue jira4claude.ConversionIssue)
	ErrorFn           func(err error)

	ViewConversionIssueFn func(issue jira4claude.ViewConversionIssue)
	SavedAttachmentsFn    func(key string, saved []jira4claude.SavedAttachmentView)

	// Captured calls for assertions
	IssueCalls          []jira4claude.IssueView
	IssueViewsCalls     [][]jira4claude.IssueView
	IssueSelectionCalls []jira4claude.IssueSelectionView
	IssuesCalls         []jira4claude.IssueListView
	CommentCalls        []jira4claude.CommentView
	TransitionsCalls    []struct {
		Key         string
		Transitions []*jira4claude.Transition
	}
	FieldsCalls []jira4claude.IssueFieldsView
	LinksCalls  []struct {
		Key   string
		Links []jira4claude.RelatedIssueView
	}
	AttachmentsCalls []struct {
		Key         string
		Attachments []jira4claude.AttachmentView
	}
	MutationCalls        []jira4claude.MutationResult
	InitializedCalls     []jira4claude.InitResult
	WarningCalls         []string
	ConversionIssueCalls []jira4claude.ConversionIssue
	ErrorCalls           []error

	ViewConversionIssueCalls []jira4claude.ViewConversionIssue
	SavedAttachmentsCalls    []struct {
		Key   string
		Saved []jira4claude.SavedAttachmentView
	}
}

func (p *Printer) ViewConversionIssue(issue jira4claude.ViewConversionIssue) {
	p.ViewConversionIssueCalls = append(p.ViewConversionIssueCalls, issue)
	if p.ViewConversionIssueFn != nil {
		p.ViewConversionIssueFn(issue)
	}
}

func (p *Printer) Issue(view jira4claude.IssueView) {
	p.IssueCalls = append(p.IssueCalls, view)
	if p.IssueFn != nil {
		p.IssueFn(view)
	}
}

func (p *Printer) IssueViews(views []jira4claude.IssueView) {
	p.IssueViewsCalls = append(p.IssueViewsCalls, views)
	if p.IssueViewsFn != nil {
		p.IssueViewsFn(views)
	}
}

func (p *Printer) IssueSelection(view jira4claude.IssueSelectionView) {
	p.IssueSelectionCalls = append(p.IssueSelectionCalls, view)
	if p.IssueSelectionFn != nil {
		p.IssueSelectionFn(view)
	}
}

func (p *Printer) Issues(view jira4claude.IssueListView) {
	p.IssuesCalls = append(p.IssuesCalls, view)
	if p.IssuesFn != nil {
		p.IssuesFn(view)
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

func (p *Printer) Fields(view jira4claude.IssueFieldsView) {
	p.FieldsCalls = append(p.FieldsCalls, view)
	if p.FieldsFn != nil {
		p.FieldsFn(view)
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

func (p *Printer) Attachments(key string, attachments []jira4claude.AttachmentView) {
	p.AttachmentsCalls = append(p.AttachmentsCalls, struct {
		Key         string
		Attachments []jira4claude.AttachmentView
	}{key, attachments})
	if p.AttachmentsFn != nil {
		p.AttachmentsFn(key, attachments)
	}
}

func (p *Printer) SavedAttachments(key string, saved []jira4claude.SavedAttachmentView) {
	p.SavedAttachmentsCalls = append(p.SavedAttachmentsCalls, struct {
		Key   string
		Saved []jira4claude.SavedAttachmentView
	}{key, saved})
	if p.SavedAttachmentsFn != nil {
		p.SavedAttachmentsFn(key, saved)
	}
}

func (p *Printer) Mutation(result jira4claude.MutationResult) {
	p.MutationCalls = append(p.MutationCalls, result)
	if p.MutationFn != nil {
		p.MutationFn(result)
	}
}

func (p *Printer) Initialized(result jira4claude.InitResult) {
	p.InitializedCalls = append(p.InitializedCalls, result)
	if p.InitializedFn != nil {
		p.InitializedFn(result)
	}
}

func (p *Printer) Warning(msg string) {
	p.WarningCalls = append(p.WarningCalls, msg)
	if p.WarningFn != nil {
		p.WarningFn(msg)
	}
}

func (p *Printer) ConversionIssue(issue jira4claude.ConversionIssue) {
	p.ConversionIssueCalls = append(p.ConversionIssueCalls, issue)
	if p.ConversionIssueFn != nil {
		p.ConversionIssueFn(issue)
	}
}

func (p *Printer) Error(err error) {
	p.ErrorCalls = append(p.ErrorCalls, err)
	if p.ErrorFn != nil {
		p.ErrorFn(err)
	}
}
