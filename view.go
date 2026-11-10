package jira4claude

import (
	"bytes"
	"encoding/json"
	"slices"
	"strconv"
	"strings"
	"time"
)

// IssueView is a display-ready representation of an issue with ADF converted to markdown.
type IssueView struct {
	Key           string             `json:"key"`
	Project       string             `json:"project,omitempty"`
	Summary       string             `json:"summary"`
	Description   string             `json:"description,omitempty"`
	Status        string             `json:"status"`
	Type          string             `json:"type"`
	Priority      string             `json:"priority,omitempty"`
	Assignee      string             `json:"assignee,omitempty"`
	Reporter      string             `json:"reporter,omitempty"`
	Labels        []string           `json:"labels,omitempty"`
	RelatedIssues []RelatedIssueView `json:"relatedIssues"`
	Comments      []CommentView      `json:"comments,omitempty"`
	Attachments   []AttachmentView   `json:"attachments,omitempty"`
	Created       string             `json:"created"`
	Updated       string             `json:"updated"`
	URL           string             `json:"url,omitempty"`

	// CustomFields are read-only custom fields keyed by field ID, passed
	// through from the issue as-is (no ADF conversion). The JSON view always
	// carries all of them.
	CustomFields map[string]CustomFieldValue `json:"customFields,omitempty"`

	// AllCustomFields asks the text view to show every custom field (issue
	// view --all-fields). Without it the text view hides the empty and
	// machine-only ones; see SelectCustomFields.
	AllCustomFields bool `json:"-"`
}

// MarshalJSON ensures RelatedIssues is always an array, never null.
func (v IssueView) MarshalJSON() ([]byte, error) {
	type Alias IssueView
	if v.RelatedIssues == nil {
		v.RelatedIssues = []RelatedIssueView{}
	}
	return json.Marshal(Alias(v))
}

// CommentView is a display-ready representation of a comment with ADF converted to markdown.
type CommentView struct {
	ID      string `json:"id"`
	Author  string `json:"author"`
	Body    string `json:"body"`
	Created string `json:"created"`
}

// RelatedIssueView is a unified display-ready representation of a related issue.
// It consolidates parents, subtasks, and links into a single format.
type RelatedIssueView struct {
	Relationship string `json:"relationship"` // "parent", "subtask", or link type (e.g., "blocks", "is blocked by")
	Key          string `json:"key"`
	Type         string `json:"type"`   // "Epic", "Task", "Sub-task", etc.
	Status       string `json:"status"` // "To Do", "In Progress", "Done", etc.
	Summary      string `json:"summary"`
}

// ToIssueView converts a domain Issue to a display-ready IssueView.
// The converter is used to convert ADF to markdown, and every conversion issue
// is passed to the warn callback. Reads never fail on conversion issues, even
// dropped ones.
func ToIssueView(issue *Issue, conv Converter, warn func(ConversionIssue), serverURL string) IssueView {
	var description string
	if issue.Description != nil {
		desc, warnings := conv.ToMarkdown(issue.Description)
		description = desc
		for _, w := range warnings {
			warn(w)
		}
	}

	comments := make([]CommentView, 0, len(issue.Comments))
	for _, c := range issue.Comments {
		body, warnings := conv.ToMarkdown(c.Body)
		for _, w := range warnings {
			warn(w)
		}
		comments = append(comments, CommentView{
			ID:      c.ID,
			Author:  displayName(c.Author),
			Body:    body,
			Created: c.Created.Format(time.RFC3339),
		})
	}

	relatedIssues := ToRelatedIssuesView(issue)

	var url string
	if serverURL != "" {
		url = serverURL + "/browse/" + issue.Key
	}

	return IssueView{
		Key:           issue.Key,
		Project:       issue.Project,
		Summary:       issue.Summary,
		Description:   description,
		Status:        issue.Status,
		Type:          issue.Type,
		Priority:      issue.Priority,
		Assignee:      displayName(issue.Assignee),
		Reporter:      displayName(issue.Reporter),
		Labels:        issue.Labels,
		RelatedIssues: relatedIssues,
		Comments:      comments,
		Attachments:   ToAttachmentViews(issue.Attachments),
		Created:       issue.Created.Format(time.RFC3339),
		Updated:       FormatTimestamp(issue.Updated),
		URL:           url,
		CustomFields:  issue.ReadCustomFields,
	}
}

