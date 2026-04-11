---
description: Execute one iteration of the Ralph loop - pick next ready issue from milestone, implement, commit
allowed-tools: Bash(gh:*), Bash(git:*), Bash(make:*), Bash(jq:*), Bash(node:*), Bash(codex:*), Bash(touch:*), Bash(rm:*)
argument-hint: "<milestone-name>"
---

# Ralph Iterate

Execute one task from a GitHub milestone. Designed for autonomous batch processing.

**Design principle**: Run to completion without user interaction. Exit cleanly so the loop can restart.

## Arguments

Milestone name: $ARGUMENTS

## Current State

Branch: !`git branch --show-current`
Uncommitted changes: !`git status --porcelain`

---

## Phase 1: Find Ready Task

### 1.1 Get Open Issues in Milestone

```bash
gh issue list --milestone "$ARGUMENTS" --state open --json number,title
```

If no open issues, create `.ralph-complete` and exit immediately:

```bash
touch .ralph-complete
echo "Milestone complete - no open issues"
```

### 1.2 Find First Ready Issue

For each open issue (by number, ascending), check for open blockers using the
dependency API (same approach as the gh-workflow skill):

```bash
# For each issue number:
gh api repos/{owner}/{repo}/issues/<number>/dependencies/blocked_by \
  --jq 'map(select(.state == "open")) | length'
```

An issue is **ready** if the result is `0` (zero open blockers).

Pick the first ready issue found (ascending by number).

**If no ready issues found** (all blocked): Create `.ralph-complete` and exit.

```bash
touch .ralph-complete
echo "All remaining issues are blocked by dependencies"
```

### 1.3 Claim the Issue

Once a ready issue is found:

```bash
# Show full issue with comments for context
gh issue view <number> --comments
```

Read any referenced files or design docs mentioned in the issue.

---

## Phase 2: Setup Branch

### 2.1 Ensure Clean Main

If not on `main` with clean working tree:

```bash
git checkout main
git pull origin main
```

If there are uncommitted changes, stash them:
```bash
git stash push -m "ralph-stash-$(date +%s)"
```

### 2.2 Create Feature Branch

```bash
git checkout -b <number>-<short-kebab-description>
```

Example: `42-add-user-authentication`

---

## Phase 3: Implement

### 3.1 TDD Workflow

**Invoke `/test-driven-development` skill** for all behavioral code.

Follow RED-GREEN-REFACTOR:
1. Write a failing test first
2. Implement minimal code to pass
3. Refactor if needed
4. Repeat

**TDD applies to:**
- Functions with logic or side effects
- Modules with behavior to verify
- Integration points
- Error handling paths

**TDD does NOT apply to:**
- Type definitions and interfaces
- Data structures without behavior
- Pure configuration or constants
- Boilerplate wiring code

For non-behavioral work, implement directly without forcing artificial tests.

### 3.2 Validate Frequently

```bash
make validate
```

Fix issues before proceeding.

### 3.3 Documentation Updates

Check issue validation criteria - some may require updating:
- `CLAUDE.md` files
- Doc comments on public APIs

---

## Phase 4: Review

### 4.1 Pre-Review Validation

```bash
make validate
```

Must pass before review.

### 4.2 Stage Changes

```bash
git add -A
```

### 4.3 Parallel Code Reviews

Launch TWO review tracks in parallel. Use a single message with multiple
tool calls for actual parallelism.

**Track A -- Claude code-review subagent:**

Launch as a Task subagent with `subagent_type="superpowers:code-reviewer"`:

Prompt: Review the current branch changes vs main for structural discipline
and project standards. Read CLAUDE.md for codebase-specific rules. Return
APPROVE/REJECT verdict with specific feedback.

**Track B -- Codex review:**

First check if Codex is available:
```bash
command -v codex >/dev/null 2>&1 && echo "available" || echo "unavailable"
```

If unavailable, skip this track. Do not block on Codex.

If available, run via `codex exec`:

