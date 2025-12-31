package main_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/fwojciec/jira4claude"
	main "github.com/fwojciec/jira4claude/cmd/jira4claude"
	"github.com/fwojciec/jira4claude/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test helpers

func makeApp(t *testing.T, svc jira4claude.IssueService, jsonOut bool) (*main.App, *bytes.Buffer) {
	t.Helper()
	var buf bytes.Buffer
	app := main.NewApp(
		&jira4claude.Config{Project: "TEST", Server: "https://test.atlassian.net"},
		svc,
		jsonOut,
		&buf,
	)
	return app, &buf
}

func makeIssue(key string) *jira4claude.Issue {
	return &jira4claude.Issue{
		Key:     key,
		Summary: "Test issue " + key,
		Status:  "To Do",
		Type:    "Task",
		Project: "TEST",
	}
}

// ViewCmd tests

func TestViewCmd(t *testing.T) {
	t.Parallel()

	t.Run("displays issue details", func(t *testing.T) {
		t.Parallel()

		svc := &mock.IssueService{
			GetFn: func(ctx context.Context, key string) (*jira4claude.Issue, error) {
				return &jira4claude.Issue{
					Key:     "TEST-1",
					Summary: "Test issue",
					Status:  "To Do",
					Type:    "Task",
				}, nil
			},
		}

		app, buf := makeApp(t, svc, false)
		cmd := main.ViewCmd{Key: "TEST-1"}
		err := cmd.Run(app)

		require.NoError(t, err)
		assert.Contains(t, buf.String(), "TEST-1")
		assert.Contains(t, buf.String(), "Test issue")
	})

	t.Run("returns service error", func(t *testing.T) {
		t.Parallel()

		svc := &mock.IssueService{
			GetFn: func(ctx context.Context, key string) (*jira4claude.Issue, error) {
				return nil, errors.New("issue not found")
			},
		}

		app, _ := makeApp(t, svc, false)
		cmd := main.ViewCmd{Key: "NOTFOUND-1"}
		err := cmd.Run(app)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "issue not found")
	})

	t.Run("outputs JSON when requested", func(t *testing.T) {
		t.Parallel()

		svc := &mock.IssueService{
			GetFn: func(ctx context.Context, key string) (*jira4claude.Issue, error) {
				return &jira4claude.Issue{
					Key:     "TEST-1",
					Summary: "Test issue",
					Status:  "To Do",
					Type:    "Task",
				}, nil
			},
		}

		app, buf := makeApp(t, svc, true)
		cmd := main.ViewCmd{Key: "TEST-1"}
		err := cmd.Run(app)

		require.NoError(t, err)
		var result map[string]any
		require.NoError(t, json.Unmarshal(buf.Bytes(), &result))
		assert.Equal(t, "TEST-1", result["key"])
	})

	t.Run("shows parent when present", func(t *testing.T) {
		t.Parallel()

		svc := &mock.IssueService{
			GetFn: func(ctx context.Context, key string) (*jira4claude.Issue, error) {
				return &jira4claude.Issue{
					Key:     "TEST-2",
					Summary: "Test subtask",
					Status:  "To Do",
					Type:    "Subtask",
					Parent:  "TEST-1",
				}, nil
			},
		}

		app, buf := makeApp(t, svc, false)
		cmd := main.ViewCmd{Key: "TEST-2"}
		err := cmd.Run(app)

		require.NoError(t, err)
		assert.Contains(t, buf.String(), "Parent:")
		assert.Contains(t, buf.String(), "TEST-1")
	})

	t.Run("hides parent line when empty", func(t *testing.T) {
		t.Parallel()

		svc := &mock.IssueService{
			GetFn: func(ctx context.Context, key string) (*jira4claude.Issue, error) {
				return &jira4claude.Issue{
					Key:     "TEST-1",
					Summary: "Test issue",
					Status:  "To Do",
					Type:    "Task",
					Parent:  "",
				}, nil
			},
		}

		app, buf := makeApp(t, svc, false)
		cmd := main.ViewCmd{Key: "TEST-1"}
		err := cmd.Run(app)

		require.NoError(t, err)
		assert.NotContains(t, buf.String(), "Parent:")
	})
}

