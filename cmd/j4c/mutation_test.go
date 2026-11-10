package main_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/fwojciec/jira4claude"
	main "github.com/fwojciec/jira4claude/cmd/j4c"
	"github.com/fwojciec/jira4claude/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Mutation results (design 2.4): every command that writes prints what it did
// on stdout, as a typed result. The --json shape of each is pinned by a golden
// file in testdata/mutation; set J4C_UPDATE_GOLDEN=1 to rewrite them.

const testURL = "https://test.atlassian.net/browse/"

// writtenAt is the updated time of an issue after a write.
func writtenAt() time.Time {
	return time.Date(2026, 11, 10, 9, 0, 0, 456_000_000, time.UTC)
}

// beforeIssue is TEST-1 as read before a write.
func beforeIssue() *jira4claude.Issue {
	return &jira4claude.Issue{
		Key:      "TEST-1",
		Summary:  "Old summary",
		Status:   "Backlog",
		Type:     "Task",
		Priority: "Medium",
		Assignee: &jira4claude.User{AccountID: "u-jane", DisplayName: "Jane Doe"},
		Labels:   []string{"a"},
		Updated:  guardedUpdated(),
		ReadCustomFields: map[string]jira4claude.CustomFieldValue{
			"customfield_10801": {Name: "Severity", Value: json.RawMessage(`{"value":"Low"}`)},
		},
	}
}

// afterIssue is TEST-1 as read back after the update of fieldsUpdateArgs.
func afterIssue() *jira4claude.Issue {
	return &jira4claude.Issue{
		Key:      "TEST-1",
		Summary:  "New summary",
		Status:   "Backlog",
		Type:     "Task",
		Priority: "High",
		Assignee: &jira4claude.User{AccountID: "u-me", DisplayName: "Me Myself"},
		Labels:   []string{"b", "c"},
		Parent:   &jira4claude.LinkedIssue{Key: "TEST-9"},
		Updated:  writtenAt(),
		ReadCustomFields: map[string]jira4claude.CustomFieldValue{
			"customfield_10801": {Name: "Severity", Value: json.RawMessage(`{"value":"High"}`)},
		},
	}
}

// guardedAfter is guardedIssue as read back after a description write.
func guardedAfter() *jira4claude.Issue {
	issue := guardedIssue()
	issue.Updated = writtenAt()
	return issue
}

// readSequence serves the issues on successive Get calls, repeating the last.
func readSequence(issues ...*jira4claude.Issue) func(context.Context, string) (*jira4claude.Issue, error) {
	n := 0
	return func(context.Context, string) (*jira4claude.Issue, error) {
		issue := issues[min(n, len(issues)-1)]
		n++
		return issue, nil
	}
}

// readsKey serves a bare issue with the key asked for: the read update,
// transition and assign make before writing, for tests about something else.
func readsKey(_ context.Context, key string) (*jira4claude.Issue, error) {
	return &jira4claude.Issue{Key: key}, nil
}

func me() *mock.UserService {
	return &mock.UserService{GetMyselfFn: func(context.Context) (*jira4claude.User, error) {
		return &jira4claude.User{AccountID: "u-me", DisplayName: "Me Myself"}, nil
	}}
}

// updating serves before on Get and after from Update.
func updating(before, after *jira4claude.Issue) *mock.IssueService {
	return &mock.IssueService{
		GetFn: readSequence(before),
		UpdateFn: func(context.Context, string, jira4claude.IssueUpdate) (*jira4claude.Issue, error) {
			return after, nil
		},
	}
}

// guardedTokens returns the media and mention tokens issue view prints for
// guardedIssue, and that Markdown.
func guardedTokens(t *testing.T) (md, media, mention string) {
	t.Helper()
	md, _ = viewJSON(t, guardedIssue())
	lines := tokenLines(md)
	require.Len(t, lines, 2, "description: %q", md)
	media = lines[0]
	start := strings.Index(lines[1], "{{")
	end := strings.Index(lines[1], "}}")
	mention = lines[1][start : end+2]
	return md, media, mention
}

type mutationCase struct {
	name string
	args func(t *testing.T) []string
	deps func(t *testing.T) main.Deps
	text string // stdout in text mode
}

func fixedArgs(args ...string) func(t *testing.T) []string {
	return func(*testing.T) []string { return args }
}

