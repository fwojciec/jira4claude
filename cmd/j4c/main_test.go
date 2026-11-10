package main_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
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

// Run entry point tests
//
// These drive the whole CLI through main.Run and pin what goes to stdout, what
// goes to stderr and which exit code is returned. Some of the pinned behaviour
// is deliberately odd (errors on stdout in --json mode, usage on stdout with
// exit 80); it is pinned so that changing it later is a visible decision.

type runResult struct {
	stdout string
	stderr string
	code   int
}

func runCLI(t *testing.T, deps main.Deps, args ...string) runResult {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := main.Run(args, strings.NewReader(""), &stdout, &stderr, deps)
	return runResult{stdout: stdout.String(), stderr: stderr.String(), code: code}
}

// testDeps returns deps whose config loader and service factory succeed and
// hand back the given services.
func testDeps(issueSvc jira4claude.IssueService, userSvc jira4claude.UserService) main.Deps {
	return main.Deps{
		Version: "test-version",
		LoadConfig: func(path string) (*jira4claude.Config, error) {
			return &jira4claude.Config{Server: "https://test.atlassian.net", Project: "TEST"}, nil
		},
		NewServices: func(cfg *jira4claude.Config) (jira4claude.IssueService, jira4claude.UserService, error) {
			return issueSvc, userSvc, nil
		},
	}
}

// unreachableDeps fails the test if any dependency is touched.
func unreachableDeps(t *testing.T) main.Deps {
	t.Helper()
	return main.Deps{
		Version: "test-version",
		LoadConfig: func(path string) (*jira4claude.Config, error) {
			t.Error("LoadConfig must not be called")
			return nil, errors.New("unexpected LoadConfig call")
		},
		NewServices: func(cfg *jira4claude.Config) (jira4claude.IssueService, jira4claude.UserService, error) {
			t.Error("NewServices must not be called")
			return nil, nil, errors.New("unexpected NewServices call")
		},
		ConfigService: &mock.ConfigService{
			InitFn: func(dir, server, project string) (*jira4claude.InitResult, error) {
				t.Error("ConfigService.Init must not be called")
				return nil, errors.New("unexpected Init call")
			},
		},
	}
}

func issueGetter(issue *jira4claude.Issue, err error) *mock.IssueService {
	return &mock.IssueService{
		GetFn: func(ctx context.Context, key string) (*jira4claude.Issue, error) {
			return issue, err
		},
	}
}

func TestRun_IssueViewSuccess(t *testing.T) {
	t.Parallel()

	newIssue := func() *jira4claude.Issue {
		return &jira4claude.Issue{
			Key:     "TEST-1",
			Summary: "First issue",
			Status:  "To Do",
			Type:    "Task",
		}
	}

	t.Run("markdown output goes to stdout with exit 0", func(t *testing.T) {
		t.Parallel()

		var gotKey string
		svc := &mock.IssueService{
			GetFn: func(ctx context.Context, key string) (*jira4claude.Issue, error) {
				gotKey = key
				return newIssue(), nil
			},
		}

		res := runCLI(t, testDeps(svc, &mock.UserService{}), "issue", "view", "TEST-1")

		assert.Equal(t, 0, res.code)
		assert.Equal(t, "TEST-1", gotKey)
		assert.True(t, strings.HasPrefix(res.stdout, "# TEST-1: First issue\n"), "stdout: %q", res.stdout)
		assert.Contains(t, res.stdout, "**Status:** To Do")
		assert.Contains(t, res.stdout, "https://test.atlassian.net/browse/TEST-1")
		assert.Empty(t, res.stderr)
	})

	t.Run("JSON output goes to stdout with exit 0", func(t *testing.T) {
		t.Parallel()

		res := runCLI(t, testDeps(issueGetter(newIssue(), nil), &mock.UserService{}), "--json", "issue", "view", "TEST-1")

		assert.Equal(t, 0, res.code)
		assert.Empty(t, res.stderr)
		var got map[string]any
		require.NoError(t, json.Unmarshal([]byte(res.stdout), &got))
		assert.Equal(t, "TEST-1", got["key"])
		assert.Equal(t, "First issue", got["summary"])
		assert.Equal(t, "https://test.atlassian.net/browse/TEST-1", got["url"])
	})
}

