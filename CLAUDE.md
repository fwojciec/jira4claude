# CLAUDE.md

Strategic guidance for LLMs working with this codebase.

## Why This Codebase Exists

**Core Problem**: The official jira-cli hangs in non-interactive contexts (like Claude Code) because it prompts for optional fields. AI agents need predictable, non-interactive commands with structured output.

**Solution**: A minimal Jira CLI designed specifically for AI agents - zero interactivity, readable flags, clean output.

## Design Philosophy

- **Ben Johnson Standard Package Layout** - domain types in root, dependencies in subdirectories
- **Agent-first** - never prompt, never hang, always explicit
- **Minimal scope** - only commands agents actually need (~8 endpoints, not 417)
- **Process over polish** - systematic validation results in quality

## Workflows

### Single-Session Development

| Command | Purpose |
|---------|---------|
| `/start-task` | Pick a Jira task, create branch, implement with TDD |
| `/finish-task` | Validate, transition Jira issue to Done, create PR |
| `/address-pr-comments` | Fetch, evaluate, and respond to PR feedback |

### Parallel Development (Multiple Sessions)

Run multiple Claude Code sessions on different tasks using git worktrees:

| Command | Run from | Purpose |
|---------|----------|---------|
| `/create-worktree J4C-XX` | Main repo | Create worktree, transition Jira to In Progress |
| `/worktree-task` | Worktree | Start work (infers task from branch name) |
| `/worktree-finish` | Worktree | Validate, push, create PR, transition to Done |
| `/cleanup-worktrees` | Main repo | Remove worktrees for merged PRs |

**Workflow:**
```
[main repo]   /create-worktree J4C-42
[terminal]    cd .worktrees/J4C-42 && claude
[worktree]    /worktree-task → work → /worktree-finish
[terminal]    close session
[main repo]   /cleanup-worktrees
```

Worktrees are stored in `.worktrees/` (gitignored). Each worktree builds its own binary for dogfooding.

**Quick reference**:
```bash
make validate     # Quality gate - run before completing any task
```

**Planning workflow** (mandatory for new work):
1. Research the problem
2. Use `/brainstorm` to refine into design
3. Write design doc to `docs/plans/`
4. Use `jira-workflow` skill to create tasks with dependencies
5. Use `ready` command to find unblocked work

## Architecture Patterns

**Ben Johnson Pattern**:
- Root package: domain types and interfaces only (no external dependencies)
- Subdirectories: one per external dependency (e.g., `jira/` for API client)
- `mock/`: manual mocks with function fields for testing
- `cmd/jira4claude/`: wires everything together

**File Naming Convention**:
- `foo/foo.go`: shared utilities for the package
- `foo/foo_test.go`: shared test utilities (in `foo_test` package)
- Entity files: named after domain entity (`issue.go`, `client.go`)

When uncertain about where code belongs, use the `go-standard-package-layout` skill.

## Skills

### Task Management

**`jira-workflow`** - Use when:
- Creating new tasks or subtasks
- Checking what work is ready (unblocked)
- Linking tasks with dependencies
- Transitioning task status

### Architecture

**`go-standard-package-layout`** - Use when:
- Creating new packages or files
- Deciding where code belongs
- Naming packages or files
- Writing mocks in `mock/`

### Development (invoked automatically by `/start-task`)

- **`superpowers:test-driven-development`** - Write test first, watch it fail, implement
- **`superpowers:systematic-debugging`** - Understand root cause before fixing
- **`superpowers:verification-before-completion`** - Evidence before assertions

## Writing Issues

Issues should be easy to complete. Create via Jira API:

```bash
curl -s -n -X POST -H "Content-Type: application/json" \
  https://fwojciec.atlassian.net/rest/api/3/issue \
  -d '{"fields": {"project": {"key": "J4C"}, "summary": "Title", "issuetype": {"name": "Task"}, "description": {"type": "doc", "version": 1, "content": [{"type": "paragraph", "content": [{"type": "text", "text": "Description here"}]}]}}}'
```

**Principles**:
- Write **what** needs doing, not **how**
- One issue = one PR
- Reference specific files to reduce discovery time

## Test Philosophy

**TDD is mandatory** - write failing tests first, then implement.

**Package Convention**:
- All tests MUST use external test packages: `package foo_test` (not `package foo`)
- This enforces testing through the public API only
- Linter (`testpackage`) will fail on tests in the same package

**Parallel Tests**:
- All tests MUST call `t.Parallel()` at the start of:
  - Every top-level test function
  - Every subtest (`t.Run` callback)
- Linter (`paralleltest`) will fail on missing parallel calls

**Example Pattern**:
```go
package jira_test  // External test package

func TestCreateIssue(t *testing.T) {
    t.Parallel()  // Required

    t.Run("with valid fields", func(t *testing.T) {
        t.Parallel()  // Also required
        // test code...
    })
}
```

**Assertions**:
- Use `require` for setup (fails fast)
- Use `assert` for test assertions (continues on failure)

**Interface Compliance Checks**:
Go's `var _ Interface = (*Type)(nil)` pattern verifies interface implementation at compile time. These checks MUST be in production code, NOT in tests.

## Linting

golangci-lint enforces:
- No global state (`gochecknoglobals`) - per Ben Johnson pattern
- Separate test packages (`testpackage`)
- Error checking (`errcheck`) - all errors must be handled

## LSP Tools (cclsp MCP)

This project has the `cclsp` MCP configured, providing Go language server integration via gopls.

**Available tools:**

| Tool | Use when |
|------|----------|
| `mcp__cclsp__find_definition` | Jump to where a symbol is defined |
| `mcp__cclsp__find_references` | Find all usages of a function, type, or variable |
| `mcp__cclsp__rename_symbol` | Safely rename symbols across the codebase |
| `mcp__cclsp__get_diagnostics` | Check a file for errors, warnings, or hints |

**When to use:**
- Navigating unfamiliar code - use `find_definition` to understand what a symbol is
- Refactoring - use `find_references` before changing a function signature
- Renaming - use `rename_symbol` instead of manual find/replace
- Validation - use `get_diagnostics` to check for compile errors after edits

## Reference Documentation

- `.claude/commands/` - Workflow commands
- `docs/design.md` - Architecture and API design
