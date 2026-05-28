package main_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/alecthomas/kong"
	"github.com/fwojciec/jira4claude"
	main "github.com/fwojciec/jira4claude/cmd/j4c"
	"github.com/fwojciec/jira4claude/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockConverter returns a converter that creates valid ADF from markdown.
func mockConverter() *mock.Converter {
	return &mock.Converter{
		ToADFFn: func(markdown string) (*jira4claude.ADFNode, []string) {
			return &jira4claude.ADFNode{
				Type:    "doc",
				Version: 1,
				Content: []jira4claude.ADFNode{
					{
						Type: "paragraph",
						Content: []jira4claude.ADFNode{
							{
								Type: "text",
								Text: markdown,
							},
						},
					},
				},
			}, nil
		},
		ToMarkdownFn: func(adf *jira4claude.ADFNode) (string, []string) {
			var result string
			for _, block := range adf.Content {
				for _, node := range block.Content {
					result += node.Text
				}
			}
			return result, nil
		},
	}
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

// IssueTransitionsCmd tests

func TestIssueTransitionsCmd(t *testing.T) {
	t.Parallel()

	t.Run("lists available transitions", func(t *testing.T) {
		t.Parallel()

		svc := &mock.IssueService{
			TransitionsFn: func(ctx context.Context, key string) ([]*jira4claude.Transition, error) {
				require.Equal(t, "TEST-123", key)
				return []*jira4claude.Transition{
					{ID: "21", Name: "In Progress"},
					{ID: "31", Name: "Done"},
				}, nil
			},
		}

		printer := &mock.Printer{}
		ctx := &main.IssueContext{
			Service:   svc,
			Printer:   printer,
			Converter: &mock.Converter{},
			Config:    &jira4claude.Config{Project: "TEST"},
		}

		cmd := main.IssueTransitionsCmd{Key: "TEST-123"}
		err := cmd.Run(ctx)

		require.NoError(t, err)
		require.Len(t, printer.TransitionsCalls, 1)
		assert.Equal(t, "TEST-123", printer.TransitionsCalls[0].Key)
		assert.Len(t, printer.TransitionsCalls[0].Transitions, 2)
		assert.Equal(t, "In Progress", printer.TransitionsCalls[0].Transitions[0].Name)
		assert.Equal(t, "Done", printer.TransitionsCalls[0].Transitions[1].Name)
	})

	t.Run("returns error when service fails", func(t *testing.T) {
		t.Parallel()

		svc := &mock.IssueService{
			TransitionsFn: func(ctx context.Context, key string) ([]*jira4claude.Transition, error) {
				require.Equal(t, "INVALID-123", key)
				return nil, &jira4claude.Error{Code: jira4claude.ENotFound, Message: "Issue not found"}
			},
		}

		printer := &mock.Printer{}
		ctx := &main.IssueContext{
			Service:   svc,
			Printer:   printer,
			Converter: &mock.Converter{},
			Config:    &jira4claude.Config{Project: "TEST"},
		}

		cmd := main.IssueTransitionsCmd{Key: "INVALID-123"}
		err := cmd.Run(ctx)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "Issue not found")
		assert.Empty(t, printer.TransitionsCalls)
	})
}

// IssueTransitionCmd tests

func TestIssueTransitionCmd_InvalidStatusShowsQuotedOptions(t *testing.T) {
	t.Parallel()

	svc := &mock.IssueService{
		TransitionsFn: func(ctx context.Context, key string) ([]*jira4claude.Transition, error) {
			return []*jira4claude.Transition{
				{ID: "21", Name: "In Progress"},
				{ID: "31", Name: "Done"},
			}, nil
		},
	}

	printer := &mock.Printer{}
	ctx := &main.IssueContext{
		Service:   svc,
		Printer:   printer,
		Converter: &mock.Converter{},
		Config:    &jira4claude.Config{Project: "TEST"},
	}

	cmd := &main.IssueTransitionCmd{
		Key:    "TEST-123",
		Status: "invalid-status",
	}

	err := cmd.Run(ctx)

	require.Error(t, err)
	errMsg := err.Error()
	// Should quote user's invalid status and available options
	assert.Contains(t, errMsg, `"invalid-status"`)
	assert.Contains(t, errMsg, `"In Progress"`)
	assert.Contains(t, errMsg, `"Done"`)
}

// IssueCreateCmd tests

func TestIssueCreateCmd(t *testing.T) {
	t.Parallel()

	t.Run("always converts description as GFM", func(t *testing.T) {
		t.Parallel()

		var capturedIssue *jira4claude.Issue
		svc := &mock.IssueService{
			CreateFn: func(ctx context.Context, issue *jira4claude.Issue) (*jira4claude.Issue, error) {
				capturedIssue = issue
				return &jira4claude.Issue{Key: "TEST-1"}, nil
			},
		}

		printer := &mock.Printer{}
		ctx := &main.IssueContext{
			Service:   svc,
			Printer:   printer,
			Converter: mockConverter(),
			Config:    &jira4claude.Config{Project: "TEST", Server: "https://test.atlassian.net"},
		}
		cmd := main.IssueCreateCmd{
			Summary:     "Test issue",
			Description: "**bold** and *italic*",
		}
		err := cmd.Run(ctx)

		require.NoError(t, err)
		require.NotNil(t, capturedIssue)
		// Description should be ADF
		assert.Equal(t, "doc", capturedIssue.Description.Type)
	})

	t.Run("plain text input is valid GFM", func(t *testing.T) {
		t.Parallel()

		var capturedIssue *jira4claude.Issue
		svc := &mock.IssueService{
			CreateFn: func(ctx context.Context, issue *jira4claude.Issue) (*jira4claude.Issue, error) {
				capturedIssue = issue
				return &jira4claude.Issue{Key: "TEST-1"}, nil
			},
		}

		printer := &mock.Printer{}
		ctx := &main.IssueContext{
			Service:   svc,
			Printer:   printer,
			Converter: mockConverter(),
			Config:    &jira4claude.Config{Project: "TEST", Server: "https://test.atlassian.net"},
		}
		cmd := main.IssueCreateCmd{
			Summary:     "Test issue",
			Description: "plain text without formatting",
		}
		err := cmd.Run(ctx)

		require.NoError(t, err)
		require.NotNil(t, capturedIssue)
		// Plain text is valid GFM and should be converted to ADF
		assert.Equal(t, "doc", capturedIssue.Description.Type)
	})

	t.Run("skips conversion when description is empty", func(t *testing.T) {
		t.Parallel()

		var capturedIssue *jira4claude.Issue
		svc := &mock.IssueService{
			CreateFn: func(ctx context.Context, issue *jira4claude.Issue) (*jira4claude.Issue, error) {
				capturedIssue = issue
				return &jira4claude.Issue{Key: "TEST-1"}, nil
			},
		}

		printer := &mock.Printer{}
		ctx := &main.IssueContext{
			Service:   svc,
			Printer:   printer,
			Converter: mockConverter(),
			Config:    &jira4claude.Config{Project: "TEST", Server: "https://test.atlassian.net"},
		}
		cmd := main.IssueCreateCmd{
			Summary:     "Test issue",
			Description: "",
		}
		err := cmd.Run(ctx)

		require.NoError(t, err)
		require.NotNil(t, capturedIssue)
		// Description should remain empty
		assert.Empty(t, capturedIssue.Description)
	})

	t.Run("passes Sub-task type through when parent is specified", func(t *testing.T) {
		t.Parallel()

		var capturedIssue *jira4claude.Issue
		svc := &mock.IssueService{
			CreateFn: func(ctx context.Context, issue *jira4claude.Issue) (*jira4claude.Issue, error) {
				capturedIssue = issue
				return &jira4claude.Issue{Key: "TEST-2"}, nil
			},
		}

		printer := &mock.Printer{}
		ctx := &main.IssueContext{
			Service:   svc,
			Printer:   printer,
			Converter: mockConverter(),
			Config:    &jira4claude.Config{Project: "TEST", Server: "https://test.atlassian.net"},
		}
		cmd := main.IssueCreateCmd{
			Type:    "Sub-task",
			Summary: "Subtask issue",
			Parent:  "TEST-1",
		}
		err := cmd.Run(ctx)

		require.NoError(t, err)
		require.NotNil(t, capturedIssue)
		// Type must be exactly "Sub-task" (with hyphen) for Jira API
		assert.Equal(t, "Sub-task", capturedIssue.Type)
		require.NotNil(t, capturedIssue.Parent)
		assert.Equal(t, "TEST-1", capturedIssue.Parent.Key)
	})

	t.Run("preserves Task type when parent is specified (no coercion)", func(t *testing.T) {
		// Tasks (and Stories) can have an Epic parent in team-managed projects.
		// The CLI must not coerce non-empty --type to Sub-task.
		t.Parallel()

		var capturedIssue *jira4claude.Issue
		svc := &mock.IssueService{
			CreateFn: func(ctx context.Context, issue *jira4claude.Issue) (*jira4claude.Issue, error) {
				capturedIssue = issue
				return &jira4claude.Issue{Key: "TEST-3"}, nil
			},
		}

		printer := &mock.Printer{}
		ctx := &main.IssueContext{
			Service:   svc,
			Printer:   printer,
			Converter: mockConverter(),
			Config:    &jira4claude.Config{Project: "TEST", Server: "https://test.atlassian.net"},
		}
		cmd := main.IssueCreateCmd{
			Type:    "Task",
			Summary: "Task under epic",
			Parent:  "EPIC-1",
		}
		err := cmd.Run(ctx)

		require.NoError(t, err)
		require.NotNil(t, capturedIssue)
		assert.Equal(t, "Task", capturedIssue.Type)
		require.NotNil(t, capturedIssue.Parent)
		assert.Equal(t, "EPIC-1", capturedIssue.Parent.Key)
	})

	t.Run("assignee me creates issue then self-assigns", func(t *testing.T) {
		t.Parallel()

		var assignedKey, assignedID string
		svc := &mock.IssueService{
			CreateFn: func(ctx context.Context, issue *jira4claude.Issue) (*jira4claude.Issue, error) {
				return &jira4claude.Issue{Key: "TEST-1"}, nil
			},
			AssignFn: func(ctx context.Context, key, accountID string) error {
				assignedKey = key
				assignedID = accountID
				return nil
			},
		}
		userSvc := &mock.UserService{
			GetMyselfFn: func(ctx context.Context) (*jira4claude.User, error) {
				return &jira4claude.User{AccountID: "myself-123"}, nil
			},
		}

		printer := &mock.Printer{}
		ctx := &main.IssueContext{
			Service:     svc,
			UserService: userSvc,
			Printer:     printer,
			Converter:   mockConverter(),
			Config:      &jira4claude.Config{Project: "TEST", Server: "https://test.atlassian.net"},
		}
		cmd := main.IssueCreateCmd{
			Summary:  "Test issue",
			Assignee: "me",
		}
		err := cmd.Run(ctx)

		require.NoError(t, err)
		assert.Equal(t, "TEST-1", assignedKey)
		assert.Equal(t, "myself-123", assignedID)
		require.Len(t, printer.SuccessCalls, 1)
		assert.Equal(t, "Created:", printer.SuccessCalls[0].Msg)
	})

	t.Run("assignee email creates issue then assigns by email", func(t *testing.T) {
		t.Parallel()

		var assignedKey, assignedID string
		svc := &mock.IssueService{
			CreateFn: func(ctx context.Context, issue *jira4claude.Issue) (*jira4claude.Issue, error) {
				return &jira4claude.Issue{Key: "TEST-1"}, nil
			},
			AssignFn: func(ctx context.Context, key, accountID string) error {
				assignedKey = key
				assignedID = accountID
				return nil
			},
		}
		userSvc := &mock.UserService{
			FindUsersFn: func(ctx context.Context, query string) ([]*jira4claude.User, error) {
				return []*jira4claude.User{{AccountID: "user-456"}}, nil
			},
		}

		printer := &mock.Printer{}
		ctx := &main.IssueContext{
			Service:     svc,
			UserService: userSvc,
			Printer:     printer,
			Converter:   mockConverter(),
			Config:      &jira4claude.Config{Project: "TEST", Server: "https://test.atlassian.net"},
		}
		cmd := main.IssueCreateCmd{
			Summary:  "Test issue",
			Assignee: "user@example.com",
		}
		err := cmd.Run(ctx)

		require.NoError(t, err)
		assert.Equal(t, "TEST-1", assignedKey)
		assert.Equal(t, "user-456", assignedID)
		require.Len(t, printer.SuccessCalls, 1)
		assert.Equal(t, "Created:", printer.SuccessCalls[0].Msg)
	})

	t.Run("omitting assignee creates without assignment", func(t *testing.T) {
		t.Parallel()

		assignCalled := false
		svc := &mock.IssueService{
			CreateFn: func(ctx context.Context, issue *jira4claude.Issue) (*jira4claude.Issue, error) {
				return &jira4claude.Issue{Key: "TEST-1"}, nil
			},
			AssignFn: func(ctx context.Context, key, accountID string) error {
				assignCalled = true
				return nil
			},
		}

		printer := &mock.Printer{}
		ctx := &main.IssueContext{
			Service:   svc,
			Printer:   printer,
			Converter: mockConverter(),
			Config:    &jira4claude.Config{Project: "TEST", Server: "https://test.atlassian.net"},
		}
		cmd := main.IssueCreateCmd{
			Summary: "Test issue",
		}
		err := cmd.Run(ctx)

		require.NoError(t, err)
		assert.False(t, assignCalled, "Assign should not be called when assignee is empty")
		require.Len(t, printer.SuccessCalls, 1)
		assert.Equal(t, "Created:", printer.SuccessCalls[0].Msg)
	})

	t.Run("assignment failure returns error but creation success is printed", func(t *testing.T) {
		t.Parallel()

		svc := &mock.IssueService{
			CreateFn: func(ctx context.Context, issue *jira4claude.Issue) (*jira4claude.Issue, error) {
				return &jira4claude.Issue{Key: "TEST-1"}, nil
			},
			AssignFn: func(ctx context.Context, key, accountID string) error {
				return &jira4claude.Error{Code: jira4claude.EInternal, Message: "assign failed"}
			},
		}
		userSvc := &mock.UserService{
			GetMyselfFn: func(ctx context.Context) (*jira4claude.User, error) {
				return &jira4claude.User{AccountID: "myself-123"}, nil
			},
		}

		printer := &mock.Printer{}
		ctx := &main.IssueContext{
			Service:     svc,
			UserService: userSvc,
			Printer:     printer,
			Converter:   mockConverter(),
			Config:      &jira4claude.Config{Project: "TEST", Server: "https://test.atlassian.net"},
		}
		cmd := main.IssueCreateCmd{
			Summary:  "Test issue",
			Assignee: "me",
		}
		err := cmd.Run(ctx)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "assign failed")
		// Creation success should still be printed before the assignment error
		require.Len(t, printer.SuccessCalls, 1)
		assert.Equal(t, "Created:", printer.SuccessCalls[0].Msg)
	})

	t.Run("resolve failure returns error but creation success is printed", func(t *testing.T) {
		t.Parallel()

		svc := &mock.IssueService{
			CreateFn: func(ctx context.Context, issue *jira4claude.Issue) (*jira4claude.Issue, error) {
				return &jira4claude.Issue{Key: "TEST-1"}, nil
			},
		}
		userSvc := &mock.UserService{
			GetMyselfFn: func(ctx context.Context) (*jira4claude.User, error) {
				return nil, &jira4claude.Error{Code: jira4claude.EInternal, Message: "auth failed"}
			},
		}

		printer := &mock.Printer{}
		ctx := &main.IssueContext{
			Service:     svc,
			UserService: userSvc,
			Printer:     printer,
			Converter:   mockConverter(),
			Config:      &jira4claude.Config{Project: "TEST", Server: "https://test.atlassian.net"},
		}
		cmd := main.IssueCreateCmd{
			Summary:  "Test issue",
			Assignee: "me",
		}
		err := cmd.Run(ctx)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "auth failed")
		require.Len(t, printer.SuccessCalls, 1)
		assert.Equal(t, "Created:", printer.SuccessCalls[0].Msg)
	})
}

