package jira4claude

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"time"
)

// Status constants for common Jira workflow states.
const (
	StatusToDo       = "To Do"
	StatusInProgress = "In Progress"
	StatusDone       = "Done"
	StatusWontDo     = "Won't Do"
)

// LinkInwardBlockedBy is the inward description for a "Blocks" link type.
// When issue A blocks issue B, issue B has an inward link with this description.
const LinkInwardBlockedBy = "is blocked by"

// User represents a Jira user.
type User struct {
	AccountID   string
	DisplayName string
	Email       string
}

// Transition represents a workflow transition available for an issue.
type Transition struct {
	ID   string
	Name string
	To   string // name of the status the transition leads to; empty if unknown
}

// IssueLinkType represents the type of relationship between linked issues.
type IssueLinkType struct {
	Name    string // e.g., "Blocks"
	Inward  string // e.g., "is blocked by"
	Outward string // e.g., "blocks"
}

// IssueLink represents a link between two issues.
type IssueLink struct {
	ID           string
	Type         IssueLinkType
	OutwardIssue *LinkedIssue // Present when this issue is the source
	InwardIssue  *LinkedIssue // Present when this issue is the target
}

// LinkedIssue contains summary information about a linked issue.
type LinkedIssue struct {
	Key     string
	Summary string
	Status  string
	Type    string
}

// Comment represents a comment on an issue.
type Comment struct {
	ID      string
	Author  *User
	Body    *ADFNode // ADF document; conversion to markdown happens at CLI boundary
	Created time.Time
}

// Issue represents a Jira issue with its core fields.
type Issue struct {
	Key          string
	Project      string
	Summary      string
	Description  *ADFNode // ADF document; conversion to markdown happens at CLI boundary
	Status       string
	Type         string
	Priority     string
	Assignee     *User
	Reporter     *User
	Labels       []string
	Links        []*IssueLink
	Comments     []*Comment     // Comments on the issue
	Parent       *LinkedIssue   // Parent issue (for subtasks or epic children); nil otherwise
	Subtasks     []*LinkedIssue // Subtasks or epic children; nil if none
	Attachments  []*Attachment  // read-only: populated by Get; nil if none
	Created      time.Time
	Updated      time.Time
	CustomFields map[string]json.RawMessage // write-only on Create/Update; populated from --field-json.

	// ReadCustomFields is read-only: populated by Get (via expand=names) with
	// the issue's populated custom fields, keyed by field ID. List leaves it
	// empty (its field whitelist excludes custom fields).
	ReadCustomFields map[string]CustomFieldValue
}

// CustomFieldValue is a read-side custom field: its display name plus the raw
// API value. Keyed by field ID in Issue.ReadCustomFields. The raw Value is the
// value as returned by Jira, suitable for round-tripping back to --field-json.
type CustomFieldValue struct {
	Name  string          `json:"name"`
	Value json.RawMessage `json:"value"`
}

// IssueFilter specifies criteria for listing issues.
// If JQL is set, it is used directly and other fields are ignored.
// Otherwise, non-empty fields are combined with AND logic.
type IssueFilter struct {
	Project       string
	Status        string // Exact match: status = "X"
	ExcludeStatus string // Exclusion: status != "X" (ignored if Status is set)
	Assignee      string
	Parent        string   // Filter by parent issue key (for subtasks)
	Labels        []string // Issues must have ALL specified labels
	OrderBy       string   // e.g., "created DESC"
	JQL           string   // Raw JQL query; overrides other fields if set
	Limit         int      // Maximum number of issues to return (page size)
	PageToken     string   // Cursor for a page after the first; empty for the first page
}

// IssueUpdate specifies fields to update on an issue.
// Pointer fields: nil means no change, non-nil means set to that value.
// For Assignee: empty string means unassign.
// For Labels: nil means no change, empty slice means clear all labels.
// For Parent: nil means no change, empty string means clear, non-empty means set.
type IssueUpdate struct {
	Summary      *string
	Description  DescriptionUpdate // zero value = no change; conversion from markdown happens at CLI boundary
	Priority     *string
	Assignee     *string
	Labels       *[]string
	Parent       *string                    // nil = no change, "" = clear parent, "KEY" = set parent
	CustomFields map[string]json.RawMessage // nil = no change
}

// DescriptionMode says what an update does to an issue's description.
type DescriptionMode int

// Description update modes. DescriptionKeep is the zero value.
const (
	DescriptionKeep   DescriptionMode = iota // leave the description unchanged
	DescriptionSet                           // replace it with a new document
	DescriptionAppend                        // add nodes after the current content
	DescriptionClear                         // remove it
)

// DescriptionUpdate is the description part of an IssueUpdate. The zero value
// keeps the description; the other modes are built with SetDescription,
// AppendDescription and ClearDescription, so an append always carries the
// current description it extends.
type DescriptionUpdate struct {
	mode DescriptionMode
	doc  *ADFNode
}

