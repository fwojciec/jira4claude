package main_test

// End-to-end tests: the j4c binary, built once per test run, run as a child
// process against a stub Jira (e2e_stub_test.go).
//
// Every other CLI test drives main.Run with mock services, and every HTTP
// test exercises one service alone. These run the real wiring together: main
// and defaultDeps, the netrc lookup in a scratch HOME, loadConfig and its
// walk-up through parent directories and the home directory, stdin as main
// wires it, the real printers and the real HTTP services. Each scenario runs
// the README's scripting steps and checks stdout, stderr, the exit code and
// the requests the stub received.
//
// The child gets an explicit environment: HOME, pointing at a scratch
// directory holding .netrc, and nothing else (no PATH, no proxy variables,
// not the developer's HOME). Its working directory is set through cmd.Dir
// and its stdin through cmd.Stdin; the test process never changes its own
// directory or environment, so the scenarios run in parallel. Each has its
// own stub, HOME and working directory.
//
// What this cannot catch: the stub answers in the request and response
// shapes the http package's unit tests use, which come from Atlassian's
// OpenAPI document and examples, the same assumptions the client is built
// on. A wrong assumption about Jira (a field name, a status code, whether
// redirect=false returns the bytes) is shared by the client and the stub and
// passes here. Only a run against a live site can find those; the design
// document's smoke-test table lists the open ones. The media host is a
// second server on 127.0.0.1 with another port, so the redirect it receives
// crosses origins by port, not by host name.

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Nodes of PROJ-1's description, in the key order and escaping Jira uses
// (attrs before marks, '<' and '&' unescaped), which is not the order and
// escaping Go's encoder produces. The media node is the one of
// markdown/reattach_test.go, the others those of markdown/rich_nodes_test.go.
const (
	e2eMedia = `{"type":"mediaSingle","attrs":{"layout":"wrap-left","width":66.67,"widthType":"percentage"},"content":[` +
		`{"type":"media","attrs":{"id":"0c0f","type":"file","collection":"contentId-1","width":1.50,"height":2e3,` +
		`"alt":"a <b> & c","occurrenceKey":"k","__fileName":"shot.png","__fileSize":1024,"__fileMimeType":"image/png",` +
		`"__contextId":null,"__external":false,"localId":"l1"},` +
		`"marks":[{"type":"border","attrs":{"size":2,"color":"#091e4224"}}]},` +
		`{"type":"caption","content":[{"type":"text","text":"cap"}]}]}`
	e2eMention = `{"type":"mention","attrs":{"id":"5b10ac8d82e05b22cc7d4ef5","text":"@Jane Doe","accessLevel":"CONTAINER","userType":"DEFAULT","localId":"m1"}}`
	e2eStatus  = `{"type":"status","attrs":{"text":"In Review","color":"blue","localId":"s1","style":""}}`
	e2eDate    = `{"type":"date","attrs":{"timestamp":"1763596800000"}}`

	e2eDescription = `{"type":"doc","version":1,"content":[` +
		`{"type":"paragraph","content":[{"type":"text","text":"Sessions expire after 5 minutes. Screenshot:"}]},` +
		e2eMedia + `,` +
		`{"type":"paragraph","content":[{"type":"text","text":"Ping "},` + e2eMention + `,{"type":"text","text":" before merging."}]},` +
		`{"type":"paragraph","content":[{"type":"text","text":"Now "},` + e2eStatus + `,{"type":"text","text":", due "},` + e2eDate + `]}]}`

	e2eUpdated = "2026-11-09T10:15:30.123+0000"
)

// e2eRun is what one run of the binary produced.
type e2eRun struct {
	stdout string
	stderr string
	code   int
}

// e2eEnv is one scenario's world: a stub Jira, a scratch HOME holding a
// .netrc for it, and a working directory holding .jira4claude.yaml.
type e2eEnv struct {
	t    *testing.T
	bin  string
	stub *stubJira
	home string
	work string
}

