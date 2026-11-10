# Changelog

Changes that affect people and scripts using `j4c`. The README describes the
current behaviour in full.

## v0.4.0 (unreleased): scripted input and output, attachments

### Breaking changes

**Errors go to stderr in `--json` mode.** Each error and warning is one JSON
object per line on stderr, with a `level` key. stdout holds only data, or
nothing when a command fails, except where part of the work was done:
`issue view` with several keys and `attachment download` and `upload` with
several files print what succeeded, `issue create` prints the new key when
its separate assignment step failed, and `issue update` and `issue assign`
print their result when only the read-back after the write failed. When
every key or file of such a command fails, `--json` still prints the empty
result (`[]`, or the upload result with `"attachments": []`); text mode
prints nothing.

```text
before (stdout):  {"code": "not_found", "error": true, "message": "issue not found"}   (indented over five lines)
after  (stderr):  {"level":"error","error":true,"code":"not_found","message":"issue not found"}
```

Migration: read errors from stderr, one JSON object per line, and select by
`level`.

**Warnings in `--json` mode are JSON.** They were plain text lines starting
with `warning: ` on stderr; now they are objects with `"level":"warning"`,
and conversion warnings also carry `kind`, `node` and `detail`, and `token`
when the warning is about a token.

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

**Every write to Jira prints a result instead of an `[ok]` line.** (`init`
prints its own shape; see below.)

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

**`issue transition --id` refuses an ID that is not available.** The ID used
to be sent to Jira as given, and Jira's answer decided the result. Now an ID
that is not among the issue's transitions is refused (exit 1) before any
write, with the available ones listed.

```text
before: j4c issue transition PROJ-123 --id 99  ->  POST sent; Jira's error, exit 1
after:  j4c issue transition PROJ-123 --id 99  ->  [error] transition id "99" is not available; available: "Start work" (id 11) → In Progress, ...   (nothing sent)
```

Migration: none, unless a script relied on Jira's own message.

**A 413 from Jira is a validation error on every command.** Jira's "request
too large" answer was an internal error; it now means the input must change.

```text
before: Jira answers 413  ->  exit 7, code "internal"
after:  Jira answers 413  ->  exit 1, code "validation"
```

Migration: treat 413 as you treat other refused input (exit 1); do not retry
it unchanged.

**`init --json` prints one object.** It printed one success object per file
written or found; it now prints one object of three booleans. In text mode
the lines lost their `[ok] ` prefix.

```text
before: {"message": "Created .jira4claude.yaml", "success": true}
        {"message": "Added .jira4claude.yaml to .gitignore", "success": true}   (each indented over several lines)
after:  {"configCreated": true, "gitignoreAdded": true, "gitignoreExists": false}   (indented)
```

Migration: read the booleans.

**`issue update` and `issue transition` read the issue before every write,
and `issue assign` before and after.** The read gives the old value of each
change reported. An update that neither edits the description nor passes
`--if-updated` (the only cases v0.3.0 read the issue for) and a transition
make one more request than before, a GET before the write, and an assign
two, a GET before and after; a failed read before the write now stops the
command with nothing written.

Migration: none for output; count the extra requests against rate limits.

**`issue comment --body ""` is an error.** An empty or blank body (inline or
from a file) exits 1. It used to be sent to Jira.

Migration: do not send empty comments.

**A blank description in `issue update` is an error.** A `--description` or
`--append-description` of only spaces, tabs and line breaks, inline or from a
file, used to be sent (and became an empty document); it now exits 1 like an
empty one. In `issue create` a blank description still means none.

Migration: use `--clear-description` to remove a description.

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
into text and the smart links it turns into plain links. Since jir-h44x both
come back while their paragraph is unchanged, and a smart link in a changed
paragraph is restored when its link can only be that card (its URL as its
text, a whole text node with no other mark, identical cards for that URL,
and as many copies of the link in the new text as in the read). An emoji
with neither a character nor a short name, which read as nothing and was
lost on a rewrite, is reported as `dropped` when its paragraph is edited, so
that write is refused unless `--force`.

Migration: read the description again before editing. Markdown saved from an
earlier read of a description holding a status or date is refused: it lacks
their tokens, and its token numbers may name other kinds of node now.
Keep status and date tokens as you keep mentions; leaving one out is refused
unless `--force`, and so is `--clear-description` on a description holding
one.

