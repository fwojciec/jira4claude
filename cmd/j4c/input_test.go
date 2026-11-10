package main_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/fwojciec/jira4claude"
	main "github.com/fwojciec/jira4claude/cmd/j4c"
	"github.com/fwojciec/jira4claude/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// File and stdin input (design 2.3): --description-file, --append-description-file
// and --body-file read the text an inline flag would carry from a file, or
// from standard input for "-". The rules: a leading UTF-8 byte order mark is
// removed, exactly one trailing newline (LF or CRLF) is removed, the rest is
// passed to the converter as it is; the bytes must be valid UTF-8 and at most
// 1 MiB; an empty result follows the inline flag's rule for "".

// runCLIStdin is runCLI with stdin holding the given text.
func runCLIStdin(t *testing.T, deps main.Deps, stdin string, args ...string) runResult {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := main.Run(args, strings.NewReader(stdin), &stdout, &stderr, deps)
	return runResult{stdout: stdout.String(), stderr: stderr.String(), code: code}
}

// writeFile writes data to name in a fresh temporary directory and returns
// its path.
func writeFile(t *testing.T, name string, data []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	require.NoError(t, os.WriteFile(path, data, 0o600))
	return path
}

// recorder captures what the mutating calls of an issue service were sent.
type recorder struct {
	mu      sync.Mutex
	created *jira4claude.Issue
	update  *jira4claude.IssueUpdate
	comment *jira4claude.ADFNode
}

// service serves current on Get and records every write.
func (r *recorder) service(current *jira4claude.Issue) *mock.IssueService {
	return &mock.IssueService{
		GetFn: func(context.Context, string) (*jira4claude.Issue, error) { return current, nil },
		CreateFn: func(_ context.Context, issue *jira4claude.Issue) (*jira4claude.Issue, error) {
			r.mu.Lock()
			defer r.mu.Unlock()
			r.created = issue
			return &jira4claude.Issue{Key: "TEST-7"}, nil
		},
		UpdateFn: func(_ context.Context, key string, update jira4claude.IssueUpdate) (*jira4claude.Issue, error) {
			r.mu.Lock()
			defer r.mu.Unlock()
			r.update = &update
			return &jira4claude.Issue{Key: key}, nil
		},
		AddCommentFn: func(_ context.Context, _ string, body *jira4claude.ADFNode) (*jira4claude.Comment, error) {
			r.mu.Lock()
			defer r.mu.Unlock()
			r.comment = body
			return &jira4claude.Comment{ID: "10001"}, nil
		},
	}
}

// written returns the document the one write recorded carries: the created
// issue's description, the update's description or the comment body.
func (r *recorder) written(t *testing.T) string {
	t.Helper()
	r.mu.Lock()
	defer r.mu.Unlock()
	switch {
	case r.created != nil:
		return marshal(t, r.created.Description)
	case r.update != nil:
		return marshal(t, r.update.Description.Document())
	case r.comment != nil:
		return marshal(t, r.comment)
	}
	t.Fatal("nothing was written")
	return ""
}

// inputCommand is a command that takes text inline or from a file.
type inputCommand struct {
	name     string
	args     []string // the command without its text flag
	inline   string   // the inline flag
	fileFlag string   // its file form
	current  *jira4claude.Issue
}

func inputCommands() []inputCommand {
	return []inputCommand{
		{"issue create", []string{"issue", "create", "-s", "New"}, "--description", "--description-file", nil},
		{"issue update", []string{"issue", "update", "TEST-1"}, "--description", "--description-file", &jira4claude.Issue{Key: "TEST-1"}},
		{"issue update append", []string{"issue", "update", "TEST-1"}, "--append-description", "--append-description-file", &jira4claude.Issue{Key: "TEST-1"}},
		{"issue comment", []string{"issue", "comment", "TEST-1"}, "--body", "--body-file", nil},
	}
}

func (c inputCommand) with(flags ...string) []string {
	return append(append([]string(nil), c.args...), flags...)
}

const inputMarkdown = "# Title\n\nSome *text* with a [link](https://example.com).\n\n```go\nx := 1\n```\n\n- one\n- two"

