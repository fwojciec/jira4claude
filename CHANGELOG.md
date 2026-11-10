# Changelog

Changes that affect people and scripts using `j4c`. The README describes the
current behaviour in full.

## v0.4.0 (unreleased): scripted input and output, attachments

### Breaking changes

**Errors go to stderr in `--json` mode.** Each error and warning is one JSON
object per line on stderr, with a `level` key. stdout holds only data, or
nothing when a command fails.

```text
before (stdout):  {"code": "not_found", "error": true, "message": "issue not found"}   (indented over five lines)
after  (stderr):  {"level":"error","error":true,"code":"not_found","message":"issue not found"}
```

Migration: read errors from stderr, one JSON object per line, and select by
`level`.

**Warnings in `--json` mode are JSON.** They were plain text lines starting
with `warning: ` on stderr; now they are objects with `"level":"warning"`,
and conversion warnings also carry `kind`, `node`, `detail` and `token`.

Migration: parse stderr lines as JSON.

**Notices moved from stdout to stderr.** `[info] No issues found`,
`[info] No links for KEY` and `[info] No transitions available for KEY` used
to be printed on stdout. An empty list now prints nothing on stdout in text
mode.

Migration: test for empty output instead of matching the notice.

**Usage errors exit 1 and print usage on stderr.** An unknown flag or a
missing argument used to exit 80 with the usage text on stdout.

```text
before: j4c issue view   ->  usage on stdout, "j4c: error: expected "<key>"" on stderr, exit 80
after:  j4c issue view   ->  usage and error on stderr, stdout empty, exit 1
```

With `--json`, only a JSON error line is printed. Migration: treat exit 1 as
any validation error; stop testing for 80.

**Every write prints a result instead of an `[ok]` line.**

```text
before:
[ok] Updated: PROJ-123
https://example.atlassian.net/browse/PROJ-123

after:
PROJ-123: updated
  summary: Fix login timeout → New
  updated: 2026-11-10T09:00:00.456Z
https://example.atlassian.net/browse/PROJ-123
```

In `--json` mode the old object was
`{"keys": ["PROJ-123"], "message": "Updated:", "success": true, "urls": [...]}`;
the new one is `{"key": ..., "url": ..., "action": "updated", "changes": {...},
"updated": ...}`. A transition prints `PROJ-123: In Progress → In Review`.
`attachment upload` and `attachment delete` print attachment lines.

Migration: read `key` and `action` from the JSON result, or the first word of
the text result.

**List lines are tab-separated.** `issue list` and `issue ready` print key,
status, type, priority, assignee and summary, separated by tabs. Summaries are
no longer cut at 60 characters, and the priority is Jira's name instead of
`P0`..`P4`.

```text
before: - **PROJ-123** [In Progress] [P1] Fix login timeout
after:  PROJ-123	In Progress	Bug	High	Alice Smith	Fix login timeout
```

Migration: split each line on tabs (six columns, empty values as `-`).

**`issue transitions` prints ID, name and target status.**

```text
before: - Start work
after:  11	Start work	In Progress
```

The JSON form gained `to`. Migration: split on tabs; the name is the second
column.

**`issue transition --status` matches the target status first.** It used to
match only the transition's name. Now a transition whose target status is the
value wins over one whose name is the value, and two transitions to the same
status are refused (exit 1) with their IDs listed.

Migration: pass `--id` where the status is ambiguous; a name that is not also
a target status still works.

**`issue comment --body ""` is an error.** An empty or blank body (inline or
from a file) exits 1. It used to be sent to Jira.

Migration: do not send empty comments.

**`--field-json` refuses two cases.** `--field-json description=...` is
refused (it would bypass the protection of images and mentions), and so is a
`--field-json` for a field that a typed flag of the same command also sets,
such as `--field-json priority=... --priority High`.

Migration: use `--description` or `--description-file`; give each field once.

**`issue view` hides empty and machine-only custom fields** in text mode, and
prints `[info] N empty or machine-only custom field(s) hidden — use
--all-fields to show all.` on stderr. `--json` is unchanged.

Migration: pass `--all-fields`, or use `--json` or `--field customfield_N`.

**Comment headers in `issue view` show the comment ID.**

```text
before: **Bob Jones** (2026-11-08 14:02):
after:  **Bob Jones** (2026-11-08 14:02, id 10042):
```

Migration: adjust any pattern matching the header.

