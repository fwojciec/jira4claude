package main_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/fwojciec/jira4claude"
	"github.com/fwojciec/jira4claude/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Dry run (design 2.3): --dry-run on create, update and comment prints what
// the write would do, with the text as Jira would hold it, and writes
// nothing. Warnings and refusals are those of the real run.

// noWrites sets every mutating function of svc to fail the test when called,
// and returns svc. Reads are left as they are.
func noWrites(t *testing.T, svc *mock.IssueService) *mock.IssueService {
	t.Helper()
	fail := func(name string) { t.Errorf("%s must not be called under --dry-run", name) }
	svc.CreateFn = func(context.Context, *jira4claude.Issue) (*jira4claude.Issue, error) {
		fail("Create")
		return &jira4claude.Issue{Key: "TEST-99"}, nil
	}
	svc.UpdateFn = func(_ context.Context, key string, _ jira4claude.IssueUpdate) (*jira4claude.Issue, error) {
		fail("Update")
		return &jira4claude.Issue{Key: key}, nil
	}
	svc.AddCommentFn = func(context.Context, string, *jira4claude.ADFNode) (*jira4claude.Comment, error) {
		fail("AddComment")
		return &jira4claude.Comment{ID: "99"}, nil
	}
	svc.AssignFn = func(context.Context, string, string) error { fail("Assign"); return nil }
	svc.DeleteFn = func(context.Context, string, bool) error { fail("Delete"); return nil }
	svc.DeleteCommentFn = func(context.Context, string, string) error { fail("DeleteComment"); return nil }
	svc.TransitionFn = func(context.Context, string, string) error { fail("Transition"); return nil }
	svc.LinkFn = func(context.Context, string, string, string) error { fail("Link"); return nil }
	svc.UnlinkFn = func(context.Context, string, string) error { fail("Unlink"); return nil }
	return svc
}

func TestRun_DryRunWritesNothing(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		args   []string
		stdin  string
		action string
	}{
		{"create", []string{"issue", "create", "-s", "New", "-d", "Text"}, "", jira4claude.ActionWouldCreate},
		{"create with every field", []string{"issue", "create", "-s", "New", "-d", "Text", "--assignee=me", "--labels=a",
			"--priority=High", "--parent=TEST-9", `--field-json=customfield_1={"value":"x"}`}, "", jira4claude.ActionWouldCreate},
		{"create from stdin", []string{"issue", "create", "-s", "New", "--description-file", "-"}, "Text\n", jira4claude.ActionWouldCreate},
		{"update every field", []string{"issue", "update", "TEST-1", "--summary=New", "--priority=High", "--assignee=me",
			"--labels=b", "--parent=TEST-9", `--field-json=customfield_10801={"value":"High"}`, "--description=Text"}, "", jira4claude.ActionWouldUpdate},
		{"update append", []string{"issue", "update", "TEST-1", "--append-description=More"}, "", jira4claude.ActionWouldUpdate},
		{"update clear", []string{"issue", "update", "TEST-1", "--clear-description", "--clear-labels", "--clear-parent", "--assignee="}, "", jira4claude.ActionWouldUpdate},
		{"update from a file on stdin", []string{"issue", "update", "TEST-1", "--description-file", "-"}, "Text\n", jira4claude.ActionWouldUpdate},
		{"comment", []string{"issue", "comment", "TEST-1", "-b", "Hello"}, "", jira4claude.ActionWouldAddComment},
		{"comment from stdin", []string{"issue", "comment", "TEST-1", "--body-file=-"}, "Hello\n", jira4claude.ActionWouldAddComment},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			svc := noWrites(t, &mock.IssueService{GetFn: readSequence(beforeIssue())})
			res := runCLIStdin(t, testDeps(svc, me()), tc.stdin, append([]string{"--json"}, append(tc.args, "--dry-run")...)...)

			require.Equal(t, 0, res.code, "stderr: %s", res.stderr)
			var result map[string]any
			require.NoError(t, json.Unmarshal([]byte(res.stdout), &result))
			assert.Equal(t, tc.action, result["action"])
			assert.Equal(t, true, result["dryRun"])
			assert.NotContains(t, result, "updated", "no write, no updated time")
			assert.NotContains(t, result, "commentId")
		})
	}
}

