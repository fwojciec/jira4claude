package jira4claude

import (
	"strconv"
	"strings"
)

// Mutation actions: what a command did, in MutationResult.Action.
const (
	ActionCreated        = "created"
	ActionUpdated        = "updated"
	ActionTransitioned   = "transitioned"
	ActionAssigned       = "assigned"
	ActionUnassigned     = "unassigned"
	ActionDeleted        = "deleted"
	ActionCommentAdded   = "comment_added"
	ActionCommentDeleted = "comment_deleted"
	ActionLinked         = "linked"
	ActionUnlinked       = "unlinked"
	// ActionAttachmentAdded reports files attached to an issue; the
	// attachments Jira created are in Attachments.
	ActionAttachmentAdded = "attachment_added"
	// ActionAttachmentDeleted reports an attachment deleted, in
	// Attachments as it was read just before the delete.
	ActionAttachmentDeleted = "attachment_deleted"

	// Actions of a dry run (--dry-run): what the command would do. They
	// differ from the actions of real writes, so that a script testing for
	// "created" or "updated" cannot take a dry run for a write.
	ActionWouldCreate     = "would_create"
	ActionWouldUpdate     = "would_update"
	ActionWouldAddComment = "would_add_comment"
)

// Description edits, the Edit of the "description" Change.
const (
	EditSet      = "set"
	EditAppended = "appended"
	EditCleared  = "cleared"
)

// MutationResult is what a command that writes to Jira reports on stdout: the
// issue it wrote, what it did, and the resulting values, so that a script
// learns what happened without reading the issue again.
type MutationResult struct {
	// Key is the issue written: the new issue for a create, the issue
	// commented on, the source issue of a link. It is empty for the dry run
	// of a create, which has no key yet (Project names the project then), and
	// for an attachment delete: Jira's attachment metadata does not name the
	// issue.
	Key     string `json:"key,omitempty"`
	Project string `json:"project,omitempty"`
	// URL browses the issue. It is empty for a deleted issue and for the dry
	// run of a create.
	URL    string `json:"url,omitempty"`
	Action string `json:"action"` // one of the Action constants
	// DryRun is set when nothing was written: the result says what the
	// command would do, and Action is one of the ActionWould constants.
	DryRun bool `json:"dryRun,omitempty"`
	// Changes maps each field the command wrote to its value before and after
	// the write: Jira field names (summary, status, assignee, priority, labels,
	// parent, description) or custom field IDs. A printer emits it as {} when
	// empty.
	Changes map[string]Change `json:"changes"`
	// Removed names the existing content of a description that the write
	// deleted under --force: protected-node tokens such as
	// {{mediaSingle:1 shot.png}}, and nodes that had no Markdown form.
	Removed []string `json:"removed,omitempty"`
	// Updated is the issue's updated time after an issue update, read back
	// after the write in the form --if-updated takes, so that a script can
	// chain the next guarded update.
	Updated string `json:"updated,omitempty"`
	// CommentID is the comment added or deleted.
	CommentID string `json:"commentId,omitempty"`
	// Link is the link created or deleted.
	Link *LinkEnds `json:"link,omitempty"`
	// Text is, for a dry run, the Markdown Jira would hold after the write,
	// read back from the document that would be sent: the whole description
	// (after re-attachment, and with the current content for an append; ""
	// for a clear) or the comment body. It is nil when the command writes no
	// text.
	Text *string `json:"text,omitempty"`
	// Attachments are the attachments added or deleted, in the order the
	// files were given.
	Attachments []AttachmentView `json:"attachments,omitempty"`
}

// Change is one field's value before (From) and after (To) a write. Values
// are a string, nil for an empty field (no assignee, no parent), a []string
// for labels, or the raw JSON Jira holds for a custom field. A description is
// not reported as text: Edit says how it was changed (EditSet, EditAppended
// or EditCleared), and From and To are unused. Edit is also EditSet for a
// built-in field written by ID (--field-json), whose value the read does not
// carry.
type Change struct {
	From any
	To   any
	Edit string
}

