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

## Workflow

`make validate` is the quality gate (fmt, vet, tidy, lint, race tests). There is no CI on
branches, so a change is done only when the gate is green on the tree that will be `main`.

**Tickets**: the work graph lives in `.tickets/`, managed by the vendored tracker
(`tools/ticket/README.md`): one epic per milestone, tasks as children, blocking edges
with `dep`. `tools/ticket/ticket ready` lists unblocked work (not a claim);
`ticket show <id>` is the task's contract (description and acceptance criteria); run
`ticket dep cycle` after editing dependencies. `docs/plans/` stays the spec. Create
tickets from the main checkout: in a worktree the tracker takes its id prefix from the
directory name. GitHub issues are the inbox for outside reports only; turn one into a
ticket (`--external-ref gh-<n>`) before working on it.

**New work**:
1. Research the problem; for anything beyond a small fix, write a design doc to `docs/plans/`.
2. Create tickets with dependencies under the milestone's epic. The ticket is the contract.
3. Implement each ticket on a local branch (a worktree when several run in parallel), tests first.
4. Review the diff against the ticket's acceptance criteria.
5. Rebase onto `main`, run `make validate`, merge to `main`, close the ticket
   (`ticket close <id>`, with `ticket add-note` naming the commit), push.

No pull requests for routine work: local validation is the gate. Pull requests are for
outside contributions.

## Architecture Patterns

**Ben Johnson Pattern**:
- Root package: domain types and interfaces only (no external dependencies)
- Subdirectories: one per external dependency (e.g., `jira/` for API client)
- `mock/`: manual mocks with function fields for testing
- `cmd/j4c/`: wires everything together

**File Naming Convention**:
- `foo/foo.go`: shared utilities for the package
- `foo/foo_test.go`: shared test utilities (in `foo_test` package)
- Entity files: named after domain entity (`issue.go`, `client.go`)

When uncertain about where code belongs, use the `go-standard-package-layout` skill.

## Skills

### Architecture

**`go-standard-package-layout`** - Use when:
- Creating new packages or files
- Deciding where code belongs
- Naming packages or files
- Writing mocks in `mock/`

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

- `README.md` - what the tool does, for scripts and agents
- `docs/plans/2026-11-09-scripted-use-alignment-design.md` - the current design and what was built
- `docs/design.md` - the original design, kept as history
