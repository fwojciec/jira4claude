# Jira to GitHub Migration

**Date**: 2026-04-10
**Status**: Approved

## Context

The free Jira account used for project management has expired. All workflow
infrastructure (skills, commands, CLAUDE.md) needs to migrate from Jira-based
(`j4c` CLI) to GitHub-native (`gh` CLI) equivalents. This is also an
opportunity to simplify: consolidate 9 workflow files into 4 and drop
worktree-based parallel development (unused).

## Decisions

- **Single `/work` command** replaces `/start-task`, `/finish-task`, and all
  worktree commands
- **Dual review**: Claude code-review subagent + Codex review via `codex exec`,
  running in parallel
- **Reflect phase** bundled into the Claude review subagent (not a separate
  step in the main agent)
- **`gh-workflow` skill** ported from quiver as the GitHub CLI reference
- **Branch naming**: `<number>-<short-description>` (e.g., `42-add-auth`)
- **No dogfooding section** in CLAUDE.md (Jira account expired)
- **`/address-pr-comments`** kept as a separate command (minor cleanup only)

## Files to Delete

| File | Reason |
|------|--------|
| `.claude/commands/start-task.md` | Replaced by `/work` |
| `.claude/commands/finish-task.md` | Replaced by `/work` |
| `.claude/commands/create-worktree.md` | Worktree workflow removed |
| `.claude/commands/worktree-task.md` | Worktree workflow removed |
| `.claude/commands/worktree-finish.md` | Worktree workflow removed |
| `.claude/commands/worktree-status.md` | Worktree workflow removed |
| `.claude/commands/cleanup-worktrees.md` | Worktree workflow removed |
| `.claude/skills/jira-workflow/SKILL.md` | Replaced by `gh-workflow` |

## Files to Create

### `.claude/commands/work.md`

Single end-to-end workflow command.

#### Frontmatter

```yaml
---
description: Pick a GitHub issue, implement with TDD, review, and create PR
allowed-tools: Bash(gh:*), Bash(git:*), Bash(make:*), Bash(jq:*), Bash(node:*), Bash(codex:*)
---
```

#### Phase 0: Detect Resume

Before anything else, check if already on a feature branch:

- If branch matches `<number>-<description>` pattern:
  - Extract issue number, fetch issue details
  - Determine state: uncommitted changes → resume at Phase 2,
    clean with commits → resume at Phase 3 (review),
    PR already exists → report status and offer to address comments
  - Ask user to confirm resume or start fresh
- If on `main`: proceed to Phase 1

#### Phase 1: Select & Setup

- Pre-flight: must be on `main` with clean working tree
- If argument provided, use that issue number directly
- Otherwise list open issues and filter to ready (unblocked) work:
  1. `gh issue list --state open --json number,title,labels --limit 50`
  2. For each issue, check for open blockers:
     `gh api repos/{owner}/{repo}/issues/<number>/dependencies/blocked_by --jq 'map(select(.state == "open")) | length'`
     (dependency records persist after a blocker is closed, so filter by state)
  3. An issue is "ready" if it has zero open blockers. Filter out blocked
     issues before presenting.
  4. Present only ready issues to the user.
- User picks an issue
- View issue details + comments: `gh issue view <number> --comments`
- Create branch: `git checkout -b <number>-<short-description>`

#### Phase 2: Implement

- TDD-first using `superpowers:test-driven-development` for behavioral code
  (functions with logic, modules with behavior, integration points, error
  handling). Skip TDD for type definitions, data structures, config, boilerplate.
- Run `make validate` iteratively
- Commit meaningful chunks as you go

#### Phase 3: Review (two parallel tracks)

**Track A -- Claude code-review subagent:**

Runs as a Task subagent with `subagent_type="superpowers:code-reviewer"`.

1. Reviews diff for code quality, structural discipline, CLAUDE.md standards
2. Returns APPROVE/REJECT verdict with structured feedback
3. Runs reflect phase:
   - Extract issue number from branch name
   - Find open downstream issues via dependency API:
     `gh api repos/{owner}/{repo}/issues/$ISSUE_NUMBER/dependencies/blocking --jq 'map(select(.state == "open"))'`
   - Get milestone: `gh issue view $ISSUE_NUMBER --json milestone --jq '.milestone.title // empty'`
   - If milestone exists, find related issues: `gh issue list --milestone "$MILESTONE" --state open`
   - For each genuinely related issue where the current work provides useful
     context, post a comment with: branch name, APPROVE/REJECT status,
     what was implemented, impact on downstream issue, implementation guidance
   - Skip comments when relationship is superficial

**Track B -- Codex review:**

