package jira4claude

import (
	"context"
	"time"
)

// Attachment is a file attached to an issue.
//
// The download URL Jira returns with each attachment is deliberately not kept:
// content is fetched by ID from the configured host
// (/rest/api/3/attachment/content/{id}), so credentials never follow a URL the
// server supplied.
type Attachment struct {
	ID       string
	Filename string // as Jira reports it; not safe to use as a path
	MimeType string
	Size     int64 // in bytes
	Author   *User // nil when Jira does not report one
	Created  time.Time
}

// AttachmentService defines operations on issue attachments.
//
// Download, upload and delete are expected to join it, by attachment ID where
// the API addresses attachments by ID:
//
//	Download(ctx context.Context, id string, w io.Writer) (int64, error)
//	Upload(ctx context.Context, key string, files []AttachmentFile) ([]*Attachment, error)
//	Delete(ctx context.Context, id string) error
type AttachmentService interface {
	// List returns the attachments of the issue with the given key, in the
	// order Jira reports them. An issue without attachments gives an empty
	// list, not an error.
	List(ctx context.Context, key string) ([]*Attachment, error)
}

// AttachmentView is a display-ready attachment. Size is the exact byte count;
// Created is RFC 3339, empty when unknown.
type AttachmentView struct {
	ID       string `json:"id"`
	Filename string `json:"filename"`
	MimeType string `json:"mimeType"`
	Size     int64  `json:"size"`
	Author   string `json:"author,omitempty"`
	Created  string `json:"created,omitempty"`
}

// ToAttachmentViews converts attachments to views, keeping their order. The
// result is never nil, so it encodes as [] when empty.
func ToAttachmentViews(attachments []*Attachment) []AttachmentView {
	views := make([]AttachmentView, 0, len(attachments))
	for _, a := range attachments {
		var created string
		if !a.Created.IsZero() {
			created = a.Created.Format(time.RFC3339)
		}
		views = append(views, AttachmentView{
			ID:       a.ID,
			Filename: a.Filename,
			MimeType: a.MimeType,
			Size:     a.Size,
			Author:   displayName(a.Author),
			Created:  created,
		})
	}
	return views
}