func withServices(issueSvc *mock.IssueService, userSvc *mock.UserService) func(t *testing.T) main.Deps {
	return func(*testing.T) main.Deps { return testDeps(issueSvc, userSvc) }
}

func mutationCases() []mutationCase {
	return []mutationCase{
		{
			name: "issue-create",
			args: fixedArgs("issue", "create", "-s", "New issue"),
			deps: func(*testing.T) main.Deps {
				return testDeps(&mock.IssueService{CreateFn: func(_ context.Context, issue *jira4claude.Issue) (*jira4claude.Issue, error) {
					created := *issue
					created.Key = "TEST-7"
					return &created, nil
				}}, &mock.UserService{})
			},
			text: "TEST-7: created\n" + testURL + "TEST-7\n",
		},
		{
			name: "issue-update-fields",
			args: fixedArgs("issue", "update", "TEST-1", "--summary=New summary", "--priority=High", "--assignee=me",
				"--labels=b,c", "--parent=TEST-9", `--field-json=customfield_10801={"value":"High"}`),
			deps: func(*testing.T) main.Deps { return testDeps(updating(beforeIssue(), afterIssue()), me()) },
			text: "TEST-1: updated\n" +
				"  assignee: Jane Doe → Me Myself\n" +
				`  customfield_10801: {"value":"Low"} → {"value":"High"}` + "\n" +
				"  labels: a → b, c\n" +
				"  parent: (none) → TEST-9\n" +
				"  priority: Medium → High\n" +
				"  summary: Old summary → New summary\n" +
				"  updated: 2026-11-10T09:00:00.456Z\n" +
				testURL + "TEST-1\n",
		},
		{
			name: "issue-update-clear",
			args: fixedArgs("issue", "update", "TEST-1", "--assignee=", "--clear-labels", "--clear-parent"),
			deps: func(*testing.T) main.Deps {
				after := beforeIssue()
				after.Assignee, after.Labels, after.Updated = nil, nil, writtenAt()
				before := beforeIssue()
				before.Parent = &jira4claude.LinkedIssue{Key: "TEST-9"}
				return testDeps(updating(before, after), &mock.UserService{})
			},
			text: "TEST-1: updated\n" +
				"  assignee: Jane Doe → (none)\n" +
				"  labels: a → (none)\n" +
				"  parent: TEST-9 → (none)\n" +
				"  updated: 2026-11-10T09:00:00.456Z\n" +
				testURL + "TEST-1\n",
		},
		{
			name: "issue-update-description-set",
			args: func(t *testing.T) []string {
				md, _, _ := guardedTokens(t)
				return []string{"issue", "update", "TEST-1", "--description=" + strings.Replace(md, "Before the image.", "Rewritten.", 1)}
			},
			deps: func(*testing.T) main.Deps {
				return testDeps(updating(guardedIssue(), guardedAfter()), &mock.UserService{})
			},
			text: "TEST-1: updated\n  description: set\n  updated: 2026-11-10T09:00:00.456Z\n" + testURL + "TEST-1\n",
		},
		{
			name: "issue-update-description-set-force",
			args: func(t *testing.T) []string {
				md, _, mention := guardedTokens(t)
				return []string{"issue", "update", "TEST-1", "--force", "--description=" + strings.Replace(md, mention, "", 1)}
			},
			deps: func(*testing.T) main.Deps {
				return testDeps(updating(guardedIssue(), guardedAfter()), &mock.UserService{})
			},
			text: "TEST-1: updated\n  description: set\n  removed: {{mention:2 Jane Doe}}\n  updated: 2026-11-10T09:00:00.456Z\n" +
				testURL + "TEST-1\n",
		},
		{
			name: "issue-update-description-append",
			args: fixedArgs("issue", "update", "TEST-1", "--append-description=More."),
			deps: func(*testing.T) main.Deps {
				return testDeps(updating(guardedIssue(), guardedAfter()), &mock.UserService{})
			},
			text: "TEST-1: updated\n  description: appended\n  updated: 2026-11-10T09:00:00.456Z\n" + testURL + "TEST-1\n",
		},
		{
			name: "issue-update-description-clear",
			args: fixedArgs("issue", "update", "TEST-1", "--clear-description", "--force"),
			deps: func(*testing.T) main.Deps {
				after := guardedAfter()
				after.Description = nil
				return testDeps(updating(guardedIssue(), after), &mock.UserService{})
			},
			text: "TEST-1: updated\n  description: cleared\n" +
				"  removed: {{mediaSingle:1 shot.png}}, {{mention:2 Jane Doe}}\n" +
				"  updated: 2026-11-10T09:00:00.456Z\n" + testURL + "TEST-1\n",
		},
		{
			name: "issue-transition",
			args: fixedArgs("issue", "transition", "TEST-1", "--status=Done"),
			deps: withServices(&mock.IssueService{
				GetFn: readSequence(beforeIssue()),
				TransitionsFn: func(context.Context, string) ([]*jira4claude.Transition, error) {
					return []*jira4claude.Transition{{ID: "11", Name: "Start", To: "In Progress"}, {ID: "31", Name: "Finish", To: "Done"}}, nil
				},
				TransitionFn: func(context.Context, string, string) error { return nil },
			}, &mock.UserService{}),
			text: "TEST-1: Backlog → Done\n" + testURL + "TEST-1\n",
		},
		{
			name: "issue-assign",
			args: fixedArgs("issue", "assign", "TEST-1", "-a", "me"),
			deps: withServices(&mock.IssueService{
				GetFn:    readSequence(beforeIssue(), afterIssue()),
				AssignFn: func(context.Context, string, string) error { return nil },
			}, me()),
			text: "TEST-1: assigned\n  assignee: Jane Doe → Me Myself\n" + testURL + "TEST-1\n",
		},
		{
			name: "issue-unassign",
			args: fixedArgs("issue", "assign", "TEST-1"),
			deps: func(*testing.T) main.Deps {
				after := beforeIssue()
				after.Assignee = nil
				return testDeps(&mock.IssueService{
					GetFn:    readSequence(beforeIssue(), after),
					AssignFn: func(context.Context, string, string) error { return nil },
				}, &mock.UserService{})
			},
			text: "TEST-1: unassigned\n  assignee: Jane Doe → (none)\n" + testURL + "TEST-1\n",
		},
		{
			name: "issue-comment",
			args: fixedArgs("issue", "comment", "TEST-1", "-b", "Hello"),
			deps: withServices(&mock.IssueService{
				AddCommentFn: func(context.Context, string, *jira4claude.ADFNode) (*jira4claude.Comment, error) {
					return &jira4claude.Comment{ID: "10001"}, nil
				},
			}, &mock.UserService{}),
			text: "TEST-1: added comment 10001\n" + testURL + "TEST-1\n",
		},
		{
			name: "issue-delete-comment",
			args: fixedArgs("issue", "delete-comment", "TEST-1", "10001"),
			deps: withServices(&mock.IssueService{
				DeleteCommentFn: func(context.Context, string, string) error { return nil },
			}, &mock.UserService{}),
			text: "TEST-1: deleted comment 10001\n" + testURL + "TEST-1\n",
		},
		{
			name: "issue-delete",
			args: fixedArgs("issue", "delete", "TEST-1"),
			deps: withServices(&mock.IssueService{
				DeleteFn: func(context.Context, string, bool) error { return nil },
			}, &mock.UserService{}),
			text: "TEST-1: deleted\n",
		},
		{
			name: "link-create",
			args: fixedArgs("link", "create", "TEST-1", "Blocks", "TEST-2"),
			deps: withServices(&mock.IssueService{
				LinkFn: func(context.Context, string, string, string) error { return nil },
			}, &mock.UserService{}),
			text: "TEST-1: linked, TEST-1 Blocks TEST-2\n" + testURL + "TEST-1\n",
		},
		{
			name: "link-delete",
			args: fixedArgs("link", "delete", "TEST-1", "TEST-2"),
			deps: withServices(&mock.IssueService{
				UnlinkFn: func(context.Context, string, string) error { return nil },
			}, &mock.UserService{}),
			text: "TEST-1: unlinked from TEST-2\n" + testURL + "TEST-1\n",
		},
		{
			name: "init",
			args: fixedArgs("init", "--server=https://test.atlassian.net", "--project=TEST"),
			deps: func(t *testing.T) main.Deps {
				t.Helper()
				deps := unreachableDeps(t)
				deps.ConfigService = &mock.ConfigService{
					InitFn: func(string, string, string) (*jira4claude.InitResult, error) {
						return &jira4claude.InitResult{ConfigCreated: true, GitignoreAdded: true}, nil
					},
				}
				return deps
			},
			text: "Created .jira4claude.yaml\nAdded .jira4claude.yaml to .gitignore\n",
		},
	}
}

