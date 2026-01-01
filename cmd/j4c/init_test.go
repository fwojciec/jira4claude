package main_test

import (
	"bytes"
	"testing"

	"github.com/fwojciec/jira4claude"
	main "github.com/fwojciec/jira4claude/cmd/j4c"
	"github.com/fwojciec/jira4claude/gogh"
	"github.com/fwojciec/jira4claude/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func makeConfigContext(t *testing.T, svc jira4claude.ConfigService, out *bytes.Buffer) *main.ConfigContext {
	t.Helper()
	io := gogh.NewIO(out, out)
	printer := gogh.NewTextPrinter(io)
	return &main.ConfigContext{
		Service: svc,
		Printer: printer,
	}
}

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

		var buf bytes.Buffer
		ctx := makeConfigContext(t, svc, &buf)
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

		var buf bytes.Buffer
		ctx := makeConfigContext(t, svc, &buf)
		cmd := main.InitCmd{Server: "https://example.atlassian.net", Project: "TEST"}

		err := cmd.Run(ctx)

		require.NoError(t, err)
		assert.Contains(t, buf.String(), "Created .jira4claude.yaml")
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

		var buf bytes.Buffer
		ctx := makeConfigContext(t, svc, &buf)
		cmd := main.InitCmd{Server: "https://example.atlassian.net", Project: "TEST"}

		err := cmd.Run(ctx)

		require.NoError(t, err)
		assert.Contains(t, buf.String(), "Added .jira4claude.yaml to .gitignore")
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

		var buf bytes.Buffer
		ctx := makeConfigContext(t, svc, &buf)
		cmd := main.InitCmd{Server: "https://example.atlassian.net", Project: "TEST"}

		err := cmd.Run(ctx)

		require.NoError(t, err)
		assert.Contains(t, buf.String(), ".jira4claude.yaml already in .gitignore")
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

		var buf bytes.Buffer
		ctx := makeConfigContext(t, svc, &buf)
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

		var buf bytes.Buffer
		ctx := makeConfigContext(t, svc, &buf)
		cmd := main.InitCmd{Server: "https://example.atlassian.net", Project: "TEST"}

		err := cmd.Run(ctx)

		require.Error(t, err)
		assert.Equal(t, jira4claude.EInternal, jira4claude.ErrorCode(err))
	})
}
