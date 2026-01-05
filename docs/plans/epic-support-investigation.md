# Epic Support Investigation (J4C-95)

## Summary

This document captures findings from investigating how Jira represents epic relationships in the REST API and the finalized design for a unified Related Issues display.

## Key Findings

### 1. The `parent` Field is Unified

Atlassian has standardized parent-child relationships through a single `parent` field that works for both:
- **Subtasks** → parent is a regular issue (hierarchyLevel: 0)
- **Stories/Tasks** → parent is an epic (hierarchyLevel: 1)

The old custom fields (`Epic Link`, `Parent Link`) are deprecated.

### 2. Parent Field Structure

When an issue has a parent, the API returns:

```json
{
  "fields": {
    "parent": {
      "id": "10293",
      "key": "J4C-96",
      "self": "https://example.atlassian.net/rest/api/3/issue/10293",
      "fields": {
        "summary": "Parent issue summary",
        "status": {
          "name": "In Progress",
          "statusCategory": { "name": "In Progress", "colorName": "blue" }
        },
        "priority": { "name": "Medium" },
        "issuetype": {
          "name": "Task",
          "hierarchyLevel": 0
        }
      }
    }
  }
}
```

### 3. Hierarchy Levels

The `issuetype.hierarchyLevel` field distinguishes relationship types:

| Level | Type | Example |
|-------|------|---------|
| 1 | Epic | Epic issues |
| 0 | Standard | Task, Story, Bug |
| -1 | Subtask | Sub-task |

### 4. Epic Custom Fields (Legacy)

These fields exist but are deprecated:

| Field ID | Name | Purpose |
|----------|------|---------|
| customfield_10014 | Epic Link | Links issue to epic (deprecated) |
| customfield_10011 | Epic Name | Epic's name field |
| customfield_10018 | Parent Link | Alternative parent link (deprecated) |

**Recommendation**: Do not use these. The `parent` field is the forward-compatible approach.

## Finalized Design: Unified Related Issues

After brainstorming, we decided on a unified approach rather than separate display sections.

### Design Principles

1. **Generic approach** - One "Related Issues" section instead of separate Parent/Subtasks/Links sections
2. **Mechanical and predictable** - Always show type in parentheses for AI agent consumption
3. **No noise when empty** - Section omitted if no relationships exist
4. **Unify at view level only** - Domain model keeps separate fields to mirror Jira API

### Display Format

```markdown
## Related Issues

**parent:**
- **J4C-96** [Done] (Epic) Epic summary here

**child:**
- **J4C-100** [To Do] (Sub-task) First subtask
- **J4C-101** [To Do] (Task) Task under epic

**blocks:**
- **J4C-102** [To Do] (Task) Downstream work

**is blocked by:**
- **J4C-98** [Done] (Task) Prerequisite work
```

**Ordering:** parent → child → blocks → is blocked by (fixed order)

### Data Model Changes

**Domain model (`issue.go`):**
```go
type Issue struct {
    Parent   *LinkedIssue   // was: string
    Subtasks []*LinkedIssue // unchanged
    Children []*LinkedIssue // NEW: for epics
    Links    []*IssueLink   // unchanged
}
```

**View model (`view.go`):**
```go
type IssueView struct {
    RelatedIssues []RelatedIssueView `json:"relatedIssues,omitempty"`
    // Remove: Links, Subtasks, Parent (now in RelatedIssues)
}

type RelatedIssueView struct {
    Relationship string `json:"relationship"` // "parent", "child", "blocks", "is blocked by"
    Key          string `json:"key"`
    Type         string `json:"type"`         // "Epic", "Task", "Sub-task"
    Status       string `json:"status"`
    Summary      string `json:"summary"`
}
```

### Epic Children Fetching

When viewing an epic, fetch children via JQL in `IssueService.Get()`:

```go
if issue.Type == "Epic" {
    children, _ := s.List(ctx, jira4claude.IssueFilter{
        JQL: fmt.Sprintf("parent = %q", issue.Key),
    })
    issue.Children = toLinkedIssues(children)
}
```

## Implementation Tasks

Epic: **J4C-114** - Unified Related Issues Display

| Key | Summary | Blocked By |
|-----|---------|------------|
| J4C-115 | Expand Parent field in domain model | - |
| J4C-116 | Add Children field for epics | J4C-115 |
| J4C-117 | Create unified RelatedIssueView | J4C-116 |
| J4C-118 | Update printer for unified display | J4C-117 |
| J4C-119 | Update JSON output schema | J4C-117 |

## Sources

- [Atlassian: Deprecation of Epic Link and Parent Link](https://community.developer.atlassian.com/t/deprecation-of-the-epic-link-parent-link-and-other-related-fields-in-rest-apis-and-webhooks/54048)
- [Atlassian: Epic and Parent Fields API Deprecation](https://docs.adaptavist.com/sr4jc/latest/release-notes/breaking-changes/atlassian-epic-and-parent-fields-jira-rest-api-deprecation)
