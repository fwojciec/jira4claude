package jira4claude

// IssuePrinter handles issue command output.
type IssuePrinter interface {
	Issue(view IssueView)
	// IssueViews prints the issues of an issue view asked for several keys,
	// in the order asked (possibly none, when every key failed).
	IssueViews(views []IssueView)
	// IssueSelection prints the fields issue view --field selected.
	IssueSelection(view IssueSelectionView)
	Issues(view IssueListView)
	Comment(view CommentView)
	Transitions(key string, ts []*Transition)
	Fields(view IssueFieldsView)
}

// LinkPrinter handles link command output.
type LinkPrinter interface {
	Links(key string, links []RelatedIssueView)
}

// AttachmentPrinter handles attachment command output.
type AttachmentPrinter interface {
	// Attachments prints the attachments of the issue with the given key.
	Attachments(key string, attachments []AttachmentView)
}

// MessagePrinter handles success/error/warning output. Success is written
// with the data, to stdout. Warning, ConversionIssue and Error are diagnostics:
// an implementation writes them to stderr, never stdout, so that stdout holds
// only data.
type MessagePrinter interface {
	Success(msg string, keys ...string)
	Warning(msg string)
	// ConversionIssue reports content that a Markdown/ADF conversion dropped
	// or degraded. It is a warning: it never changes the exit code.
	ConversionIssue(issue ConversionIssue)
	Error(err error)
}

// Printer combines all output capabilities.
type Printer interface {
	IssuePrinter
	LinkPrinter
	AttachmentPrinter
	MessagePrinter
}
