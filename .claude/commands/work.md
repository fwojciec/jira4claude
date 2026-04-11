---
description: Pick a GitHub issue, implement with TDD, review, and create PR
allowed-tools: Bash(gh:*), Bash(git:*), Bash(make:*), Bash(jq:*), Bash(node:*), Bash(codex:*)
argument-hint: "[issue-number]"
---

# Work on GitHub Issue

End-to-end workflow: select issue → create branch → implement → review → PR.

**Design principle**: This workflow should run to completion with minimal user interaction. Only stop for:
- Pre-flight failures (not on main, dirty working tree)
- Task selection (if no issue number provided)
- Blocking review feedback (REJECT verdicts)
- Post-PR merge decision

Everything else (implementation, validation, reviews, PR creation) should flow automatically.

## Current State

Branch: !`git branch --show-current`
Uncommitted changes: !`git status --porcelain`

## Arguments

$ARGUMENTS

---

## Phase 0: Detect Resume

Before anything else, check the current branch:

```bash
BRANCH=$(git branch --show-current)
```

**If branch matches `<number>-<description>` pattern** (e.g., `42-add-auth`):
- Extract issue number: `echo "$BRANCH" | grep -oE '^[0-9]+'`
- Fetch issue details: `gh issue view <number> --comments`
- Determine state:
  - Uncommitted changes → resume at Phase 2 (implement)
  - Clean with commits ahead of main → resume at Phase 3 (review)
  - PR already exists (`gh pr view 2>/dev/null`) → report status and offer `/address-pr-comments`
- Ask user to confirm resume or start fresh (checkout main first)

**If on `main`**: proceed to Phase 1.

---

## Phase 1: Select & Setup

### 1.1 Pre-flight Validation

Before proceeding, verify:
- [ ] Currently on `main` branch (if not, ask user before proceeding)
- [ ] Working tree is clean (if not, ask user how to proceed)

If any checks fail, stop and resolve with the user before continuing.

### 1.2 Task Selection

**If issue number provided in $ARGUMENTS**: Use that issue.

**If no arguments**: List open issues filtered to ready (unblocked) work:

1. Fetch open issues:
   ```bash
   gh issue list --state open --json number,title,labels --limit 50
   ```

2. For each issue, check for open blockers (dependency records persist after
   a blocker is closed, so filter by state):
   ```bash
   gh api repos/{owner}/{repo}/issues/<number>/dependencies/blocked_by \
     --jq 'map(select(.state == "open")) | length'
   ```

3. An issue is "ready" if it has zero open blockers. Filter out blocked
   issues before presenting.

4. Present only ready issues to the user.

Use AskUserQuestion to let user pick which issue to work on.

### 1.3 Branch Setup

Once issue is selected:

1. Get issue details for branch name:
   ```bash
   gh issue view <number> --json number,title
   ```

2. Create branch with format `<number>-<short-description>`:
   - Convert title to lowercase kebab-case
   - Truncate to reasonable length (50 chars max)
   - Example: `42-add-user-authentication`

   ```bash
   git checkout -b <branch-name>
   ```

3. Show full issue details **including comments**:
   ```bash
   gh issue view <number> --comments
   ```

   **IMPORTANT**: Always read comments. Earlier work on related issues often
   leaves context comments that affect implementation choices (e.g., suggested
   approaches, API decisions, integration notes).

---

## Phase 2: Implementation

### 2.1 TDD Workflow

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

### 2.2 Validation During Development

Run validation frequently:
```bash
make validate
```

Address any issues before continuing.

---

## Phase 3: Review

**IMPORTANT**: This entire phase should complete automatically without
waiting for user input, unless a review returns REJECT with blocking issues
that require user decision.

### 3.1 Pre-Review Validation

Ensure validation passes:
```bash
make validate
```

### 3.2 Parallel Code Reviews

Launch TWO review tracks in parallel. Use a single message with multiple
tool calls for actual parallelism.

**Track A -- Claude code-review subagent:**

Launch as a Task subagent with `subagent_type="superpowers:code-reviewer"`:

Prompt: Review the current branch changes vs main for structural discipline
and project standards. Read CLAUDE.md for codebase-specific rules. Return
APPROVE/REJECT verdict with specific feedback.