// MarshalJSON writes {"from":...,"to":...}, or {"edit":...} for a description.
func (c Change) MarshalJSON() ([]byte, error) {
	if c.Edit != "" {
		return marshalLiteral(struct {
			Edit string `json:"edit"`
		}{c.Edit})
	}
	return marshalLiteral(struct {
		From any `json:"from"`
		To   any `json:"to"`
	}{c.From, c.To})
}

// LinkEnds names the issues of a link as the command was given them. Type is
// empty for a deleted link, which is found by its two issues alone.
type LinkEnds struct {
	Type   string `json:"type,omitempty"`
	Source string `json:"source"`
	Target string `json:"target"`
}

// IssueURL returns the browse URL of key on server, or "" without a server.
func IssueURL(server, key string) string {
	if server == "" {
		return ""
	}
	return server + "/browse/" + key
}

// UpdatedTimestamp returns issue's updated time as --if-updated takes it, or
// "" when it is unknown.
func UpdatedTimestamp(issue *Issue) string {
	if issue == nil || issue.Updated.IsZero() {
		return ""
	}
	return FormatTimestamp(issue.Updated)
}

// UpdateChanges reports each field update writes, with its value in before
// (the issue read just before the write) and after (read back after it). A
// field written with the value it already had is reported too, with equal
// From and To. The result is never nil.
func UpdateChanges(before, after *Issue, update IssueUpdate) map[string]Change {
	changes := make(map[string]Change)
	if update.Summary != nil {
		changes["summary"] = Change{From: optional(before.Summary), To: optional(after.Summary)}
	}
	if update.Priority != nil {
		changes["priority"] = Change{From: optional(before.Priority), To: optional(after.Priority)}
	}
	if update.Assignee != nil {
		changes["assignee"] = AssigneeChange(before, after)
	}
	if update.Labels != nil {
		changes["labels"] = Change{From: labels(before.Labels), To: labels(after.Labels)}
	}
	if update.Parent != nil {
		changes["parent"] = Change{From: parentKey(before), To: parentKey(after)}
	}
	for id := range update.CustomFields {
		if strings.HasPrefix(id, "customfield_") {
			changes[id] = Change{From: customValue(before, id), To: customValue(after, id)}
			continue
		}
		// A built-in field set by --field-json: the read holds no raw value
		// for it, so report the write without claiming values. A typed flag
		// for the same field keeps its own entry.
		if _, typed := changes[id]; !typed {
			changes[id] = Change{Edit: EditSet}
		}
	}
	switch update.Description.Mode() {
	case DescriptionKeep:
	case DescriptionSet:
		changes["description"] = Change{Edit: EditSet}
	case DescriptionAppend:
		changes["description"] = Change{Edit: EditAppended}
	case DescriptionClear:
		changes["description"] = Change{Edit: EditCleared}
	}
	return changes
}

// PlannedChanges is UpdateChanges for a write that is not made (a dry run):
// From comes from before, the issue as read, and To from the values update
// would send. An assignee is named by the account ID sent, or by the name the
// read gave when it is the current assignee; only Jira knows another
// account's name, and how it would normalise any value.
func PlannedChanges(before *Issue, update IssueUpdate) map[string]Change {
	after := *before
	if update.Summary != nil {
		after.Summary = *update.Summary
	}
	if update.Priority != nil {
		after.Priority = *update.Priority
	}
	if update.Assignee != nil {
		switch {
		case *update.Assignee == "":
			after.Assignee = nil
		case before.Assignee == nil || before.Assignee.AccountID != *update.Assignee:
			after.Assignee = &User{AccountID: *update.Assignee}
		}
	}
	if update.Labels != nil {
		after.Labels = *update.Labels
	}
	if update.Parent != nil {
		after.Parent = nil
		if *update.Parent != "" {
			after.Parent = &LinkedIssue{Key: *update.Parent}
		}
	}
	if len(update.CustomFields) > 0 {
		after.ReadCustomFields = make(map[string]CustomFieldValue, len(before.ReadCustomFields)+len(update.CustomFields))
		for id, v := range before.ReadCustomFields {
			after.ReadCustomFields[id] = v
		}
		for id, raw := range update.CustomFields {
			v := after.ReadCustomFields[id]
			v.Value = raw
			after.ReadCustomFields[id] = v
		}
	}
	return UpdateChanges(before, &after, update)
}