func TestRun_MutationText(t *testing.T) {
	t.Parallel()

	for _, tc := range mutationCases() {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			res := runCLI(t, tc.deps(t), tc.args(t)...)

			require.Equal(t, 0, res.code, "stderr: %s", res.stderr)
			assert.Equal(t, tc.text, res.stdout)
		})
	}
}

func TestRun_MutationJSON(t *testing.T) {
	t.Parallel()

	for _, tc := range mutationCases() {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			res := runCLI(t, tc.deps(t), append([]string{"--json"}, tc.args(t)...)...)

			require.Equal(t, 0, res.code, "stderr: %s", res.stderr)
			assertOneJSONValue(t, res.stdout)
			assertGolden(t, filepath.Join(goldenDir(t), tc.name+".json"), res.stdout)
		})
	}
}

// assertOneJSONValue fails unless s holds exactly one JSON value.
func assertOneJSONValue(t *testing.T, s string) {
	t.Helper()
	dec := json.NewDecoder(strings.NewReader(s))
	var v any
	require.NoError(t, dec.Decode(&v), "stdout: %q", s)
	assert.False(t, dec.More(), "stdout must hold one JSON value: %q", s)
}

// goldenDir is the absolute path of testdata/mutation, found from this source
// file rather than the working directory, which a parallel test in this
// package changes (TestInitCmd_CreatesConfigAndUpdatesGitignore).
func goldenDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok, "cannot locate the test source")
	return filepath.Join(filepath.Dir(file), "testdata", "mutation")
}

