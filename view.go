package jira4claude

import "time"

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
	RelatedIssues []RelatedIssueView `json:"relatedIssues,omitempty"`
	Comments      []CommentView      `json:"comments,omitempty"`
	Created       string             `json:"created"`
	Updated       string             `json:"updated"`
	URL           string             `json:"url,omitempty"`
}

// CommentView is a display-ready representation of a comment with ADF converted to markdown.
type CommentView struct {
	ID      string `json:"id"`
	Author  string `json:"author"`
	Body    string `json:"body"`
	Created string `json:"created"`
}

// RelatedIssueView is a unified display-ready representation of a related issue.
// It consolidates parents, children, subtasks, and links into a single format.
type RelatedIssueView struct {
	Relationship string `json:"relationship"` // "parent", "child", "subtask", "blocks", "is blocked by"
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
	}
}

// ToIssuesView converts a slice of domain Issues to display-ready IssueViews.
func ToIssuesView(issues []*Issue, conv Converter, warn func(string), serverURL string) []IssueView {
	views := make([]IssueView, len(issues))
	for i, issue := range issues {
		views[i] = ToIssueView(issue, conv, warn, serverURL)
	}
	return views
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
// The relationship field uses the link type's outward/inward description (e.g., "blocks", "is blocked by").
func ToLinksView(links []*IssueLink) []RelatedIssueView {
	var views []RelatedIssueView
	var blocks []RelatedIssueView
	var blockedBy []RelatedIssueView

	for _, link := range links {
		if link.OutwardIssue != nil {
			blocks = append(blocks, RelatedIssueView{
				Relationship: link.Type.Outward,
				Key:          link.OutwardIssue.Key,
				Type:         link.OutwardIssue.Type,
				Status:       link.OutwardIssue.Status,
				Summary:      link.OutwardIssue.Summary,
			})
		}
		if link.InwardIssue != nil {
			blockedBy = append(blockedBy, RelatedIssueView{
				Relationship: link.Type.Inward,
				Key:          link.InwardIssue.Key,
				Type:         link.InwardIssue.Type,
				Status:       link.InwardIssue.Status,
				Summary:      link.InwardIssue.Summary,
			})
		}
	}

	// Order: blocks (outward) first, then is blocked by (inward)
	views = append(views, blocks...)
	views = append(views, blockedBy...)
	return views
}

func displayName(user *User) string {
	if user == nil {
		return ""
	}
	return user.DisplayName
}

// ToRelatedIssuesView converts all related issues (parent, children, subtasks, links) into a unified slice.
// Results are ordered: parent → children → subtasks → blocks → is blocked by.
func ToRelatedIssuesView(issue *Issue) []RelatedIssueView {
	// Pre-allocate capacity: parent(1) + children + subtasks + links*2 (outward + inward)
	parentCount := 0
	if issue.Parent != nil {
		parentCount = 1
	}
	cap := parentCount + len(issue.Children) + len(issue.Subtasks) + len(issue.Links)*2
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

	// 2. Children (for epics)
	for _, child := range issue.Children {
		related = append(related, RelatedIssueView{
			Relationship: "child",
			Key:          child.Key,
			Type:         child.Type,
			Status:       child.Status,
			Summary:      child.Summary,
		})
	}

	// 3. Subtasks
	for _, subtask := range issue.Subtasks {
		related = append(related, RelatedIssueView{
			Relationship: "subtask",
			Key:          subtask.Key,
			Type:         subtask.Type,
			Status:       subtask.Status,
			Summary:      subtask.Summary,
		})
	}

	// 4. Links - split into "blocks" (outward) and "is blocked by" (inward)
	var blocks []RelatedIssueView
	var blockedBy []RelatedIssueView

	for _, link := range issue.Links {
		if link.OutwardIssue != nil {
			blocks = append(blocks, RelatedIssueView{
				Relationship: link.Type.Outward,
				Key:          link.OutwardIssue.Key,
				Type:         link.OutwardIssue.Type,
				Status:       link.OutwardIssue.Status,
				Summary:      link.OutwardIssue.Summary,
			})
		}
		if link.InwardIssue != nil {
			blockedBy = append(blockedBy, RelatedIssueView{
				Relationship: link.Type.Inward,
				Key:          link.InwardIssue.Key,
				Type:         link.InwardIssue.Type,
				Status:       link.InwardIssue.Status,
				Summary:      link.InwardIssue.Summary,
			})
		}
	}

	// Append in order: blocks first, then is blocked by
	related = append(related, blocks...)
	related = append(related, blockedBy...)

	return related
}
