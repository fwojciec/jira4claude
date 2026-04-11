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

#### Phase 1: Select & Setup

- Pre-flight: must be on `main` with clean working tree
- If argument provided, use that issue number directly
- Otherwise list open issues: `gh issue list --state open --json number,title,labels --limit 20`
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
   - Find downstream issues: `gh issue list --search "blocked by #$ISSUE_NUMBER" --state open`
   - Get milestone: `gh issue view $ISSUE_NUMBER --json milestone --jq '.milestone.title // empty'`
   - If milestone exists, find related issues: `gh issue list --milestone "$MILESTONE" --state open`
   - For each genuinely related issue where the current work provides useful
     context, post a comment with: branch name, APPROVE/REJECT status,
     what was implemented, impact on downstream issue, implementation guidance
   - Skip comments when relationship is superficial

**Track B -- Codex review:**

Via `codex exec` (same approach as quiver's `codex-review` skill):

1. Detects staged or committed changes
2. Reads CLAUDE.md and REVIEW_CRITERIA.md for project standards
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
- **Dependencies**: "Depends on #N" convention in issue body; checking if deps
  are closed before marking ready. Include both text-based parsing and
  `gh api` dependency endpoints.
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
  `gh issue list` for finding ready work
- **Keep unchanged**: architecture patterns, test philosophy, linting, LSP
  tools, file naming conventions

### `.claude/commands/address-pr-comments.md`

Minimal changes:
- Ensure `gh api` patterns match `gh-workflow` skill conventions
- No Jira references to remove (already GitHub-native)

## Concept Mapping Reference

| Jira concept | GitHub equivalent |
|---|---|
| `j4c issue list` | `gh issue list` |
| `j4c issue view` | `gh issue view` |
| `j4c issue create` | `gh issue create` |
| `j4c issue transition --status="Done"` | `gh issue close` |
| `j4c issue transition --status="Start Progress"` | Labels or implicit (branch exists) |
| `j4c issue ready` | Parse "Depends on #N" + check if deps closed |
| `j4c link create A Blocks B` | "Depends on #N" in issue body |
| `j4c issue comment` | `gh issue comment` |
| JQL queries | GitHub search syntax |
| Issue keys (`J4C-42`) | Issue numbers (`#42`) |
| `.jira4claude.yaml` | Not needed (`gh` uses repo context) |
| Branch name `J4C-42` | Branch name `42-add-description` |