// IssueUpdateCmd tests

func TestIssueUpdateCmd(t *testing.T) {
	t.Parallel()

	t.Run("always converts description as GFM", func(t *testing.T) {
		t.Parallel()

		var capturedUpdate jira4claude.IssueUpdate
		svc := &mock.IssueService{
			UpdateFn: func(ctx context.Context, key string, update jira4claude.IssueUpdate) (*jira4claude.Issue, error) {
				capturedUpdate = update
				return makeIssue(key), nil
			},
		}

		printer := &mock.Printer{}
		ctx := &main.IssueContext{
			Service:   svc,
			Printer:   printer,
			Converter: mockConverter(),
			Config:    &jira4claude.Config{Project: "TEST", Server: "https://test.atlassian.net"},
		}
		description := "**bold** and *italic*"
		cmd := main.IssueUpdateCmd{Key: "TEST-1", Description: &description}
		err := cmd.Run(ctx)

		require.NoError(t, err)
		require.NotNil(t, capturedUpdate.Description)
		// Description should be ADF
		assert.Equal(t, "doc", (*capturedUpdate.Description).Type)
	})

	t.Run("plain text input is valid GFM", func(t *testing.T) {
		t.Parallel()

		var capturedUpdate jira4claude.IssueUpdate
		svc := &mock.IssueService{
			UpdateFn: func(ctx context.Context, key string, update jira4claude.IssueUpdate) (*jira4claude.Issue, error) {
				capturedUpdate = update
				return makeIssue(key), nil
			},
		}

		printer := &mock.Printer{}
		ctx := &main.IssueContext{
			Service:   svc,
			Printer:   printer,
			Converter: mockConverter(),
			Config:    &jira4claude.Config{Project: "TEST", Server: "https://test.atlassian.net"},
		}
		description := "plain text without formatting"
		cmd := main.IssueUpdateCmd{Key: "TEST-1", Description: &description}
		err := cmd.Run(ctx)

		require.NoError(t, err)
		require.NotNil(t, capturedUpdate.Description)
		// Plain text is valid GFM and should be converted to ADF
		assert.Equal(t, "doc", (*capturedUpdate.Description).Type)
	})

	t.Run("skips conversion when description is empty", func(t *testing.T) {
		t.Parallel()

		var capturedUpdate jira4claude.IssueUpdate
		svc := &mock.IssueService{
			UpdateFn: func(ctx context.Context, key string, update jira4claude.IssueUpdate) (*jira4claude.Issue, error) {
				capturedUpdate = update
				return makeIssue(key), nil
			},
		}

		printer := &mock.Printer{}
		ctx := &main.IssueContext{
			Service:   svc,
			Printer:   printer,
			Converter: mockConverter(),
			Config:    &jira4claude.Config{Project: "TEST", Server: "https://test.atlassian.net"},
		}
		description := ""
		cmd := main.IssueUpdateCmd{Key: "TEST-1", Description: &description}
		err := cmd.Run(ctx)

		require.NoError(t, err)
		// Empty description is not converted - nil is passed
		assert.Nil(t, capturedUpdate.Description)
	})

	t.Run("sets parent when parent flag provided", func(t *testing.T) {
		t.Parallel()

		var capturedUpdate jira4claude.IssueUpdate
		svc := &mock.IssueService{
			UpdateFn: func(ctx context.Context, key string, update jira4claude.IssueUpdate) (*jira4claude.Issue, error) {
				capturedUpdate = update
				result := makeIssue(key)
				result.Parent = &jira4claude.LinkedIssue{Key: "EPIC-1"}
				return result, nil
			},
		}

		printer := &mock.Printer{}
		ctx := &main.IssueContext{
			Service:   svc,
			Printer:   printer,
			Converter: mockConverter(),
			Config:    &jira4claude.Config{Project: "TEST", Server: "https://test.atlassian.net"},
		}
		parent := "EPIC-1"
		cmd := main.IssueUpdateCmd{Key: "TEST-5", Parent: &parent}
		err := cmd.Run(ctx)

		require.NoError(t, err)
		require.NotNil(t, capturedUpdate.Parent)
		assert.Equal(t, "EPIC-1", *capturedUpdate.Parent)
	})

	t.Run("clears parent when clear-parent flag set", func(t *testing.T) {
		t.Parallel()

		var capturedUpdate jira4claude.IssueUpdate
		svc := &mock.IssueService{
			UpdateFn: func(ctx context.Context, key string, update jira4claude.IssueUpdate) (*jira4claude.Issue, error) {
				capturedUpdate = update
				return makeIssue(key), nil
			},
		}

		printer := &mock.Printer{}
		ctx := &main.IssueContext{
			Service:   svc,
			Printer:   printer,
			Converter: mockConverter(),
			Config:    &jira4claude.Config{Project: "TEST", Server: "https://test.atlassian.net"},
		}
		cmd := main.IssueUpdateCmd{Key: "TEST-5", ClearParent: true}
		err := cmd.Run(ctx)

		require.NoError(t, err)
		require.NotNil(t, capturedUpdate.Parent)
		assert.Empty(t, *capturedUpdate.Parent)
	})

	t.Run("does not modify parent when neither flag set", func(t *testing.T) {
		t.Parallel()

		var capturedUpdate jira4claude.IssueUpdate
		svc := &mock.IssueService{
			UpdateFn: func(ctx context.Context, key string, update jira4claude.IssueUpdate) (*jira4claude.Issue, error) {
				capturedUpdate = update
				return makeIssue(key), nil
			},
		}

		printer := &mock.Printer{}
		ctx := &main.IssueContext{
			Service:   svc,
			Printer:   printer,
			Converter: mockConverter(),
			Config:    &jira4claude.Config{Project: "TEST", Server: "https://test.atlassian.net"},
		}
		summary := "Updated summary"
		cmd := main.IssueUpdateCmd{Key: "TEST-5", Summary: &summary}
		err := cmd.Run(ctx)

		require.NoError(t, err)
		// Parent should be nil (no change)
		assert.Nil(t, capturedUpdate.Parent)
	})

	t.Run("assignee me resolves to current user", func(t *testing.T) {
		t.Parallel()

		var capturedUpdate jira4claude.IssueUpdate
		svc := &mock.IssueService{
			UpdateFn: func(ctx context.Context, key string, update jira4claude.IssueUpdate) (*jira4claude.Issue, error) {
				capturedUpdate = update
				return makeIssue(key), nil
			},
		}
		userSvc := &mock.UserService{
			GetMyselfFn: func(ctx context.Context) (*jira4claude.User, error) {
				return &jira4claude.User{AccountID: "myself-123"}, nil
			},
		}

		printer := &mock.Printer{}
		ctx := &main.IssueContext{
			Service:     svc,
			UserService: userSvc,
			Printer:     printer,
			Converter:   mockConverter(),
			Config:      &jira4claude.Config{Project: "TEST", Server: "https://test.atlassian.net"},
		}
		assignee := "me"
		cmd := main.IssueUpdateCmd{Key: "TEST-1", Assignee: &assignee}
		err := cmd.Run(ctx)

		require.NoError(t, err)
		require.NotNil(t, capturedUpdate.Assignee)
		assert.Equal(t, "myself-123", *capturedUpdate.Assignee)
	})

	t.Run("assignee email resolves by email", func(t *testing.T) {
		t.Parallel()

		var capturedUpdate jira4claude.IssueUpdate
		svc := &mock.IssueService{
			UpdateFn: func(ctx context.Context, key string, update jira4claude.IssueUpdate) (*jira4claude.Issue, error) {
				capturedUpdate = update
				return makeIssue(key), nil
			},
		}
		userSvc := &mock.UserService{
			FindUsersFn: func(ctx context.Context, query string) ([]*jira4claude.User, error) {
				return []*jira4claude.User{{AccountID: "user-456"}}, nil
			},
		}

		printer := &mock.Printer{}
		ctx := &main.IssueContext{
			Service:     svc,
			UserService: userSvc,
			Printer:     printer,
			Converter:   mockConverter(),
			Config:      &jira4claude.Config{Project: "TEST", Server: "https://test.atlassian.net"},
		}
		assignee := "user@example.com"
		cmd := main.IssueUpdateCmd{Key: "TEST-1", Assignee: &assignee}
		err := cmd.Run(ctx)

		require.NoError(t, err)
		require.NotNil(t, capturedUpdate.Assignee)
		assert.Equal(t, "user-456", *capturedUpdate.Assignee)
	})

	t.Run("assignee account ID passes through", func(t *testing.T) {
		t.Parallel()

		var capturedUpdate jira4claude.IssueUpdate
		svc := &mock.IssueService{
			UpdateFn: func(ctx context.Context, key string, update jira4claude.IssueUpdate) (*jira4claude.Issue, error) {
				capturedUpdate = update
				return makeIssue(key), nil
			},
		}

		printer := &mock.Printer{}
		ctx := &main.IssueContext{
			Service:   svc,
			Printer:   printer,
			Converter: mockConverter(),
			Config:    &jira4claude.Config{Project: "TEST", Server: "https://test.atlassian.net"},
		}
		assignee := "abc123"
		cmd := main.IssueUpdateCmd{Key: "TEST-1", Assignee: &assignee}
		err := cmd.Run(ctx)

		require.NoError(t, err)
		require.NotNil(t, capturedUpdate.Assignee)
		assert.Equal(t, "abc123", *capturedUpdate.Assignee)
	})

	t.Run("assignee empty string unassigns", func(t *testing.T) {
		t.Parallel()

		var capturedUpdate jira4claude.IssueUpdate
		svc := &mock.IssueService{
			UpdateFn: func(ctx context.Context, key string, update jira4claude.IssueUpdate) (*jira4claude.Issue, error) {
				capturedUpdate = update
				return makeIssue(key), nil
			},
		}

		printer := &mock.Printer{}
		ctx := &main.IssueContext{
			Service:   svc,
			Printer:   printer,
			Converter: mockConverter(),
			Config:    &jira4claude.Config{Project: "TEST", Server: "https://test.atlassian.net"},
		}
		assignee := ""
		cmd := main.IssueUpdateCmd{Key: "TEST-1", Assignee: &assignee}
		err := cmd.Run(ctx)

		require.NoError(t, err)
		require.NotNil(t, capturedUpdate.Assignee)
		assert.Empty(t, *capturedUpdate.Assignee)
	})

	t.Run("assignee resolve failure returns error", func(t *testing.T) {
		t.Parallel()

		svc := &mock.IssueService{}
		userSvc := &mock.UserService{
			GetMyselfFn: func(ctx context.Context) (*jira4claude.User, error) {
				return nil, &jira4claude.Error{Code: jira4claude.EInternal, Message: "auth failed"}
			},
		}

		printer := &mock.Printer{}
		ctx := &main.IssueContext{
			Service:     svc,
			UserService: userSvc,
			Printer:     printer,
			Converter:   mockConverter(),
			Config:      &jira4claude.Config{Project: "TEST", Server: "https://test.atlassian.net"},
		}
		assignee := "me"
		cmd := main.IssueUpdateCmd{Key: "TEST-1", Assignee: &assignee}
		err := cmd.Run(ctx)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "auth failed")
	})
}

