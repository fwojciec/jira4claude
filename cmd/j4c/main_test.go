package main_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/alecthomas/kong"
	"github.com/fwojciec/jira4claude"
	main "github.com/fwojciec/jira4claude/cmd/j4c"
	"github.com/fwojciec/jira4claude/mock"
	"github.com/fwojciec/jira4claude/yaml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// InitCmd tests

func TestInitCmd_RequiredFlags(t *testing.T) {
	t.Parallel()

	t.Run("fails without required server flag", func(t *testing.T) {
		t.Parallel()

		var cli main.CLI
		parser, err := kong.New(&cli)
		require.NoError(t, err)

		_, err = parser.Parse([]string{"init", "--project=TEST"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "server")
	})

	t.Run("fails without required project flag", func(t *testing.T) {
		t.Parallel()

		var cli main.CLI
		parser, err := kong.New(&cli)
		require.NoError(t, err)

		_, err = parser.Parse([]string{"init", "--server=https://test.atlassian.net"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "project")
	})

	t.Run("succeeds with both required flags", func(t *testing.T) {
		t.Parallel()

		var cli main.CLI
		parser, err := kong.New(&cli)
		require.NoError(t, err)

		_, err = parser.Parse([]string{"init", "--server=https://test.atlassian.net", "--project=TEST"})
		require.NoError(t, err)
		assert.Equal(t, "https://test.atlassian.net", cli.Init.Server)
		assert.Equal(t, "TEST", cli.Init.Project)
	})
}

func TestInitCmd_CreatesConfigAndUpdatesGitignore(t *testing.T) {
	t.Parallel()

	t.Run("creates config file and updates gitignore", func(t *testing.T) {
		t.Parallel()

		// Create temp directory
		tmpDir := t.TempDir()
		originalWd, err := os.Getwd()
		require.NoError(t, err)
		require.NoError(t, os.Chdir(tmpDir))
		t.Cleanup(func() { _ = os.Chdir(originalWd) })

		// Set up the config context with real service and mock printer
		printer := &mock.Printer{}
		ctx := &main.ConfigContext{
			Service: yaml.NewService(),
			Printer: printer,
		}

		cmd := main.InitCmd{
			Server:  "https://test.atlassian.net",
			Project: "TEST",
		}
		err = cmd.Run(ctx)

		require.NoError(t, err)

		// Verify config file was created
		configPath := filepath.Join(tmpDir, ".jira4claude.yaml")
		_, err = os.Stat(configPath)
		require.NoError(t, err, "config file should exist")

		// Verify gitignore was updated
		gitignorePath := filepath.Join(tmpDir, ".gitignore")
		content, err := os.ReadFile(gitignorePath)
		require.NoError(t, err)
		assert.Contains(t, string(content), ".jira4claude.yaml")

		// Verify success messages were printed
		require.Len(t, printer.SuccessCalls, 2)
		assert.Equal(t, "Created .jira4claude.yaml", printer.SuccessCalls[0].Msg)
		assert.Equal(t, "Added .jira4claude.yaml to .gitignore", printer.SuccessCalls[1].Msg)
	})
}

// Issue command flag parsing tests

func TestIssueCreateCmd_RequiredFlags(t *testing.T) {
	t.Parallel()

	t.Run("fails without required summary flag", func(t *testing.T) {
		t.Parallel()

		var cli main.CLI
		parser, err := kong.New(&cli)
		require.NoError(t, err)

		_, err = parser.Parse([]string{"issue", "create"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "summary")
	})

	t.Run("succeeds with summary flag", func(t *testing.T) {
		t.Parallel()

		var cli main.CLI
		parser, err := kong.New(&cli)
		require.NoError(t, err)

		_, err = parser.Parse([]string{"issue", "create", "--summary=Test issue"})
		require.NoError(t, err)
		assert.Equal(t, "Test issue", cli.Issue.Create.Summary)
	})
}

func TestIssueCommentCmd_RequiredFlags(t *testing.T) {
	t.Parallel()

	t.Run("fails without required body flag", func(t *testing.T) {
		t.Parallel()

		var cli main.CLI
		parser, err := kong.New(&cli)
		require.NoError(t, err)

		_, err = parser.Parse([]string{"issue", "comment", "TEST-1"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "body")
	})

	t.Run("succeeds with body flag", func(t *testing.T) {
		t.Parallel()

		var cli main.CLI
		parser, err := kong.New(&cli)
		require.NoError(t, err)

		_, err = parser.Parse([]string{"issue", "comment", "TEST-1", "--body=A comment"})
		require.NoError(t, err)
		assert.Equal(t, "TEST-1", cli.Issue.Comment.Key)
		assert.Equal(t, "A comment", cli.Issue.Comment.Body)
	})
}

func TestIssueListCmd_DefaultLimit(t *testing.T) {
	t.Parallel()

	t.Run("uses default limit of 50", func(t *testing.T) {
		t.Parallel()

		var cli main.CLI
		parser, err := kong.New(&cli)
		require.NoError(t, err)

		_, err = parser.Parse([]string{"issue", "list"})
		require.NoError(t, err)
		assert.Equal(t, 50, cli.Issue.List.Limit)
	})

	t.Run("overrides default limit", func(t *testing.T) {
		t.Parallel()

		var cli main.CLI
		parser, err := kong.New(&cli)
		require.NoError(t, err)

		_, err = parser.Parse([]string{"issue", "list", "--limit=100"})
		require.NoError(t, err)
		assert.Equal(t, 100, cli.Issue.List.Limit)
	})
}

func TestIssueCreateCmd_DefaultType(t *testing.T) {
	t.Parallel()

	t.Run("uses default type of Task", func(t *testing.T) {
		t.Parallel()

		var cli main.CLI
		parser, err := kong.New(&cli)
		require.NoError(t, err)

		_, err = parser.Parse([]string{"issue", "create", "--summary=Test"})
		require.NoError(t, err)
		assert.Equal(t, "Task", cli.Issue.Create.Type)
	})

	t.Run("overrides default type", func(t *testing.T) {
		t.Parallel()

		var cli main.CLI
		parser, err := kong.New(&cli)
		require.NoError(t, err)

		_, err = parser.Parse([]string{"issue", "create", "--summary=Test", "--type=Bug"})
		require.NoError(t, err)
		assert.Equal(t, "Bug", cli.Issue.Create.Type)
	})
}

// Error propagation tests

func TestIssueViewCmd_ReturnsServiceError(t *testing.T) {
	t.Parallel()

	t.Run("returns error from service", func(t *testing.T) {
		t.Parallel()

		svc := &mock.IssueService{
			GetFn: func(ctx context.Context, key string) (*jira4claude.Issue, error) {
				return nil, errors.New("issue not found")
			},
		}
		printer := &mock.Printer{}
		issueCtx := &main.IssueContext{
			Service: svc,
			Printer: printer,
		}

		cmd := main.IssueViewCmd{Key: "TEST-999"}
		err := cmd.Run(issueCtx)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "issue not found")
		// Verify printer.Error was NOT called - errors are returned, not printed
		assert.Empty(t, printer.ErrorCalls)
	})
}

func TestIssueCreateCmd_ReturnsServiceError(t *testing.T) {
	t.Parallel()

	t.Run("returns error from service", func(t *testing.T) {
		t.Parallel()

		svc := &mock.IssueService{
			CreateFn: func(ctx context.Context, issue *jira4claude.Issue) (*jira4claude.Issue, error) {
				return nil, errors.New("create failed")
			},
		}
		printer := &mock.Printer{}
		issueCtx := &main.IssueContext{
			Service: svc,
			Printer: printer,
			Config:  &jira4claude.Config{Project: "TEST"},
		}

		cmd := main.IssueCreateCmd{Summary: "Test issue"}
		err := cmd.Run(issueCtx)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "create failed")
		assert.Empty(t, printer.ErrorCalls)
	})
}

func TestIssueTransitionCmd_RequiresStatusOrID(t *testing.T) {
	t.Parallel()

	t.Run("returns validation error when neither status nor ID provided", func(t *testing.T) {
		t.Parallel()

		svc := &mock.IssueService{
			TransitionsFn: func(ctx context.Context, key string) ([]*jira4claude.Transition, error) {
				return []*jira4claude.Transition{}, nil
			},
		}
		printer := &mock.Printer{}
		issueCtx := &main.IssueContext{
			Service: svc,
			Printer: printer,
		}

		cmd := main.IssueTransitionCmd{Key: "TEST-1"}
		err := cmd.Run(issueCtx)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "either --status or --id is required")
	})
}

// Link command tests

func TestLinkCreateCmd_ReturnsServiceError(t *testing.T) {
	t.Parallel()

	t.Run("returns error from service", func(t *testing.T) {
		t.Parallel()

		svc := &mock.IssueService{
			LinkFn: func(ctx context.Context, inward, linkType, outward string) error {
				return errors.New("link failed")
			},
		}
		printer := &mock.Printer{}
		linkCtx := &main.LinkContext{
			Service: svc,
			Printer: printer,
		}

		cmd := main.LinkCreateCmd{
			InwardKey:  "TEST-1",
			LinkType:   "Blocks",
			OutwardKey: "TEST-2",
		}
		err := cmd.Run(linkCtx)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "link failed")
		assert.Empty(t, printer.ErrorCalls)
	})
}

func TestLinkDeleteCmd_ReturnsServiceError(t *testing.T) {
	t.Parallel()

	t.Run("returns error from service", func(t *testing.T) {
		t.Parallel()

		svc := &mock.IssueService{
			UnlinkFn: func(ctx context.Context, key1, key2 string) error {
				return errors.New("unlink failed")
			},
		}
		printer := &mock.Printer{}
		linkCtx := &main.LinkContext{
			Service: svc,
			Printer: printer,
		}

		cmd := main.LinkDeleteCmd{
			Key1: "TEST-1",
			Key2: "TEST-2",
		}
		err := cmd.Run(linkCtx)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "unlink failed")
		assert.Empty(t, printer.ErrorCalls)
	})
}
