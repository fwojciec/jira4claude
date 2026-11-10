package main_test

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/fwojciec/jira4claude"
	"github.com/fwojciec/jira4claude/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Guarded description update tests (#150). They drive main.Run with the real
// Markdown converter: the description an agent edits is the one issue view
// printed, tokens and all.

// guardedMediaSingle and guardedMention are the protected nodes of the
// description guardedIssue returns.
func guardedMediaSingle() jira4claude.ADFNode {
	return jira4claude.ADFNode{
		Type:  "mediaSingle",
		Attrs: json.RawMessage(`{"layout":"center","width":50}`),
		Content: []jira4claude.ADFNode{{
			Type:  "media",
			Attrs: json.RawMessage(`{"id":"abc-123","type":"file","collection":"contentId-1","alt":"shot.png"}`),
		}},
	}
}

func guardedMention() jira4claude.ADFNode {
	return jira4claude.ADFNode{
		Type:  "mention",
		Attrs: json.RawMessage(`{"id":"user-1","text":"@Jane Doe","accessLevel":""}`),
	}
}

// guardedUpdated is the updated time of guardedIssue, with milliseconds.
func guardedUpdated() time.Time {
	return time.Date(2026, 11, 9, 10, 15, 30, 123_000_000, time.UTC)
}

func guardedIssue() *jira4claude.Issue {
	return &jira4claude.Issue{
		Key:     "TEST-1",
		Summary: "Guarded",
		Status:  "To Do",
		Type:    "Task",
		Updated: guardedUpdated(),
		Description: &jira4claude.ADFNode{Type: "doc", Version: 1, Content: []jira4claude.ADFNode{
			{Type: "paragraph", Content: []jira4claude.ADFNode{{Type: "text", Text: "Before the image."}}},
			guardedMediaSingle(),
			{Type: "paragraph", Content: []jira4claude.ADFNode{
				{Type: "text", Text: "Ping "},
				guardedMention(),
				{Type: "text", Text: " about it."},
			}},
		}},
	}
}

// guardedService serves issue on Get and records the calls made, in order,
// and the update sent.
type guardedService struct {
	mock.IssueService

	mu     sync.Mutex
	calls  []string
	update *jira4claude.IssueUpdate
}

func newGuardedService(issue *jira4claude.Issue) *guardedService {
	s := &guardedService{}
	s.GetFn = func(ctx context.Context, key string) (*jira4claude.Issue, error) {
		s.record("get " + key)
		return issue, nil
	}
	s.UpdateFn = func(ctx context.Context, key string, update jira4claude.IssueUpdate) (*jira4claude.Issue, error) {
		s.record("update " + key)
		s.mu.Lock()
		s.update = &update
		s.mu.Unlock()
		return &jira4claude.Issue{Key: key}, nil
	}
	return s
}

func (s *guardedService) record(call string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls = append(s.calls, call)
}

func (s *guardedService) Calls() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.calls...)
}

func (s *guardedService) Sent() *jira4claude.IssueUpdate {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.update
}

// viewJSON runs issue view --json on issue and returns the description and
// updated it prints.
func viewJSON(t *testing.T, issue *jira4claude.Issue) (description, updated string) {
	t.Helper()
	res := runCLI(t, testDeps(newGuardedService(issue), &mock.UserService{}), "--json", "issue", "view", issue.Key)
	require.Equal(t, 0, res.code, "stdout: %s stderr: %s", res.stdout, res.stderr)
	var got struct {
		Description string `json:"description"`
		Updated     string `json:"updated"`
	}
	require.NoError(t, json.Unmarshal([]byte(res.stdout), &got))
	return got.Description, got.Updated
}

// findNodes returns every node of type nodeType in doc, depth first.
func findNodes(doc *jira4claude.ADFNode, nodeType string) []jira4claude.ADFNode {
	var found []jira4claude.ADFNode
	var walk func(nodes []jira4claude.ADFNode)
	walk = func(nodes []jira4claude.ADFNode) {
		for _, n := range nodes {
			if n.Type == nodeType {
				found = append(found, n)
			}
			walk(n.Content)
		}
	}
	if doc != nil {
		walk(doc.Content)
	}
	return found
}

func marshal(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	require.NoError(t, err)
	return string(b)
}

// tokenLines returns the lines of md that hold a token.
func tokenLines(md string) []string {
	var lines []string
	for _, line := range strings.Split(md, "\n") {
		if strings.Contains(line, "{{") {
			lines = append(lines, line)
		}
	}
	return lines
}