// customFieldPrefix starts the ID of every Jira custom field.
const customFieldPrefix = "customfield_"

// ViewFieldNames returns the built-in field names issue view --field accepts:
// the keys of the JSON view, in its order. A custom field is selected by its
// ID (customfield_NNNNN) instead; it has no name here.
func ViewFieldNames() []string {
	return []string{
		"key", "project", "summary", "description", "status", "type", "priority",
		"assignee", "reporter", "labels", "relatedIssues", "comments", "attachments",
		"created", "updated", "url",
	}
}

// ParseViewFields checks the field names given to issue view --field and
// returns them as ViewFieldNames spells them (custom field IDs in lower case),
// in the order given, each once. Names match without regard to case. A custom
// field is named by its ID, customfield_ followed by digits; display names are
// not accepted, because they are known only after the issue has been read,
// and a typo must fail before any request. An unknown name is a validation
// error that lists the valid ones.
func ParseViewFields(names []string) ([]string, error) {
	builtin := ViewFieldNames()
	fields := make([]string, 0, len(names))
	for _, name := range names {
		field, ok := canonicalViewField(name, builtin)
		if !ok {
			return nil, &Error{
				Code: EValidation,
				Message: "unknown field " + strconv.Quote(name) + "; valid fields: " + strings.Join(builtin, ", ") +
					", or a custom field ID such as customfield_10016 (see issue view --json, customFields)",
			}
		}
		if !slices.Contains(fields, field) {
			fields = append(fields, field)
		}
	}
	return fields, nil
}

// canonicalViewField returns the spelling of name in builtin, or the lower
// case form of a custom field ID, and whether name is either.
func canonicalViewField(name string, builtin []string) (string, bool) {
	for _, b := range builtin {
		if strings.EqualFold(name, b) {
			return b, true
		}
	}
	lower := strings.ToLower(name)
	digits, ok := strings.CutPrefix(lower, customFieldPrefix)
	if !ok || digits == "" {
		return "", false
	}
	for _, r := range digits {
		if r < '0' || r > '9' {
			return "", false
		}
	}
	return lower, true
}

// ViewFieldValue is one selected field of an issue view. Value is a string
// for the scalar fields, []string for labels, []CommentView for comments,
// []AttachmentView for attachments, []RelatedIssueView for relatedIssues, and the raw API json.RawMessage for a
// custom field (nil, which encodes as null, when the issue does not set it).
type ViewFieldValue struct {
	Name  string
	Value any
}

// SelectedIssueView is the fields of one issue that issue view --field
// selected, in the order asked. It encodes as a JSON object whose first key
// is "key", followed by the selected fields ("key" itself only once).
type SelectedIssueView struct {
	Key    string
	Fields []ViewFieldValue
}

// MarshalJSON encodes the selection as an object that keeps the field order.
func (s SelectedIssueView) MarshalJSON() ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteString(`{"key":`)
	key, err := json.Marshal(s.Key)
	if err != nil {
		return nil, err
	}
	buf.Write(key)
	for _, f := range s.Fields {
		if f.Name == "key" {
			continue
		}
		name, err := json.Marshal(f.Name)
		if err != nil {
			return nil, err
		}
		value, err := json.Marshal(f.Value)
		if err != nil {
			return nil, err
		}
		buf.WriteByte(',')
		buf.Write(name)
		buf.WriteByte(':')
		buf.Write(value)
	}
	buf.WriteByte('}')
	return buf.Bytes(), nil
}

