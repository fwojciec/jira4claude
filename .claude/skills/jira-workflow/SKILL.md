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
   select(.type.name == "Blocks" and .inwardIssue != null) |
   select(.inwardIssue.fields.status.statusCategory.key != "done")
  ] | length == 0
) |
{key, summary: .fields.summary, status: .fields.status.name}
EOF

curl -s -n 'https://fwojciec.atlassian.net/rest/api/3/search/jql?jql=project%3DJ4C%20AND%20status%20NOT%20IN%20(Done)&fields=key,summary,status,issuelinks' | jq -f /tmp/ready_filter.jq
```

This filters for tasks where all "is blocked by" links point to Done issues (or have no blockers).

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

Create a "blocks" dependency between tasks:

```bash
curl -s -n -X POST -H "Content-Type: application/json" \
  'https://fwojciec.atlassian.net/rest/api/3/issueLink' \
  -d '{
    "type": {"name": "Blocks"},
    "inwardIssue": {"key": "J4C-2"},
    "outwardIssue": {"key": "J4C-1"}
  }'
```

This creates: J4C-1 blocks J4C-2 (J4C-2 is blocked by J4C-1).

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