// assertGolden compares got with the golden file at path, or rewrites the
// file when J4C_UPDATE_GOLDEN is set.
func assertGolden(t *testing.T, path, got string) {
	t.Helper()
	if os.Getenv("J4C_UPDATE_GOLDEN") != "" {
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte(got), 0o600))
		return
	}
	want, err := os.ReadFile(path)
	require.NoError(t, err, "run with J4C_UPDATE_GOLDEN=1 to create it")
	assert.Equal(t, string(want), got)
}

func TestRun_IssueUpdateChainsIfUpdated(t *testing.T) {
	t.Parallel()

	res := runCLI(t, testDeps(updating(beforeIssue(), afterIssue()), me()), "--json", "issue", "update", "TEST-1", "--summary=New summary")
	require.Equal(t, 0, res.code, "stderr: %s", res.stderr)
	var first struct {
		Updated string `json:"updated"`
	}
	require.NoError(t, json.Unmarshal([]byte(res.stdout), &first))
	require.Equal(t, "2026-11-10T09:00:00.456Z", first.Updated)

	// The next update reads the issue as the first one left it.
	var wrote bool
	svc := &mock.IssueService{
		GetFn: readSequence(afterIssue()),
		UpdateFn: func(context.Context, string, jira4claude.IssueUpdate) (*jira4claude.Issue, error) {
			wrote = true
			return afterIssue(), nil
		},
	}
	res = runCLI(t, testDeps(svc, me()), "issue", "update", "TEST-1", "--summary=Again", "--if-updated="+first.Updated)

	assert.Equal(t, 0, res.code, "stderr: %s", res.stderr)
	assert.True(t, wrote, "the printed updated time passes --if-updated")
}

func TestRun_IssueUpdateReadsBeforeEveryWrite(t *testing.T) {
	t.Parallel()

	var calls []string
	svc := &mock.IssueService{
		GetFn: func(_ context.Context, key string) (*jira4claude.Issue, error) {
			calls = append(calls, "get "+key)
			return beforeIssue(), nil
		},
		UpdateFn: func(_ context.Context, key string, _ jira4claude.IssueUpdate) (*jira4claude.Issue, error) {
			calls = append(calls, "update "+key)
			return afterIssue(), nil
		},
	}

	res := runCLI(t, testDeps(svc, &mock.UserService{}), "issue", "update", "TEST-1", "--summary=New summary")

	require.Equal(t, 0, res.code, "stderr: %s", res.stderr)
	assert.Equal(t, []string{"get TEST-1", "update TEST-1"}, calls,
		"one read for the from values; Update itself reads the issue back")
	assert.Contains(t, res.stdout, "summary: Old summary → New summary")
}

