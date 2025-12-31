package gogh

import (
	"encoding/json"
	"io"

	"github.com/fwojciec/jira4claude"
)

// JSONPrinter outputs JSON format to stdout for machine parsing.
type JSONPrinter struct {
	out       io.Writer
	serverURL string
}

// SetServerURL sets the server URL for generating issue URLs.
func (p *JSONPrinter) SetServerURL(url string) {
	p.serverURL = url
}

// NewJSONPrinter creates a JSON printer that writes to out.
func NewJSONPrinter(out io.Writer) *JSONPrinter {
	return &JSONPrinter{out: out}
}

func (p *JSONPrinter) encode(v any) {
	enc := json.NewEncoder(p.out)
	enc.SetIndent("", "  ")
	// Error ignored: encoding known map structures should not fail.
	// If the writer fails, CLI output has no useful recovery path.
	_ = enc.Encode(v)
}

// Issue prints a single issue as JSON.
func (p *JSONPrinter) Issue(issue *jira4claude.Issue) {
	m := issueToMap(issue)
	if p.serverURL != "" {
		m["url"] = p.serverURL + "/browse/" + issue.Key
	}
	p.encode(m)
}

// Issues prints multiple issues as JSON array.
func (p *JSONPrinter) Issues(issues []*jira4claude.Issue) {
	result := make([]map[string]any, len(issues))
	for i, issue := range issues {
		result[i] = issueToMap(issue)
	}
	p.encode(result)
}

// Transitions prints transitions as JSON array.
func (p *JSONPrinter) Transitions(_ string, ts []*jira4claude.Transition) {
	result := make([]map[string]any, len(ts))
	for i, t := range ts {
		result[i] = map[string]any{"id": t.ID, "name": t.Name}
	}
	p.encode(result)
}

// Links prints links as JSON array.
func (p *JSONPrinter) Links(_ string, links []*jira4claude.IssueLink) {
	result := make([]map[string]any, len(links))
	for i, link := range links {
		result[i] = linkToMap(link)
	}
	p.encode(result)
}

// Success prints a success message as JSON.
func (p *JSONPrinter) Success(msg string, keys ...string) {
	result := map[string]any{
		"success": true,
		"message": msg,
	}
	if len(keys) > 0 {
		result["keys"] = keys
		if p.serverURL != "" {
			urls := make([]string, len(keys))
			for i, k := range keys {
				urls[i] = p.serverURL + "/browse/" + k
			}
			result["urls"] = urls
		}
	}
	p.encode(result)
}

// Error prints an error as JSON to stdout (for machine parsing).
func (p *JSONPrinter) Error(err error) {
	p.encode(map[string]any{
		"error":   true,
		"code":    jira4claude.ErrorCode(err),
		"message": jira4claude.ErrorMessage(err),
	})
}

func issueToMap(issue *jira4claude.Issue) map[string]any {
	m := map[string]any{
		"key":         issue.Key,
		"project":     issue.Project,
		"summary":     issue.Summary,
		"description": issue.Description,
		"status":      issue.Status,
		"type":        issue.Type,
		"priority":    issue.Priority,
		"labels":      issue.Labels,
		"parent":      issue.Parent,
		"created":     issue.Created,
		"updated":     issue.Updated,
	}
	if issue.Assignee != nil {
		m["assignee"] = userToMap(issue.Assignee)
	}
	if issue.Reporter != nil {
		m["reporter"] = userToMap(issue.Reporter)
	}
	if len(issue.Links) > 0 {
		links := make([]map[string]any, len(issue.Links))
		for i, link := range issue.Links {
			links[i] = linkToMap(link)
		}
		m["links"] = links
	}
	if len(issue.Comments) > 0 {
		comments := make([]map[string]any, len(issue.Comments))
		for i, comment := range issue.Comments {
			comments[i] = commentToMap(comment)
		}
		m["comments"] = comments
	}
	return m
}

func commentToMap(comment *jira4claude.Comment) map[string]any {
	m := map[string]any{
		"id":      comment.ID,
		"body":    comment.Body,
		"created": comment.Created,
	}
	if comment.Author != nil {
		m["author"] = userToMap(comment.Author)
	}
	return m
}

func userToMap(user *jira4claude.User) map[string]any {
	return map[string]any{
		"accountId":   user.AccountID,
		"displayName": user.DisplayName,
		"email":       user.Email,
	}
}

func linkToMap(link *jira4claude.IssueLink) map[string]any {
	lm := map[string]any{
		"id": link.ID,
		"type": map[string]any{
			"name":    link.Type.Name,
			"inward":  link.Type.Inward,
			"outward": link.Type.Outward,
		},
	}
	if link.OutwardIssue != nil {
		lm["outwardIssue"] = linkedIssueToMap(link.OutwardIssue)
	}
	if link.InwardIssue != nil {
		lm["inwardIssue"] = linkedIssueToMap(link.InwardIssue)
	}
	return lm
}

func linkedIssueToMap(issue *jira4claude.LinkedIssue) map[string]any {
	return map[string]any{
		"key":     issue.Key,
		"summary": issue.Summary,
		"status":  issue.Status,
		"type":    issue.Type,
	}
}

// Verify interface compliance at compile time.
var _ jira4claude.Printer = (*JSONPrinter)(nil)
