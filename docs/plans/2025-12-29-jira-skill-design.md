# Jira Workflow Skill - Design Document

## Overview

A temporary, project-specific skill for managing J4C tasks via Jira API until the jira4claude CLI is built.

## Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Scope | Project-specific (J4C) | Stopgap until CLI exists, keep simple |
| Dependencies | Issue Links (blocks/is blocked by) | Queryable, flat structure fits small tasks |
| Commands | 7 (list, ready, show, create, link, transition, comment) | Minimal complete workflow |
| Location | `.claude/skills/jira-workflow/` | Versioned with project, easy to delete |
| Curl approach | Inline | YAGNI - delete entire skill when CLI works |
| Templates | Reference section | Valuable docs without added complexity |

## Skill Structure

```
.claude/skills/jira-workflow/
└── SKILL.md
```

Single file containing:
1. Metadata and overview
2. All 7 commands with curl examples
3. Task creation templates

## Commands

### List Open Tasks

```bash
curl -s -n 'https://fwojciec.atlassian.net/rest/api/3/search?jql=project=J4C+AND+status!=Done&fields=key,summary,status' | jq '.issues[] | {key, summary: .fields.summary, status: .fields.status.name}'
```

### Show Unblocked Tasks (Ready)

```bash
# Two-step approach if issueFunction not available:
# 1. Get open tasks
# 2. For each, check if it has unresolved "is blocked by" links
curl -s -n 'https://fwojciec.atlassian.net/rest/api/3/search?jql=project=J4C+AND+status!=Done&fields=key,summary,status,issuelinks'
# Filter client-side for tasks with no unresolved inward "Blocks" links
```

### Show Task Details

```bash
curl -s -n 'https://fwojciec.atlassian.net/rest/api/3/issue/J4C-123' | jq '{key, summary: .fields.summary, status: .fields.status.name, description: .fields.description}'
```

### Create Task

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

### Link Tasks (Blocks Relationship)

```bash
curl -s -n -X POST -H "Content-Type: application/json" \
  'https://fwojciec.atlassian.net/rest/api/3/issueLink' \
  -d '{
    "type": {"name": "Blocks"},
    "inwardIssue": {"key": "J4C-2"},
    "outwardIssue": {"key": "J4C-1"}
  }'
# J4C-1 blocks J4C-2 (J4C-2 is blocked by J4C-1)
```

### Transition Task

```bash
# Get available transitions
curl -s -n 'https://fwojciec.atlassian.net/rest/api/3/issue/J4C-123/transitions' | jq '.transitions[] | {id, name}'

# Execute transition
curl -s -n -X POST -H "Content-Type: application/json" \
  'https://fwojciec.atlassian.net/rest/api/3/issue/J4C-123/transitions' \
  -d '{"transition": {"id": "TRANSITION_ID"}}'
```

### Add Comment

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

## CLAUDE.md Integration

Add to Workflows section:

```markdown
**Planning workflow** (mandatory for new work):
1. Research the problem
2. Use `/brainstorm` to refine into design
3. Write design doc to `docs/plans/`
4. Use `jira-workflow` skill to create tasks with dependencies
5. Use `ready` command to find unblocked work
```

Add skill reference:

```markdown
### Task Management

**`jira-workflow`** - Use when:
- Creating new tasks or subtasks
- Checking what work is ready (unblocked)
- Linking tasks with dependencies
- Transitioning task status
```

## Implementation Steps

1. Create `.claude/skills/jira-workflow/SKILL.md` with commands and templates
2. Test all 7 commands work correctly
3. Update CLAUDE.md with planning workflow and skill reference
4. Remove obsolete Jira references from existing commands (if any duplication)
