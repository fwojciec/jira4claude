# jira4claude

A minimal Jira CLI designed for AI coding agents.

[![Go Reference](https://pkg.go.dev/badge/github.com/fwojciec/jira4claude.svg)](https://pkg.go.dev/github.com/fwojciec/jira4claude)
[![Go Report Card](https://goreportcard.com/badge/github.com/fwojciec/jira4claude)](https://goreportcard.com/report/github.com/fwojciec/jira4claude)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

![Demo of Claude Code using j4c to list and view Jira tasks](assets/demo.gif)

## Why?

AI agents need CLI tools designed for non-interactive contexts:

- **Never prompt** for input or hang waiting for user interaction
- **Produce predictable output** that can be parsed programmatically
- **Have explicit flags** instead of interactive menus

This CLI is built specifically for that use case.

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

## Installation

### From Source

```bash
go install github.com/fwojciec/jira4claude/cmd/j4c@latest
```

### From Releases

Download binaries from the [releases page](https://github.com/fwojciec/jira4claude/releases).

## Commands

### Issue Operations

```bash
# View an issue
j4c issue view PROJ-123

# List issues with filters
j4c issue list --project=PROJ --status="In Progress" --assignee=me

# List issues ready to work on (no blockers)
j4c issue ready --project=PROJ

# Create an issue
j4c issue create --project=PROJ --type=Bug --summary="Fix login timeout"

# Update an issue
j4c issue update PROJ-123 --priority=High --labels=urgent

# Transition an issue to a new status
j4c issue transitions PROJ-123           # List available transitions
j4c issue transition PROJ-123 --status="Done"

# Assign an issue
j4c issue assign PROJ-123 --account-id=user-account-id

# Add a comment
j4c issue comment PROJ-123 --body="Fixed in PR #456"
```

### Link Operations

```bash
# Create a dependency link
j4c link create PROJ-123 Blocks PROJ-124

# List links for an issue
j4c link list PROJ-123

# Delete a link
j4c link delete PROJ-123 PROJ-124
```

## Output Modes

### Human-Readable (Default)

Clean, scannable output for humans watching agents work:

```
j4c issue list --project=J4C --limit=3
```

```
KEY     STATUS       TYPE  PRIORITY  SUMMARY
J4C-62  In Progress  Task  Medium    Create comprehensive README.md
J4C-61  Done         Task  Medium    Add semantic exit codes
J4C-60  Done         Task  Medium    Implement link commands
```

### JSON Mode

Structured output for programmatic use:

```
j4c issue view PROJ-123 --json
```

```json
{
  "key": "PROJ-123",
  "project": "PROJ",
  "summary": "Fix login timeout",
  "description": "Users experience timeout after 30s",
  "status": "In Progress",
  "type": "Bug",
  "priority": "High",
  "assignee": {
    "accountId": "user-id",
    "displayName": "Alice",
    "email": "alice@example.com"
  },
  "reporter": { ... },
  "labels": ["urgent"],
  "links": [ ... ],
  "comments": [ ... ],
  "created": "2025-01-15T10:30:00-08:00",
  "updated": "2025-01-15T14:22:00-08:00",
  "url": "https://yourcompany.atlassian.net/browse/PROJ-123"
}
```

*Some fields abbreviated. Full schema available via `--json` output.*

JSON errors include structured information:

```json
{
  "error": true,
  "code": "not_found",
  "message": "Issue PROJ-999 not found"
}
```

## Exit Codes

The CLI uses semantic exit codes for programmatic error handling:

| Code | Constant | Meaning |
|------|----------|---------|
| 0 | - | Success |
| 1 | `EValidation` | Validation error (invalid input) |
| 2 | `EUnauthorized` | Authentication failed |
| 3 | `EForbidden` | Permission denied |
| 4 | `ENotFound` | Resource not found |
| 5 | `EConflict` | Conflict (e.g., duplicate) |
| 6 | `ERateLimit` | Rate limit exceeded |
| 7 | `EInternal` | Internal/unexpected error |

## For AI Agent Developers

This CLI is designed as a tool for AI coding assistants. Key design decisions:

1. **No interactivity**: Commands never prompt. Missing required flags produce errors.
2. **Predictable output**: Same input always produces same output structure.
3. **Semantic exit codes**: Agents can handle errors programmatically without parsing messages.
4. **JSON mode**: `--json` flag on any command produces machine-readable output.
5. **Explicit flags**: All options are explicit flags, not positional arguments that could be ambiguous.

### Example: Claude Code Integration

```bash
# In .claude/commands/jira-task.md
j4c issue view $ISSUE_KEY --json | jq '.description'
```

## Status

Alpha. The API is subject to change. Built to solve a specific workflow problem with AI coding assistants.

## License

MIT
