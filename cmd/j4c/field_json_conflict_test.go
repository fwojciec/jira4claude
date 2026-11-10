package main_test

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/fwojciec/jira4claude"
	"github.com/fwojciec/jira4claude/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A --field-json for a field a typed flag of the same command also sets is
// ambiguous (the raw value would win in the request, unseen by the result and
// the dry run), so it is refused before any service call. A --field-json for
// the description is always refused: a raw ADF write would skip re-attachment,
// the --if-updated loss checks and the dropped-content refusal.

func TestRun_FieldJSONConflicts(t *testing.T) {
	t.Parallel()

	type conflictCase struct {
		name    string
		args    []string
		message string
	}
	pairs := []conflictCase{
		{"create --summary", []string{"issue", "create", "-s", "New", `--field-json=summary="Other"`},
			"--field-json summary and --summary both set summary; give one of them"},
		{"create --type", []string{"issue", "create", "-s", "New", "--type=Bug", `--field-json=issuetype={"id":"1"}`},
			"--field-json issuetype and --type both set issuetype; give one of them"},
		{"create --project", []string{"issue", "create", "-s", "New", "--project=TEST", `--field-json=project={"id":"1"}`},
			"--field-json project and --project both set project; give one of them"},
		{"create --priority", []string{"issue", "create", "-s", "New", "--priority=High", `--field-json=priority={"name":"Low"}`},
			"--field-json priority and --priority both set priority; give one of them"},
		{"create --labels", []string{"issue", "create", "-s", "New", "--labels=a", `--field-json=labels=["b"]`},
			"--field-json labels and --labels both set labels; give one of them"},
		{"create --parent", []string{"issue", "create", "-s", "New", "--parent=TEST-9", `--field-json=parent={"key":"TEST-8"}`},
			"--field-json parent and --parent both set parent; give one of them"},
		{"create --assignee", []string{"issue", "create", "-s", "New", "--assignee=me", `--field-json=assignee={"id":"u"}`},
			"--field-json assignee and --assignee both set assignee; give one of them"},
		{"create, field ID in another case", []string{"issue", "create", "-s", "New", `--field-json=Summary="Other"`},
			"--field-json Summary and --summary both set summary; give one of them"},
		{"update --summary", []string{"issue", "update", "TEST-1", "--summary=New", `--field-json=summary="Other"`},
			"--field-json summary and --summary both set summary; give one of them"},
		{"update --priority", []string{"issue", "update", "TEST-1", "--priority=High", `--field-json=priority={"name":"Low"}`},
			"--field-json priority and --priority both set priority; give one of them"},
		{"update --assignee", []string{"issue", "update", "TEST-1", "--assignee=me", `--field-json=assignee={"accountId":"u"}`},
			"--field-json assignee and --assignee both set assignee; give one of them"},
		{"update unassign", []string{"issue", "update", "TEST-1", "--assignee=", `--field-json=assignee=null`},
			"--field-json assignee and --assignee both set assignee; give one of them"},
		{"update --labels", []string{"issue", "update", "TEST-1", "--labels=a", `--field-json=labels=["b"]`},
			"--field-json labels and --labels both set labels; give one of them"},
		{"update --clear-labels", []string{"issue", "update", "TEST-1", "--clear-labels", `--field-json=labels=["b"]`},
			"--field-json labels and --clear-labels both set labels; give one of them"},
		{"update --parent", []string{"issue", "update", "TEST-1", "--parent=TEST-9", `--field-json=parent={"key":"TEST-8"}`},
			"--field-json parent and --parent both set parent; give one of them"},
		{"update --clear-parent", []string{"issue", "update", "TEST-1", "--clear-parent", `--field-json=parent=null`},
			"--field-json parent and --clear-parent both set parent; give one of them"},
	}
	const description = "--field-json cannot set the description: a raw document skips re-attaching existing images and mentions, " +
		"the checks for lost content and the dropped-content refusal; use --description or --description-file"
	descriptions := [][]string{
		{"issue", "create", "-s", "New", `--field-json=description={"type":"doc","version":1,"content":[]}`},
		{"issue", "create", "-s", "New", "-d", "Text", `--field-json=description=null`},
		{"issue", "update", "TEST-1", `--field-json=description={"type":"doc","version":1,"content":[]}`},
		{"issue", "update", "TEST-1", "--description=Text", `--field-json=description=null`},
		{"issue", "update", "TEST-1", "--append-description=Text", `--field-json=description=null`},
		{"issue", "update", "TEST-1", "--clear-description", `--field-json=description=null`},
		{"issue", "update", "TEST-1", `--field-json=Description=null`},
	}
	cases := make([]conflictCase, 0, len(pairs)+len(descriptions))
	cases = append(cases, pairs...)
	for i, args := range descriptions {
		cases = append(cases, conflictCase{fmt.Sprintf("description %d: %s", i, strings.Join(args[1:], " ")), args, description})
	}

	for _, tc := range cases {
		for _, dry := range []bool{false, true} {
			args := tc.args
			name := tc.name
			if dry {
				args = append(append([]string(nil), args...), "--dry-run")
				name += " --dry-run"
			}

			t.Run(name+"/text", func(t *testing.T) {
				t.Parallel()

				// Zero-value services panic on any call: none may happen.
				res := runCLI(t, testDeps(&mock.IssueService{}, &mock.UserService{}), args...)

				assert.Equal(t, 1, res.code)
				assert.Empty(t, res.stdout)
				assert.Equal(t, "[error] "+tc.message+"\n", res.stderr)
			})

			t.Run(name+"/json", func(t *testing.T) {
				t.Parallel()

				res := runCLI(t, testDeps(&mock.IssueService{}, &mock.UserService{}), append([]string{"--json"}, args...)...)

				assert.Equal(t, 1, res.code)
				assert.Empty(t, res.stdout)
				assertJSONError(t, res.stderr, jira4claude.EValidation, tc.message)
			})
		}
	}
}