func TestRun_FileInputSameWriteAsInline(t *testing.T) {
	t.Parallel()

	for _, cmd := range inputCommands() {
		t.Run(cmd.name, func(t *testing.T) {
			t.Parallel()

			inline := &recorder{}
			res := runCLI(t, testDeps(inline.service(cmd.current), &mock.UserService{}), cmd.with(cmd.inline+"="+inputMarkdown)...)
			require.Equal(t, 0, res.code, "stderr: %s", res.stderr)
			want := inline.written(t)

			t.Run("file", func(t *testing.T) {
				t.Parallel()

				path := writeFile(t, "text.md", []byte(inputMarkdown+"\n"))
				rec := &recorder{}
				res := runCLI(t, testDeps(rec.service(cmd.current), &mock.UserService{}), cmd.with(cmd.fileFlag, path)...)

				require.Equal(t, 0, res.code, "stderr: %s", res.stderr)
				assert.Empty(t, res.stderr)
				assert.Equal(t, want, rec.written(t))
			})

			t.Run("stdin with a space", func(t *testing.T) {
				t.Parallel()

				rec := &recorder{}
				res := runCLIStdin(t, testDeps(rec.service(cmd.current), &mock.UserService{}), inputMarkdown+"\n", cmd.with(cmd.fileFlag, "-")...)

				require.Equal(t, 0, res.code, "stderr: %s", res.stderr)
				assert.Equal(t, want, rec.written(t))
			})

			t.Run("stdin with =", func(t *testing.T) {
				t.Parallel()

				rec := &recorder{}
				res := runCLIStdin(t, testDeps(rec.service(cmd.current), &mock.UserService{}), inputMarkdown, cmd.with(cmd.fileFlag+"=-")...)

				require.Equal(t, 0, res.code, "stderr: %s", res.stderr)
				assert.Equal(t, want, rec.written(t))
			})

			t.Run("BOM and CRLF", func(t *testing.T) {
				t.Parallel()

				crlf := "\xef\xbb\xbf" + strings.ReplaceAll(inputMarkdown, "\n", "\r\n") + "\r\n"
				path := writeFile(t, "text.md", []byte(crlf))
				rec := &recorder{}
				res := runCLI(t, testDeps(rec.service(cmd.current), &mock.UserService{}), cmd.with(cmd.fileFlag, path)...)

				require.Equal(t, 0, res.code, "stderr: %s", res.stderr)
				assert.Equal(t, want, rec.written(t))
			})
		})
	}
}

func TestRun_FileInputUsageErrors(t *testing.T) {
	t.Parallel()

	path := writeFile(t, "text.md", []byte("text\n"))
	cases := []struct {
		name string
		args []string
	}{
		{"create: inline and file", []string{"issue", "create", "-s", "New", "--description=x", "--description-file", path}},
		{"update: inline and file", []string{"issue", "update", "TEST-1", "--description=x", "--description-file", path}},
		{"update: file and append file", []string{"issue", "update", "TEST-1", "--description-file", path, "--append-description-file", path}},
		{"update: both files read stdin", []string{"issue", "update", "TEST-1", "--description-file=-", "--append-description-file=-"}},
		{"update: append and append file", []string{"issue", "update", "TEST-1", "--append-description=x", "--append-description-file", path}},
		{"update: file and clear", []string{"issue", "update", "TEST-1", "--clear-description", "--description-file", path}},
		{"comment: inline and file", []string{"issue", "comment", "TEST-1", "--body=x", "--body-file", path}},
		{"comment: neither", []string{"issue", "comment", "TEST-1"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// A zero-value service panics on any call: none may happen.
			res := runCLI(t, testDeps(&mock.IssueService{}, &mock.UserService{}), tc.args...)

			assert.Equal(t, 1, res.code)
			assert.Empty(t, res.stdout)
			assert.Contains(t, res.stderr, "Usage:")
		})
	}
}