func TestRun_DryRunUpdateReadsAndChecks(t *testing.T) {
	t.Parallel()

	t.Run("the issue is read once and nothing else is called", func(t *testing.T) {
		t.Parallel()

		var calls []string
		svc := noWrites(t, &mock.IssueService{GetFn: func(_ context.Context, key string) (*jira4claude.Issue, error) {
			calls = append(calls, "get "+key)
			return beforeIssue(), nil
		}})

		res := runCLI(t, testDeps(svc, me()), "issue", "update", "TEST-1", "--summary=New summary", "--assignee=me", "--dry-run",
			"--if-updated=2026-11-09T10:15:30.123Z")

		require.Equal(t, 0, res.code, "stderr: %s", res.stderr)
		assert.Equal(t, []string{"get TEST-1"}, calls)
		assert.Equal(t, "TEST-1: would update (dry run; nothing was written)\n"+
			"  assignee: Jane Doe → u-me\n"+
			"  summary: Old summary → New summary\n"+
			testURL+"TEST-1\n", res.stdout)
	})

	t.Run("a stale --if-updated is a conflict, as in a real run", func(t *testing.T) {
		t.Parallel()

		svc := noWrites(t, &mock.IssueService{GetFn: readSequence(beforeIssue())})

		res := runCLI(t, testDeps(svc, me()), "issue", "update", "TEST-1", "--summary=New", "--dry-run",
			"--if-updated=2026-11-09T10:15:30.122Z")

		assert.Equal(t, 5, res.code)
		assert.Empty(t, res.stdout)
	})
}

// The dry run's text is what issue view would print after the real write.
func TestRun_DryRunTextIsTheViewAfterTheWrite(t *testing.T) {
	t.Parallel()

	md, _, mention := guardedTokens(t)
	cases := map[string][]string{
		"set":    {"--force", "--description=" + strings.Replace(md, mention, "@Jane", 1) + "\n\n| a | b |\n|---|---|\n| 1 | 2 |"},
		"append": {"--append-description=More *text*."},
		"clear":  {"--clear-description", "--force"},
	}
	for name, flags := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			args := append([]string{"--json", "issue", "update", "TEST-1"}, flags...)
			actual := newGuardedService(guardedIssue())
			res := runCLI(t, testDeps(actual, &mock.UserService{}), args...)
			require.Equal(t, 0, res.code, "stderr: %s", res.stderr)

			after := guardedIssue()
			after.Description = actual.Sent().Description.Document()
			view := runCLI(t, testDeps(newGuardedService(after), &mock.UserService{}), "issue", "view", "TEST-1", "--field", "description")
			require.Equal(t, 0, view.code, "stderr: %s", view.stderr)

			dry := runCLI(t, testDeps(noWrites(t, &mock.IssueService{GetFn: readSequence(guardedIssue())}), &mock.UserService{}),
				append(args, "--dry-run")...)
			require.Equal(t, 0, dry.code, "stderr: %s", dry.stderr)
			var result struct {
				Text *string `json:"text"`
			}
			require.NoError(t, json.Unmarshal([]byte(dry.stdout), &result))
			require.NotNil(t, result.Text)
			assert.Equal(t, strings.TrimSuffix(view.stdout, "\n"), *result.Text)
		})
	}
}