func TestRun_NotFoundError(t *testing.T) {
	t.Parallel()

	notFound := &jira4claude.Error{Code: jira4claude.ENotFound, Message: "issue not found"}

	t.Run("markdown mode prints the error to stderr and exits 4", func(t *testing.T) {
		t.Parallel()

		res := runCLI(t, testDeps(issueGetter(nil, notFound), &mock.UserService{}), "issue", "view", "TEST-999")

		assert.Equal(t, 4, res.code)
		assert.Empty(t, res.stdout)
		assert.Equal(t, "[error] issue not found\n", res.stderr)
	})

	// Pins current behaviour: in --json mode errors are written to stdout.
	t.Run("JSON mode prints the error to stdout and exits 4", func(t *testing.T) {
		t.Parallel()

		res := runCLI(t, testDeps(issueGetter(nil, notFound), &mock.UserService{}), "--json", "issue", "view", "TEST-999")

		assert.Equal(t, 4, res.code)
		assert.Empty(t, res.stderr)
		var got map[string]any
		require.NoError(t, json.Unmarshal([]byte(res.stdout), &got))
		assert.Equal(t, map[string]any{"error": true, "code": "not_found", "message": "issue not found"}, got)
	})
}

func TestRun_ValidationError(t *testing.T) {
	t.Parallel()

	t.Run("markdown mode prints the error to stderr and exits 1", func(t *testing.T) {
		t.Parallel()

		res := runCLI(t, testDeps(&mock.IssueService{}, &mock.UserService{}), "issue", "transition", "TEST-1")

		assert.Equal(t, 1, res.code)
		assert.Empty(t, res.stdout)
		assert.True(t, strings.HasPrefix(res.stderr, "[error] "), "stderr: %q", res.stderr)
		assert.Contains(t, res.stderr, "either --status or --id is required")
	})

	// Pins current behaviour: in --json mode errors are written to stdout.
	t.Run("JSON mode prints the error to stdout and exits 1", func(t *testing.T) {
		t.Parallel()

		res := runCLI(t, testDeps(&mock.IssueService{}, &mock.UserService{}), "--json", "issue", "transition", "TEST-1")

		assert.Equal(t, 1, res.code)
		assert.Empty(t, res.stderr)
		var got map[string]any
		require.NoError(t, json.Unmarshal([]byte(res.stdout), &got))
		assert.Equal(t, true, got["error"])
		assert.Equal(t, "validation", got["code"])
		assert.Contains(t, got["message"], "either --status or --id is required")
	})
}

// TestRun_UsageError pins kong's current usage-error behaviour: the usage text
// goes to stdout, the error line goes to stderr and the exit code is 80. A later
// milestone changes this to exit 1 with usage on stderr.
func TestRun_UsageError(t *testing.T) {
	t.Parallel()

	t.Run("missing argument prints usage to stdout and exits 80", func(t *testing.T) {
		t.Parallel()

		res := runCLI(t, unreachableDeps(t), "issue", "view")

		assert.Equal(t, 80, res.code)
		assert.True(t, strings.HasPrefix(res.stdout, "Usage: j4c issue view <key>"), "stdout: %q", res.stdout)
		assert.Equal(t, "j4c: error: expected \"<key>\"\n", res.stderr)
	})

	t.Run("JSON mode does not change usage error output", func(t *testing.T) {
		t.Parallel()

		res := runCLI(t, unreachableDeps(t), "--json", "issue", "view")

		assert.Equal(t, 80, res.code)
		assert.True(t, strings.HasPrefix(res.stdout, "Usage: j4c issue view <key>"), "stdout: %q", res.stdout)
		assert.Equal(t, "j4c: error: expected \"<key>\"\n", res.stderr)
	})

	t.Run("no command prints full usage to stdout and exits 80", func(t *testing.T) {
		t.Parallel()

		res := runCLI(t, unreachableDeps(t))

		assert.Equal(t, 80, res.code)
		assert.True(t, strings.HasPrefix(res.stdout, "Usage: j4c <command> [flags]"), "stdout: %q", res.stdout)
		assert.Equal(t, "j4c: error: expected one of \"issue\", \"link\", \"init\"\n", res.stderr)
	})

	t.Run("mutually exclusive flags exit 80", func(t *testing.T) {
		t.Parallel()

		res := runCLI(t, unreachableDeps(t), "issue", "transition", "TEST-1", "--status=Done", "--id=31")

		assert.Equal(t, 80, res.code)
		assert.Contains(t, res.stdout, "Usage: j4c issue transition <key>")
		assert.Equal(t, "j4c: error: --status and --id can't be used together\n", res.stderr)
	})
}

