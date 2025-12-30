package http

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
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
		fields["description"] = TextToADF(issue.Description)
	}
	if issue.Priority != "" {
		fields["priority"] = map[string]any{"name": issue.Priority}
	}
	if len(issue.Labels) > 0 {
		fields["labels"] = issue.Labels
	}

	body := map[string]any{"fields": fields}
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, &jira4claude.Error{
			Code:    jira4claude.EInternal,
			Message: "failed to marshal request",
			Inner:   err,
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "/rest/api/3/issue", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, &jira4claude.Error{
			Code:    jira4claude.EInternal,
			Message: "failed to create request",
			Inner:   err,
		}
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, &jira4claude.Error{
			Code:    jira4claude.EInternal,
			Message: "request failed",
			Inner:   err,
		}
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, &jira4claude.Error{
			Code:    jira4claude.EInternal,
			Message: "failed to read response",
			Inner:   err,
		}
	}

	if resp.StatusCode != http.StatusCreated {
		if apiErr := ParseErrorResponse(resp.StatusCode, respBody); apiErr != nil {
			return nil, apiErr
		}
		return nil, &jira4claude.Error{
			Code:    statusCodeToErrorCode(resp.StatusCode),
			Message: fmt.Sprintf("unexpected status: %d", resp.StatusCode),
		}
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
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "/rest/api/3/issue/"+key, nil)
	if err != nil {
		return nil, &jira4claude.Error{
			Code:    jira4claude.EInternal,
			Message: "failed to create request",
			Inner:   err,
		}
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, &jira4claude.Error{
			Code:    jira4claude.EInternal,
			Message: "request failed",
			Inner:   err,
		}
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, &jira4claude.Error{
			Code:    jira4claude.EInternal,
			Message: "failed to read response",
			Inner:   err,
		}
	}

	if resp.StatusCode != http.StatusOK {
		if apiErr := ParseErrorResponse(resp.StatusCode, respBody); apiErr != nil {
			return nil, apiErr
		}
		return nil, &jira4claude.Error{
			Code:    statusCodeToErrorCode(resp.StatusCode),
			Message: fmt.Sprintf("unexpected status: %d", resp.StatusCode),
		}
	}

	return parseIssueResponse(respBody)
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
	fields := "key,summary,status,issuetype,project,priority,assignee,reporter,labels,created,updated,description"
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

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, &jira4claude.Error{
			Code:    jira4claude.EInternal,
			Message: "request failed",
			Inner:   err,
		}
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, &jira4claude.Error{
			Code:    jira4claude.EInternal,
			Message: "failed to read response",
			Inner:   err,
		}
	}

	if resp.StatusCode != http.StatusOK {
		if apiErr := ParseErrorResponse(resp.StatusCode, respBody); apiErr != nil {
			return nil, apiErr
		}
		return nil, &jira4claude.Error{
			Code:    statusCodeToErrorCode(resp.StatusCode),
			Message: fmt.Sprintf("unexpected status: %d", resp.StatusCode),
		}
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
	// Pre-allocate for max possible clauses: project, status, assignee, + labels
	clauses := make([]string, 0, 3+len(filter.Labels))

	if filter.Project != "" {
		clauses = append(clauses, fmt.Sprintf("project = %q", filter.Project))
	}
	if filter.Status != "" {
		clauses = append(clauses, fmt.Sprintf("status = %q", filter.Status))
	}
	if filter.Assignee != "" {
		clauses = append(clauses, fmt.Sprintf("assignee = %q", filter.Assignee))
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
		fields["description"] = TextToADF(*update.Description)
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

	body := map[string]any{"fields": fields}
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, &jira4claude.Error{
			Code:    jira4claude.EInternal,
			Message: "failed to marshal request",
			Inner:   err,
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, "/rest/api/3/issue/"+key, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, &jira4claude.Error{
			Code:    jira4claude.EInternal,
			Message: "failed to create request",
			Inner:   err,
		}
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, &jira4claude.Error{
			Code:    jira4claude.EInternal,
			Message: "request failed",
			Inner:   err,
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		respBody, _ := io.ReadAll(resp.Body)
		if apiErr := ParseErrorResponse(resp.StatusCode, respBody); apiErr != nil {
			return nil, apiErr
		}
		return nil, &jira4claude.Error{
			Code:    statusCodeToErrorCode(resp.StatusCode),
			Message: fmt.Sprintf("unexpected status: %d", resp.StatusCode),
		}
	}

	// Fetch and return the updated issue
	return s.Get(ctx, key)
}

// Delete deletes an issue by its key.
func (s *IssueService) Delete(ctx context.Context, key string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, "/rest/api/3/issue/"+key, nil)
	if err != nil {
		return &jira4claude.Error{
			Code:    jira4claude.EInternal,
			Message: "failed to create request",
			Inner:   err,
		}
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return &jira4claude.Error{
			Code:    jira4claude.EInternal,
			Message: "request failed",
			Inner:   err,
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		respBody, _ := io.ReadAll(resp.Body)
		if apiErr := ParseErrorResponse(resp.StatusCode, respBody); apiErr != nil {
			return apiErr
		}
		return &jira4claude.Error{
			Code:    statusCodeToErrorCode(resp.StatusCode),
			Message: fmt.Sprintf("unexpected status: %d", resp.StatusCode),
		}
	}

	return nil
}

// AddComment adds a comment to an issue.
func (s *IssueService) AddComment(ctx context.Context, key, body string) (*jira4claude.Comment, error) {
	reqBody := map[string]any{
		"body": TextToADF(body),
	}
	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, &jira4claude.Error{
			Code:    jira4claude.EInternal,
			Message: "failed to marshal request",
			Inner:   err,
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "/rest/api/3/issue/"+key+"/comment", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, &jira4claude.Error{
			Code:    jira4claude.EInternal,
			Message: "failed to create request",
			Inner:   err,
		}
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, &jira4claude.Error{
			Code:    jira4claude.EInternal,
			Message: "request failed",
			Inner:   err,
		}
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, &jira4claude.Error{
			Code:    jira4claude.EInternal,
			Message: "failed to read response",
			Inner:   err,
		}
	}

	if resp.StatusCode != http.StatusCreated {
		if apiErr := ParseErrorResponse(resp.StatusCode, respBody); apiErr != nil {
			return nil, apiErr
		}
		return nil, &jira4claude.Error{
			Code:    statusCodeToErrorCode(resp.StatusCode),
			Message: fmt.Sprintf("unexpected status: %d", resp.StatusCode),
		}
	}

	return parseCommentResponse(respBody)
}

// Transitions returns available workflow transitions for an issue.
func (s *IssueService) Transitions(ctx context.Context, key string) ([]*jira4claude.Transition, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "/rest/api/3/issue/"+key+"/transitions", nil)
	if err != nil {
		return nil, &jira4claude.Error{
			Code:    jira4claude.EInternal,
			Message: "failed to create request",
			Inner:   err,
		}
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, &jira4claude.Error{
			Code:    jira4claude.EInternal,
			Message: "request failed",
			Inner:   err,
		}
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, &jira4claude.Error{
			Code:    jira4claude.EInternal,
			Message: "failed to read response",
			Inner:   err,
		}
	}

	if resp.StatusCode != http.StatusOK {
		if apiErr := ParseErrorResponse(resp.StatusCode, respBody); apiErr != nil {
			return nil, apiErr
		}
		return nil, &jira4claude.Error{
			Code:    statusCodeToErrorCode(resp.StatusCode),
			Message: fmt.Sprintf("unexpected status: %d", resp.StatusCode),
		}
	}

	var transitionsResp struct {
		Transitions []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"transitions"`
	}
	if err := json.Unmarshal(respBody, &transitionsResp); err != nil {
		return nil, &jira4claude.Error{
			Code:    jira4claude.EInternal,
			Message: "failed to parse response",
			Inner:   err,
		}
	}

	transitions := make([]*jira4claude.Transition, len(transitionsResp.Transitions))
	for i, t := range transitionsResp.Transitions {
		transitions[i] = &jira4claude.Transition{
			ID:   t.ID,
			Name: t.Name,
		}
	}

	return transitions, nil
}

// Transition moves an issue to a new status.
func (s *IssueService) Transition(ctx context.Context, key, transitionID string) error {
	reqBody := map[string]any{
		"transition": map[string]any{
			"id": transitionID,
		},
	}
	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return &jira4claude.Error{
			Code:    jira4claude.EInternal,
			Message: "failed to marshal request",
			Inner:   err,
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "/rest/api/3/issue/"+key+"/transitions", bytes.NewReader(jsonBody))
	if err != nil {
		return &jira4claude.Error{
			Code:    jira4claude.EInternal,
			Message: "failed to create request",
			Inner:   err,
		}
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return &jira4claude.Error{
			Code:    jira4claude.EInternal,
			Message: "request failed",
			Inner:   err,
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		respBody, _ := io.ReadAll(resp.Body)
		if apiErr := ParseErrorResponse(resp.StatusCode, respBody); apiErr != nil {
			return apiErr
		}
		return &jira4claude.Error{
			Code:    statusCodeToErrorCode(resp.StatusCode),
			Message: fmt.Sprintf("unexpected status: %d", resp.StatusCode),
		}
	}

	return nil
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

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return &jira4claude.Error{
			Code:    jira4claude.EInternal,
			Message: "failed to marshal request",
			Inner:   err,
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, "/rest/api/3/issue/"+key+"/assignee", bytes.NewReader(jsonBody))
	if err != nil {
		return &jira4claude.Error{
			Code:    jira4claude.EInternal,
			Message: "failed to create request",
			Inner:   err,
		}
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return &jira4claude.Error{
			Code:    jira4claude.EInternal,
			Message: "request failed",
			Inner:   err,
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		respBody, _ := io.ReadAll(resp.Body)
		if apiErr := ParseErrorResponse(resp.StatusCode, respBody); apiErr != nil {
			return apiErr
		}
		return &jira4claude.Error{
			Code:    statusCodeToErrorCode(resp.StatusCode),
			Message: fmt.Sprintf("unexpected status: %d", resp.StatusCode),
		}
	}

	return nil
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
		Created     string                `json:"created"`
		Updated     string                `json:"updated"`
	} `json:"fields"`
}

type userResponse struct {
	AccountID    string `json:"accountId"`
	DisplayName  string `json:"displayName"`
	EmailAddress string `json:"emailAddress"`
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
		Key:         resp.Key,
		Project:     resp.Fields.Project.Key,
		Summary:     resp.Fields.Summary,
		Description: ADFToText(resp.Fields.Description),
		Status:      resp.Fields.Status.Name,
		Type:        resp.Fields.IssueType.Name,
		Priority:    resp.Fields.Priority.Name,
		Labels:      resp.Fields.Labels,
	}

	if resp.Fields.Assignee != nil {
		issue.Assignee = &jira4claude.User{
			AccountID:   resp.Fields.Assignee.AccountID,
			DisplayName: resp.Fields.Assignee.DisplayName,
			Email:       resp.Fields.Assignee.EmailAddress,
		}
	}

	if resp.Fields.Reporter != nil {
		issue.Reporter = &jira4claude.User{
			AccountID:   resp.Fields.Reporter.AccountID,
			DisplayName: resp.Fields.Reporter.DisplayName,
			Email:       resp.Fields.Reporter.EmailAddress,
		}
	}

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

	return issue, nil
}

// parseJiraTime parses a Jira timestamp string.
func parseJiraTime(s string) (time.Time, error) {
	return time.Parse("2006-01-02T15:04:05.000-0700", s)
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
		ID:   resp.ID,
		Body: ADFToText(resp.Body),
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
