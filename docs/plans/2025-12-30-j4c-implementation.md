# j4c CLI Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Create new `j4c` binary with entity-centric commands and clean printer abstraction.

**Architecture:** Printer interfaces in root package, implementations in `gogh/` wrapping go-gh. Command-centric contexts inject only needed dependencies. Kong for CLI parsing with nested subcommands.

**Tech Stack:** Go, Kong, go-gh, lipgloss, existing jira4claude domain types and http package.

---

## Task 1: Add go-gh Dependency

**Files:**
- Modify: `go.mod`

**Step 1: Add the dependency**

Run:
```bash
go get github.com/cli/go-gh/v2
```

**Step 2: Tidy modules**

Run:
```bash
go mod tidy
```

**Step 3: Commit**

```bash
git add go.mod go.sum
git commit -m "Add go-gh dependency for terminal handling"
```

---

## Task 2: Create Printer Interfaces

**Files:**
- Create: `printer.go`
- Test: `printer_test.go`

**Step 1: Write interface compliance test**

```go
// printer_test.go
package jira4claude_test

import (
	"testing"

	"github.com/fwojciec/jira4claude"
)

func TestPrinterInterfaceComposition(t *testing.T) {
	t.Parallel()

	// Verify Printer embeds all sub-interfaces
	var p jira4claude.Printer

	// These assignments verify interface composition at compile time
	var _ jira4claude.IssuePrinter = p
	var _ jira4claude.LinkPrinter = p
	var _ jira4claude.MessagePrinter = p
}
```

**Step 2: Run test to verify it fails**

Run:
```bash
go test -v ./... -run TestPrinterInterfaceComposition
```

Expected: FAIL - undefined types

**Step 3: Write the interfaces**

```go
// printer.go
package jira4claude

// IssuePrinter handles issue command output.
type IssuePrinter interface {
	Issue(issue *Issue)
	Issues(issues []*Issue)
	Transitions(key string, ts []Transition)
}

// LinkPrinter handles link command output.
type LinkPrinter interface {
	Links(key string, links []Link)
}

// MessagePrinter handles success/error output.
type MessagePrinter interface {
	Success(msg string, keys ...string)
	Error(err error)
}

// Printer combines all output capabilities.
type Printer interface {
	IssuePrinter
	LinkPrinter
	MessagePrinter
}
```

**Step 4: Run test to verify it passes**

Run:
```bash
go test -v ./... -run TestPrinterInterfaceComposition
```

Expected: PASS

**Step 5: Commit**

```bash
git add printer.go printer_test.go
git commit -m "Add Printer interfaces to root package"
```

---

## Task 3: Create gogh Package with IO Struct

**Files:**
- Create: `gogh/gogh.go`
- Test: `gogh/gogh_test.go`

**Step 1: Write the IO tests**

```go
// gogh/gogh_test.go
package gogh_test

import (
	"bytes"
	"os"
	"testing"

	"github.com/fwojciec/jira4claude/gogh"
	"github.com/stretchr/testify/assert"
)

func TestNewIO_WithBuffers(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	io := gogh.NewIO(&out, &errOut)

	assert.Equal(t, &out, io.Out)
	assert.Equal(t, &errOut, io.Err)
	assert.False(t, io.IsTerminal, "buffers should not be detected as terminal")
}

func TestNewIO_WithStdout(t *testing.T) {
	t.Parallel()

	io := gogh.NewIO(os.Stdout, os.Stderr)

	assert.Equal(t, os.Stdout, io.Out)
	assert.Equal(t, os.Stderr, io.Err)
	// IsTerminal depends on actual terminal state - don't assert specific value
}
```

**Step 2: Run test to verify it fails**

Run:
```bash
go test -v ./gogh/... -run TestNewIO
```

Expected: FAIL - package does not exist

**Step 3: Write the IO implementation**

```go
// gogh/gogh.go
package gogh

import (
	"io"
	"os"

	"github.com/cli/go-gh/v2/pkg/term"
)

// IO encapsulates terminal-aware I/O.
type IO struct {
	Out        io.Writer
	Err        io.Writer
	IsTerminal bool
}

// NewIO creates IO with the given writers.
// Terminal detection is derived from out.
func NewIO(out, err io.Writer) *IO {
	return &IO{
		Out:        out,
		Err:        err,
		IsTerminal: isTerminal(out),
	}
}

// isTerminal checks if w is a terminal (file descriptor check).
func isTerminal(w io.Writer) bool {
	if f, ok := w.(*os.File); ok {
		return term.IsTerminal(f)
	}
	return false
}
```

**Step 4: Run test to verify it passes**

Run:
```bash
go test -v ./gogh/... -run TestNewIO
```

Expected: PASS

**Step 5: Commit**

```bash
git add gogh/gogh.go gogh/gogh_test.go
git commit -m "Add gogh package with IO struct"
```

---

## Task 4: Create Styles in gogh Package

**Files:**
- Create: `gogh/style.go`
- Test: `gogh/style_test.go`

**Step 1: Write style tests**