func TestRun_IssueViewUpdatedMilliseconds(t *testing.T) {
	t.Parallel()

	_, updated := viewJSON(t, guardedIssue())

	assert.Equal(t, "2026-11-09T10:15:30.123Z", updated)
}

func TestRun_IssueUpdateIfUpdated(t *testing.T) {
	t.Parallel()

	stale := "2026-11-09T10:15:30.122Z" // one millisecond before guardedUpdated

	t.Run("a stale value exits 5 and makes no write", func(t *testing.T) {
		t.Parallel()

		svc := newGuardedService(guardedIssue())

		res := runCLI(t, testDeps(svc, &mock.UserService{}),
			"issue", "update", "TEST-1", "--summary=New", "--if-updated="+stale)

		assert.Equal(t, 5, res.code)
		assert.Empty(t, res.stdout)
		assert.Contains(t, res.stderr, "[error] ")
		assert.Contains(t, res.stderr, "2026-11-09T10:15:30.123Z")
		assert.Equal(t, []string{"get TEST-1"}, svc.Calls())
	})

	t.Run("a stale value exits 5 with a JSON error on stdout", func(t *testing.T) {
		t.Parallel()

		svc := newGuardedService(guardedIssue())

		res := runCLI(t, testDeps(svc, &mock.UserService{}),
			"--json", "issue", "update", "TEST-1", "--description=New text", "--if-updated="+stale)

		assert.Equal(t, 5, res.code)
		assert.Empty(t, res.stderr)
		var got map[string]any
		require.NoError(t, json.Unmarshal([]byte(res.stdout), &got))
		assert.Equal(t, true, got["error"])
		assert.Equal(t, "conflict", got["code"])
		assert.Contains(t, got["message"], "2026-11-09T10:15:30.123Z")
		assert.Equal(t, []string{"get TEST-1"}, svc.Calls())
	})

	for _, flag := range []string{"--append-description=More", "--clear-description", "--force", "--summary=New"} {
		t.Run("a stale value also stops "+flag, func(t *testing.T) {
			t.Parallel()

			svc := newGuardedService(guardedIssue())

			res := runCLI(t, testDeps(svc, &mock.UserService{}),
				"issue", "update", "TEST-1", flag, "--if-updated="+stale)

			assert.Equal(t, 5, res.code)
			assert.Equal(t, []string{"get TEST-1"}, svc.Calls())
		})
	}

	t.Run("the value issue view --json printed writes", func(t *testing.T) {
		t.Parallel()

		md, updated := viewJSON(t, guardedIssue())
		svc := newGuardedService(guardedIssue())

		res := runCLI(t, testDeps(svc, &mock.UserService{}),
			"issue", "update", "TEST-1", "--description="+md, "--if-updated="+updated)

		assert.Equal(t, 0, res.code, "stderr: %s", res.stderr)
		assert.Equal(t, []string{"get TEST-1", "update TEST-1"}, svc.Calls())
	})

	t.Run("the form Jira returns writes", func(t *testing.T) {
		t.Parallel()

		svc := newGuardedService(guardedIssue())

		res := runCLI(t, testDeps(svc, &mock.UserService{}),
			"issue", "update", "TEST-1", "--summary=New", "--if-updated=2026-11-09T11:15:30.123+0100")

		assert.Equal(t, 0, res.code, "stderr: %s", res.stderr)
		assert.Equal(t, []string{"get TEST-1", "update TEST-1"}, svc.Calls())
	})

	t.Run("a malformed value exits 1 and makes no service call", func(t *testing.T) {
		t.Parallel()

		// Zero-value mocks panic on any call.
		res := runCLI(t, testDeps(&mock.IssueService{}, &mock.UserService{}),
			"issue", "update", "TEST-1", "--summary=New", "--if-updated=yesterday")

		assert.Equal(t, 1, res.code)
		assert.Contains(t, res.stderr, "--if-updated")
	})

	t.Run("an issue without an updated time is not written", func(t *testing.T) {
		t.Parallel()

		issue := guardedIssue()
		issue.Updated = time.Time{}
		svc := newGuardedService(issue)

		res := runCLI(t, testDeps(svc, &mock.UserService{}),
			"issue", "update", "TEST-1", "--summary=New", "--if-updated=2026-11-09T10:15:30.123Z")

		assert.NotEqual(t, 0, res.code)
		assert.NotEqual(t, 5, res.code)
		assert.Equal(t, []string{"get TEST-1"}, svc.Calls())
	})

	t.Run("without --if-updated a summary update makes no read", func(t *testing.T) {
		t.Parallel()

		svc := newGuardedService(guardedIssue())

		res := runCLI(t, testDeps(svc, &mock.UserService{}), "issue", "update", "TEST-1", "--summary=New")

		assert.Equal(t, 0, res.code, "stderr: %s", res.stderr)
		assert.Equal(t, []string{"update TEST-1"}, svc.Calls())
	})

	t.Run("the help text states the remaining race", func(t *testing.T) {
		t.Parallel()

		res := runCLI(t, unreachableDeps(t), "issue", "update", "--help")

		assert.Equal(t, 0, res.code)
		assert.Contains(t, res.stdout, "--if-updated")
		assert.Contains(t, res.stdout, "not caught")
	})
}

