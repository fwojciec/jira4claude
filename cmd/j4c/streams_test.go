package main_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/fwojciec/jira4claude"
	main "github.com/fwojciec/jira4claude/cmd/j4c"
	"github.com/fwojciec/jira4claude/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Streams and exit codes, for every command in both output modes: stdout
// carries data only, errors and warnings go to stderr (one JSON object per
// line in --json mode), and the exit code follows the error code.

// commandCase is one command driven through Run. deps builds dependencies in
// which the command's service call returns err.
type commandCase struct {
	name string
	args []string
	deps func(t *testing.T, err error) main.Deps
}

// everyCommand lists every j4c command with a service call that fails with err.
func everyCommand() []commandCase {
	failing := func(build func(err error) *mock.IssueService) func(t *testing.T, err error) main.Deps {
		return func(t *testing.T, err error) main.Deps {
			t.Helper()
			return testDeps(build(err), &mock.UserService{})
		}
	}
	return []commandCase{
		{"issue view", []string{"issue", "view", "TEST-1"}, failing(func(err error) *mock.IssueService {
			return issueGetter(nil, err)
		})},
		{"issue view --field", []string{"issue", "view", "TEST-1", "--field", "status"}, failing(func(err error) *mock.IssueService {
			return issueGetter(nil, err)
		})},
		{"issue list", []string{"issue", "list"}, failing(func(err error) *mock.IssueService {
			return &mock.IssueService{ListFn: func(context.Context, jira4claude.IssueFilter) ([]*jira4claude.Issue, string, error) {
				return nil, "", err
			}}
		})},
		{"issue ready", []string{"issue", "ready"}, failing(func(err error) *mock.IssueService {
			return &mock.IssueService{ListFn: func(context.Context, jira4claude.IssueFilter) ([]*jira4claude.Issue, string, error) {
				return nil, "", err
			}}
		})},
		{"issue create", []string{"issue", "create", "-s", "New"}, failing(func(err error) *mock.IssueService {
			return &mock.IssueService{CreateFn: func(context.Context, *jira4claude.Issue) (*jira4claude.Issue, error) {
				return nil, err
			}}
		})},
		{"issue update", []string{"issue", "update", "TEST-1", "--summary=New"}, failing(func(err error) *mock.IssueService {
			return &mock.IssueService{GetFn: readsKey, UpdateFn: func(context.Context, string, jira4claude.IssueUpdate) (*jira4claude.Issue, error) {
				return nil, err
			}}
		})},
		{"issue delete", []string{"issue", "delete", "TEST-1"}, failing(func(err error) *mock.IssueService {
			return &mock.IssueService{DeleteFn: func(context.Context, string, bool) error { return err }}
		})},
		{"issue fields (create)", []string{"issue", "fields"}, failing(func(err error) *mock.IssueService {
			return &mock.IssueService{GetCreateFieldsFn: func(context.Context, string, string) ([]*jira4claude.IssueField, error) {
				return nil, err
			}}
		})},
		{"issue fields (edit)", []string{"issue", "fields", "--key=TEST-1"}, failing(func(err error) *mock.IssueService {
			return &mock.IssueService{GetEditFieldsFn: func(context.Context, string) ([]*jira4claude.IssueField, error) {
				return nil, err
			}}
		})},
		{"issue transitions", []string{"issue", "transitions", "TEST-1"}, failing(func(err error) *mock.IssueService {
			return &mock.IssueService{TransitionsFn: func(context.Context, string) ([]*jira4claude.Transition, error) {
				return nil, err
			}}
		})},
		{"issue transition", []string{"issue", "transition", "TEST-1", "--id=31"}, failing(func(err error) *mock.IssueService {
			return &mock.IssueService{
				GetFn: readsKey,
				TransitionsFn: func(context.Context, string) ([]*jira4claude.Transition, error) {
					return []*jira4claude.Transition{{ID: "31", Name: "Finish", To: "Done"}}, nil
				},
				TransitionFn: func(context.Context, string, string) error { return err },
			}
		})},
		{"issue assign", []string{"issue", "assign", "TEST-1", "-a", "me"}, func(t *testing.T, err error) main.Deps {
			t.Helper()
			return testDeps(&mock.IssueService{}, &mock.UserService{
				GetMyselfFn: func(context.Context) (*jira4claude.User, error) { return nil, err },
			})
		}},
		{"issue comment", []string{"issue", "comment", "TEST-1", "-b", "Hello"}, failing(func(err error) *mock.IssueService {
			return &mock.IssueService{AddCommentFn: func(context.Context, string, *jira4claude.ADFNode) (*jira4claude.Comment, error) {
				return nil, err
			}}
		})},
		{"issue delete-comment", []string{"issue", "delete-comment", "TEST-1", "10001"}, failing(func(err error) *mock.IssueService {
			return &mock.IssueService{DeleteCommentFn: func(context.Context, string, string) error { return err }}
		})},
		{"link create", []string{"link", "create", "TEST-1", "Blocks", "TEST-2"}, failing(func(err error) *mock.IssueService {
			return &mock.IssueService{LinkFn: func(context.Context, string, string, string) error { return err }}
		})},
		{"link delete", []string{"link", "delete", "TEST-1", "TEST-2"}, failing(func(err error) *mock.IssueService {
			return &mock.IssueService{UnlinkFn: func(context.Context, string, string) error { return err }}
		})},
		{"link list", []string{"link", "list", "TEST-1"}, failing(func(err error) *mock.IssueService {
			return issueGetter(nil, err)
		})},
		{"attachment list", []string{"attachment", "list", "TEST-1"}, func(t *testing.T, err error) main.Deps {
			t.Helper()
			return attachmentDeps(t, attachmentLister(nil, err))
		}},
		{"init", []string{"init", "--server=https://test.atlassian.net", "--project=TEST"}, func(t *testing.T, err error) main.Deps {
			t.Helper()
			deps := unreachableDeps(t)
			deps.ConfigService = &mock.ConfigService{
				InitFn: func(string, string, string) (*jira4claude.InitResult, error) { return nil, err },
			}
			return deps
		}},
	}
}

