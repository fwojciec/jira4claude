package main_test

import (
	"testing"
	"time"

	"github.com/fwojciec/jira4claude"
	"github.com/fwojciec/jira4claude/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --timeout reaches the services through the config they are built from.
func TestTimeoutFlag(t *testing.T) {
	t.Parallel()

	// timeoutSeen runs args and returns the timeout the services were built
	// with.
	timeoutSeen := func(t *testing.T, args ...string) (time.Duration, runResult) {
		t.Helper()
		var seen *time.Duration
		svc := &mock.IssueService{}
		deps := testDeps(svc, &mock.UserService{})
		deps.NewServices = func(cfg *jira4claude.Config) (jira4claude.IssueService, jira4claude.UserService, error) {
			seen = &cfg.Timeout
			return nil, nil, &jira4claude.Error{Code: jira4claude.EInternal, Message: "stop here"}
		}
		deps.NewAttachmentService = func(cfg *jira4claude.Config) (jira4claude.AttachmentService, error) {
			seen = &cfg.Timeout
			return nil, &jira4claude.Error{Code: jira4claude.EInternal, Message: "stop here"}
		}
		res := runCLI(t, deps, args...)
		if seen == nil {
			return -1, res
		}
		return *seen, res
	}

	tests := []struct {
		name string
		args []string
		want time.Duration
	}{
		{"defaults to 30 seconds", []string{"issue", "view", "TEST-1"}, 30 * time.Second},
		{"is given before the command", []string{"--timeout", "2m", "issue", "view", "TEST-1"}, 2 * time.Minute},
		{"is given after the command", []string{"issue", "view", "TEST-1", "--timeout=500ms"}, 500 * time.Millisecond},
		{"0 means no timeout", []string{"--timeout", "0", "issue", "view", "TEST-1"}, 0},
		{"reaches attachment commands", []string{"--timeout", "5s", "attachment", "list", "TEST-1"}, 5 * time.Second},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, res := timeoutSeen(t, tt.args...)
			assert.Equal(t, tt.want, got, res.stderr)
		})
	}

	t.Run("a negative timeout is a usage error", func(t *testing.T) {
		t.Parallel()
		got, res := timeoutSeen(t, "--timeout", "-1s", "issue", "view", "TEST-1")
		assert.Equal(t, time.Duration(-1), got, "no service is built")
		assert.Equal(t, 1, res.code)
		assert.Empty(t, res.stdout)
		assert.Contains(t, res.stderr, "--timeout")

		got, res = timeoutSeen(t, "--timeout=-1s", "issue", "view", "TEST-1")
		assert.Equal(t, time.Duration(-1), got, "no service is built")
		assert.Equal(t, 1, res.code)
		assert.Contains(t, res.stderr, "--timeout cannot be negative")
	})

	t.Run("a value without a unit is a usage error", func(t *testing.T) {
		t.Parallel()
		_, res := timeoutSeen(t, "--timeout", "30", "issue", "view", "TEST-1")
		require.Equal(t, 1, res.code)
		assert.Contains(t, res.stderr, "--timeout")
	})
}
