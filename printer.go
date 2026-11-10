package jira4claude

// IssuePrinter handles issue command output.
type IssuePrinter interface {
	Issue(view IssueView)
	// IssueViews prints the issues of an issue view asked for several keys,
	// in the order asked (possibly none, when every key failed).
	IssueViews(views []IssueView)
	// IssueSelection prints the fields issue view --field selected.
	IssueSelection(view IssueSelectionView)
	// ViewConversionIssue reports, on stderr like ConversionIssue, content
	// that reading an issue for issue view dropped or degraded, attributed to
	// the issue.
	ViewConversionIssue(issue ViewConversionIssue)
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
	// SavedAttachments prints the attachments of the issue with the given
	// key that were saved to local files.
	SavedAttachments(key string, saved []SavedAttachmentView)
}

// MutationPrinter reports what a command that writes did. It writes to stdout,
// as data.
type MutationPrinter interface {
	// Mutation reports a write to Jira.
	Mutation(result MutationResult)
	// Initialized reports what init wrote.
	Initialized(result InitResult)
}

// MessagePrinter handles diagnostics. Warning, ConversionIssue and Error are
// written to stderr, never stdout, so that stdout holds only data.
type MessagePrinter interface {
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
	MutationPrinter
	MessagePrinter
}
