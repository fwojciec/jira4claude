package http

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/fwojciec/jira4claude"
)

// IssueService implements jira4claude.IssueService using the Jira REST API.
type IssueService struct {
	client *Client
}

// issuePath builds an escaped URL path for issue API endpoints.
func issuePath(key string, segments ...string) string {
	path := "/rest/api/3/issue/" + url.PathEscape(key)
	for _, seg := range segments {
		path += "/" + url.PathEscape(seg)
	}
	return path
}

// Compile-time interface verification.
var _ jira4claude.IssueService = (*IssueService)(nil)

// NewIssueService creates a new IssueService using the provided HTTP client.
func NewIssueService(client *Client) *IssueService {
	return &IssueService{client: client}
}

// Create creates a new issue and returns it with Key populated.
func (s *IssueService) Create(ctx context.Context, issue *jira4claude.Issue) (*jira4claude.Issue, error) {
	// Build request body
	fields := map[string]any{
		"project":   map[string]any{"key": issue.Project},
		"summary":   issue.Summary,
		"issuetype": map[string]any{"name": issue.Type},
	}

	if issue.Description != "" {
		fields["description"] = textOrADF(issue.Description)
	}
	if issue.Priority != "" {
		fields["priority"] = map[string]any{"name": issue.Priority}
	}
	if len(issue.Labels) > 0 {
		fields["labels"] = issue.Labels
	}
	if issue.Parent != "" {
		fields["parent"] = map[string]any{"key": issue.Parent}
	}

	req, err := s.client.NewJSONRequest(ctx, http.MethodPost, "/rest/api/3/issue", map[string]any{"fields": fields})
	if err != nil {
		return nil, err
	}

	respBody, err := s.client.DoRequest(req, http.StatusCreated)
	if err != nil {
		return nil, err
	}

	// Parse response
	var createResp struct {
		Key string `json:"key"`
	}
	if err := json.Unmarshal(respBody, &createResp); err != nil {
		return nil, &jira4claude.Error{
			Code:    jira4claude.EInternal,
			Message: "failed to parse response",
			Inner:   err,
		}
	}

	// Return issue with key populated
	result := *issue
	result.Key = createResp.Key
	return &result, nil
}

// Get retrieves an issue by its key.
func (s *IssueService) Get(ctx context.Context, key string) (*jira4claude.Issue, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, issuePath(key), nil)
	if err != nil {
		return nil, &jira4claude.Error{
			Code:    jira4claude.EInternal,
			Message: "failed to create request",
			Inner:   err,
		}
	}

	body, err := s.client.DoRequest(req, http.StatusOK)
	if err != nil {
		return nil, err
	}

	return parseIssueResponse(body)
}

// List returns issues matching the filter criteria.
func (s *IssueService) List(ctx context.Context, filter jira4claude.IssueFilter) ([]*jira4claude.Issue, error) {
	// Build JQL query
	jql := filter.JQL
	if jql == "" {
		jql = buildJQL(filter)
	}

	// Build request URL with query parameters
	// The /search/jql endpoint requires explicit field selection
	fields := "key,summary,status,issuetype,project,priority,assignee,reporter,labels,issuelinks,parent,created,updated,description"
	reqURL := "/rest/api/3/search/jql?jql=" + url.QueryEscape(jql) + "&fields=" + fields
	if filter.Limit > 0 {
		reqURL += "&maxResults=" + strconv.Itoa(filter.Limit)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, &jira4claude.Error{
			Code:    jira4claude.EInternal,
			Message: "failed to create request",
			Inner:   err,
		}
	}

	respBody, err := s.client.DoRequest(req, http.StatusOK)
	if err != nil {
		return nil, err
	}

	// Parse response
	var searchResp struct {
		Issues []json.RawMessage `json:"issues"`
	}
	if err := json.Unmarshal(respBody, &searchResp); err != nil {
		return nil, &jira4claude.Error{
			Code:    jira4claude.EInternal,
			Message: "failed to parse response",
			Inner:   err,
		}
	}

	issues := make([]*jira4claude.Issue, 0, len(searchResp.Issues))
	for _, raw := range searchResp.Issues {
		issue, err := parseIssueResponse(raw)
		if err != nil {
			return nil, err
		}
		issues = append(issues, issue)
	}

	return issues, nil
}

