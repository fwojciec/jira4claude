---
name: jira-workflow
description: Manage J4C project tasks via Jira API. Use for ALL Jira project management tasks including creating tasks, checking ready work, linking dependencies, transitioning status, or adding comments.
---

# Jira Workflow Skill

Project-specific skill for managing J4C tasks using jira4claude CLI.

**This skill MUST be used for ANY Jira project management work.**

## Handling Missing Config

If you see: `Error: no config file found`

Create a local config:

```bash
./jira4claude init --server=https://fwojciec.atlassian.net --project=J4C
```

This creates `.jira4claude.yaml` and adds it to `.gitignore`.

## MANDATORY: Issue Creation Template

**CRITICAL: ALL issues MUST use this template. Do not create issues without following this structure.**

```markdown
## Context

[What needs to be built and why - 1-3 sentences. No implementation details here.]

## Investigation Starting Points

- Examine [file/class] to understand existing patterns
- Review [reference] for similar functionality

## Scope Constraints

- Implement only what is specified
- Do not add [specific exclusions]
- [Other constraints]

## Validation Requirements

### Behavioral

- [Specific observable behavior to verify]
- [Another testable requirement]

### Quality

- All tests pass
- No linting errors
- Follows patterns in [reference file]
```

**Template Rules:**
1. Context explains WHAT and WHY, never HOW
2. Investigation points help discovery - reference specific files
3. Scope constraints prevent over-engineering
4. Validation requirements must be testable/observable

## Text Formatting in Descriptions

**Note:** The template above uses markdown (##, -, etc.) to show the STRUCTURE Claude should follow. However, Jira descriptions are plain text - markdown won't render. Use the template headings as plain text section headers.

The CLI automatically converts plain text to Atlassian Document Format (ADF). Use these patterns:

| Format | Input | Result |
|--------|-------|--------|
| Paragraphs | Double newlines (`\n\n`) | Separate paragraphs |
| Line breaks | Single newlines (`\n`) | Line break within paragraph |
| Plain text | Regular text | Preserved as-is |

**Example:**
```
First paragraph here.

Second paragraph here.
With a line break.
```

**Currently NOT supported in Jira:** Bold, italic, code blocks, lists, links, mentions. Use plain text only.

## Commands

### List Open Tasks

Show all tasks not marked Done:

```bash
./jira4claude list --jql="status != Done"
```

For JSON output, add `--json`:

```bash
./jira4claude list --jql="status != Done" --json
```

### Show Ready Tasks (Unblocked)

Find tasks with no unresolved blockers:

```bash
./jira4claude ready
```

For JSON output:

```bash
./jira4claude ready --json
```

This shows tasks where all blockers are Done (or have no blockers).

### Show Task Details

Get full details for a specific task:

```bash
./jira4claude view J4C-123
```

For JSON output:

```bash
./jira4claude view J4C-123 --json
```

### Create Task

Create a new task with description:

```bash
./jira4claude create \
  --summary="Task title here" \
  --description="Description here"
```

For JSON output (returns the created issue key):

```bash
./jira4claude create \
  --summary="Task title here" \
  --description="Description here" \
  --json
```

Multi-line descriptions are supported - newlines are preserved.

### Link Tasks (Blocks Relationship)

**CRITICAL: Get the direction right or the dependency graph will be wrong!**

#### Understanding the Link Model

The CLI uses the same semantic as the Jira API: `[inward-key] blocks [outward-key]`

```
inward-key  ──blocks──>  outward-key
(BLOCKER)                (BLOCKED)
(do first)               (do after)
```

When you view an issue's links:
- `inwardIssue` in a Blocks link → that issue is blocking THIS one
- `outwardIssue` in a Blocks link → THIS issue is blocking that one

#### Example

**Goal:** J4C-7 (error handling) must be done before J4C-8 (config loading)

This means: "J4C-7 blocks J4C-8" or "J4C-8 is blocked by J4C-7"

```bash
./jira4claude link J4C-7 Blocks J4C-8
```

#### Verification

Always verify after creating links:

```bash
./jira4claude view J4C-8 --json | jq '.links[] | select(.inwardIssue) | {blockedBy: .inwardIssue.key}'
# Should show: "blockedBy": "J4C-7"

./jira4claude view J4C-7 --json | jq '.links[] | select(.outwardIssue) | {blocks: .outwardIssue.key}'
# Should show: "blocks": "J4C-8"
```

#### Quick Reference

| You want | Command |
|----------|---------|
| A blocks B | `./jira4claude link A Blocks B` |
| B depends on A | `./jira4claude link A Blocks B` |

### Delete Link

If you created a link with wrong direction, delete and recreate:

```bash
./jira4claude unlink J4C-7 J4C-8
```

This removes any link between the two issues (regardless of direction).

### Transition Task

List available transitions for a task:

```bash
./jira4claude transition J4C-123 --list-only
```

Execute a transition by status name:

```bash
./jira4claude transition J4C-123 --status="Done"
```

Or by transition ID:

```bash
./jira4claude transition J4C-123 --transition-id="21"
```

Common transitions (may vary by workflow):
- "Start Progress" (To Do -> In Progress)
- "Done" (In Progress -> Done)

### Add Comment

Add a comment to a task:

```bash
./jira4claude comment J4C-123 --body="Comment text here"
```

For JSON output:

```bash
./jira4claude comment J4C-123 --body="Comment text here" --json
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

## Notes

- **CLI auto-discovers config**: searches `./.jira4claude.yaml` then `~/.jira4claude.yaml`
- **CLI credentials**: reads from `.netrc`
- Add `--json` to any CLI command for JSON output
- The CLI handles Atlassian Document Format (ADF) conversion automatically
