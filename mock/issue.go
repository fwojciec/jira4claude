// Package mock provides test doubles for jira4claude interfaces.
//
// Service mocks (e.g., IssueService) use function fields that panic if nil,
// requiring tests to set fields before calling methods.
//
// Output mocks (e.g., Printer) record calls for assertions and do not panic
// if function fields are nil.
//
// Example usage:
//
//	svc := &mock.IssueService{
//	    GetFn: func(ctx context.Context, key string) (*jira4claude.Issue, error) {
//	        return &jira4claude.Issue{Key: key, Summary: "Test"}, nil
//	    },
//	}
//	issue, err := svc.Get(ctx, "TEST-1")
package mock

import (
	"context"

	"github.com/fwojciec/jira4claude"
)

// Compile-time interface verification.
var _ jira4claude.IssueService = (*IssueService)(nil)

// IssueService is a mock implementation of jira4claude.IssueService.
// Each method delegates to its corresponding function field (e.g., Get calls GetFn).
// Calling a method without setting its function field will panic.
type IssueService struct {
	CreateFn      func(ctx context.Context, issue *jira4claude.Issue) (*jira4claude.Issue, error)
	GetFn         func(ctx context.Context, key string) (*jira4claude.Issue, error)
	ListFn        func(ctx context.Context, filter jira4claude.IssueFilter) ([]*jira4claude.Issue, error)
	UpdateFn      func(ctx context.Context, key string, update jira4claude.IssueUpdate) (*jira4claude.Issue, error)
	DeleteFn      func(ctx context.Context, key string) error
	AddCommentFn  func(ctx context.Context, key, body string) (*jira4claude.Comment, error)
	TransitionsFn func(ctx context.Context, key string) ([]*jira4claude.Transition, error)
	TransitionFn  func(ctx context.Context, key, transitionID string) error
	AssignFn      func(ctx context.Context, key, accountID string) error
	LinkFn        func(ctx context.Context, inwardKey, linkType, outwardKey string) error
	UnlinkFn      func(ctx context.Context, key1, key2 string) error
}

func (s *IssueService) Create(ctx context.Context, issue *jira4claude.Issue) (*jira4claude.Issue, error) {
	return s.CreateFn(ctx, issue)
}

func (s *IssueService) Get(ctx context.Context, key string) (*jira4claude.Issue, error) {
	return s.GetFn(ctx, key)
}

func (s *IssueService) List(ctx context.Context, filter jira4claude.IssueFilter) ([]*jira4claude.Issue, error) {
	return s.ListFn(ctx, filter)
}

func (s *IssueService) Update(ctx context.Context, key string, update jira4claude.IssueUpdate) (*jira4claude.Issue, error) {
	return s.UpdateFn(ctx, key, update)
}

func (s *IssueService) Delete(ctx context.Context, key string) error {
	return s.DeleteFn(ctx, key)
}

func (s *IssueService) AddComment(ctx context.Context, key, body string) (*jira4claude.Comment, error) {
	return s.AddCommentFn(ctx, key, body)
}

func (s *IssueService) Transitions(ctx context.Context, key string) ([]*jira4claude.Transition, error) {
	return s.TransitionsFn(ctx, key)
}

func (s *IssueService) Transition(ctx context.Context, key, transitionID string) error {
	return s.TransitionFn(ctx, key, transitionID)
}

func (s *IssueService) Assign(ctx context.Context, key, accountID string) error {
	return s.AssignFn(ctx, key, accountID)
}

func (s *IssueService) Link(ctx context.Context, inwardKey, linkType, outwardKey string) error {
	return s.LinkFn(ctx, inwardKey, linkType, outwardKey)
}

func (s *IssueService) Unlink(ctx context.Context, key1, key2 string) error {
	return s.UnlinkFn(ctx, key1, key2)
}
