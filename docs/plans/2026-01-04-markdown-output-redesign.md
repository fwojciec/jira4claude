# Markdown Output Redesign

## Goal

Simplify CLI output to a single markdown-based format, eliminating the color/no-color duality while maintaining JSON mode for programmatic access.

## Principles

1. All content output is valid GitHub-Flavored Markdown
2. Bracket notation `[x]` for visual indicators (status, priority, messages)
3. Omit empty fields rather than showing placeholders
4. Minimal decoration - no `===` separators, no `...` lines
5. Consistent message vocabulary: `[ok]`, `[warn]`, `[error]`, `[info]`

## Output Modes

- **Markdown** (default) - GFM-compatible, optimized for AI agents
- **JSON** (`--json`) - Unchanged, for programmatic access

## What Gets Removed

- Color mode / lipgloss styling
- Unicode indicators (✓, ▶, ○, ▲, ▽)
- Card borders and decorative separators
- Glamour markdown rendering (just pass through raw markdown)
- The entire `gogh/` package

## Output Formats

### Single Issue View

```markdown
# J4C-100: cmd package cleanup and test coverage improvements

**Type:** Task
**Status:** Done
**Priority:** Medium
**Assignee:** Filip Wojciechowski
**Reporter:** Filip Wojciechowski
**Parent:** J4C-96
**Labels:** backend, cleanup

## Context

Code review identified stale TODO comments, low-value tests, and missing test
coverage in the cmd/j4c package.

## Investigation Starting Points

- Examine cmd/j4c/issue_test.go:609,689 for stale J4C-80 TODOs
- Review cmd/j4c/main_test.go:110-146 for low-value tests

## Subtasks

- [x] J4C-97: Investigate current subtask behavior and gaps
- [x] J4C-98: Fix subtask type name mismatch
- [ ] J4C-99: Display subtasks in parent issue view

## Linked Issues

**blocks:**
- [ ] J4C-78: Rename adf package

**is blocked by:**
- [x] J4C-74: Inject Converter into CLI IssueContext

## Comments

**Filip Wojciechowski** (2026-01-04 10:30):
Completed the initial investigation, found three issues.

[View in Jira](https://fwojciec.atlassian.net/browse/J4C-100)
```

**Rules:**
- H1 = Key + Summary
- Metadata fields one per line, omit if empty
- Field order: Type, Status, Priority, Assignee, Reporter, Parent, Labels
- Description passes through as-is (already markdown)
- Section order: Subtasks, Linked Issues, Comments, URL
- Sections only appear if they have content
- URL as markdown link at end

### Issue List

```markdown
- [x] [!] **J4C-103** http package: cleanup stale comments and reduce duplication
- [x] [!] **J4C-102** goldmark package: improve test coverage for edge cases
- [ ] [!] **J4C-95** Investigate epic support in issue display
- [~] [!!] **J4C-104** Implement new feature
```

**Format:** `- [status] [priority] **KEY** summary`

**Status indicators:**
- `[x]` - Done
- `[ ]` - To Do
- `[~]` - In Progress

**Priority indicators:**
- `[!!!]` - Highest
- `[!!]` - High
- `[!]` - Medium
- `[-]` - Low
- `[--]` - Lowest

**Truncation:** Constant `maxSummaryLength = 60`, truncate with `...`

### Links List

```markdown
**blocks:**
- [ ] J4C-78: Rename adf package

**is blocked by:**
- [x] J4C-74: Inject Converter into CLI IssueContext
```

### Transitions List

```markdown
- In Progress
- Done
- Blocked
```

### System Messages

```
[ok] Issue J4C-105 created
[ok] Transitioned J4C-100 to Done
[warn] Description contained unsupported formatting
[error] Failed to connect to Jira API
[info] No issues found
```

### Empty States

```
[info] No issues found
[info] No links for J4C-100
[info] No transitions available for J4C-100
```

## Package Structure

### Before

```
jira4claude/
├── goldmark/       # Markdown conversion
├── gogh/           # Text + JSON printing with styling
└── ...
```

### After

```
jira4claude/
├── markdown/           # Renamed from goldmark
│   ├── markdown.go     # Parser setup
│   ├── to_adf.go       # GFM → ADF conversion
│   ├── to_markdown.go  # ADF → GFM conversion
│   └── printer.go      # MarkdownPrinter implementation
│
├── json/               # New package
│   └── printer.go      # JSONPrinter implementation
│
├── printer.go          # Printer interface (root package)
├── view.go             # View types (unchanged)
└── cmd/j4c/            # CLI wiring
```

## Migration Plan

Incremental migration with separate PRs:

1. **Create `json/` package** - Copy JSONPrinter from gogh, adapt
2. **Rename `goldmark/` → `markdown/`** - Package rename only
3. **Implement MarkdownPrinter** - New printer in `markdown/`
4. **Update CLI** - Switch `cmd/j4c/` to use new printers
5. **Delete `gogh/`** - Remove once nothing imports it

Each step is independently testable and deployable.

## File Mapping

| Old Location | New Location |
|--------------|--------------|
| `gogh/json.go` | `json/printer.go` |
| `gogh/text.go` | Replaced by `markdown/printer.go` |
| `gogh/theme.go` | Deleted |
| `gogh/render.go` | Deleted |
| `gogh/io.go` | Root package or inline |
| `goldmark/*` | `markdown/*` |

## Dependencies Removed

- `github.com/charmbracelet/lipgloss`
- `github.com/charmbracelet/glamour`
- `github.com/muesli/termenv`