// EditCmd tests

func TestEditCmd(t *testing.T) {
	t.Parallel()

	t.Run("updates issue successfully", func(t *testing.T) {
		t.Parallel()

		var capturedUpdate jira4claude.IssueUpdate
		svc := &mock.IssueService{
			UpdateFn: func(ctx context.Context, key string, update jira4claude.IssueUpdate) (*jira4claude.Issue, error) {
				capturedUpdate = update
				return makeIssue(key), nil
			},
		}

		app, buf := makeApp(t, svc, false)
		summary := "New summary"
		cmd := main.EditCmd{Key: "TEST-1", Summary: &summary}
		err := cmd.Run(app)

		require.NoError(t, err)
		require.NotNil(t, capturedUpdate.Summary)
		assert.Equal(t, "New summary", *capturedUpdate.Summary)
		assert.Contains(t, buf.String(), "Updated: TEST-1")
	})

	t.Run("returns service error", func(t *testing.T) {
		t.Parallel()

		svc := &mock.IssueService{
			UpdateFn: func(ctx context.Context, key string, update jira4claude.IssueUpdate) (*jira4claude.Issue, error) {
				return nil, errors.New("update failed")
			},
		}

		app, _ := makeApp(t, svc, false)
		summary := "New summary"
		cmd := main.EditCmd{Key: "TEST-1", Summary: &summary}
		err := cmd.Run(app)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "update failed")
	})

	t.Run("clears labels when requested", func(t *testing.T) {
		t.Parallel()

		var capturedUpdate jira4claude.IssueUpdate
		svc := &mock.IssueService{
			UpdateFn: func(ctx context.Context, key string, update jira4claude.IssueUpdate) (*jira4claude.Issue, error) {
				capturedUpdate = update
				return makeIssue(key), nil
			},
		}

		app, _ := makeApp(t, svc, false)
		cmd := main.EditCmd{Key: "TEST-1", ClearLabels: true}
		err := cmd.Run(app)

		require.NoError(t, err)
		require.NotNil(t, capturedUpdate.Labels)
		assert.Empty(t, *capturedUpdate.Labels)
	})

	t.Run("sets labels when provided", func(t *testing.T) {
		t.Parallel()

		var capturedUpdate jira4claude.IssueUpdate
		svc := &mock.IssueService{
			UpdateFn: func(ctx context.Context, key string, update jira4claude.IssueUpdate) (*jira4claude.Issue, error) {
				capturedUpdate = update
				return makeIssue(key), nil
			},
		}

		app, _ := makeApp(t, svc, false)
		cmd := main.EditCmd{Key: "TEST-1", Labels: []string{"bug", "urgent"}}
		err := cmd.Run(app)

		require.NoError(t, err)
		require.NotNil(t, capturedUpdate.Labels)
		assert.Equal(t, []string{"bug", "urgent"}, *capturedUpdate.Labels)
	})

	t.Run("outputs JSON when requested", func(t *testing.T) {
		t.Parallel()

		svc := &mock.IssueService{
			UpdateFn: func(ctx context.Context, key string, update jira4claude.IssueUpdate) (*jira4claude.Issue, error) {
				return makeIssue(key), nil
			},
		}

		app, buf := makeApp(t, svc, true)
		summary := "New summary"
		cmd := main.EditCmd{Key: "TEST-1", Summary: &summary}
		err := cmd.Run(app)

		require.NoError(t, err)
		var result map[string]any
		require.NoError(t, json.Unmarshal(buf.Bytes(), &result))
		assert.Equal(t, "TEST-1", result["key"])
	})
}

// CommentCmd tests

