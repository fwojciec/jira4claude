# Output System and Command Structure Design

## Overview

Redesign of jira4claude's output formatting and command structure. Creates a new `j4c` binary with entity-centric commands (mirroring Jira REST API like `gh` mirrors GitHub API) and a clean printer abstraction backed by `go-gh`.

## Goals

- **Testability** - Output easy to capture and assert
- **Scalability** - Prepare for more formats and commands
- **Consistency** - Centralized formatting, no duplication
- **Standards** - NO_COLOR support, proper stderr usage, TTY detection
- **Clear patterns** - Documented, easy to extend

## Command Structure

New binary: `j4c`

### Issue Commands

```
j4c issue view KEY
j4c issue list [--project, --status, --assignee, --jql, --limit]
j4c issue ready [--project, --limit]
j4c issue create -s "summary" [--project, --type, --description, --priority, --labels, --parent]
j4c issue edit KEY [--summary, --description, --priority, --assignee, --labels]
j4c issue transitions KEY
j4c issue transition KEY (-s STATUS | -i ID)
j4c issue assign KEY [--account-id]
j4c issue comment KEY -b "body"
```

### Link Commands

```
j4c link create KEY1 TYPE KEY2
j4c link delete KEY1 KEY2
j4c link list KEY
```

### Other Commands

```
j4c init --server URL --project KEY
```

### Global Flags

- `--json`, `-j` - Output in JSON format
- `--config` - Path to config file

### Design Rationale

- **Entity-centric** - Commands grouped by resource (`issue`, `link`), like `gh`
- **Mirrors Jira API** - Resource structure matches REST API
- **Friendly verbs** - `view` not `get`, `edit` not `update`, `create`/`delete`
- **Plural for list, singular for action** - `issue transitions` lists, `issue transition` executes

## Package Structure

```
jira4claude/
├── printer.go              # Printer interfaces (domain)
├── gogh/
│   ├── gogh.go             # IO struct, constructors
│   ├── text.go             # TextPrinter implementation
│   ├── json.go             # JSONPrinter implementation
│   └── style.go            # lipgloss styles, NO_COLOR handling
├── cmd/
│   ├── jira4claude/        # existing binary (keep until migration complete)
│   └── j4c/                # new binary
│       └── main.go
└── ...
```

## Printer Interfaces

In `jira4claude/printer.go`:

```go
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

### Design Rationale

- **Interface Segregation** - Small focused interfaces that compose
- **Domain-level** - Interfaces in root package, implementations in `gogh/`
- **Type-safe** - Methods per entity type, not generic `Print(any)`
- **No return errors** - Output failures mean app is broken; keeps command code clean

## Command Contexts

Each command group gets its own context with exactly what it needs:

```go
type IssueContext struct {
    Service IssueService
    Printer IssuePrinter
    Config  *Config
}

type LinkContext struct {
    Service IssueService
    Printer LinkPrinter
    Config  *Config
}
```

Commands declare minimal dependencies:

```go
func (c *IssueViewCmd) Run(ctx *IssueContext) error {
    issue, err := ctx.Service.Get(context.Background(), c.Key)
    if err != nil {
        return err
    }
    ctx.Printer.Issue(issue)
    return nil
}
```

## The `gogh/` Package

### IO Struct

```go
package gogh

import (
    "io"
    "os"

    "github.com/cli/go-gh/v2/pkg/term"
)

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

func isTerminal(w io.Writer) bool {
    if f, ok := w.(*os.File); ok {
        return term.IsTerminal(f)
    }
    return false
}
```

Unified constructor works for both production and tests:

```go
// Production
io := gogh.NewIO(os.Stdout, os.Stderr)

// Test
var out, err bytes.Buffer
io := gogh.NewIO(&out, &err)
```

### TextPrinter

```go
type TextPrinter struct {
    io *IO
}

func NewTextPrinter(io *IO) *TextPrinter {
    return &TextPrinter{io: io}
}

func (p *TextPrinter) Issue(issue *jira4claude.Issue) {
    // styled detail view using lipgloss
}

func (p *TextPrinter) Issues(issues []*jira4claude.Issue) {
    // table using go-gh tableprinter
}

func (p *TextPrinter) Success(msg string, keys ...string) {
    // message with styled keys to io.Out
}

func (p *TextPrinter) Error(err error) {
    // styled error to io.Err (stderr)
}
```

### JSONPrinter

```go
type JSONPrinter struct {
    out io.Writer
}

func NewJSONPrinter(out io.Writer) *JSONPrinter {
    return &JSONPrinter{out: out}
}

func (p *JSONPrinter) Issue(issue *jira4claude.Issue) {
    json.NewEncoder(p.out).Encode(issueToMap(issue))
}

func (p *JSONPrinter) Error(err error) {
    // JSON error to stdout (not stderr) for machine parsing
    json.NewEncoder(p.out).Encode(map[string]any{
        "error":   true,
        "message": jira4claude.ErrorMessage(err),
    })
}
```

**Key difference:** TextPrinter errors go to stderr, JSONPrinter errors go to stdout for machine parsing.

## Main Wiring

```go
package main

func main() {
    var cli CLI
    ctx := kong.Parse(&cli)

    // Build IO and printer
    io := gogh.NewIO(os.Stdout, os.Stderr)
    var printer jira4claude.Printer
    if cli.JSON {
        printer = gogh.NewJSONPrinter(io.Out)
    } else {
        printer = gogh.NewTextPrinter(io)
    }

    // Load config, build service
    cfg, _ := loadConfig(cli.Config)
    svc := http.NewIssueService(http.NewClient(cfg.Server))

    // Build contexts per command group
    issueCtx := &IssueContext{Service: svc, Printer: printer, Config: cfg}
    linkCtx := &LinkContext{Service: svc, Printer: printer, Config: cfg}

    // Kong binds and runs
    err := ctx.Run(issueCtx, linkCtx)
    if err != nil {
        printer.Error(err)
        os.Exit(1)
    }
}
```

## Dependencies

New dependency:
- `github.com/cli/go-gh/v2` - Terminal detection, table printing

Existing (continue using):
- `github.com/alecthomas/kong` - CLI parsing
- `github.com/charmbracelet/lipgloss` - Styling

## Migration Strategy

1. Create `cmd/j4c/` with new structure
2. Implement `gogh/` package with printer interfaces
3. Port commands one at a time
4. Validate parity with existing `jira4claude`
5. Delete `cmd/jira4claude/` when ready

## Testing Strategy

- Mock `Printer` interfaces in `mock/` package
- Use `gogh.NewIO(&buf, &buf)` to capture output in tests
- TextPrinter and JSONPrinter tested independently
- Commands tested with mock service and mock printer
