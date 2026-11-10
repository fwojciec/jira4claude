package jira4claude

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
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

// AttachmentFile is a local file to upload as an attachment.
type AttachmentFile struct {
	// Name is the filename sent to Jira: the base name of the local path.
	Name string
	// Size is the number of bytes sent.
	Size int64
	// Open returns the content from its first byte. It is called once for
	// each attempt, so that a retried upload sends the whole file again; the
	// caller closes what it returns.
	Open func() (io.ReadCloser, error)
}

// AttachmentService defines operations on issue attachments.
type AttachmentService interface {
	// List returns the attachments of the issue with the given key, in the
	// order Jira reports them. An issue without attachments gives an empty
	// list, not an error.
	List(ctx context.Context, key string) ([]*Attachment, error)

	// Download writes the content of the attachment with the given ID to w
	// and returns the number of bytes written. On error, w may already hold
	// part of the content: the caller owns cleaning it up.
	Download(ctx context.Context, id string, w io.Writer) (int64, error)

	// Upload attaches one file to the issue with the given key and returns
	// the attachment Jira created.
	Upload(ctx context.Context, key string, file AttachmentFile) (*Attachment, error)
}

// maxFilenameBytes bounds a local file name. Most file systems allow 255
// bytes per name; the margin leaves room for the ID that LocalNames may add.
const maxFilenameBytes = 200

// maxExtensionBytes is the longest extension that truncation keeps.
const maxExtensionBytes = 20

// SafeFilename returns a local file name for an attachment, derived from the
// file name Jira reports (which is not safe to use as a path) and the
// attachment ID:
//
//   - Only the part after the last '/' or '\' is kept, so no name has a
//     directory part.
//   - Control and invisible formatting characters and invalid UTF-8 become
//     '_', so a name never holds a NUL or breaks a line of output.
//   - A name that is empty, "." or ".." becomes "attachment-ID", with every
//     character of the ID outside [A-Za-z0-9_-] replaced by '_'.
//   - A name longer than 200 bytes is cut on a character boundary, keeping a
//     short extension.
func SafeFilename(filename, id string) string {
	if i := strings.LastIndexAny(filename, `/\`); i >= 0 {
		filename = filename[i+1:]
	}
	name := replaceUnsafeRunes(filename)
	if name == "" || name == "." || name == ".." {
		if safe := safeID(id); safe != "" {
			return "attachment-" + safe
		}
		return "attachment"
	}
	stem, ext := splitExtension(name)
	return truncateUTF8(stem, maxFilenameBytes-len(ext)) + ext
}

// LocalNames returns the local file name of each attachment, in order. Names
// are SafeFilename names, made unique ignoring case (so that they cannot
// replace one another on a case-insensitive file system): the first
// attachment with a name keeps it and each later one gets its ID before the
// extension ("report.pdf", "report-10001.pdf"). If names still clash after
// that, LocalNames returns a conflict error naming the attachments.
func LocalNames(attachments []*Attachment) ([]string, error) {
	names := make([]string, len(attachments))
	taken := make(map[string]bool, len(attachments))
	for i, a := range attachments {
		name := SafeFilename(a.Filename, a.ID)
		key := strings.ToLower(name)
		if taken[key] {
			name = withID(name, a.ID)
		}
		taken[key] = true
		names[i] = name
	}

	owner := make(map[string]int, len(names))
	for i, name := range names {
		key := strings.ToLower(name)
		if j, ok := owner[key]; ok {
			return nil, &Error{
				Code: EConflict,
				Message: fmt.Sprintf("attachments %s and %s would be saved under the same name %q; download them one at a time with --id",
					attachments[j].ID, attachments[i].ID, name),
			}
		}
		owner[key] = i
	}
	return names, nil
}

// withID inserts "-ID" before the extension of a SafeFilename name, cutting
// the stem so that the result stays within maxFilenameBytes.
func withID(name, id string) string {
	safe := safeID(id)
	if safe == "" {
		safe = "_"
	}
	suffix := "-" + safe
	stem, ext := splitExtension(name)
	return truncateUTF8(stem, maxFilenameBytes-len(ext)-len(suffix)) + suffix + ext
}

// replaceUnsafeRunes replaces control characters, invisible formatting
// characters (such as a right-to-left override) and invalid UTF-8 with '_'.
func replaceUnsafeRunes(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		switch {
		case r == utf8.RuneError && size == 1, unicode.IsControl(r), unicode.Is(unicode.Cf, r):
			b.WriteByte('_')
		default:
			b.WriteString(s[i : i+size])
		}
		i += size
	}
	return b.String()
}

// maxIDBytes bounds the part of a file name taken from an attachment ID.
const maxIDBytes = 40

// safeID returns id with every character outside [A-Za-z0-9_-] replaced by
// '_', cut to maxIDBytes.
func safeID(id string) string {
	var b strings.Builder
	for _, r := range id {
		if b.Len() >= maxIDBytes {
			break
		}
		if r == '-' || r == '_' || ('0' <= r && r <= '9') || ('a' <= r && r <= 'z') || ('A' <= r && r <= 'Z') {
			b.WriteRune(r)
			continue
		}
		b.WriteByte('_')
	}
	return b.String()
}

// splitExtension splits name before its last '.', when that dot is not the
// first character and the extension is short. Otherwise ext is empty.
func splitExtension(name string) (stem, ext string) {
	i := strings.LastIndexByte(name, '.')
	if i <= 0 || len(name)-i > maxExtensionBytes {
		return name, ""
	}
	return name[:i], name[i:]
}

// truncateUTF8 cuts s to at most n bytes without splitting a character.
func truncateUTF8(s string, n int) string {
	if len(s) <= n {
		return s
	}
	for n > 0 && !utf8.RuneStart(s[n]) {
		n--
	}
	return s[:n]
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

// SavedAttachmentView is an attachment saved to a local file. Filename is the
// name Jira reports; Path is the absolute path of the file written, whose
// base name may differ from Filename (see SafeFilename and LocalNames). Size
// is the number of bytes written.
type SavedAttachmentView struct {
	ID       string `json:"id"`
	Filename string `json:"filename"`
	Path     string `json:"path"`
	Size     int64  `json:"size"`
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
