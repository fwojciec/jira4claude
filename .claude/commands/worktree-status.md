---
description: Show status of all git worktrees with their Jira issue and PR states
allowed-tools: Bash(git:*), Bash(gh:*), Bash(ls:*)
---

## Current State

Is worktree: !`[ -f .git ] && echo "yes (cannot run from worktree)" || echo "no (main repo)"`
Worktrees: !`ls -1 .worktrees 2>/dev/null || echo "(none)"`

## Your Workflow

### 1. Pre-flight Validation

Before proceeding, verify:
- [ ] Not in a worktree (`.git` must be a directory, not a file)

If in a worktree, tell user to run this from the main repo.
If no `.worktrees/` directory or it's empty, inform user there are no active worktrees.

### 2. Gather Status for Each Worktree

For each worktree directory in `.worktrees/`, gather:

**Jira status:**
```bash
./jira4claude --config=.jira4claude.yaml view <task-id> --json | jq -r '.status'
```

**PR status:**
```bash
gh pr view <task-id> --json state,url --jq '{state: .state, url: .url}' 2>/dev/null || echo "no PR"
```

**Git status (uncommitted changes):**
```bash
git -C .worktrees/<task-id> status --porcelain | wc -l | xargs
```

### 3. Display Summary Table

Present results in a table format:

| Worktree | Jira Status | PR Status | Uncommitted |
|----------|-------------|-----------|-------------|
| J4C-42   | In Progress | Open      | 3 files     |
| J4C-43   | Done        | Merged    | clean       |

### 4. Provide Recommendations

Based on status, suggest actions:

- **Jira Done + PR Merged**: Ready for cleanup (`/cleanup-worktrees`)
- **Jira Done + No PR**: May need `/worktree-finish` or manual cleanup
- **Jira In Progress + No PR**: Work in progress, continue in that worktree
- **Jira In Progress + PR Open**: Awaiting review or needs `/address-pr-comments`