// buildJQL constructs a JQL query from IssueFilter fields.
func buildJQL(filter jira4claude.IssueFilter) string {
	// Pre-allocate for max possible clauses: project, status, assignee, parent, + labels
	clauses := make([]string, 0, 4+len(filter.Labels))

	if filter.Project != "" {
		clauses = append(clauses, fmt.Sprintf("project = %q", filter.Project))
	}
	if filter.Status != "" {
		clauses = append(clauses, fmt.Sprintf("status = %q", filter.Status))
	}
	if filter.Assignee != "" {
		clauses = append(clauses, fmt.Sprintf("assignee = %q", filter.Assignee))
	}
	if filter.Parent != "" {
		clauses = append(clauses, fmt.Sprintf("parent = %q", filter.Parent))
	}
	for _, label := range filter.Labels {
		clauses = append(clauses, fmt.Sprintf("labels = %q", label))
	}

	return strings.Join(clauses, " AND ")
}

// Update modifies an existing issue and returns the updated issue.
func (s *IssueService) Update(ctx context.Context, key string, update jira4claude.IssueUpdate) (*jira4claude.Issue, error) {
	// Build request body with only the fields that are set
	fields := make(map[string]any)

	if update.Summary != nil {
		fields["summary"] = *update.Summary
	}
	if update.Description != nil {
		fields["description"] = textOrADF(*update.Description)
	}
	if update.Priority != nil {
		fields["priority"] = map[string]any{"name": *update.Priority}
	}
	if update.Assignee != nil {
		if *update.Assignee == "" {
			fields["assignee"] = nil
		} else {
			fields["assignee"] = map[string]any{"accountId": *update.Assignee}
		}
	}
	if update.Labels != nil {
		fields["labels"] = *update.Labels
	}

	req, err := s.client.NewJSONRequest(ctx, http.MethodPut, issuePath(key), map[string]any{"fields": fields})
	if err != nil {
		return nil, err
	}

	_, err = s.client.DoRequest(req, http.StatusNoContent)
	if err != nil {
		return nil, err
	}

	// Fetch and return the updated issue
	return s.Get(ctx, key)
}

// Delete deletes an issue by its key.
func (s *IssueService) Delete(ctx context.Context, key string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, issuePath(key), nil)
	if err != nil {
		return &jira4claude.Error{
			Code:    jira4claude.EInternal,
			Message: "failed to create request",
			Inner:   err,
		}
	}

	_, err = s.client.DoRequest(req, http.StatusNoContent)
	return err
}

// AddComment adds a comment to an issue.
func (s *IssueService) AddComment(ctx context.Context, key, body string) (*jira4claude.Comment, error) {
	reqBody := map[string]any{
		"body": textOrADF(body),
	}

	req, err := s.client.NewJSONRequest(ctx, http.MethodPost, issuePath(key, "comment"), reqBody)
	if err != nil {
		return nil, err
	}

	respBody, err := s.client.DoRequest(req, http.StatusCreated)
	if err != nil {
		return nil, err
	}

	return parseCommentResponse(respBody)
}

// Transitions returns available workflow transitions for an issue.
func (s *IssueService) Transitions(ctx context.Context, key string) ([]*jira4claude.Transition, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, issuePath(key, "transitions"), nil)
	if err != nil {
		return nil, &jira4claude.Error{
			Code:    jira4claude.EInternal,
			Message: "failed to create request",
			Inner:   err,
		}
	}

	respBody, err := s.client.DoRequest(req, http.StatusOK)
	if err != nil {
		return nil, err
	}

	var transitionsResp struct {
		Transitions []transitionResponse `json:"transitions"`
	}
	if err := json.Unmarshal(respBody, &transitionsResp); err != nil {
		return nil, &jira4claude.Error{
			Code:    jira4claude.EInternal,
			Message: "failed to parse response",
			Inner:   err,
		}
	}

	return mapTransitions(transitionsResp.Transitions), nil
}

