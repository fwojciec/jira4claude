---
name: jira-workflow
description: Manage J4C project tasks via Jira API. Use when creating tasks, checking ready work, linking dependencies, or transitioning status.
---

# Jira Workflow Skill

Temporary, project-specific skill for managing J4C tasks via Jira API until jira4claude CLI is built.

## Commands

### List Open Tasks

Show all tasks not marked Done:

```bash
curl -s -n 'https://fwojciec.atlassian.net/rest/api/3/search/jql?jql=project%3DJ4C%20AND%20status%20NOT%20IN%20(Done)&fields=key,summary,status' | jq '.issues[] | {key, summary: .fields.summary, status: .fields.status.name}'
```

### Show Ready Tasks (Unblocked)

Find tasks with no unresolved blockers. Save the jq filter to a file first (shell escaping issues with `!=`):

```bash
cat > /tmp/ready_filter.jq << 'EOF'
.issues[] |
select(
  [.fields.issuelinks[]? |
   select(.type.name == "Blocks" and .outwardIssue != null) |
   select(.outwardIssue.fields.status.statusCategory.key != "done")
  ] | length == 0
) |
{key, summary: .fields.summary, status: .fields.status.name}
EOF

curl -s -n 'https://fwojciec.atlassian.net/rest/api/3/search/jql?jql=project%3DJ4C%20AND%20status%20NOT%20IN%20(Done)&fields=key,summary,status,issuelinks' | jq -f /tmp/ready_filter.jq
```

This filters for tasks where all blockers (outwardIssue in "Blocks" links) are Done (or have no blockers).

### Show Task Details

Get full details for a specific task:

```bash
curl -s -n 'https://fwojciec.atlassian.net/rest/api/3/issue/J4C-123' | jq '{key, summary: .fields.summary, status: .fields.status.name, description: .fields.description}'
```

### Create Task

Create a new task with description:

```bash
curl -s -n -X POST -H "Content-Type: application/json" \
  'https://fwojciec.atlassian.net/rest/api/3/issue' \
  -d '{
    "fields": {
      "project": {"key": "J4C"},
      "summary": "Task title here",
      "issuetype": {"name": "Task"},
      "description": {
        "type": "doc",
        "version": 1,
        "content": [{"type": "paragraph", "content": [{"type": "text", "text": "Description here"}]}]
      }
    }
  }'
```

For multi-paragraph descriptions, add multiple paragraph blocks to the content array.

### Link Tasks (Blocks Relationship)

**CRITICAL: Get the direction right or the dependency graph will be wrong!**

The Jira API uses confusing terminology. Here's what the fields mean:

| Field | Meaning | Role |
|-------|---------|------|
| `outwardIssue` | The **BLOCKER** | Must be done FIRST |
| `inwardIssue` | The **BLOCKED** | Cannot start until blocker is done |

**Example:** If J4C-7 (error handling) must be done before J4C-8 (config loading):

```bash
curl -s -n -X POST -H "Content-Type: application/json" \
  'https://fwojciec.atlassian.net/rest/api/3/issueLink' \
  -d '{
    "type": {"name": "Blocks"},
    "outwardIssue": {"key": "J4C-7"},
    "inwardIssue": {"key": "J4C-8"}
  }'
```

This creates: **J4C-7 blocks J4C-8** (J4C-8 depends on J4C-7).

**Verification:** After creating links, always verify with:

```bash
curl -s -n 'https://fwojciec.atlassian.net/rest/api/3/issue/J4C-8?fields=issuelinks' | \
  jq '.fields.issuelinks[] | {type: .type.name, blocks: .outwardIssue.key, blockedBy: .inwardIssue.key}'
```

You should see `"blockedBy": "J4C-7"` for J4C-8.

### Delete Link

If you created a link with wrong direction, delete and recreate:

```bash
# First find the link ID
curl -s -n 'https://fwojciec.atlassian.net/rest/api/3/issue/J4C-8?fields=issuelinks' | \
  jq '.fields.issuelinks[] | {id, type: .type.name, outward: .outwardIssue.key, inward: .inwardIssue.key}'

# Then delete by ID
curl -s -n -X DELETE 'https://fwojciec.atlassian.net/rest/api/3/issueLink/LINK_ID'
```

### Transition Task

First, get available transitions for the task:

```bash
curl -s -n 'https://fwojciec.atlassian.net/rest/api/3/issue/J4C-123/transitions' | jq '.transitions[] | {id, name}'
```

Then execute the transition:

```bash
curl -s -n -X POST -H "Content-Type: application/json" \
  'https://fwojciec.atlassian.net/rest/api/3/issue/J4C-123/transitions' \
  -d '{"transition": {"id": "TRANSITION_ID"}}'
```

Common transition IDs (may vary by workflow):
- `11` - Start Progress (To Do -> In Progress)
- `21` - Done (In Progress -> Done)

### Add Comment

Add a comment to a task:

```bash
curl -s -n -X POST -H "Content-Type: application/json" \
  'https://fwojciec.atlassian.net/rest/api/3/issue/J4C-123/comment' \
  -d '{
    "body": {
      "type": "doc",
      "version": 1,
      "content": [{"type": "paragraph", "content": [{"type": "text", "text": "Comment text here"}]}]
    }
  }'
```

## Planning Dependencies

Before creating tasks with dependencies, draw the dependency graph first:

```
BLOCKER → BLOCKED (arrow points to what depends on it)

Example for jira4claude:
  J4C-6 (domain types) ──→ J4C-13 (mocks)
  J4C-7 (error handling) ──→ J4C-8 (config)
  J4C-9 (HTTP client) ──→ J4C-11 (IssueService CRUD)
  J4C-10 (ADF helper) ──→ J4C-11
  J4C-11 ──→ J4C-12 (other ops)
  J4C-11, J4C-12, J4C-13 ──→ J4C-14 (CLI)
```

**Rules:**
1. Foundation tasks (no dependencies) should be done first
2. Only link immediate dependencies, not transitive ones
3. After creating links, run "Show Ready Tasks" to verify correct tasks are unblocked

## Task Templates

### Issue Creation Template

Use this structure when creating new issues:

```markdown
## Title
[Imperative verb phrase - e.g., "Implement issue list command"]

## Context
[What needs to be built and why - no implementation details]

## Investigation Starting Points
- Examine [file/class] to understand existing patterns
- Review [reference] for similar functionality

## Scope Constraints
- Implement only what is specified
- Do not add [specific exclusions]
- Maintain modesty - accomplish only what is specified

## Validation Requirements

### Behavioral
- [Specific observable behavior to verify]

### Quality
- All tests pass
- No linting errors
- Follows patterns in [reference]
```

### Subtask Template

For tasks that are part of a larger effort:

```markdown
## Context
Part of J4C-X: [Parent summary]
Depends on: J4C-Y (if applicable)

[What this piece accomplishes - 1-2 sentences]

## Scope
- [What THIS task includes]
- [What THIS task excludes]

## Validation
- [Specific test command]
- [Observable behavior]
```

## Notes

- All commands use `-n` flag for `.netrc` authentication
- Responses are piped through `jq` for readable JSON output
- The Atlassian Document Format (ADF) is required for description and comment bodies