func TestRun_HelpAndVersion(t *testing.T) {
	t.Parallel()

	t.Run("help prints to stdout and exits 0", func(t *testing.T) {
		t.Parallel()

		res := runCLI(t, unreachableDeps(t), "--help")

		assert.Equal(t, 0, res.code)
		assert.True(t, strings.HasPrefix(res.stdout, "Usage: j4c <command> [flags]"), "stdout: %q", res.stdout)
		assert.Contains(t, res.stdout, "A minimal Jira CLI for AI agents")
		assert.Empty(t, res.stderr)
	})

	t.Run("subcommand help prints to stdout and exits 0", func(t *testing.T) {
		t.Parallel()

		res := runCLI(t, unreachableDeps(t), "issue", "create", "--help")

		assert.Equal(t, 0, res.code)
		assert.True(t, strings.HasPrefix(res.stdout, "Usage: j4c issue create --summary=STRING [flags]"), "stdout: %q", res.stdout)
		assert.Empty(t, res.stderr)
	})

	t.Run("version prints to stdout and exits 0", func(t *testing.T) {
		t.Parallel()

		res := runCLI(t, unreachableDeps(t), "--version")

		assert.Equal(t, 0, res.code)
		assert.Equal(t, "test-version\n", res.stdout)
		assert.Empty(t, res.stderr)
	})
}