// IssueCommentCmd tests

func TestIssueCommentCmd(t *testing.T) {
	t.Parallel()

	t.Run("always converts body as GFM", func(t *testing.T) {
		t.Parallel()

		var capturedBody *jira4claude.ADFNode
		svc := &mock.IssueService{
			AddCommentFn: func(ctx context.Context, key string, body *jira4claude.ADFNode) (*jira4claude.Comment, error) {
				capturedBody = body
				return &jira4claude.Comment{
					ID:      "12345",
					Body:    body,
					Author:  &jira4claude.User{DisplayName: "Test User"},
					Created: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
				}, nil
			},
		}

		printer := &mock.Printer{}
		ctx := &main.IssueContext{
			Service:   svc,
			Printer:   printer,
			Converter: mockConverter(),
			Config:    &jira4claude.Config{Project: "TEST", Server: "https://test.atlassian.net"},
		}
		cmd := main.IssueCommentCmd{Key: "TEST-1", Body: "**bold** and *italic*"}
		err := cmd.Run(ctx)

		require.NoError(t, err)
		// Body should be ADF
		assert.Equal(t, "doc", capturedBody.Type)
	})

	t.Run("plain text input is valid GFM", func(t *testing.T) {
		t.Parallel()

		var capturedBody *jira4claude.ADFNode
		svc := &mock.IssueService{
			AddCommentFn: func(ctx context.Context, key string, body *jira4claude.ADFNode) (*jira4claude.Comment, error) {
				capturedBody = body
				return &jira4claude.Comment{
					ID:      "12345",
					Body:    body,
					Author:  &jira4claude.User{DisplayName: "Test User"},
					Created: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
				}, nil
			},
		}

		printer := &mock.Printer{}
		ctx := &main.IssueContext{
			Service:   svc,
			Printer:   printer,
			Converter: mockConverter(),
			Config:    &jira4claude.Config{Project: "TEST", Server: "https://test.atlassian.net"},
		}
		cmd := main.IssueCommentCmd{Key: "TEST-1", Body: "plain text without formatting"}
		err := cmd.Run(ctx)

		require.NoError(t, err)
		// Plain text is valid GFM and should be converted to ADF
		assert.Equal(t, "doc", capturedBody.Type)
	})
}

// IssueDeleteCmd tests

func TestIssueDeleteCmd(t *testing.T) {
	t.Parallel()

	t.Run("calls Delete with key and prints success", func(t *testing.T) {
		t.Parallel()

		var capturedKey string
		var capturedDeleteSubtasks bool
		svc := &mock.IssueService{
			DeleteFn: func(ctx context.Context, key string, deleteSubtasks bool) error {
				capturedKey = key
				capturedDeleteSubtasks = deleteSubtasks
				return nil
			},
		}

		printer := &mock.Printer{}
		ctx := &main.IssueContext{
			Service:   svc,
			Printer:   printer,
			Converter: mockConverter(),
			Config:    &jira4claude.Config{Project: "TEST", Server: "https://test.atlassian.net"},
		}
		cmd := main.IssueDeleteCmd{Key: "TEST-1"}
		err := cmd.Run(ctx)

		require.NoError(t, err)
		assert.Equal(t, "TEST-1", capturedKey)
		assert.False(t, capturedDeleteSubtasks)
		require.Len(t, printer.SuccessCalls, 1)
		// Key goes in the message, not as a variadic key arg, because the
		// markdown/JSON printers turn variadic keys into /browse/<key> URLs
		// — which would 404 for an issue we just deleted.
		assert.Contains(t, printer.SuccessCalls[0].Msg, "TEST-1")
		assert.Empty(t, printer.SuccessCalls[0].Keys)
	})

	t.Run("forwards --delete-subtasks=true to service", func(t *testing.T) {
		t.Parallel()

		var capturedDeleteSubtasks bool
		svc := &mock.IssueService{
			DeleteFn: func(ctx context.Context, key string, deleteSubtasks bool) error {
				capturedDeleteSubtasks = deleteSubtasks
				return nil
			},
		}

		printer := &mock.Printer{}
		ctx := &main.IssueContext{
			Service:   svc,
			Printer:   printer,
			Converter: mockConverter(),
			Config:    &jira4claude.Config{Project: "TEST", Server: "https://test.atlassian.net"},
		}
		cmd := main.IssueDeleteCmd{Key: "EPIC-1", DeleteSubtasks: true}
		err := cmd.Run(ctx)

		require.NoError(t, err)
		assert.True(t, capturedDeleteSubtasks)
	})

	t.Run("returns service error", func(t *testing.T) {
		t.Parallel()

		svc := &mock.IssueService{
			DeleteFn: func(ctx context.Context, key string, deleteSubtasks bool) error {
				return errors.New("not found")
			},
		}

		printer := &mock.Printer{}
		ctx := &main.IssueContext{
			Service:   svc,
			Printer:   printer,
			Converter: mockConverter(),
			Config:    &jira4claude.Config{Project: "TEST", Server: "https://test.atlassian.net"},
		}
		cmd := main.IssueDeleteCmd{Key: "TEST-99"}
		err := cmd.Run(ctx)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
		assert.Empty(t, printer.SuccessCalls)
	})
}

