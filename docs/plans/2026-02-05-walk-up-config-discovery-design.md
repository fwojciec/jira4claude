# Walk-Up Config Discovery

Design for improving config discovery to walk up the directory tree.

## Problem

Currently `DiscoverConfig` checks exactly two locations: the current working directory, then the home directory. This fails in monorepo setups where the config lives at the repo root but the agent starts work in a subdirectory (e.g., `repo/services/billing/`).

## Solution

Change `DiscoverConfig` to walk up from the working directory, checking each ancestor for `.jira4claude.yaml`. First match wins. Home directory remains a final fallback if the walk didn't pass through it.

## Discovery Order

1. **`--config` flag** — Explicit path, highest priority (unchanged)
2. **Walk up from CWD** — Check each directory from the working directory upward for `.jira4claude.yaml`. First match wins. Walk stops at the filesystem root.
3. **Home directory fallback** — If the walk didn't pass through `~`, check `~/.jira4claude.yaml` as a final fallback.

### Monorepo Example

```
~/code/monorepo/.jira4claude.yaml      ← config lives here
~/code/monorepo/services/billing/      ← agent starts here
```

Discovery walks: `services/billing/` → `services/` → `monorepo/` → found.

### Override Example

```
~/code/monorepo/.jira4claude.yaml                   ← project: PLATFORM
~/code/monorepo/services/billing/.jira4claude.yaml  ← project: BILLING
~/code/monorepo/services/billing/src/               ← agent starts here
```

Discovery walks: `src/` → `billing/` → found (BILLING). Never sees the root config.

No merging — the closest config file wins entirely.

## Implementation

**Files to modify:**

| File | Change |
|------|--------|
| `yaml/config.go` | Replace two-location check with walk-up loop in `DiscoverConfig` |
| `yaml/config_test.go` | Add test cases for nested discovery, override, home fallback |

**Updated `DiscoverConfig` logic:**

```go
func DiscoverConfig(workDir, homeDir string) (string, error) {
    dir := workDir
    for {
        path := filepath.Join(dir, ".jira4claude.yaml")
        if fileExists(path) {
            return path, nil
        }
        parent := filepath.Dir(dir)
        if parent == dir {
            break // reached filesystem root
        }
        dir = parent
    }
    // Fallback: home directory (if walk didn't already cover it)
    if !isSubdir(workDir, homeDir) {
        path := filepath.Join(homeDir, ".jira4claude.yaml")
        if fileExists(path) {
            return path, nil
        }
    }
    return "", ErrConfigNotFound
}
```

## Test Cases

1. **Config in CWD** — Works as before (no regression)
2. **Config in parent directory** — CWD is `repo/services/billing/`, config at `repo/`
3. **Config several levels up** — CWD is `repo/a/b/c/d/`, config at `repo/`
4. **Override in subdirectory** — Config at both `repo/` and `repo/services/billing/`, deeper one wins
5. **Home directory fallback** — CWD is outside home (e.g., `/tmp/x/`), no config in tree, falls back to `~/`
6. **No config anywhere** — Returns error (unchanged)
7. **`--config` flag trumps all** — Explicit path skips discovery (unchanged, existing test)

## Not Changing

- `Config` struct (still `server` + `project`)
- `LoadConfig` function signature
- Config file format (`.jira4claude.yaml`, YAML)
- `Init` command behavior
- `~/.netrc` authentication
- `--config` flag behavior