func TestRun_ConfigLoading(t *testing.T) {
	t.Parallel()

	t.Run("passes --config path to the loader", func(t *testing.T) {
		t.Parallel()

		configPath := filepath.Join(t.TempDir(), "custom.yaml")
		var gotPath string
		deps := testDeps(issueGetter(&jira4claude.Issue{Key: "TEST-1"}, nil), &mock.UserService{})
		deps.LoadConfig = func(path string) (*jira4claude.Config, error) {
			gotPath = path
			return &jira4claude.Config{Server: "https://test.atlassian.net"}, nil
		}

		res := runCLI(t, deps, "--config", configPath, "issue", "view", "TEST-1")

		assert.Equal(t, 0, res.code)
		assert.Equal(t, configPath, gotPath)
	})

	t.Run("passes loaded config to the service factory", func(t *testing.T) {
		t.Parallel()

		var gotServer string
		deps := testDeps(nil, nil)
		deps.NewServices = func(cfg *jira4claude.Config) (jira4claude.IssueService, jira4claude.UserService, error) {
			gotServer = cfg.Server
			return issueGetter(&jira4claude.Issue{Key: "TEST-1"}, nil), &mock.UserService{}, nil
		}

		res := runCLI(t, deps, "issue", "view", "TEST-1")

		assert.Equal(t, 0, res.code)
		assert.Equal(t, "https://test.atlassian.net", gotServer)
	})

	t.Run("config error is printed to stderr and its exit code returned", func(t *testing.T) {
		t.Parallel()

		deps := unreachableDeps(t)
		deps.LoadConfig = func(path string) (*jira4claude.Config, error) {
			return nil, &jira4claude.Error{Code: jira4claude.ENotFound, Message: "no config file found"}
		}

		res := runCLI(t, deps, "issue", "view", "TEST-1")

		assert.Equal(t, 4, res.code)
		assert.Empty(t, res.stdout)
		assert.Equal(t, "[error] no config file found\n", res.stderr)
	})

	// Pins current behaviour: in --json mode errors are written to stdout.
	t.Run("config error in JSON mode is printed to stdout", func(t *testing.T) {
		t.Parallel()

		deps := unreachableDeps(t)
		deps.LoadConfig = func(path string) (*jira4claude.Config, error) {
			return nil, &jira4claude.Error{Code: jira4claude.ENotFound, Message: "no config file found"}
		}

		res := runCLI(t, deps, "--json", "issue", "view", "TEST-1")

		assert.Equal(t, 4, res.code)
		assert.Empty(t, res.stderr)
		var got map[string]any
		require.NoError(t, json.Unmarshal([]byte(res.stdout), &got))
		assert.Equal(t, "not_found", got["code"])
	})

	t.Run("service factory error is printed and its exit code returned", func(t *testing.T) {
		t.Parallel()

		deps := testDeps(nil, nil)
		deps.NewServices = func(cfg *jira4claude.Config) (jira4claude.IssueService, jira4claude.UserService, error) {
			return nil, nil, &jira4claude.Error{Code: jira4claude.EUnauthorized, Message: "could not read netrc file"}
		}

		res := runCLI(t, deps, "issue", "view", "TEST-1")

		assert.Equal(t, 2, res.code)
		assert.Empty(t, res.stdout)
		assert.Equal(t, "[error] could not read netrc file\n", res.stderr)
	})
}

func TestRun_Init(t *testing.T) {
	t.Parallel()

	t.Run("uses the config service without loading config", func(t *testing.T) {
		t.Parallel()

		var gotServer, gotProject string
		deps := unreachableDeps(t)
		deps.ConfigService = &mock.ConfigService{
			InitFn: func(dir, server, project string) (*jira4claude.InitResult, error) {
				gotServer, gotProject = server, project
				return &jira4claude.InitResult{ConfigCreated: true}, nil
			},
		}

		res := runCLI(t, deps, "init", "--server=https://test.atlassian.net", "--project=TEST")

		assert.Equal(t, 0, res.code)
		assert.Equal(t, "https://test.atlassian.net", gotServer)
		assert.Equal(t, "TEST", gotProject)
		assert.Contains(t, res.stdout, "Created .jira4claude.yaml")
		assert.Empty(t, res.stderr)
	})

	t.Run("init error is printed and its exit code returned", func(t *testing.T) {
		t.Parallel()

		deps := unreachableDeps(t)
		deps.ConfigService = &mock.ConfigService{
			InitFn: func(dir, server, project string) (*jira4claude.InitResult, error) {
				return nil, &jira4claude.Error{Code: jira4claude.EConflict, Message: "config exists"}
			},
		}

		res := runCLI(t, deps, "init", "--server=https://test.atlassian.net", "--project=TEST")

		assert.Equal(t, 5, res.code)
		assert.Empty(t, res.stdout)
		assert.Equal(t, "[error] config exists\n", res.stderr)
	})
}

func TestRun_UnrelatedPanicsPropagate(t *testing.T) {
	t.Parallel()

	t.Run("a panic inside a command is not swallowed", func(t *testing.T) {
		t.Parallel()

		svc := &mock.IssueService{
			GetFn: func(ctx context.Context, key string) (*jira4claude.Issue, error) {
				panic("boom")
			},
		}

		assert.PanicsWithValue(t, "boom", func() {
			runCLI(t, testDeps(svc, &mock.UserService{}), "issue", "view", "TEST-1")
		})
	})
}