func TestCommentCmd(t *testing.T) {
	t.Parallel()

	t.Run("adds comment successfully", func(t *testing.T) {
		t.Parallel()

		var capturedBody string
		svc := &mock.IssueService{
			AddCommentFn: func(ctx context.Context, key, body string) (*jira4claude.Comment, error) {
				capturedBody = body
				return &jira4claude.Comment{
					ID:      "12345",
					Body:    body,
					Author:  &jira4claude.User{DisplayName: "Test User"},
					Created: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
				}, nil
			},
		}

		app, buf := makeApp(t, svc, false)
		cmd := main.CommentCmd{Key: "TEST-1", Body: "This is a comment"}
		err := cmd.Run(app)

		require.NoError(t, err)
		assert.Equal(t, "This is a comment", capturedBody)
		assert.Contains(t, buf.String(), "Added comment 12345 to TEST-1")
	})

	t.Run("returns service error", func(t *testing.T) {
		t.Parallel()

		svc := &mock.IssueService{
			AddCommentFn: func(ctx context.Context, key, body string) (*jira4claude.Comment, error) {
				return nil, errors.New("comment failed")
			},
		}

		app, _ := makeApp(t, svc, false)
		cmd := main.CommentCmd{Key: "TEST-1", Body: "Comment"}
		err := cmd.Run(app)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "comment failed")
	})

	t.Run("outputs JSON when requested", func(t *testing.T) {
		t.Parallel()

		svc := &mock.IssueService{
			AddCommentFn: func(ctx context.Context, key, body string) (*jira4claude.Comment, error) {
				return &jira4claude.Comment{
					ID:      "12345",
					Body:    body,
					Author:  &jira4claude.User{DisplayName: "Test User"},
					Created: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
				}, nil
			},
		}

		app, buf := makeApp(t, svc, true)
		cmd := main.CommentCmd{Key: "TEST-1", Body: "Comment"}
		err := cmd.Run(app)

		require.NoError(t, err)
		var result map[string]any
		require.NoError(t, json.Unmarshal(buf.Bytes(), &result))
		assert.Equal(t, "12345", result["id"])
		assert.Equal(t, "Test User", result["author"])
	})
}

// TransitionCmd tests