**Statuses and dates in a description read as tokens.** A status lozenge read
as bold text and a date as plain text, and writing the description back
turned them into that text, losing the lozenge's colour and the date's
timestamp. They are now tokens like mentions, re-attached on write, and the
token numbers of the nodes after them shift.

```text
before: Now **IN PROGRESS**, due 2026-11-20. Ping {{mention:1 Jane Doe}}
after:  Now {{status:1 IN PROGRESS}}, due {{date:2 2026-11-20}}. Ping {{mention:3 Jane Doe}}
```

Writing a description back now warns, as `degraded`, about the emoji it turns
into text and the smart links it turns into plain links; a smart link that
comes back unchanged is restored. An emoji with neither a character nor a
short name, which read as nothing and was lost on a rewrite, is reported as
`dropped`, so a rewrite of its description is refused unless `--force`.

Migration: read the description again before editing. Markdown saved from an
earlier read of a description holding a status or date is refused: it lacks
their tokens, and its token numbers may name other kinds of node now.
Keep status and date tokens as you keep mentions; leaving one out is refused
unless `--force`, and so is `--clear-description` on a description holding
one.

### Added

- `issue view KEY...` takes several keys. Text pages are a blank line apart;
  `--json` gives an array. A failed key does not stop the others.
- `issue view --field NAME` (repeatable or comma-separated) prints chosen
  fields. One key and one field print the raw value, so
  `issue view KEY --field description > d.md` works.
- `issue view --all-fields` shows every custom field.
- `--description-file`, `--append-description-file` and `--body-file` read
  text from a file, or from standard input for `-`.
- `--dry-run` on `issue create`, `issue update` and `issue comment` prints
  the text as Jira would hold it, with the same checks, and writes nothing.
- Every write reports a typed result: action, each field's old and new value,
  the content removed under `--force`, and the issue's `updated` time after an
  update.
- `issue list` and `issue ready` items include `type` and `assignee`, in text
  and JSON.
- Attachments: `attachment list`, `attachment download` (`--id`, `--name` or
  `--all` into `--dir`, with `--overwrite`), `attachment upload`,
  `attachment delete`, and an attachments section in `issue view`.

### Fixed

- In `--json` mode a usage error was printed as text even when `--json` was
  given; it is now a JSON error line, also when `--json` comes after the bad
  argument.
- Unknown priorities were shown as `P2` in lists; the priority is now printed
  as Jira names it, and an empty one as `-`.

### Known limits

- None of this was run against a live Jira site. Attachment download
  redirects, upload of empty files and duplicate names, and Jira's MIME
  types come from Atlassian's API reference only.
- An upload is not retried after a server error, which may have attached the
  file anyway. Check with `attachment list` before sending it again.
- Downloads have no timeout.
- A new image cannot be placed in a description; upload it as an attachment.
- `attachment delete` results have no issue key: Jira's attachment data does
  not name the issue.
- In text mode, values printed after `==> KEY FIELD <==` headers are not
  escaped; use `--json` for free text.
- The usage line of `issue comment` lists both `--body` and `--body-file` as
  required; exactly one of them is.

## v0.3.0 (unreleased): write safety

### Breaking changes

**Writes that would drop content are refused.** `issue create`,
`issue update --description` and `issue comment` exit 1 and write nothing when
the Markdown has no Jira form, for example an image. Before, the content was
lost without a word:

```text
before: --description "See ![shot](https://x.test/a.png) here"  ->  Jira gets "See shot here"
after:  [error] description would drop unsupported content (Image); remove it. Nothing was written. Pass --force to write anyway, without that content
```

Migration: remove the construct, or pass `--force`.

**Descriptions read with tokens and character references.** Images, other
media, mentions and unknown nodes appear as tokens such as
`{{mention:2 Jane Doe}}` and `{{mediaSingle:1 shot.png}}` instead of
disappearing. Text is escaped so that it reads back exactly: `&#32;` for an
edge space, `&#104;ttps://` for a URL that is not a link, `\<`, and a
backslash at the end of a line for a hard break. `<br>` and HTML comments
show as text.

```text
before: Ping  before merging.
after:  Ping {{mention:2 Jane Doe}} before merging.
```

Migration: keep tokens and references unchanged when you edit and write back
(see "Descriptions as Markdown" in the README).

