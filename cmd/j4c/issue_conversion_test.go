package main_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/alecthomas/kong"
	"github.com/fwojciec/jira4claude"
	main "github.com/fwojciec/jira4claude/cmd/j4c"
	"github.com/fwojciec/jira4claude/markdown"
	"github.com/fwojciec/jira4claude/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// imageMarkdown is a Markdown image, which the converter cannot write yet and
// therefore reports as dropped.
const imageMarkdown = "![alt](http://x/y.png)"

// assertDroppedImageWarned checks that the dropped Image was reported as a
// conversion warning.
func assertDroppedImageWarned(t *testing.T, printer *mock.Printer) {
	t.Helper()
	require.Len(t, printer.ConversionIssueCalls, 1)
	assert.Equal(t, jira4claude.ConversionDropped, printer.ConversionIssueCalls[0].Kind)
	assert.Equal(t, "Image", printer.ConversionIssueCalls[0].Node)
}

// assertDroppedImageRefusal checks that err is a validation error (exit 1)
// naming the dropped Image node.
func assertDroppedImageRefusal(t *testing.T, err error) {
	t.Helper()
	require.Error(t, err)
	assert.Equal(t, jira4claude.EValidation, jira4claude.ErrorCode(err))
	assert.Equal(t, 1, jira4claude.ExitCode(err))
	assert.Contains(t, jira4claude.ErrorMessage(err), "Image")
	assert.Contains(t, jira4claude.ErrorMessage(err), "--force")
}

// degradedConverter reports a degraded (not dropped) issue on every write.
func degradedConverter() *mock.Converter {
	conv := mockConverter()
	inner := conv.ToADFFn
	conv.ToADFFn = func(md string) (*jira4claude.ADFNode, []jira4claude.ConversionIssue) {
		doc, _ := inner(md)
		return doc, []jira4claude.ConversionIssue{
			{Kind: jira4claude.ConversionDegraded, Node: "emoji", Detail: "rendered as text"},
		}
	}
	return conv
}

func TestIssueCreateCmd_DroppedContent(t *testing.T) {
	t.Parallel()

	t.Run("refuses a description that drops an image and makes no service call", func(t *testing.T) {
		t.Parallel()

		called := false
		svc := &mock.IssueService{
			CreateFn: func(ctx context.Context, issue *jira4claude.Issue) (*jira4claude.Issue, error) {
				called = true
				return &jira4claude.Issue{Key: "TEST-1"}, nil
			},
		}
		printer := &mock.Printer{}
		ctx := &main.IssueContext{
			Service:   svc,
			Printer:   printer,
			Converter: markdown.New(),
			Config:    &jira4claude.Config{Project: "TEST"},
		}
		cmd := main.IssueCreateCmd{Summary: "s", Description: imageMarkdown}

		err := cmd.Run(ctx)

		assertDroppedImageRefusal(t, err)
		assert.False(t, called, "Create must not be called")
		assert.Empty(t, printer.SuccessCalls)
	})

	t.Run("--force creates the issue and warns about the dropped image", func(t *testing.T) {
		t.Parallel()

		called := false
		svc := &mock.IssueService{
			CreateFn: func(ctx context.Context, issue *jira4claude.Issue) (*jira4claude.Issue, error) {
				called = true
				return &jira4claude.Issue{Key: "TEST-1"}, nil
			},
		}
		printer := &mock.Printer{}
		ctx := &main.IssueContext{
			Service:   svc,
			Printer:   printer,
			Converter: markdown.New(),
			Config:    &jira4claude.Config{Project: "TEST"},
		}
		cmd := main.IssueCreateCmd{Summary: "s", Description: imageMarkdown, Force: true}

		err := cmd.Run(ctx)

		require.NoError(t, err)
		assert.True(t, called, "Create must be called with --force")
		assertDroppedImageWarned(t, printer)
	})

	t.Run("degraded content only warns", func(t *testing.T) {
		t.Parallel()

		called := false
		svc := &mock.IssueService{
			CreateFn: func(ctx context.Context, issue *jira4claude.Issue) (*jira4claude.Issue, error) {
				called = true
				return &jira4claude.Issue{Key: "TEST-1"}, nil
			},
		}
		printer := &mock.Printer{}
		ctx := &main.IssueContext{
			Service:   svc,
			Printer:   printer,
			Converter: degradedConverter(),
			Config:    &jira4claude.Config{Project: "TEST"},
		}
		cmd := main.IssueCreateCmd{Summary: "s", Description: "text :smile:"}

		err := cmd.Run(ctx)

		require.NoError(t, err)
		assert.True(t, called)
		require.Len(t, printer.ConversionIssueCalls, 1)
		assert.Equal(t, jira4claude.ConversionDegraded, printer.ConversionIssueCalls[0].Kind)
	})
}