// errorCodes pins the exit code of every error code in error.go. The internal
// row uses a plain Go error: anything that is not a jira4claude.Error is
// internal.
func errorCodes() []struct {
	code string
	exit int
	err  error
} {
	appErr := func(code string) error {
		return &jira4claude.Error{Code: code, Message: "failed with " + code}
	}
	return []struct {
		code string
		exit int
		err  error
	}{
		{jira4claude.EValidation, 1, appErr(jira4claude.EValidation)},
		{jira4claude.EUnauthorized, 2, appErr(jira4claude.EUnauthorized)},
		{jira4claude.EForbidden, 3, appErr(jira4claude.EForbidden)},
		{jira4claude.ENotFound, 4, appErr(jira4claude.ENotFound)},
		{jira4claude.EConflict, 5, appErr(jira4claude.EConflict)},
		{jira4claude.ERateLimit, 6, appErr(jira4claude.ERateLimit)},
		{jira4claude.EInternal, 7, errors.New("failed with " + jira4claude.EInternal)},
	}
}

func TestRun_ServiceErrorStreams(t *testing.T) {
	t.Parallel()

	for _, cmd := range everyCommand() {
		for _, ec := range errorCodes() {
			message := "failed with " + ec.code

			t.Run(cmd.name+"/"+ec.code+"/text", func(t *testing.T) {
				t.Parallel()

				res := runCLI(t, cmd.deps(t, ec.err), cmd.args...)

				assert.Equal(t, ec.exit, res.code)
				assert.Empty(t, res.stdout)
				assert.Equal(t, "[error] "+message+"\n", res.stderr)
			})

			t.Run(cmd.name+"/"+ec.code+"/json", func(t *testing.T) {
				t.Parallel()

				res := runCLI(t, cmd.deps(t, ec.err), append([]string{"--json"}, cmd.args...)...)

				assert.Equal(t, ec.exit, res.code)
				assert.Empty(t, res.stdout)
				assertJSONError(t, res.stderr, ec.code, message)
			})
		}
	}
}

