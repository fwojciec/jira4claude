package jira4claude

import "context"

// UserService defines operations for looking up Jira users.
type UserService interface {
	// GetMyself returns the currently authenticated user.
	GetMyself(ctx context.Context) (*User, error)

	// FindUsers searches for users matching the given query string.
	FindUsers(ctx context.Context, query string) ([]*User, error)
}
