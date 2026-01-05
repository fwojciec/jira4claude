# jira4claude

A minimal Jira CLI designed for AI coding agents.

[![Go Reference](https://pkg.go.dev/badge/github.com/fwojciec/jira4claude.svg)](https://pkg.go.dev/github.com/fwojciec/jira4claude)
[![Go Report Card](https://goreportcard.com/badge/github.com/fwojciec/jira4claude)](https://goreportcard.com/report/github.com/fwojciec/jira4claude)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

![Demo of Claude Code using j4c to list and view Jira tasks](assets/demo.gif)

## Why?

AI coding agents run in non-interactive contexts where prompts hang and complex output is hard to parse. This CLI is designed from first principles for that environment:

- **Never prompts** - missing required flags produce errors, not interactive prompts
- **Markdown output** - valid GFM that AI reads directly, humans pipe to `glow`
- **Minimal scope** - ~11 commands covering what agents actually need
- **Unix philosophy** - line-oriented output that composes with `fzf`, `grep`, `jq`
- **Predictable structure** - same input always produces same output format

## Quick Start

```bash
# Install
go install github.com/fwojciec/jira4claude/cmd/j4c@latest

# Configure credentials (~/.netrc)
echo "machine yourcompany.atlassian.net
  login your-email@example.com
  password your-api-token" >> ~/.netrc

# Initialize project config
j4c init --server=https://yourcompany.atlassian.net --project=PROJ

# Start working
j4c issue list --assignee=me
j4c issue view PROJ-123
```

Get an API token from [Atlassian Account Settings](https://id.atlassian.com/manage-profile/security/api-tokens).

## Output Modes

### Markdown (Default)

All output is valid GitHub-Flavored Markdown:

```bash
j4c issue view J4C-81
```

```markdown
# J4C-81: Implement linked issues display

**Type:** Task
**Status:** Done
**Priority:** Medium
**Assignee:** Filip Wojciechowski

## Context

Add linked issues section to issue view output...

## Linked Issues

**is blocked by:**
- **J4C-74** [Done] Inject Converter into CLI IssueContext
- **J4C-76** [Done] Add warning propagation

**blocks:**
- **J4C-78** [Done] Rename adf package

[View in Jira](https://company.atlassian.net/browse/J4C-81)
```

Issue lists are line-oriented for easy piping:

```bash
j4c issue list --limit=3
```

```
- **J4C-110** [To Do] [P2] Standardize issue list item format
- **J4C-95** [To Do] [P2] Investigate epic support
- **J4C-66** [To Do] [P2] Create Homebrew tap
```

### JSON Mode

Add `--json` for programmatic access:

```bash
j4c issue view PROJ-123 --json
```

```json
{
  "key": "PROJ-123",
  "summary": "Fix login timeout",
  "status": "In Progress",
  "type": "Bug",
  "priority": "High",
  "assignee": "Alice",
  "url": "https://company.atlassian.net/browse/PROJ-123"
}
```

## Unix Composability

The markdown format is designed for composition with standard tools:

```bash
# Pretty rendering
j4c issue view J4C-123 | glow

# Interactive selection
j4c issue list | fzf

# Search issues
j4c issue list | grep "bug"

# Extract keys programmatically
j4c issue list --json | jq -r '.[].key'

# Batch transition
j4c issue list --status="In Progress" --json | \
  jq -r '.[].key' | \
  xargs -I{} j4c issue transition {} --status="Done"
```

| Format | Use Case | Pipe to |
|--------|----------|---------|
| Markdown | Human/AI reading, interactive selection | `glow`, `fzf`, `grep` |
| JSON | Programmatic extraction, scripting | `jq`, `xargs`, scripts |

## Markdown and Jira

Jira stores content in Atlassian Document Format (ADF), not markdown. This CLI handles conversion automatically:

- **Input**: Write descriptions and comments in GitHub-Flavored Markdown
- **Storage**: CLI converts to ADF when sending to Jira API
- **Output**: CLI converts ADF back to markdown when displaying

Unsupported elements (like embedded images) generate warnings but don't block operations.

## Commands

### Issue Operations

```bash
j4c issue view PROJ-123                    # View single issue
j4c issue list --project=PROJ              # List issues
j4c issue list --status="In Progress"      # Filter by status
j4c issue list --assignee=me               # Filter by assignee
j4c issue list --labels=urgent,backend     # Filter by labels
j4c issue list --jql="priority = High"     # Raw JQL query
j4c issue ready                            # Issues with no blockers
j4c issue create --summary="Title"         # Create issue
j4c issue update PROJ-123 --priority=High  # Update issue
j4c issue transitions PROJ-123             # List available transitions
j4c issue transition PROJ-123 --status="Done"
j4c issue assign PROJ-123 --account-id=... # Assign issue
j4c issue comment PROJ-123 --body="Done"   # Add comment
```

### Link Operations

```bash
j4c link create PROJ-1 Blocks PROJ-2       # PROJ-1 blocks PROJ-2
j4c link list PROJ-123                     # List links
j4c link delete PROJ-1 PROJ-2              # Remove link
```

### Configuration

```bash
j4c init --server=https://... --project=PROJ
```

Creates `.jira4claude.yaml` in current directory.

## Exit Codes

Semantic exit codes for programmatic error handling:

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | Validation error (invalid input) |
| 2 | Authentication failed |
| 3 | Permission denied |
| 4 | Resource not found |
| 5 | Conflict (e.g., duplicate) |
| 6 | Rate limit exceeded |
| 7 | Internal/unexpected error |

## For AI Agent Developers

Design decisions that make this CLI agent-friendly:

1. **No interactivity** - Commands never prompt. Missing required flags produce errors.
2. **Predictable output** - Same input always produces same output structure.
3. **Semantic exit codes** - Handle errors programmatically without parsing messages.
4. **Markdown by default** - AI can read directly; humans pipe to `glow`.
5. **JSON for extraction** - `--json` flag for programmatic field access.
6. **Line-oriented lists** - Works naturally with `fzf`, `grep`, line-based tools.

## Installation

### From Source

```bash
go install github.com/fwojciec/jira4claude/cmd/j4c@latest
```

### From Releases

Download binaries from the [releases page](https://github.com/fwojciec/jira4claude/releases).

## License

MIT
