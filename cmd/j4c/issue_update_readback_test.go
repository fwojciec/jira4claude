package main_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/fwojciec/jira4claude"
	"github.com/fwojciec/jira4claude/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// An update whose write Jira applied but whose read-back failed still
// prints its result, from the read before the write, without an updated
// time, and fails with the read's code and a message saying the write was
// applied: as issue assign does.
func TestIssueUpdate_ReadBackFails(t *testing.T) {
	t.Parallel()

	readErr := &jira4claude.Error{Code: jira4claude.EForbidden, Message: "no browse permission"}
	newService := func() *guardedService {
		s := newGuardedService(guardedIssue())
		s.UpdateFn = func(ctx context.Context, key string, update jira4claude.IssueUpdate) (*jira4claude.Issue, error) {
			s.record("update " + key)
			return nil, &jira4claude.ReadBackError{Err: readErr}
		}
		return s
	}

	t.Run("text", func(t *testing.T) {
		t.Parallel()
		svc := newService()

		res := runCLI(t, testDeps(svc, &mock.UserService{}), "issue", "update", "TEST-1", "--summary", "New")

		assert.Equal(t, 3, res.code)
		assert.Equal(t, "TEST-1: updated\n  summary: Guarded → New\nhttps://test.atlassian.net/browse/TEST-1\n", res.stdout)
		assert.Equal(t, "[error] updated TEST-1, but reading it back failed; the write was applied, do not repeat it: no browse permission\n", res.stderr)
		assert.Equal(t, []string{"get TEST-1", "update TEST-1"}, svc.Calls())
	})

	t.Run("json", func(t *testing.T) {
		t.Parallel()
		svc := newService()

		res := runCLI(t, testDeps(svc, &mock.UserService{}), "--json", "issue", "update", "TEST-1", "--append-description", "More.")

		assert.Equal(t, 3, res.code)
		var got map[string]any
		require.NoError(t, json.Unmarshal([]byte(res.stdout), &got))
		assert.Equal(t, "updated", got["action"])
		assert.Equal(t, map[string]any{"description": map[string]any{"edit": "appended"}}, got["changes"])
		assert.NotContains(t, got, "updated")
		assert.JSONEq(t, `{"level":"error","error":true,"code":"forbidden","message":"updated TEST-1, but reading it back failed; the write was applied, do not repeat it: no browse permission"}`, res.stderr)
	})
}