// degradedMarkdown converts with a degraded (not dropped) conversion issue: a
// heading cannot sit in a list item, so it becomes a paragraph.
const degradedMarkdown = "- # Heading"

// degradedIssue returns an issue whose description reads back with a degraded
// conversion issue: a code block language holding whitespace keeps only its
// first word.
func degradedIssue() *jira4claude.Issue {
	return &jira4claude.Issue{
		Key:     "TEST-1",
		Summary: "First issue",
		Status:  "To Do",
		Type:    "Task",
		Description: &jira4claude.ADFNode{Type: "doc", Version: 1, Content: []jira4claude.ADFNode{{
			Type:    "codeBlock",
			Attrs:   json.RawMessage(`{"language":"go lang"}`),
			Content: []jira4claude.ADFNode{{Type: "text", Text: "x := 1"}},
		}}},
	}
}

// warningCase is a command that succeeds while warning.
type warningCase struct {
	name     string
	args     []string
	services func() (*mock.IssueService, *mock.UserService)
	// textWarning is a prefix of a [warn] line on stderr in text mode.
	textWarning string
	// textStdout is a prefix of stdout in text mode.
	textStdout string
	// jsonWarning holds keys every JSON warning line on stderr must carry.
	jsonWarning map[string]any
}

func warningCases() []warningCase {
	degraded := func(node string) map[string]any {
		return map[string]any{"level": "warning", "kind": jira4claude.ConversionDegraded, "node": node}
	}
	// A view warning also names its issue.
	viewDegraded := func(node string) map[string]any {
		w := degraded(node)
		w["key"] = "TEST-1"
		return w
	}
	createSvc := func() (*mock.IssueService, *mock.UserService) {
		return &mock.IssueService{CreateFn: func(context.Context, *jira4claude.Issue) (*jira4claude.Issue, error) {
			return &jira4claude.Issue{Key: "TEST-1"}, nil
		}}, &mock.UserService{}
	}
	truncatedList := func() (*mock.IssueService, *mock.UserService) {
		return &mock.IssueService{ListFn: func(context.Context, jira4claude.IssueFilter) ([]*jira4claude.Issue, string, error) {
			return []*jira4claude.Issue{{Key: "TEST-1", Summary: "First", Status: "To Do"}}, "next-page", nil
		}}, &mock.UserService{}
	}
	return []warningCase{
		{
			name: "issue view reads degraded content",
			args: []string{"issue", "view", "TEST-1"},
			services: func() (*mock.IssueService, *mock.UserService) {
				return issueGetter(degradedIssue(), nil), &mock.UserService{}
			},
			textWarning: "[warn] degraded codeBlock",
			textStdout:  "# TEST-1: First issue\n",
			jsonWarning: viewDegraded("codeBlock"),
		},
		{
			name: "issue view --field reads degraded content",
			args: []string{"issue", "view", "TEST-1", "--field", "description"},
			services: func() (*mock.IssueService, *mock.UserService) {
				return issueGetter(degradedIssue(), nil), &mock.UserService{}
			},
			textWarning: "[warn] degraded codeBlock",
			textStdout:  "```go\n",
			jsonWarning: viewDegraded("codeBlock"),
		},
		{
			name:        "issue create writes degraded content",
			args:        []string{"issue", "create", "-s", "New", "--description=" + degradedMarkdown},
			services:    createSvc,
			textWarning: "[warn] degraded heading",
			textStdout:  "TEST-1: created\n",
			jsonWarning: degraded("heading"),
		},
		{
			name: "issue update writes degraded content",
			args: []string{"issue", "update", "TEST-1", "--description=" + degradedMarkdown},
			services: func() (*mock.IssueService, *mock.UserService) {
				return &mock.IssueService{
					GetFn: func(context.Context, string) (*jira4claude.Issue, error) {
						return &jira4claude.Issue{Key: "TEST-1"}, nil
					},
					UpdateFn: func(context.Context, string, jira4claude.IssueUpdate) (*jira4claude.Issue, error) {
						return &jira4claude.Issue{Key: "TEST-1"}, nil
					},
				}, &mock.UserService{}
			},
			textWarning: "[warn] degraded heading",
			textStdout:  "TEST-1: updated\n",
			jsonWarning: degraded("heading"),
		},
		{
			name: "issue comment writes degraded content",
			args: []string{"issue", "comment", "TEST-1", "--body=" + degradedMarkdown},
			services: func() (*mock.IssueService, *mock.UserService) {
				return &mock.IssueService{AddCommentFn: func(context.Context, string, *jira4claude.ADFNode) (*jira4claude.Comment, error) {
					return &jira4claude.Comment{ID: "10001"}, nil
				}}, &mock.UserService{}
			},
			textWarning: "[warn] degraded heading",
			textStdout:  "TEST-1: added comment 10001\n",
			jsonWarning: degraded("heading"),
		},
		{
			name:        "issue create with --force writes past dropped content",
			args:        []string{"issue", "create", "-s", "New", "-d", imageMarkdown, "--force"},
			services:    createSvc,
			textWarning: "[warn] dropped Image",
			textStdout:  "TEST-1: created\n",
			jsonWarning: map[string]any{"level": "warning", "kind": jira4claude.ConversionDropped, "node": "Image"},
		},
		{
			name:        "issue list is truncated",
			args:        []string{"issue", "list", "--limit=1"},
			services:    truncatedList,
			textWarning: "[warn] showing the first 1 issues; more match",
			textStdout:  "TEST-1\t",
			jsonWarning: map[string]any{"level": "warning", "message": "showing the first 1 issues; more match (raise --limit to see them)"},
		},
		{
			name:        "issue ready is truncated",
			args:        []string{"issue", "ready", "--limit=1"},
			services:    truncatedList,
			textWarning: "[warn] showing the first 1 ready issues",
			textStdout:  "TEST-1\t",
			jsonWarning: map[string]any{"level": "warning", "message": "showing the first 1 ready issues; more may exist (raise --limit to see them)"},
		},
	}
}