func TestRun_FileInputValidationErrors(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	missing := filepath.Join(dir, "missing.md")
	unreadable := writeFile(t, "secret.md", []byte("text"))
	require.NoError(t, os.Chmod(unreadable, 0o000))
	readable := func() bool {
		f, err := os.Open(unreadable)
		if err == nil {
			_ = f.Close()
		}
		return err == nil
	}()
	invalid := writeFile(t, "latin1.md", []byte("caf\xe9 au lait\n"))
	large := writeFile(t, "large.md", bytes.Repeat([]byte("a"), 1<<20+1))

	cases := []struct {
		name    string
		path    string
		message string
	}{
		{"missing file", missing, "cannot read --%s " + missing + ": no such file or directory"},
		{"directory", dir, "--%s " + dir + " is a directory"},
		{"empty path", "", "--%s needs a file path, or - for standard input"},
		{"invalid UTF-8", invalid, "--%s " + invalid + " is not valid UTF-8 (first invalid byte at offset 3)"},
		{"larger than 1 MiB", large, "--%s " + large + " is larger than 1 MiB"},
	}
	if !readable {
		cases = append(cases, struct {
			name    string
			path    string
			message string
		}{"unreadable file", unreadable, "cannot read --%s " + unreadable + ": permission denied"})
	}

	for _, cmd := range inputCommands() {
		for _, tc := range cases {
			flag := strings.TrimPrefix(cmd.fileFlag, "--")
			want := strings.ReplaceAll(tc.message, "%s", flag)

			t.Run(cmd.name+"/"+tc.name+"/text", func(t *testing.T) {
				t.Parallel()

				// No service call may happen: the input is read first.
				res := runCLI(t, testDeps(&mock.IssueService{}, &mock.UserService{}), cmd.with(cmd.fileFlag+"="+tc.path)...)

				assert.Equal(t, 1, res.code)
				assert.Empty(t, res.stdout)
				assert.Equal(t, "[error] "+want+"\n", res.stderr)
			})

			t.Run(cmd.name+"/"+tc.name+"/json", func(t *testing.T) {
				t.Parallel()

				res := runCLI(t, testDeps(&mock.IssueService{}, &mock.UserService{}), append([]string{"--json"}, cmd.with(cmd.fileFlag+"="+tc.path)...)...)

				assert.Equal(t, 1, res.code)
				assert.Empty(t, res.stdout)
				assertJSONError(t, res.stderr, jira4claude.EValidation, want)
			})
		}
	}

	t.Run("standard input over 1 MiB", func(t *testing.T) {
		t.Parallel()

		res := runCLIStdin(t, testDeps(&mock.IssueService{}, &mock.UserService{}), strings.Repeat("a", 1<<20+1),
			"issue", "comment", "TEST-1", "--body-file", "-")

		assert.Equal(t, 1, res.code)
		assert.Equal(t, "[error] --body-file - (standard input) is larger than 1 MiB\n", res.stderr)
	})

	t.Run("invalid UTF-8 on standard input", func(t *testing.T) {
		t.Parallel()

		res := runCLIStdin(t, testDeps(&mock.IssueService{}, &mock.UserService{}), "ok\n\xff",
			"issue", "comment", "TEST-1", "--body-file", "-")

		assert.Equal(t, 1, res.code)
		assert.Equal(t, "[error] --body-file - (standard input) is not valid UTF-8 (first invalid byte at offset 3)\n", res.stderr)
	})

	t.Run("exactly 1 MiB is accepted", func(t *testing.T) {
		t.Parallel()

		rec := &recorder{}
		res := runCLIStdin(t, testDeps(rec.service(nil), &mock.UserService{}), strings.Repeat("a", 1<<20),
			"issue", "comment", "TEST-1", "--body-file", "-")

		assert.Equal(t, 0, res.code, "stderr: %s", res.stderr)
	})
}