// Transition moves an issue to a new status.
func (s *IssueService) Transition(ctx context.Context, key, transitionID string) error {
	reqBody := map[string]any{
		"transition": map[string]any{
			"id": transitionID,
		},
	}

	req, err := s.client.NewJSONRequest(ctx, http.MethodPost, issuePath(key, "transitions"), reqBody)
	if err != nil {
		return err
	}

	_, err = s.client.DoRequest(req, http.StatusNoContent)
	return err
}

// Assign assigns an issue to a user by account ID.
// If accountID is empty, the issue is unassigned.
func (s *IssueService) Assign(ctx context.Context, key, accountID string) error {
	var reqBody map[string]any
	if accountID == "" {
		reqBody = map[string]any{"accountId": nil}
	} else {
		reqBody = map[string]any{"accountId": accountID}
	}

	req, err := s.client.NewJSONRequest(ctx, http.MethodPut, issuePath(key, "assignee"), reqBody)
	if err != nil {
		return err
	}

	_, err = s.client.DoRequest(req, http.StatusNoContent)
	return err
}

// issueResponse represents the JSON structure returned by Jira API for an issue.
type issueResponse struct {
	Key    string `json:"key"`
	Fields struct {
		Project     struct{ Key string }  `json:"project"`
		Summary     string                `json:"summary"`
		Description map[string]any        `json:"description"`
		Status      struct{ Name string } `json:"status"`
		IssueType   struct{ Name string } `json:"issuetype"`
		Priority    struct{ Name string } `json:"priority"`
		Assignee    *userResponse         `json:"assignee"`
		Reporter    *userResponse         `json:"reporter"`
		Labels      []string              `json:"labels"`
		IssueLinks  []issueLinkResponse   `json:"issuelinks"`
		Comment     *commentsResponse     `json:"comment"`
		Parent      *struct{ Key string } `json:"parent"`
		Created     string                `json:"created"`
		Updated     string                `json:"updated"`
	} `json:"fields"`
}

// commentsResponse represents the comment container in the Jira API response.
type commentsResponse struct {
	Comments []commentAPIResponse `json:"comments"`
	Total    int                  `json:"total"`
}

// commentAPIResponse represents a single comment in the issue response.
type commentAPIResponse struct {
	ID      string         `json:"id"`
	Author  *userResponse  `json:"author"`
	Body    map[string]any `json:"body"`
	Created string         `json:"created"`
}

// issueLinkResponse represents a link in the Jira API response.
type issueLinkResponse struct {
	ID   string `json:"id"`
	Type struct {
		Name    string `json:"name"`
		Inward  string `json:"inward"`
		Outward string `json:"outward"`
	} `json:"type"`
	OutwardIssue *linkedIssueResponse `json:"outwardIssue"`
	InwardIssue  *linkedIssueResponse `json:"inwardIssue"`
}

// linkedIssueResponse represents a linked issue in the Jira API response.
type linkedIssueResponse struct {
	Key    string `json:"key"`
	Fields struct {
		Summary   string                `json:"summary"`
		Status    struct{ Name string } `json:"status"`
		IssueType struct{ Name string } `json:"issuetype"`
	} `json:"fields"`
}

type userResponse struct {
	AccountID    string `json:"accountId"`
	DisplayName  string `json:"displayName"`
	EmailAddress string `json:"emailAddress"`
}