```bash
ISSUE_NUM="<issue-number>"
ISSUE_CONTEXT=$(gh issue view "$ISSUE_NUM" --json number,title,body \
  --jq '"Issue #\(.number): \(.title)\n\n\(.body)"' 2>/dev/null || echo "No issue context")

STAGED=$(git diff --cached --stat 2>/dev/null)
COMMITTED=$(git diff main...HEAD --stat 2>/dev/null)

if [ -n "$STAGED" ]; then
  DIFF_CMD="git diff --cached"
  STAT_CMD="git diff --cached --stat"
elif [ -n "$COMMITTED" ]; then
  DIFF_CMD="git diff main...HEAD"
  STAT_CMD="git diff main...HEAD --stat"
else
  echo "No changes to review"
  exit 0
fi

codex exec \
  --dangerously-bypass-approvals-and-sandbox \
  -o /tmp/codex-review-result.txt \
  "# Code Review Request

You are reviewing changes on the current branch.

## Issue Context
$ISSUE_CONTEXT

## Your Task
1. Run \`$STAT_CMD\` to see what changed
2. Run \`$DIFF_CMD\` to see the actual diff
3. Read \`CLAUDE.md\` for project standards
4. Read changed files that need closer inspection
5. Run tests: \`make validate\`
6. If an issue was provided, evaluate whether the implementation satisfies the validation criteria
7. Provide your review

Be direct. Reference file paths and line numbers. Focus on actionable feedback."

cat /tmp/codex-review-result.txt
```

**Note**: If Codex review fails (network issues, rate limits), proceed with the code review verdict alone.

### 4.4 Handle Review Results

Collect verdicts from reviewers.

**If ALL APPROVE:**
- Apply any reasonable suggestions
- Proceed to reflect phase (4.5), then Phase 5

**If ANY REJECT:**
- Do NOT force through
- Proceed to reflect phase (4.5), then yield (4.6)

### 4.5 Reflect Phase: Update Downstream Issues

**This runs after reviews complete, regardless of APPROVE or REJECT outcome.**

The main agent (you) executes this directly - not a subagent.

#### Find Related Issues

```bash
# Current issue number
ISSUE_NUMBER=<number>

# Find issues explicitly blocked by this one (via dependency API)
gh api repos/{owner}/{repo}/issues/$ISSUE_NUMBER/dependencies/blocking \
  --jq 'map(select(.state == "open")) | .[] | "#\(.number) \(.title)"'

# Find other issues in same milestone
gh issue list --milestone "$ARGUMENTS" --state open --json number,title
```

#### Post Context Comments

For each genuinely related issue where this work provides useful context:

```bash
gh issue comment <downstream-number> --body "$(cat <<'EOF'
## Context from #<current-number>

**Branch**: `<branch-name>`
**Status**: <APPROVED / REJECTED>

### What Was Implemented/Attempted

<1-2 sentence summary>

### Impact on This Issue

<How this work affects the downstream issue>

### Notes for Implementation

<Any guidance, caveats, or learnings>
EOF
)"
```

**When to comment:**
- Implementation affects how downstream issue should be approached
- Architectural decisions downstream work should know about
- (If REJECT) Learnings about what didn't work

**When to skip:**
- Relationship is superficial
- Work doesn't materially affect downstream implementation

### 4.6 Yield (On Review Failure)

**Only execute this section if any review returned REJECT.**

1. **Update the current issue with learnings:**

```bash
gh issue comment <number> --body "$(cat <<'EOF'
## Attempt Failed - Learnings

**What was tried:**
- <brief description of approach>

**Why it was rejected:**
- <specific actionable feedback from reviewers>

**What to do differently:**
- <concrete changes for next attempt>

**Files touched (for context):**
- <list of files that were modified>
EOF
)"
```

2. **Abandon the branch and return to main:**

```bash
git checkout main
git branch -D <branch-name>
git pull origin main
```

3. **Exit cleanly** - do NOT create `.ralph-complete`

The loop will retry this issue on the next iteration with the improved context from the comment.

---

## Phase 5: Complete

**Only reach here if ALL reviews APPROVED.**

### 5.1 Final Validation

```bash
make validate
```

### 5.2 Commit

Use the issue title for the commit message:

```bash
git add -A
git commit -m "$(cat <<'EOF'
<issue-title>

<brief description of changes>

Closes #<issue-number>

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>
EOF
)"
```

### 5.3 Merge to Main

```bash
git checkout main
git pull origin main
git merge <branch-name> --no-edit
git push origin main
git branch -d <branch-name>
```

### 5.4 Close Issue

```bash
gh issue comment <number> --body "$(cat <<'EOF'
## Completed

### Changes
- <summary of what was changed>

### Validation
- `make validate` passes
- <other validation criteria checked>
EOF
)"

gh issue close <number>
```

### 5.5 Clean Exit

You should now be on `main` with the changes merged and pushed.

Exit normally. Do NOT create `.ralph-complete` - there may be more issues.

The loop script will invoke another iteration.