// buildJ4C builds the binary from this package's source into dir.
func buildJ4C(t *testing.T, dir string) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok, "cannot locate the test source file")
	bin := filepath.Join(dir, "j4c")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	cmd := exec.Command("go", "build", "-o", bin, ".")
	cmd.Dir = filepath.Dir(file)
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "go build: %s", out)
	return bin
}

// newE2EEnv makes a scenario's world with PROJ-1 (the README's issue, with
// a description holding a media node, a mention, a status and a date, and
// two attachments), PROJ-2 and PROJ-3.
func newE2EEnv(t *testing.T, bin string) *e2eEnv {
	t.Helper()
	root := t.TempDir()
	e := &e2eEnv{
		t:    t,
		bin:  bin,
		stub: newStubJira(t),
		home: filepath.Join(root, "home"),
		work: filepath.Join(root, "work"),
	}
	require.NoError(t, os.Mkdir(e.home, 0o700))
	require.NoError(t, os.Mkdir(e.work, 0o700))
	e.writeNetrc(e.stub.host())
	e.writeConfig(e.work)

	e.stub.addIssue(&stubIssue{
		Key: "PROJ-1", Summary: "Fix login timeout", Status: "In Progress", Type: "Bug", Priority: "High",
		Assignee: stubAlice(), Description: json.RawMessage(e2eDescription), Updated: e2eUpdated,
	})
	e.stub.addIssue(&stubIssue{
		Key: "PROJ-2", Summary: "Release 2.4", Status: "To Do",
		Description: json.RawMessage(`{"type":"doc","version":1,"content":[{"type":"paragraph","content":[{"type":"text","text":"Plain text."}]}]}`),
		Updated:     "2026-11-07T16:40:02.005+0000",
	})
	e.stub.addIssue(&stubIssue{
		Key: "PROJ-3", Summary: "Export\treport as CSV", Status: "To Do", Type: "Story", Priority: "Medium",
		Updated: "2026-11-07T16:40:02.005+0000",
	})
	e.stub.addAttachment(&stubAttachment{ID: "10000", Issue: "PROJ-1", Filename: "shot.png", MimeType: "image/png", Content: []byte("\x89PNG fake image bytes")})
	e.stub.addAttachment(&stubAttachment{ID: "10001", Issue: "PROJ-1", Filename: "debug log.txt", MimeType: "text/plain", Content: []byte("line 1\nline 2\n")})
	return e
}

func (s *stubJira) host() string {
	return strings.TrimPrefix(s.server.URL, "http://")
}

func (e *e2eEnv) writeNetrc(machine string) {
	e.t.Helper()
	netrc := "machine " + machine + " login " + e.stub.login + " password " + e.stub.password + "\n"
	require.NoError(e.t, os.WriteFile(filepath.Join(e.home, ".netrc"), []byte(netrc), 0o600))
}

func (e *e2eEnv) writeConfig(dir string) {
	e.t.Helper()
	config := "server: " + e.stub.server.URL + "\nproject: PROJ\n"
	require.NoError(e.t, os.WriteFile(filepath.Join(dir, ".jira4claude.yaml"), []byte(config), 0o600))
}

// run runs the binary in the working directory.
func (e *e2eEnv) run(args ...string) e2eRun {
	e.t.Helper()
	return e.runIn(e.work, nil, args...)
}

// runIn runs the binary in dir with stdin (nil: no input), HOME as its
// only environment variable, and a deadline after which it is killed.
func (e *e2eEnv) runIn(dir string, stdin io.Reader, args ...string) e2eRun {
	e.t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, e.bin, args...)
	cmd.Dir = dir
	cmd.Env = []string{"HOME=" + e.home}
	cmd.Stdin = stdin
	cmd.WaitDelay = 5 * time.Second
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	require.NoError(e.t, ctx.Err(), "j4c %v did not finish; stderr: %s", args, stderr.String())
	code := 0
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		code = exitErr.ExitCode()
	} else {
		require.NoError(e.t, err)
	}
	return e2eRun{stdout: stdout.String(), stderr: stderr.String(), code: code}
}