func TestRun_WarningStreams(t *testing.T) {
	t.Parallel()

	for _, tc := range warningCases() {
		t.Run(tc.name+"/text", func(t *testing.T) {
			t.Parallel()

			issueSvc, userSvc := tc.services()
			res := runCLI(t, testDeps(issueSvc, userSvc), tc.args...)

			assert.Equal(t, 0, res.code, "stderr: %q", res.stderr)
			assert.True(t, strings.HasPrefix(res.stdout, tc.textStdout), "stdout: %q", res.stdout)
			assert.NotContains(t, res.stdout, "[warn]")
			lines := strings.Split(strings.TrimSuffix(res.stderr, "\n"), "\n")
			require.NotEmpty(t, res.stderr)
			for _, line := range lines {
				assert.True(t, strings.HasPrefix(line, "[warn] "), "stderr line: %q", line)
			}
			assert.Contains(t, res.stderr, tc.textWarning)
		})

		t.Run(tc.name+"/json", func(t *testing.T) {
			t.Parallel()

			issueSvc, userSvc := tc.services()
			res := runCLI(t, testDeps(issueSvc, userSvc), append([]string{"--json"}, tc.args...)...)

			assert.Equal(t, 0, res.code, "stderr: %q", res.stderr)
			// stdout is the data alone: one JSON value with nothing after it.
			var data map[string]any
			require.NoError(t, json.Unmarshal([]byte(res.stdout), &data), "stdout: %q", res.stdout)
			require.NotEmpty(t, res.stderr)
			for _, line := range strings.Split(strings.TrimSuffix(res.stderr, "\n"), "\n") {
				var warning map[string]any
				require.NoError(t, json.Unmarshal([]byte(line), &warning), "stderr line: %q", line)
				for k, v := range tc.jsonWarning {
					assert.Equal(t, v, warning[k], "key %q in stderr line %q", k, line)
				}
				assert.NotEmpty(t, warning["message"])
			}
		})
	}
}

// emptyCase is a list-like command whose result is empty.
type emptyCase struct {
	name    string
	args    []string
	svc     func() *mock.IssueService
	notice  string // the text-mode [info] line on stderr
	jsonOut string // the --json data on stdout

	// deps, when set, builds the dependencies instead of svc, for a command
	// that does not use the issue service.
	deps func(t *testing.T) main.Deps
}