Via `codex exec` (same approach as quiver's `codex-review` skill).
Graceful degradation: if `codex` is not installed or not authenticated,
skip this track and warn the user. Do not block on Codex availability.

1. Detects staged or committed changes
2. Reads CLAUDE.md for project standards (this repo has no REVIEW_CRITERIA.md)
3. Reads changed files, runs `make validate`
4. If issue number available, evaluates against issue's validation criteria
5. Returns structured verdict

**Processing results:**

- Both APPROVE: proceed to Phase 4 (do not stop to ask)
- APPROVE with suggestions: apply judgment, minor fixes inline, proceed
- Any REJECT: present blocking issues to user via AskUserQuestion

Use `superpowers:receiving-code-review` to evaluate each suggestion on merit.

#### Phase 4: PR & Close

- Push branch: `git push -u origin <branch-name>`
- Create PR:
  ```
  gh pr create --title "<title>" --body "
  ## Summary
  <2-3 bullets>

  Closes #<issue-number>

  ## Test Plan
  - [ ] `make validate` passes
  - [ ] <additional verification>

  Generated with [Claude Code](https://claude.com/claude-code)
  "
  ```
- Offer merge options:
  1. "Merge it" -> `gh pr merge --squash --delete-branch`
  2. "I'll merge it myself" -> return to main
  3. "Keep working" -> stay on branch

### `.claude/skills/gh-workflow/SKILL.md`

Ported from quiver with adjustments for this project.

**Sections:**

- **Issue CRUD**: `gh issue list/view/create/edit` patterns
- **Issue creation template** (mandatory):
  - Context: WHAT and WHY (not HOW)
  - Investigation Starting Points: file/code references
  - Scope Constraints: what's NOT in scope
  - Validation Requirements: testable acceptance criteria
- **Dependencies**: GitHub dependency API as single source of truth.
  - Check blockers: `gh api repos/{owner}/{repo}/issues/N/dependencies/blocked_by`
  - Find downstream: `gh api repos/{owner}/{repo}/issues/N/dependencies/blocking`
  - Create dependency: first fetch the blocker's REST id via
    `gh api repos/{owner}/{repo}/issues/<blocker-number> --jq '.id'`
    (note: this is the numeric REST id, NOT the GraphQL node_id string),
    then POST: `gh api repos/{owner}/{repo}/issues/<blocked-number>/dependencies/blocked_by -f issue_id=<REST-id>`
  - Delete dependency: `gh api -X DELETE repos/{owner}/{repo}/issues/<number>/dependencies/blocked_by/<REST-id>`
  - "Depends on #N" text in issue body is optional human documentation,
    not used for programmatic dependency resolution.
- **Milestones**: `gh api` for creating/listing, `--milestone` flag on issues
- **PR comment replies**: inline reply syntax with
  `gh api repos/{owner}/{repo}/pulls/<pr>/comments`
- **Formatting**: always use GFM in issue bodies and comments

## Files to Update

### `CLAUDE.md`

- **Remove**: "Dogfooding" section entirely
- **Update "Workflows"** section to:

  | Command | Purpose |
  |---------|---------|
  | `/work` | Pick a GitHub issue, implement with TDD, review, create PR |
  | `/address-pr-comments` | Fetch, evaluate, and respond to PR feedback |

- **Update "Skills"** section: replace `jira-workflow` reference with
  `gh-workflow`
- **Update "Planning workflow"**: reference `gh-workflow` skill, use
  dependency-filtered issue listing for finding ready work (see Phase 1
  of `/work` for the concrete algorithm)
- **Keep unchanged**: architecture patterns, test philosophy, linting, LSP
  tools, file naming conventions

### `.claude/commands/address-pr-comments.md`

Minimal changes:
- Ensure `gh api` patterns match `gh-workflow` skill conventions
- No Jira references to remove (already GitHub-native)

### `README.md`

- Remove the "Claude Code Skill" section (lines 181-195) that tells users
  to install the jira-workflow skill via curl
- Replace with equivalent gh-workflow installation instructions, or remove
  entirely if the skill is project-specific and not meant for external use

## Concept Mapping Reference

| Jira concept | GitHub equivalent |
|---|---|
| `j4c issue list` | `gh issue list` |
| `j4c issue view` | `gh issue view` |
| `j4c issue create` | `gh issue create` |
| `j4c issue transition --status="Done"` | `gh issue close` |
| `j4c issue transition --status="Start Progress"` | Labels or implicit (branch exists) |
| `j4c issue ready` | List open issues, then for each check `gh api repos/{owner}/{repo}/issues/<number>/dependencies/blocked_by --jq 'map(select(.state == "open")) \| length'`; ready if 0 |
| `j4c link create A Blocks B` | First get REST id: `gh api repos/{owner}/{repo}/issues/<blocker-number> --jq '.id'`, then `gh api repos/{owner}/{repo}/issues/<blocked-number>/dependencies/blocked_by -f issue_id=<REST-id>` |
| `j4c issue comment` | `gh issue comment` |
| JQL queries | GitHub search syntax |
| Issue keys (`J4C-42`) | Issue numbers (`#42`) |
| `.jira4claude.yaml` | Not needed (`gh` uses repo context) |
| Branch name `J4C-42` | Branch name `42-add-description` |
