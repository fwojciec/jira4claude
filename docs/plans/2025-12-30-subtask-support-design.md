# Subtask Support Design

## Problem

The jira4claude CLI cannot create or manage subtasks. Users need to:
1. Create subtasks with a parent issue
2. View parent info when looking at a subtask
3. List all subtasks of a parent issue

## Design Decisions

- **Parent field**: Store as simple string (issue key) rather than full struct - keeps domain simple
- **Auto-set type**: When `--parent` is specified, automatically set type to "Subtask" - avoids redundant flags
- **List children**: Use `--parent` filter on existing `list` command for consistency with other filters
- **View display**: Show parent only (not children) - use `list --parent` to see children

## Changes

### Domain (`issue.go`)

Add `Parent` field to `Issue`:

```go
type Issue struct {
    // ... existing fields ...
    Parent string // Parent issue key (empty if not a subtask)
}
```

Add `Parent` field to `IssueFilter`:

```go
type IssueFilter struct {
    // ... existing fields ...
    Parent string // Filter by parent issue key
}
```

### CLI (`cmd/jira4claude/main.go`)

**CreateCmd** - add `--parent` flag:

```go
type CreateCmd struct {
    // ... existing fields ...
    Parent string `help:"Parent issue key (creates Subtask)" short:"P"`
}
```

In `Run()`, if `Parent` is set, override `Type` to "Subtask".

**ListCmd** - add `--parent` filter:

```go
type ListCmd struct {
    // ... existing fields ...
    Parent string `help:"Filter by parent issue (list subtasks)" short:"P"`
}
```

### API Layer (`http/issue.go`)

**Create** - send parent in request:

```go
if issue.Parent != "" {
    fields["parent"] = map[string]any{"key": issue.Parent}
}
```

**Parse response** - extract parent:

```go
type issueResponse struct {
    Fields struct {
        // ... existing fields ...
        Parent *struct {
            Key string `json:"key"`
        } `json:"parent"`
    } `json:"fields"`
}
```

**Build JQL** - add parent clause:

```go
if filter.Parent != "" {
    clauses = append(clauses, fmt.Sprintf("parent = %q", filter.Parent))
}
```

### Display (`cmd/jira4claude/main.go`)

**printIssueDetail** - show parent:

```go
if issue.Parent != "" {
    fmt.Printf("Parent: %s\n", keyStyle.Render(issue.Parent))
}
```

**issueToMap** - include parent in JSON:

```go
m["parent"] = issue.Parent
```

## Usage Examples

```bash
# Create a subtask
./jira4claude create --parent J4C-10 -s "Implement validation"

# List subtasks of an issue
./jira4claude list --parent J4C-10

# View a subtask (shows parent)
./jira4claude view J4C-11
# Output includes: Parent: J4C-10
```

## Implementation Tasks

1. Add Parent field to Issue domain type
2. Add Parent field to IssueFilter
3. Update CreateCmd with --parent flag and auto-type logic
4. Update ListCmd with --parent filter
5. Update http Create to send parent field
6. Update http parseIssueResponse to extract parent
7. Update buildJQL to handle parent filter
8. Update display functions for parent output
9. Add tests for subtask creation and listing