// build returns the dependencies of an empty case.
func (tc emptyCase) build(t *testing.T) main.Deps {
	t.Helper()
	if tc.deps != nil {
		return tc.deps(t)
	}
	return testDeps(tc.svc(), &mock.UserService{})
}

func emptyCases() []emptyCase {
	noIssues := func() *mock.IssueService {
		return &mock.IssueService{ListFn: func(context.Context, jira4claude.IssueFilter) ([]*jira4claude.Issue, string, error) {
			return nil, "", nil
		}}
	}
	noCreateFields := func() *mock.IssueService {
		return &mock.IssueService{GetCreateFieldsFn: func(context.Context, string, string) ([]*jira4claude.IssueField, error) {
			return nil, nil
		}}
	}
	return []emptyCase{
		{
			name: "issue list", args: []string{"issue", "list"}, svc: noIssues,
			notice:  "[info] No issues found\n",
			jsonOut: `{"issues":[],"truncated":false}`,
		},
		{
			name: "issue ready", args: []string{"issue", "ready"}, svc: noIssues,
			notice:  "[info] No issues found\n",
			jsonOut: `{"issues":[],"truncated":false}`,
		},
		{
			name: "issue transitions", args: []string{"issue", "transitions", "TEST-1"},
			svc: func() *mock.IssueService {
				return &mock.IssueService{TransitionsFn: func(context.Context, string) ([]*jira4claude.Transition, error) {
					return nil, nil
				}}
			},
			notice:  "[info] No transitions available for TEST-1\n",
			jsonOut: `[]`,
		},
		{
			name: "issue fields (create)", args: []string{"issue", "fields"}, svc: noCreateFields,
			notice:  "[info] No fields for TEST / Task\n",
			jsonOut: `{"source":"TEST / Task","scope":"default","omitted":0,"fields":[]}`,
		},
		{
			name: "issue fields (edit)", args: []string{"issue", "fields", "--key=TEST-1"},
			svc: func() *mock.IssueService {
				return &mock.IssueService{GetEditFieldsFn: func(context.Context, string) ([]*jira4claude.IssueField, error) {
					return nil, nil
				}}
			},
			notice:  "[info] No fields for TEST-1 (edit)\n",
			jsonOut: `{"source":"TEST-1 (edit)","scope":"default","omitted":0,"fields":[]}`,
		},
		{
			name: "issue fields --filter with no match", args: []string{"issue", "fields", "--filter=nothing"}, svc: noCreateFields,
			notice:  "[info] No fields match the filter for TEST / Task\n",
			jsonOut: `{"source":"TEST / Task","scope":"filtered","omitted":0,"fields":[]}`,
		},
		{
			name: "link list", args: []string{"link", "list", "TEST-1"},
			svc: func() *mock.IssueService {
				return issueGetter(&jira4claude.Issue{Key: "TEST-1"}, nil)
			},
			notice:  "[info] No links for TEST-1\n",
			jsonOut: `[]`,
		},
		{
			name: "attachment list", args: []string{"attachment", "list", "TEST-1"},
			deps: func(t *testing.T) main.Deps {
				t.Helper()
				return attachmentDeps(t, attachmentLister(nil, nil))
			},
			notice:  "[info] No attachments for TEST-1\n",
			jsonOut: `[]`,
		},
	}
}

func TestRun_EmptyResultStreams(t *testing.T) {
	t.Parallel()

	for _, tc := range emptyCases() {
		t.Run(tc.name+"/text", func(t *testing.T) {
			t.Parallel()

			res := runCLI(t, tc.build(t), tc.args...)

			assert.Equal(t, 0, res.code)
			assert.Empty(t, res.stdout, "an empty result prints no data")
			assert.Equal(t, tc.notice, res.stderr)
		})

		t.Run(tc.name+"/json", func(t *testing.T) {
			t.Parallel()

			res := runCLI(t, tc.build(t), append([]string{"--json"}, tc.args...)...)

			assert.Equal(t, 0, res.code)
			assert.JSONEq(t, tc.jsonOut, res.stdout)
			assert.Empty(t, res.stderr)
		})
	}
}

