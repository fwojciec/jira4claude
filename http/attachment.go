package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"
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

// Download streams the content of an attachment to w from
// GET /rest/api/3/attachment/content/{id}?redirect=false on the configured
// host. The content URL Jira reports with each attachment is never used.
//
// Atlassian's REST v3 OpenAPI document is reported (in the ticket brief; the
// document was not fetched for this change) to list 200, 206 and 303
// responses for this endpoint and a redirect query parameter. redirect=false
// is assumed to ask for the bytes directly. Neither is verified against a
// live site, so a redirect is still handled, under Client.Stream's
// credential rules, and the code is safe whichever way the server answers.
//
// The ID must be a plain token of letters, digits, '-' and '_' (Jira's IDs
// are numeric), so it cannot change the request path.
func (s *AttachmentService) Download(ctx context.Context, id string, w io.Writer) (int64, error) {
	if err := checkID(id); err != nil {
		return 0, err
	}
	return s.client.Stream(ctx, "/rest/api/3/attachment/content/"+id+"?redirect=false", w)
}

// Get reads an attachment's metadata from GET /rest/api/3/attachment/{id},
// operation getAttachment of Atlassian's REST v3 OpenAPI document (the
// version Upload names): schema AttachmentMetadata has id (int64), filename,
// mimeType, size, author and created, and no field naming the issue. Its
// example gives id as a number and created in Jira's timestamp form; both
// forms of each are accepted, as for List. Not verified against a live site.
//
// The ID must be a plain token, as for Download.
func (s *AttachmentService) Get(ctx context.Context, id string) (*jira4claude.Attachment, error) {
	if err := checkID(id); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, attachmentPath(id), nil)
	if err != nil {
		return nil, &jira4claude.Error{Code: jira4claude.EInternal, Message: "failed to create request", Inner: err}
	}
	body, err := s.client.DoRequest(req, http.StatusOK)
	if err != nil {
		return nil, err
	}
	var entry attachmentResponse
	if err := json.Unmarshal(body, &entry); err != nil {
		return nil, &jira4claude.Error{Code: jira4claude.EInternal, Message: "failed to parse response", Inner: err}
	}
	return mapAttachments([]attachmentResponse{entry})[0], nil
}

// Delete deletes an attachment with DELETE /rest/api/3/attachment/{id},
// operation removeAttachment of Atlassian's REST v3 OpenAPI document (the
// version Upload names), answered 204 on success, 403 without permission and
// 404 when the attachment is not found or attachments are disabled. Not
// verified against a live site.
//
// DELETE is idempotent, so DoRequest retries it on 429, 5xx and connection
// errors (design 1.5): a delete that the server applied before failing is
// answered 404 on the retry. The ID must be a plain token, as for Download.
func (s *AttachmentService) Delete(ctx context.Context, id string) error {
	if err := checkID(id); err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, attachmentPath(id), nil)
	if err != nil {
		return &jira4claude.Error{Code: jira4claude.EInternal, Message: "failed to create request", Inner: err}
	}
	_, err = s.client.DoRequest(req, http.StatusNoContent)
	return err
}

// attachmentPath is the path of an attachment's metadata; id is a plain token
// (checkID), so it needs no escaping.
func attachmentPath(id string) string {
	return "/rest/api/3/attachment/" + id
}

// checkID refuses an attachment ID that is not a plain token of letters,
// digits, '-' and '_' (Jira's IDs are numeric), so that it cannot change the
// request path.
func checkID(id string) error {
	if !isPlainID(id) {
		return &jira4claude.Error{
			Code:    jira4claude.EValidation,
			Message: "invalid attachment id " + strconv.Quote(id),
		}
	}
	return nil
}

