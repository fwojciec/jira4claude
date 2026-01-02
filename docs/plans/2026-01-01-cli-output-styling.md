# CLI Output Styling Design

Design for enhanced CLI output with dual-mode rendering: rich terminal styling and graceful text-only degradation.

## Problem

Current CLI output is functional but plain. We need:
1. **Rich terminal mode** - Polished visual experience for direct CLI use
2. **Text-only mode** - Clean output for Claude Code (NO_COLOR) that's readable by humans and parseable by AI

## Design Principles

- Both modes are first-class citizens, not fallbacks
- 80-column width for comfortable reading
- Consistent visual language across all output types
- Status indicators work without color (`[x]`, `[>]`, `[ ]`)
- Markdown preserved in descriptions (readable as-is in monospace)

## Output Designs

### Issue Detail View

#### Colored Terminal Mode

```
╭────────────────────────────────────────────────────────────────────────────────╮
│  J4C-81                                                                  TASK  │
│  Add CLI view models and update handlers                                       │
│┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄│
│  STATUS              PRIORITY                                                  │
│  ✓ Done              ▲ Medium                                                  │
│┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄│
│  Assignee:                                                Filip Wojciechowski  │
│  Reporter:                                                Filip Wojciechowski  │
╰────────────────────────────────────────────────────────────────────────────────╯

╭─ LINKED ISSUES ────────────────────────────────────────────────────────────────╮
│  is blocked by                                                                 │
│  J4C-74  [Done]   Inject Converter into CLI IssueContext                       │
│  J4C-76  [Done]   Add warning propagation from Converter to presentation       │
│  J4C-80  [Done]   Remove Converter from HTTP layer                             │
│                                                                                │
│  blocks                                                                        │
│  J4C-78  [To Do]  Rename adf package to goldmark per Ben Johnson pattern       │
╰────────────────────────────────────────────────────────────────────────────────╯

## Context

CLI layer needs view models to convert domain types (with ADF) to display-ready
types (with markdown strings). This enables both JSON and text output to show
human-readable content.

## Scope Constraints

- Create `cmd/j4c/view.go` with issueView, commentView, linkView types
- Add JSON struct tags for serialization control
- Update CLI handlers to convert domain → view before printing

https://fwojciec.atlassian.net/browse/J4C-81
```

#### Text-Only Mode (NO_COLOR)

```
J4C-81                                                                     TASK
Add CLI view models and update handlers
................................................................................
STATUS              PRIORITY
[x] Done            [!] Medium
................................................................................
Assignee:                                                   Filip Wojciechowski
Reporter:                                                   Filip Wojciechowski

=== LINKED ISSUES ============================================================

is blocked by
  J4C-74  [Done]   Inject Converter into CLI IssueContext
  J4C-76  [Done]   Add warning propagation from Converter to presentation
  J4C-80  [Done]   Remove Converter from HTTP layer

blocks
  J4C-78  [To Do]  Rename adf package to goldmark per Ben Johnson pattern

-------------------------------------------------------------------------------

## Context

CLI layer needs view models to convert domain types (with ADF) to display-ready
types (with markdown strings). This enables both JSON and text output to show
human-readable content.

## Scope Constraints

- Create cmd/j4c/view.go with issueView, commentView, linkView types
- Add JSON struct tags for serialization control
- Update CLI handlers to convert domain → view before printing

https://fwojciec.atlassian.net/browse/J4C-81
```

### Issue List / Ready (Table)

#### Colored Mode

```
╭──────────────────────────────────────────────────────────────────────────────╮
│  KEY       STATUS        ASSIGNEE               SUMMARY                      │
│┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄│
│  J4C-78    To Do         Filip Wojciechowski    Rename adf package to gold…  │
│  J4C-66    To Do         Filip Wojciechowski    Create Homebrew tap for j4c  │
╰──────────────────────────────────────────────────────────────────────────────╯
```

#### Text-Only Mode

```
KEY       STATUS        ASSIGNEE               SUMMARY
--------  ------------  ---------------------  ----------------------------------
J4C-78    To Do         Filip Wojciechowski    Rename adf package to goldmark...
J4C-66    To Do         Filip Wojciechowski    Create Homebrew tap for j4c...
```

### Success Messages

#### Colored Mode