func TestTransitionCmd(t *testing.T) {
	t.Parallel()

	t.Run("lists available transitions", func(t *testing.T) {
		t.Parallel()

		svc := &mock.IssueService{
			TransitionsFn: func(ctx context.Context, key string) ([]*jira4claude.Transition, error) {
				return []*jira4claude.Transition{
					{ID: "11", Name: "In Progress"},
					{ID: "21", Name: "Done"},
				}, nil
			},
		}

		app, buf := makeApp(t, svc, false)
		cmd := main.TransitionCmd{Key: "TEST-1", ListOnly: true}
		err := cmd.Run(app)

		require.NoError(t, err)
		assert.Contains(t, buf.String(), "Available transitions")
		assert.Contains(t, buf.String(), "In Progress")
		assert.Contains(t, buf.String(), "Done")
	})

	t.Run("transitions by status name", func(t *testing.T) {
		t.Parallel()

		var capturedTransitionID string
		svc := &mock.IssueService{
			TransitionsFn: func(ctx context.Context, key string) ([]*jira4claude.Transition, error) {
				return []*jira4claude.Transition{
					{ID: "11", Name: "In Progress"},
					{ID: "21", Name: "Done"},
				}, nil
			},
			TransitionFn: func(ctx context.Context, key, transitionID string) error {
				capturedTransitionID = transitionID
				return nil
			},
		}

		app, buf := makeApp(t, svc, false)
		cmd := main.TransitionCmd{Key: "TEST-1", Status: "Done"}
		err := cmd.Run(app)

		require.NoError(t, err)
		assert.Equal(t, "21", capturedTransitionID)
		assert.Contains(t, buf.String(), "Transitioned TEST-1")
	})

	t.Run("transitions by ID", func(t *testing.T) {
		t.Parallel()

		var capturedTransitionID string
		svc := &mock.IssueService{
			TransitionsFn: func(ctx context.Context, key string) ([]*jira4claude.Transition, error) {
				return []*jira4claude.Transition{
					{ID: "11", Name: "In Progress"},
				}, nil
			},
			TransitionFn: func(ctx context.Context, key, transitionID string) error {
				capturedTransitionID = transitionID
				return nil
			},
		}

		app, _ := makeApp(t, svc, false)
		cmd := main.TransitionCmd{Key: "TEST-1", TransitionID: "11"}
		err := cmd.Run(app)

		require.NoError(t, err)
		assert.Equal(t, "11", capturedTransitionID)
	})

	t.Run("returns error for missing status or ID", func(t *testing.T) {
		t.Parallel()

		svc := &mock.IssueService{
			TransitionsFn: func(ctx context.Context, key string) ([]*jira4claude.Transition, error) {
				return []*jira4claude.Transition{}, nil
			},
		}

		app, _ := makeApp(t, svc, false)
		cmd := main.TransitionCmd{Key: "TEST-1"}
		err := cmd.Run(app)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "either --status or --transition-id is required")
	})

	t.Run("returns error for unknown status", func(t *testing.T) {
		t.Parallel()

		svc := &mock.IssueService{
			TransitionsFn: func(ctx context.Context, key string) ([]*jira4claude.Transition, error) {
				return []*jira4claude.Transition{
					{ID: "11", Name: "In Progress"},
				}, nil
			},
		}

		app, _ := makeApp(t, svc, false)
		cmd := main.TransitionCmd{Key: "TEST-1", Status: "Unknown"}
		err := cmd.Run(app)

		require.Error(t, err)
		assert.Contains(t, err.Error(), `status "Unknown" not found`)
		assert.Contains(t, err.Error(), "In Progress")
	})

	t.Run("outputs JSON when listing transitions", func(t *testing.T) {
		t.Parallel()

		svc := &mock.IssueService{
			TransitionsFn: func(ctx context.Context, key string) ([]*jira4claude.Transition, error) {
				return []*jira4claude.Transition{
					{ID: "11", Name: "In Progress"},
				}, nil
			},
		}

		app, buf := makeApp(t, svc, true)
		cmd := main.TransitionCmd{Key: "TEST-1", ListOnly: true}
		err := cmd.Run(app)

		require.NoError(t, err)
		var result []map[string]any
		require.NoError(t, json.Unmarshal(buf.Bytes(), &result))
		require.Len(t, result, 1)
		assert.Equal(t, "11", result[0]["id"])
		assert.Equal(t, "In Progress", result[0]["name"])
	})

	t.Run("outputs JSON when transition succeeds", func(t *testing.T) {
		t.Parallel()

		svc := &mock.IssueService{
			TransitionsFn: func(ctx context.Context, key string) ([]*jira4claude.Transition, error) {
				return []*jira4claude.Transition{
					{ID: "21", Name: "Done"},
				}, nil
			},
			TransitionFn: func(ctx context.Context, key, transitionID string) error {
				return nil
			},
		}

		app, buf := makeApp(t, svc, true)
		cmd := main.TransitionCmd{Key: "TEST-1", Status: "Done"}
		err := cmd.Run(app)

		require.NoError(t, err)
		var result map[string]any
		require.NoError(t, json.Unmarshal(buf.Bytes(), &result))
		assert.Equal(t, "TEST-1", result["key"])
		assert.Equal(t, true, result["transitioned"])
	})
}

// AssignCmd tests

