package main_test

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/fwojciec/jira4claude"
	main "github.com/fwojciec/jira4claude/cmd/j4c"
	"github.com/fwojciec/jira4claude/gogh"
	"github.com/fwojciec/jira4claude/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test helpers

func makeIssueContext(t *testing.T, svc jira4claude.IssueService, out *bytes.Buffer) *main.IssueContext {
	t.Helper()
	io := gogh.NewIO(out, out)
	printer := gogh.NewTextPrinter(io)
	return &main.IssueContext{
		Service: svc,
		Printer: printer,
		Config:  &jira4claude.Config{Project: "TEST", Server: "https://test.atlassian.net"},
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

// IssueTransitionCmd tests

func TestIssueTransitionCmd_InvalidStatusShowsQuotedOptions(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	io := gogh.NewIO(&out, &errOut)
	printer := gogh.NewTextPrinter(io)

	svc := &mock.IssueService{
		TransitionsFn: func(ctx context.Context, key string) ([]*jira4claude.Transition, error) {
			return []*jira4claude.Transition{
				{ID: "21", Name: "In Progress"},
				{ID: "31", Name: "Done"},
			}, nil
		},
	}

	ctx := &main.IssueContext{
		Service: svc,
		Printer: printer,
		Config:  &jira4claude.Config{Project: "TEST"},
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

	t.Run("converts markdown description when flag is set", func(t *testing.T) {
		t.Parallel()

		var capturedIssue *jira4claude.Issue
		svc := &mock.IssueService{
			CreateFn: func(ctx context.Context, issue *jira4claude.Issue) (*jira4claude.Issue, error) {
				capturedIssue = issue
				return &jira4claude.Issue{Key: "TEST-1"}, nil
			},
		}

		var buf bytes.Buffer
		ctx := makeIssueContext(t, svc, &buf)
		cmd := main.IssueCreateCmd{
			Summary:     "Test issue",
			Description: "**bold** and *italic*",
			Markdown:    true,
		}
		err := cmd.Run(ctx)

		require.NoError(t, err)
		require.NotNil(t, capturedIssue)
		// Description should be pre-converted ADF JSON when markdown flag is set
		assert.True(t, strings.HasPrefix(capturedIssue.Description, `{"content":[`))
	})

	t.Run("keeps plain text description when markdown flag is not set", func(t *testing.T) {
		t.Parallel()

		var capturedIssue *jira4claude.Issue
		svc := &mock.IssueService{
			CreateFn: func(ctx context.Context, issue *jira4claude.Issue) (*jira4claude.Issue, error) {
				capturedIssue = issue
				return &jira4claude.Issue{Key: "TEST-1"}, nil
			},
		}

		var buf bytes.Buffer
		ctx := makeIssueContext(t, svc, &buf)
		cmd := main.IssueCreateCmd{
			Summary:     "Test issue",
			Description: "**bold** and *italic*",
			Markdown:    false,
		}
		err := cmd.Run(ctx)

		require.NoError(t, err)
		require.NotNil(t, capturedIssue)
		// Description should remain as plain text
		assert.Equal(t, "**bold** and *italic*", capturedIssue.Description)
	})

	t.Run("skips markdown conversion when description is empty", func(t *testing.T) {
		t.Parallel()

		var capturedIssue *jira4claude.Issue
		svc := &mock.IssueService{
			CreateFn: func(ctx context.Context, issue *jira4claude.Issue) (*jira4claude.Issue, error) {
				capturedIssue = issue
				return &jira4claude.Issue{Key: "TEST-1"}, nil
			},
		}

		var buf bytes.Buffer
		ctx := makeIssueContext(t, svc, &buf)
		cmd := main.IssueCreateCmd{
			Summary:     "Test issue",
			Description: "",
			Markdown:    true,
		}
		err := cmd.Run(ctx)

		require.NoError(t, err)
		require.NotNil(t, capturedIssue)
		// Description should remain empty
		assert.Empty(t, capturedIssue.Description)
	})
}

// IssueEditCmd tests

func TestIssueEditCmd(t *testing.T) {
	t.Parallel()

	t.Run("converts markdown description when flag is set", func(t *testing.T) {
		t.Parallel()

		var capturedUpdate jira4claude.IssueUpdate
		svc := &mock.IssueService{
			UpdateFn: func(ctx context.Context, key string, update jira4claude.IssueUpdate) (*jira4claude.Issue, error) {
				capturedUpdate = update
				return makeIssue(key), nil
			},
		}

		var buf bytes.Buffer
		ctx := makeIssueContext(t, svc, &buf)
		description := "**bold** and *italic*"
		cmd := main.IssueEditCmd{Key: "TEST-1", Description: &description, Markdown: true}
		err := cmd.Run(ctx)

		require.NoError(t, err)
		require.NotNil(t, capturedUpdate.Description)
		// Description should be pre-converted ADF JSON when markdown flag is set
		assert.True(t, strings.HasPrefix(*capturedUpdate.Description, `{"content":[`))
	})

	t.Run("keeps plain text description when markdown flag is not set", func(t *testing.T) {
		t.Parallel()

		var capturedUpdate jira4claude.IssueUpdate
		svc := &mock.IssueService{
			UpdateFn: func(ctx context.Context, key string, update jira4claude.IssueUpdate) (*jira4claude.Issue, error) {
				capturedUpdate = update
				return makeIssue(key), nil
			},
		}

		var buf bytes.Buffer
		ctx := makeIssueContext(t, svc, &buf)
		description := "**bold** and *italic*"
		cmd := main.IssueEditCmd{Key: "TEST-1", Description: &description, Markdown: false}
		err := cmd.Run(ctx)

		require.NoError(t, err)
		require.NotNil(t, capturedUpdate.Description)
		// Description should remain as plain text
		assert.Equal(t, "**bold** and *italic*", *capturedUpdate.Description)
	})

	t.Run("skips markdown conversion when description is empty", func(t *testing.T) {
		t.Parallel()

		var capturedUpdate jira4claude.IssueUpdate
		svc := &mock.IssueService{
			UpdateFn: func(ctx context.Context, key string, update jira4claude.IssueUpdate) (*jira4claude.Issue, error) {
				capturedUpdate = update
				return makeIssue(key), nil
			},
		}

		var buf bytes.Buffer
		ctx := makeIssueContext(t, svc, &buf)
		description := ""
		cmd := main.IssueEditCmd{Key: "TEST-1", Description: &description, Markdown: true}
		err := cmd.Run(ctx)

		require.NoError(t, err)
		require.NotNil(t, capturedUpdate.Description)
		// Description should remain empty
		assert.Empty(t, *capturedUpdate.Description)
	})
}

// IssueCommentCmd tests

func TestIssueCommentCmd(t *testing.T) {
	t.Parallel()

	t.Run("converts markdown body when flag is set", func(t *testing.T) {
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

		var buf bytes.Buffer
		ctx := makeIssueContext(t, svc, &buf)
		cmd := main.IssueCommentCmd{Key: "TEST-1", Body: "**bold** and *italic*", Markdown: true}
		err := cmd.Run(ctx)

		require.NoError(t, err)
		// Body should be pre-converted ADF JSON when markdown flag is set
		assert.True(t, strings.HasPrefix(capturedBody, `{"content":[`))
	})

	t.Run("keeps plain text body when markdown flag is not set", func(t *testing.T) {
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

		var buf bytes.Buffer
		ctx := makeIssueContext(t, svc, &buf)
		cmd := main.IssueCommentCmd{Key: "TEST-1", Body: "**bold** and *italic*", Markdown: false}
		err := cmd.Run(ctx)

		require.NoError(t, err)
		// Body should remain as plain text
		assert.Equal(t, "**bold** and *italic*", capturedBody)
	})
}

// IssueViewCmd tests

func TestIssueViewCmd(t *testing.T) {
	t.Parallel()

	t.Run("displays description as markdown when flag is set", func(t *testing.T) {
		t.Parallel()

		svc := &mock.IssueService{
			GetFn: func(ctx context.Context, key string) (*jira4claude.Issue, error) {
				return &jira4claude.Issue{
					Key:     "TEST-1",
					Summary: "Test issue",
					Status:  "To Do",
					Type:    "Task",
					// DescriptionADF contains a heading that should render as "# Hello"
					DescriptionADF: map[string]any{
						"type":    "doc",
						"version": 1,
						"content": []any{
							map[string]any{
								"type": "heading",
								"attrs": map[string]any{
									"level": 1,
								},
								"content": []any{
									map[string]any{
										"type": "text",
										"text": "Hello",
									},
								},
							},
						},
					},
				}, nil
			},
		}

		var buf bytes.Buffer
		ctx := makeIssueContext(t, svc, &buf)
		cmd := main.IssueViewCmd{Key: "TEST-1", Markdown: true}
		err := cmd.Run(ctx)

		require.NoError(t, err)
		assert.Contains(t, buf.String(), "# Hello")
	})

	t.Run("displays description as plain text by default", func(t *testing.T) {
		t.Parallel()

		svc := &mock.IssueService{
			GetFn: func(ctx context.Context, key string) (*jira4claude.Issue, error) {
				return &jira4claude.Issue{
					Key:         "TEST-1",
					Summary:     "Test issue",
					Status:      "To Do",
					Type:        "Task",
					Description: "Hello",
					DescriptionADF: map[string]any{
						"type":    "doc",
						"version": 1,
						"content": []any{
							map[string]any{
								"type": "heading",
								"attrs": map[string]any{
									"level": 1,
								},
								"content": []any{
									map[string]any{
										"type": "text",
										"text": "Hello",
									},
								},
							},
						},
					},
				}, nil
			},
		}

		var buf bytes.Buffer
		ctx := makeIssueContext(t, svc, &buf)
		cmd := main.IssueViewCmd{Key: "TEST-1"}
		err := cmd.Run(ctx)

		require.NoError(t, err)
		// Should NOT contain markdown heading syntax
		assert.NotContains(t, buf.String(), "# Hello")
		// Should contain plain text
		assert.Contains(t, buf.String(), "Hello")
	})

	t.Run("falls back to plain text when markdown flag is set but no ADF", func(t *testing.T) {
		t.Parallel()

		svc := &mock.IssueService{
			GetFn: func(ctx context.Context, key string) (*jira4claude.Issue, error) {
				return &jira4claude.Issue{
					Key:            "TEST-1",
					Summary:        "Test issue",
					Status:         "To Do",
					Type:           "Task",
					Description:    "Plain text description",
					DescriptionADF: nil,
				}, nil
			},
		}

		var buf bytes.Buffer
		ctx := makeIssueContext(t, svc, &buf)
		cmd := main.IssueViewCmd{Key: "TEST-1", Markdown: true}
		err := cmd.Run(ctx)

		require.NoError(t, err)
		assert.Contains(t, buf.String(), "Plain text description")
	})

	t.Run("displays comment bodies as markdown when flag is set", func(t *testing.T) {
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
							Body:   "Comment text",
							BodyADF: map[string]any{
								"type":    "doc",
								"version": 1,
								"content": []any{
									map[string]any{
										"type": "paragraph",
										"content": []any{
											map[string]any{
												"type": "text",
												"text": "Comment with ",
											},
											map[string]any{
												"type": "text",
												"text": "bold",
												"marks": []any{
													map[string]any{"type": "strong"},
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

		var buf bytes.Buffer
		ctx := makeIssueContext(t, svc, &buf)
		cmd := main.IssueViewCmd{Key: "TEST-1", Markdown: true}
		err := cmd.Run(ctx)

		require.NoError(t, err)
		// Comment body should contain markdown bold syntax
		assert.Contains(t, buf.String(), "Comment with **bold**")
	})

	t.Run("displays comment bodies as plain text by default", func(t *testing.T) {
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
							Body:   "Comment text",
							BodyADF: map[string]any{
								"type":    "doc",
								"version": 1,
								"content": []any{
									map[string]any{
										"type": "paragraph",
										"content": []any{
											map[string]any{
												"type": "text",
												"text": "Comment with ",
											},
											map[string]any{
												"type": "text",
												"text": "bold",
												"marks": []any{
													map[string]any{"type": "strong"},
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

		var buf bytes.Buffer
		ctx := makeIssueContext(t, svc, &buf)
		cmd := main.IssueViewCmd{Key: "TEST-1"}
		err := cmd.Run(ctx)

		require.NoError(t, err)
		// Comment body should be plain text (no markdown syntax)
		assert.Contains(t, buf.String(), "Comment text")
		assert.NotContains(t, buf.String(), "**bold**")
	})
}

// IssueAssignCmd tests

func TestIssueAssignCmd(t *testing.T) {
	t.Parallel()

	t.Run("prints success message when assigning to user", func(t *testing.T) {
		t.Parallel()

		svc := &mock.IssueService{
			AssignFn: func(ctx context.Context, key, accountID string) error {
				return nil
			},
		}

		var buf bytes.Buffer
		ctx := makeIssueContext(t, svc, &buf)
		cmd := main.IssueAssignCmd{Key: "TEST-1", AccountID: "abc123"}
		err := cmd.Run(ctx)

		require.NoError(t, err)
		assert.Contains(t, buf.String(), "Assigned:")
		assert.Contains(t, buf.String(), "TEST-1")
	})

	t.Run("prints unassign message when account ID is empty", func(t *testing.T) {
		t.Parallel()

		svc := &mock.IssueService{
			AssignFn: func(ctx context.Context, key, accountID string) error {
				return nil
			},
		}

		var buf bytes.Buffer
		ctx := makeIssueContext(t, svc, &buf)
		cmd := main.IssueAssignCmd{Key: "TEST-1", AccountID: ""}
		err := cmd.Run(ctx)

		require.NoError(t, err)
		assert.Contains(t, buf.String(), "Unassigned:")
		assert.Contains(t, buf.String(), "TEST-1")
	})

	t.Run("returns error when service fails", func(t *testing.T) {
		t.Parallel()

		svc := &mock.IssueService{
			AssignFn: func(ctx context.Context, key, accountID string) error {
				return &jira4claude.Error{Code: jira4claude.ENotFound, Message: "issue not found"}
			},
		}

		var buf bytes.Buffer
		ctx := makeIssueContext(t, svc, &buf)
		cmd := main.IssueAssignCmd{Key: "NOTFOUND-1", AccountID: "abc123"}
		err := cmd.Run(ctx)

		require.Error(t, err)
		assert.Equal(t, jira4claude.ENotFound, jira4claude.ErrorCode(err))
	})
}