func TestRun_IssueUpdateReattachesDescription(t *testing.T) {
	t.Parallel()

	t.Run("without --if-updated the update fetches first", func(t *testing.T) {
		t.Parallel()

		md, _ := viewJSON(t, guardedIssue())
		svc := newGuardedService(guardedIssue())

		res := runCLI(t, testDeps(svc, &mock.UserService{}), "issue", "update", "TEST-1", "--description="+md)

		assert.Equal(t, 0, res.code, "stderr: %s", res.stderr)
		assert.Equal(t, []string{"get TEST-1", "update TEST-1"}, svc.Calls())
	})

	t.Run("an edit that keeps the tokens sends the original media and mention unchanged", func(t *testing.T) {
		t.Parallel()

		md, _ := viewJSON(t, guardedIssue())
		require.Len(t, tokenLines(md), 2, "description: %q", md)
		edited := strings.Replace(md, "Before the image.", "Rewritten intro.", 1) + "\n\nA new paragraph."
		svc := newGuardedService(guardedIssue())

		res := runCLI(t, testDeps(svc, &mock.UserService{}), "issue", "update", "TEST-1", "--description="+edited)

		require.Equal(t, 0, res.code, "stderr: %s", res.stderr)
		assert.Equal(t, "[ok] Updated: TEST-1\nhttps://test.atlassian.net/browse/TEST-1\n", res.stdout, "the success message is unchanged")
		sent := svc.Sent()
		require.NotNil(t, sent)
		assert.Equal(t, jira4claude.DescriptionSet, sent.Description.Mode())
		doc := sent.Description.Document()
		media := findNodes(doc, "mediaSingle")
		require.Len(t, media, 1)
		assert.Equal(t, marshal(t, guardedMediaSingle()), marshal(t, media[0]))
		mentions := findNodes(doc, "mention")
		require.Len(t, mentions, 1)
		assert.Equal(t, marshal(t, guardedMention()), marshal(t, mentions[0]))
		assert.Contains(t, marshal(t, doc), "Rewritten intro.")
		assert.Contains(t, marshal(t, doc), "A new paragraph.")
		assert.NotContains(t, marshal(t, doc), "{{")
	})

	t.Run("an edit that removes the media token is refused and names it", func(t *testing.T) {
		t.Parallel()

		md, _ := viewJSON(t, guardedIssue())
		mediaToken := tokenLines(md)[0]
		require.Contains(t, mediaToken, "media")
		edited := strings.Replace(md, mediaToken, "", 1)
		svc := newGuardedService(guardedIssue())

		res := runCLI(t, testDeps(svc, &mock.UserService{}), "issue", "update", "TEST-1", "--description="+edited)

		assert.Equal(t, 1, res.code)
		assert.Empty(t, res.stdout)
		assert.Contains(t, res.stderr, mediaToken)
		assert.Contains(t, res.stderr, "--force")
		assert.Equal(t, []string{"get TEST-1"}, svc.Calls())
	})

	t.Run("a description written without reading first is refused", func(t *testing.T) {
		t.Parallel()

		svc := newGuardedService(guardedIssue())

		res := runCLI(t, testDeps(svc, &mock.UserService{}), "issue", "update", "TEST-1", "--description=Fresh text")

		assert.Equal(t, 1, res.code)
		assert.Contains(t, res.stderr, "mediaSingle:1")
		assert.Contains(t, res.stderr, "mention:2")
		assert.Equal(t, []string{"get TEST-1"}, svc.Calls())
	})

	t.Run("--force writes without the removed node and warns", func(t *testing.T) {
		t.Parallel()

		md, _ := viewJSON(t, guardedIssue())
		mediaToken := tokenLines(md)[0]
		edited := strings.Replace(md, mediaToken, "", 1)
		svc := newGuardedService(guardedIssue())

		res := runCLI(t, testDeps(svc, &mock.UserService{}), "issue", "update", "TEST-1", "--force", "--description="+edited)

		require.Equal(t, 0, res.code, "stderr: %s", res.stderr)
		assert.Contains(t, res.stderr, "[warn] dropped "+mediaToken)
		doc := svc.Sent().Description.Document()
		assert.Empty(t, findNodes(doc, "mediaSingle"))
		mentions := findNodes(doc, "mention")
		require.Len(t, mentions, 1, "the kept token must still be re-attached under --force")
		assert.Equal(t, marshal(t, guardedMention()), marshal(t, mentions[0]))
	})

	t.Run("new content that would be dropped is refused before any service call", func(t *testing.T) {
		t.Parallel()

		// Zero-value mocks panic on any call.
		res := runCLI(t, testDeps(&mock.IssueService{}, &mock.UserService{}),
			"issue", "update", "TEST-1", "--description=![new](https://example.com/new.png)")

		assert.Equal(t, 1, res.code)
		assert.Contains(t, res.stderr, "Image")
	})
}

