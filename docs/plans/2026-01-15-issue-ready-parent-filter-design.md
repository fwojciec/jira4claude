# Issue Ready Parent Filter Design

Add `--parent` flag to `j4c issue ready` command to filter for unblocked issues that are children of a specific parent.

## Problem

The `ready` command currently shows all unblocked issues in a project. When working with epics or parent issues that have subtasks, it's useful to scope the ready list to children of a specific parent.

## Solution

Rather than a quick fix, refactor `IssueFilter` to support the fields that `ready` currently hardcodes in raw JQL. This enables consistent patterns across commands.

## Design

### Domain Type Changes

**`issue.go` - IssueFilter struct**

Add two new fields:

```go
type IssueFilter struct {
    Project       string
    Status        string   // Exact match: status = "X"
    ExcludeStatus string   // Exclusion: status != "X"  (NEW)
    Assignee      string
    Parent        string
    Labels        []string
    OrderBy       string   // e.g., "created DESC"  (NEW)
    JQL           string   // Raw JQL; overrides other fields
    Limit         int
}
```

Semantics:
- `Status` and `ExcludeStatus`: if both set, `Status` wins (more specific)
- `OrderBy`: appends `ORDER BY {value}` to generated JQL
- `JQL`: still overrides everything when set

### JQL Builder Changes

**`http/issue.go` - buildJQL function**

```go
func buildJQL(filter jira4claude.IssueFilter) string {
    clauses := make([]string, 0, 5+len(filter.Labels))

    if filter.Project != "" {
        clauses = append(clauses, fmt.Sprintf("project = %q", filter.Project))
    }
    if filter.Status != "" {
        clauses = append(clauses, fmt.Sprintf("status = %q", filter.Status))
    }
    if filter.ExcludeStatus != "" && filter.Status == "" {
        clauses = append(clauses, fmt.Sprintf("status != %q", filter.ExcludeStatus))
    }
    if filter.Assignee != "" {
        clauses = append(clauses, fmt.Sprintf("assignee = %q", filter.Assignee))
    }
    if filter.Parent != "" {
        clauses = append(clauses, fmt.Sprintf("parent = %q", filter.Parent))
    }
    for _, label := range filter.Labels {
        clauses = append(clauses, fmt.Sprintf("labels = %q", label))
    }

    jql := strings.Join(clauses, " AND ")

    if filter.OrderBy != "" {
        jql += " ORDER BY " + filter.OrderBy
    }

    return jql
}
```

### CLI Command Changes

**IssueReadyCmd** - add `--parent` flag, use filter fields:

```go
type IssueReadyCmd struct {
    Project string `help:"Filter by project" short:"p"`
    Parent  string `help:"Filter by parent issue" short:"P"`
    Limit   int    `help:"Maximum number of results" default:"50"`
}

func (c *IssueReadyCmd) Run(ctx *IssueContext) error {
    project := c.Project
    if project == "" {
        project = ctx.Config.Project
    }

    filter := jira4claude.IssueFilter{
        Project:       project,
        Parent:        c.Parent,
        ExcludeStatus: "Done",
        OrderBy:       "created DESC",
        Limit:         c.Limit,
    }
    // ... rest unchanged
}
```

**IssueListCmd** - add new flags for consistency:

```go
type IssueListCmd struct {
    // ... existing fields ...
    ExcludeStatus string `help:"Exclude issues with this status" name:"exclude-status"`
    OrderBy       string `help:"Order results (e.g., 'created DESC')" name:"order-by"`
}
```

## Testing

1. **`http/issue_test.go` - buildJQL tests**
   - `ExcludeStatus` alone generates `status != "Done"`
   - `Status` takes precedence over `ExcludeStatus`
   - `OrderBy` appends correctly
   - Combined filters with new fields

2. **`cmd/j4c/issue_test.go` - command tests**
   - `IssueReadyCmd` with `--parent` flag
   - `IssueListCmd` with `--exclude-status` and `--order-by` flags

## Files Changed

| File | Changes |
|------|---------|
| `issue.go` | Add `ExcludeStatus`, `OrderBy` to `IssueFilter` |
| `http/issue.go` | Update `buildJQL()` for new fields |
| `cmd/j4c/issue.go` | Add `--parent` to `ready`, add `--exclude-status`/`--order-by` to `list` |
| `http/issue_test.go` | Tests for `buildJQL()` changes |
| `cmd/j4c/issue_test.go` | Tests for new CLI flags |