// AssigneeChange reports the assignee of before and after by display name,
// as issue view shows it; nil means unassigned.
func AssigneeChange(before, after *Issue) Change {
	return Change{From: assigneeName(before), To: assigneeName(after)}
}

// StatusChange reports a transition of before, the issue as read just before
// it, to the status transition leads to; To is nil when Jira did not name it.
func StatusChange(before *Issue, transition *Transition) Change {
	return Change{From: optional(before.Status), To: optional(transition.To)}
}

// RemovedNodes names the content of an existing description that a write
// removes: the dropped issues of a description set or clear that are about
// the existing document (Existing). Dropped issues about the new text, such
// as a token used twice, remove nothing and are left out.
func RemovedNodes(issues []ConversionIssue) []string {
	var removed []string
	for _, i := range DroppedIssues(issues) {
		if i.Existing {
			removed = append(removed, i.Node)
		}
	}
	return removed
}

// SelectTransition picks the transition --status names: the one whose target
// status is status, or failing that the one named status, both compared
// without case. More than one match at the first tier that matches is
// ambiguous; no match at all is not found. Both are validation errors that
// list the choices.
func SelectTransition(transitions []*Transition, status string) (*Transition, error) {
	for _, same := range []func(t *Transition) bool{
		func(t *Transition) bool { return t.To != "" && strings.EqualFold(t.To, status) },
		func(t *Transition) bool { return strings.EqualFold(t.Name, status) },
	} {
		var matches []*Transition
		for _, t := range transitions {
			if same(t) {
				matches = append(matches, t)
			}
		}
		switch len(matches) {
		case 0:
			continue
		case 1:
			return matches[0], nil
		default:
			return nil, &Error{
				Code: EValidation,
				Message: `status "` + status + `" is ambiguous: it matches ` + describeTransitions(matches) +
					". Nothing was written. Pass --id to choose one",
			}
		}
	}
	return nil, &Error{
		Code:    EValidation,
		Message: `status "` + status + `" not found; available: ` + describeTransitions(transitions),
	}
}

// TransitionByID returns the transition with id, or a validation error that
// lists the available ones.
func TransitionByID(transitions []*Transition, id string) (*Transition, error) {
	for _, t := range transitions {
		if t.ID == id {
			return t, nil
		}
	}
	return nil, &Error{
		Code:    EValidation,
		Message: "transition id " + strconv.Quote(id) + " is not available; available: " + describeTransitions(transitions),
	}
}

// describeTransitions lists transitions as "Name" (id ID) → Status.
func describeTransitions(transitions []*Transition) string {
	if len(transitions) == 0 {
		return "none"
	}
	parts := make([]string, len(transitions))
	for i, t := range transitions {
		parts[i] = strconv.Quote(t.Name) + " (id " + t.ID + ")"
		if t.To != "" {
			parts[i] += " → " + t.To
		}
	}
	return strings.Join(parts, ", ")
}

// optional returns nil for an empty string, s otherwise.
func optional(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func assigneeName(issue *Issue) any {
	if issue.Assignee == nil {
		return nil
	}
	if issue.Assignee.DisplayName != "" {
		return issue.Assignee.DisplayName
	}
	return optional(issue.Assignee.AccountID)
}

// labels returns ls as a non-nil slice, so that no labels is [] rather than
// null.
func labels(ls []string) []string {
	if ls == nil {
		return []string{}
	}
	return ls
}

func parentKey(issue *Issue) any {
	if issue.Parent == nil {
		return nil
	}
	return optional(issue.Parent.Key)
}

func customValue(issue *Issue, id string) any {
	v, ok := issue.ReadCustomFields[id]
	if !ok || len(v.Value) == 0 {
		return nil
	}
	return v.Value
}