// Upload attaches one file to an issue with a multipart/form-data POST to
// /rest/api/3/issue/{key}/attachments, as Atlassian's REST v3 OpenAPI
// document describes operation addAttachment (version
// 1001.0.0-SNAPSHOT-37bdb440188c5b94db5486d134b4d3bbf7517e25, fetched
// 2026-11-10): the X-Atlassian-Token: no-check header, one part named "file"
// per file, a 200 answered with an array of Attachment, and a 413 when the
// attachments exceed the site's size limit, more than 60 files are sent, or
// the issue's attachment limit is reached. Not verified against a live site.
//
// One file per request: the body is the part header, the file's first
// file.Size bytes and the closing delimiter, streamed, never held in memory,
// with an exact Content-Length (no chunked encoding). The boundary is random
// per call and the same for every attempt. GetBody calls file.Open again, so
// a POST retried after a 429 (the only status DoRequest retries a POST on)
// sends the whole file again. A 5xx or a connection error is not retried and
// says the upload may have been applied.
//
// Redirects are not followed: a 307 or 308 would send the file again to
// whatever host the Location header names.
//
// The part's Content-Type is application/octet-stream, as curl -F and
// Go's multipart.Writer send for a file of unknown type; Jira is assumed to
// set the attachment's MIME type itself (the document's Python example sends
// "application-type").
func (s *AttachmentService) Upload(ctx context.Context, key string, file jira4claude.AttachmentFile) (*jira4claude.Attachment, error) {
	head, tail, contentType := multipartFrame(file.Name)
	open := func() (io.ReadCloser, error) {
		content, err := file.Open()
		if err != nil {
			return nil, err
		}
		return &uploadBody{
			Reader: io.MultiReader(bytes.NewReader(head), io.LimitReader(content, file.Size), bytes.NewReader(tail)),
			closer: content,
		}, nil
	}
	body, err := open()
	if err != nil {
		return nil, &jira4claude.Error{
			Code:    jira4claude.EInternal,
			Message: fmt.Sprintf("cannot read %q", file.Name),
			Inner:   err,
		}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, issuePath(key, "attachments"), body)
	if err != nil {
		// Error ignored: the request failed already.
		_ = body.Close()
		return nil, &jira4claude.Error{Code: jira4claude.EInternal, Message: "failed to create request", Inner: err}
	}
	req.ContentLength = int64(len(head)) + file.Size + int64(len(tail))
	req.GetBody = open
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("X-Atlassian-Token", "no-check")

	respBody, status, err := s.client.withoutRedirects().doRequest(req, http.StatusOK)
	switch {
	case status == http.StatusRequestEntityTooLarge:
		return nil, tooLargeError(file, respBody)
	case isRedirectStatus(status):
		return nil, &jira4claude.Error{
			Code: jira4claude.EInternal,
			Message: fmt.Sprintf("Jira answered the upload of %q with a redirect (HTTP %d); it was not followed and the file was sent nowhere else",
				file.Name, status),
		}
	case err != nil:
		return nil, err
	}

	var entries []attachmentResponse
	err = json.Unmarshal(respBody, &entries)
	if err != nil || len(entries) != 1 || entries[0].ID == "" {
		return nil, &jira4claude.Error{
			Code: jira4claude.EInternal,
			Message: fmt.Sprintf("Jira accepted %q (HTTP 200), but its response does not name the one attachment created; check with attachment list %s before uploading again",
				file.Name, key),
			Inner: err,
		}
	}
	return mapAttachments(entries)[0], nil
}

// multipartFrame returns what a multipart/form-data body of one file part
// named "file" holds before the file's content (the boundary and part header)
// and after it (the closing delimiter), with the request's Content-Type. The
// boundary is random, so that it cannot be predicted from earlier requests.
// multipart.Writer quotes the filename, escaping '"' and '\' and writing CR
// and LF as %0D and %0A.
func multipartFrame(filename string) (head, tail []byte, contentType string) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	// Errors ignored: a bytes.Buffer does not fail to write.
	_, _ = w.CreateFormFile("file", filename)
	head = bytes.Clone(buf.Bytes())
	buf.Reset()
	_ = w.Close()
	tail = bytes.Clone(buf.Bytes())
	return head, tail, w.FormDataContentType()
}

// uploadBody is one attempt's request body; closing it closes the file.
type uploadBody struct {
	io.Reader
	closer io.Closer
}

func (b *uploadBody) Close() error {
	return b.closer.Close()
}

// tooLargeError reports a 413, with Jira's own message when the body holds
// one. The causes are those the OpenAPI document lists for addAttachment,
// except more than 60 files, which one file per request cannot cause.
func tooLargeError(file jira4claude.AttachmentFile, body []byte) error {
	msg := fmt.Sprintf("Jira refused %q (%d bytes) as too large (HTTP 413): it exceeds the site's attachment size limit, or the issue has reached its attachment limit",
		file.Name, file.Size)
	if messages, err := errorMessages(body); err == nil && len(messages) > 0 {
		msg += "; Jira says: " + strings.Join(messages, "; ")
	}
	return &jira4claude.Error{Code: jira4claude.EValidation, Message: msg}
}

// isPlainID reports whether id is non-empty and holds only ASCII letters,
// digits, '-' and '_'.
func isPlainID(id string) bool {
	if id == "" {
		return false
	}
	for _, r := range id {
		if r != '-' && r != '_' && (r < '0' || r > '9') && (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') {
			return false
		}
	}
	return true
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