func TestAssignCmd(t *testing.T) {
	t.Parallel()

	t.Run("assigns issue to user", func(t *testing.T) {
		t.Parallel()

		var capturedAccountID string
		svc := &mock.IssueService{
			AssignFn: func(ctx context.Context, key, accountID string) error {
				capturedAccountID = accountID
				return nil
			},
		}

		app, buf := makeApp(t, svc, false)
		cmd := main.AssignCmd{Key: "TEST-1", AccountID: "user123"}
		err := cmd.Run(app)

		require.NoError(t, err)
		assert.Equal(t, "user123", capturedAccountID)
		assert.Contains(t, buf.String(), "Assigned TEST-1 to user123")
	})

	t.Run("unassigns issue when no account ID", func(t *testing.T) {
		t.Parallel()

		var capturedAccountID string
		svc := &mock.IssueService{
			AssignFn: func(ctx context.Context, key, accountID string) error {
				capturedAccountID = accountID
				return nil
			},
		}

		app, buf := makeApp(t, svc, false)
		cmd := main.AssignCmd{Key: "TEST-1", AccountID: ""}
		err := cmd.Run(app)

		require.NoError(t, err)
		assert.Empty(t, capturedAccountID)
		assert.Contains(t, buf.String(), "Unassigned TEST-1")
	})

	t.Run("returns service error", func(t *testing.T) {
		t.Parallel()

		svc := &mock.IssueService{
			AssignFn: func(ctx context.Context, key, accountID string) error {
				return errors.New("assign failed")
			},
		}

		app, _ := makeApp(t, svc, false)
		cmd := main.AssignCmd{Key: "TEST-1", AccountID: "user123"}
		err := cmd.Run(app)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "assign failed")
	})

	t.Run("outputs JSON when assigned", func(t *testing.T) {
		t.Parallel()

		svc := &mock.IssueService{
			AssignFn: func(ctx context.Context, key, accountID string) error {
				return nil
			},
		}

		app, buf := makeApp(t, svc, true)
		cmd := main.AssignCmd{Key: "TEST-1", AccountID: "user123"}
		err := cmd.Run(app)

		require.NoError(t, err)
		var result map[string]any
		require.NoError(t, json.Unmarshal(buf.Bytes(), &result))
		assert.Equal(t, "TEST-1", result["key"])
		assert.Equal(t, "user123", result["assigned"])
	})

	t.Run("outputs JSON when unassigned", func(t *testing.T) {
		t.Parallel()

		svc := &mock.IssueService{
			AssignFn: func(ctx context.Context, key, accountID string) error {
				return nil
			},
		}

		app, buf := makeApp(t, svc, true)
		cmd := main.AssignCmd{Key: "TEST-1", AccountID: ""}
		err := cmd.Run(app)

		require.NoError(t, err)
		var result map[string]any
		require.NoError(t, json.Unmarshal(buf.Bytes(), &result))
		assert.Equal(t, "TEST-1", result["key"])
		assert.Equal(t, true, result["unassigned"])
	})
}

// LinkCmd tests

func TestLinkCmd(t *testing.T) {
	t.Parallel()

	t.Run("links issues successfully", func(t *testing.T) {
		t.Parallel()

		var capturedArgs struct {
			inwardKey, linkType, outwardKey string
		}
		svc := &mock.IssueService{
			LinkFn: func(ctx context.Context, inwardKey, linkType, outwardKey string) error {
				capturedArgs.inwardKey = inwardKey
				capturedArgs.linkType = linkType
				capturedArgs.outwardKey = outwardKey
				return nil
			},
		}

		app, buf := makeApp(t, svc, false)
		cmd := main.LinkCmd{InwardKey: "TEST-1", LinkType: "Blocks", OutwardKey: "TEST-2"}
		err := cmd.Run(app)

		require.NoError(t, err)
		assert.Equal(t, "TEST-1", capturedArgs.inwardKey)
		assert.Equal(t, "Blocks", capturedArgs.linkType)
		assert.Equal(t, "TEST-2", capturedArgs.outwardKey)
		assert.Contains(t, buf.String(), "Linked TEST-1 Blocks TEST-2")
	})

	t.Run("returns service error", func(t *testing.T) {
		t.Parallel()

		svc := &mock.IssueService{
			LinkFn: func(ctx context.Context, inwardKey, linkType, outwardKey string) error {
				return errors.New("link failed")
			},
		}

		app, _ := makeApp(t, svc, false)
		cmd := main.LinkCmd{InwardKey: "TEST-1", LinkType: "Blocks", OutwardKey: "TEST-2"}
		err := cmd.Run(app)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "link failed")
	})

	t.Run("outputs JSON when requested", func(t *testing.T) {
		t.Parallel()

		svc := &mock.IssueService{
			LinkFn: func(ctx context.Context, inwardKey, linkType, outwardKey string) error {
				return nil
			},
		}

		app, buf := makeApp(t, svc, true)
		cmd := main.LinkCmd{InwardKey: "TEST-1", LinkType: "Blocks", OutwardKey: "TEST-2"}
		err := cmd.Run(app)

		require.NoError(t, err)
		var result map[string]any
		require.NoError(t, json.Unmarshal(buf.Bytes(), &result))
		assert.Equal(t, true, result["linked"])
		assert.Equal(t, "TEST-1", result["inwardKey"])
		assert.Equal(t, "Blocks", result["linkType"])
		assert.Equal(t, "TEST-2", result["outwardKey"])
	})
}

