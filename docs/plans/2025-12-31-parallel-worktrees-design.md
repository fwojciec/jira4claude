# Parallel Development with Git Worktrees

Enable multiple Claude Code sessions working on different Jira tasks simultaneously using git worktrees.

## Problem

Currently, development is single-session: one Claude Code instance works on one task at a time. With LLMs writing code, we want to parallelize - multiple sessions working on different tasks concurrently.

## Solution

Four new Claude commands that manage worktree lifecycle:

| Command | Run from | Purpose |
|---------|----------|---------|
| `/create-worktree J4C-XX` | Main repo | Creates worktree, transitions Jira to In Progress |
| `/worktree-task` | Worktree | Starts work (infers task from branch name) |
| `/worktree-finish` | Worktree | Finishes work (push, PR, transition Jira to Done) |
| `/cleanup-worktrees` | Main repo | Removes worktrees for merged PRs |

## Directory Structure

```
jira4claude/
├── .worktrees/           # gitignored
│   ├── J4C-42/           # worktree for task J4C-42
│   └── J4C-43/           # worktree for task J4C-43
├── .gitignore            # includes .worktrees/
└── ...
```

## Workflow

```
[main repo]     /create-worktree J4C-42
[you]           open terminal, cd .worktrees/J4C-42, run claude
[worktree]      /worktree-task → work → /worktree-finish
[you]           close that terminal
[main repo]     /cleanup-worktrees (when ready)
```

## Command Details

### `/create-worktree J4C-XX`

**Pre-flight:**
- Must be in main repo (not already in a worktree)
- Must be on `main` branch
- Task ID provided as argument (required)
- Verify task exists in Jira and is workable (not Done)

**Actions:**
1. Ensure `.worktrees/` directory exists
2. Ensure `.worktrees/` is in `.gitignore`
3. Fetch latest from origin
4. Create worktree: `git worktree add .worktrees/J4C-XX -b J4C-XX origin/main`
5. Transition Jira task to "In Progress"
6. Report: "Worktree ready at `.worktrees/J4C-XX/` - open a terminal there and run `claude`"

**Does not:** Run build steps or launch Claude Code.

### `/worktree-task`

**Pre-flight:**
- Verify we're in a git worktree (not main repo)
- Branch name matches `J4C-*` pattern
- Working tree is clean

**Actions:**
1. Extract task ID from branch name
2. Build the binary: `go build -o jira4claude ./cmd/jira4claude`
3. Fetch task details from Jira: `./jira4claude view J4C-XX`
4. Display task summary
5. Proceed to implementation (same TDD workflow as `/start-task`)

**Key differences from `/start-task`:**
- No branch creation (already on task branch)
- No "must be on main" check
- No Jira transition (done by `/create-worktree`)
- Builds binary as setup step

### `/worktree-finish`

**Pre-flight:**
- Verify we're in a git worktree (not main repo)
- Branch name matches `J4C-*` pattern
- Working tree is clean (all work committed)
- `make validate` passes

**Actions:**
1. Extract task ID from branch name
2. Push branch: `git push -u origin J4C-XX`
3. Create PR via `gh pr create`
4. Transition Jira task to "Done"
5. Report: "PR created. Run `/cleanup-worktrees` from main repo when ready."

### `/cleanup-worktrees`

**Pre-flight:**
- Must be in main repo (not in a worktree)

**Actions:**
1. List all worktrees in `.worktrees/`
2. For each, check PR status via `gh pr view`
3. Categorize: merged (auto-remove), open (prompt), no PR (prompt)
4. Remove worktrees as appropriate
5. Delete local branches for removed worktrees
6. Run `git worktree prune`

**Output example:**
```
Checking worktrees...
  J4C-42: PR merged - removing
  J4C-43: PR open - keep? [y/N]

Removed 1 worktree. 1 remains.
```

## Technical Details

**Worktree detection:**
```bash
# In a worktree, .git is a file; in main repo, it's a directory
[ -f .git ] && echo "worktree" || echo "main repo"
```

**Files to create/modify:**
- `.claude/commands/create-worktree.md` (new)
- `.claude/commands/worktree-task.md` (new)
- `.claude/commands/worktree-finish.md` (new)
- `.claude/commands/cleanup-worktrees.md` (new)
- `.gitignore` (add `.worktrees/`)

## Design Decisions

1. **Separate commands** - `/worktree-*` variants rather than modifying existing commands, for clarity and safety.

2. **Worktrees inside repo** - `.worktrees/` subdirectory keeps everything together and is easily gitignored.

3. **Manual session launch** - User opens terminals and runs `claude` manually; coordinator doesn't try to spawn sessions.

4. **Build in worktree** - Each worktree builds its own binary for dogfooding.

5. **Jira as coordination** - Task status (In Progress) signals work is claimed; no additional coordination mechanism needed.

6. **Cleanup via PR status** - Merged PRs are safe to auto-remove; others prompt for confirmation.