// transitionResponse represents a single transition in the Jira API response.
type transitionResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// parseIssueResponse parses the JSON response from Jira into a domain Issue.
func parseIssueResponse(body []byte) (*jira4claude.Issue, error) {
	var resp issueResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, &jira4claude.Error{
			Code:    jira4claude.EInternal,
			Message: "failed to parse response",
			Inner:   err,
		}
	}

	issue := &jira4claude.Issue{
		Key:            resp.Key,
		Project:        resp.Fields.Project.Key,
		Summary:        resp.Fields.Summary,
		Description:    ADFToText(resp.Fields.Description),
		DescriptionADF: resp.Fields.Description,
		Status:         resp.Fields.Status.Name,
		Type:           resp.Fields.IssueType.Name,
		Priority:       resp.Fields.Priority.Name,
		Labels:         resp.Fields.Labels,
	}

	if resp.Fields.Parent != nil {
		issue.Parent = resp.Fields.Parent.Key
	}

	issue.Assignee = mapUser(resp.Fields.Assignee)
	issue.Reporter = mapUser(resp.Fields.Reporter)

	// Parse timestamps
	if resp.Fields.Created != "" {
		if t, err := parseJiraTime(resp.Fields.Created); err == nil {
			issue.Created = t
		}
	}
	if resp.Fields.Updated != "" {
		if t, err := parseJiraTime(resp.Fields.Updated); err == nil {
			issue.Updated = t
		}
	}

	issue.Links = mapIssueLinks(resp.Fields.IssueLinks)
	issue.Comments = mapComments(resp.Fields.Comment)

	return issue, nil
}

// mapUser converts a userResponse to a domain User. Returns nil if input is nil.
func mapUser(resp *userResponse) *jira4claude.User {
	if resp == nil {
		return nil
	}
	return &jira4claude.User{
		AccountID:   resp.AccountID,
		DisplayName: resp.DisplayName,
		Email:       resp.EmailAddress,
	}
}

// mapLinkedIssue converts a linkedIssueResponse to a domain LinkedIssue. Returns nil if input is nil.
func mapLinkedIssue(resp *linkedIssueResponse) *jira4claude.LinkedIssue {
	if resp == nil {
		return nil
	}
	return &jira4claude.LinkedIssue{
		Key:     resp.Key,
		Summary: resp.Fields.Summary,
		Status:  resp.Fields.Status.Name,
		Type:    resp.Fields.IssueType.Name,
	}
}

// mapIssueLinks converts a slice of issueLinkResponse to domain IssueLinks. Returns nil if input is empty.
func mapIssueLinks(links []issueLinkResponse) []*jira4claude.IssueLink {
	if len(links) == 0 {
		return nil
	}
	result := make([]*jira4claude.IssueLink, len(links))
	for i, link := range links {
		result[i] = &jira4claude.IssueLink{
			ID: link.ID,
			Type: jira4claude.IssueLinkType{
				Name:    link.Type.Name,
				Inward:  link.Type.Inward,
				Outward: link.Type.Outward,
			},
			OutwardIssue: mapLinkedIssue(link.OutwardIssue),
			InwardIssue:  mapLinkedIssue(link.InwardIssue),
		}
	}
	return result
}

// mapComments converts a commentsResponse to domain Comments. Returns nil if input is nil or empty.
func mapComments(resp *commentsResponse) []*jira4claude.Comment {
	if resp == nil || len(resp.Comments) == 0 {
		return nil
	}
	result := make([]*jira4claude.Comment, len(resp.Comments))
	for i, c := range resp.Comments {
		comment := &jira4claude.Comment{
			ID:      c.ID,
			Body:    ADFToText(c.Body),
			BodyADF: c.Body,
			Author:  mapUser(c.Author),
		}
		if c.Created != "" {
			if t, err := parseJiraTime(c.Created); err == nil {
				comment.Created = t
			}
		}
		result[i] = comment
	}
	return result
}

// mapTransitions converts a slice of transitionResponse to domain Transitions. Returns nil if input is empty.
func mapTransitions(transitions []transitionResponse) []*jira4claude.Transition {
	if len(transitions) == 0 {
		return nil
	}
	result := make([]*jira4claude.Transition, len(transitions))
	for i, t := range transitions {
		result[i] = &jira4claude.Transition{
			ID:   t.ID,
			Name: t.Name,
		}
	}
	return result
}

// parseJiraTime parses a Jira timestamp string.
func parseJiraTime(s string) (time.Time, error) {
	return time.Parse("2006-01-02T15:04:05.000-0700", s)
}

