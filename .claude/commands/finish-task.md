---
description: Validate, close beads issue, and create PR for current task
allowed-tools: Bash(bd:*), Bash(git:*), Bash(gh:*), Bash(make:*)
---

## Current State

Branch: !`git branch --show-current`
Git status: !`git status --porcelain`

## Your Workflow

### 1. Final Validation

Run `make validate` (the full validation suite).

If any issues arise:
- Fix them systematically
- Re-run validation
- Do not proceed until validation passes cleanly

### 2. Commit Outstanding Work

Ensure all implementation work is committed:
- [ ] No uncommitted code changes
- [ ] No temporary files or debug artifacts
- [ ] All commits have meaningful messages

### 3. Close Beads Issue

Extract the task ID from the current branch name (format: `jira4claude-XXX`).

1. Close the issue: `bd update <task-id> -s closed`
2. Sync beads to remote: `bd sync`

This syncs beads state to the `beads-sync` branch independently of the PR.

### 4. Verify Clean State

Before creating PR, verify:
- [ ] `git status --porcelain` shows nothing (code changes committed)
- [ ] All work is committed

### 5. Create Pull Request

Push branch and create PR:

```bash
git push -u origin <branch-name>
gh pr create --title "<title>" --body "$(cat <<'EOF'
## Summary
<2-3 bullets of what changed>

## Test Plan
- [ ] <verification steps>

🤖 Generated with [Claude Code](https://claude.com/claude-code)
EOF
)"
```

### 6. Final Verification

After PR creation:
- [ ] Branch is pushed to origin
- [ ] PR is created and URL is shared with user
- [ ] `git status` is completely clean
- [ ] Beads issue shows as `closed` in `bd show <task-id>`

Report the PR URL to the user.
