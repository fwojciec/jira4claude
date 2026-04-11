---
name: gh-workflow
description: Manage project work via GitHub CLI. Use for ALL GitHub tasks including creating/viewing issues, organizing with milestones, establishing blocked-by/blocking dependencies, and responding to PR comments.
---

# GitHub Workflow Skill

Skill for managing project tasks using the `gh` CLI.

**This skill MUST be used for ANY GitHub project management work.**

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

## Core Issue Commands

```bash
# List issues
gh issue list
gh issue list --state open --json number,title,labels --limit 20
gh issue list --milestone "v1.0"

# View issue (always read comments for context from related work)
gh issue view <number>
gh issue view <number> --comments

# Create issue
gh issue create --title "..." --body "..." --label "..." --milestone "..."

# Edit issue
gh issue edit <number> --add-label "..." --milestone "..."

# Close issue
gh issue close <number>

# Add comment
gh issue comment <number> --body "Comment text here"
```

## Issue Dependencies (via REST API)

GitHub has native blocked-by/blocking support. The `gh` CLI doesn't have
direct flags, so use `gh api`. The dependency API is the **single source of
truth** for dependency resolution.

### Creating a Dependency

**CRITICAL: The API requires the numeric REST `id`, NOT the issue number
and NOT the GraphQL `node_id` string.**

```bash
# Step 1: Get the blocker's REST id (numeric integer)
BLOCKER_ID=$(gh api repos/{owner}/{repo}/issues/<blocker-number> --jq '.id')

# Step 2: Mark the blocked issue as blocked by the blocker
gh api repos/{owner}/{repo}/issues/<blocked-number>/dependencies/blocked_by \
  -f issue_id="$BLOCKER_ID"
```

**Memory aid:** Read it as: "blocked-number is blocked by blocker-number"

### Querying Dependencies

```bash
# List what blocks an issue (its blockers)
gh api repos/{owner}/{repo}/issues/<number>/dependencies/blocked_by

# List what an issue blocks (its downstream dependents)
gh api repos/{owner}/{repo}/issues/<number>/dependencies/blocking

# Count OPEN blockers (closed blockers still have dependency records)
gh api repos/{owner}/{repo}/issues/<number>/dependencies/blocked_by \
  --jq 'map(select(.state == "open")) | length'
```

### Removing a Dependency

```bash
gh api -X DELETE repos/{owner}/{repo}/issues/<blocked-number>/dependencies/blocked_by/<blocker-REST-id>
```

### Finding Ready Work (Unblocked Issues)

An issue is "ready" when it has zero **open** blockers. Dependency records
persist after a blocker is closed, so always filter by state:

```bash
# List open issues, then for each check open blockers
gh issue list --state open --json number,title,labels --limit 50

# For each issue, check if it has open blockers
gh api repos/{owner}/{repo}/issues/<number>/dependencies/blocked_by \
  --jq 'map(select(.state == "open")) | length'
# Ready if result is 0
```

### MANDATORY Verification

**Always verify dependencies after creating them:**

Check that the blocker appears as ready (0 open blockers) and the blocked
issue does NOT appear as ready (has open blockers).

### Common Mistake

**Wrong:** You want A done before B, but you add A's id to A's own blocked_by.
**Right:** Add A's REST id to B's blocked_by endpoint.

### Quick Reference

| You want | Command |
|----------|---------|
| A before B | Add A's REST id to B's `blocked_by` |
| B depends on A | Add A's REST id to B's `blocked_by` |

## Organization with Milestones

```bash
# List milestones
gh api repos/{owner}/{repo}/milestones

# Create milestone
gh api repos/{owner}/{repo}/milestones -f title="v1.0" -f description="..."

# Create issue in milestone
gh issue create --title "..." --body "..." --milestone "v1.0"

# List issues in milestone
gh issue list --milestone "v1.0" --state open
```

## PR Comment Commands

```bash
# Add general comment to PR
gh pr comment <number> --body "..."

# View PR with comments
gh pr view <number> --comments

# Reply to a specific inline review comment
gh api repos/{owner}/{repo}/pulls/<pr>/comments/<comment-id>/replies \
  -f body="..."

# List review comments to find comment IDs
gh api repos/{owner}/{repo}/pulls/<pr>/comments

# Reply to a general PR comment (issue-style comment)
gh api repos/{owner}/{repo}/issues/<pr>/comments -f body="..."
```

**Critical syntax rules for `gh api`:**
- `{owner}/{repo}` is a **literal placeholder** — type it exactly as shown,
  `gh api` auto-substitutes it with the current repo
- Do NOT replace `{owner}/{repo}` with the actual repo name
- DO replace `<pr>`, `<comment-id>`, etc. with actual numeric values

## Formatting

**Always use GitHub-flavored markdown (GFM)** for issue bodies and comments.

| Markdown | Result |
|----------|--------|
| `## Heading` | Heading level 2 |
| `### Heading` | Heading level 3 |
| `- item` | Bullet list |
| `1. item` | Numbered list |
| `**bold**` | Bold text |
| `*italic*` | Italic text |
| `` `code` `` | Inline code |
| ` ``` ` blocks | Code blocks |
| `[text](url)` | Links |
| `- [ ] item` | Task list (unchecked) |
| `- [x] item` | Task list (checked) |

## Planning Dependencies

Before creating issues with dependencies, draw the dependency graph first:

```
BLOCKER -> BLOCKED (arrow points to what depends on it)

Example:
  #6 (domain types) --> #13 (mocks)
  #7 (error handling) --> #8 (config)
  #9 (HTTP client) --> #11 (IssueService CRUD)
```

**Rules:**
1. Foundation tasks (no dependencies) should be done first
2. Only link immediate dependencies, not transitive ones
3. After creating dependencies, verify with the ready-work check
