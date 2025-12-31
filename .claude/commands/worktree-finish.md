---
description: Finish work in a git worktree - validate, push, create PR, transition Jira
allowed-tools: Bash(git:*), Bash(gh:*), Bash(make:*)
---

## Current State

Branch: !`git branch --show-current`
Is worktree: !`[ -f .git ] && echo "yes" || echo "no (main repo - use /finish-task instead)"`
Git status: !`git status --porcelain`

## Your Workflow

### 1. Pre-flight Validation

Before proceeding, verify:
- [ ] In a worktree (`.git` must be a file, not a directory)
- [ ] Branch name matches `J4C-*` pattern
- [ ] Working tree is clean (all work committed)

If any checks fail, stop and resolve with the user before continuing.

### 2. Final Validation

Run `make validate` (the full validation suite).

If any issues arise:
- Fix them systematically
- Re-run validation
- Do not proceed until validation passes cleanly

### 3. Transition Jira Issue

Extract the task ID from the current branch name (format: `J4C-XXX`).

1. Transition to "Done":
   ```bash
   ./j4c issue transition <task-id> --status="Done"
   ```

2. Verify status:
   ```bash
   ./j4c issue view <task-id> --markdown
   ```

**Note**: Use `./j4c issue transitions <task-id>` to see available transitions if needed.

### 4. Push and Create Pull Request

Push branch and create PR. **Important**: Prefix the PR title with the Jira issue key for easy identification (e.g., "J4C-39: Add --markdown flag").

```bash
git push -u origin <branch-name>
gh pr create --title "<task-id>: <title>" --body "$(cat <<'EOF'
## Summary
<2-3 bullets of what changed>

## Jira
https://fwojciec.atlassian.net/browse/<task-id>

## Test Plan
- [ ] <verification steps>

🤖 Generated with [Claude Code](https://claude.com/claude-code)
EOF
)"
```

### 5. Final Verification

After PR creation:
- [ ] Branch is pushed to origin
- [ ] PR is created and URL is shared with user
- [ ] Jira issue shows as "Done"

### 6. Report Completion

Tell the user:
- PR URL
- Jira task transitioned to Done
- **Important**: "Close this terminal. Run `/cleanup-worktrees` from the main repo when ready to remove this worktree."