```go
// gogh/style_test.go
package gogh_test

import (
	"os"
	"testing"

	"github.com/fwojciec/jira4claude/gogh"
	"github.com/stretchr/testify/assert"
)

func TestStyles_NoColor(t *testing.T) {
	// Don't run parallel - modifies environment
	original := os.Getenv("NO_COLOR")
	defer os.Setenv("NO_COLOR", original)

	os.Setenv("NO_COLOR", "1")
	styles := gogh.NewStyles()

	// When NO_COLOR is set, styled text should equal input
	assert.Equal(t, "TEST-123", styles.Key("TEST-123"))
}

func TestStyles_WithColor(t *testing.T) {
	// Don't run parallel - modifies environment
	original := os.Getenv("NO_COLOR")
	defer os.Unsetenv("NO_COLOR")
	os.Unsetenv("NO_COLOR")
	defer os.Setenv("NO_COLOR", original)

	styles := gogh.NewStyles()

	// With color, output should contain ANSI codes (be different from input)
	// We just verify it returns something - actual styling depends on terminal
	result := styles.Key("TEST-123")
	assert.Contains(t, result, "TEST-123")
}
```

**Step 2: Run test to verify it fails**

Run:
```bash
go test -v ./gogh/... -run TestStyles
```

Expected: FAIL - NewStyles undefined

**Step 3: Write the styles implementation**

```go
// gogh/style.go
package gogh

import (
	"os"

	"github.com/charmbracelet/lipgloss"
)

// Styles provides text styling for terminal output.
type Styles struct {
	noColor bool
	key     lipgloss.Style
	status  lipgloss.Style
	error   lipgloss.Style
	label   lipgloss.Style
	header  lipgloss.Style
}

// NewStyles creates styles respecting NO_COLOR environment variable.
func NewStyles() *Styles {
	noColor := os.Getenv("NO_COLOR") != ""

	return &Styles{
		noColor: noColor,
		key:     lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12")),
		status:  lipgloss.NewStyle().Foreground(lipgloss.Color("10")),
		error:   lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("9")),
		label:   lipgloss.NewStyle().Foreground(lipgloss.Color("14")),
		header:  lipgloss.NewStyle().Bold(true).Underline(true),
	}
}

// Key styles an issue key.
func (s *Styles) Key(text string) string {
	if s.noColor {
		return text
	}
	return s.key.Render(text)
}

// Status styles a status value.
func (s *Styles) Status(text string) string {
	if s.noColor {
		return text
	}
	return s.status.Render(text)
}

// Error styles an error message.
func (s *Styles) Error(text string) string {
	if s.noColor {
		return text
	}
	return s.error.Render(text)
}

// Label styles a label.
func (s *Styles) Label(text string) string {
	if s.noColor {
		return text
	}
	return s.label.Render(text)
}

// Header styles a header.
func (s *Styles) Header(text string) string {
	if s.noColor {
		return text
	}
	return s.header.Render(text)
}
```

**Step 4: Run test to verify it passes**

Run:
```bash
go test -v ./gogh/... -run TestStyles
```

Expected: PASS

**Step 5: Commit**

```bash
git add gogh/style.go gogh/style_test.go
git commit -m "Add styles with NO_COLOR support"
```

---

## Task 5: Create JSONPrinter

**Files:**
- Create: `gogh/json.go`
- Test: `gogh/json_test.go`

**Step 1: Write JSONPrinter tests**

```go
// gogh/json_test.go
package gogh_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"

	"github.com/fwojciec/jira4claude"
	"github.com/fwojciec/jira4claude/gogh"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJSONPrinter_Issue(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	p := gogh.NewJSONPrinter(&out)

	issue := &jira4claude.Issue{
		Key:     "TEST-123",
		Summary: "Test issue",
		Status:  "Open",
	}

	p.Issue(issue)

	var result map[string]any
	err := json.Unmarshal(out.Bytes(), &result)
	require.NoError(t, err)
	assert.Equal(t, "TEST-123", result["key"])
	assert.Equal(t, "Test issue", result["summary"])
}

func TestJSONPrinter_Issues(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	p := gogh.NewJSONPrinter(&out)

	issues := []*jira4claude.Issue{
		{Key: "TEST-1", Summary: "First"},
		{Key: "TEST-2", Summary: "Second"},
	}

	p.Issues(issues)

	var result []map[string]any
	err := json.Unmarshal(out.Bytes(), &result)
	require.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Equal(t, "TEST-1", result[0]["key"])
}

func TestJSONPrinter_Success(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	p := gogh.NewJSONPrinter(&out)

	p.Success("Created issue", "TEST-123")

	var result map[string]any
	err := json.Unmarshal(out.Bytes(), &result)
	require.NoError(t, err)
	assert.Equal(t, true, result["success"])
	assert.Equal(t, "Created issue", result["message"])
}

func TestJSONPrinter_Error(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	p := gogh.NewJSONPrinter(&out)

	p.Error(errors.New("something went wrong"))

	var result map[string]any
	err := json.Unmarshal(out.Bytes(), &result)
	require.NoError(t, err)
	assert.Equal(t, true, result["error"])
	assert.Equal(t, "something went wrong", result["message"])
}

// Verify JSONPrinter implements Printer interface
var _ jira4claude.Printer = (*gogh.JSONPrinter)(nil)
```

**Step 2: Run test to verify it fails**

Run:
```bash
go test -v ./gogh/... -run TestJSONPrinter
```

Expected: FAIL - JSONPrinter undefined

**Step 3: Write the JSONPrinter implementation**