// IssueSelectionView is the output of issue view --field: the selected fields
// of every issue that was read, in the order the keys were asked.
//
// Several records whether more than one key was asked; it decides the shape
// (a list rather than a single record) even when only one issue could be read.
type IssueSelectionView struct {
	Fields  []string
	Issues  []SelectedIssueView
	Several bool
}

// ToSelectedIssueView converts issue for issue view --field: it converts, and
// passes to warn the conversion issues of, only the Markdown fields selected
// (description, comments), then picks fields out of the view.
func ToSelectedIssueView(issue *Issue, fields []string, conv Converter, warn func(ConversionIssue), serverURL string) SelectedIssueView {
	read := *issue
	if !slices.Contains(fields, "description") {
		read.Description = nil
	}
	if !slices.Contains(fields, "comments") {
		read.Comments = nil
	}
	return SelectViewFields(ToIssueView(&read, conv, warn, serverURL), fields)
}

// ViewConversionIssue is a conversion issue met while reading an issue for
// issue view. Key is the issue's key as Jira returned it. Several records
// whether more than one key was asked: the text printer names the issue only
// then, so that one-key output stays as it was.
type ViewConversionIssue struct {
	ConversionIssue
	Key     string
	Several bool
}

// SelectViewFields picks fields, as returned by ParseViewFields, out of view.
// List values are never nil, so that they encode as [].
func SelectViewFields(view IssueView, fields []string) SelectedIssueView {
	selected := SelectedIssueView{Key: view.Key, Fields: make([]ViewFieldValue, 0, len(fields))}
	for _, f := range fields {
		selected.Fields = append(selected.Fields, ViewFieldValue{Name: f, Value: viewFieldValue(view, f)})
	}
	return selected
}

func viewFieldValue(view IssueView, field string) any {
	switch field {
	case "key":
		return view.Key
	case "project":
		return view.Project
	case "summary":
		return view.Summary
	case "description":
		return view.Description
	case "status":
		return view.Status
	case "type":
		return view.Type
	case "priority":
		return view.Priority
	case "assignee":
		return view.Assignee
	case "reporter":
		return view.Reporter
	case "labels":
		return nonNil(view.Labels)
	case "relatedIssues":
		return nonNil(view.RelatedIssues)
	case "comments":
		return nonNil(view.Comments)
	case "attachments":
		return nonNil(view.Attachments)
	case "created":
		return view.Created
	case "updated":
		return view.Updated
	case "url":
		return view.URL
	}
	cf, ok := view.CustomFields[field]
	if !ok {
		return json.RawMessage(nil)
	}
	return cf.Value
}

func nonNil[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}

// IssueListItem is a minimal representation for list display.
// It contains only the fields needed for issue lists, avoiding
// expensive ADF-to-markdown conversion. Assignee is the display name, empty
// (and left out of the JSON, as in IssueView) when the issue is unassigned.
type IssueListItem struct {
	Key      string `json:"key"`
	Status   string `json:"status"`
	Type     string `json:"type"`
	Priority string `json:"priority,omitempty"`
	Assignee string `json:"assignee,omitempty"`
	Summary  string `json:"summary"`
}

// IssueListView is a list of issues plus whether more matching issues exist
// beyond those shown.
type IssueListView struct {
	Issues    []IssueListItem `json:"issues"`
	Truncated bool            `json:"truncated"`
}

// IssueFieldsView is a display-ready representation of issue field metadata.
// Source describes the context (e.g., "INT / Bug" for create-mode, "INT-1118 (edit)" for edit-mode).
// Scope records which selection produced Fields, and Omitted is the number of
// settable fields not shown under that selection (so an agent knows there is
// more behind --all / --filter).
type IssueFieldsView struct {
	Source  string        `json:"source"`
	Scope   string        `json:"scope"`
	Omitted int           `json:"omitted"`
	Fields  []*IssueField `json:"fields"`
}

