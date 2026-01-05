package jira4claude

import "time"

// IssueView is a display-ready representation of an issue with ADF converted to markdown.
type IssueView struct {
	Key         string        `json:"key"`
	Project     string        `json:"project,omitempty"`
	Summary     string        `json:"summary"`
	Description string        `json:"description,omitempty"`
	Status      string        `json:"status"`
	Type        string        `json:"type"`
	Priority    string        `json:"priority,omitempty"`
	Assignee    string        `json:"assignee,omitempty"`
	Reporter    string        `json:"reporter,omitempty"`
	Labels      []string      `json:"labels,omitempty"`
	Links       []LinkView    `json:"links,omitempty"`
	Subtasks    []SubtaskView `json:"subtasks,omitempty"`
	Comments    []CommentView `json:"comments,omitempty"`
	Parent      string        `json:"parent,omitempty"`
	Created     string        `json:"created"`
	Updated     string        `json:"updated"`
	URL         string        `json:"url,omitempty"`
}

// CommentView is a display-ready representation of a comment with ADF converted to markdown.
type CommentView struct {
	ID      string `json:"id"`
	Author  string `json:"author"`
	Body    string `json:"body"`
	Created string `json:"created"`
}

// LinkView is a display-ready representation of an issue link.
type LinkView struct {
	Type      string `json:"type"`
	Direction string `json:"direction"`
	IssueKey  string `json:"issueKey"`
	Summary   string `json:"summary"`
	Status    string `json:"status"`
}

// SubtaskView is a display-ready representation of a subtask.
type SubtaskView struct {
	Key     string `json:"key"`
	Summary string `json:"summary"`
	Status  string `json:"status"`
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

	links := ToLinksView(issue.Links)
	subtasks := ToSubtasksView(issue.Subtasks)

	var url string
	if serverURL != "" {
		url = serverURL + "/browse/" + issue.Key
	}

	return IssueView{
		Key:         issue.Key,
		Project:     issue.Project,
		Summary:     issue.Summary,
		Description: description,
		Status:      issue.Status,
		Type:        issue.Type,
		Priority:    issue.Priority,
		Assignee:    displayName(issue.Assignee),
		Reporter:    displayName(issue.Reporter),
		Labels:      issue.Labels,
		Links:       links,
		Subtasks:    subtasks,
		Comments:    comments,
		Parent:      parentKey(issue.Parent),
		Created:     issue.Created.Format(time.RFC3339),
		Updated:     issue.Updated.Format(time.RFC3339),
		URL:         url,
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

// ToLinksView converts a slice of domain IssueLinks to display-ready LinkViews.
func ToLinksView(links []*IssueLink) []LinkView {
	views := make([]LinkView, 0, len(links))
	for _, link := range links {
		if link.OutwardIssue != nil {
			views = append(views, LinkView{
				Type:      link.Type.Outward,
				Direction: "outward",
				IssueKey:  link.OutwardIssue.Key,
				Summary:   link.OutwardIssue.Summary,
				Status:    link.OutwardIssue.Status,
			})
		}
		if link.InwardIssue != nil {
			views = append(views, LinkView{
				Type:      link.Type.Inward,
				Direction: "inward",
				IssueKey:  link.InwardIssue.Key,
				Summary:   link.InwardIssue.Summary,
				Status:    link.InwardIssue.Status,
			})
		}
	}
	return views
}

// ToSubtasksView converts a slice of domain LinkedIssues to display-ready SubtaskViews.
func ToSubtasksView(subtasks []*LinkedIssue) []SubtaskView {
	if len(subtasks) == 0 {
		return nil
	}
	views := make([]SubtaskView, len(subtasks))
	for i, subtask := range subtasks {
		views[i] = SubtaskView{
			Key:     subtask.Key,
			Summary: subtask.Summary,
			Status:  subtask.Status,
		}
	}
	return views
}

func displayName(user *User) string {
	if user == nil {
		return ""
	}
	return user.DisplayName
}

func parentKey(parent *LinkedIssue) string {
	if parent == nil {
		return ""
	}
	return parent.Key
}