```go
// gogh/json.go
package gogh

import (
	"encoding/json"
	"io"

	"github.com/fwojciec/jira4claude"
)

// JSONPrinter outputs JSON format.
type JSONPrinter struct {
	out io.Writer
}

// NewJSONPrinter creates a JSON printer.
func NewJSONPrinter(out io.Writer) *JSONPrinter {
	return &JSONPrinter{out: out}
}

func (p *JSONPrinter) encode(v any) {
	enc := json.NewEncoder(p.out)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}

// Issue prints a single issue as JSON.
func (p *JSONPrinter) Issue(issue *jira4claude.Issue) {
	p.encode(issueToMap(issue))
}

// Issues prints multiple issues as JSON array.
func (p *JSONPrinter) Issues(issues []*jira4claude.Issue) {
	result := make([]map[string]any, len(issues))
	for i, issue := range issues {
		result[i] = issueToMap(issue)
	}
	p.encode(result)
}

// Transitions prints transitions as JSON array.
func (p *JSONPrinter) Transitions(key string, ts []jira4claude.Transition) {
	result := make([]map[string]any, len(ts))
	for i, t := range ts {
		result[i] = map[string]any{"id": t.ID, "name": t.Name}
	}
	p.encode(result)
}

// Links prints links as JSON array.
func (p *JSONPrinter) Links(key string, links []jira4claude.Link) {
	result := make([]map[string]any, len(links))
	for i, link := range links {
		result[i] = linkToMap(link)
	}
	p.encode(result)
}

// Success prints a success message as JSON.
func (p *JSONPrinter) Success(msg string, keys ...string) {
	result := map[string]any{
		"success": true,
		"message": msg,
	}
	if len(keys) > 0 {
		result["keys"] = keys
	}
	p.encode(result)
}

// Error prints an error as JSON to stdout (for machine parsing).
func (p *JSONPrinter) Error(err error) {
	p.encode(map[string]any{
		"error":   true,
		"code":    jira4claude.ErrorCode(err),
		"message": jira4claude.ErrorMessage(err),
	})
}

func issueToMap(issue *jira4claude.Issue) map[string]any {
	m := map[string]any{
		"key":         issue.Key,
		"project":     issue.Project,
		"summary":     issue.Summary,
		"description": issue.Description,
		"status":      issue.Status,
		"type":        issue.Type,
		"priority":    issue.Priority,
		"labels":      issue.Labels,
		"parent":      issue.Parent,
		"created":     issue.Created,
		"updated":     issue.Updated,
	}
	if issue.Assignee != nil {
		m["assignee"] = map[string]any{
			"accountId":   issue.Assignee.AccountID,
			"displayName": issue.Assignee.DisplayName,
			"email":       issue.Assignee.Email,
		}
	}
	if issue.Reporter != nil {
		m["reporter"] = map[string]any{
			"accountId":   issue.Reporter.AccountID,
			"displayName": issue.Reporter.DisplayName,
			"email":       issue.Reporter.Email,
		}
	}
	if len(issue.Links) > 0 {
		links := make([]map[string]any, len(issue.Links))
		for i, link := range issue.Links {
			links[i] = linkToMap(link)
		}
		m["links"] = links
	}
	return m
}

func linkToMap(link jira4claude.Link) map[string]any {
	lm := map[string]any{
		"id": link.ID,
		"type": map[string]any{
			"name":    link.Type.Name,
			"inward":  link.Type.Inward,
			"outward": link.Type.Outward,
		},
	}
	if link.OutwardIssue != nil {
		lm["outwardIssue"] = map[string]any{
			"key":     link.OutwardIssue.Key,
			"summary": link.OutwardIssue.Summary,
			"status":  link.OutwardIssue.Status,
		}
	}
	if link.InwardIssue != nil {
		lm["inwardIssue"] = map[string]any{
			"key":     link.InwardIssue.Key,
			"summary": link.InwardIssue.Summary,
			"status":  link.InwardIssue.Status,
		}
	}
	return lm
}
```

**Step 4: Run test to verify it passes**

Run:
```bash
go test -v ./gogh/... -run TestJSONPrinter
```

Expected: PASS

**Step 5: Commit**

```bash
git add gogh/json.go gogh/json_test.go
git commit -m "Add JSONPrinter implementation"
```

---

## Task 6: Create TextPrinter

**Files:**
- Create: `gogh/text.go`
- Test: `gogh/text_test.go`

**Step 1: Write TextPrinter tests**