func TestIssueUpdateCmd_DroppedContent(t *testing.T) {
	t.Parallel()

	t.Run("refuses an append that drops an image and makes no service call", func(t *testing.T) {
		t.Parallel()

		// Zero-value mock panics on any call, including the Get for the append.
		printer := &mock.Printer{}
		ctx := &main.IssueContext{
			Service:   &mock.IssueService{},
			Printer:   printer,
			Converter: markdown.New(),
			Config:    &jira4claude.Config{Project: "TEST"},
		}
		text := imageMarkdown
		cmd := main.IssueUpdateCmd{Key: "TEST-1", AppendDescription: &text}

		err := cmd.Run(ctx)

		assertDroppedImageRefusal(t, err)
		assert.Empty(t, printer.SuccessCalls)
	})

	t.Run("refuses a description that drops an image and makes no service call", func(t *testing.T) {
		t.Parallel()

		called := false
		svc := &mock.IssueService{
			UpdateFn: func(ctx context.Context, key string, update jira4claude.IssueUpdate) (*jira4claude.Issue, error) {
				called = true
				return &jira4claude.Issue{Key: key}, nil
			},
		}
		userCalled := false
		userSvc := &mock.UserService{
			GetMyselfFn: func(ctx context.Context) (*jira4claude.User, error) {
				userCalled = true
				return &jira4claude.User{AccountID: "me-id"}, nil
			},
		}
		printer := &mock.Printer{}
		ctx := &main.IssueContext{
			Service:     svc,
			UserService: userSvc,
			Printer:     printer,
			Converter:   markdown.New(),
			Config:      &jira4claude.Config{Project: "TEST"},
		}
		desc := imageMarkdown
		assignee := "me"
		cmd := main.IssueUpdateCmd{Key: "TEST-1", Description: &desc, Assignee: &assignee}

		err := cmd.Run(ctx)

		assertDroppedImageRefusal(t, err)
		assert.False(t, called, "Update must not be called")
		assert.False(t, userCalled, "assignee must not be resolved before the refusal")
		assert.Empty(t, printer.SuccessCalls)
	})

	t.Run("--force updates the issue and warns about the dropped image", func(t *testing.T) {
		t.Parallel()

		called := false
		svc := &mock.IssueService{
			UpdateFn: func(ctx context.Context, key string, update jira4claude.IssueUpdate) (*jira4claude.Issue, error) {
				called = true
				return &jira4claude.Issue{Key: key}, nil
			},
		}
		printer := &mock.Printer{}
		ctx := &main.IssueContext{
			Service:   svc,
			Printer:   printer,
			Converter: markdown.New(),
			Config:    &jira4claude.Config{Project: "TEST"},
		}
		desc := imageMarkdown
		cmd := main.IssueUpdateCmd{Key: "TEST-1", Description: &desc, Force: true}

		err := cmd.Run(ctx)

		require.NoError(t, err)
		assert.True(t, called, "Update must be called with --force")
		assertDroppedImageWarned(t, printer)
	})

	t.Run("degraded content only warns", func(t *testing.T) {
		t.Parallel()

		called := false
		svc := &mock.IssueService{
			UpdateFn: func(ctx context.Context, key string, update jira4claude.IssueUpdate) (*jira4claude.Issue, error) {
				called = true
				return &jira4claude.Issue{Key: key}, nil
			},
		}
		printer := &mock.Printer{}
		ctx := &main.IssueContext{
			Service:   svc,
			Printer:   printer,
			Converter: degradedConverter(),
			Config:    &jira4claude.Config{Project: "TEST"},
		}
		desc := "text :smile:"
		cmd := main.IssueUpdateCmd{Key: "TEST-1", Description: &desc}

		err := cmd.Run(ctx)

		require.NoError(t, err)
		assert.True(t, called)
		require.Len(t, printer.ConversionIssueCalls, 1)
		assert.Equal(t, jira4claude.ConversionDegraded, printer.ConversionIssueCalls[0].Kind)
	})
}

