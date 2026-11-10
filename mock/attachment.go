package mock

import (
	"context"

	"github.com/fwojciec/jira4claude"
)

// Compile-time interface verification.
var _ jira4claude.AttachmentService = (*AttachmentService)(nil)

// AttachmentService is a mock implementation of jira4claude.AttachmentService.
// Each method delegates to its corresponding function field (e.g., List calls ListFn).
// Calling a method without setting its function field will panic.
type AttachmentService struct {
	ListFn func(ctx context.Context, key string) ([]*jira4claude.Attachment, error)
}

func (s *AttachmentService) List(ctx context.Context, key string) ([]*jira4claude.Attachment, error) {
	return s.ListFn(ctx, key)
}