```go
// gogh/text_test.go
package gogh_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/fwojciec/jira4claude"
	"github.com/fwojciec/jira4claude/gogh"
	"github.com/stretchr/testify/assert"
)

func TestTextPrinter_Issue(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	io := gogh.NewIO(&out, &errOut)
	p := gogh.NewTextPrinter(io)

	issue := &jira4claude.Issue{
		Key:     "TEST-123",
		Summary: "Test issue",
		Status:  "Open",
		Type:    "Task",
	}

	p.Issue(issue)

	output := out.String()
	assert.Contains(t, output, "TEST-123")
	assert.Contains(t, output, "Test issue")
	assert.Contains(t, output, "Open")
}

func TestTextPrinter_Issues_Empty(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	io := gogh.NewIO(&out, &errOut)
	p := gogh.NewTextPrinter(io)

	p.Issues([]*jira4claude.Issue{})

	assert.Contains(t, out.String(), "No issues found")
}

func TestTextPrinter_Issues_Table(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	io := gogh.NewIO(&out, &errOut)
	p := gogh.NewTextPrinter(io)

	issues := []*jira4claude.Issue{
		{Key: "TEST-1", Summary: "First", Status: "Open"},
		{Key: "TEST-2", Summary: "Second", Status: "Done"},
	}

	p.Issues(issues)

	output := out.String()
	assert.Contains(t, output, "TEST-1")
	assert.Contains(t, output, "TEST-2")
	assert.Contains(t, output, "KEY")
}

func TestTextPrinter_Success(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	io := gogh.NewIO(&out, &errOut)
	p := gogh.NewTextPrinter(io)

	p.Success("Created issue", "TEST-123")

	assert.Contains(t, out.String(), "Created issue")
	assert.Contains(t, out.String(), "TEST-123")
}

func TestTextPrinter_Error(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	io := gogh.NewIO(&out, &errOut)
	p := gogh.NewTextPrinter(io)

	p.Error(errors.New("something went wrong"))

	assert.Empty(t, out.String(), "errors should not go to stdout")
	assert.Contains(t, errOut.String(), "Error:")
	assert.Contains(t, errOut.String(), "something went wrong")
}

// Verify TextPrinter implements Printer interface
var _ jira4claude.Printer = (*gogh.TextPrinter)(nil)
```

**Step 2: Run test to verify it fails**

Run:
```bash
go test -v ./gogh/... -run TestTextPrinter
```

Expected: FAIL - TextPrinter undefined

**Step 3: Write the TextPrinter implementation**

```go
// gogh/text.go
package gogh

import (
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/fwojciec/jira4claude"
)

// TextPrinter outputs human-readable text format.
type TextPrinter struct {
	io     *IO
	styles *Styles
}

// NewTextPrinter creates a text printer.
func NewTextPrinter(io *IO) *TextPrinter {
	return &TextPrinter{
		io:     io,
		styles: NewStyles(),
	}
}

// Issue prints a single issue in detail format.
func (p *TextPrinter) Issue(issue *jira4claude.Issue) {
	fmt.Fprintf(p.io.Out, "%s  %s\n", p.styles.Key(issue.Key), issue.Summary)
	fmt.Fprintf(p.io.Out, "Status: %s  Type: %s", p.styles.Status(issue.Status), issue.Type)
	if issue.Priority != "" {
		fmt.Fprintf(p.io.Out, "  Priority: %s", issue.Priority)
	}
	fmt.Fprintln(p.io.Out)

	if issue.Assignee != nil {
		fmt.Fprintf(p.io.Out, "Assignee: %s\n", issue.Assignee.DisplayName)
	}
	if issue.Reporter != nil {
		fmt.Fprintf(p.io.Out, "Reporter: %s\n", issue.Reporter.DisplayName)
	}
	if issue.Parent != "" {
		fmt.Fprintf(p.io.Out, "Parent: %s\n", p.styles.Key(issue.Parent))
	}
	if len(issue.Labels) > 0 {
		fmt.Fprintf(p.io.Out, "Labels: %s\n", p.styles.Label(strings.Join(issue.Labels, ", ")))
	}
	if len(issue.Links) > 0 {
		fmt.Fprintln(p.io.Out, "Links:")
		for _, link := range issue.Links {
			if link.OutwardIssue != nil {
				fmt.Fprintf(p.io.Out, "  %s %s (%s)\n",
					link.Type.Outward,
					p.styles.Key(link.OutwardIssue.Key),
					link.OutwardIssue.Summary)
			}
			if link.InwardIssue != nil {
				fmt.Fprintf(p.io.Out, "  %s %s (%s)\n",
					link.Type.Inward,
					p.styles.Key(link.InwardIssue.Key),
					link.InwardIssue.Summary)
			}
		}
	}

	if issue.Description != "" {
		fmt.Fprintf(p.io.Out, "\n%s\n", issue.Description)
	}
}

// Issues prints multiple issues in table format.
func (p *TextPrinter) Issues(issues []*jira4claude.Issue) {
	if len(issues) == 0 {
		fmt.Fprintln(p.io.Out, "No issues found")
		return
	}

	w := tabwriter.NewWriter(p.io.Out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, p.styles.Header("KEY")+"\t"+p.styles.Header("STATUS")+"\t"+p.styles.Header("ASSIGNEE")+"\t"+p.styles.Header("SUMMARY"))
	for _, issue := range issues {
		assignee := "-"
		if issue.Assignee != nil {
			assignee = issue.Assignee.DisplayName
		}
		summary := truncateString(issue.Summary, 50)
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
			p.styles.Key(issue.Key),
			p.styles.Status(issue.Status),
			assignee,
			summary,
		)
	}
	w.Flush()
}

// Transitions prints available transitions.
func (p *TextPrinter) Transitions(key string, ts []jira4claude.Transition) {
	fmt.Fprintf(p.io.Out, "Available transitions for %s:\n", p.styles.Key(key))
	for _, t := range ts {
		fmt.Fprintf(p.io.Out, "  %s: %s\n", t.ID, p.styles.Status(t.Name))
	}
}

// Links prints issue links.
func (p *TextPrinter) Links(key string, links []jira4claude.Link) {
	if len(links) == 0 {
		fmt.Fprintf(p.io.Out, "No links for %s\n", p.styles.Key(key))
		return
	}

	fmt.Fprintf(p.io.Out, "Links for %s:\n", p.styles.Key(key))
	for _, link := range links {
		if link.OutwardIssue != nil {
			fmt.Fprintf(p.io.Out, "  %s %s (%s)\n",
				link.Type.Outward,
				p.styles.Key(link.OutwardIssue.Key),
				link.OutwardIssue.Summary)
		}
		if link.InwardIssue != nil {
			fmt.Fprintf(p.io.Out, "  %s %s (%s)\n",
				link.Type.Inward,
				p.styles.Key(link.InwardIssue.Key),
				link.InwardIssue.Summary)
		}
	}
}

// Success prints a success message.
func (p *TextPrinter) Success(msg string, keys ...string) {
	styledKeys := make([]any, len(keys))
	for i, k := range keys {
		styledKeys[i] = p.styles.Key(k)
	}

	if len(styledKeys) > 0 {
		// Build format string with placeholders for keys
		format := msg
		for range styledKeys {
			if !strings.Contains(format, "%s") {
				format += " %s"
			}
		}
		fmt.Fprintf(p.io.Out, format+"\n", styledKeys...)
	} else {
		fmt.Fprintln(p.io.Out, msg)
	}
}

// Error prints an error to stderr.
func (p *TextPrinter) Error(err error) {
	fmt.Fprintln(p.io.Err, p.styles.Error("Error: ")+jira4claude.ErrorMessage(err))
}

func truncateString(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen-3]) + "..."
}
```

