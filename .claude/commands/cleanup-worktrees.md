---
description: Clean up git worktrees - auto-remove merged PRs, prompt for others
allowed-tools: Bash(git:*), Bash(gh:*), Bash(ls:*)
---

## Current State

Is worktree: !`[ -f .git ] && echo "yes (cannot run from worktree)" || echo "no (main repo)"`
Worktrees: !`ls -1 .worktrees 2>/dev/null || echo "(none)"`

## Your Workflow

### 1. Pre-flight Validation

Before proceeding, verify:
- [ ] Not in a worktree (`.git` must be a directory, not a file)
- [ ] `.worktrees/` directory exists

If in a worktree, tell user to run this from the main repo.
If no `.worktrees/` directory, inform user there are no worktrees to clean.

### 2. Enumerate Worktrees

List all directories in `.worktrees/`:
```bash
ls -1 .worktrees
```

For each worktree directory, the directory name is the task ID (e.g., `J4C-42`).

### 3. Check PR Status for Each

For each worktree, check its PR status:
```bash
gh pr view <task-id> --json state --jq '.state' 2>/dev/null || echo "NO_PR"
```

Categorize each worktree based on the result:
- **MERGED**: PR exists and is merged → auto-remove
- **OPEN**: PR exists but not merged → prompt user
- **NO_PR**: No PR found → prompt user

### 4. Process Worktrees

**For merged PRs** (auto-remove):
```bash
git worktree remove .worktrees/<task-id>
git branch -d <task-id>
```

**For open PRs or no PR**:
Use AskUserQuestion to ask: "Worktree `<task-id>` has [open PR / no PR]. Remove it?"
- If yes: remove worktree and branch
- If no: keep it

### 5. Final Cleanup

Run git worktree prune to clean any stale metadata:
```bash
git worktree prune
```

### 6. Report Summary

Tell the user:
- How many worktrees were removed
- How many remain
- List remaining worktrees if any
