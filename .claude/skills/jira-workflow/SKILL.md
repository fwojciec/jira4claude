---
name: jira-workflow
description: Manage J4C project tasks via Jira API. Use for ALL Jira project management tasks including creating tasks, checking ready work, linking dependencies, transitioning status, or adding comments.
---

# Jira Workflow Skill

Project-specific skill for managing J4C tasks using jira4claude CLI.

**This skill MUST be used for ANY Jira project management work.**

**Note:** The CLI requires `--config=.jira4claude.yaml` when running from the project directory.

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
./jira4claude --config=.jira4claude.yaml list --jql="status != Done"
```

For JSON output, add `--json`:

```bash
./jira4claude --config=.jira4claude.yaml list --jql="status != Done" --json
```

### Show Ready Tasks (Unblocked)

Find tasks with no unresolved blockers. This requires curl (issue links not yet in CLI):

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

This filters for tasks where all blockers (`inwardIssue` in "Blocks" links) are Done (or have no blockers).

### Show Task Details

Get full details for a specific task:

```bash
./jira4claude --config=.jira4claude.yaml view J4C-123
```

For JSON output:

```bash
./jira4claude --config=.jira4claude.yaml view J4C-123 --json
```

### Create Task

Create a new task with description:

```bash
./jira4claude --config=.jira4claude.yaml create \
  --summary="Task title here" \
  --description="Description here"
```

For JSON output (returns the created issue key):

```bash
./jira4claude --config=.jira4claude.yaml create \
  --summary="Task title here" \
  --description="Description here" \
  --json
```

Multi-line descriptions are supported - newlines are preserved.

### Link Tasks (Blocks Relationship)

**CRITICAL: Get the direction right or the dependency graph will be wrong!**

#### Understanding the Jira Link Model

Think of it as a sentence: `[inwardIssue] blocks [outwardIssue]`

```
inwardIssue  ──blocks──>  outwardIssue
(BLOCKER)                 (BLOCKED)
(do first)                (do after)
```

When you view an issue's links:
- If it has an `inwardIssue` in a Blocks link → that issue is blocking THIS one
- If it has an `outwardIssue` in a Blocks link → THIS issue is blocking that one

#### Example

**Goal:** J4C-7 (error handling) must be done before J4C-8 (config loading)

This means: "J4C-7 blocks J4C-8" or "J4C-8 is blocked by J4C-7"

```bash
curl -s -n -X POST -H "Content-Type: application/json" \
  'https://fwojciec.atlassian.net/rest/api/3/issueLink' \
  -d '{
    "type": {"name": "Blocks"},
    "inwardIssue": {"key": "J4C-7"},
    "outwardIssue": {"key": "J4C-8"}
  }'
```

#### Verification

Always verify after creating links:

```bash
# Check J4C-8 (the blocked issue)
curl -s -n 'https://fwojciec.atlassian.net/rest/api/3/issue/J4C-8?fields=issuelinks' | \
  jq '.fields.issuelinks[] | {blockedBy: .inwardIssue.key}'
# Should show: "blockedBy": "J4C-7"

# Check J4C-7 (the blocker)
curl -s -n 'https://fwojciec.atlassian.net/rest/api/3/issue/J4C-7?fields=issuelinks' | \
  jq '.fields.issuelinks[] | {blocks: .outwardIssue.key}'
# Should show: "blocks": "J4C-8"
```

#### Quick Reference

| You want | inwardIssue | outwardIssue |
|----------|-------------|--------------|
| A blocks B | A (blocker) | B (blocked) |
| B depends on A | A (blocker) | B (blocked) |

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

List available transitions for a task:

```bash
./jira4claude --config=.jira4claude.yaml transition J4C-123 --list-only
```

Execute a transition by status name:

```bash
./jira4claude --config=.jira4claude.yaml transition J4C-123 --status="Done"
```

Or by transition ID:

```bash
./jira4claude --config=.jira4claude.yaml transition J4C-123 --transition-id="21"
```

Common transitions (may vary by workflow):
- "Start Progress" (To Do -> In Progress)
- "Done" (In Progress -> Done)

### Add Comment

Add a comment to a task:

```bash
./jira4claude --config=.jira4claude.yaml comment J4C-123 --body="Comment text here"
```

For JSON output:

```bash
./jira4claude --config=.jira4claude.yaml comment J4C-123 --body="Comment text here" --json
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

- **CLI commands** use `./jira4claude --config=.jira4claude.yaml` - the CLI reads credentials from `.netrc`
- **curl commands** (for linking) use `-n` flag for `.netrc` authentication
- Add `--json` to any CLI command for JSON output
- The CLI handles Atlassian Document Format (ADF) conversion automatically

### Missing CLI Features

These operations still require curl (tracked for future implementation):
- Issue linking (Blocks relationships)
- Ready task filtering (requires issue links)
