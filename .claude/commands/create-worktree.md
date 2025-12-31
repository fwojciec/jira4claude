---
description: Create a git worktree for a Jira task to enable parallel development
allowed-tools: Bash(git:*), Bash(mkdir:*), Bash(grep:*)
---

## Current State

Branch: !`git branch --show-current`
Is worktree: !`[ -f .git ] && echo "yes (cannot run from worktree)" || echo "no (main repo)"`

## Your Workflow

### 1. Pre-flight Validation

Before proceeding, verify:
- [ ] Not in a worktree (`.git` must be a directory, not a file)
- [ ] Currently on `main` branch
- [ ] Task ID provided as argument ($ARGUMENTS)

If any checks fail, stop and explain the issue to the user.

### 2. Validate Task

Fetch the task from Jira to verify it exists and is workable:
```bash
./jira4claude --config=.jira4claude.yaml view $ARGUMENTS
```

If task is already "Done" or doesn't exist, stop and inform the user.

### 3. Setup Worktrees Directory

Ensure `.worktrees/` directory exists:
```bash
mkdir -p .worktrees
```

Verify `.worktrees/` is in `.gitignore`:
```bash
grep -q "^\.worktrees/$" .gitignore || echo ".worktrees/ not in .gitignore - add it!"
```

### 4. Fetch Latest and Create Worktree

```bash
git fetch origin main
git worktree add .worktrees/$ARGUMENTS -b $ARGUMENTS origin/main
```

### 5. Transition Jira Task

Transition the task to "In Progress" to claim it:
```bash
./jira4claude --config=.jira4claude.yaml transition $ARGUMENTS --status="Start Progress"
```

**Note**: Use `./jira4claude --config=.jira4claude.yaml transition $ARGUMENTS --list-only` to see available transitions if the above fails.

### 6. Report Success

Tell the user:
- Worktree created at `.worktrees/$ARGUMENTS/`
- To start working: open a new terminal, `cd .worktrees/$ARGUMENTS`, run `claude`
- Once in that session, run `/worktree-task` to begin
