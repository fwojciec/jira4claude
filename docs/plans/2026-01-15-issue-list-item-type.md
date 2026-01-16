# Issue List Item Type

Separate view types for list display vs detail display to avoid unnecessary ADF-to-markdown conversion.

## Problem

`ToIssueView()` always converts description and comments from ADF to markdown, even when displaying issue lists that only show key, status, priority, and summary. This causes:

- Wasted CPU cycles converting unused fields
- Spurious warnings when descriptions contain unsupported ADF nodes (e.g., tables)

## Solution

Create a minimal `IssueListItem` type for list contexts.

### New Types (view.go)

```go
// IssueListItem is a minimal representation for list display.
// It contains only the fields needed for issue lists, avoiding
// expensive ADF-to-markdown conversion.
type IssueListItem struct {
    Key      string `json:"key"`
    Status   string `json:"status"`
    Priority string `json:"priority,omitempty"`
    Summary  string `json:"summary"`
}

// ToIssueListItems converts domain issues to list items.
// No ADF conversion is performed - this is a direct field copy.
func ToIssueListItems(issues []*Issue) []IssueListItem {
    items := make([]IssueListItem, len(issues))
    for i, issue := range issues {
        items[i] = IssueListItem{
            Key:      issue.Key,
            Status:   issue.Status,
            Priority: issue.Priority,
            Summary:  issue.Summary,
        }
    }
    return items
}
```

### Printer Interface (printer.go)

```go
type Printer interface {
    Issue(view IssueView)
    Issues(items []IssueListItem)  // Changed from []IssueView
    // ...
}
```

### Command Changes (cmd/j4c/issue.go)

List and ready commands simplify:

```go
func (c *IssueListCmd) Run(ctx *IssueContext) error {
    // ...
    issues, err := ctx.Service.List(context.Background(), filter)
    if err != nil {
        return err
    }
    items := jira4claude.ToIssueListItems(issues)
    ctx.Printer.Issues(items)
    return nil
}
```

## Files Changed

| File | Change |
|------|--------|
| `view.go` | Add `IssueListItem` struct and `ToIssueListItems()` |
| `printer.go` | Update `Printer.Issues()` signature |
| `markdown/printer.go` | Update `Issues()` implementation |
| `json/printer.go` | Update `Issues()` implementation |
| `mock/printer.go` | Update `Issues()` implementation |
| `cmd/j4c/issue.go` | Simplify list/ready commands |
| `*_test.go` | Update corresponding tests |

## Outcome

- List commands no longer trigger ADF conversion
- No spurious warnings about unsupported ADF nodes
- Cleaner separation between list and detail contexts
