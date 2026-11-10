# ticket (vendored)

Vendored copy of [wedow/ticket](https://github.com/wedow/ticket) — a minimal
ticket system with dependency tracking. One POSIX bash script; tickets are
markdown files with YAML frontmatter in `.tickets/` at the repo root.

- Vendored from upstream commit `194b71a8bbc3771da1ce9f579395937c976bbddc`
  (MIT, see LICENSE); copied into jira4claude on 2026-11-09 from a copy
  audited on 2026-07-28, unchanged.
- Audited before vendoring: writes confined to `.tickets/`; no network, no
  eval, no git mutations (reads `git config user.name` only).
- We do not track upstream. Update deliberately by re-vendoring and
  re-auditing; local patches to this copy are fine.
- Plugin note: the script executes `tk-*`/`ticket-*` commands from PATH
  before built-ins. We install no plugins; `ticket super <cmd>` bypasses.

Usage (from the repo root of the main checkout):

```
tools/ticket/ticket create "Title" -t task --parent <epic-id> -d "..." --acceptance "..."
tools/ticket/ticket dep <id> <blocker-id>
tools/ticket/ticket ready        # unblocked work frontier
tools/ticket/ticket show <id>    # the contract: description and acceptance criteria
tools/ticket/ticket dep cycle    # run after dependency edits (not enforced on write)
```

Convention here: one epic per milestone, milestone tasks as children via
`--parent`, blocking edges with `dep`. `docs/plans/` stays the spec; tickets
are the work graph and each task's contract. Ids take the `jir-` prefix from
the directory name, so create tickets from the main checkout, not a worktree.
