package main_test

import (
	"testing"

	"github.com/fwojciec/jira4claude"
	main "github.com/fwojciec/jira4claude/cmd/j4c"
	"github.com/fwojciec/jira4claude/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInitCmd(t *testing.T) {
	t.Parallel()

	t.Run("calls ConfigService.Init with correct arguments", func(t *testing.T) {
		t.Parallel()

		var capturedDir, capturedServer, capturedProject string
		svc := &mock.ConfigService{
			InitFn: func(dir, server, project string) (*jira4claude.InitResult, error) {
				capturedDir = dir
				capturedServer = server
				capturedProject = project
				return &jira4claude.InitResult{ConfigCreated: true}, nil
			},
		}

		printer := &mock.Printer{}
		ctx := &main.ConfigContext{Service: svc, Printer: printer}
		cmd := main.InitCmd{
			Server:  "https://example.atlassian.net",
			Project: "TEST",
		}

		err := cmd.Run(ctx)

		require.NoError(t, err)
		assert.NotEmpty(t, capturedDir) // Should be working directory
		assert.Equal(t, "https://example.atlassian.net", capturedServer)
		assert.Equal(t, "TEST", capturedProject)
	})

	t.Run("prints success when config created", func(t *testing.T) {
		t.Parallel()

		svc := &mock.ConfigService{
			InitFn: func(dir, server, project string) (*jira4claude.InitResult, error) {
				return &jira4claude.InitResult{ConfigCreated: true}, nil
			},
		}

		printer := &mock.Printer{}
		ctx := &main.ConfigContext{Service: svc, Printer: printer}
		cmd := main.InitCmd{Server: "https://example.atlassian.net", Project: "TEST"}

		err := cmd.Run(ctx)

		require.NoError(t, err)
		require.Len(t, printer.SuccessCalls, 1)
		assert.Equal(t, "Created .jira4claude.yaml", printer.SuccessCalls[0].Msg)
	})

	t.Run("prints success when gitignore entry added", func(t *testing.T) {
		t.Parallel()

		svc := &mock.ConfigService{
			InitFn: func(dir, server, project string) (*jira4claude.InitResult, error) {
				return &jira4claude.InitResult{
					ConfigCreated:  true,
					GitignoreAdded: true,
				}, nil
			},
		}

		printer := &mock.Printer{}
		ctx := &main.ConfigContext{Service: svc, Printer: printer}
		cmd := main.InitCmd{Server: "https://example.atlassian.net", Project: "TEST"}

		err := cmd.Run(ctx)

		require.NoError(t, err)
		require.Len(t, printer.SuccessCalls, 2)
		assert.Equal(t, "Created .jira4claude.yaml", printer.SuccessCalls[0].Msg)
		assert.Equal(t, "Added .jira4claude.yaml to .gitignore", printer.SuccessCalls[1].Msg)
	})

	t.Run("prints success when gitignore entry already exists", func(t *testing.T) {
		t.Parallel()

		svc := &mock.ConfigService{
			InitFn: func(dir, server, project string) (*jira4claude.InitResult, error) {
				return &jira4claude.InitResult{
					ConfigCreated:   true,
					GitignoreExists: true,
				}, nil
			},
		}

		printer := &mock.Printer{}
		ctx := &main.ConfigContext{Service: svc, Printer: printer}
		cmd := main.InitCmd{Server: "https://example.atlassian.net", Project: "TEST"}

		err := cmd.Run(ctx)

		require.NoError(t, err)
		require.Len(t, printer.SuccessCalls, 2)
		assert.Equal(t, "Created .jira4claude.yaml", printer.SuccessCalls[0].Msg)
		assert.Equal(t, ".jira4claude.yaml already in .gitignore", printer.SuccessCalls[1].Msg)
	})

	t.Run("returns error when config already exists", func(t *testing.T) {
		t.Parallel()

		svc := &mock.ConfigService{
			InitFn: func(dir, server, project string) (*jira4claude.InitResult, error) {
				return nil, &jira4claude.Error{
					Code:    jira4claude.EValidation,
					Message: ".jira4claude.yaml already exists",
				}
			},
		}

		printer := &mock.Printer{}
		ctx := &main.ConfigContext{Service: svc, Printer: printer}
		cmd := main.InitCmd{Server: "https://example.atlassian.net", Project: "TEST"}

		err := cmd.Run(ctx)

		require.Error(t, err)
		assert.Equal(t, jira4claude.EValidation, jira4claude.ErrorCode(err))
	})

	t.Run("returns error on service failure", func(t *testing.T) {
		t.Parallel()

		svc := &mock.ConfigService{
			InitFn: func(dir, server, project string) (*jira4claude.InitResult, error) {
				return nil, &jira4claude.Error{
					Code:    jira4claude.EInternal,
					Message: "failed to create config file",
				}
			},
		}

		printer := &mock.Printer{}
		ctx := &main.ConfigContext{Service: svc, Printer: printer}
		cmd := main.InitCmd{Server: "https://example.atlassian.net", Project: "TEST"}

		err := cmd.Run(ctx)

		require.Error(t, err)
		assert.Equal(t, jira4claude.EInternal, jira4claude.ErrorCode(err))
	})
}