// UnlinkCmd tests

func TestUnlinkCmd(t *testing.T) {
	t.Parallel()

	t.Run("unlinks issues successfully", func(t *testing.T) {
		t.Parallel()

		var capturedKeys struct{ key1, key2 string }
		svc := &mock.IssueService{
			UnlinkFn: func(ctx context.Context, key1, key2 string) error {
				capturedKeys.key1 = key1
				capturedKeys.key2 = key2
				return nil
			},
		}

		app, buf := makeApp(t, svc, false)
		cmd := main.UnlinkCmd{Key1: "TEST-1", Key2: "TEST-2"}
		err := cmd.Run(app)

		require.NoError(t, err)
		assert.Equal(t, "TEST-1", capturedKeys.key1)
		assert.Equal(t, "TEST-2", capturedKeys.key2)
		assert.Contains(t, buf.String(), "Unlinked TEST-1 and TEST-2")
	})

	t.Run("returns service error", func(t *testing.T) {
		t.Parallel()

		svc := &mock.IssueService{
			UnlinkFn: func(ctx context.Context, key1, key2 string) error {
				return errors.New("unlink failed")
			},
		}

		app, _ := makeApp(t, svc, false)
		cmd := main.UnlinkCmd{Key1: "TEST-1", Key2: "TEST-2"}
		err := cmd.Run(app)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "unlink failed")
	})

	t.Run("outputs JSON when requested", func(t *testing.T) {
		t.Parallel()

		svc := &mock.IssueService{
			UnlinkFn: func(ctx context.Context, key1, key2 string) error {
				return nil
			},
		}

		app, buf := makeApp(t, svc, true)
		cmd := main.UnlinkCmd{Key1: "TEST-1", Key2: "TEST-2"}
		err := cmd.Run(app)

		require.NoError(t, err)
		var result map[string]any
		require.NoError(t, json.Unmarshal(buf.Bytes(), &result))
		assert.Equal(t, true, result["unlinked"])
		assert.Equal(t, "TEST-1", result["key1"])
		assert.Equal(t, "TEST-2", result["key2"])
	})
}

// ReadyCmd tests

