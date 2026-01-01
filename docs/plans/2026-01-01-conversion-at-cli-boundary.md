# Conversion at CLI Boundary Design

Move all ADF/markdown conversion from HTTP layer to CLI layer for cleaner separation of concerns.

## Problem

Current architecture has conversion logic spread across layers:
- HTTP layer injects `Converter`, calls `textOrADF` for input
- CLI layer does conversion for output display
- Warnings are difficult to surface because conversion happens deep in HTTP

## Solution

CLI layer owns all conversion. HTTP layer is pure transport dealing only in ADF (Jira's native format).

```
Before:
  CLI (markdown) -> HTTP (converts, sends ADF) -> Jira
  Jira -> HTTP (receives ADF) -> CLI (converts to markdown)

After:
  CLI (converts markdown->ADF) -> HTTP (ADF passthrough) -> Jira
  Jira -> HTTP (ADF passthrough) -> CLI (converts ADF->markdown, shows warnings)
```

## Design

### 1. ADF Type Alias

Add semantic type alias to root package:

```go
// jira4claude.go
// ADF represents an Atlassian Document Format document.
type ADF = map[string]any
```

### 2. Domain Types (ADF Only)

Remove dual-field pattern. Domain types use Jira's native format:

```go
// issue.go
type Issue struct {
    Key         string
    Project     string
    Summary     string
    Description ADF       // was: Description string + DescriptionADF map[string]any
    Status      string
    Type        string
    Priority    string
    Assignee    *User
    Reporter    *User
    Labels      []string
    Links       []*IssueLink
    Comments    []*Comment
    Parent      string
    Created     time.Time
    Updated     time.Time
}

type Comment struct {
    ID      string
    Author  *User
    Body    ADF           // was: Body string + BodyADF map[string]any
    Created time.Time
}

type IssueUpdate struct {
    Summary     *string
    Description *ADF      // was: *string
    Priority    *string
    Assignee    *string
    Labels      *[]string
}
```

### 3. IssueService Interface

Methods accept/return ADF:

```go
type IssueService interface {
    Create(ctx context.Context, issue *Issue) (*Issue, error)
    Get(ctx context.Context, key string) (*Issue, error)
    List(ctx context.Context, filter IssueFilter) ([]*Issue, error)
    Update(ctx context.Context, key string, update IssueUpdate) (*Issue, error)
    Delete(ctx context.Context, key string) error
    AddComment(ctx context.Context, key string, body ADF) (*Comment, error)  // was: body string
    Transitions(ctx context.Context, key string) ([]*Transition, error)
    Transition(ctx context.Context, key, transitionID string) error
    Assign(ctx context.Context, key, accountID string) error
    Link(ctx context.Context, inwardKey, linkType, outwardKey string) error
    Unlink(ctx context.Context, key1, key2 string) error
}
```

### 4. Converter Interface

Return warnings as `[]string` instead of error:

```go
// converter.go
type Converter interface {
    // ToADF converts GitHub-flavored markdown to ADF.
    // Returns the ADF document and any warnings about skipped/unsupported content.
    ToADF(markdown string) (ADF, []string)

    // ToMarkdown converts ADF to GitHub-flavored markdown.
    // Returns the markdown string and any warnings about skipped/unsupported content.
    ToMarkdown(adf ADF) (string, []string)
}
```

Warnings are informational - operation always succeeds with best-effort output.

### 5. HTTP Layer (Pure Transport)

Remove Converter dependency:

```go
// http/client.go
type Client struct {
    baseURL    *url.URL
    username   string
    password   string
    httpClient *http.Client
    // converter field removed
}

func NewClient(baseURL string, opts ...Option) (*Client, error)
```

IssueService implementation passes ADF directly:

```go
// http/issue.go
func (s *IssueService) Create(ctx context.Context, issue *jira4claude.Issue) (*jira4claude.Issue, error) {
    reqBody := createIssueRequest{
        Fields: createIssueFields{
            Description: issue.Description,  // ADF passed directly
            // ...
        },
    }
    // ...
}

func (s *IssueService) AddComment(ctx context.Context, key string, body jira4claude.ADF) (*jira4claude.Comment, error) {
    reqBody := map[string]any{"body": body}  // ADF passed directly
    // ...
}
```

### 6. Warning Display

Add Warning to MessagePrinter:

```go
// printer.go
type MessagePrinter interface {
    Success(msg string, keys ...string)
    Warning(msg string)  // NEW
    Error(err error)
}
```

Implementation in gogh:

```go
// gogh/style.go
type Styles struct {
    // ...existing...
    warn lipgloss.Style
}

func NewStyles() *Styles {
    return &Styles{
        // ...existing...
        warn: lipgloss.NewStyle().Foreground(lipgloss.Color("11")),  // yellow
    }
}

func (s *Styles) Warning(text string) string {
    if s.noColor { return "warning: " + text }
    return s.warn.Render("warning: " + text)
}

// gogh/text.go
func (p *TextPrinter) Warning(msg string) {
    fmt.Fprintln(p.io.Err, p.styles.Warning(msg))
}

// gogh/json.go
func (p *JSONPrinter) Warning(msg string) {
    fmt.Fprintln(p.io.Err, "warning: "+msg)
}
```

### 7. CLI View Models

View types for display with JSON tags:

```go
// cmd/j4c/view.go
type issueView struct {
    Key         string        `json:"key"`
    Summary     string        `json:"summary"`
    Description string        `json:"description,omitempty"`
    Status      string        `json:"status"`
    Type        string        `json:"type"`
    Priority    string        `json:"priority,omitempty"`
    Assignee    string        `json:"assignee,omitempty"`
    Reporter    string        `json:"reporter,omitempty"`
    Labels      []string      `json:"labels,omitempty"`
    Links       []linkView    `json:"links,omitempty"`
    Comments    []commentView `json:"comments,omitempty"`
    Parent      string        `json:"parent,omitempty"`
    Created     string        `json:"created"`
    Updated     string        `json:"updated"`
    URL         string        `json:"url,omitempty"`
}

type commentView struct {
    ID      string `json:"id"`
    Author  string `json:"author"`
    Body    string `json:"body"`
    Created string `json:"created"`
}

type linkView struct {
    Type       string `json:"type"`
    Direction  string `json:"direction"`  // "outward" or "inward"
    IssueKey   string `json:"issueKey"`
    Summary    string `json:"summary"`
}
```

Conversion helper:

```go
func toIssueView(issue *jira4claude.Issue, conv jira4claude.Converter, warn func(string), serverURL string) issueView {
    desc, warnings := conv.ToMarkdown(issue.Description)
    for _, w := range warnings { warn(w) }

    var comments []commentView
    for _, c := range issue.Comments {
        body, warnings := conv.ToMarkdown(c.Body)
        for _, w := range warnings { warn(w) }
        comments = append(comments, commentView{
            ID:      c.ID,
            Author:  displayName(c.Author),
            Body:    body,
            Created: c.Created.Format(time.RFC3339),
        })
    }

    return issueView{
        Key:         issue.Key,
        Summary:     issue.Summary,
        Description: desc,
        Status:      issue.Status,
        // ...
    }
}
```

### 8. CLI Handler Pattern

```go
func (c *IssueViewCmd) Run(ctx *IssueContext) error {
    issue, err := ctx.Service.Get(context.Background(), c.Key)
    if err != nil { return err }

    view := toIssueView(issue, ctx.Converter, ctx.Printer.Warning, ctx.ServerURL)
    ctx.Printer.Issue(view)
    return nil
}

func (c *IssueCreateCmd) Run(ctx *IssueContext) error {
    // Convert markdown input to ADF
    desc, warnings := ctx.Converter.ToADF(c.Description)
    for _, w := range warnings { ctx.Printer.Warning(w) }

    issue := &jira4claude.Issue{
        Project:     ctx.Project,
        Summary:     c.Summary,
        Description: desc,  // ADF
        Type:        c.Type,
    }

    created, err := ctx.Service.Create(context.Background(), issue)
    if err != nil { return err }

    ctx.Printer.Success("Created:", created.Key)
    return nil
}
```

## Benefits

1. **Clear separation**: HTTP is transport, CLI is presentation
2. **Natural warning display**: Warnings surface at CLI boundary where they can be shown
3. **Simpler HTTP tests**: No mock converter needed
4. **Agent-friendly output**: Both JSON and text show markdown (fewer tokens than ADF)
5. **Explicit conversion**: All conversion visible in CLI handlers
6. **Custom JSON formatting**: View models control serialization (dates, omitempty, etc.)

## Migration

### Tasks to Update

| Task | Change |
|------|--------|
| J4C-74 | Keep - CLI needs Converter injection |
| J4C-75 | Modify - return `[]string` not result type |
| J4C-76 | Simplify - just add `Printer.Warning()` |
| J4C-78 | Keep - rename adf -> goldmark last |

### New Tasks Needed

1. Add `ADF` type alias to root package
2. Update domain types to ADF-only (remove dual fields)
3. Update `IssueService` interface signatures
4. Remove Converter from HTTP layer
5. Update HTTP implementation to use ADF directly
6. Add `Warning()` to MessagePrinter interface
7. Implement Warning in gogh TextPrinter and JSONPrinter
8. Update Converter interface to return `[]string` warnings
9. Update goldmark implementation to accumulate warnings
10. Create view types in CLI package
11. Create conversion helpers with warning callbacks
12. Update IssuePrinter interface to use view types
13. Update gogh printer implementations for view types
14. Update CLI handlers to use view model pattern
15. Rename adf package to goldmark (after all else done)

### Order

```
1. Foundation: ADF type alias, domain types, interfaces
2. HTTP: Remove Converter, use ADF directly
3. Converter: Change to []string warnings, accumulate
4. Printer: Add Warning() method
5. CLI: View models, conversion helpers, handler updates
6. Cleanup: Rename adf -> goldmark
```

## Reversal of Previous Work

This design reverses J4C-70 (Inject Converter into HTTP). That work is now done but will be removed. The Converter injection was the wrong abstraction - conversion belongs at the CLI boundary, not in HTTP transport.