**Step 4: Run test to verify it passes**

Run:
```bash
go test -v ./gogh/... -run TestTextPrinter
```

Expected: PASS

**Step 5: Commit**

```bash
git add gogh/text.go gogh/text_test.go
git commit -m "Add TextPrinter implementation"
```

---

## Task 7: Add Mock Printer

**Files:**
- Create: `mock/printer.go`

**Step 1: Write the mock**

```go
// mock/printer.go
package mock

import "github.com/fwojciec/jira4claude"

// Printer is a mock implementation of jira4claude.Printer.
type Printer struct {
	IssueFunc       func(issue *jira4claude.Issue)
	IssuesFunc      func(issues []*jira4claude.Issue)
	TransitionsFunc func(key string, ts []jira4claude.Transition)
	LinksFunc       func(key string, links []jira4claude.Link)
	SuccessFunc     func(msg string, keys ...string)
	ErrorFunc       func(err error)

	// Captured calls for assertions
	IssueCalls       []*jira4claude.Issue
	IssuesCalls      [][]*jira4claude.Issue
	TransitionsCalls []struct {
		Key         string
		Transitions []jira4claude.Transition
	}
	LinksCalls []struct {
		Key   string
		Links []jira4claude.Link
	}
	SuccessCalls []struct {
		Msg  string
		Keys []string
	}
	ErrorCalls []error
}

func (p *Printer) Issue(issue *jira4claude.Issue) {
	p.IssueCalls = append(p.IssueCalls, issue)
	if p.IssueFunc != nil {
		p.IssueFunc(issue)
	}
}

func (p *Printer) Issues(issues []*jira4claude.Issue) {
	p.IssuesCalls = append(p.IssuesCalls, issues)
	if p.IssuesFunc != nil {
		p.IssuesFunc(issues)
	}
}

func (p *Printer) Transitions(key string, ts []jira4claude.Transition) {
	p.TransitionsCalls = append(p.TransitionsCalls, struct {
		Key         string
		Transitions []jira4claude.Transition
	}{key, ts})
	if p.TransitionsFunc != nil {
		p.TransitionsFunc(key, ts)
	}
}

func (p *Printer) Links(key string, links []jira4claude.Link) {
	p.LinksCalls = append(p.LinksCalls, struct {
		Key   string
		Links []jira4claude.Link
	}{key, links})
	if p.LinksFunc != nil {
		p.LinksFunc(key, links)
	}
}

func (p *Printer) Success(msg string, keys ...string) {
	p.SuccessCalls = append(p.SuccessCalls, struct {
		Msg  string
		Keys []string
	}{msg, keys})
	if p.SuccessFunc != nil {
		p.SuccessFunc(msg, keys...)
	}
}

func (p *Printer) Error(err error) {
	p.ErrorCalls = append(p.ErrorCalls, err)
	if p.ErrorFunc != nil {
		p.ErrorFunc(err)
	}
}

// Verify interface compliance
var _ jira4claude.Printer = (*Printer)(nil)
```

**Step 2: Run linter to verify**

Run:
```bash
make validate
```

Expected: PASS

**Step 3: Commit**

```bash
git add mock/printer.go
git commit -m "Add mock Printer for testing"
```

---

## Task 8: Create j4c Command Structure

**Files:**
- Create: `cmd/j4c/main.go`
- Create: `cmd/j4c/issue.go`
- Create: `cmd/j4c/link.go`

**Step 1: Create main.go with CLI structure**

