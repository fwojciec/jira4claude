package http

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"

	"github.com/fwojciec/jira4claude"
)

// fieldSchema models the schema object on a single field entry.
// schema.items is a primitive string (e.g. "option") in Jira's response,
// not a nested object.
type fieldSchema struct {
	Type   string `json:"type"`
	Items  string `json:"items,omitempty"`
	Custom string `json:"custom,omitempty"`
}

// fieldEntry models a single per-field entry in the createmeta/editmeta map.
// The map key (field id) is NOT part of the entry; it is held separately.
type fieldEntry struct {
	Name          string            `json:"name"`
	Required      bool              `json:"required"`
	Schema        fieldSchema       `json:"schema"`
	AllowedValues []json.RawMessage `json:"allowedValues,omitempty"`
}

// createMetaResponse models the legacy createmeta response shape.
type createMetaResponse struct {
	Projects []createMetaProject `json:"projects"`
}

type createMetaProject struct {
	IssueTypes []createMetaIssueType `json:"issuetypes"`
}

type createMetaIssueType struct {
	Fields map[string]json.RawMessage `json:"fields"`
}

// editMetaResponse models the editmeta response shape.
type editMetaResponse struct {
	Fields map[string]json.RawMessage `json:"fields"`
}

// GetCreateFields returns the fields settable on issue create for the given
// project and issue type, by calling Jira's legacy createmeta endpoint.
func (s *IssueService) GetCreateFields(ctx context.Context, projectKey, issueType string) ([]*jira4claude.IssueField, error) {
	q := url.Values{}
	q.Set("projectKeys", projectKey)
	q.Set("issuetypeNames", issueType)
	q.Set("expand", "projects.issuetypes.fields")
	reqURL := "/rest/api/3/issue/createmeta?" + q.Encode()

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

	var resp createMetaResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, &jira4claude.Error{
			Code:    jira4claude.EInternal,
			Message: "failed to parse response",
			Inner:   err,
		}
	}

	// Defensive parsing: createmeta returns 200 on bad filters, not 404.
	if len(resp.Projects) == 0 {
		return nil, &jira4claude.Error{
			Code:    jira4claude.ENotFound,
			Message: fmt.Sprintf("project %q not found", projectKey),
		}
	}
	if len(resp.Projects[0].IssueTypes) == 0 {
		return nil, &jira4claude.Error{
			Code:    jira4claude.ENotFound,
			Message: fmt.Sprintf("issue type %q not found in project %q", issueType, projectKey),
		}
	}

	return parseFieldsMap(resp.Projects[0].IssueTypes[0].Fields)
}

// GetEditFields returns the fields editable on the specified issue, by calling
// Jira's editmeta endpoint. Reflects the current workflow, screens, and the
// caller's permissions for that specific issue.
func (s *IssueService) GetEditFields(ctx context.Context, key string) ([]*jira4claude.IssueField, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, issuePath(key, "editmeta"), nil)
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

	var resp editMetaResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, &jira4claude.Error{
			Code:    jira4claude.EInternal,
			Message: "failed to parse response",
			Inner:   err,
		}
	}

	return parseFieldsMap(resp.Fields)
}

// parseFieldsMap is the shared inner parser. It takes a map keyed by field id
// (canonical id source — the entry body does not reliably carry it) and
// produces a []*IssueField in a deterministic order: required fields first,
// customfield_ prefix before builtins within each required-ness bucket, then
// by ID ascending. Map iteration order in Go is randomized, so without an
// explicit sort the same Jira response would shuffle across runs.
func parseFieldsMap(raw map[string]json.RawMessage) ([]*jira4claude.IssueField, error) {
	if len(raw) == 0 {
		return []*jira4claude.IssueField{}, nil
	}

	result := make([]*jira4claude.IssueField, 0, len(raw))
	for id, body := range raw {
		var entry fieldEntry
		if err := json.Unmarshal(body, &entry); err != nil {
			return nil, &jira4claude.Error{
				Code:    jira4claude.EInternal,
				Message: fmt.Sprintf("failed to parse field %q", id),
				Inner:   err,
			}
		}

		field := &jira4claude.IssueField{
			ID:            id,
			Name:          entry.Name,
			Required:      entry.Required,
			SchemaType:    entry.Schema.Type,
			SchemaItems:   entry.Schema.Items,
			SchemaCustom:  entry.Schema.Custom,
			AllowedValues: parseAllowedValues(entry.AllowedValues),
		}
		field.Example = exampleFor(field)

		result = append(result, field)
	}

	sort.Slice(result, func(i, j int) bool {
		a, b := result[i], result[j]
		if a.Required != b.Required {
			return a.Required
		}
		aCustom := strings.HasPrefix(a.ID, "customfield_")
		bCustom := strings.HasPrefix(b.ID, "customfield_")
		if aCustom != bCustom {
			return aCustom
		}
		return a.ID < b.ID
	})

	return result, nil
}

