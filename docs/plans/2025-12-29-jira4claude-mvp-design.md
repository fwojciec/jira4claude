# jira4claude MVP - Design Document

## Overview

A minimal Jira CLI designed for AI agents. Zero interactivity, predictable commands, human-readable output with tasteful colors.

## Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Scope | 8 commands (full CRUD) | Complete workflow for agents |
| Output | Human-readable only | Simpler MVP, add JSON later |
| Auth | Netrc only | Same as jira-cli, simple |
| Config | File only (~/.jira4claude.yaml) | Predictable, no env var complexity |
| Descriptions | Plain text → ADF | Easy for agents, auto-convert |
| Transitions | List + transition by ID | Agent orchestrates |
| Errors | Ben Johnson pattern | Code/Message/Inner for all consumers |
| CLI framework | Kong | Struct-based, codegen-friendly |
| Styling | Lipgloss | Non-interactive, tasteful colors |
| Pagination | Fixed limit (~50) | Simple MVP |
| Custom fields | Ignore | Standard fields only for MVP |

## Architecture

Following Ben Johnson's Standard Package Layout:

```
jira4claude/
├── jira4claude.go          # Domain: Issue, User, Comment, Transition
├── issue.go                # IssueService interface
├── error.go                # Error struct + codes
├── config.go               # Config struct
│
├── http/                   # Jira API client (wraps net/http)
│   ├── http.go             # Client, auth (netrc), ADF conversion
│   ├── issue.go            # IssueService implementation
│   └── issue_test.go
│
├── mock/                   # Function-field mocks
│   └── issue.go
│
└── cmd/jira4claude/        # CLI (Kong)
    └── main.go             # CLI struct, wiring, Lipgloss output
```

**Key principles:**
- Root package has zero external dependencies (only stdlib)
- `http/` package owns all HTTP/API interaction
- CLI is thin - parses args, calls services, formats output
- Errors flow up as domain errors, CLI formats for humans

## Dependencies

- `github.com/alecthomas/kong` - CLI parsing
- `github.com/charmbracelet/lipgloss` - Styled output
- `gopkg.in/yaml.v3` - Config file
- `github.com/jdx/go-netrc` - Netrc parsing

## Domain Types

```go
// jira4claude.go
package jira4claude

import "time"

type Issue struct {
    Key         string
    Summary     string
    Description string
    Status      string
    Type        string
    Priority    string
    Assignee    *User
    Reporter    *User
    Labels      []string
    Created     time.Time
    Updated     time.Time
}

type User struct {
    AccountID   string
    DisplayName string
    Email       string
}

type Comment struct {
    ID      string
    Author  *User
    Body    string
    Created time.Time
}

type Transition struct {
    ID   string
    Name string
}
```

## Service Interface

```go
// issue.go
package jira4claude

import "context"

type IssueService interface {
    Create(ctx context.Context, issue *Issue) (*Issue, error)
    Get(ctx context.Context, key string) (*Issue, error)
    List(ctx context.Context, filter IssueFilter) ([]*Issue, error)
    Update(ctx context.Context, key string, update IssueUpdate) (*Issue, error)
    Delete(ctx context.Context, key string) error

    AddComment(ctx context.Context, key string, body string) (*Comment, error)
    Transitions(ctx context.Context, key string) ([]*Transition, error)
    Transition(ctx context.Context, key string, transitionID string) error
    Assign(ctx context.Context, key string, accountID string) error
}

type IssueFilter struct {
    Project  string
    Status   string
    Assignee string
    Limit    int
}

type IssueUpdate struct {
    Summary     *string
    Description *string
    Priority    *string
    Labels      []string
}
```

## Error Handling

Ben Johnson's "Failure is Your Domain" pattern:

```go
// error.go
package jira4claude

const (
    ENotFound     = "not_found"
    EConflict     = "conflict"
    EUnauthorized = "unauthorized"
    EForbidden    = "forbidden"
    EValidation   = "validation"
    ERateLimit    = "rate_limit"
    EInternal     = "internal"
)

type Error struct {
    Code    string // Machine-readable (for app recovery)
    Message string // Human-readable (for end users)
    Inner   error  // Wrapped error (for operators/debugging)
}

func (e Error) Error() string
func (e Error) Unwrap() error
func ErrorCode(err error) string
func ErrorMessage(err error) string
```

**Three consumers:**
- End users → see `Message`
- Applications → check `Code` for recovery
- Operators → see full chain via `Inner`

## Configuration

**Config file** (`~/.jira4claude.yaml`):
```yaml
server: https://acme.atlassian.net
project: J4C
```

**Netrc for auth** (`~/.netrc`):
```
machine acme.atlassian.net
  login user@example.com
  password <api-token>
```

## CLI Structure (Kong)

```go
type CLI struct {
    Issue IssueCmd `cmd:"" help:"Issue operations"`
}

type IssueCmd struct {
    Create     CreateCmd     `cmd:"" help:"Create an issue"`
    View       ViewCmd       `cmd:"" help:"View an issue"`
    List       ListCmd       `cmd:"" help:"List issues"`
    Update     UpdateCmd     `cmd:"" help:"Update an issue"`
    Comment    CommentCmd    `cmd:"" help:"Add a comment"`
    Transition TransitionCmd `cmd:"" help:"Transition an issue"`
    Assign     AssignCmd     `cmd:"" help:"Assign an issue"`
}
```

**Commands:**
```bash
jira4claude issue create --project=J4C --type=Task --summary="Fix bug"
jira4claude issue view J4C-123
jira4claude issue list --project=J4C --status=Open
jira4claude issue update J4C-123 --priority=High
jira4claude issue comment J4C-123 --body="Fixed in PR #456"
jira4claude issue transition J4C-123 --list
jira4claude issue transition J4C-123 --to=21
jira4claude issue assign J4C-123 --assignee=user@example.com
```

## Output Format

Human-readable with Lipgloss colors. Keep lines short (<100 chars) for Claude Code compatibility.

**Create:**
```
Created: J4C-123
https://acme.atlassian.net/browse/J4C-123
```

**View:**
```
J4C-123: Fix login timeout
Status:   Open
Type:     Bug
Priority: High
Assignee: alice@example.com
Reporter: bob@example.com
Created:  2025-12-29 10:30
Updated:  2025-12-29 14:45

Description:
Users are experiencing timeouts when logging in during peak hours.
```

**List:**
```
KEY       STATUS        ASSIGNEE              SUMMARY
J4C-123   Open          alice@example.com     Fix login timeout
J4C-124   In Progress   bob@example.com       Add dark mode
J4C-125   Done          alice@example.com     Update docs
```

**Transitions:**
```
ID    NAME
11    To Do
21    In Progress
31    Done
```

**Styling:**
- Issue keys: bold
- Status: colored (green=open, yellow=in progress, gray=done)
- Errors: red, to stderr

## Mock Pattern

Function-field mocks matching diffstory style:

```go
// mock/issue.go
package mock

var _ jira4claude.IssueService = (*IssueService)(nil)

type IssueService struct {
    CreateFn      func(ctx context.Context, issue *jira4claude.Issue) (*jira4claude.Issue, error)
    GetFn         func(ctx context.Context, key string) (*jira4claude.Issue, error)
    ListFn        func(ctx context.Context, filter jira4claude.IssueFilter) ([]*jira4claude.Issue, error)
    // ... all methods
}

func (s *IssueService) Create(ctx context.Context, issue *jira4claude.Issue) (*jira4claude.Issue, error) {
    return s.CreateFn(ctx, issue)
}
```

## Testing

- External test packages (`package http_test`)
- `t.Parallel()` on all tests and subtests
- `httptest.Server` for API client tests
- Mock package for CLI/integration tests
- `require` for setup, `assert` for assertions