func TestRun_FieldJSONWithoutConflict(t *testing.T) {
	t.Parallel()

	t.Run("create: a custom field and unpaired built-ins are sent", func(t *testing.T) {
		t.Parallel()

		rec := &recorder{}
		res := runCLI(t, testDeps(rec.service(nil), &mock.UserService{}), "issue", "create", "-s", "New",
			`--field-json=customfield_1={"value":"x"}`, `--field-json=issuetype={"id":"10001"}`,
			`--field-json=project={"id":"100"}`, `--field-json=duedate="2026-12-01"`)

		require.Equal(t, 0, res.code, "stderr: %s", res.stderr)
		assert.Equal(t, map[string]json.RawMessage{
			"customfield_1": json.RawMessage(`{"value":"x"}`),
			"issuetype":     json.RawMessage(`{"id":"10001"}`),
			"project":       json.RawMessage(`{"id":"100"}`),
			"duedate":       json.RawMessage(`"2026-12-01"`),
		}, rec.created.CustomFields)
		assert.Equal(t, "Task", rec.created.Type, "without --type the type is still Task, which the raw value overrides")
	})

	for _, dry := range []bool{false, true} {
		name := "update: an unpaired built-in is reported as set"
		args := []string{"--json", "issue", "update", "TEST-1", "--summary=New", `--field-json=priority={"name":"Low"}`,
			`--field-json=customfield_10801={"value":"High"}`}
		if dry {
			name += " (dry run)"
			args = append(args, "--dry-run")
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			svc := updating(beforeIssue(), afterIssue())
			if dry {
				svc = noWrites(t, &mock.IssueService{GetFn: readSequence(beforeIssue())})
			}
			res := runCLI(t, testDeps(svc, &mock.UserService{}), args...)

			require.Equal(t, 0, res.code, "stderr: %s", res.stderr)
			var result struct {
				Changes map[string]json.RawMessage `json:"changes"`
			}
			require.NoError(t, json.Unmarshal([]byte(res.stdout), &result))
			assert.JSONEq(t, `{"edit":"set"}`, string(result.Changes["priority"]))
			assert.Contains(t, result.Changes, "summary")
			assert.Contains(t, result.Changes, "customfield_10801")
		})
	}
}
