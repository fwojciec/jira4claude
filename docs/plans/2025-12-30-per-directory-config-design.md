# Per-Directory Config Support

Design for J4C-17: Support per-directory config files.

## Problem

Currently the CLI only looks for `~/.jira4claude.yaml` by default. Different projects need different Jira configurations (different servers, different project keys).

## Solution

Add automatic config discovery with local-first precedence, plus an `init` command for easy setup.

## Config Discovery Logic

**When `--config` flag is provided:**
- Use only that path, no fallback

**When `--config` not provided (empty default):**
1. Check `./.jira4claude.yaml` (current directory)
2. Check `~/.jira4claude.yaml` (home directory)
3. If neither exists, error with actionable message

**Error message when no config found:**
```
Error: no config file found
Searched: ./.jira4claude.yaml, ~/.jira4claude.yaml
Run: jira4claude init --server=URL --project=KEY
```

## Init Command

New subcommand: `jira4claude init`

**Required flags:**
- `--server` - Jira server URL (e.g., `https://example.atlassian.net`)
- `--project` - Default project key (e.g., `J4C`)

**Behavior:**
1. Error if `.jira4claude.yaml` already exists
2. Create `.jira4claude.yaml` with provided values
3. Add `.jira4claude.yaml` to `.gitignore` (create if needed)
4. Output what was done

**Example:**
```bash
jira4claude init --server=https://fwojciec.atlassian.net --project=J4C
```

**Output:**
```
Created .jira4claude.yaml
Added .jira4claude.yaml to .gitignore
```

## Implementation

**Files to modify:**

| File | Change |
|------|--------|
| `yaml/config.go` | Add `DiscoverConfig()` function |
| `yaml/config_test.go` | Add discovery tests |
| `cmd/jira4claude/main.go` | Add `InitCmd`, change `--config` default to empty, call `DiscoverConfig()` when empty |
| `.claude/skills/jira-workflow/SKILL.md` | Remove `--config` from commands, add "Handling Missing Config" section |

**New function in `yaml/config.go`:**
```go
// DiscoverConfig searches for config files in standard locations.
// Returns the path to the first config file found.
// Search order: ./.jira4claude.yaml, ~/.jira4claude.yaml
func DiscoverConfig() (string, error)
```

## Testing

**Config discovery tests:**
- Local config exists → returns local path
- Only global config exists → returns global path
- Neither exists → returns `ENotFound` error
- Both exist → returns local path (precedence)

**Init command tests:**
- Creates `.jira4claude.yaml` with correct content
- Adds entry to existing `.gitignore`
- Creates `.gitignore` if missing
- Errors if `.jira4claude.yaml` already exists
- Errors if required flags missing

## Jira-Workflow Skill Updates

1. Remove `--config=.jira4claude.yaml` from all command examples
2. Add "Handling Missing Config" section explaining the `init` command
3. Update Notes section to explain auto-discovery

## Not Changing

- `yaml.LoadConfig()` function signature (still takes explicit path)
- Config file format
- Any other CLI command behavior
