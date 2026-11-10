package mock

import (
	"context"
	"io"

	"github.com/fwojciec/jira4claude"
)

// Compile-time interface verification.
var _ jira4claude.AttachmentService = (*AttachmentService)(nil)

// AttachmentService is a mock implementation of jira4claude.AttachmentService.
// Each method delegates to its corresponding function field (e.g., List calls ListFn).
// Calling a method without setting its function field will panic.
type AttachmentService struct {
	ListFn     func(ctx context.Context, key string) ([]*jira4claude.Attachment, error)
	DownloadFn func(ctx context.Context, id string, w io.Writer) (int64, error)
	UploadFn   func(ctx context.Context, key string, file jira4claude.AttachmentFile) (*jira4claude.Attachment, error)
	GetFn      func(ctx context.Context, id string) (*jira4claude.Attachment, error)
	DeleteFn   func(ctx context.Context, id string) error
}

func (s *AttachmentService) List(ctx context.Context, key string) ([]*jira4claude.Attachment, error) {
	return s.ListFn(ctx, key)
}

func (s *AttachmentService) Download(ctx context.Context, id string, w io.Writer) (int64, error) {
	return s.DownloadFn(ctx, id, w)
}

func (s *AttachmentService) Upload(ctx context.Context, key string, file jira4claude.AttachmentFile) (*jira4claude.Attachment, error) {
	return s.UploadFn(ctx, key, file)
}

func (s *AttachmentService) Get(ctx context.Context, id string) (*jira4claude.Attachment, error) {
	return s.GetFn(ctx, id)
}

func (s *AttachmentService) Delete(ctx context.Context, id string) error {
	return s.DeleteFn(ctx, id)
}