```go
// cmd/j4c/main.go
package main

import (
	"os"

	"github.com/alecthomas/kong"
	"github.com/fwojciec/jira4claude"
	"github.com/fwojciec/jira4claude/gogh"
	"github.com/fwojciec/jira4claude/http"
	"github.com/fwojciec/jira4claude/yaml"
)

// CLI defines the command structure for j4c.
type CLI struct {
	Config string `help:"Path to config file" type:"path"`
	JSON   bool   `help:"Output in JSON format" short:"j"`

	Issue IssueCmd `cmd:"" help:"Issue operations"`
	Link  LinkCmd  `cmd:"" help:"Link operations"`
	Init  InitCmd  `cmd:"" help:"Initialize config file"`
}

// IssueContext provides dependencies for issue commands.
type IssueContext struct {
	Service jira4claude.IssueService
	Printer jira4claude.IssuePrinter
	Config  *jira4claude.Config
}

// LinkContext provides dependencies for link commands.
type LinkContext struct {
	Service jira4claude.IssueService
	Printer jira4claude.LinkPrinter
	Config  *jira4claude.Config
}

// MessageContext provides dependencies for message-only commands.
type MessageContext struct {
	Printer jira4claude.MessagePrinter
}

func main() {
	var cli CLI
	ctx := kong.Parse(&cli,
		kong.Name("j4c"),
		kong.Description("A minimal Jira CLI for AI agents"),
		kong.UsageOnError(),
	)

	// Build IO and printer
	io := gogh.NewIO(os.Stdout, os.Stderr)
	var printer jira4claude.Printer
	if cli.JSON {
		printer = gogh.NewJSONPrinter(io.Out)
	} else {
		printer = gogh.NewTextPrinter(io)
	}

	// Init command doesn't need config
	if ctx.Command() == "init" {
		msgCtx := &MessageContext{Printer: printer}
		if err := ctx.Run(msgCtx); err != nil {
			printer.Error(err)
			os.Exit(1)
		}
		return
	}

	// Load config
	cfg, err := loadConfig(cli.Config)
	if err != nil {
		printer.Error(err)
		os.Exit(1)
		return
	}

	// Build service
	client, err := http.NewClient(cfg.Server)
	if err != nil {
		printer.Error(err)
		os.Exit(1)
		return
	}
	svc := http.NewIssueService(client)

	// Build contexts
	issueCtx := &IssueContext{Service: svc, Printer: printer, Config: cfg}
	linkCtx := &LinkContext{Service: svc, Printer: printer, Config: cfg}

	// Run command
	if err := ctx.Run(issueCtx, linkCtx); err != nil {
		printer.Error(err)
		os.Exit(1)
	}
}

func loadConfig(configPath string) (*jira4claude.Config, error) {
	if configPath == "" {
		workDir, err := os.Getwd()
		if err != nil {
			return nil, err
		}
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		configPath, err = yaml.DiscoverConfig(workDir, homeDir)
		if err != nil {
			return nil, err
		}
	}
	return yaml.LoadConfig(configPath)
}
```

**Step 2: Create issue.go with issue commands**