// Field selection scopes reported by SelectIssueFields and surfaced in
// IssueFieldsView.Scope.
const (
	FieldScopeDefault  = "default"  // required + custom fields
	FieldScopeAll      = "all"      // every settable field
	FieldScopeFiltered = "filtered" // substring match across all fields
)

// SelectIssueFields chooses which settable fields to display and reports the
// scope plus the count of fields omitted by that selection.
//
//   - When filter is non-empty, it matches (case-insensitively) the field name
//     or ID across ALL fields, ignoring all; scope is "filtered". This lets a
//     caller look up exactly the field a create error named.
//   - Otherwise when all is true, every field is returned; scope is "all".
//   - Otherwise required and custom (customfield_*) fields are returned; scope
//     is "default".
//
// Input order is preserved; callers rely on fields already being sorted.
func SelectIssueFields(fields []*IssueField, filter string, all bool) (selected []*IssueField, scope string, omitted int) {
	switch {
	case filter != "":
		needle := strings.ToLower(filter)
		for _, f := range fields {
			if strings.Contains(strings.ToLower(f.Name), needle) ||
				strings.Contains(strings.ToLower(f.ID), needle) {
				selected = append(selected, f)
			}
		}
		scope = FieldScopeFiltered
	case all:
		selected = fields
		scope = FieldScopeAll
	default:
		for _, f := range fields {
			if f.Required || strings.HasPrefix(f.ID, "customfield_") {
				selected = append(selected, f)
			}
		}
		scope = FieldScopeDefault
	}
	return selected, scope, len(fields) - len(selected)
}

// MachineOnlyFieldTypes returns the custom field types (schema custom keys)
// whose values are for Jira's own use, not for a reader: the board rank and
// the development panel summary. issue view hides fields of these types.
//
// The rank key is documented by third-party field references. The development
// summary key is unverified against a live site: sources give both
// "devsummarycf" and "devsummary", so both are listed.
func MachineOnlyFieldTypes() []string {
	return []string{
		"com.pyxis.greenhopper.jira:gh-lexo-rank",
		"com.atlassian.jira.plugins.jira-development-integration-plugin:devsummarycf",
		"com.atlassian.jira.plugins.jira-development-integration-plugin:devsummary",
	}
}

// SelectCustomFields chooses the custom fields the issue view text shows and
// counts the ones it hides, as SelectIssueFields does for issue fields.
//
//   - When all is true, every field is shown and none hidden.
//   - Otherwise a field is hidden when its value is empty ([], {}, "" or
//     null, whatever the whitespace) or its custom type is one of
//     MachineOnlyFieldTypes. Fields are told apart by type, never by display
//     name; a field whose type is unknown is hidden only when empty.
func SelectCustomFields(fields map[string]CustomFieldValue, all bool) (shown map[string]CustomFieldValue, hidden int) {
	if all {
		return fields, 0
	}
	shown = make(map[string]CustomFieldValue, len(fields))
	for id, cf := range fields {
		if isEmptyJSON(cf.Value) || slices.Contains(MachineOnlyFieldTypes(), cf.SchemaCustom) {
			hidden++
			continue
		}
		shown[id] = cf
	}
	return shown, hidden
}

// isEmptyJSON reports whether raw is [], {}, "" or null (or nothing at all).
func isEmptyJSON(raw json.RawMessage) bool {
	if len(bytes.TrimSpace(raw)) == 0 {
		return true
	}
	var buf bytes.Buffer
	if err := json.Compact(&buf, raw); err != nil {
		return false
	}
	switch buf.String() {
	case "[]", "{}", `""`, "null":
		return true
	}
	return false
}

// ToIssueListItems converts domain issues to list items.
// No ADF conversion is performed — this is a direct field copy.
func ToIssueListItems(issues []*Issue) []IssueListItem {
	items := make([]IssueListItem, len(issues))
	for i, issue := range issues {
		items[i] = IssueListItem{
			Key:      issue.Key,
			Status:   issue.Status,
			Type:     issue.Type,
			Priority: issue.Priority,
			Assignee: displayName(issue.Assignee),
			Summary:  issue.Summary,
		}
	}
	return items
}