**Placeholders read as tokens, and a rewrite keeps what it does not edit.**
A placeholder (a template's hint text) read as nothing and was lost on a
rewrite; it now reads as a token, `{{placeholder:4 Type something}}`, and
the token numbers after it shift. A table row with more cells than the
header row (a header cell spanning columns) lost its extra cells on read;
the header now gains empty cells. Writing a description back now gives back
every block you did not change exactly as Jira held it, and a changed block
keeps the attributes and formatting Markdown cannot show where it matches
the original block. What still changes is reported, one line per kind with
a count: `degraded` for a form change (text colour, alignment, a panel's
colour, a layout flattened, a decision list turned into a bullet list, ids
not kept), and `dropped` for inline comments in an edited paragraph, whose
marks would come off the text. So an unchanged rewrite no longer warns about
emoji or smart links, a rewrite is no longer refused for content with no
Markdown form in a part it does not change, and an edit can now warn or be
refused where it changed something silently before.

Migration: keep placeholder tokens as you keep mentions, or pass `--force`
to remove one. Treat the new warnings as reports of what the edit changes in
Jira; to keep an inline comment, leave its paragraph unchanged.

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
  the text as Jira would hold it and writes nothing. An update dry run reads
  the issue and makes every check of a real run, `--if-updated` included; a
  create or comment dry run reads nothing from Jira (but an `--assignee`
  lookup), so it makes only the local checks and does not prove the project
  or issue exists, that you may write to it, or that Jira accepts the fields.
- Every write to Jira reports a typed result: action, the fields written
  (old and new values, or `edit` for the description and for a built-in field
  set with `--field-json`; none for a create), the content removed under
  `--force`, and the issue's `updated` time after an update. An update or
  assign whose read-back failed after the write still prints its result,
  without `updated`, and fails with the read's code.
- `issue list` and `issue ready` items include `type` and `assignee`, in text
  and JSON.
- Attachments: `attachment list`, `attachment download` (`--id`, `--name` or
  `--all` into `--dir`, with `--overwrite`), `attachment upload`,
  `attachment delete`, and an attachments section in `issue view`.

### Fixed

- A server that accepted a connection and never answered hung any command
  forever. `--timeout DURATION` (default `30s`, `0` for none) now bounds each
  request: an API call must be answered in full within it; an attachment
  download or upload fails only after that long with nothing moving, never
  for its total time. A timed-out request is not retried, exits 7, and a
  timed-out write says it may have been applied.
- In `--json` mode a usage error was printed as text even when `--json` was
  given; it is now a JSON error line, also when `--json` comes after the bad
  argument.
- Unknown priorities were shown as `P2` in lists; the priority is now printed
  as Jira names it, and an empty one as `-`.
- JSON on stdout wrote `&`, `<` and `>` inside strings as `\u0026`,
  `\u003c` and `\u003e`; it now keeps them as they are, as the stderr lines always
  did. A JSON parser reads both forms the same; a `grep` for `<` now matches.
- Rewriting a description lost, without a word: placeholders; layouts
  (flattened); decision lists (turned into bullet lists); nested expands;
  task items holding paragraphs; a panel's custom or tip type, icon and
  colour; a table's layout, width, numbered column and display mode; cell
  widths, spans and backgrounds; text colour, highlight and inline comments;
  alignment, indentation and breakout; link titles; empty paragraphs; a line
  break at the end of a paragraph or in a heading or cell; control
  characters; and the ids of task items, tables and other nodes. Each now
  comes back, or is reported (jir-h44x).
- A block HTML comment that mentions `<details>`, such as
  `<!-- <details> -->`, was dropped, so a write refused it; it is text, like
  every other comment.

### Known limits

- None of this was run against a live Jira site. Attachment download
  redirects, upload of empty files and duplicate names, and Jira's MIME
  types come from Atlassian's API reference only.
- An upload is not retried after a server error, which may have attached the
  file anyway. Check with `attachment list` before sending it again.
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
backslash at the end of a line for a hard break. (Not exactly, as built: a
carriage return or control character, a line break in inline code, and a
line break at the end of a paragraph or in a heading or table cell have no
Markdown form and read changed, with a warning; see v0.4.0 for how a rewrite
keeps them.) `<br>` and HTML comments
show as text: `<!-- note -->` in Jira's text reads as `\<!-- note -->`, and
one written in Markdown reaches Jira as that text.

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

Migration: edit the description as `issue view --json` gives it in
`description` (`issue view --field description`, which prints it alone, came
in v0.4.0); use `--append-description` to add text without rewriting.

**`issue update --description ""` is an error.** It used to do nothing and
print `[ok] Updated: KEY`. (`issue create` with an empty description still
creates the issue without one.)

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
- A bare URL or email address in Markdown was dropped on write
  (`Mail a@b.co` reached Jira as `Mail `). A bare URL now becomes a link; a
  bare email address stays plain text, as Jira itself leaves a typed
  address; `<a@b.co>` becomes a link to `mailto:a@b.co`. On read, an address
  in plain text reads as it is, and a `mailto:` link whose text is its
  address reads as `<a@b.co>` (as `[a@b.co](mailto:a@b.co)` where that form
  would not read back), which writes back as the same link.
- Line endings, control characters and invalid UTF-8 are handled the same
  way on write and on read: CRLF and a lone CR become a line break, other
  control characters but tab a space, and invalid UTF-8 U+FFFD; a read that
  meets one in Jira's text reports it as `degraded`.
- `issue list` and `issue ready` no longer convert descriptions, so they no
  longer print conversion warnings about descriptions they do not show.
- `--field-json` split a JSON value at its commas
  (`--field-json 'labels=["a","b"]'` failed); a value is kept whole.
- A status the tool did not know (such as Won't Do) was shown as To Do; every
  status is shown as Jira names it.
- Jira's field errors name the field (`body: INVALID_INPUT`, not
  `INVALID_INPUT`), sorted so the message is the same on every run.
- `issue fields` lists fields in the same order on every run, quotes allowed
  values, and skips an allowed value with no display value.
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
- Rewriting a description also loses, without a word, placeholders, layouts,
  decision lists, nested expands, the attributes and marks Markdown cannot
  show (panel colours and custom types, table layout and cell widths,
  text colour, inline comments, alignment, link titles, task item ids), empty
  paragraphs, line breaks at the end of a paragraph and control characters.
  Rewriting a description that held content with no Markdown form was
  refused even when that part was not edited. v0.4.0 gives back every block
  the edit does not change and reports the rest.