// paths returns "METHOD path?query" for each request.
func paths(reqs []stubRequest) []string {
	out := make([]string, 0, len(reqs))
	for _, r := range reqs {
		line := r.Method + " " + r.Path
		if r.RawQuery != "" {
			line += "?" + r.RawQuery
		}
		out = append(out, line)
	}
	return out
}

// writes returns the requests that may change something: any method but
// GET and HEAD.
func writes(reqs []stubRequest) []stubRequest {
	var out []stubRequest
	for _, r := range reqs {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			out = append(out, r)
		}
	}
	return out
}

const getProj1 = "GET /rest/api/3/issue/PROJ-1?expand=names,schema"

func TestE2E(t *testing.T) {
	t.Parallel()

	bin := buildJ4C(t, t.TempDir())

	t.Run("init writes the config; later commands find it by walk-up and in HOME", func(t *testing.T) {
		t.Parallel()
		e := newE2EEnv(t, bin)
		project := filepath.Join(filepath.Dir(e.work), "project")
		require.NoError(t, os.Mkdir(project, 0o700))

		r := e.runIn(project, nil, "init", "--server="+e.stub.server.URL, "--project=PROJ")
		assert.Equal(t, 0, r.code, r.stderr)
		assert.Empty(t, r.stderr)
		assert.NotEmpty(t, r.stdout)
		config, err := os.ReadFile(filepath.Join(project, ".jira4claude.yaml"))
		require.NoError(t, err)
		assert.Contains(t, string(config), e.stub.server.URL)
		gitignore, err := os.ReadFile(filepath.Join(project, ".gitignore"))
		require.NoError(t, err)
		assert.Equal(t, ".jira4claude.yaml\n", string(gitignore))
		assert.Empty(t, e.stub.Requests(), "init asks Jira nothing")

		sub := filepath.Join(project, "a", "b")
		require.NoError(t, os.MkdirAll(sub, 0o700))
		r = e.runIn(sub, nil, "issue", "view", "PROJ-1", "--field", "summary")
		assert.Equal(t, e2eRun{stdout: "Fix login timeout\n"}, r)
		assert.Equal(t, []string{getProj1}, paths(e.stub.Requests()))

		// A directory with no config above it falls back to HOME.
		elsewhere := filepath.Join(filepath.Dir(e.work), "elsewhere")
		require.NoError(t, os.Mkdir(elsewhere, 0o700))
		r = e.runIn(elsewhere, nil, "issue", "view", "PROJ-2", "--field", "summary")
		assert.Equal(t, 4, r.code, "no config yet: %s", r.stderr)
		assert.Contains(t, r.stderr, "no config file found")
		e.writeConfig(e.home)
		r = e.runIn(elsewhere, nil, "issue", "view", "PROJ-2", "--field", "summary")
		assert.Equal(t, e2eRun{stdout: "Release 2.4\n"}, r)
	})

	t.Run("view --field updated and --field description", func(t *testing.T) {
		t.Parallel()
		e := newE2EEnv(t, bin)

		r := e.run("issue", "view", "PROJ-1", "--field", "updated")
		assert.Equal(t, e2eRun{stdout: "2026-11-09T10:15:30.123Z\n"}, r)

		r = e.run("issue", "view", "PROJ-1", "--field", "description")
		assert.Equal(t, 0, r.code)
		assert.Empty(t, r.stderr)
		assert.Equal(t, "Sessions expire after 5 minutes. Screenshot:\n\n"+
			"{{mediaSingle:1 a \\<b> & c}}\n\n"+
			"Ping {{mention:2 Jane Doe}} before merging.\n\n"+
			"Now {{status:3 In Review}}, due {{date:4 2025-11-20}}\n", r.stdout)

		for _, req := range e.stub.Requests() {
			assert.Equal(t, e.stub.authorization(), req.Header.Get("Authorization"))
			assert.Equal(t, "application/json", req.Header.Get("Accept"))
		}
	})

	t.Run("guarded edit re-attaches the media, mention, status and date nodes", func(t *testing.T) {
		t.Parallel()
		e := newE2EEnv(t, bin)

		updated := e.run("issue", "view", "PROJ-1", "--field", "updated")
		require.Equal(t, 0, updated.code, updated.stderr)
		desc := e.run("issue", "view", "PROJ-1", "--field", "description")
		require.Equal(t, 0, desc.code, desc.stderr)
		edited := strings.Replace(desc.stdout, "5 minutes", "30 minutes", 1)
		require.NotEqual(t, desc.stdout, edited)
		file := filepath.Join(e.work, "d.md")
		require.NoError(t, os.WriteFile(file, []byte(edited), 0o600))

		r := e.run("issue", "update", "PROJ-1", "--description-file", "d.md", "--if-updated", strings.TrimSpace(updated.stdout))
		assert.Equal(t, e2eRun{stdout: "PROJ-1: updated\n" +
			"  description: set\n" +
			"  updated: 2026-11-10T09:00:00.456Z\n" +
			e.stub.server.URL + "/browse/PROJ-1\n"}, r)

		assert.Equal(t, []string{getProj1, getProj1, getProj1, "PUT /rest/api/3/issue/PROJ-1", getProj1}, paths(e.stub.Requests()))
		puts := writes(e.stub.Requests())
		require.Len(t, puts, 1)
		assert.Equal(t, "application/json", puts[0].Header.Get("Content-Type"))
		var body struct {
			Fields struct {
				Description json.RawMessage `json:"description"`
			} `json:"fields"`
		}
		require.NoError(t, json.Unmarshal(puts[0].Body, &body))
		want := strings.Replace(e2eDescription, "5 minutes", "30 minutes", 1)
		assert.JSONEq(t, want, string(body.Fields.Description), "the PUT holds the original nodes in place of their tokens")
		// The mention, status and date go back byte for byte. The media node
		// goes back equal as JSON but not as bytes: ADFNode re-encodes it, so
		// marks come before attrs and '<', '>' and '&' in the raw attrs are
		// written as \u003c, \u003e and \u0026. Jira parses JSON, so both
		// differences are invisible to it.
		for _, node := range []string{e2eMention, e2eStatus, e2eDate} {
			assert.Contains(t, string(puts[0].Body), node, "node sent back byte for byte")
		}
		var doc struct {
			Content []json.RawMessage `json:"content"`
		}
		require.NoError(t, json.Unmarshal(body.Fields.Description, &doc))
		require.Len(t, doc.Content, 4)
		assert.JSONEq(t, e2eMedia, string(doc.Content[1]))
		assert.NotContains(t, string(puts[0].Body), e2eMedia, "if the media node now goes back byte for byte, tighten this test")

		// The description now reads back as it was written.
		after := e.run("issue", "view", "PROJ-1", "--field", "description")
		assert.Equal(t, e2eRun{stdout: edited}, after)
	})

	t.Run("a stale --if-updated exits 5 and writes nothing", func(t *testing.T) {
		t.Parallel()
		e := newE2EEnv(t, bin)
		require.NoError(t, os.WriteFile(filepath.Join(e.work, "d.md"), []byte("New text."), 0o600))

		r := e.run("issue", "update", "PROJ-1", "--description-file", "d.md", "--force", "--if-updated", "2026-11-09T10:15:30.000Z")
		assert.Equal(t, 5, r.code)
		assert.Empty(t, r.stdout)
		assert.Equal(t, "[error] PROJ-1 was updated at 2026-11-09T10:15:30.123Z, not at 2026-11-09T10:15:30.000Z as --if-updated expects. "+
			"Nothing was written. View the issue again and redo the change against its current state\n", r.stderr)
		assert.Equal(t, []string{getProj1}, paths(e.stub.Requests()))
	})

	t.Run("--description-file - reads the description from a pipe", func(t *testing.T) {
		t.Parallel()
		e := newE2EEnv(t, bin)

		r := e.runIn(e.work, strings.NewReader("\ufeffPiped **text**.\n"), "issue", "update", "PROJ-2", "--description-file", "-")
		assert.Equal(t, 0, r.code, r.stderr)
		assert.Empty(t, r.stderr)
		assert.True(t, strings.HasPrefix(r.stdout, "PROJ-2: updated\n  description: set\n"), r.stdout)
		puts := writes(e.stub.Requests())
		require.Len(t, puts, 1)
		assert.JSONEq(t, `{"fields":{"description":{"type":"doc","version":1,"content":[{"type":"paragraph","content":[`+
			`{"type":"text","text":"Piped "},{"type":"text","text":"text","marks":[{"type":"strong"}]},{"type":"text","text":"."}]}]}}}`,
			string(puts[0].Body))
	})

	t.Run("--dry-run sends no write", func(t *testing.T) {
		t.Parallel()
		e := newE2EEnv(t, bin)
		require.NoError(t, os.WriteFile(filepath.Join(e.work, "d.md"), []byte("Only text now."), 0o600))

		r := e.run("issue", "update", "PROJ-2", "--description-file", "d.md", "--dry-run")
		assert.Equal(t, e2eRun{stdout: "PROJ-2: would update (dry run; nothing was written)\n" +
			"  description: set\n" + e.stub.server.URL + "/browse/PROJ-2\n\nOnly text now.\n"}, r)
		r = e.run("issue", "create", "--summary", "Add retry", "--description", "Retry **twice**.", "--dry-run")
		assert.Equal(t, 0, r.code, r.stderr)
		r = e.run("issue", "comment", "PROJ-1", "--body", "Deployed.", "--dry-run")
		assert.Equal(t, 0, r.code, r.stderr)

		assert.Empty(t, writes(e.stub.Requests()))
		assert.Equal(t, []string{"GET /rest/api/3/issue/PROJ-2?expand=names,schema"}, paths(e.stub.Requests()))
	})

	t.Run("--clear-description is refused over protected nodes, then forced", func(t *testing.T) {
		t.Parallel()
		e := newE2EEnv(t, bin)

		r := e.run("issue", "update", "PROJ-1", "--clear-description")
		assert.Equal(t, 1, r.code)
		assert.Empty(t, r.stdout)
		assert.Contains(t, r.stderr, "[error] ")
		assert.Contains(t, r.stderr, "--force")
		assert.Empty(t, writes(e.stub.Requests()))

		r = e.run("issue", "update", "PROJ-1", "--clear-description", "--force")
		assert.Equal(t, 0, r.code, r.stderr)
		assert.Contains(t, r.stdout, "  description: cleared\n")
		puts := writes(e.stub.Requests())
		require.Len(t, puts, 1)
		assert.JSONEq(t, `{"fields":{"description":null}}`, string(puts[0].Body))
		assert.Nil(t, e.stub.issue("PROJ-1").Description)
	})

	t.Run("an update whose read-back fails still reports the applied write", func(t *testing.T) {
		t.Parallel()
		e := newE2EEnv(t, bin)
		e.stub.mu.Lock()
		e.stub.failReadAfterWrite = true
		e.stub.mu.Unlock()

		r := e.run("issue", "update", "PROJ-2", "--append-description", "Fixed in build 512.")
		assert.Equal(t, e2eRun{
			stdout: "PROJ-2: updated\n  description: appended\n" + e.stub.server.URL + "/browse/PROJ-2\n",
			stderr: "[error] updated PROJ-2, but reading it back failed; the write was applied, do not repeat it: Service unavailable\n",
			code:   7,
		}, r)
		assert.Len(t, writes(e.stub.Requests()), 1, "one PUT, not retried")
		assert.Contains(t, string(e.stub.issue("PROJ-2").Description), "Fixed in build 512.")
	})

	t.Run("issue list warns when cut by --limit; issue ready follows the pages", func(t *testing.T) {
		t.Parallel()
		e := newE2EEnv(t, bin)

		r := e.run("issue", "list", "--limit", "2")
		assert.Equal(t, e2eRun{
			stdout: "PROJ-1\tIn Progress\tBug\tHigh\tAlice Smith\tFix login timeout\n" +
				"PROJ-2\tTo Do\tTask\t-\t-\tRelease 2.4\n",
			stderr: "[warn] showing the first 2 issues; more match (raise --limit to see them)\n",
		}, r)
		reqs := e.stub.Requests()
		require.Len(t, reqs, 1)
		assert.Equal(t, "/rest/api/3/search/jql", reqs[0].Path)
		query := mustParseQuery(t, reqs[0].RawQuery)
		assert.Equal(t, `project = "PROJ"`, query.Get("jql"))
		assert.Equal(t, "2", query.Get("maxResults"))

		e.stub.mu.Lock()
		e.stub.pageSize = 2
		e.stub.mu.Unlock()
		r = e.run("issue", "ready")
		assert.Equal(t, e2eRun{
			stdout: "PROJ-1\tIn Progress\tBug\tHigh\tAlice Smith\tFix login timeout\n" +
				"PROJ-2\tTo Do\tTask\t-\t-\tRelease 2.4\n" +
				"PROJ-3\tTo Do\tStory\tMedium\t-\tExport\\treport as CSV\n",
		}, r)
		reqs = e.stub.Requests()[1:]
		require.Len(t, reqs, 2, "two pages")
		assert.Empty(t, mustParseQuery(t, reqs[0].RawQuery).Get("nextPageToken"))
		assert.Equal(t, "page-2", mustParseQuery(t, reqs[1].RawQuery).Get("nextPageToken"))

		r = e.run("--json", "issue", "list", "--limit", "2")
		assert.Equal(t, 0, r.code)
		var list struct {
			Issues    []map[string]any `json:"issues"`
			Truncated bool             `json:"truncated"`
		}
		require.NoError(t, json.Unmarshal([]byte(r.stdout), &list))
		assert.Len(t, list.Issues, 2)
		assert.True(t, list.Truncated)
		assert.JSONEq(t, `{"level":"warning","message":"showing the first 2 issues; more match (raise --limit to see them)"}`, r.stderr)
	})

	t.Run("issue transition --status", func(t *testing.T) {
		t.Parallel()
		e := newE2EEnv(t, bin)

		r := e.run("issue", "transition", "PROJ-1", "--status", "in review")
		assert.Equal(t, e2eRun{stdout: "PROJ-1: In Progress → In Review\n" + e.stub.server.URL + "/browse/PROJ-1\n"}, r)
		assert.Equal(t, []string{
			"GET /rest/api/3/issue/PROJ-1/transitions", getProj1, "POST /rest/api/3/issue/PROJ-1/transitions",
		}, paths(e.stub.Requests()))
		assert.JSONEq(t, `{"transition":{"id":"41"}}`, string(writes(e.stub.Requests())[0].Body))
		assert.Equal(t, "In Review", e.stub.issue("PROJ-1").Status)
	})

	t.Run("issue create with an assignee by email", func(t *testing.T) {
		t.Parallel()
		e := newE2EEnv(t, bin)

		r := e.run("issue", "create", "--summary", "Add retry", "--type", "Bug", "--assignee", "alice@example.com", "--description", "Retry **twice**.")
		assert.Equal(t, e2eRun{stdout: "PROJ-101: created\n" + e.stub.server.URL + "/browse/PROJ-101\n"}, r)
		assert.Equal(t, []string{"GET /rest/api/3/user/search?query=alice%40example.com", "POST /rest/api/3/issue"}, paths(e.stub.Requests()))
		assert.JSONEq(t, `{"fields":{"project":{"key":"PROJ"},"summary":"Add retry","issuetype":{"name":"Bug"},`+
			`"description":{"type":"doc","version":1,"content":[{"type":"paragraph","content":[`+
			`{"type":"text","text":"Retry "},{"type":"text","text":"twice","marks":[{"type":"strong"}]},{"type":"text","text":"."}]}]},`+
			`"assignee":{"id":"acc-alice"}}}`, string(writes(e.stub.Requests())[0].Body))
		assert.Equal(t, "acc-alice", e.stub.issue("PROJ-101").Assignee.AccountID)
	})

	t.Run("issue comment --body-file", func(t *testing.T) {
		t.Parallel()
		e := newE2EEnv(t, bin)
		require.NoError(t, os.WriteFile(filepath.Join(e.work, "note.md"), []byte("Deployed build `512`.\n"), 0o600))

		r := e.run("issue", "comment", "PROJ-1", "--body-file", "note.md")
		assert.Equal(t, 0, r.code, r.stderr)
		assert.Empty(t, r.stderr)
		assert.Contains(t, r.stdout, "PROJ-1")
		assert.Contains(t, r.stdout, "10101")
		assert.Equal(t, []string{"POST /rest/api/3/issue/PROJ-1/comment"}, paths(e.stub.Requests()))
		assert.JSONEq(t, `{"body":{"type":"doc","version":1,"content":[{"type":"paragraph","content":[`+
			`{"type":"text","text":"Deployed build "},{"type":"text","text":"512","marks":[{"type":"code"}]},{"type":"text","text":"."}]}]}}`,
			string(writes(e.stub.Requests())[0].Body))
	})

	t.Run("link create and link list", func(t *testing.T) {
		t.Parallel()
		e := newE2EEnv(t, bin)

		r := e.run("link", "create", "PROJ-1", "Blocks", "PROJ-2")
		assert.Equal(t, 0, r.code, r.stderr)
		assert.JSONEq(t, `{"type":{"name":"Blocks"},"inwardIssue":{"key":"PROJ-1"},"outwardIssue":{"key":"PROJ-2"}}`,
			string(writes(e.stub.Requests())[0].Body))
		r = e.run("link", "list", "PROJ-1")
		assert.Equal(t, 0, r.code, r.stderr)
		assert.Contains(t, r.stdout, "PROJ-2")
	})

	t.Run("attachment list", func(t *testing.T) {
		t.Parallel()
		e := newE2EEnv(t, bin)

		r := e.run("attachment", "list", "PROJ-1")
		assert.Equal(t, e2eRun{stdout: "10000\t21\timage/png\tshot.png\n10001\t14\ttext/plain\tdebug log.txt\n"}, r)
		assert.Equal(t, []string{"GET /rest/api/3/issue/PROJ-1?fields=attachment"}, paths(e.stub.Requests()))
	})

	t.Run("attachment download --all through a redirect to another origin", func(t *testing.T) {
		t.Parallel()
		e := newE2EEnv(t, bin)
		dir := filepath.Join(e.work, "files")
		require.NoError(t, os.Mkdir(dir, 0o700))

		r := e.run("attachment", "download", "PROJ-1", "--all", "--dir", "files")
		assert.Equal(t, e2eRun{stdout: filepath.Join(dir, "shot.png") + "\n" + filepath.Join(dir, "debug log.txt") + "\n"}, r)
		got, err := os.ReadFile(filepath.Join(dir, "debug log.txt"))
		require.NoError(t, err)
		assert.Equal(t, "line 1\nline 2\n", string(got))
		got, err = os.ReadFile(filepath.Join(dir, "shot.png"))
		require.NoError(t, err)
		assert.Equal(t, "\x89PNG fake image bytes", string(got))

		assert.Equal(t, []string{
			"GET /rest/api/3/issue/PROJ-1?fields=attachment",
			"GET /rest/api/3/attachment/content/10000?redirect=false",
			"GET /rest/api/3/attachment/content/10001?redirect=false",
		}, paths(e.stub.Requests()))
		for _, req := range e.stub.Requests() {
			assert.Equal(t, e.stub.authorization(), req.Header.Get("Authorization"), req.Path)
		}
		media := e.stub.MediaRequests()
		require.Len(t, media, 2)
		for _, req := range media {
			assert.Empty(t, req.Header.Get("Authorization"), "the media host gets no credentials")
			assert.Empty(t, req.Header.Get("Cookie"), "the media host gets no cookie")
		}
	})

	t.Run("attachment upload of two files", func(t *testing.T) {
		t.Parallel()
		e := newE2EEnv(t, bin)
		require.NoError(t, os.WriteFile(filepath.Join(e.work, "notes.txt"), []byte("notes\n"), 0o600))
		require.NoError(t, os.Mkdir(filepath.Join(e.work, "logs"), 0o700))
		require.NoError(t, os.WriteFile(filepath.Join(e.work, "logs", "run.txt"), []byte("run 42 ok\n"), 0o600))

		r := e.run("attachment", "upload", "PROJ-1", "notes.txt", "logs/run.txt")
		assert.Equal(t, e2eRun{stdout: "10101\t6\ttext/plain\tnotes.txt\n10102\t10\ttext/plain\trun.txt\n"}, r)

		uploads := writes(e.stub.Requests())
		require.Len(t, uploads, 2)
		for i, want := range []struct{ name, content string }{{"notes.txt", "notes\n"}, {"run.txt", "run 42 ok\n"}} {
			req := uploads[i]
			assert.Equal(t, "/rest/api/3/issue/PROJ-1/attachments", req.Path)
			assert.Equal(t, "no-check", req.Header.Get("X-Atlassian-Token"))
			assert.Equal(t, strconv.Itoa(len(req.Body)), req.Header.Get("Content-Length"))
			assert.Empty(t, req.Header.Get("Transfer-Encoding"))
			mediaType, params, err := mime.ParseMediaType(req.Header.Get("Content-Type"))
			require.NoError(t, err)
			assert.Equal(t, "multipart/form-data", mediaType)
			mr := multipart.NewReader(bytes.NewReader(req.Body), params["boundary"])
			part, err := mr.NextPart()
			require.NoError(t, err)
			assert.Equal(t, "file", part.FormName())
			assert.Equal(t, want.name, part.FileName())
			content, err := io.ReadAll(part)
			require.NoError(t, err)
			assert.Equal(t, want.content, string(content))
			_, err = mr.NextPart()
			assert.Equal(t, io.EOF, err, "one part per request")
		}
	})

	t.Run("attachment delete", func(t *testing.T) {
		t.Parallel()
		e := newE2EEnv(t, bin)

		r := e.run("attachment", "delete", "10001")
		assert.Equal(t, e2eRun{stdout: "10001\t14\ttext/plain\tdebug log.txt\n"}, r)
		assert.Equal(t, []string{"GET /rest/api/3/attachment/10001", "DELETE /rest/api/3/attachment/10001"}, paths(e.stub.Requests()))
		assert.Equal(t, []string{"10000"}, e.stub.issue("PROJ-1").Attachments)
	})

	t.Run("a --json error is one line on stderr and stdout is empty", func(t *testing.T) {
		t.Parallel()
		e := newE2EEnv(t, bin)

		r := e.run("issue", "view", "PROJ-999", "--json")
		assert.Equal(t, 4, r.code)
		assert.Empty(t, r.stdout)
		assert.Equal(t, 1, strings.Count(r.stderr, "\n"), "one line: %q", r.stderr)
		assert.JSONEq(t, `{"level":"error","error":true,"code":"not_found","message":"Issue does not exist or you do not have permission to see it."}`, r.stderr)
	})

	t.Run("no netrc entry for the server", func(t *testing.T) {
		t.Parallel()
		e := newE2EEnv(t, bin)
		e.writeNetrc("other.example.net")

		r := e.run("issue", "view", "PROJ-1")
		assert.Equal(t, e2eRun{stderr: "[error] no netrc entry for " + e.stub.host() + "\n", code: 2}, r)
		assert.Empty(t, e.stub.Requests())
	})

	t.Run("a usage error exits 1 with usage on stderr", func(t *testing.T) {
		t.Parallel()
		e := newE2EEnv(t, bin)

		r := e.run("issue", "view")
		assert.Equal(t, 1, r.code)
		assert.Empty(t, r.stdout)
		assert.Contains(t, r.stderr, "Usage: j4c issue view")
		assert.Contains(t, r.stderr, `j4c: error: expected "<key> ..."`)
		assert.Empty(t, e.stub.Requests())
	})
}

func mustParseQuery(t *testing.T, raw string) url.Values {
	t.Helper()
	q, err := url.ParseQuery(raw)
	require.NoError(t, err)
	return q
}
