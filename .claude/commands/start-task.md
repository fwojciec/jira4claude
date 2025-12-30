---
description: Pick a Jira task, create branch, and implement with behavioral TDD
allowed-tools: Bash(curl:*), Bash(git:*), Bash(make:*), Bash(jq:*)
---

## Current State

Branch: !`git branch --show-current`
Uncommitted changes: !`git status --porcelain`

## Your Workflow

### 1. Pre-flight Validation

Before proceeding, verify:
- [ ] Currently on `main` branch (if not, ask user before proceeding)
- [ ] Working tree is clean (if not, ask user how to proceed)

If any checks fail, stop and resolve with the user before continuing.

### 2. Task Selection

**If a task ID was provided via argument ($1)**:
- Fetch the task from Jira: `./jira4claude --config=.jira4claude.yaml view $1`
- Skip to step 3 (Branch Setup)

**If no task ID was provided**:
- List open tasks: `./jira4claude --config=.jira4claude.yaml list --jql="status != Done"`
- Present the tasks to the user
- Use the AskUserQuestion tool to let the user choose which task to work on

### 3. Branch Setup

Once you have a task ID:
1. Create branch: `git checkout -b <task-id>` (e.g., `git checkout -b J4C-42`)
2. Transition to "In Progress" (if workflow supports it):
   ```bash
   ./jira4claude --config=.jira4claude.yaml transition <task-id> --status="Start Progress"
   ```
3. Show task details: `./jira4claude --config=.jira4claude.yaml view <task-id>`

**Note**: Use `./jira4claude --config=.jira4claude.yaml transition <task-id> --list-only` to see available transitions.

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

Only proceed to `/finish-task` when review is addressed and validation passes.
