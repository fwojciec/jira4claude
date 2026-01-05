package jira4claude

// IssuePrinter handles issue command output.
type IssuePrinter interface {
	Issue(view IssueView)
	Issues(views []IssueView)
	Comment(view CommentView)
	Transitions(key string, ts []*Transition)
}

// LinkPrinter handles link command output.
type LinkPrinter interface {
	Links(key string, links []RelatedIssueView)
}

// MessagePrinter handles success/error/warning output.
type MessagePrinter interface {
	Success(msg string, keys ...string)
	Warning(msg string)
	Error(err error)
}

// Printer combines all output capabilities.
type Printer interface {
	IssuePrinter
	LinkPrinter
	MessagePrinter
}
