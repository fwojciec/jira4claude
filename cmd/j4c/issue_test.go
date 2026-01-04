package main_test

import (
	"bytes"
	"context"
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
	converter := &mock.Converter{
		ToADFFn: func(markdown string) (jira4claude.ADF, []string) {
			// Simple mock that creates a valid ADF document
			return jira4claude.ADF{
				"type":    "doc",
				"version": 1,
				"content": []any{
					map[string]any{
						"type": "paragraph",
						"content": []any{
							map[string]any{
								"type": "text",
								"text": markdown,
							},
						},
					},
				},
			}, nil
		},
		ToMarkdownFn: func(adf jira4claude.ADF) (string, []string) {
			// Simple mock that extracts and concatenates text from ADF
			var result string
			if content, ok := adf["content"].([]any); ok {
				for _, block := range content {
					if para, ok := block.(map[string]any); ok {
						if paraContent, ok := para["content"].([]any); ok {
							for _, node := range paraContent {
								if textNode, ok := node.(map[string]any); ok {
									if text, ok := textNode["text"].(string); ok {
										result += text
									}
								}
							}
						}
					}
				}
			}
			return result, nil
		},
	}
	return &main.IssueContext{
		Service:   svc,
		Printer:   printer,
		Converter: converter,
		Config:    &jira4claude.Config{Project: "TEST", Server: "https://test.atlassian.net"},
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
		Service:   svc,
		Printer:   printer,
		Converter: &mock.Converter{}, // Not used by transitions, but required
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

		var buf bytes.Buffer
		ctx := makeIssueContext(t, svc, &buf)
		cmd := main.IssueCreateCmd{
			Summary:     "Test issue",
			Description: "**bold** and *italic*",
		}
		err := cmd.Run(ctx)

		require.NoError(t, err)
		require.NotNil(t, capturedIssue)
		// Description should be ADF (map[string]any)
		assert.Equal(t, "doc", capturedIssue.Description["type"])
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

		var buf bytes.Buffer
		ctx := makeIssueContext(t, svc, &buf)
		cmd := main.IssueCreateCmd{
			Summary:     "Test issue",
			Description: "plain text without formatting",
		}
		err := cmd.Run(ctx)

		require.NoError(t, err)
		require.NotNil(t, capturedIssue)
		// Plain text is valid GFM and should be converted to ADF
		assert.Equal(t, "doc", capturedIssue.Description["type"])
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

		var buf bytes.Buffer
		ctx := makeIssueContext(t, svc, &buf)
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

	t.Run("sets type to Sub-task when parent is specified", func(t *testing.T) {
		t.Parallel()

		var capturedIssue *jira4claude.Issue
		svc := &mock.IssueService{
			CreateFn: func(ctx context.Context, issue *jira4claude.Issue) (*jira4claude.Issue, error) {
				capturedIssue = issue
				return &jira4claude.Issue{Key: "TEST-2"}, nil
			},
		}

		var buf bytes.Buffer
		ctx := makeIssueContext(t, svc, &buf)
		cmd := main.IssueCreateCmd{
			Summary: "Subtask issue",
			Parent:  "TEST-1",
		}
		err := cmd.Run(ctx)

		require.NoError(t, err)
		require.NotNil(t, capturedIssue)
		// Type must be exactly "Sub-task" (with hyphen) for Jira API
		assert.Equal(t, "Sub-task", capturedIssue.Type)
		assert.Equal(t, "TEST-1", capturedIssue.Parent)
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

		var buf bytes.Buffer
		ctx := makeIssueContext(t, svc, &buf)
		description := "**bold** and *italic*"
		cmd := main.IssueUpdateCmd{Key: "TEST-1", Description: &description}
		err := cmd.Run(ctx)

		require.NoError(t, err)
		require.NotNil(t, capturedUpdate.Description)
		// Description should be ADF (map[string]any)
		assert.Equal(t, "doc", (*capturedUpdate.Description)["type"])
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

		var buf bytes.Buffer
		ctx := makeIssueContext(t, svc, &buf)
		description := "plain text without formatting"
		cmd := main.IssueUpdateCmd{Key: "TEST-1", Description: &description}
		err := cmd.Run(ctx)

		require.NoError(t, err)
		require.NotNil(t, capturedUpdate.Description)
		// Plain text is valid GFM and should be converted to ADF
		assert.Equal(t, "doc", (*capturedUpdate.Description)["type"])
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

		var buf bytes.Buffer
		ctx := makeIssueContext(t, svc, &buf)
		description := ""
		cmd := main.IssueUpdateCmd{Key: "TEST-1", Description: &description}
		err := cmd.Run(ctx)

		require.NoError(t, err)
		// Empty description is not converted - nil is passed
		assert.Nil(t, capturedUpdate.Description)
	})
}

// IssueCommentCmd tests

func TestIssueCommentCmd(t *testing.T) {
	t.Parallel()

	t.Run("always converts body as GFM", func(t *testing.T) {
		t.Parallel()

		var capturedBody jira4claude.ADF
		svc := &mock.IssueService{
			AddCommentFn: func(ctx context.Context, key string, body jira4claude.ADF) (*jira4claude.Comment, error) {
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
		cmd := main.IssueCommentCmd{Key: "TEST-1", Body: "**bold** and *italic*"}
		err := cmd.Run(ctx)

		require.NoError(t, err)
		// Body should be ADF (map[string]any)
		assert.Equal(t, "doc", capturedBody["type"])
	})

	t.Run("plain text input is valid GFM", func(t *testing.T) {
		t.Parallel()

		var capturedBody jira4claude.ADF
		svc := &mock.IssueService{
			AddCommentFn: func(ctx context.Context, key string, body jira4claude.ADF) (*jira4claude.Comment, error) {
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
		cmd := main.IssueCommentCmd{Key: "TEST-1", Body: "plain text without formatting"}
		err := cmd.Run(ctx)

		require.NoError(t, err)
		// Plain text is valid GFM and should be converted to ADF
		assert.Equal(t, "doc", capturedBody["type"])
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

		var buf bytes.Buffer
		ctx := makeIssueContext(t, svc, &buf)
		cmd := main.IssueReadyCmd{} // No project specified
		err := cmd.Run(ctx)

		require.NoError(t, err)
		// JQL should contain the config project "TEST"
		assert.Contains(t, capturedFilter.JQL, `project = "TEST"`)
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

		var buf bytes.Buffer
		ctx := makeIssueContext(t, svc, &buf)
		cmd := main.IssueReadyCmd{Project: "CUSTOM"}
		err := cmd.Run(ctx)

		require.NoError(t, err)
		// JQL should contain the explicit project "CUSTOM"
		assert.Contains(t, capturedFilter.JQL, `project = "CUSTOM"`)
		assert.NotContains(t, capturedFilter.JQL, `project = "TEST"`)
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

		var buf bytes.Buffer
		ctx := makeIssueContext(t, svc, &buf)
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

		var buf bytes.Buffer
		ctx := makeIssueContext(t, svc, &buf)
		cmd := main.IssueReadyCmd{}
		err := cmd.Run(ctx)

		require.NoError(t, err)
		output := buf.String()
		// Ready issues should be in output
		assert.Contains(t, output, "TEST-1")
		assert.Contains(t, output, "TEST-4")
		// Blocked issue should NOT be in output
		assert.NotContains(t, output, "TEST-2")
	})

	t.Run("handles empty result", func(t *testing.T) {
		t.Parallel()

		svc := &mock.IssueService{
			ListFn: func(ctx context.Context, filter jira4claude.IssueFilter) ([]*jira4claude.Issue, error) {
				return []*jira4claude.Issue{}, nil
			},
		}

		var buf bytes.Buffer
		ctx := makeIssueContext(t, svc, &buf)
		cmd := main.IssueReadyCmd{}
		err := cmd.Run(ctx)

		require.NoError(t, err)
		// Should not panic or error on empty result
		// Output may be empty or contain a header, but no issue keys
		assert.NotContains(t, buf.String(), "TEST-")
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

		var buf bytes.Buffer
		ctx := makeIssueContext(t, svc, &buf)
		cmd := main.IssueReadyCmd{}
		err := cmd.Run(ctx)

		require.NoError(t, err)
		// No ready issues should be in output
		assert.NotContains(t, buf.String(), "TEST-1")
	})

	t.Run("propagates service errors", func(t *testing.T) {
		t.Parallel()

		expectedErr := &jira4claude.Error{Code: jira4claude.EInternal, Message: "connection failed"}
		svc := &mock.IssueService{
			ListFn: func(ctx context.Context, filter jira4claude.IssueFilter) ([]*jira4claude.Issue, error) {
				return nil, expectedErr
			},
		}

		var buf bytes.Buffer
		ctx := makeIssueContext(t, svc, &buf)
		cmd := main.IssueReadyCmd{}
		err := cmd.Run(ctx)

		require.Error(t, err)
		assert.Equal(t, expectedErr, err)
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
					// Description is now ADF - the printer outputs it as JSON
					Description: jira4claude.ADF{
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
		assert.Contains(t, buf.String(), "Hello")
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

		var buf bytes.Buffer
		ctx := makeIssueContext(t, svc, &buf)
		cmd := main.IssueViewCmd{Key: "TEST-1"}
		err := cmd.Run(ctx)

		require.NoError(t, err)
		// No description should be shown when nil
		assert.NotContains(t, buf.String(), "description")
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
							// Body is now ADF - the printer outputs it as JSON
							Body: jira4claude.ADF{
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
		assert.Contains(t, buf.String(), "Comment with ")
		assert.Contains(t, buf.String(), "bold")
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

		var buf bytes.Buffer
		ctx := makeIssueContext(t, svc, &buf)
		// Config has Project set to "TEST" via makeIssueContext
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

		var buf bytes.Buffer
		ctx := makeIssueContext(t, svc, &buf)
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

		var buf bytes.Buffer
		ctx := makeIssueContext(t, svc, &buf)
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

		var buf bytes.Buffer
		ctx := makeIssueContext(t, svc, &buf)
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

		var buf bytes.Buffer
		ctx := makeIssueContext(t, svc, &buf)
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

		var buf bytes.Buffer
		ctx := makeIssueContext(t, svc, &buf)
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

		var buf bytes.Buffer
		ctx := makeIssueContext(t, svc, &buf)
		cmd := main.IssueListCmd{Project: "TEST"}
		err := cmd.Run(ctx)

		require.NoError(t, err)
		output := buf.String()
		assert.Contains(t, output, "TEST-1")
		assert.Contains(t, output, "TEST-2")
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
