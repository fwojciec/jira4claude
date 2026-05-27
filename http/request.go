package http

import "encoding/json"

// createRequest represents the request body for creating a Jira issue.
type createRequest struct {
	Fields createFields `json:"fields"`
}

// createFields contains the fields for creating an issue.
type createFields struct {
	Project      projectRef                 `json:"project"`
	Summary      string                     `json:"summary"`
	IssueType    issueTypeRef               `json:"issuetype"`
	Description  any                        `json:"description,omitempty"`
	Priority     *priorityRef               `json:"priority,omitempty"`
	Labels       []string                   `json:"labels,omitempty"`
	Parent       *parentRef                 `json:"parent,omitempty"`
	CustomFields map[string]json.RawMessage `json:"-"`
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
	Summary      *string                    `json:"summary,omitempty"`
	Description  any                        `json:"description,omitempty"`
	Priority     *priorityRef               `json:"priority,omitempty"`
	Assignee     *assigneeField             `json:"assignee,omitempty"`
	Labels       *[]string                  `json:"labels,omitempty"`
	Parent       *parentField               `json:"parent,omitempty"`
	CustomFields map[string]json.RawMessage `json:"-"`
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

// MarshalJSON implements json.Marshaler for createRequest. See mergeCustomFields.
func (r createRequest) MarshalJSON() ([]byte, error) {
	type alias createRequest
	base, err := json.Marshal(alias(r))
	if err != nil {
		return nil, err
	}
	return mergeCustomFields(base, r.Fields.CustomFields)
}

// MarshalJSON implements json.Marshaler for updateRequest. See mergeCustomFields.
func (r updateRequest) MarshalJSON() ([]byte, error) {
	type alias updateRequest
	base, err := json.Marshal(alias(r))
	if err != nil {
		return nil, err
	}
	return mergeCustomFields(base, r.Fields.CustomFields)
}

// mergeCustomFields overlays custom onto the typed "fields" object in base.
// Custom values win on key collision. When custom is empty, base is returned
// unchanged — preserving byte-identity (and thus struct-declaration key order)
// for the non-regression invariant.
func mergeCustomFields(base []byte, custom map[string]json.RawMessage) ([]byte, error) {
	if len(custom) == 0 {
		return base, nil
	}
	var wrapper struct {
		Fields map[string]json.RawMessage `json:"fields"`
	}
	if err := json.Unmarshal(base, &wrapper); err != nil {
		return nil, err
	}
	for k, v := range custom {
		wrapper.Fields[k] = v
	}
	return json.Marshal(wrapper)
}