func TestRun_FileInputEmpty(t *testing.T) {
	t.Parallel()

	// Each file holds nothing but blanks once the BOM and one trailing
	// newline are gone: the converter makes an empty document of them.
	empties := map[string][]byte{
		"empty":        {},
		"newline":      []byte("\n"),
		"CRLF":         []byte("\r\n"),
		"BOM":          []byte("\xef\xbb\xbf"),
		"BOM, newline": []byte("\xef\xbb\xbf\n"),
		"blank lines":  []byte("\n\n\n"),
		"blanks":       []byte(" \t\n \r\n"),
	}

	for name, data := range empties {
		t.Run("update --description-file: "+name, func(t *testing.T) {
			t.Parallel()

			path := writeFile(t, "empty.md", data)
			res := runCLI(t, testDeps(&mock.IssueService{}, &mock.UserService{}), "issue", "update", "TEST-1", "--description-file", path)

			assert.Equal(t, 1, res.code)
			assert.Equal(t, "[error] --description-file "+path+" is empty; use --clear-description to remove the description\n", res.stderr)
		})

		t.Run("update --append-description-file: "+name, func(t *testing.T) {
			t.Parallel()

			path := writeFile(t, "empty.md", data)
			res := runCLI(t, testDeps(&mock.IssueService{}, &mock.UserService{}), "issue", "update", "TEST-1", "--append-description-file", path)

			assert.Equal(t, 1, res.code)
			assert.Equal(t, "[error] --append-description-file "+path+" is empty\n", res.stderr)
		})

		t.Run("comment --body-file: "+name, func(t *testing.T) {
			t.Parallel()

			path := writeFile(t, "empty.md", data)
			res := runCLI(t, testDeps(&mock.IssueService{}, &mock.UserService{}), "issue", "comment", "TEST-1", "--body-file", path)

			assert.Equal(t, 1, res.code)
			assert.Equal(t, "[error] --body-file "+path+" is empty\n", res.stderr)
		})

		// As --description "" does, an empty file creates the issue without
		// a description.
		t.Run("create --description-file: "+name, func(t *testing.T) {
			t.Parallel()

			path := writeFile(t, "empty.md", data)
			rec := &recorder{}
			res := runCLI(t, testDeps(rec.service(nil), &mock.UserService{}), "issue", "create", "-s", "New", "--description-file", path)

			require.Equal(t, 0, res.code, "stderr: %s", res.stderr)
			assert.Nil(t, rec.created.Description)
		})
	}

	t.Run("empty standard input", func(t *testing.T) {
		t.Parallel()

		res := runCLIStdin(t, testDeps(&mock.IssueService{}, &mock.UserService{}), "", "issue", "update", "TEST-1", "--description-file", "-")

		assert.Equal(t, 1, res.code)
		assert.Equal(t, "[error] --description-file - (standard input) is empty; use --clear-description to remove the description\n", res.stderr)
	})

	t.Run("the inline forms follow the same rule", func(t *testing.T) {
		t.Parallel()

		for _, value := range []string{"", " ", "\n\t\n"} {
			res := runCLI(t, testDeps(&mock.IssueService{}, &mock.UserService{}), "issue", "comment", "TEST-1", "--body="+value)
			assert.Equal(t, 1, res.code)
			assert.Equal(t, "[error] --body cannot be empty\n", res.stderr)

			res = runCLI(t, testDeps(&mock.IssueService{}, &mock.UserService{}), "issue", "update", "TEST-1", "--description="+value)
			assert.Equal(t, 1, res.code)
			assert.Equal(t, "[error] --description cannot be empty; use --clear-description to remove the description\n", res.stderr)

			res = runCLI(t, testDeps(&mock.IssueService{}, &mock.UserService{}), "issue", "update", "TEST-1", "--append-description="+value)
			assert.Equal(t, 1, res.code)
			assert.Equal(t, "[error] --append-description cannot be empty\n", res.stderr)

			rec := &recorder{}
			res = runCLI(t, testDeps(rec.service(nil), &mock.UserService{}), "issue", "create", "-s", "New", "--description="+value)
			require.Equal(t, 0, res.code, "stderr: %s", res.stderr)
			assert.Nil(t, rec.created.Description, "%q", value)
		}
	})
}