// A dry run reports the same warnings and refusals, with the same exit code,
// as the real run: only stdout differs.
func TestRun_DryRunSameDiagnostics(t *testing.T) {
	t.Parallel()

	md, media, mention := guardedTokens(t)
	notFound := &jira4claude.Error{Code: jira4claude.ENotFound, Message: "issue does not exist"}
	cases := []struct {
		name  string
		args  []string
		issue *jira4claude.Issue
		read  error
		code  int
	}{
		{"create refuses an image", []string{"issue", "create", "-s", "New", "-d", imageMarkdown}, nil, nil, 1},
		{"create forces an image", []string{"issue", "create", "-s", "New", "-d", imageMarkdown, "--force"}, nil, nil, 0},
		{"create warns on degraded content and token text", []string{"issue", "create", "-s", "New", "--description=" + degradedMarkdown + "\n\n{{mention:1 x}}"}, nil, nil, 0},
		{"comment refuses an image", []string{"issue", "comment", "TEST-1", "-b", imageMarkdown}, nil, nil, 1},
		{"comment warns on degraded content", []string{"issue", "comment", "TEST-1", "--body=" + degradedMarkdown}, nil, nil, 0},
		{"comment with an empty body", []string{"issue", "comment", "TEST-1", "-b", ""}, nil, nil, 1},
		{"update refuses an image", []string{"issue", "update", "TEST-1", "-d", imageMarkdown}, guardedIssue(), nil, 1},
		{"update refuses a missing token", []string{"issue", "update", "TEST-1", "-d", strings.Replace(md, media, "", 1)}, guardedIssue(), nil, 1},
		{"update forces a missing token", []string{"issue", "update", "TEST-1", "--force", "-d", strings.Replace(md, mention, "", 1)}, guardedIssue(), nil, 0},
		{"update warns on a duplicate token", []string{"issue", "update", "TEST-1", "--force", "-d", md + "\n\n" + mention}, guardedIssue(), nil, 0},
		{"update refuses a clear of protected nodes", []string{"issue", "update", "TEST-1", "--clear-description"}, guardedIssue(), nil, 1},
		{"update append warns on degraded content", []string{"issue", "update", "TEST-1", "--append-description=" + degradedMarkdown}, guardedIssue(), nil, 0},
		{"update with a stale --if-updated", []string{"issue", "update", "TEST-1", "--summary=x", "--if-updated=2026-11-09T10:15:30.122Z"}, guardedIssue(), nil, 5},
		{"update of a missing issue", []string{"issue", "update", "TEST-1", "--summary=x"}, nil, notFound, 4},
		{"update with an empty description", []string{"issue", "update", "TEST-1", "--description="}, guardedIssue(), nil, 1},
	}
	for _, tc := range cases {
		for _, mode := range []string{"text", "json"} {
			t.Run(tc.name+"/"+mode, func(t *testing.T) {
				t.Parallel()

				args := tc.args
				if mode == "json" {
					args = append([]string{"--json"}, args...)
				}
				get := func(context.Context, string) (*jira4claude.Issue, error) { return tc.issue, tc.read }

				rec := &recorder{}
				svc := rec.service(tc.issue)
				svc.GetFn = get
				actual := runCLI(t, testDeps(svc, &mock.UserService{}), args...)

				dry := runCLI(t, testDeps(noWrites(t, &mock.IssueService{GetFn: get}), &mock.UserService{}), append(args, "--dry-run")...)

				assert.Equal(t, tc.code, actual.code, "stderr: %s", actual.stderr)
				assert.Equal(t, actual.code, dry.code)
				assert.Equal(t, actual.stderr, dry.stderr)
				if tc.code != 0 {
					assert.Empty(t, dry.stdout)
				} else {
					assert.Contains(t, dry.stdout, "would")
				}
			})
		}
	}
}

// The stream rules hold under --dry-run: the read that an update still makes
// fails as in a real run, with only stderr written.
func TestRun_DryRunServiceErrorStreams(t *testing.T) {
	t.Parallel()

	for _, ec := range errorCodes() {
		message := "failed with " + ec.code

		t.Run(ec.code+"/text", func(t *testing.T) {
			t.Parallel()

			res := runCLI(t, testDeps(noWrites(t, issueGetter(nil, ec.err)), &mock.UserService{}),
				"issue", "update", "TEST-1", "--summary=New", "--dry-run")

			assert.Equal(t, ec.exit, res.code)
			assert.Empty(t, res.stdout)
			assert.Equal(t, "[error] "+message+"\n", res.stderr)
		})

		t.Run(ec.code+"/json", func(t *testing.T) {
			t.Parallel()

			res := runCLI(t, testDeps(noWrites(t, issueGetter(nil, ec.err)), &mock.UserService{}),
				"--json", "issue", "update", "TEST-1", "--summary=New", "--dry-run")

			assert.Equal(t, ec.exit, res.code)
			assert.Empty(t, res.stdout)
			assertJSONError(t, res.stderr, ec.code, message)
		})

		// The assignee is still resolved, a read, so its failure is reported.
		t.Run(ec.code+"/create resolving the assignee", func(t *testing.T) {
			t.Parallel()

			res := runCLI(t, testDeps(noWrites(t, &mock.IssueService{}), &mock.UserService{
				GetMyselfFn: func(context.Context) (*jira4claude.User, error) { return nil, ec.err },
			}), "--json", "issue", "create", "-s", "New", "--assignee=me", "--dry-run")

			assert.Equal(t, ec.exit, res.code)
			assert.Empty(t, res.stdout)
			assertJSONError(t, res.stderr, ec.code, message)
		})
	}
}