```go
// cmd/j4c/issue.go
package main

import (
	"context"

	"github.com/fwojciec/jira4claude"
)

// IssueCmd groups issue subcommands.
type IssueCmd struct {
	View        IssueViewCmd        `cmd:"" help:"View an issue"`
	List        IssueListCmd        `cmd:"" help:"List issues"`
	Ready       IssueReadyCmd       `cmd:"" help:"List issues ready to work on"`
	Create      IssueCreateCmd      `cmd:"" help:"Create an issue"`
	Update      IssueUpdateCmd      `cmd:"" help:"Update an issue"`
	Transitions IssueTransitionsCmd `cmd:"" help:"List available transitions"`
	Transition  IssueTransitionCmd  `cmd:"" help:"Transition an issue"`
	Assign      IssueAssignCmd      `cmd:"" help:"Assign an issue"`
	Comment     IssueCommentCmd     `cmd:"" help:"Add a comment to an issue"`
}

// IssueViewCmd views an issue.
type IssueViewCmd struct {
	Key string `arg:"" help:"Issue key (e.g., PROJ-123)"`
}

func (c *IssueViewCmd) Run(ctx *IssueContext) error {
	issue, err := ctx.Service.Get(context.Background(), c.Key)
	if err != nil {
		return err
	}
	ctx.Printer.Issue(issue)
	return nil
}

// IssueListCmd lists issues.
type IssueListCmd struct {
	Project  string   `help:"Filter by project" short:"p"`
	Status   string   `help:"Filter by status" short:"s"`
	Assignee string   `help:"Filter by assignee (use 'me' for current user)" short:"a"`
	Parent   string   `help:"Filter by parent issue" short:"P"`
	Labels   []string `help:"Filter by labels" short:"l"`
	JQL      string   `help:"Raw JQL query (overrides other filters)"`
	Limit    int      `help:"Maximum number of results" default:"50"`
}

func (c *IssueListCmd) Run(ctx *IssueContext) error {
	filter := jira4claude.IssueFilter{
		Project:  c.Project,
		Status:   c.Status,
		Assignee: c.Assignee,
		Parent:   c.Parent,
		Labels:   c.Labels,
		JQL:      c.JQL,
		Limit:    c.Limit,
	}
	if filter.Project == "" && filter.JQL == "" {
		filter.Project = ctx.Config.Project
	}

	issues, err := ctx.Service.List(context.Background(), filter)
	if err != nil {
		return err
	}
	ctx.Printer.Issues(issues)
	return nil
}

// IssueReadyCmd lists ready issues.
type IssueReadyCmd struct {
	Project string `help:"Filter by project" short:"p"`
	Limit   int    `help:"Maximum number of results" default:"50"`
}

func (c *IssueReadyCmd) Run(ctx *IssueContext) error {
	project := c.Project
	if project == "" {
		project = ctx.Config.Project
	}

	filter := jira4claude.IssueFilter{
		JQL:   "project = " + project + " AND status != Done ORDER BY created DESC",
		Limit: c.Limit,
	}

	issues, err := ctx.Service.List(context.Background(), filter)
	if err != nil {
		return err
	}

	ready := make([]*jira4claude.Issue, 0, len(issues))
	for _, issue := range issues {
		if jira4claude.IsReady(issue) {
			ready = append(ready, issue)
		}
	}

	ctx.Printer.Issues(ready)
	return nil
}

// IssueCreateCmd creates an issue.
type IssueCreateCmd struct {
	Project     string   `help:"Project key" short:"p"`
	Type        string   `help:"Issue type" short:"t" default:"Task"`
	Summary     string   `help:"Issue summary" short:"s" required:""`
	Description string   `help:"Issue description" short:"d"`
	Priority    string   `help:"Issue priority"`
	Labels      []string `help:"Issue labels" short:"l"`
	Parent      string   `help:"Parent issue key (creates a Subtask)" short:"P"`
}

func (c *IssueCreateCmd) Run(ctx *IssueContext) error {
	project := c.Project
	if project == "" {
		project = ctx.Config.Project
	}

	issueType := c.Type
	if c.Parent != "" {
		issueType = "Subtask"
	}

	issue := &jira4claude.Issue{
		Project:     project,
		Type:        issueType,
		Summary:     c.Summary,
		Description: c.Description,
		Priority:    c.Priority,
		Labels:      c.Labels,
		Parent:      c.Parent,
	}

	created, err := ctx.Service.Create(context.Background(), issue)
	if err != nil {
		return err
	}

	ctx.Printer.(jira4claude.MessagePrinter).Success("Created:", created.Key)
	return nil
}

// IssueUpdateCmd updates an issue.
type IssueUpdateCmd struct {
	Key         string   `arg:"" help:"Issue key"`
	Summary     *string  `help:"New summary" short:"s"`
	Description *string  `help:"New description" short:"d"`
	Priority    *string  `help:"New priority"`
	Assignee    *string  `help:"New assignee" short:"a"`
	Labels      []string `help:"New labels" short:"l"`
	ClearLabels bool     `help:"Clear all labels" name:"clear-labels"`
}

func (c *IssueUpdateCmd) Run(ctx *IssueContext) error {
	update := jira4claude.IssueUpdate{
		Summary:     c.Summary,
		Description: c.Description,
		Priority:    c.Priority,
		Assignee:    c.Assignee,
	}

	if len(c.Labels) > 0 {
		update.Labels = &c.Labels
	} else if c.ClearLabels {
		empty := []string{}
		update.Labels = &empty
	}

	updated, err := ctx.Service.Update(context.Background(), c.Key, update)
	if err != nil {
		return err
	}

	ctx.Printer.(jira4claude.MessagePrinter).Success("Updated:", updated.Key)
	return nil
}

// IssueTransitionsCmd lists available transitions.
type IssueTransitionsCmd struct {
	Key string `arg:"" help:"Issue key"`
}

func (c *IssueTransitionsCmd) Run(ctx *IssueContext) error {
	transitions, err := ctx.Service.Transitions(context.Background(), c.Key)
	if err != nil {
		return err
	}
	ctx.Printer.Transitions(c.Key, transitions)
	return nil
}

// IssueTransitionCmd transitions an issue.
type IssueTransitionCmd struct {
	Key    string `arg:"" help:"Issue key"`
	Status string `help:"Target status name" short:"s" xor:"target"`
	ID     string `help:"Transition ID" short:"i" xor:"target"`
}

func (c *IssueTransitionCmd) Run(ctx *IssueContext) error {
	if c.Status == "" && c.ID == "" {
		return &jira4claude.Error{
			Code:    jira4claude.EValidation,
			Message: "either --status or --id is required",
		}
	}

	transitions, err := ctx.Service.Transitions(context.Background(), c.Key)
	if err != nil {
		return err
	}

	var transitionID string
	if c.ID != "" {
		transitionID = c.ID
	} else {
		for _, t := range transitions {
			if t.Name == c.Status {
				transitionID = t.ID
				break
			}
		}
		if transitionID == "" {
			return &jira4claude.Error{
				Code:    jira4claude.EValidation,
				Message: "status not found: " + c.Status,
			}
		}
	}

	if err := ctx.Service.Transition(context.Background(), c.Key, transitionID); err != nil {
		return err
	}

	ctx.Printer.(jira4claude.MessagePrinter).Success("Transitioned:", c.Key)
	return nil
}

// IssueAssignCmd assigns an issue.
type IssueAssignCmd struct {
	Key       string `arg:"" help:"Issue key"`
	AccountID string `help:"User account ID (omit to unassign)" short:"a"`
}

func (c *IssueAssignCmd) Run(ctx *IssueContext) error {
	if err := ctx.Service.Assign(context.Background(), c.Key, c.AccountID); err != nil {
		return err
	}

	if c.AccountID == "" {
		ctx.Printer.(jira4claude.MessagePrinter).Success("Unassigned:", c.Key)
	} else {
		ctx.Printer.(jira4claude.MessagePrinter).Success("Assigned:", c.Key)
	}
	return nil
}

// IssueCommentCmd adds a comment.
type IssueCommentCmd struct {
	Key  string `arg:"" help:"Issue key"`
	Body string `help:"Comment body" short:"b" required:""`
}

func (c *IssueCommentCmd) Run(ctx *IssueContext) error {
	comment, err := ctx.Service.AddComment(context.Background(), c.Key, c.Body)
	if err != nil {
		return err
	}

	ctx.Printer.(jira4claude.MessagePrinter).Success("Added comment "+comment.ID+" to", c.Key)
	return nil
}
```