func TestRun_FileInputTrailingNewlines(t *testing.T) {
	t.Parallel()

	// Exactly one trailing newline is removed; any others are part of the
	// text. In an unclosed code fence they are content.
	cases := []struct {
		name string
		file string
		same string // the inline value giving the same write
	}{
		{"one LF", "text\n", "text"},
		{"one CRLF", "text\r\n", "text"},
		{"none", "text", "text"},
		{"unclosed fence, one newline", "```\ncode\n", "```\ncode"},
		{"unclosed fence, two newlines", "```\ncode\n\n", "```\ncode\n"},
		{"trailing blank line kept", "```\ncode\n\n\n", "```\ncode\n\n"},
		{"trailing spaces kept", "a  \nb  \n", "a  \nb  "},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			inline := &recorder{}
			res := runCLI(t, testDeps(inline.service(nil), &mock.UserService{}), "issue", "comment", "TEST-1", "--body="+tc.same)
			require.Equal(t, 0, res.code, "stderr: %s", res.stderr)

			path := writeFile(t, "text.md", []byte(tc.file))
			rec := &recorder{}
			res = runCLI(t, testDeps(rec.service(nil), &mock.UserService{}), "issue", "comment", "TEST-1", "--body-file", path)

			require.Equal(t, 0, res.code, "stderr: %s", res.stderr)
			assert.Equal(t, inline.written(t), rec.written(t))
		})
	}

	t.Run("the fence cases differ", func(t *testing.T) {
		t.Parallel()

		one, two := &recorder{}, &recorder{}
		require.Equal(t, 0, runCLI(t, testDeps(one.service(nil), &mock.UserService{}), "issue", "comment", "TEST-1",
			"--body-file", writeFile(t, "a.md", []byte("```\ncode\n"))).code)
		require.Equal(t, 0, runCLI(t, testDeps(two.service(nil), &mock.UserService{}), "issue", "comment", "TEST-1",
			"--body-file", writeFile(t, "b.md", []byte("```\ncode\n\n\n"))).code)
		assert.NotEqual(t, one.written(t), two.written(t), "only one trailing newline is removed; the blank line left is code content")
	})
}

// roundTripIssues are descriptions whose last line is a block token, an
// inline token, a code block, and text with edge blanks.
func roundTripIssues() map[string]*jira4claude.Issue {
	withDescription := func(content ...jira4claude.ADFNode) *jira4claude.Issue {
		issue := guardedIssue()
		issue.Description = &jira4claude.ADFNode{Type: "doc", Version: 1, Content: content}
		return issue
	}
	para := func(nodes ...jira4claude.ADFNode) jira4claude.ADFNode {
		return jira4claude.ADFNode{Type: "paragraph", Content: nodes}
	}
	text := func(s string) jira4claude.ADFNode { return jira4claude.ADFNode{Type: "text", Text: s} }
	return map[string]*jira4claude.Issue{
		"block token last": tokenTailIssue(),
		"inline token last": withDescription(
			guardedMediaSingle(),
			para(text("Ping "), guardedMention()),
		),
		"code block last": withDescription(
			para(text("Run:")),
			jira4claude.ADFNode{Type: "codeBlock", Attrs: json.RawMessage(`{"language":"sh"}`), Content: []jira4claude.ADFNode{text("make validate\n")}},
		),
		"edge blanks": withDescription(
			para(guardedMention(), text(" ends with blanks  ")),
		),
		"guarded": guardedIssue(),
	}
}

func TestRun_FileInputRoundTrip(t *testing.T) {
	t.Parallel()

	for name, issue := range roundTripIssues() {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			view := runCLI(t, testDeps(newGuardedService(issue), &mock.UserService{}),
				"issue", "view", "TEST-1", "--field", "description")
			require.Equal(t, 0, view.code, "stderr: %s", view.stderr)
			require.Empty(t, view.stderr)
			original := marshal(t, issue.Description)

			// The control: the value without view's newline, inline.
			inline := newGuardedService(issue)
			res := runCLI(t, testDeps(inline, &mock.UserService{}),
				"issue", "update", "TEST-1", "--description="+strings.TrimSuffix(view.stdout, "\n"))
			require.Equal(t, 0, res.code, "stderr: %s", res.stderr)

			t.Run("file", func(t *testing.T) {
				t.Parallel()

				path := writeFile(t, "d.md", []byte(view.stdout))
				svc := newGuardedService(issue)
				res := runCLI(t, testDeps(svc, &mock.UserService{}),
					"issue", "update", "TEST-1", "--description-file", path, "--if-updated", "2026-11-09T10:15:30.123Z")

				require.Equal(t, 0, res.code, "stderr: %s", res.stderr)
				assert.Empty(t, res.stderr)
				assert.Equal(t, original, marshal(t, svc.Sent().Description.Document()))
				assert.Equal(t, marshal(t, inline.Sent().Description.Document()), marshal(t, svc.Sent().Description.Document()))
			})

			t.Run("stdin", func(t *testing.T) {
				t.Parallel()

				svc := newGuardedService(issue)
				res := runCLIStdin(t, testDeps(svc, &mock.UserService{}), view.stdout,
					"issue", "update", "TEST-1", "--description-file", "-")

				require.Equal(t, 0, res.code, "stderr: %s", res.stderr)
				assert.Empty(t, res.stderr)
				assert.Equal(t, original, marshal(t, svc.Sent().Description.Document()))
			})
		})
	}
}