After the code review verdict, the subagent also runs the **reflect phase**:

1. Extract issue number from branch name
2. Find open downstream issues via dependency API:
   ```bash
   gh api repos/{owner}/{repo}/issues/$ISSUE_NUMBER/dependencies/blocking \
     --jq 'map(select(.state == "open"))'
   ```
3. Get milestone:
   ```bash
   gh issue view $ISSUE_NUMBER --json milestone --jq '.milestone.title // empty'
   ```
4. If milestone exists, find related open issues:
   ```bash
   gh issue list --milestone "$MILESTONE" --state open --json number,title
   ```
5. For each genuinely related issue where the current work provides useful
   context, post a comment:
   ```bash
   gh issue comment <downstream-number> --body "$(cat <<'EOF'
   ## Context from #<current-number>

   **Branch**: `<branch-name>`
   **Status**: <APPROVED / REJECTED>

   ### What Was Implemented
   <1-2 sentence summary>

   ### Impact on This Issue
   <How this work affects the downstream issue>

   ### Notes for Implementation
   <Any guidance, caveats, or learnings>
   EOF
   )"
   ```

   **When to comment:**
   - The implementation affects how that issue should be approached
   - Architectural decisions were made that downstream work should know about
   - Integration points or APIs that related work will use

   **When to skip:**
   - Relationship is superficial (just same milestone, no real connection)
   - Current work doesn't materially affect the related issue

**Track B -- Codex review:**

First check if Codex is available:
```bash
command -v codex >/dev/null 2>&1 && echo "available" || echo "unavailable"
```

If unavailable, skip this track and warn the user. Do not block on Codex.

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

### 3.3 Process Review Results

When reviews complete, evaluate the verdicts:

**If all APPROVE**: Proceed to Phase 4. Do not wait for user input.

**If APPROVE with suggestions**: Apply technical judgment. Minor improvements
can be made inline, then proceed to Phase 4.

**If REJECT**: Present the blocking issues to the user. Use AskUserQuestion
to determine how to proceed:
- Fix the issues and re-review
- Proceed anyway (user override)
- Abandon the work

Use `superpowers:receiving-code-review` to evaluate each suggestion on merit.

**Do NOT stop to ask "should I continue?" or "reviews are done, what next?"**
-- the workflow is designed to flow continuously into Phase 4.

---

## Phase 4: Finish

**CHECKPOINT**: You should only be here after Phase 3 reviews are complete
and feedback is addressed.

### 4.1 Final Validation

Run validation:
```bash
make validate
```

Do not proceed until this passes cleanly.

### 4.2 Commit Outstanding Work

Ensure all work is committed:
- [ ] No uncommitted code changes
- [ ] No temporary files or debug artifacts
- [ ] All commits have meaningful messages

### 4.3 Create Pull Request

Push branch and create PR:

```bash
git push -u origin <branch-name>

gh pr create --title "<title>" --body "$(cat <<'EOF'
## Summary
<2-3 bullets of what changed>

Closes #<issue-number>

## Test Plan
- [ ] `make validate` passes
- [ ] <additional verification steps>

Generated with [Claude Code](https://claude.com/claude-code)
EOF
)"
```

### 4.4 Final Status

Post completion comment on the issue:

```bash
gh issue comment <number> --body "$(cat <<'EOF'
## Implementation Complete

PR: <pr-url>

### Summary
<brief description of what was implemented>

### Changes
- <key changes>

### Testing
- <how it was validated>
EOF
)"
```

Report the PR URL to the user.

### 4.5 Await Merge

After creating the PR, ask the user what to do next:

Use AskUserQuestion with options:
1. **"Merge it"** - Merge the PR now
2. **"I'll merge it myself"** - User will handle merge
3. **"Keep working"** - Stay on branch for more changes

**If "Merge it":**
```bash
gh pr merge <pr-number> --squash --delete-branch
git checkout main
git pull origin main
```

**If "I'll merge it myself":**
```bash
git checkout main
git pull origin main
```

**If "Keep working":**
Stay on the current branch and await further instructions.

Always end with a clean state: on `main` branch with latest changes pulled.