func TestReadyCmd(t *testing.T) {
	t.Parallel()

	t.Run("shows only ready issues", func(t *testing.T) {
		t.Parallel()

		svc := &mock.IssueService{
			ListFn: func(ctx context.Context, filter jira4claude.IssueFilter) ([]*jira4claude.Issue, error) {
				return []*jira4claude.Issue{
					{Key: "TEST-1", Summary: "Ready issue", Status: "To Do"},
					{Key: "TEST-2", Summary: "Blocked issue", Status: "To Do", Links: []*jira4claude.IssueLink{
						{
							Type:        jira4claude.IssueLinkType{Name: "Blocks", Inward: "is blocked by", Outward: "blocks"},
							InwardIssue: &jira4claude.LinkedIssue{Key: "TEST-3", Status: "In Progress"},
						},
					}},
				}, nil
			},
		}

		app, buf := makeApp(t, svc, false)
		cmd := main.ReadyCmd{}
		err := cmd.Run(app)

		require.NoError(t, err)
		assert.Contains(t, buf.String(), "TEST-1")
		assert.NotContains(t, buf.String(), "TEST-2")
	})

	t.Run("shows no issues message when none ready", func(t *testing.T) {
		t.Parallel()

		svc := &mock.IssueService{
			ListFn: func(ctx context.Context, filter jira4claude.IssueFilter) ([]*jira4claude.Issue, error) {
				return []*jira4claude.Issue{
					{Key: "TEST-1", Summary: "Blocked", Status: "To Do", Links: []*jira4claude.IssueLink{
						{
							Type:        jira4claude.IssueLinkType{Name: "Blocks", Inward: "is blocked by", Outward: "blocks"},
							InwardIssue: &jira4claude.LinkedIssue{Key: "TEST-2", Status: "In Progress"},
						},
					}},
				}, nil
			},
		}

		app, buf := makeApp(t, svc, false)
		cmd := main.ReadyCmd{}
		err := cmd.Run(app)

		require.NoError(t, err)
		assert.Contains(t, buf.String(), "No ready issues found")
	})

	t.Run("returns service error", func(t *testing.T) {
		t.Parallel()

		svc := &mock.IssueService{
			ListFn: func(ctx context.Context, filter jira4claude.IssueFilter) ([]*jira4claude.Issue, error) {
				return nil, errors.New("list failed")
			},
		}

		app, _ := makeApp(t, svc, false)
		cmd := main.ReadyCmd{}
		err := cmd.Run(app)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "list failed")
	})

	t.Run("outputs JSON when requested", func(t *testing.T) {
		t.Parallel()

		svc := &mock.IssueService{
			ListFn: func(ctx context.Context, filter jira4claude.IssueFilter) ([]*jira4claude.Issue, error) {
				return []*jira4claude.Issue{
					{Key: "TEST-1", Summary: "Ready issue", Status: "To Do"},
				}, nil
			},
		}

		app, buf := makeApp(t, svc, true)
		cmd := main.ReadyCmd{}
		err := cmd.Run(app)

		require.NoError(t, err)
		var result []map[string]any
		require.NoError(t, json.Unmarshal(buf.Bytes(), &result))
		require.Len(t, result, 1)
		assert.Equal(t, "TEST-1", result[0]["key"])
	})
}

// ListCmd tests

func TestListCmd(t *testing.T) {
	t.Parallel()

	t.Run("lists issues", func(t *testing.T) {
		t.Parallel()

		svc := &mock.IssueService{
			ListFn: func(ctx context.Context, filter jira4claude.IssueFilter) ([]*jira4claude.Issue, error) {
				return []*jira4claude.Issue{
					makeIssue("TEST-1"),
					makeIssue("TEST-2"),
				}, nil
			},
		}

		app, buf := makeApp(t, svc, false)
		cmd := main.ListCmd{}
		err := cmd.Run(app)

		require.NoError(t, err)
		assert.Contains(t, buf.String(), "TEST-1")
		assert.Contains(t, buf.String(), "TEST-2")
	})

	t.Run("shows no issues message when empty", func(t *testing.T) {
		t.Parallel()

		svc := &mock.IssueService{
			ListFn: func(ctx context.Context, filter jira4claude.IssueFilter) ([]*jira4claude.Issue, error) {
				return []*jira4claude.Issue{}, nil
			},
		}

		app, buf := makeApp(t, svc, false)
		cmd := main.ListCmd{}
		err := cmd.Run(app)

		require.NoError(t, err)
		assert.Contains(t, buf.String(), "No issues found")
	})

	t.Run("populates parent filter when specified", func(t *testing.T) {
		t.Parallel()

		var capturedFilter jira4claude.IssueFilter
		svc := &mock.IssueService{
			ListFn: func(ctx context.Context, filter jira4claude.IssueFilter) ([]*jira4claude.Issue, error) {
				capturedFilter = filter
				return []*jira4claude.Issue{}, nil
			},
		}

		app, _ := makeApp(t, svc, false)
		cmd := main.ListCmd{Parent: "TEST-1"}
		err := cmd.Run(app)

		require.NoError(t, err)
		assert.Equal(t, "TEST-1", capturedFilter.Parent)
	})

	t.Run("returns service error", func(t *testing.T) {
		t.Parallel()

		svc := &mock.IssueService{
			ListFn: func(ctx context.Context, filter jira4claude.IssueFilter) ([]*jira4claude.Issue, error) {
				return nil, errors.New("list failed")
			},
		}

		app, _ := makeApp(t, svc, false)
		cmd := main.ListCmd{}
		err := cmd.Run(app)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "list failed")
	})

	t.Run("outputs JSON when requested", func(t *testing.T) {
		t.Parallel()

		svc := &mock.IssueService{
			ListFn: func(ctx context.Context, filter jira4claude.IssueFilter) ([]*jira4claude.Issue, error) {
				return []*jira4claude.Issue{makeIssue("TEST-1")}, nil
			},
		}

		app, buf := makeApp(t, svc, true)
		cmd := main.ListCmd{}
		err := cmd.Run(app)

		require.NoError(t, err)
		var result []map[string]any
		require.NoError(t, json.Unmarshal(buf.Bytes(), &result))
		require.Len(t, result, 1)
		assert.Equal(t, "TEST-1", result[0]["key"])
	})
}

