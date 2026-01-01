---
description: Validate, transition Jira issue to Done, and create PR for current task
allowed-tools: Bash(curl:*), Bash(git:*), Bash(gh:*), Bash(make:*), Bash(jq:*)
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

### 3. Transition Jira Issue

Extract the task ID from the current branch name (format: `J4C-XXX`).

1. Transition to "Done":
   ```bash
   ./j4c issue transition <task-id> --status="Done"
   ```

2. Verify status:
   ```bash
   ./j4c issue view <task-id>
   ```

**Note**: Use `./j4c issue transitions <task-id>` to see available transitions if needed.

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

## Jira
https://fwojciec.atlassian.net/browse/<task-id>

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
- [ ] Jira issue shows as "Done"

Report the PR URL to the user.