func TestRun_IssueTransitionStatus(t *testing.T) {
	t.Parallel()

	// A zero-value TransitionFn panics: a test that reaches it fails.
	transitions := func(ts ...*jira4claude.Transition) *mock.IssueService {
		return &mock.IssueService{
			TransitionsFn: func(context.Context, string) ([]*jira4claude.Transition, error) { return ts, nil },
		}
	}

	t.Run("an ambiguous status lists the choices and writes nothing", func(t *testing.T) {
		t.Parallel()

		svc := transitions(
			&jira4claude.Transition{ID: "31", Name: "Resolve", To: "Done"},
			&jira4claude.Transition{ID: "41", Name: "Close", To: "Done"},
		)

		res := runCLI(t, testDeps(svc, &mock.UserService{}), "issue", "transition", "TEST-1", "--status=Done")

		assert.Equal(t, 1, res.code)
		assert.Empty(t, res.stdout)
		assert.Contains(t, res.stderr, `"Resolve" (id 31) → Done`)
		assert.Contains(t, res.stderr, `"Close" (id 41) → Done`)
		assert.Contains(t, res.stderr, "--id")
	})

	t.Run("an ambiguous status is a JSON error on stderr", func(t *testing.T) {
		t.Parallel()

		svc := transitions(
			&jira4claude.Transition{ID: "31", Name: "Resolve", To: "Done"},
			&jira4claude.Transition{ID: "41", Name: "Close", To: "Done"},
		)

		res := runCLI(t, testDeps(svc, &mock.UserService{}), "--json", "issue", "transition", "TEST-1", "--status=Done")

		assert.Equal(t, 1, res.code)
		assert.Empty(t, res.stdout)
		var line map[string]any
		require.NoError(t, json.Unmarshal([]byte(res.stderr), &line))
		assert.Equal(t, "validation", line["code"])
		assert.Contains(t, line["message"], "ambiguous")
	})

	t.Run("the target status wins over a transition of that name", func(t *testing.T) {
		t.Parallel()

		var sent string
		svc := transitions(
			&jira4claude.Transition{ID: "21", Name: "Done", To: "Closed"},
			&jira4claude.Transition{ID: "31", Name: "Finish", To: "Done"},
		)
		svc.GetFn = readSequence(beforeIssue())
		svc.TransitionFn = func(_ context.Context, _, id string) error {
			sent = id
			return nil
		}

		res := runCLI(t, testDeps(svc, &mock.UserService{}), "issue", "transition", "TEST-1", "--status=done")

		require.Equal(t, 0, res.code, "stderr: %s", res.stderr)
		assert.Equal(t, "31", sent)
		assert.Equal(t, "TEST-1: Backlog → Done\n"+testURL+"TEST-1\n", res.stdout)
	})

	t.Run("--id that is not available lists the choices and writes nothing", func(t *testing.T) {
		t.Parallel()

		svc := transitions(&jira4claude.Transition{ID: "31", Name: "Finish", To: "Done"})

		res := runCLI(t, testDeps(svc, &mock.UserService{}), "issue", "transition", "TEST-1", "--id=99")

		assert.Equal(t, 1, res.code)
		assert.Empty(t, res.stdout)
		assert.Contains(t, res.stderr, `"Finish" (id 31) → Done`)
	})

	t.Run("--id reports the transition's target status", func(t *testing.T) {
		t.Parallel()

		svc := transitions(&jira4claude.Transition{ID: "31", Name: "Finish", To: "Done"})
		svc.GetFn = readSequence(beforeIssue())
		svc.TransitionFn = func(context.Context, string, string) error { return nil }

		res := runCLI(t, testDeps(svc, &mock.UserService{}), "issue", "transition", "TEST-1", "--id=31")

		require.Equal(t, 0, res.code, "stderr: %s", res.stderr)
		assert.Equal(t, "TEST-1: Backlog → Done\n"+testURL+"TEST-1\n", res.stdout)
	})
}

