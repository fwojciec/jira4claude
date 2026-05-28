package jira4claude

import (
	"encoding/json"
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
	Created       string             `json:"created"`
	Updated       string             `json:"updated"`
	URL           string             `json:"url,omitempty"`

	// CustomFields are read-only custom fields keyed by field ID, passed
	// through from the issue as-is (no ADF conversion).
	CustomFields map[string]CustomFieldValue `json:"customFields,omitempty"`
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
// The converter is used to convert ADF to markdown, and any warnings are passed to the warn callback.
func ToIssueView(issue *Issue, conv Converter, warn func(string), serverURL string) IssueView {
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
		Created:       issue.Created.Format(time.RFC3339),
		Updated:       issue.Updated.Format(time.RFC3339),
		URL:           url,
		CustomFields:  issue.ReadCustomFields,
	}
}

// IssueListItem is a minimal representation for list display.
// It contains only the fields needed for issue lists, avoiding
// expensive ADF-to-markdown conversion.
type IssueListItem struct {
	Key      string `json:"key"`
	Status   string `json:"status"`
	Priority string `json:"priority,omitempty"`
	Summary  string `json:"summary"`
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

// ToIssueListItems converts domain issues to list items.
// No ADF conversion is performed — this is a direct field copy.
func ToIssueListItems(issues []*Issue) []IssueListItem {
	items := make([]IssueListItem, len(issues))
	for i, issue := range issues {
		items[i] = IssueListItem{
			Key:      issue.Key,
			Status:   issue.Status,
			Priority: issue.Priority,
			Summary:  issue.Summary,
		}
	}
	return items
}

// ToCommentView converts a domain Comment to a display-ready CommentView.
func ToCommentView(comment *Comment, conv Converter, warn func(string)) CommentView {
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