// IssueDeleteCommentCmd tests

func TestIssueDeleteCommentCmd(t *testing.T) {
	t.Parallel()

	t.Run("calls DeleteComment with key and comment id", func(t *testing.T) {
		t.Parallel()

		var capturedKey, capturedID string
		svc := &mock.IssueService{
			DeleteCommentFn: func(ctx context.Context, key, commentID string) error {
				capturedKey = key
				capturedID = commentID
				return nil
			},
		}

		printer := &mock.Printer{}
		ctx := &main.IssueContext{
			Service:   svc,
			Printer:   printer,
			Converter: mockConverter(),
			Config:    &jira4claude.Config{Project: "TEST", Server: "https://test.atlassian.net"},
		}
		cmd := main.IssueDeleteCommentCmd{Key: "TEST-1", CommentID: "10001"}
		err := cmd.Run(ctx)

		require.NoError(t, err)
		assert.Equal(t, "TEST-1", capturedKey)
		assert.Equal(t, "10001", capturedID)
	})

	t.Run("returns service error", func(t *testing.T) {
		t.Parallel()

		svc := &mock.IssueService{
			DeleteCommentFn: func(ctx context.Context, key, commentID string) error {
				return errors.New("not found")
			},
		}

		printer := &mock.Printer{}
		ctx := &main.IssueContext{
			Service:   svc,
			Printer:   printer,
			Converter: mockConverter(),
			Config:    &jira4claude.Config{Project: "TEST", Server: "https://test.atlassian.net"},
		}
		cmd := main.IssueDeleteCommentCmd{Key: "TEST-1", CommentID: "99999"}
		err := cmd.Run(ctx)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

// IssueReadyCmd tests

func TestIssueReadyCmd(t *testing.T) {
	t.Parallel()

	t.Run("uses project from config when not specified", func(t *testing.T) {
		t.Parallel()

		var capturedFilter jira4claude.IssueFilter
		svc := &mock.IssueService{
			ListFn: func(ctx context.Context, filter jira4claude.IssueFilter) ([]*jira4claude.Issue, error) {
				capturedFilter = filter
				return []*jira4claude.Issue{}, nil
			},
		}

		printer := &mock.Printer{}
		ctx := &main.IssueContext{
			Service:   svc,
			Printer:   printer,
			Converter: mockConverter(),
			Config:    &jira4claude.Config{Project: "TEST", Server: "https://test.atlassian.net"},
		}
		cmd := main.IssueReadyCmd{} // No project specified
		err := cmd.Run(ctx)

		require.NoError(t, err)
		// Project should be set from config
		assert.Equal(t, "TEST", capturedFilter.Project)
	})

	t.Run("uses explicit project when specified", func(t *testing.T) {
		t.Parallel()

		var capturedFilter jira4claude.IssueFilter
		svc := &mock.IssueService{
			ListFn: func(ctx context.Context, filter jira4claude.IssueFilter) ([]*jira4claude.Issue, error) {
				capturedFilter = filter
				return []*jira4claude.Issue{}, nil
			},
		}

		printer := &mock.Printer{}
		ctx := &main.IssueContext{
			Service:   svc,
			Printer:   printer,
			Converter: mockConverter(),
			Config:    &jira4claude.Config{Project: "TEST", Server: "https://test.atlassian.net"},
		}
		cmd := main.IssueReadyCmd{Project: "CUSTOM"}
		err := cmd.Run(ctx)

		require.NoError(t, err)
		// Project should be set from command flag, not config
		assert.Equal(t, "CUSTOM", capturedFilter.Project)
	})

	t.Run("passes limit parameter to filter", func(t *testing.T) {
		t.Parallel()

		var capturedFilter jira4claude.IssueFilter
		svc := &mock.IssueService{
			ListFn: func(ctx context.Context, filter jira4claude.IssueFilter) ([]*jira4claude.Issue, error) {
				capturedFilter = filter
				return []*jira4claude.Issue{}, nil
			},
		}

		printer := &mock.Printer{}
		ctx := &main.IssueContext{
			Service:   svc,
			Printer:   printer,
			Converter: mockConverter(),
			Config:    &jira4claude.Config{Project: "TEST", Server: "https://test.atlassian.net"},
		}
		cmd := main.IssueReadyCmd{Limit: 25}
		err := cmd.Run(ctx)

		require.NoError(t, err)
		assert.Equal(t, 25, capturedFilter.Limit)
	})

	t.Run("filters out issues that are not ready", func(t *testing.T) {
		t.Parallel()

		svc := &mock.IssueService{
			ListFn: func(ctx context.Context, filter jira4claude.IssueFilter) ([]*jira4claude.Issue, error) {
				return []*jira4claude.Issue{
					{
						Key:    "TEST-1",
						Status: "To Do",
						Links:  nil, // No blockers, ready
					},
					{
						Key:    "TEST-2",
						Status: "To Do",
						Links: []*jira4claude.IssueLink{
							{
								Type: jira4claude.IssueLinkType{
									Name:   "Blocks",
									Inward: "is blocked by",
								},
								InwardIssue: &jira4claude.LinkedIssue{
									Key:    "TEST-3",
									Status: "In Progress", // Open blocker, not ready
								},
							},
						},
					},
					{
						Key:    "TEST-4",
						Status: "To Do",
						Links: []*jira4claude.IssueLink{
							{
								Type: jira4claude.IssueLinkType{
									Name:   "Blocks",
									Inward: "is blocked by",
								},
								InwardIssue: &jira4claude.LinkedIssue{
									Key:    "TEST-5",
									Status: "Done", // Blocker done, ready
								},
							},
						},
					},
				}, nil
			},
		}

		printer := &mock.Printer{}
		ctx := &main.IssueContext{
			Service:   svc,
			Printer:   printer,
			Converter: mockConverter(),
			Config:    &jira4claude.Config{Project: "TEST", Server: "https://test.atlassian.net"},
		}
		cmd := main.IssueReadyCmd{}
		err := cmd.Run(ctx)

		require.NoError(t, err)
		// Should have called Issues with 2 ready issues (TEST-1 and TEST-4)
		require.Len(t, printer.IssuesCalls, 1)
		views := printer.IssuesCalls[0]
		require.Len(t, views, 2)
		assert.Equal(t, "TEST-1", views[0].Key)
		assert.Equal(t, "TEST-4", views[1].Key)
	})

	t.Run("handles empty result", func(t *testing.T) {
		t.Parallel()

		svc := &mock.IssueService{
			ListFn: func(ctx context.Context, filter jira4claude.IssueFilter) ([]*jira4claude.Issue, error) {
				return []*jira4claude.Issue{}, nil
			},
		}

		printer := &mock.Printer{}
		ctx := &main.IssueContext{
			Service:   svc,
			Printer:   printer,
			Converter: mockConverter(),
			Config:    &jira4claude.Config{Project: "TEST", Server: "https://test.atlassian.net"},
		}
		cmd := main.IssueReadyCmd{}
		err := cmd.Run(ctx)

		require.NoError(t, err)
		// Should have called Issues with empty slice
		require.Len(t, printer.IssuesCalls, 1)
		assert.Empty(t, printer.IssuesCalls[0])
	})

	t.Run("handles all issues filtered out", func(t *testing.T) {
		t.Parallel()

		svc := &mock.IssueService{
			ListFn: func(ctx context.Context, filter jira4claude.IssueFilter) ([]*jira4claude.Issue, error) {
				return []*jira4claude.Issue{
					{
						Key:    "TEST-1",
						Status: "To Do",
						Links: []*jira4claude.IssueLink{
							{
								Type: jira4claude.IssueLinkType{
									Name:   "Blocks",
									Inward: "is blocked by",
								},
								InwardIssue: &jira4claude.LinkedIssue{
									Key:    "TEST-2",
									Status: "In Progress", // Open blocker
								},
							},
						},
					},
				}, nil
			},
		}

		printer := &mock.Printer{}
		ctx := &main.IssueContext{
			Service:   svc,
			Printer:   printer,
			Converter: mockConverter(),
			Config:    &jira4claude.Config{Project: "TEST", Server: "https://test.atlassian.net"},
		}
		cmd := main.IssueReadyCmd{}
		err := cmd.Run(ctx)

		require.NoError(t, err)
		// Should have called Issues with empty slice (all filtered out)
		require.Len(t, printer.IssuesCalls, 1)
		assert.Empty(t, printer.IssuesCalls[0])
	})

	t.Run("propagates service errors", func(t *testing.T) {
		t.Parallel()

		expectedErr := &jira4claude.Error{Code: jira4claude.EInternal, Message: "connection failed"}
		svc := &mock.IssueService{
			ListFn: func(ctx context.Context, filter jira4claude.IssueFilter) ([]*jira4claude.Issue, error) {
				return nil, expectedErr
			},
		}

		printer := &mock.Printer{}
		ctx := &main.IssueContext{
			Service:   svc,
			Printer:   printer,
			Converter: mockConverter(),
			Config:    &jira4claude.Config{Project: "TEST", Server: "https://test.atlassian.net"},
		}
		cmd := main.IssueReadyCmd{}
		err := cmd.Run(ctx)

		require.Error(t, err)
		assert.Equal(t, expectedErr, err)
	})

	t.Run("passes parent flag to filter", func(t *testing.T) {
		t.Parallel()

		var capturedFilter jira4claude.IssueFilter
		svc := &mock.IssueService{
			ListFn: func(ctx context.Context, filter jira4claude.IssueFilter) ([]*jira4claude.Issue, error) {
				capturedFilter = filter
				return []*jira4claude.Issue{}, nil
			},
		}

		printer := &mock.Printer{}
		ctx := &main.IssueContext{
			Service:   svc,
			Printer:   printer,
			Converter: mockConverter(),
			Config:    &jira4claude.Config{Project: "TEST", Server: "https://test.atlassian.net"},
		}
		cmd := main.IssueReadyCmd{Parent: "TEST-1"}
		err := cmd.Run(ctx)

		require.NoError(t, err)
		assert.Equal(t, "TEST-1", capturedFilter.Parent)
	})

	t.Run("uses filter fields instead of raw JQL", func(t *testing.T) {
		t.Parallel()

		var capturedFilter jira4claude.IssueFilter
		svc := &mock.IssueService{
			ListFn: func(ctx context.Context, filter jira4claude.IssueFilter) ([]*jira4claude.Issue, error) {
				capturedFilter = filter
				return []*jira4claude.Issue{}, nil
			},
		}

		printer := &mock.Printer{}
		ctx := &main.IssueContext{
			Service:   svc,
			Printer:   printer,
			Converter: mockConverter(),
			Config:    &jira4claude.Config{Project: "TEST", Server: "https://test.atlassian.net"},
		}
		cmd := main.IssueReadyCmd{Project: "CUSTOM", Parent: "CUSTOM-1", Limit: 25}
		err := cmd.Run(ctx)

		require.NoError(t, err)
		// Should use filter fields, not raw JQL
		assert.Empty(t, capturedFilter.JQL)
		assert.Equal(t, "CUSTOM", capturedFilter.Project)
		assert.Equal(t, "CUSTOM-1", capturedFilter.Parent)
		assert.Equal(t, "Done", capturedFilter.ExcludeStatus)
		assert.Equal(t, "created DESC", capturedFilter.OrderBy)
		assert.Equal(t, 25, capturedFilter.Limit)
	})
}

// IssueViewCmd tests

func TestIssueViewCmd(t *testing.T) {
	t.Parallel()

	t.Run("displays ADF description", func(t *testing.T) {
		t.Parallel()

		svc := &mock.IssueService{
			GetFn: func(ctx context.Context, key string) (*jira4claude.Issue, error) {
				return &jira4claude.Issue{
					Key:     "TEST-1",
					Summary: "Test issue",
					Status:  "To Do",
					Type:    "Task",
					Description: &jira4claude.ADFNode{
						Type:    "doc",
						Version: 1,
						Content: []jira4claude.ADFNode{
							{
								Type:  "heading",
								Attrs: json.RawMessage(`{"level":1}`),
								Content: []jira4claude.ADFNode{
									{
										Type: "text",
										Text: "Hello",
									},
								},
							},
						},
					},
				}, nil
			},
		}

		printer := &mock.Printer{}
		ctx := &main.IssueContext{
			Service:   svc,
			Printer:   printer,
			Converter: mockConverter(),
			Config:    &jira4claude.Config{Project: "TEST", Server: "https://test.atlassian.net"},
		}
		cmd := main.IssueViewCmd{Key: "TEST-1"}
		err := cmd.Run(ctx)

		require.NoError(t, err)
		require.Len(t, printer.IssueCalls, 1)
		// Description should contain "Hello" after conversion
		assert.Contains(t, printer.IssueCalls[0].Description, "Hello")
	})

	t.Run("handles nil description", func(t *testing.T) {
		t.Parallel()

		svc := &mock.IssueService{
			GetFn: func(ctx context.Context, key string) (*jira4claude.Issue, error) {
				return &jira4claude.Issue{
					Key:         "TEST-1",
					Summary:     "Test issue",
					Status:      "To Do",
					Type:        "Task",
					Description: nil,
				}, nil
			},
		}

		printer := &mock.Printer{}
		ctx := &main.IssueContext{
			Service:   svc,
			Printer:   printer,
			Converter: mockConverter(),
			Config:    &jira4claude.Config{Project: "TEST", Server: "https://test.atlassian.net"},
		}
		cmd := main.IssueViewCmd{Key: "TEST-1"}
		err := cmd.Run(ctx)

		require.NoError(t, err)
		require.Len(t, printer.IssueCalls, 1)
		// Description should be empty when nil
		assert.Empty(t, printer.IssueCalls[0].Description)
	})

	t.Run("displays comment bodies as ADF", func(t *testing.T) {
		t.Parallel()

		svc := &mock.IssueService{
			GetFn: func(ctx context.Context, key string) (*jira4claude.Issue, error) {
				return &jira4claude.Issue{
					Key:     "TEST-1",
					Summary: "Test issue",
					Status:  "To Do",
					Type:    "Task",
					Comments: []*jira4claude.Comment{
						{
							ID:     "10001",
							Author: &jira4claude.User{DisplayName: "John Doe"},
							Body: &jira4claude.ADFNode{
								Type:    "doc",
								Version: 1,
								Content: []jira4claude.ADFNode{
									{
										Type: "paragraph",
										Content: []jira4claude.ADFNode{
											{
												Type: "text",
												Text: "Comment with ",
											},
											{
												Type: "text",
												Text: "bold",
												Marks: []jira4claude.ADFMark{
													{Type: "strong"},
												},
											},
										},
									},
								},
							},
							Created: time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
						},
					},
				}, nil
			},
		}

		printer := &mock.Printer{}
		ctx := &main.IssueContext{
			Service:   svc,
			Printer:   printer,
			Converter: mockConverter(),
			Config:    &jira4claude.Config{Project: "TEST", Server: "https://test.atlassian.net"},
		}
		cmd := main.IssueViewCmd{Key: "TEST-1"}
		err := cmd.Run(ctx)

		require.NoError(t, err)
		require.Len(t, printer.IssueCalls, 1)
		require.Len(t, printer.IssueCalls[0].Comments, 1)
		// Comment body should contain the text after conversion
		assert.Contains(t, printer.IssueCalls[0].Comments[0].Body, "Comment with ")
	})
}

// IssueListCmd tests

func TestIssueListCmd(t *testing.T) {
	t.Parallel()

	t.Run("uses config project when project flag not specified", func(t *testing.T) {
		t.Parallel()

		var capturedFilter jira4claude.IssueFilter
		svc := &mock.IssueService{
			ListFn: func(ctx context.Context, filter jira4claude.IssueFilter) ([]*jira4claude.Issue, error) {
				capturedFilter = filter
				return []*jira4claude.Issue{}, nil
			},
		}

		printer := &mock.Printer{}
		ctx := &main.IssueContext{
			Service:   svc,
			Printer:   printer,
			Converter: mockConverter(),
			Config:    &jira4claude.Config{Project: "TEST", Server: "https://test.atlassian.net"},
		}
		cmd := main.IssueListCmd{}
		err := cmd.Run(ctx)

		require.NoError(t, err)
		assert.Equal(t, "TEST", capturedFilter.Project)
	})

	t.Run("uses specified project over config project", func(t *testing.T) {
		t.Parallel()

		var capturedFilter jira4claude.IssueFilter
		svc := &mock.IssueService{
			ListFn: func(ctx context.Context, filter jira4claude.IssueFilter) ([]*jira4claude.Issue, error) {
				capturedFilter = filter
				return []*jira4claude.Issue{}, nil
			},
		}

		printer := &mock.Printer{}
		ctx := &main.IssueContext{
			Service:   svc,
			Printer:   printer,
			Converter: mockConverter(),
			Config:    &jira4claude.Config{Project: "TEST", Server: "https://test.atlassian.net"},
		}
		cmd := main.IssueListCmd{Project: "OVERRIDE"}
		err := cmd.Run(ctx)

		require.NoError(t, err)
		assert.Equal(t, "OVERRIDE", capturedFilter.Project)
	})

	t.Run("does not use config project when JQL is specified", func(t *testing.T) {
		t.Parallel()

		var capturedFilter jira4claude.IssueFilter
		svc := &mock.IssueService{
			ListFn: func(ctx context.Context, filter jira4claude.IssueFilter) ([]*jira4claude.Issue, error) {
				capturedFilter = filter
				return []*jira4claude.Issue{}, nil
			},
		}

		printer := &mock.Printer{}
		ctx := &main.IssueContext{
			Service:   svc,
			Printer:   printer,
			Converter: mockConverter(),
			Config:    &jira4claude.Config{Project: "TEST", Server: "https://test.atlassian.net"},
		}
		cmd := main.IssueListCmd{JQL: "project = CUSTOM"}
		err := cmd.Run(ctx)

		require.NoError(t, err)
		// When JQL is provided, project should not be set from config
		assert.Empty(t, capturedFilter.Project)
		assert.Equal(t, "project = CUSTOM", capturedFilter.JQL)
	})

	t.Run("passes all filter flags to service", func(t *testing.T) {
		t.Parallel()

		var capturedFilter jira4claude.IssueFilter
		svc := &mock.IssueService{
			ListFn: func(ctx context.Context, filter jira4claude.IssueFilter) ([]*jira4claude.Issue, error) {
				capturedFilter = filter
				return []*jira4claude.Issue{}, nil
			},
		}

		printer := &mock.Printer{}
		ctx := &main.IssueContext{
			Service:   svc,
			Printer:   printer,
			Converter: mockConverter(),
			Config:    &jira4claude.Config{Project: "TEST", Server: "https://test.atlassian.net"},
		}
		cmd := main.IssueListCmd{
			Project:  "MYPROJ",
			Status:   "In Progress",
			Assignee: "john.doe",
			Parent:   "MYPROJ-1",
			Labels:   []string{"bug", "urgent"},
			Limit:    25,
		}
		err := cmd.Run(ctx)

		require.NoError(t, err)
		assert.Equal(t, "MYPROJ", capturedFilter.Project)
		assert.Equal(t, "In Progress", capturedFilter.Status)
		assert.Equal(t, "john.doe", capturedFilter.Assignee)
		assert.Equal(t, "MYPROJ-1", capturedFilter.Parent)
		assert.Equal(t, []string{"bug", "urgent"}, capturedFilter.Labels)
		assert.Equal(t, 25, capturedFilter.Limit)
	})

	t.Run("passes JQL to service", func(t *testing.T) {
		t.Parallel()

		var capturedFilter jira4claude.IssueFilter
		svc := &mock.IssueService{
			ListFn: func(ctx context.Context, filter jira4claude.IssueFilter) ([]*jira4claude.Issue, error) {
				capturedFilter = filter
				return []*jira4claude.Issue{}, nil
			},
		}

		printer := &mock.Printer{}
		ctx := &main.IssueContext{
			Service:   svc,
			Printer:   printer,
			Converter: mockConverter(),
			Config:    &jira4claude.Config{Project: "TEST", Server: "https://test.atlassian.net"},
		}
		cmd := main.IssueListCmd{
			JQL:   "assignee = currentUser() ORDER BY created DESC",
			Limit: 10,
		}
		err := cmd.Run(ctx)

		require.NoError(t, err)
		assert.Equal(t, "assignee = currentUser() ORDER BY created DESC", capturedFilter.JQL)
		assert.Equal(t, 10, capturedFilter.Limit)
	})

	t.Run("returns error from service", func(t *testing.T) {
		t.Parallel()

		svc := &mock.IssueService{
			ListFn: func(ctx context.Context, filter jira4claude.IssueFilter) ([]*jira4claude.Issue, error) {
				return nil, &jira4claude.Error{Code: jira4claude.ENotFound, Message: "Project not found"}
			},
		}

		printer := &mock.Printer{}
		ctx := &main.IssueContext{
			Service:   svc,
			Printer:   printer,
			Converter: mockConverter(),
			Config:    &jira4claude.Config{Project: "TEST", Server: "https://test.atlassian.net"},
		}
		cmd := main.IssueListCmd{Project: "NONEXISTENT"}
		err := cmd.Run(ctx)

		require.Error(t, err)
		assert.Equal(t, jira4claude.ENotFound, jira4claude.ErrorCode(err))
	})

	t.Run("prints issues to output", func(t *testing.T) {
		t.Parallel()

		svc := &mock.IssueService{
			ListFn: func(ctx context.Context, filter jira4claude.IssueFilter) ([]*jira4claude.Issue, error) {
				return []*jira4claude.Issue{
					makeIssue("TEST-1"),
					makeIssue("TEST-2"),
				}, nil
			},
		}

		printer := &mock.Printer{}
		ctx := &main.IssueContext{
			Service:   svc,
			Printer:   printer,
			Converter: mockConverter(),
			Config:    &jira4claude.Config{Project: "TEST", Server: "https://test.atlassian.net"},
		}
		cmd := main.IssueListCmd{Project: "TEST"}
		err := cmd.Run(ctx)

		require.NoError(t, err)
		require.Len(t, printer.IssuesCalls, 1)
		views := printer.IssuesCalls[0]
		require.Len(t, views, 2)
		assert.Equal(t, "TEST-1", views[0].Key)
		assert.Equal(t, "TEST-2", views[1].Key)
	})

	t.Run("passes exclude-status flag to filter", func(t *testing.T) {
		t.Parallel()

		var capturedFilter jira4claude.IssueFilter
		svc := &mock.IssueService{
			ListFn: func(ctx context.Context, filter jira4claude.IssueFilter) ([]*jira4claude.Issue, error) {
				capturedFilter = filter
				return []*jira4claude.Issue{}, nil
			},
		}

		printer := &mock.Printer{}
		ctx := &main.IssueContext{
			Service:   svc,
			Printer:   printer,
			Converter: mockConverter(),
			Config:    &jira4claude.Config{Project: "TEST", Server: "https://test.atlassian.net"},
		}
		cmd := main.IssueListCmd{ExcludeStatus: "Done"}
		err := cmd.Run(ctx)

		require.NoError(t, err)
		assert.Equal(t, "Done", capturedFilter.ExcludeStatus)
	})

	t.Run("passes order-by flag to filter", func(t *testing.T) {
		t.Parallel()

		var capturedFilter jira4claude.IssueFilter
		svc := &mock.IssueService{
			ListFn: func(ctx context.Context, filter jira4claude.IssueFilter) ([]*jira4claude.Issue, error) {
				capturedFilter = filter
				return []*jira4claude.Issue{}, nil
			},
		}

		printer := &mock.Printer{}
		ctx := &main.IssueContext{
			Service:   svc,
			Printer:   printer,
			Converter: mockConverter(),
			Config:    &jira4claude.Config{Project: "TEST", Server: "https://test.atlassian.net"},
		}
		cmd := main.IssueListCmd{OrderBy: "created DESC"}
		err := cmd.Run(ctx)

		require.NoError(t, err)
		assert.Equal(t, "created DESC", capturedFilter.OrderBy)
	})

	t.Run("passes all new filter flags to service", func(t *testing.T) {
		t.Parallel()

		var capturedFilter jira4claude.IssueFilter
		svc := &mock.IssueService{
			ListFn: func(ctx context.Context, filter jira4claude.IssueFilter) ([]*jira4claude.Issue, error) {
				capturedFilter = filter
				return []*jira4claude.Issue{}, nil
			},
		}

		printer := &mock.Printer{}
		ctx := &main.IssueContext{
			Service:   svc,
			Printer:   printer,
			Converter: mockConverter(),
			Config:    &jira4claude.Config{Project: "TEST", Server: "https://test.atlassian.net"},
		}
		cmd := main.IssueListCmd{
			Project:       "MYPROJ",
			Parent:        "MYPROJ-1",
			ExcludeStatus: "Done",
			OrderBy:       "created DESC",
			Limit:         25,
		}
		err := cmd.Run(ctx)

		require.NoError(t, err)
		assert.Equal(t, "MYPROJ", capturedFilter.Project)
		assert.Equal(t, "MYPROJ-1", capturedFilter.Parent)
		assert.Equal(t, "Done", capturedFilter.ExcludeStatus)
		assert.Equal(t, "created DESC", capturedFilter.OrderBy)
		assert.Equal(t, 25, capturedFilter.Limit)
	})
}

// IssueAssignCmd tests

func TestIssueAssignCmd(t *testing.T) {
	t.Parallel()

	t.Run("resolves me to authenticated user and assigns", func(t *testing.T) {
		t.Parallel()

		var assignedID string
		svc := &mock.IssueService{
			AssignFn: func(ctx context.Context, key, accountID string) error {
				require.Equal(t, "TEST-1", key)
				assignedID = accountID
				return nil
			},
		}
		userSvc := &mock.UserService{
			GetMyselfFn: func(ctx context.Context) (*jira4claude.User, error) {
				return &jira4claude.User{AccountID: "myself-123"}, nil
			},
		}

		printer := &mock.Printer{}
		ctx := &main.IssueContext{
			Service:     svc,
			UserService: userSvc,
			Printer:     printer,
			Converter:   mockConverter(),
			Config:      &jira4claude.Config{Project: "TEST", Server: "https://test.atlassian.net"},
		}
		cmd := main.IssueAssignCmd{Key: "TEST-1", Assignee: "me"}
		err := cmd.Run(ctx)

		require.NoError(t, err)
		assert.Equal(t, "myself-123", assignedID)
		require.Len(t, printer.SuccessCalls, 1)
		assert.Equal(t, "Assigned:", printer.SuccessCalls[0].Msg)
		assert.Equal(t, []string{"TEST-1"}, printer.SuccessCalls[0].Keys)
	})

	t.Run("resolves email via FindUsers and assigns", func(t *testing.T) {
		t.Parallel()

		var assignedID string
		svc := &mock.IssueService{
			AssignFn: func(ctx context.Context, key, accountID string) error {
				require.Equal(t, "TEST-1", key)
				assignedID = accountID
				return nil
			},
		}
		userSvc := &mock.UserService{
			FindUsersFn: func(ctx context.Context, query string) ([]*jira4claude.User, error) {
				require.Equal(t, "user@example.com", query)
				return []*jira4claude.User{{AccountID: "found-456"}}, nil
			},
		}

		printer := &mock.Printer{}
		ctx := &main.IssueContext{
			Service:     svc,
			UserService: userSvc,
			Printer:     printer,
			Converter:   mockConverter(),
			Config:      &jira4claude.Config{Project: "TEST", Server: "https://test.atlassian.net"},
		}
		cmd := main.IssueAssignCmd{Key: "TEST-1", Assignee: "user@example.com"}
		err := cmd.Run(ctx)

		require.NoError(t, err)
		assert.Equal(t, "found-456", assignedID)
		require.Len(t, printer.SuccessCalls, 1)
		assert.Equal(t, "Assigned:", printer.SuccessCalls[0].Msg)
		assert.Equal(t, []string{"TEST-1"}, printer.SuccessCalls[0].Keys)
	})

	t.Run("passes raw account ID through directly", func(t *testing.T) {
		t.Parallel()

		var assignedID string
		svc := &mock.IssueService{
			AssignFn: func(ctx context.Context, key, accountID string) error {
				require.Equal(t, "TEST-1", key)
				assignedID = accountID
				return nil
			},
		}

		printer := &mock.Printer{}
		ctx := &main.IssueContext{
			Service:     svc,
			UserService: &mock.UserService{},
			Printer:     printer,
			Converter:   mockConverter(),
			Config:      &jira4claude.Config{Project: "TEST", Server: "https://test.atlassian.net"},
		}
		cmd := main.IssueAssignCmd{Key: "TEST-1", Assignee: "abc123"}
		err := cmd.Run(ctx)

		require.NoError(t, err)
		assert.Equal(t, "abc123", assignedID)
		require.Len(t, printer.SuccessCalls, 1)
		assert.Equal(t, "Assigned:", printer.SuccessCalls[0].Msg)
		assert.Equal(t, []string{"TEST-1"}, printer.SuccessCalls[0].Keys)
	})

	t.Run("unassigns when assignee flag is omitted", func(t *testing.T) {
		t.Parallel()

		var assignedID string
		svc := &mock.IssueService{
			AssignFn: func(ctx context.Context, key, accountID string) error {
				require.Equal(t, "TEST-1", key)
				assignedID = accountID
				return nil
			},
		}

		printer := &mock.Printer{}
		ctx := &main.IssueContext{
			Service:     svc,
			UserService: &mock.UserService{},
			Printer:     printer,
			Converter:   mockConverter(),
			Config:      &jira4claude.Config{Project: "TEST", Server: "https://test.atlassian.net"},
		}
		cmd := main.IssueAssignCmd{Key: "TEST-1", Assignee: ""}
		err := cmd.Run(ctx)

		require.NoError(t, err)
		assert.Empty(t, assignedID)
		require.Len(t, printer.SuccessCalls, 1)
		assert.Equal(t, "Unassigned:", printer.SuccessCalls[0].Msg)
		assert.Equal(t, []string{"TEST-1"}, printer.SuccessCalls[0].Keys)
	})

	t.Run("returns error when email resolves to no user", func(t *testing.T) {
		t.Parallel()

		svc := &mock.IssueService{}
		userSvc := &mock.UserService{
			FindUsersFn: func(ctx context.Context, query string) ([]*jira4claude.User, error) {
				require.Equal(t, "nobody@example.com", query)
				return []*jira4claude.User{}, nil
			},
		}

		printer := &mock.Printer{}
		ctx := &main.IssueContext{
			Service:     svc,
			UserService: userSvc,
			Printer:     printer,
			Converter:   mockConverter(),
			Config:      &jira4claude.Config{Project: "TEST", Server: "https://test.atlassian.net"},
		}
		cmd := main.IssueAssignCmd{Key: "TEST-1", Assignee: "nobody@example.com"}
		err := cmd.Run(ctx)

		require.Error(t, err)
		assert.Equal(t, jira4claude.ENotFound, jira4claude.ErrorCode(err))
	})

	t.Run("returns error when assign service fails", func(t *testing.T) {
		t.Parallel()

		svc := &mock.IssueService{
			AssignFn: func(ctx context.Context, key, accountID string) error {
				return &jira4claude.Error{Code: jira4claude.ENotFound, Message: "issue not found"}
			},
		}

		printer := &mock.Printer{}
		ctx := &main.IssueContext{
			Service:     svc,
			UserService: &mock.UserService{},
			Printer:     printer,
			Converter:   mockConverter(),
			Config:      &jira4claude.Config{Project: "TEST", Server: "https://test.atlassian.net"},
		}
		cmd := main.IssueAssignCmd{Key: "NOTFOUND-1", Assignee: "abc123"}
		err := cmd.Run(ctx)

		require.Error(t, err)
		assert.Equal(t, jira4claude.ENotFound, jira4claude.ErrorCode(err))
	})
}

// IssueCreateCmd --field-json plumbing tests

func TestIssueCreateCmd_FieldJSON(t *testing.T) {
	t.Parallel()

	t.Run("populates Issue.CustomFields and reaches service", func(t *testing.T) {
		t.Parallel()

		var capturedIssue *jira4claude.Issue
		svc := &mock.IssueService{
			CreateFn: func(ctx context.Context, issue *jira4claude.Issue) (*jira4claude.Issue, error) {
				capturedIssue = issue
				return &jira4claude.Issue{Key: "TEST-1"}, nil
			},
		}

		printer := &mock.Printer{}
		ctx := &main.IssueContext{
			Service:   svc,
			Printer:   printer,
			Converter: mockConverter(),
			Config:    &jira4claude.Config{Project: "TEST", Server: "https://test.atlassian.net"},
		}
		cmd := main.IssueCreateCmd{
			Summary: "Test issue",
			FieldJSON: []string{
				`customfield_10801={"value":"High"}`,
				`customfield_10838=[{"value":"Integrations"}]`,
			},
		}
		err := cmd.Run(ctx)

		require.NoError(t, err)
		require.NotNil(t, capturedIssue)
		require.Len(t, capturedIssue.CustomFields, 2)
		assert.JSONEq(t, `{"value":"High"}`, string(capturedIssue.CustomFields["customfield_10801"]))
		assert.JSONEq(t, `[{"value":"Integrations"}]`, string(capturedIssue.CustomFields["customfield_10838"]))
	})

	t.Run("nil CustomFields when no --field-json provided", func(t *testing.T) {
		t.Parallel()

		var capturedIssue *jira4claude.Issue
		svc := &mock.IssueService{
			CreateFn: func(ctx context.Context, issue *jira4claude.Issue) (*jira4claude.Issue, error) {
				capturedIssue = issue
				return &jira4claude.Issue{Key: "TEST-1"}, nil
			},
		}

		printer := &mock.Printer{}
		ctx := &main.IssueContext{
			Service:   svc,
			Printer:   printer,
			Converter: mockConverter(),
			Config:    &jira4claude.Config{Project: "TEST", Server: "https://test.atlassian.net"},
		}
		cmd := main.IssueCreateCmd{Summary: "Test issue"}
		err := cmd.Run(ctx)

		require.NoError(t, err)
		require.NotNil(t, capturedIssue)
		assert.Nil(t, capturedIssue.CustomFields)
	})

	t.Run("invalid --field-json returns error without calling service", func(t *testing.T) {
		t.Parallel()

		createCalled := false
		svc := &mock.IssueService{
			CreateFn: func(ctx context.Context, issue *jira4claude.Issue) (*jira4claude.Issue, error) {
				createCalled = true
				return &jira4claude.Issue{Key: "TEST-1"}, nil
			},
		}

		printer := &mock.Printer{}
		ctx := &main.IssueContext{
			Service:   svc,
			Printer:   printer,
			Converter: mockConverter(),
			Config:    &jira4claude.Config{Project: "TEST", Server: "https://test.atlassian.net"},
		}
		cmd := main.IssueCreateCmd{
			Summary:   "Test issue",
			FieldJSON: []string{`customfield_10801=not-json`},
		}
		err := cmd.Run(ctx)

		require.Error(t, err)
		assert.Equal(t, jira4claude.EValidation, jira4claude.ErrorCode(err))
		assert.False(t, createCalled, "service should not be called when --field-json is invalid")
		assert.Empty(t, printer.SuccessCalls)
	})
}

// IssueUpdateCmd --field-json plumbing tests

func TestIssueUpdateCmd_FieldJSON(t *testing.T) {
	t.Parallel()

	t.Run("populates IssueUpdate.CustomFields and reaches service", func(t *testing.T) {
		t.Parallel()

		var capturedUpdate jira4claude.IssueUpdate
		svc := &mock.IssueService{
			UpdateFn: func(ctx context.Context, key string, update jira4claude.IssueUpdate) (*jira4claude.Issue, error) {
				capturedUpdate = update
				return makeIssue(key), nil
			},
		}

		printer := &mock.Printer{}
		ctx := &main.IssueContext{
			Service:   svc,
			Printer:   printer,
			Converter: mockConverter(),
			Config:    &jira4claude.Config{Project: "TEST", Server: "https://test.atlassian.net"},
		}
		cmd := main.IssueUpdateCmd{
			Key: "TEST-1",
			FieldJSON: []string{
				`customfield_10801={"value":"Low"}`,
			},
		}
		err := cmd.Run(ctx)

		require.NoError(t, err)
		require.Len(t, capturedUpdate.CustomFields, 1)
		assert.JSONEq(t, `{"value":"Low"}`, string(capturedUpdate.CustomFields["customfield_10801"]))
	})

	t.Run("nil CustomFields when no --field-json provided", func(t *testing.T) {
		t.Parallel()

		var capturedUpdate jira4claude.IssueUpdate
		svc := &mock.IssueService{
			UpdateFn: func(ctx context.Context, key string, update jira4claude.IssueUpdate) (*jira4claude.Issue, error) {
				capturedUpdate = update
				return makeIssue(key), nil
			},
		}

		printer := &mock.Printer{}
		ctx := &main.IssueContext{
			Service:   svc,
			Printer:   printer,
			Converter: mockConverter(),
			Config:    &jira4claude.Config{Project: "TEST", Server: "https://test.atlassian.net"},
		}
		summary := "Updated"
		cmd := main.IssueUpdateCmd{Key: "TEST-1", Summary: &summary}
		err := cmd.Run(ctx)

		require.NoError(t, err)
		assert.Nil(t, capturedUpdate.CustomFields)
	})

	t.Run("invalid --field-json returns error without calling service", func(t *testing.T) {
		t.Parallel()

		updateCalled := false
		svc := &mock.IssueService{
			UpdateFn: func(ctx context.Context, key string, update jira4claude.IssueUpdate) (*jira4claude.Issue, error) {
				updateCalled = true
				return makeIssue(key), nil
			},
		}

		printer := &mock.Printer{}
		ctx := &main.IssueContext{
			Service:   svc,
			Printer:   printer,
			Converter: mockConverter(),
			Config:    &jira4claude.Config{Project: "TEST", Server: "https://test.atlassian.net"},
		}
		cmd := main.IssueUpdateCmd{
			Key:       "TEST-1",
			FieldJSON: []string{`bad`},
		}
		err := cmd.Run(ctx)

		require.Error(t, err)
		assert.Equal(t, jira4claude.EValidation, jira4claude.ErrorCode(err))
		assert.False(t, updateCalled, "service should not be called when --field-json is invalid")
		assert.Empty(t, printer.SuccessCalls)
	})
}

// IssueFieldsCmd tests

func TestIssueFieldsCmd(t *testing.T) {
	t.Parallel()

	t.Run("create mode: --project + --type calls GetCreateFields with those args", func(t *testing.T) {
		t.Parallel()

		var capturedProject, capturedType string
		svc := &mock.IssueService{
			GetCreateFieldsFn: func(ctx context.Context, projectKey, issueType string) ([]*jira4claude.IssueField, error) {
				capturedProject = projectKey
				capturedType = issueType
				return []*jira4claude.IssueField{
					{ID: "summary", Name: "Summary", Required: true, SchemaType: "string"},
				}, nil
			},
		}

		printer := &mock.Printer{}
		ctx := &main.IssueContext{
			Service:   svc,
			Printer:   printer,
			Converter: mockConverter(),
			Config:    &jira4claude.Config{Project: "FALLBACK"},
		}
		cmd := main.IssueFieldsCmd{Project: "PROJ", Type: "Bug"}
		err := cmd.Run(ctx)

		require.NoError(t, err)
		assert.Equal(t, "PROJ", capturedProject)
		assert.Equal(t, "Bug", capturedType)
		require.Len(t, printer.FieldsCalls, 1)
		assert.Equal(t, "PROJ / Bug", printer.FieldsCalls[0].Source)
	})

	t.Run("edit mode: --key calls GetEditFields and sets edit source", func(t *testing.T) {
		t.Parallel()

		var capturedKey string
		svc := &mock.IssueService{
			GetEditFieldsFn: func(ctx context.Context, key string) ([]*jira4claude.IssueField, error) {
				capturedKey = key
				return []*jira4claude.IssueField{
					{ID: "summary", Name: "Summary", Required: true, SchemaType: "string"},
				}, nil
			},
		}

		printer := &mock.Printer{}
		ctx := &main.IssueContext{
			Service:   svc,
			Printer:   printer,
			Converter: mockConverter(),
			Config:    &jira4claude.Config{Project: "TEST"},
		}
		cmd := main.IssueFieldsCmd{Key: "PROJ-7"}
		err := cmd.Run(ctx)

		require.NoError(t, err)
		assert.Equal(t, "PROJ-7", capturedKey)
		require.Len(t, printer.FieldsCalls, 1)
		assert.Equal(t, "PROJ-7 (edit)", printer.FieldsCalls[0].Source)
	})

	t.Run("default filter keeps required and customfield_* fields only", func(t *testing.T) {
		t.Parallel()

		svc := &mock.IssueService{
			GetCreateFieldsFn: func(ctx context.Context, projectKey, issueType string) ([]*jira4claude.IssueField, error) {
				return []*jira4claude.IssueField{
					{ID: "summary", Name: "Summary", Required: true, SchemaType: "string"},          // required builtin → keep
					{ID: "description", Name: "Description", Required: false, SchemaType: "string"}, // optional builtin → drop
					{ID: "customfield_10010", Name: "Story Points", Required: false},                // optional custom → keep
					{ID: "customfield_10801", Name: "Urgency", Required: true},                      // required custom → keep
					{ID: "priority", Name: "Priority", Required: false},                             // optional builtin → drop
				}, nil
			},
		}

		printer := &mock.Printer{}
		ctx := &main.IssueContext{
			Service:   svc,
			Printer:   printer,
			Converter: mockConverter(),
			Config:    &jira4claude.Config{Project: "PROJ"},
		}
		cmd := main.IssueFieldsCmd{Project: "PROJ", Type: "Task"}
		err := cmd.Run(ctx)

		require.NoError(t, err)
		require.Len(t, printer.FieldsCalls, 1)
		fields := printer.FieldsCalls[0].Fields
		ids := make([]string, len(fields))
		for i, f := range fields {
			ids[i] = f.ID
		}
		assert.ElementsMatch(t, []string{"summary", "customfield_10010", "customfield_10801"}, ids)
	})

	t.Run("--all bypasses filter and emits every field", func(t *testing.T) {
		t.Parallel()

		svc := &mock.IssueService{
			GetCreateFieldsFn: func(ctx context.Context, projectKey, issueType string) ([]*jira4claude.IssueField, error) {
				return []*jira4claude.IssueField{
					{ID: "summary", Required: true},
					{ID: "description", Required: false},
					{ID: "customfield_10010", Required: false},
					{ID: "priority", Required: false},
				}, nil
			},
		}

		printer := &mock.Printer{}
		ctx := &main.IssueContext{
			Service:   svc,
			Printer:   printer,
			Converter: mockConverter(),
			Config:    &jira4claude.Config{Project: "PROJ"},
		}
		cmd := main.IssueFieldsCmd{Project: "PROJ", Type: "Task", All: true}
		err := cmd.Run(ctx)

		require.NoError(t, err)
		require.Len(t, printer.FieldsCalls, 1)
		assert.Len(t, printer.FieldsCalls[0].Fields, 4)
		assert.Equal(t, jira4claude.FieldScopeAll, printer.FieldsCalls[0].Scope)
		assert.Equal(t, 0, printer.FieldsCalls[0].Omitted)
	})

	t.Run("default scope reports scope and omitted count", func(t *testing.T) {
		t.Parallel()

		svc := &mock.IssueService{
			GetCreateFieldsFn: func(ctx context.Context, projectKey, issueType string) ([]*jira4claude.IssueField, error) {
				return []*jira4claude.IssueField{
					{ID: "summary", Required: true},
					{ID: "description", Required: false},
					{ID: "customfield_10010", Required: false},
					{ID: "priority", Required: false},
				}, nil
			},
		}

		printer := &mock.Printer{}
		ctx := &main.IssueContext{
			Service:   svc,
			Printer:   printer,
			Converter: mockConverter(),
			Config:    &jira4claude.Config{Project: "PROJ"},
		}
		cmd := main.IssueFieldsCmd{Project: "PROJ", Type: "Task"}
		err := cmd.Run(ctx)

		require.NoError(t, err)
		require.Len(t, printer.FieldsCalls, 1)
		assert.Equal(t, jira4claude.FieldScopeDefault, printer.FieldsCalls[0].Scope)
		assert.Equal(t, 2, printer.FieldsCalls[0].Omitted) // description, priority
	})

	t.Run("--filter selects matching fields across all fields", func(t *testing.T) {
		t.Parallel()

		svc := &mock.IssueService{
			GetCreateFieldsFn: func(ctx context.Context, projectKey, issueType string) ([]*jira4claude.IssueField, error) {
				return []*jira4claude.IssueField{
					{ID: "summary", Name: "Summary", Required: true},
					{ID: "description", Name: "Description", Required: false},
					{ID: "customfield_10010", Name: "Story Points", Required: false},
					{ID: "priority", Name: "Priority", Required: false},
				}, nil
			},
		}

		printer := &mock.Printer{}
		ctx := &main.IssueContext{
			Service:   svc,
			Printer:   printer,
			Converter: mockConverter(),
			Config:    &jira4claude.Config{Project: "PROJ"},
		}
		// "prio" matches the optional builtin "priority" that default mode hides.
		cmd := main.IssueFieldsCmd{Project: "PROJ", Type: "Task", Filter: "prio"}
		err := cmd.Run(ctx)

		require.NoError(t, err)
		require.Len(t, printer.FieldsCalls, 1)
		require.Len(t, printer.FieldsCalls[0].Fields, 1)
		assert.Equal(t, "priority", printer.FieldsCalls[0].Fields[0].ID)
		assert.Equal(t, jira4claude.FieldScopeFiltered, printer.FieldsCalls[0].Scope)
	})

	t.Run("type falls back to Task when --type not given (Run applies the default)", func(t *testing.T) {
		t.Parallel()

		var capturedType string
		svc := &mock.IssueService{
			GetCreateFieldsFn: func(ctx context.Context, projectKey, issueType string) ([]*jira4claude.IssueField, error) {
				capturedType = issueType
				return []*jira4claude.IssueField{}, nil
			},
		}

		printer := &mock.Printer{}
		ctx := &main.IssueContext{
			Service:   svc,
			Printer:   printer,
			Converter: mockConverter(),
			Config:    &jira4claude.Config{Project: "PROJ"},
		}
		// Type left empty, mirroring Kong v1.13.0's no-default behavior.
		cmd := main.IssueFieldsCmd{Project: "PROJ"}
		err := cmd.Run(ctx)

		require.NoError(t, err)
		assert.Equal(t, "Task", capturedType)
		require.Len(t, printer.FieldsCalls, 1)
		assert.Equal(t, "PROJ / Task", printer.FieldsCalls[0].Source)
	})

	t.Run("project falls back to ctx.Config.Project when --project not given", func(t *testing.T) {
		t.Parallel()

		var capturedProject string
		svc := &mock.IssueService{
			GetCreateFieldsFn: func(ctx context.Context, projectKey, issueType string) ([]*jira4claude.IssueField, error) {
				capturedProject = projectKey
				return []*jira4claude.IssueField{}, nil
			},
		}

		printer := &mock.Printer{}
		ctx := &main.IssueContext{
			Service:   svc,
			Printer:   printer,
			Converter: mockConverter(),
			Config:    &jira4claude.Config{Project: "CONFIG"},
		}
		cmd := main.IssueFieldsCmd{Type: "Task"}
		err := cmd.Run(ctx)

		require.NoError(t, err)
		assert.Equal(t, "CONFIG", capturedProject)
		require.Len(t, printer.FieldsCalls, 1)
		assert.Equal(t, "CONFIG / Task", printer.FieldsCalls[0].Source)
	})

	t.Run("returns EValidation when project missing in both flag and config", func(t *testing.T) {
		t.Parallel()

		createFieldsCalled := false
		svc := &mock.IssueService{
			GetCreateFieldsFn: func(ctx context.Context, projectKey, issueType string) ([]*jira4claude.IssueField, error) {
				createFieldsCalled = true
				return nil, nil
			},
		}

		printer := &mock.Printer{}
		ctx := &main.IssueContext{
			Service:   svc,
			Printer:   printer,
			Converter: mockConverter(),
			Config:    &jira4claude.Config{},
		}
		cmd := main.IssueFieldsCmd{Type: "Task"}
		err := cmd.Run(ctx)

		require.Error(t, err)
		assert.Equal(t, jira4claude.EValidation, jira4claude.ErrorCode(err))
		assert.Contains(t, err.Error(), "project")
		assert.False(t, createFieldsCalled, "service should not be called when project is missing")
		assert.Empty(t, printer.FieldsCalls)
	})

	t.Run("propagates GetCreateFields error", func(t *testing.T) {
		t.Parallel()

		svc := &mock.IssueService{
			GetCreateFieldsFn: func(ctx context.Context, projectKey, issueType string) ([]*jira4claude.IssueField, error) {
				return nil, &jira4claude.Error{Code: jira4claude.ENotFound, Message: "project not found"}
			},
		}

		printer := &mock.Printer{}
		ctx := &main.IssueContext{
			Service:   svc,
			Printer:   printer,
			Converter: mockConverter(),
			Config:    &jira4claude.Config{Project: "PROJ"},
		}
		cmd := main.IssueFieldsCmd{Project: "PROJ", Type: "Task"}
		err := cmd.Run(ctx)

		require.Error(t, err)
		assert.Equal(t, jira4claude.ENotFound, jira4claude.ErrorCode(err))
		assert.Empty(t, printer.FieldsCalls)
	})

	t.Run("propagates GetEditFields error", func(t *testing.T) {
		t.Parallel()

		svc := &mock.IssueService{
			GetEditFieldsFn: func(ctx context.Context, key string) ([]*jira4claude.IssueField, error) {
				return nil, &jira4claude.Error{Code: jira4claude.ENotFound, Message: "issue not found"}
			},
		}

		printer := &mock.Printer{}
		ctx := &main.IssueContext{
			Service:   svc,
			Printer:   printer,
			Converter: mockConverter(),
			Config:    &jira4claude.Config{Project: "PROJ"},
		}
		cmd := main.IssueFieldsCmd{Key: "PROJ-1"}
		err := cmd.Run(ctx)

		require.Error(t, err)
		assert.Equal(t, jira4claude.ENotFound, jira4claude.ErrorCode(err))
		assert.Empty(t, printer.FieldsCalls)
	})
}

// IssueFieldsCmd Kong xor parsing tests

func TestIssueFieldsCmd_KongXor(t *testing.T) {
	t.Parallel()

	t.Run("--key + --project is rejected (share mode-project label)", func(t *testing.T) {
		t.Parallel()

		var cli main.CLI
		parser, err := kong.New(&cli)
		require.NoError(t, err)

		_, err = parser.Parse([]string{"issue", "fields", "--key=PROJ-1", "--project=PROJ"})
		require.Error(t, err)
		// Kong reports xor conflicts with a message that names the conflicting flags.
		errMsg := err.Error()
		assert.Contains(t, errMsg, "key")
		assert.Contains(t, errMsg, "project")
	})

	t.Run("--key + --type is rejected (share mode-type label)", func(t *testing.T) {
		t.Parallel()

		var cli main.CLI
		parser, err := kong.New(&cli)
		require.NoError(t, err)

		_, err = parser.Parse([]string{"issue", "fields", "--key=PROJ-1", "--type=Bug"})
		require.Error(t, err)
		errMsg := err.Error()
		assert.Contains(t, errMsg, "key")
		assert.Contains(t, errMsg, "type")
	})

	t.Run("--project + --type is accepted (no shared xor label)", func(t *testing.T) {
		t.Parallel()

		var cli main.CLI
		parser, err := kong.New(&cli)
		require.NoError(t, err)

		_, err = parser.Parse([]string{"issue", "fields", "--project=PROJ", "--type=Bug"})
		require.NoError(t, err)
		assert.Equal(t, "PROJ", cli.Issue.Fields.Project)
		assert.Equal(t, "Bug", cli.Issue.Fields.Type)
	})

	t.Run("--key alone is accepted (Type has no Kong default so does not trip xor)", func(t *testing.T) {
		t.Parallel()

		var cli main.CLI
		parser, err := kong.New(&cli)
		require.NoError(t, err)

		_, err = parser.Parse([]string{"issue", "fields", "--key=PROJ-1"})
		require.NoError(t, err)
		assert.Equal(t, "PROJ-1", cli.Issue.Fields.Key)
		// Type is left empty by Kong; Run() applies the "Task" fallback.
		assert.Empty(t, cli.Issue.Fields.Type)
	})

	t.Run("no flags is accepted (project comes from config, type defaults in Run)", func(t *testing.T) {
		t.Parallel()

		var cli main.CLI
		parser, err := kong.New(&cli)
		require.NoError(t, err)

		_, err = parser.Parse([]string{"issue", "fields"})
		require.NoError(t, err)
		assert.Empty(t, cli.Issue.Fields.Key)
		assert.Empty(t, cli.Issue.Fields.Project)
		assert.Empty(t, cli.Issue.Fields.Type)
	})
}