// ToCommentView converts a domain Comment to a display-ready CommentView.
// Every conversion issue is passed to the warn callback.
func ToCommentView(comment *Comment, conv Converter, warn func(ConversionIssue)) CommentView {
	body, warnings := conv.ToMarkdown(comment.Body)
	for _, w := range warnings {
		warn(w)
	}
	return CommentView{
		ID:      comment.ID,
		Author:  displayName(comment.Author),
		Body:    body,
		Created: comment.Created.Format(time.RFC3339),
	}
}

// ToLinksView converts a slice of domain IssueLinks to RelatedIssueViews.
// The relationship field uses the link type's outward/inward description.
func ToLinksView(links []*IssueLink) []RelatedIssueView {
	var outward []RelatedIssueView
	var inward []RelatedIssueView

	for _, link := range links {
		if link.OutwardIssue != nil {
			outward = append(outward, RelatedIssueView{
				Relationship: link.Type.Outward,
				Key:          link.OutwardIssue.Key,
				Type:         link.OutwardIssue.Type,
				Status:       link.OutwardIssue.Status,
				Summary:      link.OutwardIssue.Summary,
			})
		}
		if link.InwardIssue != nil {
			inward = append(inward, RelatedIssueView{
				Relationship: link.Type.Inward,
				Key:          link.InwardIssue.Key,
				Type:         link.InwardIssue.Type,
				Status:       link.InwardIssue.Status,
				Summary:      link.InwardIssue.Summary,
			})
		}
	}

	// Order: outward links first, then inward links
	views := make([]RelatedIssueView, 0, len(outward)+len(inward))
	views = append(views, outward...)
	views = append(views, inward...)
	return views
}

func displayName(user *User) string {
	if user == nil {
		return ""
	}
	return user.DisplayName
}

// ToRelatedIssuesView converts all related issues (parent, subtasks, links) into a unified slice.
// Results are ordered: parent → subtasks → outward links → inward links.
func ToRelatedIssuesView(issue *Issue) []RelatedIssueView {
	// Pre-allocate capacity: parent(1) + subtasks + links*2 (outward + inward)
	parentCount := 0
	if issue.Parent != nil {
		parentCount = 1
	}
	cap := parentCount + len(issue.Subtasks) + len(issue.Links)*2
	related := make([]RelatedIssueView, 0, cap)

	// 1. Parent (at most one)
	if issue.Parent != nil {
		related = append(related, RelatedIssueView{
			Relationship: "parent",
			Key:          issue.Parent.Key,
			Type:         issue.Parent.Type,
			Status:       issue.Parent.Status,
			Summary:      issue.Parent.Summary,
		})
	}

	// 2. Subtasks (or epic children)
	for _, subtask := range issue.Subtasks {
		related = append(related, RelatedIssueView{
			Relationship: "subtask",
			Key:          subtask.Key,
			Type:         subtask.Type,
			Status:       subtask.Status,
			Summary:      subtask.Summary,
		})
	}

	// 3. Links - split into outward and inward
	var outward []RelatedIssueView
	var inward []RelatedIssueView

	for _, link := range issue.Links {
		if link.OutwardIssue != nil {
			outward = append(outward, RelatedIssueView{
				Relationship: link.Type.Outward,
				Key:          link.OutwardIssue.Key,
				Type:         link.OutwardIssue.Type,
				Status:       link.OutwardIssue.Status,
				Summary:      link.OutwardIssue.Summary,
			})
		}
		if link.InwardIssue != nil {
			inward = append(inward, RelatedIssueView{
				Relationship: link.Type.Inward,
				Key:          link.InwardIssue.Key,
				Type:         link.InwardIssue.Type,
				Status:       link.InwardIssue.Status,
				Summary:      link.InwardIssue.Summary,
			})
		}
	}

	// Append in order: outward links first, then inward links
	related = append(related, outward...)
	related = append(related, inward...)

	return related
}