// SetDescription replaces the description with doc.
func SetDescription(doc *ADFNode) DescriptionUpdate {
	return DescriptionUpdate{mode: DescriptionSet, doc: doc}
}

// AppendDescription adds the top-level nodes of addition after those of
// current, the issue's description as read before the update. The current
// nodes are kept as they are, never converted. When current is nil (the issue
// has no description), the update is a set of addition.
func AppendDescription(current, addition *ADFNode) DescriptionUpdate {
	if current == nil {
		return SetDescription(addition)
	}
	content := make([]ADFNode, 0, len(current.Content)+len(addition.Content))
	content = append(content, current.Content...)
	content = append(content, addition.Content...)
	doc := *current
	if doc.Version == 0 {
		doc.Version = addition.Version
	}
	doc.Content = content
	return DescriptionUpdate{mode: DescriptionAppend, doc: &doc}
}

// ClearDescription removes the description.
func ClearDescription() DescriptionUpdate {
	return DescriptionUpdate{mode: DescriptionClear}
}

// ReplaceDescription prepares a description set: edited, the document
// converted from the new Markdown, gets the protected nodes of current, the
// description as read just before the write, back in place of their tokens
// (see Converter.Reattach). Existing content the write would lose is refused
// with a validation error naming it, unless force. The issues returned are
// the re-attachment's, for the caller to report.
func ReplaceDescription(conv Converter, current, edited *ADFNode, force bool) (*ADFNode, []ConversionIssue, error) {
	doc, issues := conv.Reattach(current, edited)
	if !force {
		if err := DroppedContentError("description", issues); err != nil {
			return nil, nil, err
		}
	}
	return doc, issues, nil
}

// CheckClearDescription refuses to clear a description, current, that holds
// protected nodes (images, mentions and other nodes Markdown cannot carry),
// naming them, unless force. Under force it returns one dropped issue per
// node removed, for the caller to report. current is not converted: nothing of
// it is written back.
func CheckClearDescription(conv Converter, current *ADFNode, force bool) ([]ConversionIssue, error) {
	if current == nil {
		return nil, nil
	}
	_, missing := conv.Reattach(current, nil)
	var removed []ConversionIssue
	var tokens []string
	for _, m := range DroppedIssues(missing) {
		removed = append(removed, ConversionIssue{
			Kind:     ConversionDropped,
			Node:     m.Node,
			Token:    m.Token,
			Detail:   "removed with the description, which was cleared",
			Existing: true,
		})
		tokens = append(tokens, m.Node)
	}
	if len(removed) > 0 && !force {
		return nil, &Error{
			Code: EValidation,
			Message: "clearing the description would remove content that cannot be written back from Markdown (" +
				strings.Join(tokens, ", ") + "). Nothing was written. Pass --force to clear it anyway",
		}
	}
	return removed, nil
}

// LiteralTokens reports text in doc, converted from new Markdown for a
// create, a comment or an append, that reads as a protected-node token. Only a
// description set restores tokens; anywhere else a token is written as the
// plain text it is, so each one is reported as degraded: a token copied from
// issue view does not carry its image or mention along.
func LiteralTokens(conv Converter, doc *ADFNode) []ConversionIssue {
	found := conv.FindTokens(doc)
	issues := make([]ConversionIssue, 0, len(found))
	for _, token := range found {
		issues = append(issues, ConversionIssue{
			Kind:   ConversionDegraded,
			Node:   token,
			Token:  token,
			Detail: "written as plain text; a token restores existing content only in issue update --description",
		})
	}
	return issues
}

// timestampLayout is how timestamps compared by --if-updated are printed:
// RFC 3339 with exactly three fractional digits, Jira's own precision.
const timestampLayout = "2006-01-02T15:04:05.000Z07:00"

// jiraTimestampLayout is the form Jira itself returns, as in
// 2026-11-09T10:15:30.123+0000.
const jiraTimestampLayout = "2006-01-02T15:04:05.000-0700"

// FormatTimestamp formats t as RFC 3339 with milliseconds, the precision
// --if-updated compares at.
func FormatTimestamp(t time.Time) string {
	return t.Format(timestampLayout)
}

// ParseTimestamp parses an --if-updated value: RFC 3339, with or without
// fractional seconds (the form issue view --json prints), or the form Jira
// returns. A value it cannot parse is a validation error.
func ParseTimestamp(s string) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return t, nil
	}
	if t, err := time.Parse(jiraTimestampLayout, s); err == nil {
		return t, nil
	}
	return time.Time{}, &Error{
		Code: EValidation,
		Message: "--if-updated " + strconv.Quote(s) + " is not a timestamp; pass the updated value " +
			"issue view --json prints, such as 2026-11-09T10:15:30.123Z",
	}
}