// noisyIssue returns TEST-1 with one meaningful custom field and three that
// issue view hides: an empty list, an empty string and the rank field type.
func noisyIssue() *jira4claude.Issue {
	issue := bareIssue()
	issue.ReadCustomFields = map[string]jira4claude.CustomFieldValue{
		"customfield_10016": {Name: "Story Points", Value: json.RawMessage(`5`), SchemaType: "number"},
		"customfield_10020": {Name: "Sprint", Value: json.RawMessage(`[]`), SchemaType: "array"},
		"customfield_10021": {Name: "Flagged", Value: json.RawMessage(`""`)},
		"customfield_10019": {Name: "Rank", Value: json.RawMessage(`"0|i0004v:"`), SchemaType: "any", SchemaCustom: "com.pyxis.greenhopper.jira:gh-lexo-rank"},
	}
	return issue
}

func TestRun_HiddenCustomFieldsNotice(t *testing.T) {
	t.Parallel()

	t.Run("text mode prints the shown fields on stdout and the hidden count on stderr", func(t *testing.T) {
		t.Parallel()

		res := runCLI(t, testDeps(issueGetter(noisyIssue(), nil), &mock.UserService{}), "issue", "view", "TEST-1")

		assert.Equal(t, 0, res.code)
		assert.Contains(t, res.stdout, "- **Story Points:** 5\n")
		assert.NotContains(t, res.stdout, "Rank")
		assert.NotContains(t, res.stdout, "[info]")
		assert.Equal(t, "[info] 3 empty or machine-only custom field(s) hidden — use --all-fields to show all.\n", res.stderr)
	})

	t.Run("json mode keeps every field and prints no notice", func(t *testing.T) {
		t.Parallel()

		res := runCLI(t, testDeps(issueGetter(noisyIssue(), nil), &mock.UserService{}), "--json", "issue", "view", "TEST-1")

		assert.Equal(t, 0, res.code)
		var got struct {
			CustomFields map[string]json.RawMessage `json:"customFields"`
		}
		require.NoError(t, json.Unmarshal([]byte(res.stdout), &got))
		assert.Len(t, got.CustomFields, 4)
		assert.JSONEq(t, `{"name":"Rank","value":"0|i0004v:"}`, string(got.CustomFields["customfield_10019"]),
			"the schema types stay out of the JSON")
		assert.Empty(t, res.stderr)
	})
}

func TestRun_HiddenFieldsNotice(t *testing.T) {
	t.Parallel()

	svc := func() *mock.IssueService {
		return &mock.IssueService{GetCreateFieldsFn: func(context.Context, string, string) ([]*jira4claude.IssueField, error) {
			return []*jira4claude.IssueField{
				{ID: "summary", Name: "Summary", Required: true, SchemaType: "string"},
				{ID: "description", Name: "Description", SchemaType: "string"},
			}, nil
		}}
	}

	t.Run("text mode prints the fields on stdout and the hidden count on stderr", func(t *testing.T) {
		t.Parallel()

		res := runCLI(t, testDeps(svc(), &mock.UserService{}), "issue", "fields")

		assert.Equal(t, 0, res.code)
		assert.Contains(t, res.stdout, "summary")
		assert.NotContains(t, res.stdout, "[info]")
		assert.Equal(t, "[info] 1 more field(s) hidden — use --all to show all, or --filter to find one.\n", res.stderr)
	})

	t.Run("json mode carries the hidden count in the data only", func(t *testing.T) {
		t.Parallel()

		res := runCLI(t, testDeps(svc(), &mock.UserService{}), "--json", "issue", "fields")

		assert.Equal(t, 0, res.code)
		var got struct {
			Omitted int `json:"omitted"`
		}
		require.NoError(t, json.Unmarshal([]byte(res.stdout), &got))
		assert.Equal(t, 1, got.Omitted)
		assert.Empty(t, res.stderr)
	})
}
