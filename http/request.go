package http

import "encoding/json"

// createRequest represents the request body for creating a Jira issue.
type createRequest struct {
	Fields createFields `json:"fields"`
}

// createFields contains the fields for creating an issue.
type createFields struct {
	Project     projectRef   `json:"project"`
	Summary     string       `json:"summary"`
	IssueType   issueTypeRef `json:"issuetype"`
	Description any          `json:"description,omitempty"`
	Priority    *priorityRef `json:"priority,omitempty"`
	Labels      []string     `json:"labels,omitempty"`
	Parent      *parentRef   `json:"parent,omitempty"`
}

// projectRef identifies a project by key.
type projectRef struct {
	Key string `json:"key"`
}

// issueTypeRef identifies an issue type by name.
type issueTypeRef struct {
	Name string `json:"name"`
}

// priorityRef identifies a priority by name.
type priorityRef struct {
	Name string `json:"name"`
}

// parentRef identifies a parent issue by key.
type parentRef struct {
	Key string `json:"key"`
}

// updateRequest represents the request body for updating a Jira issue.
type updateRequest struct {
	Fields updateFields `json:"fields"`
}

// updateFields contains the fields for updating an issue.
// All fields are optional - only set fields will be sent.
type updateFields struct {
	Summary     *string        `json:"summary,omitempty"`
	Description any            `json:"description,omitempty"`
	Priority    *priorityRef   `json:"priority,omitempty"`
	Assignee    *assigneeField `json:"assignee,omitempty"`
	Labels      *[]string      `json:"labels,omitempty"`
	Parent      *parentField   `json:"parent,omitempty"`
}

// assigneeRef identifies an assignee by account ID.
type assigneeRef struct {
	AccountID string `json:"accountId"`
}

// assigneeField wraps an optional assignee value.
// When AccountID is nil, it marshals to JSON null (for unassignment).
// When AccountID is set, it marshals to {"accountId": "..."}.
type assigneeField struct {
	AccountID *string
}

// MarshalJSON implements json.Marshaler for assigneeField.
func (a assigneeField) MarshalJSON() ([]byte, error) {
	if a.AccountID == nil {
		return []byte("null"), nil
	}
	return json.Marshal(assigneeRef{AccountID: *a.AccountID})
}

// parentField wraps an optional parent value for updates.
// When Key is nil, it marshals to JSON null (to clear parent).
// When Key is set, it marshals to {"key": "..."}.
type parentField struct {
	Key *string
}

// MarshalJSON implements json.Marshaler for parentField.
func (p parentField) MarshalJSON() ([]byte, error) {
	if p.Key == nil {
		return []byte("null"), nil
	}
	return json.Marshal(parentRef{Key: *p.Key})
}