// richDescription holds what a preview could leave out: protected nodes
// (a mention, an extension, a node type the reader does not know) and
// content with no Markdown form (an inline comment, a text colour, a
// panel's custom colour, a layout's columns, a line break ending a
// paragraph, a control character).
const richDescription = `{"type":"doc","version":1,"content":[` +
	`{"type":"paragraph","content":[{"type":"text","text":"commented","marks":[{"type":"annotation","attrs":{"id":"a1","annotationType":"inlineComment"}}]},` +
	`{"type":"text","text":" red","marks":[{"type":"textColor","attrs":{"color":"#ff0000"}}]},{"type":"text","text":" for "},` +
	`{"type":"mention","attrs":{"id":"u1","text":"@Jane Doe"}}]},` +
	`{"type":"paragraph","content":[{"type":"text","text":"bell\u0007here"},{"type":"hardBreak"}]},` +
	`{"type":"extension","attrs":{"extensionType":"t","extensionKey":"toc"}},` +
	`{"type":"panel","attrs":{"panelType":"custom","panelColor":"#abcdef"},"content":[{"type":"paragraph","content":[{"type":"text","text":"in panel"}]}]},` +
	`{"type":"layoutSection","content":[{"type":"layoutColumn","attrs":{"width":50},"content":[{"type":"paragraph","content":[{"type":"text","text":"col1"}]}]},` +
	`{"type":"layoutColumn","attrs":{"width":50},"content":[{"type":"paragraph","content":[{"type":"text","text":"col2"}]}]}]},` +
	`{"type":"weirdNode","content":[{"type":"paragraph","content":[{"type":"text","text":"inside"}]}]}]}`

// jir-3rwv: the text of an append's dry run is the description issue view
// prints after the real write, byte for byte: existing protected nodes are
// tokens, numbered for that description, and what has no Markdown form
// (inline comments, colours, layouts) is left out of both, as issue view
// leaves it out, while the write sends it back unchanged.
func TestRun_DryRunAppendTextIsIssueViewAfterTheWrite(t *testing.T) {
	t.Parallel()

	var doc jira4claude.ADFNode
	require.NoError(t, json.Unmarshal([]byte(richDescription), &doc))
	current := &jira4claude.Issue{Key: "TEST-1", Summary: "S", Description: &doc}
	path := writeFile(t, "more.md", []byte("More *text*.\n"))

	dry := runCLI(t, testDeps(noWrites(t, issueGetter(current, nil)), &mock.UserService{}),
		"--json", "issue", "update", "TEST-1", "--append-description-file", path, "--dry-run")
	require.Equal(t, 0, dry.code, "stderr: %s", dry.stderr)
	var result struct {
		Text string `json:"text"`
	}
	require.NoError(t, json.Unmarshal([]byte(dry.stdout), &result))

	rec := &recorder{}
	wrote := runCLI(t, testDeps(rec.service(current), &mock.UserService{}),
		"issue", "update", "TEST-1", "--append-description-file", path)
	require.Equal(t, 0, wrote.code, "stderr: %s", wrote.stderr)
	rec.mu.Lock()
	written := rec.update.Description.Document()
	rec.mu.Unlock()
	view := runCLI(t, testDeps(issueGetter(&jira4claude.Issue{Key: "TEST-1", Description: written}, nil), &mock.UserService{}),
		"issue", "view", "TEST-1", "--field", "description")
	require.Equal(t, 0, view.code, "stderr: %s", view.stderr)

	assert.Equal(t, view.stdout, result.Text+"\n")
	for _, want := range []string{"commented red for {{mention:1 Jane Doe}}", "{{extension:2 toc}}", "{{weirdNode:3}}", "More *text*."} {
		assert.Contains(t, result.Text, want)
	}
	assert.Equal(t, mustMarshal(t, doc.Content), mustMarshal(t, written.Content[:len(doc.Content)]),
		"the existing content is sent back as it was, what the preview leaves out included")
}

func mustMarshal(t *testing.T, v any) string {
	t.Helper()
	data, err := json.Marshal(v)
	require.NoError(t, err)
	return string(data)
}