**Step 3: Create link.go with link commands**

```go
// cmd/j4c/link.go
package main

import (
	"context"

	"github.com/fwojciec/jira4claude"
)

// LinkCmd groups link subcommands.
type LinkCmd struct {
	Create LinkCreateCmd `cmd:"" help:"Create a link between issues"`
	Delete LinkDeleteCmd `cmd:"" help:"Delete a link between issues"`
	List   LinkListCmd   `cmd:"" help:"List links for an issue"`
}

// LinkCreateCmd creates a link.
type LinkCreateCmd struct {
	InwardKey  string `arg:"" help:"Source issue key"`
	LinkType   string `arg:"" help:"Link type (e.g., Blocks, Clones, Relates)"`
	OutwardKey string `arg:"" help:"Target issue key"`
}

func (c *LinkCreateCmd) Run(ctx *LinkContext) error {
	if err := ctx.Service.Link(context.Background(), c.InwardKey, c.LinkType, c.OutwardKey); err != nil {
		return err
	}

	ctx.Printer.(jira4claude.MessagePrinter).Success("Linked "+c.InwardKey+" "+c.LinkType, c.OutwardKey)
	return nil
}

// LinkDeleteCmd deletes a link.
type LinkDeleteCmd struct {
	Key1 string `arg:"" help:"First issue key"`
	Key2 string `arg:"" help:"Second issue key"`
}

func (c *LinkDeleteCmd) Run(ctx *LinkContext) error {
	if err := ctx.Service.Unlink(context.Background(), c.Key1, c.Key2); err != nil {
		return err
	}

	ctx.Printer.(jira4claude.MessagePrinter).Success("Unlinked "+c.Key1+" and", c.Key2)
	return nil
}

// LinkListCmd lists links for an issue.
type LinkListCmd struct {
	Key string `arg:"" help:"Issue key"`
}

func (c *LinkListCmd) Run(ctx *LinkContext) error {
	issue, err := ctx.Service.Get(context.Background(), c.Key)
	if err != nil {
		return err
	}

	ctx.Printer.Links(c.Key, issue.Links)
	return nil
}
```

**Step 4: Create init.go with init command**

```go
// cmd/j4c/init.go
package main

import (
	"fmt"
	"os"

	"github.com/fwojciec/jira4claude"
	"github.com/fwojciec/jira4claude/yaml"
)

// InitCmd initializes config.
type InitCmd struct {
	Server  string `help:"Jira server URL" required:""`
	Project string `help:"Default project key" required:""`
}

func (c *InitCmd) Run(ctx *MessageContext) error {
	workDir, err := os.Getwd()
	if err != nil {
		return &jira4claude.Error{
			Code:    jira4claude.EInternal,
			Message: "failed to get working directory",
			Inner:   err,
		}
	}

	result, err := yaml.Init(workDir, c.Server, c.Project)
	if err != nil {
		return err
	}

	if result.ConfigCreated {
		ctx.Printer.Success(fmt.Sprintf("Created .jira4claude.yaml"))
	}
	if result.GitignoreAdded {
		ctx.Printer.Success(fmt.Sprintf("Added .jira4claude.yaml to .gitignore"))
	} else if result.GitignoreExists {
		ctx.Printer.Success(fmt.Sprintf(".jira4claude.yaml already in .gitignore"))
	}
	return nil
}
```

**Step 5: Build and verify**

Run:
```bash
go build ./cmd/j4c
```

Expected: Build succeeds

**Step 6: Commit**

```bash
git add cmd/j4c/
git commit -m "Add j4c CLI with entity-centric commands"
```

---

## Task 9: Run Full Validation

**Step 1: Run linter and tests**

Run:
```bash
make validate
```

Expected: All checks pass

**Step 2: Test j4c manually**

Run:
```bash
./j4c --help
./j4c issue --help
./j4c link --help
```

Expected: Help output shows new command structure

**Step 3: Final commit if any fixes needed**

```bash
git add -A
git commit -m "Fix any issues from validation"
```

---

## Summary

After completing all tasks:
- `gogh/` package provides IO and Printer implementations
- `printer.go` defines interfaces in root package
- `mock/printer.go` provides test mock
- `cmd/j4c/` has new CLI with entity-centric commands
- All existing functionality preserved with new structure