// CreateCmd tests

func TestCreateCmd(t *testing.T) {
	t.Parallel()

	t.Run("creates issue successfully", func(t *testing.T) {
		t.Parallel()

		var capturedIssue *jira4claude.Issue
		svc := &mock.IssueService{
			CreateFn: func(ctx context.Context, issue *jira4claude.Issue) (*jira4claude.Issue, error) {
				capturedIssue = issue
				return &jira4claude.Issue{Key: "TEST-1"}, nil
			},
		}

		app, buf := makeApp(t, svc, false)
		cmd := main.CreateCmd{Summary: "New issue"}
		err := cmd.Run(app)

		require.NoError(t, err)
		assert.Equal(t, "New issue", capturedIssue.Summary)
		assert.Contains(t, buf.String(), "Created: TEST-1")
	})

	t.Run("sets parent field on issue", func(t *testing.T) {
		t.Parallel()

		var capturedIssue *jira4claude.Issue
		svc := &mock.IssueService{
			CreateFn: func(ctx context.Context, issue *jira4claude.Issue) (*jira4claude.Issue, error) {
				capturedIssue = issue
				return &jira4claude.Issue{Key: "TEST-2", Parent: issue.Parent}, nil
			},
		}

		app, _ := makeApp(t, svc, false)
		cmd := main.CreateCmd{Summary: "Test subtask", Parent: "TEST-1"}
		err := cmd.Run(app)

		require.NoError(t, err)
		require.NotNil(t, capturedIssue)
		assert.Equal(t, "TEST-1", capturedIssue.Parent)
	})

	t.Run("auto-sets type to Subtask when parent is specified", func(t *testing.T) {
		t.Parallel()

		var capturedIssue *jira4claude.Issue
		svc := &mock.IssueService{
			CreateFn: func(ctx context.Context, issue *jira4claude.Issue) (*jira4claude.Issue, error) {
				capturedIssue = issue
				return &jira4claude.Issue{Key: "TEST-2", Type: issue.Type}, nil
			},
		}

		app, _ := makeApp(t, svc, false)
		cmd := main.CreateCmd{Summary: "Test subtask", Parent: "TEST-1", Type: "Task"}
		err := cmd.Run(app)

		require.NoError(t, err)
		require.NotNil(t, capturedIssue)
		assert.Equal(t, "Subtask", capturedIssue.Type)
	})

	t.Run("returns service error", func(t *testing.T) {
		t.Parallel()

		svc := &mock.IssueService{
			CreateFn: func(ctx context.Context, issue *jira4claude.Issue) (*jira4claude.Issue, error) {
				return nil, errors.New("create failed")
			},
		}

		app, _ := makeApp(t, svc, false)
		cmd := main.CreateCmd{Summary: "New issue"}
		err := cmd.Run(app)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "create failed")
	})

	t.Run("outputs JSON when requested", func(t *testing.T) {
		t.Parallel()

		svc := &mock.IssueService{
			CreateFn: func(ctx context.Context, issue *jira4claude.Issue) (*jira4claude.Issue, error) {
				return &jira4claude.Issue{Key: "TEST-1"}, nil
			},
		}

		app, buf := makeApp(t, svc, true)
		cmd := main.CreateCmd{Summary: "New issue"}
		err := cmd.Run(app)

		require.NoError(t, err)
		var result map[string]any
		require.NoError(t, json.Unmarshal(buf.Bytes(), &result))
		assert.Equal(t, "TEST-1", result["key"])
	})
}