```
✓ Transitioned: J4C-81
  https://fwojciec.atlassian.net/browse/J4C-81
```

#### Text-Only Mode

```
[ok] Transitioned: J4C-81
https://fwojciec.atlassian.net/browse/J4C-81
```

### Transitions List

#### Colored Mode

```
Available transitions for J4C-81:
  → Start Progress
  → Done
```

#### Text-Only Mode

```
Available transitions for J4C-81:
  -> Start Progress
  -> Done
```

### Links List

#### Colored Mode

```
╭─ Links for J4C-81 ─────────────────────────────────────────────────────────────╮
│  is blocked by                                                                 │
│  J4C-74  [Done]   Inject Converter into CLI IssueContext                       │
│                                                                                │
│  blocks                                                                        │
│  J4C-78  [To Do]  Rename adf package to goldmark per Ben Johnson pattern       │
╰────────────────────────────────────────────────────────────────────────────────╯
```

#### Text-Only Mode

```
=== Links for J4C-81 =========================================================

is blocked by
  J4C-74  [Done]   Inject Converter into CLI IssueContext

blocks
  J4C-78  [To Do]  Rename adf package to goldmark per Ben Johnson pattern
```

### Warnings / Errors

#### Colored Mode

```
⚠ Warning: ADF node type 'unknown' not supported
✗ Error: Issue J4C-999 not found
```

#### Text-Only Mode

```
[warn] ADF node type 'unknown' not supported
[error] Issue J4C-999 not found
```

## Visual Language

### Status Indicators

| Status      | Colored    | Text-Only |
|-------------|------------|-----------|
| Done        | ✓ (green)  | [x]       |
| In Progress | ▶ (blue)   | [>]       |
| To Do       | ○ (gray)   | [ ]       |

### Priority Indicators

| Priority | Colored      | Text-Only |
|----------|--------------|-----------|
| Highest  | ▲▲▲ (red)    | [!!!]     |
| High     | ▲▲ (orange)  | [!!]      |
| Medium   | ▲ (yellow)   | [!]       |
| Low      | ▽ (gray)     | [-]       |
| Lowest   | ▽▽ (gray)    | [--]      |

### Separators

| Element        | Colored | Text-Only |
|----------------|---------|-----------|
| Card border    | ╭╮╰╯│   | (none)    |
| Section header | ╭─ TITLE ─╮ | === TITLE === |
| Dotted line    | ┄┄┄┄┄   | ......... |
| Solid line     | ────    | --------- |

### Color Palette (ANSI 16)

| Purpose    | Color         | ANSI Code |
|------------|---------------|-----------|
| Key/ID     | Bright Blue   | 12        |
| Done       | Bright Green  | 10        |
| In Progress| Bright Blue   | 12        |
| To Do      | Gray          | 8         |
| Warning    | Bright Yellow | 11        |
| Error      | Bright Red    | 9         |
| Muted text | Gray          | 8         |
| Labels     | Bright Cyan   | 14        |

## Implementation

### Dependencies

Already in use:
- `github.com/charmbracelet/lipgloss` - Styling primitives
- `github.com/charmbracelet/lipgloss/table` - Table rendering

May add:
- `github.com/charmbracelet/glamour` - Markdown rendering (for descriptions)

### Architecture

1. **Theme struct** in `gogh/` - Centralizes style decisions based on NO_COLOR detection
2. **Card renderer** - Reusable component for bordered sections
3. **Update TextPrinter methods** - Apply new layouts to each output type

### Files to Modify

- `gogh/style.go` - Extend with Theme, card builders, separators
- `gogh/text.go` - Update Issue(), Issues(), Transitions(), Links(), Success(), Warning(), Error()

### Detection Logic

```go
func (t *Theme) NoColor() bool {
    return os.Getenv("NO_COLOR") != "" ||
           os.Getenv("CLICOLOR") == "0" ||
           termenv.EnvColorProfile() == termenv.Ascii
}
```

## Validation

### Behavioral

- `j4c issue view` renders card layout in color mode
- `NO_COLOR=1 j4c issue view` renders clean text-only layout
- All outputs maintain 80-column width
- Markdown in descriptions preserved and readable

### Quality

- All tests pass
- No linting errors
- Manual visual inspection in both modes
