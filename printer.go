package jira4claude

// IssuePrinter handles issue command output.
type IssuePrinter interface {
	Issue(issue *Issue)
	Issues(issues []*Issue)
	Comment(comment *Comment)
	Transitions(key string, ts []*Transition)
}

// LinkPrinter handles link command output.
type LinkPrinter interface {
	Links(key string, links []*IssueLink)
}

// MessagePrinter handles success/error output.
type MessagePrinter interface {
	Success(msg string, keys ...string)
	Error(err error)
}

// Printer combines all output capabilities.
type Printer interface {
	IssuePrinter
	LinkPrinter
	MessagePrinter
}
