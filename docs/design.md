# jira4claude - Design Document

## Problem Statement

The official jira-cli hangs in non-interactive contexts (like Claude Code) because it prompts for optional fields. The `--no-input` flag works but must be remembered for every command. A PR (#905) exists to add config support, but it's been pending for months.

More fundamentally, jira-cli is designed for human interaction with rich TUI features. AI agents need something different: predictable, non-interactive commands with structured output.

## Goals

1. **Zero interactivity** - Never prompt, never hang
2. **Readable commands** - `--project=X --type=Bug` not `-d '{"fields":{...}}'`
3. **Readable output** - Clean, scannable output for humans watching Claude work
4. **Structured output** - `--json` flag for programmatic use
5. **Minimal scope** - Only commands agents actually need
6. **Simple auth** - netrc, same as jira-cli

## Non-Goals

- Feature parity with jira-cli
- Interactive TUI features
- Broad API coverage (417 endpoints exist, we need ~10)

## Research Findings

### Jira Cloud REST API

- **OpenAPI spec**: https://developer.atlassian.com/cloud/jira/platform/swagger-v3.v3.json
- **417 total endpoints**, but core issue operations are ~30
- **Auth**: Basic auth with API token (netrc compatible)

### API Patterns

**Request pattern (create/update):**
```json
{
  "fields": {
    "project": {"key": "INT"},
    "issuetype": {"name": "Bug"},
    "summary": "Title",
    "description": {...}  // ADF format
  }
}
```

**Response pattern (create):**
```json
{
  "id": "10001",
  "key": "INT-123",
  "self": "https://..."
}
```

**Response pattern (search/list):**
```json
{
  "issues": [...],
  "total": 50,
  "maxResults": 20,
  "startAt": 0
}
```

**Error pattern:**
```json
{
  "errorMessages": ["Summary is required"],
  "errors": {
    "project": "Project 'XYZ' does not exist"
  }
}
```

### Key Endpoints for Agents

| Endpoint | Method | Use Case |
|----------|--------|----------|
| `/rest/api/3/issue` | POST | Create issue |
| `/rest/api/3/issue/{key}` | GET | View issue |
| `/rest/api/3/issue/{key}` | PUT | Edit issue |
| `/rest/api/3/search/jql` | GET | List/search issues |
| `/rest/api/3/issue/{key}/comment` | POST | Add comment |
| `/rest/api/3/issue/{key}/transitions` | GET | Get available transitions |
| `/rest/api/3/issue/{key}/transitions` | POST | Transition issue (change status) |
| `/rest/api/3/issue/{key}/assignee` | PUT | Assign issue |

## Proposed CLI Design

### Command Structure

```bash
jira4claude <resource> <action> [flags]
```

### Core Commands

```bash
# Create issue
jira4claude issue create \
  --project=INT \
  --type=Bug \
  --summary="Fix login timeout" \
  --description="Users are experiencing..."

# View issue
jira4claude issue view INT-123

# List issues
jira4claude issue list --project=INT --status=Open --assignee=me

# Edit issue
jira4claude issue edit INT-123 --summary="New title" --priority=High

# Add comment
jira4claude issue comment INT-123 --body="This is fixed in PR #456"

# Transition (change status)
jira4claude issue transition INT-123 --status="In Progress"

# Assign
jira4claude issue assign INT-123 --assignee="user@example.com"
```

### Flag Conventions

| API Field | CLI Flag |
|-----------|----------|
| `fields.project.key` | `--project` |
| `fields.issuetype.name` | `--type` |
| `fields.summary` | `--summary` |
| `fields.description` | `--description` |
| `fields.priority.name` | `--priority` |
| `fields.assignee.accountId` | `--assignee` |
| `fields.labels` | `--label` (repeatable) |

### Output Modes

**Default (human-readable):**
```
Created: INT-123
https://acuitymd.atlassian.net/browse/INT-123
```

**JSON mode (`--json`):**
```json
{"key": "INT-123", "id": "10001", "self": "https://..."}
```

**List output (human-readable):**
```
KEY       STATUS      ASSIGNEE    SUMMARY
INT-123   Open        @alice      Fix login timeout
INT-124   In Progress @bob        Add dark mode
INT-125   Done        @alice      Update docs
```

### Error Handling

**Human-readable errors:**
```
Error: Summary is required
Error: Project 'XYZ' does not exist
```

**JSON errors (`--json`):**
```json
{"error": true, "messages": ["Summary is required"]}
```

## Implementation Approach

### Option A: Hand-written (Recommended for MVP)

- Single Go file, ~500 lines
- No dependencies beyond stdlib
- Direct HTTP calls with net/http (reads netrc automatically)
- Hardcoded for the ~8 endpoints we need

**Pros:** Simple, fast to build, easy to maintain
**Cons:** Manual work to add new endpoints

### Option B: Generated from OpenAPI

- Parse OpenAPI spec at build time
- Generate command structure from paths
- Generate flags from request schemas
- Generate output formatters from response schemas

**Pros:** Automatic, scales to more endpoints
**Cons:** More complex, may over-generate

### Recommendation

Start with **Option A** for MVP. The 8 core commands can be built in a day. If we find ourselves needing many more endpoints, we can explore generation later.

## Authentication

Use netrc, same as jira-cli:

```
# ~/.netrc
machine acuitymd.atlassian.net
  login user@example.com
  password <api-token>
```

Go's `net/http` reads netrc automatically when configured.

## Configuration

Minimal config file (`~/.jira4claude.yaml` or env vars):

```yaml
server: https://acuitymd.atlassian.net
project: INT  # default project
```

Or via environment:
```bash
export JIRA_SERVER=https://acuitymd.atlassian.net
export JIRA_PROJECT=INT
```

## Open Questions

1. **ADF (Atlassian Document Format)**: Descriptions use ADF, not plain text. Accept plain text and convert? Or require ADF?

2. **Field discovery**: How to handle custom fields? Ignore for MVP?

3. **Pagination**: For `issue list`, auto-paginate or require explicit `--limit`/`--offset`?

4. **Transitions**: Status names vary by project. Need to fetch available transitions first?

## Next Steps

1. [ ] Decide on MVP scope (which 5-8 commands)
2. [ ] Implement basic HTTP client with netrc auth
3. [ ] Implement `issue create` as proof of concept
4. [ ] Add remaining core commands
5. [ ] Add `--json` output mode
6. [ ] Test with Claude Code
