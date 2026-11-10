package main_test

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/alecthomas/kong"
	"github.com/fwojciec/jira4claude"
	main "github.com/fwojciec/jira4claude/cmd/j4c"
	"github.com/fwojciec/jira4claude/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The README is checked against the CLI it documents:
//
//   - every j4c command in its shell examples parses with the real command
//     grammar (TestReadme_CommandsParse), and every command of the CLI has at
//     least one example;
//   - every long flag named anywhere in it, prose included, exists
//     (TestReadme_FlagsExist);
//   - its exit-code table matches jira4claude.ExitCode (TestReadme_ExitCodes).
//
// Commands are parsed: parsing needs no Jira, no config and no files, so an
// example that names a file or a directory is checked like any other. The
// issue view examples, which read nothing but Jira, are also run through
// main.Run against a mock that serves any key but PROJ-999, so that a field
// name the parser accepts but the command rejects fails the test
// (TestReadme_ViewExamplesRun).
//
// How commands are found. Only fenced blocks tagged bash, sh or shell are
// read. Within one:
//
//  1. A line ending in a backslash continues on the next line.
//  2. The line is split into shell words, with single quotes, double quotes
//     and backslash escapes as the shell reads them. An unquoted "#" that
//     starts a word begins a comment, which is dropped.
//  3. A command substitution "$(...)" or process substitution "<(...)" is
//     replaced by the word SUBST, and its inner text is read by the same
//     rules, so a j4c command inside it is checked too.
//  4. The words are split into simple commands at the operators |, ||, &&,
//     ; and &. In each, redirections (<, >, >>, 2>, 2>&1 and the word after
//     those that take one) are dropped, and so are leading shell keywords
//     (do, then, else, if, while, until, !) and variable assignments
//     (NAME=value).
//  5. A simple command whose first remaining word is j4c is a j4c command;
//     the words after it are its arguments.
//
// Every unquoted word j4c in a shell block must be the first word of a
// simple command, so that no example escapes the check. A shell block the
// rules cannot handle is marked by the line
//
//	<!-- readme-test: skip REASON -->
//
// right before its opening fence, and is not read.

// readmeSkipMarker opens the comment that marks a fenced block as skipped.
const readmeSkipMarker = "<!-- readme-test: skip"

// readmePath returns the path of README.md, found from this source file's
// directory rather than the working directory.
func readmePath(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok, "cannot locate the test source file")
	return filepath.Join(filepath.Dir(file), "..", "..", "README.md")
}

func readReadme(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(readmePath(t))
	require.NoError(t, err)
	return string(data)
}

// fencedBlock is one fenced code block of the README.
type fencedBlock struct {
	lang    string
	line    int // line number of the opening fence, from 1
	lines   []string
	skipped bool
}

// fencedBlocks returns the fenced code blocks of markdown in order.
func fencedBlocks(t *testing.T, markdown string) []fencedBlock {
	t.Helper()
	var blocks []fencedBlock
	var current *fencedBlock
	previous := ""
	for i, line := range strings.Split(markdown, "\n") {
		trimmed := strings.TrimSpace(line)
		switch {
		case current == nil && strings.HasPrefix(trimmed, "```"):
			current = &fencedBlock{
				lang:    strings.TrimSpace(strings.TrimPrefix(trimmed, "```")),
				line:    i + 1,
				skipped: strings.HasPrefix(previous, readmeSkipMarker),
			}
		case current != nil && trimmed == "```":
			blocks = append(blocks, *current)
			current = nil
		case current != nil:
			current.lines = append(current.lines, line)
		}
		if trimmed != "" {
			previous = trimmed
		}
	}
	require.Nil(t, current, "README.md has an unclosed fenced block")
	return blocks
}