// parseAllowedValues normalizes Jira's multiple wire shapes into FieldAllowedValue:
//   - primitive string -> {ID:"", Value:"<string>"}
//   - object with .value -> {ID:<.id if present, else "">, Value:<.value>}
//   - object with .name (no .value) -> {ID:<.id if present, else "">, Value:<.name>}
//
// Cascading-select entries with nested children are flattened to their top-level
// value; nested children are ignored.
func parseAllowedValues(raws []json.RawMessage) []jira4claude.FieldAllowedValue {
	if len(raws) == 0 {
		return nil
	}

	out := make([]jira4claude.FieldAllowedValue, 0, len(raws))
	for _, raw := range raws {
		trimmed := bytes.TrimSpace(raw)
		// Skip empty entries and explicit JSON null elements.
		if len(trimmed) == 0 || string(trimmed) == "null" {
			continue
		}
		// Primitive string -> starts with '"'.
		if trimmed[0] == '"' {
			var s string
			if err := json.Unmarshal(trimmed, &s); err != nil {
				continue
			}
			out = append(out, jira4claude.FieldAllowedValue{Value: s})
			continue
		}
		// Object: pull id, value, name. Display value = .value if set, else .name.
		var obj struct {
			ID    string `json:"id"`
			Value string `json:"value"`
			Name  string `json:"name"`
		}
		if err := json.Unmarshal(trimmed, &obj); err != nil {
			continue
		}
		display := obj.Value
		if display == "" {
			display = obj.Name
		}
		// Skip entries whose display value would be empty: an object with
		// neither .value nor .name has no useful representation downstream,
		// and emitting {"value":""} would leak into JSON output and produce
		// an empty option example.
		if display == "" {
			continue
		}
		out = append(out, jira4claude.FieldAllowedValue{ID: obj.ID, Value: display})
	}
	return out
}

// exampleFor produces a representative JSON example for a field based on its
// schema. Returns nil for shapes the table doesn't cover; the IssueField's
// `json:"example,omitempty"` tag drops nil values from output.
func exampleFor(f *jira4claude.IssueField) json.RawMessage {
	switch f.SchemaType {
	case "string":
		if strings.HasSuffix(f.SchemaCustom, ":textarea") {
			return json.RawMessage(`{"type":"doc","version":1,"content":[{"type":"paragraph","content":[{"type":"text","text":"..."}]}]}`)
		}
		return json.RawMessage(`"..."`)
	case "number":
		return json.RawMessage(`0`)
	case "date":
		return json.RawMessage(`"2026-05-27"`)
	case "datetime":
		return json.RawMessage(`"2026-05-27T12:00:00.000+0000"`)
	case "option":
		if len(f.AllowedValues) > 0 {
			return optionExample(f.AllowedValues[0].Value)
		}
		return json.RawMessage(`{"value":"..."}`)
	case "array":
		switch f.SchemaItems {
		case "option":
			if len(f.AllowedValues) > 0 {
				return arrayOptionExample(f.AllowedValues[0].Value)
			}
			return json.RawMessage(`[{"value":"..."}]`)
		case "string":
			return json.RawMessage(`["..."]`)
		}
	case "user":
		return json.RawMessage(`{"accountId":"..."}`)
	case "priority":
		return json.RawMessage(`{"name":"High"}`)
	}
	return nil
}

// optionExample marshals {"value":<v>} using json.Marshal to handle escaping.
func optionExample(v string) json.RawMessage {
	b, err := json.Marshal(map[string]string{"value": v})
	if err != nil {
		// Should not happen for string-valued maps; fall back to the placeholder.
		return json.RawMessage(`{"value":"..."}`)
	}
	return b
}

// arrayOptionExample marshals [{"value":<v>}] using json.Marshal to handle escaping.
func arrayOptionExample(v string) json.RawMessage {
	b, err := json.Marshal([]map[string]string{{"value": v}})
	if err != nil {
		return json.RawMessage(`[{"value":"..."}]`)
	}
	return b
}
