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
- `github.com/muesli/termenv` - Terminal capability detection

May add:
- `github.com/charmbracelet/glamour` - Markdown rendering (for descriptions)

### Architecture

Following Charm ecosystem best practices (from mods, soft-serve, glow):

1. **Theme struct** - Color palette definition, separate from styles
2. **Styles struct** - All lipgloss styles, grouped by component
3. **Render functions** - Pure functions for each output type (not component structs)
4. **Factory function** - Creates Styles with renderer injection, decides borders at construction

### File Structure

```
gogh/
  theme.go      # Theme (colors) + Styles struct + NewStyles() factory
  render.go     # Render functions: RenderCard(), RenderBadge(), RenderStatus(), etc.
  text.go       # TextPrinter uses Styles + render functions
  json.go       # JSONPrinter (unchanged)
  style.go      # (remove or merge into theme.go)
```

### Core Types

```go
// theme.go

// Theme defines the color palette
type Theme struct {
    Primary   lipgloss.AdaptiveColor
    Success   lipgloss.Color
    Warning   lipgloss.Color
    Error     lipgloss.Color
    Muted     lipgloss.AdaptiveColor
}

// Styles contains all application styles
type Styles struct {
    Theme   Theme
    NoColor bool
    Width   int

    // Component styles
    Card struct {
        Border  lipgloss.Style
        Title   lipgloss.Style
        Content lipgloss.Style
    }

    Badge struct {
        Done       lipgloss.Style
        InProgress lipgloss.Style
        ToDo       lipgloss.Style
    }

    // Indicators (text varies by mode)
    Indicators struct {
        StatusDone       string  // "✓" or "[x]"
        StatusInProgress string  // "▶" or "[>]"
        StatusToDo       string  // "○" or "[ ]"
        Arrow            string  // "→" or "->"
        Success          string  // "✓" or "[ok]"
        Warning          string  // "⚠" or "[warn]"
        Error            string  // "✗" or "[error]"
    }

    // Separators
    Separators struct {
        Dotted string  // "┄┄┄" or "..."
        Solid  string  // "───" or "---"
    }
}

// NewStyles creates styles based on terminal capabilities
func NewStyles(r *lipgloss.Renderer) *Styles {
    profile := termenv.EnvColorProfile()
    noColor := profile == termenv.Ascii

    // Select borders and indicators based on mode
    // ...
}

// DefaultStyles creates styles for stdout with auto-detection
func DefaultStyles() *Styles {
    return NewStyles(lipgloss.DefaultRenderer())
}
```

### Render Functions

```go
// render.go

// RenderCard creates a bordered card (or plain section in text-only mode)
func RenderCard(s *Styles, title, content string) string

// RenderIssueHeader creates the issue header card
func RenderIssueHeader(s *Styles, view IssueView) string

// RenderLinksCard creates the linked issues card
func RenderLinksCard(s *Styles, links []LinkView) string

// RenderStatusBadge formats status with indicator
func RenderStatusBadge(s *Styles, status string) string

// RenderPriorityBadge formats priority with indicator
func RenderPriorityBadge(s *Styles, priority string) string
```

### Detection Logic

Mode detection happens once at Styles construction via termenv:

```go
func NewStyles(r *lipgloss.Renderer) *Styles {
    profile := termenv.EnvColorProfile()
    noColor := profile == termenv.Ascii

    s := &Styles{
        NoColor: noColor,
        Width:   80,
    }

    // Select border based on mode
    if noColor {
        // No borders in text-only mode
        s.Card.Border = r.NewStyle()
    } else {
        s.Card.Border = r.NewStyle().
            Border(lipgloss.RoundedBorder()).
            BorderForeground(lipgloss.Color("240"))
    }

    // Select indicators based on mode
    if noColor {
        s.Indicators.StatusDone = "[x]"
        s.Indicators.Arrow = "->"
        // ...
    } else {
        s.Indicators.StatusDone = "✓"
        s.Indicators.Arrow = "→"
        // ...
    }

    return s
}
```

This approach:
- Detects capabilities once at startup (not per-render)
- Injects renderer for testability
- Uses pure render functions (easy to test, no state)
- Follows patterns from Charm's production codebases

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