func TestRun_IssueUpdateRemovedUnderForce(t *testing.T) {
	t.Parallel()

	t.Run("a token used twice removes nothing", func(t *testing.T) {
		t.Parallel()

		md, _, mention := guardedTokens(t)
		edited := md + "\n\nAgain " + mention

		res := runCLI(t, testDeps(updating(guardedIssue(), guardedAfter()), &mock.UserService{}),
			"--json", "issue", "update", "TEST-1", "--force", "--description="+edited)

		require.Equal(t, 0, res.code, "stderr: %s", res.stderr)
		assert.Contains(t, res.stderr, "more than once", "the duplicate is still a warning")
		var result map[string]any
		require.NoError(t, json.Unmarshal([]byte(res.stdout), &result))
		assert.NotContains(t, result, "removed")
	})

	t.Run("the removed nodes are also warnings on stderr", func(t *testing.T) {
		t.Parallel()

		md, media, _ := guardedTokens(t)

		res := runCLI(t, testDeps(updating(guardedIssue(), guardedAfter()), &mock.UserService{}),
			"issue", "update", "TEST-1", "--force", "--description="+strings.Replace(md, media, "", 1))

		require.Equal(t, 0, res.code, "stderr: %s", res.stderr)
		assert.Contains(t, res.stderr, "[warn] dropped "+media)
		assert.Contains(t, res.stdout, "  removed: "+media+"\n")
	})
}

// Golden files must end with the encoder's newline, so that a script reading
// stdout gets one complete line.
func TestMutationGoldenFilesEndWithNewline(t *testing.T) {
	t.Parallel()

	paths, err := filepath.Glob(filepath.Join(goldenDir(t), "*.json"))
	require.NoError(t, err)
	require.Len(t, paths, len(mutationCases()))
	for _, path := range paths {
		data, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.True(t, bytes.HasSuffix(data, []byte("}\n")), path)
	}
}

func TestRun_MutationReadFailures(t *testing.T) {
	t.Parallel()

	notFound := &jira4claude.Error{Code: jira4claude.ENotFound, Message: "issue does not exist"}

	t.Run("a failing read stops an update before it writes", func(t *testing.T) {
		t.Parallel()

		// A zero-value UpdateFn panics: reaching it fails the test.
		svc := &mock.IssueService{GetFn: func(context.Context, string) (*jira4claude.Issue, error) { return nil, notFound }}

		res := runCLI(t, testDeps(svc, &mock.UserService{}), "issue", "update", "TEST-1", "--summary=New")

		assert.Equal(t, 4, res.code)
		assert.Empty(t, res.stdout)
	})

	// As with create's assignee fallback: the write happened, so stdout still
	// reports it, with the account ID that was sent; the failure is on stderr.
	t.Run("an assignment whose read-back fails is reported, and fails", func(t *testing.T) {
		t.Parallel()

		reads := 0
		svc := &mock.IssueService{
			GetFn: func(context.Context, string) (*jira4claude.Issue, error) {
				reads++
				if reads > 1 {
					return nil, notFound
				}
				return beforeIssue(), nil
			},
			AssignFn: func(context.Context, string, string) error { return nil },
		}

		res := runCLI(t, testDeps(svc, me()), "issue", "assign", "TEST-1", "-a", "me")

		assert.Equal(t, 4, res.code)
		assert.Equal(t, "TEST-1: assigned\n  assignee: Jane Doe → u-me\n"+testURL+"TEST-1\n", res.stdout)
		assert.Equal(t, "[error] assigned TEST-1, but reading it back failed: issue does not exist\n", res.stderr)
	})
}

func TestRun_IssueTransitionsShowTargets(t *testing.T) {
	t.Parallel()

	svc := func() *mock.IssueService {
		return &mock.IssueService{TransitionsFn: func(context.Context, string) ([]*jira4claude.Transition, error) {
			return []*jira4claude.Transition{{ID: "11", Name: "Start", To: "In Progress"}, {ID: "31", Name: "Finish", To: "Done"}}, nil
		}}
	}

	t.Run("text", func(t *testing.T) {
		t.Parallel()

		res := runCLI(t, testDeps(svc(), &mock.UserService{}), "issue", "transitions", "TEST-1")

		require.Equal(t, 0, res.code, "stderr: %s", res.stderr)
		assert.Equal(t, "11\tStart\tIn Progress\n31\tFinish\tDone\n", res.stdout)
		assert.Empty(t, res.stderr)
	})

	t.Run("json", func(t *testing.T) {
		t.Parallel()

		res := runCLI(t, testDeps(svc(), &mock.UserService{}), "--json", "issue", "transitions", "TEST-1")

		require.Equal(t, 0, res.code, "stderr: %s", res.stderr)
		assert.JSONEq(t, `[{"id":"11","name":"Start","to":"In Progress"},{"id":"31","name":"Finish","to":"Done"}]`, res.stdout)
		assert.Empty(t, res.stderr)
	})
}
