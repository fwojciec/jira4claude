package jira4claude

import (
	"context"
	"time"
)

// User represents a Jira user.
type User struct {
	AccountID   string
	DisplayName string
	Email       string
}

// Transition represents a workflow transition available for an issue.
type Transition struct {
	ID   string
	Name string
}

// Comment represents a comment on an issue.
type Comment struct {
	ID      string
	Author  User
	Body    string
	Created time.Time
}

// Issue represents a Jira issue with its core fields.
type Issue struct {
	Key         string
	Project     string
	Summary     string
	Description string
	Status      string
	Type        string
	Priority    string
	Assignee    *User
	Reporter    *User
	Labels      []string
	Created     time.Time
	Updated     time.Time
}

// IssueFilter specifies criteria for listing issues.
// If JQL is set, it is used directly and other fields are ignored.
// Otherwise, non-empty fields are combined with AND logic.
type IssueFilter struct {
	Project  string
	Status   string
	Assignee string
	Labels   []string // Issues must have ALL specified labels
	JQL      string   // Raw JQL query; overrides other fields if set
}

// IssueUpdate specifies fields to update on an issue.
// Pointer fields: nil means no change, non-nil means set to that value.
// For Assignee: empty string means unassign.
// For Labels: nil means no change, empty slice means clear all labels.
type IssueUpdate struct {
	Summary     *string
	Description *string
	Priority    *string
	Assignee    *string
	Labels      *[]string
}

// IssueService defines operations for managing Jira issues.
type IssueService interface {
	// Create creates a new issue and returns it with Key populated.
	Create(ctx context.Context, projectKey, issueType, summary, description string) (*Issue, error)

	// Get retrieves an issue by its key.
	Get(ctx context.Context, key string) (*Issue, error)

	// List returns issues matching the filter criteria.
	List(ctx context.Context, filter IssueFilter) ([]*Issue, error)

	// Update modifies an existing issue.
	Update(ctx context.Context, key string, update IssueUpdate) error

	// AddComment adds a comment to an issue.
	AddComment(ctx context.Context, key, body string) (*Comment, error)

	// Transitions returns available workflow transitions for an issue.
	Transitions(ctx context.Context, key string) ([]Transition, error)

	// Transition moves an issue to a new status.
	Transition(ctx context.Context, key, transitionID string) error

	// Assign assigns an issue to a user by account ID.
	Assign(ctx context.Context, key, accountID string) error
}