**Editing a description protects what it holds.** `issue update --description`
reads the issue first and puts each token's original node back. A token left
out is refused unless `--force`; so is clearing a description that holds
images or mentions, and rewriting one that holds content with no Markdown form.

Migration: edit the text `issue view --field description` prints; use
`--append-description` to add text without rewriting.

**`--description ""` is an error.** It used to do nothing and print
`[ok] Updated: KEY`.

Migration: use `--clear-description` to remove a description.

**`issue list --json` and `issue ready --json` return an object.** The items
are also smaller.

```text
before: [{"key": "PROJ-123", "summary": ..., "status": ..., "type": ..., "priority": ..., "relatedIssues": [], "created": ..., "updated": ..., "url": ...}]
after:  {"issues": [{"key": "PROJ-123", "status": ..., "priority": ..., "summary": ...}], "truncated": false}
```

Migration: read `.issues[]` instead of `.[]`; use `issue view` for the other
fields.

**`updated` in `issue view --json` has milliseconds.**

```text
before: "updated": "2026-11-09T10:15:30Z"
after:  "updated": "2026-11-09T10:15:30.123Z"
```

Migration: parse it as RFC 3339 with fractional seconds; pass it as it is to
`--if-updated`.

**`issue assign --account-id` became `--assignee`.** It takes `me`, an
email address or an account ID; `-a` is unchanged.

Migration: replace `--account-id` with `--assignee`.

**`issue create --parent` no longer forces the type to Sub-task.** The type is
`--type`, or Task by default.

Migration: pass `--type=Sub-task` with `--parent` for a sub-task in a
company-managed project.

### Added

- `--force` on `issue create`, `issue update` and `issue comment` writes
  despite a refusal, and reports what was lost as a warning.
- `issue update --if-updated TIME` writes only if the issue's `updated` time
  matches, to the millisecond; otherwise exit 5.
- `issue update --append-description` and `--clear-description`.
- `issue delete` (with `--delete-subtasks`) and `issue delete-comment`.
- `issue fields` lists the fields settable on create (`--project`, `--type`)
  or edit (`--key`), with `--all` and `--filter`.
- `--field-json ID=JSON` on `issue create` and `issue update` sets any field,
  including custom fields; `issue view` shows custom fields.
- `--assignee` on `issue create`, resolved before the issue is created and
  sent with it; `issue assign` and `issue update --assignee` accept `me`, an
  email address or an account ID.
- `--parent` on `issue update` and `issue ready`; `--clear-parent` on
  `issue update`.
- `issue list --exclude-status` and `--order-by`.
- `issue list` warns on stderr when `--limit` cut the results, and the JSON
  form says so in `truncated`. `issue ready` pages until it has `--limit`
  ready issues.
- The config file is found in the current directory, any parent directory,
  or the home directory.
- Markdown support for tables, task lists, panels, `<details>` blocks,
  strikethrough, underline, subscript and superscript, hard breaks, rules and
  indented code blocks.

### Fixed

- A soft line break in Markdown became nothing (`line one` + `line two` gave
  `line oneline two`); it is now a space.
- Single `~` characters were deleted on write (`5~10 and 20~30` reached Jira
  as `510 and 2030`); they are kept, and only `~~` strikes through.
- Unknown, unclosed or stray inline HTML tags were deleted; they are kept as
  text.
- A create or comment request was retried after a server error and could be
  applied twice. Writes by POST are now retried only on a rate limit.
- `issue list --assignee=me` searched for a user named "me"; it now means the
  current user.
- `issue ready` listed issues in Won't Do.
- Backslash escapes and HTML entities reached Jira as literal text.
- Many round-trip losses in lists, quotes, task items, marks, code spans,
  code blocks and links.

### Known limits

- None of this was run against a live Jira site. In particular it is
  unverified that Jira accepts a media node sent back unchanged, that
  `"description": null` clears a description, and that every project accepts
  the assignee on create (a rejection falls back to create-then-assign).
- `--if-updated` checks just before the write; a change landing between the
  check and the write is not caught. Without `--if-updated`, two nodes of the
  same kind that moved between the read and the write are not detected.
- New images cannot be placed in a description.
- Rewriting a description changes four kinds of node without a warning: a
  status becomes bold text, a date becomes text, an emoji becomes its
  character, and a smart link becomes a plain link. v0.4.0 protects statuses
  and dates as tokens, restores a smart link that comes back unchanged, and
  warns about the emoji and smart links a rewrite still changes.