func TestRun_TokenTextOutsideADescriptionSet(t *testing.T) {
	t.Parallel()

	const tokenText = "As before: {{mention:2 Jane Doe}}"

	t.Run("a comment writes token text as plain text and warns", func(t *testing.T) {
		t.Parallel()

		var sent *jira4claude.ADFNode
		svc := &mock.IssueService{
			AddCommentFn: func(ctx context.Context, key string, body *jira4claude.ADFNode) (*jira4claude.Comment, error) {
				sent = body
				return &jira4claude.Comment{ID: "1"}, nil
			},
		}

		res := runCLI(t, testDeps(svc, &mock.UserService{}), "issue", "comment", "TEST-1", "--body="+tokenText)

		require.Equal(t, 0, res.code, "stderr: %s", res.stderr)
		assert.Contains(t, res.stderr, "[warn] degraded {{mention:2 Jane Doe}}")
		assert.Contains(t, marshal(t, sent), "{{mention:2 Jane Doe}}")
	})

	t.Run("an append keeps the existing nodes and warns about token text", func(t *testing.T) {
		t.Parallel()

		svc := newGuardedService(guardedIssue())

		res := runCLI(t, testDeps(svc, &mock.UserService{}), "issue", "update", "TEST-1", "--append-description="+tokenText)

		require.Equal(t, 0, res.code, "stderr: %s", res.stderr)
		assert.Contains(t, res.stderr, "[warn] degraded {{mention:2 Jane Doe}}")
		doc := svc.Sent().Description.Document()
		assert.Len(t, findNodes(doc, "mention"), 1)
		assert.Contains(t, marshal(t, doc), "{{mention:2 Jane Doe}}")
	})
}

func TestRun_IssueUpdateClearDescription(t *testing.T) {
	t.Parallel()

	t.Run("clearing a description that holds protected nodes is refused and names them", func(t *testing.T) {
		t.Parallel()

		svc := newGuardedService(guardedIssue())

		res := runCLI(t, testDeps(svc, &mock.UserService{}), "issue", "update", "TEST-1", "--clear-description")

		assert.Equal(t, 1, res.code)
		assert.Contains(t, res.stderr, "mediaSingle:1")
		assert.Contains(t, res.stderr, "mention:2")
		assert.Contains(t, res.stderr, "--force")
		assert.Equal(t, []string{"get TEST-1"}, svc.Calls())
	})

	t.Run("--force clears it and warns about each node", func(t *testing.T) {
		t.Parallel()

		svc := newGuardedService(guardedIssue())

		res := runCLI(t, testDeps(svc, &mock.UserService{}), "issue", "update", "TEST-1", "--clear-description", "--force")

		require.Equal(t, 0, res.code, "stderr: %s", res.stderr)
		assert.Equal(t, jira4claude.DescriptionClear, svc.Sent().Description.Mode())
		assert.Contains(t, res.stderr, "mediaSingle:1")
		assert.Contains(t, res.stderr, "mention:2")
	})

	t.Run("a description of plain text clears without --force", func(t *testing.T) {
		t.Parallel()

		issue := guardedIssue()
		issue.Description = &jira4claude.ADFNode{Type: "doc", Version: 1, Content: []jira4claude.ADFNode{
			{Type: "paragraph", Content: []jira4claude.ADFNode{{Type: "text", Text: "Plain."}}},
		}}
		svc := newGuardedService(issue)

		res := runCLI(t, testDeps(svc, &mock.UserService{}), "issue", "update", "TEST-1", "--clear-description")

		require.Equal(t, 0, res.code, "stderr: %s", res.stderr)
		assert.Empty(t, res.stderr)
		assert.Equal(t, jira4claude.DescriptionClear, svc.Sent().Description.Mode())
	})
}