// CheckUpdated returns a conflict error when issue, as read just before a
// write, was last updated at another time than want, compared at millisecond
// precision. An issue without an updated time cannot be checked, which is an
// internal error rather than a conflict.
func CheckUpdated(issue *Issue, want time.Time) error {
	if issue.Updated.IsZero() {
		return &Error{
			Code:    EInternal,
			Message: "cannot check --if-updated: Jira returned no updated time for " + issue.Key + ". Nothing was written",
		}
	}
	got := issue.Updated.Truncate(time.Millisecond)
	if got.Equal(want.Truncate(time.Millisecond)) {
		return nil
	}
	return &Error{
		Code: EConflict,
		Message: issue.Key + " was updated at " + FormatTimestamp(issue.Updated) + ", not at " +
			FormatTimestamp(want.In(issue.Updated.Location())) + " as --if-updated expects. Nothing was written. " +
			"View the issue again and redo the change against its current state",
	}
}

// Mode reports what the update does to the description.
func (u DescriptionUpdate) Mode() DescriptionMode {
	return u.mode
}

// Document returns the full description to write for DescriptionSet and
// DescriptionAppend (for an append, the current nodes followed by the new
// ones). It is nil for DescriptionKeep and DescriptionClear.
func (u DescriptionUpdate) Document() *ADFNode {
	return u.doc
}

// IssueField describes a single settable field as reported by Jira's
// createmeta or editmeta endpoint. The same struct serves both modes;
// callers distinguish create vs. edit via the service method used.
type IssueField struct {
	ID            string              `json:"id"`
	Name          string              `json:"name"`
	Required      bool                `json:"required"`
	SchemaType    string              `json:"schemaType"`
	SchemaItems   string              `json:"schemaItems,omitempty"`
	SchemaCustom  string              `json:"schemaCustom,omitempty"`
	AllowedValues []FieldAllowedValue `json:"allowedValues,omitempty"`
	Example       json.RawMessage     `json:"example,omitempty"`
}

// FieldAllowedValue is a single permissible value for an IssueField.
// ID is empty when Jira returns a primitive-string allowed value.
type FieldAllowedValue struct {
	ID    string `json:"id,omitempty"`
	Value string `json:"value"`
}

// IssueService defines operations for managing Jira issues.
type IssueService interface {
	// Create creates a new issue and returns it with Key and other
	// server-assigned fields populated.
	Create(ctx context.Context, issue *Issue) (*Issue, error)

	// Get retrieves an issue by its key.
	Get(ctx context.Context, key string) (*Issue, error)

	// List returns one page of issues matching the filter criteria, at most
	// filter.Limit long. next is the cursor for the following page, to be passed
	// back as filter.PageToken; it is empty when there are no more results, so
	// a non-empty next means the result was truncated.
	List(ctx context.Context, filter IssueFilter) (issues []*Issue, next string, err error)

	// Update modifies an existing issue and returns it as read back after
	// the write: the resulting values and updated time that issue update
	// reports.
	Update(ctx context.Context, key string, update IssueUpdate) (*Issue, error)

	// Delete deletes an issue by its key.
	// When deleteSubtasks is true, sub-tasks of the issue are also deleted;
	// otherwise Jira refuses to delete an issue that has sub-tasks.
	Delete(ctx context.Context, key string, deleteSubtasks bool) error

	// AddComment adds a comment to an issue.
	// The body is an ADF document; conversion from markdown happens at CLI boundary.
	AddComment(ctx context.Context, key string, body *ADFNode) (*Comment, error)

	// DeleteComment deletes a comment from an issue.
	DeleteComment(ctx context.Context, key, commentID string) error

	// Transitions returns available workflow transitions for an issue.
	Transitions(ctx context.Context, key string) ([]*Transition, error)

	// Transition moves an issue to a new status.
	Transition(ctx context.Context, key, transitionID string) error

	// Assign assigns an issue to a user by account ID.
	Assign(ctx context.Context, key, accountID string) error

	// Link creates a link between two issues.
	// The linkType is the name of the link type (e.g., "Blocks").
	// The inwardKey is the issue that has the relationship, and the outwardKey
	// is the issue it points to. For example, Link(ctx, "A", "Blocks", "B")
	// means that issue A blocks issue B (A is the blocker, B is blocked).
	Link(ctx context.Context, inwardKey, linkType, outwardKey string) error

	// Unlink removes a link between two issues.
	// It finds and removes any link connecting the two issues.
	Unlink(ctx context.Context, key1, key2 string) error

	// GetCreateFields returns the fields settable on issue create for the given
	// project and issue type. Backed by Jira's createmeta endpoint.
	GetCreateFields(ctx context.Context, projectKey, issueType string) ([]*IssueField, error)

	// GetEditFields returns the fields editable on the specified issue. Backed
	// by Jira's editmeta endpoint. Reflects current workflow + screen + permissions
	// for this specific issue, so results may differ from GetCreateFields.
	GetEditFields(ctx context.Context, key string) ([]*IssueField, error)
}
