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

// jir-dqbz: a create or comment that Jira applied but whose answer could not
// be read still prints its result, with what is known (no key for a create,
// no comment ID for a comment), and fails with the answer's code and a
// message saying the write was applied: as issue update does after a failed
// read-back.

func unreadableAnswer(what string) error {
	return &jira4claude.UnreadableAnswerError{Err: &jira4claude.Error{
		Code:    jira4claude.EInternal,
		Message: "Jira's answer does not name the new " + what,
	}}
}

func TestIssueCreate_AnswerUnreadable(t *testing.T) {
	t.Parallel()

	const message = "created an issue in TEST, but Jira's answer could not be read, so its key is unknown " +
		`(find it by its summary: issue list --jql "project = TEST ORDER BY created DESC"); ` +
		"the write was applied, do not repeat it: Jira's answer does not name the new issue"

	newService := func(creates *int) *mock.IssueService {
		return &mock.IssueService{CreateFn: func(context.Context, *jira4claude.Issue) (*jira4claude.Issue, error) {
			*creates++
			return nil, unreadableAnswer("issue")
		}}
	}

	t.Run("text", func(t *testing.T) {
		t.Parallel()
		creates := 0

		res := runCLI(t, testDeps(newService(&creates), &mock.UserService{}), "issue", "create", "-s", "New")

		assert.Equal(t, 7, res.code)
		assert.Equal(t, "TEST: created\n", res.stdout)
		assert.Equal(t, "[error] "+message+"\n", res.stderr)
		assert.Equal(t, 1, creates)
	})

	t.Run("json", func(t *testing.T) {
		t.Parallel()
		creates := 0

		res := runCLI(t, testDeps(newService(&creates), &mock.UserService{}), "--json", "issue", "create", "-s", "New")

		assert.Equal(t, 7, res.code)
		assert.JSONEq(t, `{"project":"TEST","action":"created","changes":{}}`, res.stdout)
		var line map[string]any
		require.NoError(t, json.Unmarshal([]byte(res.stderr), &line))
		assert.Equal(t, map[string]any{"level": "error", "error": true, "code": "internal", "message": message}, line)
		assert.Equal(t, 1, creates)
	})

	t.Run("the create without the rejected assignee: no assignment is tried, and the message says so", func(t *testing.T) {
		t.Parallel()
		var creates []*jira4claude.Issue
		svc := &mock.IssueService{
			CreateFn: func(_ context.Context, issue *jira4claude.Issue) (*jira4claude.Issue, error) {
				creates = append(creates, issue)
				if issue.Assignee != nil {
					return nil, assigneeRejected()
				}
				return nil, unreadableAnswer("issue")
			},
			AssignFn: func(context.Context, string, string) error {
				t.Error("Assign must not be called: the key is unknown")
				return nil
			},
		}

		res := runCLI(t, testDeps(svc, me()), "issue", "create", "-s", "New", "--assignee", "me")

		assert.Equal(t, 7, res.code)
		assert.Equal(t, "TEST: created\n", res.stdout)
		assert.Contains(t, res.stderr, "[warn] Jira rejected the assignee on create")
		assert.Contains(t, res.stderr, "[error] created an issue in TEST without its assignee, but Jira's answer could not be read, so its key is unknown "+
			`(find it by its summary: issue list --jql "project = TEST ORDER BY created DESC", then assign it with issue assign); `+
			"the write was applied, do not repeat it: Jira's answer does not name the new issue\n")
		assert.Len(t, creates, 2)
	})
}

func TestIssueComment_AnswerUnreadable(t *testing.T) {
	t.Parallel()

	const message = "added a comment to TEST-1, but Jira's answer could not be read, so its ID is unknown " +
		"(issue view TEST-1 --field comments lists it); " +
		"the write was applied, do not repeat it: Jira's answer does not name the new comment"

	newService := func(adds *int) *mock.IssueService {
		return &mock.IssueService{AddCommentFn: func(context.Context, string, *jira4claude.ADFNode) (*jira4claude.Comment, error) {
			*adds++
			return nil, unreadableAnswer("comment")
		}}
	}

	t.Run("text", func(t *testing.T) {
		t.Parallel()
		adds := 0

		res := runCLI(t, testDeps(newService(&adds), &mock.UserService{}), "issue", "comment", "TEST-1", "--body", "Done.")

		assert.Equal(t, 7, res.code)
		assert.Equal(t, "TEST-1: added comment\nhttps://test.atlassian.net/browse/TEST-1\n", res.stdout)
		assert.Equal(t, "[error] "+message+"\n", res.stderr)
		assert.Equal(t, 1, adds)
	})

	t.Run("json", func(t *testing.T) {
		t.Parallel()
		adds := 0

		res := runCLI(t, testDeps(newService(&adds), &mock.UserService{}), "--json", "issue", "comment", "TEST-1", "--body", "Done.")

		assert.Equal(t, 7, res.code)
		assert.JSONEq(t, `{"key":"TEST-1","url":"https://test.atlassian.net/browse/TEST-1","action":"comment_added","changes":{}}`, res.stdout)
		var line map[string]any
		require.NoError(t, json.Unmarshal([]byte(res.stderr), &line))
		assert.Equal(t, map[string]any{"level": "error", "error": true, "code": "internal", "message": message}, line)
	})
}
