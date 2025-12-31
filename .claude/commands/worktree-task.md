---
description: Start work on a Jira task in a git worktree (infers task from branch name)
allowed-tools: Bash(go:*), Bash(git:*), Bash(make:*)
---

## Current State

Branch: !`git branch --show-current`
Is worktree: !`[ -f .git ] && echo "yes" || echo "no (main repo - use /start-task instead)"`
Uncommitted changes: !`git status --porcelain`

## Your Workflow

### 1. Pre-flight Validation

Before proceeding, verify:
- [ ] In a worktree (`.git` must be a file, not a directory)
- [ ] Branch name matches `J4C-*` pattern
- [ ] Working tree is clean (if not, ask user how to proceed)

If any checks fail, stop and resolve with the user before continuing.

### 2. Build the CLI

Build the j4c binary for this worktree:
```bash
go build -o j4c ./cmd/j4c
```

### 3. Fetch Task Details

Extract task ID from branch name and fetch details:
```bash
./j4c issue view <task-id> --markdown
```

Display the task summary to orient the session.

### 4. Implementation

#### When to Use TDD

Use the `superpowers:test-driven-development` skill when implementing **behavioral requirements**—code that has observable effects, makes decisions, or transforms data.

**TDD applies when:**
- Implementing a use case or requirement
- Adding business logic or decision-making code
- Creating public API contracts
- Building adapters that integrate with external systems

**TDD does NOT apply when:**
- Creating pure data types (structs with no methods, or only trivial accessors)
- Defining interfaces or type aliases
- Writing code during the REFACTOR phase (new internal classes, helpers extracted from working code)
- Adding configuration or constants

#### Decision Heuristic

Before writing a test, ask: "Does this code have behavior that could be wrong?"

- **Yes** → Write a failing test first (RED-GREEN-REFACTOR)
- **No** → Implement directly; behavior tests elsewhere will catch integration issues

#### Architectural Decisions

If the task involves any of these:
- Creating new packages or files
- Deciding where code belongs
- Adding new mocks or mock methods
- Package naming decisions

Then **ALSO** use the `go-standard-package-layout` skill for guidance.

### 5. Validation

After implementation is complete:
1. Run `make validate`
2. Address any issues that arise (linting, test failures, etc.)
3. Iterate until validation passes

Only proceed to step 6 when `make validate` passes cleanly.

### 6. Self-Review

Before finishing, get an independent perspective on the implementation:

Launch a code review subagent:
- `Task(subagent_type="superpowers:code-reviewer")` - correctness, style, bugs

Use `superpowers:receiving-code-review` to evaluate each suggestion on merit. Accept what improves correctness or structural discipline. Push back on stylistic preferences.

**If changes are needed:**
- Implement fixes (return to step 4 if substantial)
- Run `make validate` again
- Repeat self-review if changes were significant

Only proceed to `/worktree-finish` when review is addressed and validation passes.
