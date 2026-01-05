package jira4claude

import (
	"context"
	"time"
)

// Status constants for common Jira workflow states.
const (
	StatusToDo       = "To Do"
	StatusInProgress = "In Progress"
	StatusDone       = "Done"
)

// LinkInwardBlockedBy is the inward description for a "Blocks" link type.
// When issue A blocks issue B, issue B has an inward link with this description.
const LinkInwardBlockedBy = "is blocked by"

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

// IssueLinkType represents the type of relationship between linked issues.
type IssueLinkType struct {
	Name    string // e.g., "Blocks"
	Inward  string // e.g., "is blocked by"
	Outward string // e.g., "blocks"
}

// IssueLink represents a link between two issues.
type IssueLink struct {
	ID           string
	Type         IssueLinkType
	OutwardIssue *LinkedIssue // Present when this issue is the source
	InwardIssue  *LinkedIssue // Present when this issue is the target
}

// LinkedIssue contains summary information about a linked issue.
type LinkedIssue struct {
	Key     string
	Summary string
	Status  string
	Type    string
}

// Comment represents a comment on an issue.
type Comment struct {
	ID      string
	Author  *User
	Body    ADF // ADF document; conversion to markdown happens at CLI boundary
	Created time.Time
}

// Issue represents a Jira issue with its core fields.
type Issue struct {
	Key         string
	Project     string
	Summary     string
	Description ADF // ADF document; conversion to markdown happens at CLI boundary
	Status      string
	Type        string
	Priority    string
	Assignee    *User
	Reporter    *User
	Labels      []string
	Links       []*IssueLink
	Comments    []*Comment     // Comments on the issue
	Parent      *LinkedIssue   // Parent issue if this is a subtask; nil otherwise
	Subtasks    []*LinkedIssue // Subtasks if this is a parent issue; nil otherwise
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
	Parent   string   // Filter by parent issue key (for subtasks)
	Labels   []string // Issues must have ALL specified labels
	JQL      string   // Raw JQL query; overrides other fields if set
	Limit    int      // Maximum number of issues to return
}

// IssueUpdate specifies fields to update on an issue.
// Pointer fields: nil means no change, non-nil means set to that value.
// For Assignee: empty string means unassign.
// For Labels: nil means no change, empty slice means clear all labels.
type IssueUpdate struct {
	Summary     *string
	Description *ADF // ADF document; conversion from markdown happens at CLI boundary
	Priority    *string
	Assignee    *string
	Labels      *[]string
}

// IssueService defines operations for managing Jira issues.
type IssueService interface {
	// Create creates a new issue and returns it with Key and other
	// server-assigned fields populated.
	Create(ctx context.Context, issue *Issue) (*Issue, error)

	// Get retrieves an issue by its key.
	Get(ctx context.Context, key string) (*Issue, error)

	// List returns issues matching the filter criteria.
	List(ctx context.Context, filter IssueFilter) ([]*Issue, error)

	// Update modifies an existing issue and returns the updated issue.
	Update(ctx context.Context, key string, update IssueUpdate) (*Issue, error)

	// Delete deletes an issue by its key.
	Delete(ctx context.Context, key string) error

	// AddComment adds a comment to an issue.
	// The body is an ADF document; conversion from markdown happens at CLI boundary.
	AddComment(ctx context.Context, key string, body ADF) (*Comment, error)

	// Transitions returns available workflow transitions for an issue.
	Transitions(ctx context.Context, key string) ([]*Transition, error)

	// Transition moves an issue to a new status.
	Transition(ctx context.Context, key, transitionID string) error

	// Assign assigns an issue to a user by account ID.
	Assign(ctx context.Context, key, accountID string) error

	// Link creates a link between two issues.
	// The linkType is the name of the link type (e.g., "Blocks").
	// The inwardKey is the issue that has the relationship, and the outwardKey
	// is the issue it points to. For example, Link(ctx, "A", "Blocks", "B")
	// means that issue A blocks issue B (A is the blocker, B is blocked).
	Link(ctx context.Context, inwardKey, linkType, outwardKey string) error

	// Unlink removes a link between two issues.
	// It finds and removes any link connecting the two issues.
	Unlink(ctx context.Context, key1, key2 string) error
}