// The stream rules of streams_test.go hold for the file forms: a service
// error is the only output, on stderr; a conversion warning goes to stderr.
func TestRun_FileInputStreams(t *testing.T) {
	t.Parallel()

	failing := func(err error) *mock.IssueService {
		return &mock.IssueService{
			GetFn: readsKey,
			CreateFn: func(context.Context, *jira4claude.Issue) (*jira4claude.Issue, error) {
				return nil, err
			},
			UpdateFn: func(context.Context, string, jira4claude.IssueUpdate) (*jira4claude.Issue, error) {
				return nil, err
			},
			AddCommentFn: func(context.Context, string, *jira4claude.ADFNode) (*jira4claude.Comment, error) {
				return nil, err
			},
		}
	}

	for _, cmd := range inputCommands() {
		for _, ec := range errorCodes() {
			message := "failed with " + ec.code

			t.Run(cmd.name+"/"+ec.code+"/text", func(t *testing.T) {
				t.Parallel()

				res := runCLIStdin(t, testDeps(failing(ec.err), &mock.UserService{}), "Text\n", cmd.with(cmd.fileFlag, "-")...)

				assert.Equal(t, ec.exit, res.code)
				assert.Empty(t, res.stdout)
				assert.Equal(t, "[error] "+message+"\n", res.stderr)
			})

			t.Run(cmd.name+"/"+ec.code+"/json", func(t *testing.T) {
				t.Parallel()

				res := runCLIStdin(t, testDeps(failing(ec.err), &mock.UserService{}), "Text\n",
					append([]string{"--json"}, cmd.with(cmd.fileFlag, "-")...)...)

				assert.Equal(t, ec.exit, res.code)
				assert.Empty(t, res.stdout)
				assertJSONError(t, res.stderr, ec.code, message)
			})
		}

		t.Run(cmd.name+"/warning/text", func(t *testing.T) {
			t.Parallel()

			rec := &recorder{}
			res := runCLIStdin(t, testDeps(rec.service(cmd.current), &mock.UserService{}), degradedMarkdown+"\n", cmd.with(cmd.fileFlag, "-")...)

			assert.Equal(t, 0, res.code, "stderr: %q", res.stderr)
			assert.NotContains(t, res.stdout, "[warn]")
			assert.True(t, strings.HasPrefix(res.stderr, "[warn] degraded heading"), "stderr: %q", res.stderr)
			assert.Equal(t, 1, strings.Count(res.stderr, "\n"), "stderr: %q", res.stderr)
		})

		t.Run(cmd.name+"/warning/json", func(t *testing.T) {
			t.Parallel()

			rec := &recorder{}
			res := runCLIStdin(t, testDeps(rec.service(cmd.current), &mock.UserService{}), degradedMarkdown+"\n",
				append([]string{"--json"}, cmd.with(cmd.fileFlag, "-")...)...)

			assert.Equal(t, 0, res.code, "stderr: %q", res.stderr)
			assertOneJSONValue(t, res.stdout)
			var warning map[string]any
			require.NoError(t, json.Unmarshal([]byte(res.stderr), &warning), "stderr: %q", res.stderr)
			assert.Equal(t, "warning", warning["level"])
			assert.Equal(t, "heading", warning["node"])
		})
	}
}
