package main_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/fwojciec/jira4claude"
	main "github.com/fwojciec/jira4claude/cmd/j4c"
	"github.com/fwojciec/jira4claude/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// attachmentDeps returns deps whose attachment service factory hands back svc.
// The issue service factory fails the test: attachment commands do not need it.
func attachmentDeps(t *testing.T, svc jira4claude.AttachmentService) main.Deps {
	t.Helper()
	deps := unreachableDeps(t)
	deps.LoadConfig = testDeps(nil, nil).LoadConfig
	deps.NewAttachmentService = func(cfg *jira4claude.Config) (jira4claude.AttachmentService, error) {
		return svc, nil
	}
	return deps
}

func attachmentLister(attachments []*jira4claude.Attachment, err error) *mock.AttachmentService {
	return &mock.AttachmentService{
		ListFn: func(context.Context, string) ([]*jira4claude.Attachment, error) {
			return attachments, err
		},
	}
}

func sampleAttachments() []*jira4claude.Attachment {
	return []*jira4claude.Attachment{
		{
			ID:       "10000",
			Filename: "picture.jpg",
			MimeType: "image/jpeg",
			Size:     23123,
			Author:   &jira4claude.User{DisplayName: "Mia Krystof"},
			Created:  time.Date(2022, 10, 6, 7, 32, 47, 0, time.UTC),
		},
		{ID: "10001", Filename: "debug log.txt", MimeType: "text/plain", Size: 2460},
	}
}

func TestRun_AttachmentList(t *testing.T) {
	t.Parallel()

	t.Run("lists the issue's attachments one per line, ID first, exact bytes", func(t *testing.T) {
		t.Parallel()

		var gotKey string
		svc := &mock.AttachmentService{ListFn: func(_ context.Context, key string) ([]*jira4claude.Attachment, error) {
			gotKey = key
			return sampleAttachments(), nil
		}}

		res := runCLI(t, attachmentDeps(t, svc), "attachment", "list", "TEST-1")

		assert.Equal(t, 0, res.code)
		assert.Equal(t, "TEST-1", gotKey)
		assert.Equal(t, "10000\t23123\timage/jpeg\tpicture.jpg\n10001\t2460\ttext/plain\tdebug log.txt\n", res.stdout)
		assert.Empty(t, res.stderr)
	})

	t.Run("prints a JSON array in --json mode", func(t *testing.T) {
		t.Parallel()

		res := runCLI(t, attachmentDeps(t, attachmentLister(sampleAttachments(), nil)), "--json", "attachment", "list", "TEST-1")

		assert.Equal(t, 0, res.code)
		assert.JSONEq(t, `[
			{"id":"10000","filename":"picture.jpg","mimeType":"image/jpeg","size":23123,"author":"Mia Krystof","created":"2022-10-06T07:32:47Z"},
			{"id":"10001","filename":"debug log.txt","mimeType":"text/plain","size":2460}
		]`, res.stdout)
		assert.Empty(t, res.stderr)
	})

	t.Run("passes the loaded config to the attachment service factory", func(t *testing.T) {
		t.Parallel()

		var gotServer string
		deps := attachmentDeps(t, nil)
		deps.NewAttachmentService = func(cfg *jira4claude.Config) (jira4claude.AttachmentService, error) {
			gotServer = cfg.Server
			return attachmentLister(nil, nil), nil
		}

		res := runCLI(t, deps, "attachment", "list", "TEST-1")

		assert.Equal(t, 0, res.code)
		assert.Equal(t, "https://test.atlassian.net", gotServer)
	})

	t.Run("a service factory error is printed and its exit code returned", func(t *testing.T) {
		t.Parallel()

		deps := attachmentDeps(t, nil)
		deps.NewAttachmentService = func(cfg *jira4claude.Config) (jira4claude.AttachmentService, error) {
			return nil, &jira4claude.Error{Code: jira4claude.EUnauthorized, Message: "could not read netrc file"}
		}

		res := runCLI(t, deps, "attachment", "list", "TEST-1")

		assert.Equal(t, 2, res.code)
		assert.Empty(t, res.stdout)
		assert.Equal(t, "[error] could not read netrc file\n", res.stderr)
	})

	t.Run("requires an issue key", func(t *testing.T) {
		t.Parallel()

		res := runCLI(t, unreachableDeps(t), "attachment", "list")

		assert.Equal(t, 1, res.code)
		assert.Empty(t, res.stdout)
		assert.Contains(t, res.stderr, "expected \"<key>\"")
	})
}

func TestRun_IssueViewAttachments(t *testing.T) {
	t.Parallel()

	withAttachments := func() *jira4claude.Issue {
		return &jira4claude.Issue{
			Key:         "TEST-1",
			Summary:     "With files",
			Status:      "To Do",
			Type:        "Task",
			Attachments: sampleAttachments(),
		}
	}

	t.Run("text view shows an attachments section from the same issue read", func(t *testing.T) {
		t.Parallel()

		calls := 0
		svc := &mock.IssueService{GetFn: func(context.Context, string) (*jira4claude.Issue, error) {
			calls++
			return withAttachments(), nil
		}}

		res := runCLI(t, testDeps(svc, &mock.UserService{}), "issue", "view", "TEST-1")

		assert.Equal(t, 0, res.code)
		assert.Contains(t, res.stdout, "\n## Attachments\n\n"+
			"- 10000 picture.jpg (22.6 KiB, image/jpeg)\n"+
			"- 10001 debug log.txt (2.4 KiB, text/plain)\n")
		assert.Empty(t, res.stderr)
		assert.Equal(t, 1, calls)
	})

	t.Run("text view has no attachments section without attachments", func(t *testing.T) {
		t.Parallel()

		res := runCLI(t, testDeps(issueGetter(&jira4claude.Issue{Key: "TEST-1"}, nil), &mock.UserService{}), "issue", "view", "TEST-1")

		assert.Equal(t, 0, res.code)
		assert.NotContains(t, res.stdout, "Attachments")
	})

	t.Run("JSON view has an attachments array", func(t *testing.T) {
		t.Parallel()

		res := runCLI(t, testDeps(issueGetter(withAttachments(), nil), &mock.UserService{}), "--json", "issue", "view", "TEST-1")

		assert.Equal(t, 0, res.code)
		var got struct {
			Attachments json.RawMessage `json:"attachments"`
		}
		require.NoError(t, json.Unmarshal([]byte(res.stdout), &got))
		assert.JSONEq(t, `[
			{"id":"10000","filename":"picture.jpg","mimeType":"image/jpeg","size":23123,"author":"Mia Krystof","created":"2022-10-06T07:32:47Z"},
			{"id":"10001","filename":"debug log.txt","mimeType":"text/plain","size":2460}
		]`, string(got.Attachments))
	})

	t.Run("JSON view omits attachments when there are none", func(t *testing.T) {
		t.Parallel()

		res := runCLI(t, testDeps(issueGetter(&jira4claude.Issue{Key: "TEST-1"}, nil), &mock.UserService{}), "--json", "issue", "view", "TEST-1")

		assert.Equal(t, 0, res.code)
		var got map[string]any
		require.NoError(t, json.Unmarshal([]byte(res.stdout), &got))
		assert.NotContains(t, got, "attachments")
	})
}