func TestIssueCommentCmd_DroppedContent(t *testing.T) {
	t.Parallel()

	newComment := func(body *jira4claude.ADFNode) *jira4claude.Comment {
		return &jira4claude.Comment{
			ID:      "12345",
			Body:    body,
			Created: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		}
	}

	t.Run("refuses a body that drops an image and makes no service call", func(t *testing.T) {
		t.Parallel()

		called := false
		svc := &mock.IssueService{
			AddCommentFn: func(ctx context.Context, key string, body *jira4claude.ADFNode) (*jira4claude.Comment, error) {
				called = true
				return newComment(body), nil
			},
		}
		printer := &mock.Printer{}
		ctx := &main.IssueContext{
			Service:   svc,
			Printer:   printer,
			Converter: markdown.New(),
			Config:    &jira4claude.Config{Project: "TEST"},
		}
		cmd := main.IssueCommentCmd{Key: "TEST-1", Body: imageMarkdown}

		err := cmd.Run(ctx)

		assertDroppedImageRefusal(t, err)
		assert.False(t, called, "AddComment must not be called")
		assert.Empty(t, printer.SuccessCalls)
	})

	t.Run("--force adds the comment and warns about the dropped image", func(t *testing.T) {
		t.Parallel()

		called := false
		svc := &mock.IssueService{
			AddCommentFn: func(ctx context.Context, key string, body *jira4claude.ADFNode) (*jira4claude.Comment, error) {
				called = true
				return newComment(body), nil
			},
		}
		printer := &mock.Printer{}
		ctx := &main.IssueContext{
			Service:   svc,
			Printer:   printer,
			Converter: markdown.New(),
			Config:    &jira4claude.Config{Project: "TEST"},
		}
		cmd := main.IssueCommentCmd{Key: "TEST-1", Body: imageMarkdown, Force: true}

		err := cmd.Run(ctx)

		require.NoError(t, err)
		assert.True(t, called, "AddComment must be called with --force")
		assertDroppedImageWarned(t, printer)
	})

	t.Run("degraded content only warns", func(t *testing.T) {
		t.Parallel()

		called := false
		svc := &mock.IssueService{
			AddCommentFn: func(ctx context.Context, key string, body *jira4claude.ADFNode) (*jira4claude.Comment, error) {
				called = true
				return newComment(body), nil
			},
		}
		printer := &mock.Printer{}
		ctx := &main.IssueContext{
			Service:   svc,
			Printer:   printer,
			Converter: degradedConverter(),
			Config:    &jira4claude.Config{Project: "TEST"},
		}
		cmd := main.IssueCommentCmd{Key: "TEST-1", Body: "text :smile:"}

		err := cmd.Run(ctx)

		require.NoError(t, err)
		assert.True(t, called)
		require.Len(t, printer.ConversionIssueCalls, 1)
		assert.Equal(t, jira4claude.ConversionDegraded, printer.ConversionIssueCalls[0].Kind)
	})

	t.Run("nested details are flattened, warned about and still written", func(t *testing.T) {
		t.Parallel()

		var written *jira4claude.ADFNode
		svc := &mock.IssueService{
			AddCommentFn: func(ctx context.Context, key string, body *jira4claude.ADFNode) (*jira4claude.Comment, error) {
				written = body
				return newComment(body), nil
			},
		}
		printer := &mock.Printer{}
		ctx := &main.IssueContext{
			Service:   svc,
			Printer:   printer,
			Converter: markdown.New(),
			Config:    &jira4claude.Config{Project: "TEST"},
		}
		body := "<details><summary>Outer</summary>\n\nbody\n\n" +
			"<details><summary>Inner</summary>\n\ninner\n\n</details>\n\n" +
			"tail\n\n</details>\n\nafter"
		cmd := main.IssueCommentCmd{Key: "TEST-1", Body: body}

		err := cmd.Run(ctx)

		require.NoError(t, err)
		require.NotNil(t, written, "AddComment must be called")
		require.Len(t, written.Content, 2)
		assert.Equal(t, "expand", written.Content[0].Type)
		require.Len(t, printer.ConversionIssueCalls, 1)
		assert.Equal(t, jira4claude.ConversionDegraded, printer.ConversionIssueCalls[0].Kind)
		assert.Equal(t, "details", printer.ConversionIssueCalls[0].Node)
	})
}

