package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/fwojciec/jira4claude"
)

// AttachmentService implements jira4claude.AttachmentService using the Jira
// REST API.
type AttachmentService struct {
	client *Client
}

// Compile-time interface verification.
var _ jira4claude.AttachmentService = (*AttachmentService)(nil)

// NewAttachmentService creates a new AttachmentService using the provided
// HTTP client.
func NewAttachmentService(client *Client) *AttachmentService {
	return &AttachmentService{client: client}
}

// List returns the attachments of an issue, read from the issue's attachment
// field (GET /rest/api/3/issue/{key}?fields=attachment). Jira has no separate
// endpoint listing an issue's attachments.
func (s *AttachmentService) List(ctx context.Context, key string) ([]*jira4claude.Attachment, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, issuePath(key)+"?fields=attachment", nil)
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

	var resp struct {
		Fields struct {
			Attachment []attachmentResponse `json:"attachment"`
		} `json:"fields"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, &jira4claude.Error{
			Code:    jira4claude.EInternal,
			Message: "failed to parse response",
			Inner:   err,
		}
	}

	return mapAttachments(resp.Fields.Attachment), nil
}

// attachmentResponse is one entry of an issue's attachment field.
//
// Field names follow Atlassian's Jira Cloud platform REST API v3 OpenAPI
// document (https://developer.atlassian.com/cloud/jira/platform/swagger-v3.v3.json,
// version 1001.0.0-SNAPSHOT-37bdb440188c5b94db5486d134b4d3bbf7517e25, fetched
// 2026-11-10): schema Attachment has id, filename, mimeType, size (int64),
// author, created, content and thumbnail. IssueBean.fields is an untyped map,
// so the issue's attachment field is documented only by the getIssue response
// example, which disagrees with the schema on two types: there id is a number
// (10000), not a string, and created is Jira's "2022-10-06T07:32:47.000+0000"
// form, while the addAttachment example gives created in epoch milliseconds.
// Both forms of each are accepted. Not verified against a live site.
//
// content (the download URL) and thumbnail are not read: downloads go to
// /rest/api/3/attachment/content/{id} on the configured host.
type attachmentResponse struct {
	ID       flexibleID      `json:"id"`
	Filename string          `json:"filename"`
	MimeType string          `json:"mimeType"`
	Size     int64           `json:"size"`
	Author   *userResponse   `json:"author"`
	Created  json.RawMessage `json:"created"`
}

// flexibleID is an ID that Jira may send as a JSON string or a JSON number.
type flexibleID string

// UnmarshalJSON accepts a string or an integer.
func (id *flexibleID) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if bytes.Equal(data, []byte("null")) {
		return nil
	}
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		*id = flexibleID(s)
		return nil
	}
	var n json.Number
	if err := json.Unmarshal(data, &n); err == nil {
		if _, err := strconv.ParseInt(n.String(), 10, 64); err == nil {
			*id = flexibleID(n.String())
			return nil
		}
	}
	return errors.New("attachment id is neither a string nor an integer: " + string(data))
}

// mapAttachments converts attachment entries to domain attachments. It
// returns nil for no entries.
func mapAttachments(entries []attachmentResponse) []*jira4claude.Attachment {
	if len(entries) == 0 {
		return nil
	}
	attachments := make([]*jira4claude.Attachment, 0, len(entries))
	for i := range entries {
		e := &entries[i]
		attachments = append(attachments, &jira4claude.Attachment{
			ID:       string(e.ID),
			Filename: e.Filename,
			MimeType: e.MimeType,
			Size:     e.Size,
			Author:   mapUser(e.Author),
			Created:  parseAttachmentTime(e.Created),
		})
	}
	return attachments
}

// parseAttachmentTime reads a created value given as Jira's timestamp string,
// an RFC 3339 string or a number of milliseconds since the epoch. Anything
// else gives the zero time, as an unparseable issue timestamp does.
func parseAttachmentTime(raw json.RawMessage) time.Time {
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		if t, err := parseJiraTime(s); err == nil {
			return t
		}
		if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
			return t
		}
		return time.Time{}
	}
	var ms int64
	if err := json.Unmarshal(raw, &ms); err == nil {
		return time.UnixMilli(ms)
	}
	return time.Time{}
}
