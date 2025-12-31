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

#### The Golden Rule

```
./jira4claude link FIRST Blocks SECOND
```

- **FIRST** = the blocker (do this first, shows in `ready`)
- **SECOND** = the blocked (do this after, NOT in `ready` until FIRST is Done)

**Memory aid:** Read it as a sentence: "FIRST blocks SECOND" or "FIRST must be done before SECOND"

#### Example

**Goal:** J4C-7 (error handling) must be done before J4C-8 (config loading)

```bash
./jira4claude link J4C-7 Blocks J4C-8
```

**After running this command:**

```bash
./jira4claude view J4C-7
# Shows: "blocks J4C-8"

./jira4claude view J4C-8
# Shows: "is blocked by J4C-7"

./jira4claude ready
# Shows J4C-7 (the blocker is ready to work on)
# Does NOT show J4C-8 (blocked until J4C-7 is Done)
```

#### MANDATORY Verification

**Always verify links using the `ready` command:**

```bash
./jira4claude ready
```

Ask yourself:
- Does the blocker (prerequisite) appear in the ready list? It should.
- Does the blocked (dependent) appear in the ready list? It should NOT (unless its blocker is Done).

If the wrong task is blocked, you got the direction backwards. Delete and recreate.

#### Common Mistake

**Wrong:** You want A done before B, but you run `link B Blocks A`
- Result: B appears blocked, A appears ready - the opposite of what you wanted!

**Fix:** Always read the command as a sentence. "A blocks B" means A is the prerequisite.

#### Quick Reference

| You want | Command | Ready shows |
|----------|---------|-------------|
| A before B | `link A Blocks B` | A (not B) |
| B depends on A | `link A Blocks B` | A (not B) |

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