func TestIssueViewCmd_UnsupportedNodesWarn(t *testing.T) {
	t.Parallel()

	svc := &mock.IssueService{
		GetFn: func(ctx context.Context, key string) (*jira4claude.Issue, error) {
			return &jira4claude.Issue{
				Key:     key,
				Summary: "s",
				Status:  "To Do",
				Type:    "Task",
				Description: &jira4claude.ADFNode{
					Type:    "doc",
					Version: 1,
					Content: []jira4claude.ADFNode{
						{Type: "paragraph", Content: []jira4claude.ADFNode{{Type: "text", Text: "Before"}}},
						{Type: "extension", Attrs: json.RawMessage(`{"extensionKey":"macro"}`)},
					},
				},
			}, nil
		},
	}
	printer := &mock.Printer{}
	ctx := &main.IssueContext{
		Service:   svc,
		Printer:   printer,
		Converter: markdown.New(),
		Config:    &jira4claude.Config{Project: "TEST"},
	}
	cmd := main.IssueViewCmd{Key: "TEST-1"}

	err := cmd.Run(ctx)

	require.NoError(t, err)
	require.Len(t, printer.IssueCalls, 1)
	assert.Equal(t, "Before", printer.IssueCalls[0].Description)
	require.Len(t, printer.ConversionIssueCalls, 1)
	assert.Equal(t, jira4claude.ConversionIssue{
		Kind:   jira4claude.ConversionDropped,
		Node:   "extension",
		Detail: "skipped unsupported node type 'extension'",
	}, printer.ConversionIssueCalls[0])
}

func TestWriteCommands_KongForceFlag(t *testing.T) {
	t.Parallel()

	t.Run("create accepts --force", func(t *testing.T) {
		t.Parallel()

		var cli main.CLI
		parser, err := kong.New(&cli)
		require.NoError(t, err)

		_, err = parser.Parse([]string{"issue", "create", "--summary=s", "--description=x", "--force"})
		require.NoError(t, err)
		assert.True(t, cli.Issue.Create.Force)
	})

	t.Run("update accepts --force", func(t *testing.T) {
		t.Parallel()

		var cli main.CLI
		parser, err := kong.New(&cli)
		require.NoError(t, err)

		_, err = parser.Parse([]string{"issue", "update", "PROJ-1", "--description=x", "--force"})
		require.NoError(t, err)
		assert.True(t, cli.Issue.Update.Force)
	})

	t.Run("comment accepts --force", func(t *testing.T) {
		t.Parallel()

		var cli main.CLI
		parser, err := kong.New(&cli)
		require.NoError(t, err)

		_, err = parser.Parse([]string{"issue", "comment", "PROJ-1", "--body=x", "--force"})
		require.NoError(t, err)
		assert.True(t, cli.Issue.Comment.Force)
	})

	t.Run("force defaults to false", func(t *testing.T) {
		t.Parallel()

		var cli main.CLI
		parser, err := kong.New(&cli)
		require.NoError(t, err)

		_, err = parser.Parse([]string{"issue", "comment", "PROJ-1", "--body=x"})
		require.NoError(t, err)
		assert.False(t, cli.Issue.Comment.Force)
	})
}