// shellLines joins the lines of a shell block that end in a backslash with
// the line after them.
func shellLines(lines []string) []string {
	var out []string
	var pending strings.Builder
	for _, line := range lines {
		if body, ok := strings.CutSuffix(line, `\`); ok {
			pending.WriteString(body)
			pending.WriteString(" ")
			continue
		}
		pending.WriteString(line)
		out = append(out, pending.String())
		pending.Reset()
	}
	if pending.Len() > 0 {
		out = append(out, pending.String())
	}
	return out
}

// shellToken is a word or an operator of a shell line.
type shellToken struct {
	text     string
	operator bool
	// bare is set for a word written without quotes or escapes.
	bare bool
}

// shellScan holds what scanning one shell line found.
type shellScan struct {
	commands [][]string // the arguments of each j4c command
	strays   int        // unquoted j4c words that are not a command word
}

// scanShell reads a shell line by the rules at the top of this file and adds
// what it finds to scan.
func scanShell(t *testing.T, line string, scan *shellScan) {
	t.Helper()
	tokens := shellTokens(t, line, scan)
	var command []shellToken
	flush := func() {
		readSimpleCommand(command, scan)
		command = nil
	}
	for _, tok := range tokens {
		if tok.operator && slices.Contains([]string{"|", "||", "&&", ";", "&"}, tok.text) {
			flush()
			continue
		}
		command = append(command, tok)
	}
	flush()
}

// readSimpleCommand records a j4c command from the tokens of one simple
// command, and counts the j4c words that are not its command word.
func readSimpleCommand(tokens []shellToken, scan *shellScan) {
	assignment := regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*=`)
	keywords := []string{"do", "then", "else", "if", "while", "until", "!"}
	var words []shellToken
	for i := 0; i < len(tokens); i++ {
		tok := tokens[i]
		if tok.operator {
			// A redirection: those that take a target drop the next word.
			if tok.text != "2>&1" {
				i++
			}
			continue
		}
		words = append(words, tok)
	}
	for len(words) > 0 &&
		((words[0].bare && slices.Contains(keywords, words[0].text)) || assignment.MatchString(words[0].text)) {
		words = words[1:]
	}
	for i, w := range words {
		if !w.bare || w.text != "j4c" {
			continue
		}
		if i > 0 {
			scan.strays++
			continue
		}
		args := make([]string, 0, len(words)-1)
		for _, a := range words[1:] {
			args = append(args, a.text)
		}
		scan.commands = append(scan.commands, args)
	}
}

// shellTokens splits a shell line into words and operators. Substitutions
// are replaced by the word SUBST, their inner text scanned into scan.
func shellTokens(t *testing.T, line string, scan *shellScan) []shellToken {
	t.Helper()
	var tokens []shellToken
	var word strings.Builder
	inWord, bare := false, true
	endWord := func() {
		if inWord {
			tokens = append(tokens, shellToken{text: word.String(), bare: bare})
		}
		word.Reset()
		inWord, bare = false, true
	}
	for i := 0; i < len(line); i++ {
		c := line[i]
		switch {
		case c == ' ' || c == '\t':
			endWord()
		case c == '#' && !inWord:
			endWord()
			return tokens
		case c == '\'':
			end := strings.IndexByte(line[i+1:], '\'')
			require.GreaterOrEqual(t, end, 0, "unclosed single quote in %q", line)
			word.WriteString(line[i+1 : i+1+end])
			inWord, bare = true, false
			i += end + 1
		case c == '"':
			i = readDoubleQuoted(t, line, i, &word, scan)
			inWord, bare = true, false
		case c == '\\' && i+1 < len(line):
			word.WriteByte(line[i+1])
			inWord, bare = true, false
			i++
		case (c == '$' || c == '<') && i+1 < len(line) && line[i+1] == '(':
			end := closingParen(t, line, i+1)
			scanShell(t, line[i+2:end], scan)
			word.WriteString("SUBST")
			inWord, bare = true, false
			i = end
		case c == '|' || c == '&' || c == ';' || c == '<' || c == '>':
			endWord()
			op := operatorAt(line[i:])
			tokens = append(tokens, shellToken{text: op, operator: true})
			i += len(op) - 1
		case c == '2' && !inWord && strings.HasPrefix(line[i:], "2>"):
			op := operatorAt(line[i:])
			tokens = append(tokens, shellToken{text: op, operator: true})
			i += len(op) - 1
		default:
			word.WriteByte(c)
			inWord = true
		}
	}
	endWord()
	return tokens
}

// readDoubleQuoted adds the text of the double-quoted string opening at
// line[start] to word, scanning any substitution in it, and returns the index
// of the closing quote.
func readDoubleQuoted(t *testing.T, line string, start int, word *strings.Builder, scan *shellScan) int {
	t.Helper()
	for i := start + 1; i < len(line); i++ {
		switch c := line[i]; {
		case c == '"':
			return i
		case c == '\\' && i+1 < len(line) && strings.ContainsRune(`"\$`+"`", rune(line[i+1])):
			word.WriteByte(line[i+1])
			i++
		case c == '$' && i+1 < len(line) && line[i+1] == '(':
			end := closingParen(t, line, i+1)
			scanShell(t, line[i+2:end], scan)
			word.WriteString("SUBST")
			i = end
		default:
			word.WriteByte(c)
		}
	}
	require.Fail(t, "unclosed double quote", "in %q", line)
	return len(line)
}

// closingParen returns the index of the ")" that closes the "(" at
// line[open], skipping quoted text and nested parentheses.
func closingParen(t *testing.T, line string, open int) int {
	t.Helper()
	depth := 0
	for i := open; i < len(line); i++ {
		switch line[i] {
		case '\'':
			end := strings.IndexByte(line[i+1:], '\'')
			require.GreaterOrEqual(t, end, 0, "unclosed single quote in %q", line)
			i += end + 1
		case '"':
			for i++; i < len(line) && line[i] != '"'; i++ {
				if line[i] == '\\' {
					i++
				}
			}
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	require.Fail(t, "unclosed parenthesis", "in %q", line)
	return len(line)
}

// operatorAt returns the shell operator at the start of s.
func operatorAt(s string) string {
	for _, op := range []string{"2>&1", "2>", "&&", "||", ">>", "|", "&", ";", "<", ">"} {
		if strings.HasPrefix(s, op) {
			return op
		}
	}
	return s[:1]
}

// readmeCommand is a j4c command found in the README.
type readmeCommand struct {
	line int // line of the block's opening fence
	args []string
}

// readmeCommands returns the j4c commands of the README's shell blocks, and
// the line of each skipped shell block.
func readmeCommands(t *testing.T) (commands []readmeCommand, skipped []int) {
	t.Helper()
	for _, block := range fencedBlocks(t, readReadme(t)) {
		if !slices.Contains([]string{"bash", "sh", "shell"}, block.lang) {
			continue
		}
		if block.skipped {
			skipped = append(skipped, block.line)
			continue
		}
		var scan shellScan
		for _, line := range shellLines(block.lines) {
			scanShell(t, line, &scan)
		}
		assert.Zero(t, scan.strays,
			"block at README.md:%d has j4c where the rules do not read a command; rewrite it or mark it %s REASON -->",
			block.line, readmeSkipMarker)
		for _, args := range scan.commands {
			commands = append(commands, readmeCommand{line: block.line, args: args})
		}
	}
	return commands, skipped
}

// newReadmeParser returns a parser for the real command grammar.
func newReadmeParser(t *testing.T) *kong.Kong {
	t.Helper()
	var cli main.CLI
	parser, err := kong.New(&cli, kong.Name("j4c"), kong.Exit(func(int) { t.Error("the parser tried to exit") }))
	require.NoError(t, err)
	return parser
}

func TestReadme_CommandsParse(t *testing.T) {
	t.Parallel()

	commands, skipped := readmeCommands(t)
	require.GreaterOrEqual(t, len(commands), 30, "too few j4c commands found in README.md; is the extraction broken?")
	for _, line := range skipped {
		t.Logf("README.md:%d: shell block skipped", line)
	}

	parser := newReadmeParser(t)
	want := map[string]bool{}
	for _, leaf := range parser.Model.Leaves(true) {
		want[leaf.Path()] = false
	}

	for _, cmd := range commands {
		parser := newReadmeParser(t)
		ctx, err := parser.Parse(cmd.args)
		if err != nil {
			// Reported without stopping, so that one run lists every bad
			// example.
			t.Errorf("README.md:%d: j4c %s: %v", cmd.line, strings.Join(cmd.args, " "), err)
			continue
		}
		want[ctx.Selected().Path()] = true
	}

	for path, found := range want {
		assert.True(t, found, "README.md has no example of j4c %s", path)
	}
}

func TestReadme_ViewExamplesRun(t *testing.T) {
	t.Parallel()

	svc := &mock.IssueService{GetFn: func(_ context.Context, key string) (*jira4claude.Issue, error) {
		if key == "PROJ-999" {
			return nil, &jira4claude.Error{Code: jira4claude.ENotFound, Message: "issue not found"}
		}
		return &jira4claude.Issue{Key: key, Summary: "Example", Status: "To Do", Type: "Task",
			Updated: time.Date(2026, 11, 9, 10, 15, 30, 123_000_000, time.UTC)}, nil
	}}

	commands, _ := readmeCommands(t)
	ran := 0
	for _, cmd := range commands {
		if len(cmd.args) < 2 || !slices.Contains(cmd.args, "issue") || !slices.Contains(cmd.args, "view") {
			continue
		}
		parser := newReadmeParser(t)
		ctx, err := parser.Parse(cmd.args)
		if err != nil || ctx.Selected().Path() != "issue view" {
			continue
		}
		ran++
		res := runCLI(t, testDeps(svc, &mock.UserService{}), cmd.args...)
		want := 0
		if slices.Contains(cmd.args, "PROJ-999") {
			want = jira4claude.ExitCode(&jira4claude.Error{Code: jira4claude.ENotFound})
		}
		assert.Equal(t, want, res.code, "README.md:%d: j4c %s\nstderr: %s",
			cmd.line, strings.Join(cmd.args, " "), res.stderr)
	}
	assert.GreaterOrEqual(t, ran, 5, "too few issue view examples found in README.md")
}

func TestReadme_FlagsExist(t *testing.T) {
	t.Parallel()

	parser := newReadmeParser(t)
	known := map[string]bool{}
	var walk func(n *kong.Node)
	walk = func(n *kong.Node) {
		for _, f := range n.Flags {
			known["--"+f.Name] = true
		}
		for _, child := range n.Children {
			walk(child)
		}
	}
	walk(parser.Model.Node)

	flag := regexp.MustCompile(`(?:^|[^\w-])(--[a-z][a-z0-9-]*)`)
	for i, line := range strings.Split(readReadme(t), "\n") {
		for _, m := range flag.FindAllStringSubmatch(line, -1) {
			assert.True(t, known[m[1]], "README.md:%d names %s, which no j4c command has", i+1, m[1])
		}
	}
}

func TestReadme_ExitCodes(t *testing.T) {
	t.Parallel()

	// The table follows the "## Exit codes" heading. Its rows are
	// "| N | `code` | meaning |", the code being the one a --json error
	// line carries; the row for 0 has none.
	text := readReadme(t)
	_, section, found := strings.Cut(text, "\n## Exit codes\n")
	require.True(t, found, `README.md has no "## Exit codes" section`)
	if next := strings.Index(section, "\n## "); next >= 0 {
		section = section[:next]
	}

	row := regexp.MustCompile("^\\|\\s*(\\d+)\\s*\\|\\s*(?:`([a-z_]+)`)?\\s*\\|")
	table := map[int]string{}
	for _, line := range strings.Split(section, "\n") {
		m := row.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		n, err := strconv.Atoi(m[1])
		require.NoError(t, err)
		_, dup := table[n]
		require.False(t, dup, "exit code %d is listed twice", n)
		table[n] = m[2]
	}

	assert.Empty(t, table[0], "exit code 0 should name no error code")
	assert.Equal(t, 0, jira4claude.ExitCode(nil))
	codes := []string{
		jira4claude.EValidation, jira4claude.EUnauthorized, jira4claude.EForbidden,
		jira4claude.ENotFound, jira4claude.EConflict, jira4claude.ERateLimit, jira4claude.EInternal,
	}
	listed := []string{}
	for n, code := range table {
		if n == 0 {
			continue
		}
		listed = append(listed, code)
		assert.Equal(t, n, jira4claude.ExitCode(&jira4claude.Error{Code: code}),
			"README.md lists exit %d for %q", n, code)
	}
	assert.ElementsMatch(t, codes, listed, "README.md's exit-code table should list every error code once")
}
