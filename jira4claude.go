// Package jira4claude provides domain types for the jira4claude CLI.
//
// This package contains pure domain entities and service interfaces
// with no external dependencies. Implementations live in subpackages
// named after their dependencies (e.g., jira/ for the API client).
package jira4claude

import "encoding/json"

// ADFNode represents a node in an Atlassian Document Format document.
// ADF is Jira's native rich text format, stored as a structured JSON tree.
// Attrs uses json.RawMessage for forward-compatibility with unknown attributes.
//
// Content is included in JSON output only when non-nil. This ensures:
//   - Doc nodes with Content: []ADFNode{} serialize with "content":[]
//   - Leaf nodes (text) with Content: nil omit the content field entirely
type ADFNode struct {
	Type    string          `json:"type"`
	Version int             `json:"version,omitempty"`
	Content []ADFNode       `json:"-"`
	Text    string          `json:"text,omitempty"`
	Marks   []ADFMark       `json:"marks,omitempty"`
	Attrs   json.RawMessage `json:"attrs,omitempty"`
}

// MarshalJSON implements json.Marshaler for ADFNode.
// Content is only included in the output when non-nil, matching the ADF spec
// where leaf nodes (text, hardBreak) have no content field.
func (n ADFNode) MarshalJSON() ([]byte, error) {
	type alias ADFNode
	if n.Content != nil {
		return json.Marshal(struct {
			alias
			Content []ADFNode `json:"content"`
		}{alias: alias(n), Content: n.Content})
	}
	return json.Marshal(alias(n))
}

// UnmarshalJSON implements json.Unmarshaler for ADFNode.
// This is needed because Content uses json:"-" for custom marshal behavior.
func (n *ADFNode) UnmarshalJSON(data []byte) error {
	type alias ADFNode
	aux := &struct {
		*alias
		Content []ADFNode `json:"content"`
	}{alias: (*alias)(n)}
	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}
	n.Content = aux.Content
	return nil
}

// ADFMark represents a mark (inline formatting) applied to an ADF text node.
type ADFMark struct {
	Type  string          `json:"type"`
	Attrs json.RawMessage `json:"attrs,omitempty"`
}
