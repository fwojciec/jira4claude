package mock

import (
	"context"

	"github.com/fwojciec/jira4claude"
)

// Compile-time interface verification.
var _ jira4claude.UserService = (*UserService)(nil)

// UserService is a mock implementation of jira4claude.UserService.
// Each method delegates to its corresponding function field (e.g., GetMyself calls GetMyselfFn).
// Calling a method without setting its function field will panic.
type UserService struct {
	GetMyselfFn func(ctx context.Context) (*jira4claude.User, error)
	FindUsersFn func(ctx context.Context, query string) ([]*jira4claude.User, error)
}

func (s *UserService) GetMyself(ctx context.Context) (*jira4claude.User, error) {
	return s.GetMyselfFn(ctx)
}

func (s *UserService) FindUsers(ctx context.Context, query string) ([]*jira4claude.User, error) {
	return s.FindUsersFn(ctx, query)
}