// Link creates a link between two issues.
func (s *IssueService) Link(ctx context.Context, inwardKey, linkType, outwardKey string) error {
	reqBody := map[string]any{
		"type":         map[string]any{"name": linkType},
		"inwardIssue":  map[string]any{"key": inwardKey},
		"outwardIssue": map[string]any{"key": outwardKey},
	}

	req, err := s.client.NewJSONRequest(ctx, http.MethodPost, "/rest/api/3/issueLink", reqBody)
	if err != nil {
		return err
	}

	_, err = s.client.DoRequest(req, http.StatusCreated)
	return err
}

// Unlink removes a link between two issues.
func (s *IssueService) Unlink(ctx context.Context, key1, key2 string) error {
	// Fetch the first issue to find the link
	linkID, err := s.findLinkID(ctx, key1, key2)
	if err != nil {
		return err
	}

	// Delete the link
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, "/rest/api/3/issueLink/"+url.PathEscape(linkID), nil)
	if err != nil {
		return &jira4claude.Error{
			Code:    jira4claude.EInternal,
			Message: "failed to create request",
			Inner:   err,
		}
	}

	_, err = s.client.DoRequest(req, http.StatusNoContent)
	return err
}

// findLinkID finds the link ID connecting two issues.
func (s *IssueService) findLinkID(ctx context.Context, key1, key2 string) (string, error) {
	// Fetch issue with links
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, issuePath(key1), nil)
	if err != nil {
		return "", &jira4claude.Error{
			Code:    jira4claude.EInternal,
			Message: "failed to create request",
			Inner:   err,
		}
	}

	respBody, err := s.client.DoRequest(req, http.StatusOK)
	if err != nil {
		return "", err
	}

	// Parse the issue to find the link
	var issueResp struct {
		Fields struct {
			IssueLinks []struct {
				ID           string `json:"id"`
				OutwardIssue *struct {
					Key string `json:"key"`
				} `json:"outwardIssue"`
				InwardIssue *struct {
					Key string `json:"key"`
				} `json:"inwardIssue"`
			} `json:"issuelinks"`
		} `json:"fields"`
	}
	if err := json.Unmarshal(respBody, &issueResp); err != nil {
		return "", &jira4claude.Error{
			Code:    jira4claude.EInternal,
			Message: "failed to parse response",
			Inner:   err,
		}
	}

	// Find the link to the target issue
	for _, link := range issueResp.Fields.IssueLinks {
		if link.OutwardIssue != nil && link.OutwardIssue.Key == key2 {
			return link.ID, nil
		}
		if link.InwardIssue != nil && link.InwardIssue.Key == key2 {
			return link.ID, nil
		}
	}

	return "", &jira4claude.Error{
		Code:    jira4claude.ENotFound,
		Message: fmt.Sprintf("no link found between %s and %s", key1, key2),
	}
}

// commentResponse represents the JSON structure returned by Jira API for a comment.
type commentResponse struct {
	ID      string         `json:"id"`
	Author  *userResponse  `json:"author"`
	Body    map[string]any `json:"body"`
	Created string         `json:"created"`
}

// parseCommentResponse parses the JSON response from Jira into a domain Comment.
func parseCommentResponse(body []byte) (*jira4claude.Comment, error) {
	var resp commentResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, &jira4claude.Error{
			Code:    jira4claude.EInternal,
			Message: "failed to parse response",
			Inner:   err,
		}
	}

	comment := &jira4claude.Comment{
		ID:      resp.ID,
		Body:    ADFToText(resp.Body),
		BodyADF: resp.Body,
	}

	if resp.Author != nil {
		comment.Author = &jira4claude.User{
			AccountID:   resp.Author.AccountID,
			DisplayName: resp.Author.DisplayName,
			Email:       resp.Author.EmailAddress,
		}
	}

	if resp.Created != "" {
		if t, err := parseJiraTime(resp.Created); err == nil {
			comment.Created = t
		}
	}

	return comment, nil
}
