# jira4claude

`j4c` is a Jira Cloud command line client for AI agents and scripts.

[![Go Reference](https://pkg.go.dev/badge/github.com/fwojciec/jira4claude.svg)](https://pkg.go.dev/github.com/fwojciec/jira4claude)
[![Go Report Card](https://goreportcard.com/badge/github.com/fwojciec/jira4claude)](https://goreportcard.com/report/github.com/fwojciec/jira4claude)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

It covers the commands an agent needs (issues, comments, transitions, links,
attachments) and nothing else. Descriptions and comments are written and read
as Markdown; the tool converts them to and from Jira's document format.

## Guarantees

- **It never prompts.** A missing required flag is an error. Deletes do not ask
  for confirmation.
- **It never waits forever on Jira.** Each request must be answered in full
  within `--timeout` (default `30s`). An attachment download or upload has
  no limit on its total time, only on silence: it fails when nothing arrives
  or is sent for that long, which also bounds the wait for the answer to
  start. A request that times out is not retried, and a command working
  through several issues or files stops at the first timeout and names what
  it did not try, so a server that stops answering ends the command after
  about that long, with exit 7. A write
  that times out may have been applied, and its error says so. `--timeout 0`
  waits forever.
- **stdout carries only data.** Errors, warnings and notices such as
  `[info] No issues found` go to stderr. With `--json`, stdout is JSON and each
  stderr line is one JSON object. `--help` and `--version` print text even
  with `--json`.
- **Exit codes say what failed**: 1 for a validation or usage error, 4 for not
  found, 5 for a conflict, and so on (see [Exit codes](#exit-codes)).
- **A failed command prints nothing on stdout**, with these exceptions, where
  part of the work was done and stdout reports that part: `issue view` with
  several keys, `attachment download` and `attachment upload` with several
  files, `issue create` whose separate assignment step failed (the new key is
  printed), and `issue update` and `issue assign` whose read-back failed
  (the write was applied; the error says not to repeat it). When every key
  or file of a several-item command fails, text mode prints nothing on
  stdout, but `--json` still prints the empty result so that stdout always
  parses: `[]` for `issue view` with several keys and for
  `attachment download`, and the usual result object with
  `"attachments": []` for `attachment upload`.
- **A write that would lose content is refused**, with exit 1 and nothing
  written, unless you pass `--force`. This covers Markdown that has no Jira
  form (an image), existing images, mentions, statuses, dates and
  placeholders left out of an edited description, and, in a part of the
  description you edited, inline comments and content the tool cannot show
  as Markdown. A part you did not edit comes back exactly as Jira held it;
  what an edit changes in form only (a colour, an emoji turned into text) is
  reported as a warning. See [Descriptions as Markdown](#descriptions-as-markdown).
- **Every write to Jira prints what it did**: the key, the action, the fields
  written (with old and new values where the tool knows them; see
  [Other updates](#other-updates)), and the issue's `updated` time after an
  update. `init`, which writes only local files, prints what it wrote in its
  own shape, with no action or changes. When `issue update` or `issue assign`
  was applied but reading the issue back failed, the result is still printed
  (an update without `updated`, an assign naming the account ID sent), and
  the command fails with the code of the failed read.

## Install and configure

```bash
brew install fwojciec/tap/j4c
# or: go install github.com/fwojciec/jira4claude/cmd/j4c@latest
```

Credentials come from `~/.netrc`. Create an API token in
[Atlassian Account Settings](https://id.atlassian.com/manage-profile/security/api-tokens), then:

```bash
printf 'machine yourcompany.atlassian.net login you@example.com password YOUR_API_TOKEN\n' >> ~/.netrc
j4c init --server=https://yourcompany.atlassian.net --project=PROJ
```

`init` writes `.jira4claude.yaml` in the current directory and adds it to
`.gitignore`. Other commands look for that file in the current directory, then
in each parent directory, then in your home directory; `--config PATH` names
it directly.

## Commands

```bash
j4c issue view PROJ-123                       # the whole issue as Markdown
j4c issue view PROJ-123 --field description   # one field, raw
j4c issue list --assignee=me --status="In Progress"
j4c issue list --jql="priority = High ORDER BY created DESC" --limit=20
j4c issue ready                               # not done and not blocked
j4c issue create --summary="Add retry" --type=Bug --description-file=body.md
j4c issue update PROJ-123 --priority=High --labels=backend,auth
j4c issue delete PROJ-123 --delete-subtasks
j4c issue fields --type=Bug                   # fields settable on create
j4c issue fields --key=PROJ-123               # fields settable on edit
j4c issue transitions PROJ-123
j4c issue transition PROJ-123 --status="In Review"
j4c issue assign PROJ-123 --assignee=me       # 'me', an email or an account ID
j4c issue comment PROJ-123 --body="Deployed to staging."
j4c issue delete-comment PROJ-123 10042
j4c link create PROJ-123 Blocks PROJ-124      # PROJ-123 blocks PROJ-124
j4c link list PROJ-123
j4c link delete PROJ-123 PROJ-124
j4c attachment list PROJ-123
j4c attachment download PROJ-123 --all --dir ./files
j4c attachment upload PROJ-123 shot.png logs/run.txt
j4c attachment delete 10001
```

`j4c --help` and `j4c COMMAND --help` describe every flag. `--json` (or `-j`)
goes before or after the command.

## Scripting

The examples use an issue `PROJ-123` on `https://example.atlassian.net`. Every
output shown is what the command prints for that issue.

### Read an issue

```bash
j4c issue view PROJ-123
```

```markdown
# PROJ-123: Fix login timeout

**Type:** Bug
**Status:** In Progress
**Priority:** High
**Assignee:** Alice Smith
**Reporter:** Bob Jones
**Labels:** backend, auth

Sessions expire after 5 minutes. Screenshot:

{{mediaSingle:1 shot.png}}

Ping {{mention:2 Jane Doe}} before merging.

Logs at &#104;ttps://logs.example.test/run/42

&#32;Indented by one space in Jira.

## Related Issues

**blocks:**
- **PROJ-124** [To Do] (Task) Release 2.4

## Comments

**Bob Jones** (2026-11-08 14:02, id 10042):
Reproduced on staging.

## Attachments

- 10000 shot.png (47.1 KiB, image/png)
- 10001 debug log.txt (2.4 KiB, text/plain)

[View in Jira](https://example.atlassian.net/browse/PROJ-123)
```

The `{{...}}` tokens and `&#...;` references are explained in
[Descriptions as Markdown](#descriptions-as-markdown). The comment ID is the
one `issue delete-comment PROJ-123 10042` takes. Custom fields follow the
description; those that are empty or machine-only (rank, development summary)
are hidden, with a count on stderr, unless you pass `--all-fields`. With
`--json` the issue is one object whose keys are the field names listed below
(empty ones left out), with custom fields under `customFields`; nothing is
hidden.

### Read one field

With one key and one `--field`, the value is printed alone and ends in exactly
one newline, so it can go straight to a file or a variable:

```bash
j4c issue view PROJ-123 --field description > d.md
updated=$(j4c issue view PROJ-123 --field updated)
```

```text
2026-11-09T10:15:30.123Z
```

Field names are the keys of the JSON view: `key`, `project`, `summary`,
`description`, `status`, `type`, `priority`, `assignee`, `reporter`, `labels`,
`relatedIssues`, `comments`, `attachments`, `created`, `updated`, `url`, or a
custom field ID such as `customfield_10016`. Case does not matter. Strings
print as they are, `labels` one per line, `attachments` as the lines
`attachment list` prints, and `comments`, `relatedIssues` and custom fields as
one line of compact JSON (`null` for an unset custom field). An empty value
prints nothing. An unknown name fails with exit 1 before any request.

### Several keys and several fields

`--field` repeats or takes a comma-separated list. With more than one value,
each follows a header line `==> KEY FIELD <==`, issues and fields in the order
asked:

```bash
j4c issue view PROJ-123 PROJ-124 --field status,updated
```

```text
==> PROJ-123 status <==
In Progress
==> PROJ-123 updated <==
2026-11-09T10:15:30.123Z
==> PROJ-124 status <==
To Do
==> PROJ-124 updated <==
2026-11-07T16:40:02.005Z
```

Values are not escaped, so a description that contains a line shaped like a
header would confuse a split on headers. Use `--json` when values are
free text (it may also follow the command):

```bash
j4c issue view PROJ-123 PROJ-124 --field status,updated --json
```

```json
[
  {
    "key": "PROJ-123",
    "status": "In Progress",
    "updated": "2026-11-09T10:15:30.123Z"
  },
  {
    "key": "PROJ-124",
    "status": "To Do",
    "updated": "2026-11-07T16:40:02.005Z"
  }
]
```

One key gives an object, several keys an array, each element carrying `key`.
Without `--field`, several keys print the full pages a blank line apart (an
array of full views with `--json`).

If some keys fail, the others still print. Each failure is an error on stderr,
followed by a summary, and the exit code is that of the first failure:

```bash
j4c issue view PROJ-123 PROJ-999 --field status
```

```text
==> PROJ-123 status <==
In Progress
```

stderr, exit 4:

```text
[error] PROJ-999: issue not found
[error] could not read 1 of 2 issues: PROJ-999
```

### Edit a description safely

The guarded edit reads the `updated` time first, then the description, edits
the file, and writes it back only if nobody changed the issue in between:

```bash
updated=$(j4c issue view PROJ-123 --field updated)
j4c issue view PROJ-123 --field description > d.md
sed -i 's/5 minutes/30 minutes/' d.md
j4c issue update PROJ-123 --description-file d.md --if-updated "$updated"
```

```text
PROJ-123: updated
  description: set
  updated: 2026-11-10T09:00:00.456Z
https://example.atlassian.net/browse/PROJ-123
```

Read `updated` before the description, not inside the update command: a
`--if-updated "$(j4c issue view PROJ-123 --field updated)"` written into the
update line reads the time at write time and so never detects a change made
while you edited.

If the issue was updated at any other time, nothing is written and the
command exits 5:

```text
[error] PROJ-123 was updated at 2026-11-09T10:15:30.123Z, not at 2026-11-09T10:15:30.000Z as --if-updated expects. Nothing was written. View the issue again and redo the change against its current state
```

`--if-updated` accepts the `updated` value from `issue view` (text or JSON)
and Jira's own form (`2026-11-09T10:15:30.123+0000`), and compares to the
millisecond. The check is made right before the write, so a change that lands
between the check and the write is not caught. Jira offers no conditional
update.

`--description-file` reads UTF-8 text of at most 1 MiB, from a file or from
standard input for `-`. A leading byte order mark and one trailing newline
are removed, so the output of `--field description` goes back unchanged.
`--append-description-file` and `issue comment --body-file` read the same way.

The `updated` value in an update's result is the time to pass to the next
guarded update.

### Preview with --dry-run

`issue create`, `issue update` and `issue comment` accept `--dry-run`. It
writes nothing and prints the text as Jira would hold it after the write.
The checks it makes depend on what it reads:

- `issue update --dry-run` reads the issue, so it makes every check, warning
  and refusal of a real run, `--if-updated` included.
- `issue create --dry-run` and `issue comment --dry-run` read nothing from
  Jira (a create still looks up an `--assignee` given as `me` or an email).
  They make the local checks and conversion refusals of a real run, but do
  not prove that the project or issue exists, that you may write to it, or
  that Jira accepts the fields.

```bash
j4c issue update PROJ-123 --description-file d.md --dry-run
```

```text
PROJ-123: would update (dry run; nothing was written)
  description: set
https://example.atlassian.net/browse/PROJ-123

Sessions expire after 30 minutes. Screenshot:

{{mediaSingle:1 shot.png}}

Ping {{mention:2 Jane Doe}} before merging.

Logs at &#104;ttps://logs.example.test/run/42

&#32;Indented by one space in Jira.
```

The tokens in the preview are numbered for the description after the write.
Keep editing your own file, not the preview. With `--json` the action is
`would_create`, `would_update` or `would_add_comment`, `dryRun` is `true`,
and the text is in `text`:

```bash
j4c --json issue create --summary "Add retry" --description "Retry **twice**." --dry-run
```

```json
{
  "project": "PROJ",
  "action": "would_create",
  "dryRun": true,
  "changes": {},
  "text": "Retry **twice**."
}
```

### Append instead of rewriting

`--append-description` adds Markdown after the current description. The
existing content is sent back as Jira holds it, never converted, so nothing
in it can be lost:

```bash
j4c issue update PROJ-123 --append-description "Fixed in build 512."
```

```text
PROJ-123: updated
  description: appended
  updated: 2026-11-10T09:00:00.456Z
https://example.atlassian.net/browse/PROJ-123
```

`--clear-description` removes the description. In `issue update` an empty
or blank `--description`, `--append-description` or input file is an error,
not a no-op; in `issue create` it means the issue has no description.

### Other updates

Each field written is reported with its old and new value, except the
description (reported as `set`, `appended` or `cleared`) and a built-in field
such as `priority` set through `--field-json` (reported as `set`, without
values: the tool does not read them). `issue create` reports no field
changes.

```bash
j4c issue update PROJ-123 --summary "Fix login timeout on staging" --priority Highest
```

```text
PROJ-123: updated
  priority: High → Highest
  summary: Fix login timeout → Fix login timeout on staging
  updated: 2026-11-10T09:00:00.456Z
https://example.atlassian.net/browse/PROJ-123
```

Custom fields are set with `--field-json ID=JSON`, for example
`--field-json customfield_10801='{"value":"High"}'`. Use `issue fields` to
find the IDs and the allowed values. `--field-json` cannot set the
description, nor a field that a typed flag of the same command sets.

### Write results as JSON

Every write to Jira prints one object with `action` and `changes` (always
present, `{}` when the command reports none, as for `issue create`).
Depending on the command it also has `key`, `url`, `updated` (an update,
unless reading the issue back failed), `removed` (content deleted under
`--force`), `commentId`, `link`, `attachments`, and for a dry run `dryRun`
and `text`. A change has `from` and `to`, or `edit` instead: `set`,
`appended` or `cleared` for the description, and `set` for a built-in field
written through `--field-json`. `init` prints
`{"configCreated": ..., "gitignoreAdded": ..., "gitignoreExists": ...}`.

```bash
j4c --json issue update PROJ-123 --description-file d.md --if-updated "$updated"
```

```json
{
  "key": "PROJ-123",
  "url": "https://example.atlassian.net/browse/PROJ-123",
  "action": "updated",
  "changes": {
    "description": {
      "edit": "set"
    }
  },
  "updated": "2026-11-10T09:00:00.456Z"
}
```

The actions are `created`, `updated`, `transitioned`, `assigned`,
`unassigned`, `deleted`, `comment_added`, `comment_deleted`, `linked`,
`unlinked`, `attachment_added` and `attachment_deleted`, and for a dry run
`would_create`, `would_update` and `would_add_comment`.

JSON on stdout and stderr keeps `&`, `<` and `>` inside strings as they are,
not escaped as `&`, `<` and `>`.

### Lists

`issue list` and `issue ready` print one tab-separated line per issue: key,
status, type, priority, assignee and summary. There is no header. The summary
is last and never cut. Another empty value prints as `-`. Tabs, newlines and
backslashes inside a value are written as `\t`, `\n` and `\\`, so every line
has exactly six columns.

```bash
j4c issue list
```

```text
PROJ-123	In Progress	Bug	High	Alice Smith	Fix login timeout
PROJ-124	To Do	Task	-	-	Release 2.4
PROJ-125	To Do	Story	Medium	Bob Jones	Export\treport as CSV
```

Split the lines with `read`:

```bash
j4c issue list --status "To Do" | while IFS=$'\t' read -r key status type priority assignee summary; do echo "$key: $summary"; done
```

With `--json` the list is an object; `priority` and `assignee` are left out
when empty, and `truncated` says whether more issues match than were
printed. For `issue ready` it says only that more ready issues may exist: the
search had more results, which may not be ready. For example:

```bash
j4c --json issue list
```

```json
{
  "issues": [
    {
      "key": "PROJ-123",
      "status": "In Progress",
      "type": "Bug",
      "priority": "High",
      "assignee": "Alice Smith",
      "summary": "Fix login timeout"
    },
    {
      "key": "PROJ-124",
      "status": "To Do",
      "type": "Task",
      "summary": "Release 2.4"
    },
    {
      "key": "PROJ-125",
      "status": "To Do",
      "type": "Story",
      "priority": "Medium",
      "assignee": "Bob Jones",
      "summary": "Export\treport as CSV"
    }
  ],
  "truncated": false
}
```

```bash
j4c --json issue list | jq -r '.issues[].key'
```

`--limit` defaults to 50. When more issues match, a warning says so on stderr:

```text
[warn] showing the first 3 issues; more match (raise --limit to see them)
```

`--assignee=me` means the account the credentials belong to. `--jql` replaces
the other filters.

### Errors and warnings on stderr

In text mode each message starts with `[error]`, `[warn]` or `[info]`. The
tool's own messages are one line each, but text quoted from Jira (an error
message it sent, a filename) can hold a line break, and the usage text
printed with a usage error has no prefix at all; read messages by their
prefix, not line by line, or use `--json`. With
`--json` each line is one JSON object with a `level` of `error` or `warning`.
An error also has `error: true`, `code` (the names in the
[exit-code table](#exit-codes)) and `message`. A conversion warning also has
`kind` (`dropped` or `degraded`), `node`, `detail`, and `token` when it is
about a token; a warning from `issue view` carries the issue's `key`.

```bash
j4c --json issue view PROJ-123 PROJ-999 --field status 2> err.jsonl
```

`err.jsonl` then holds:

```text
{"level":"error","error":true,"code":"not_found","message":"PROJ-999: issue not found"}
{"level":"error","error":true,"code":"not_found","message":"could not read 1 of 2 issues: PROJ-999"}
```

Pick the errors by level:

```bash
jq -r 'select(.level == "error") | .message' err.jsonl
```

Warnings look like these (a list cut by `--limit`, and a token written into a
comment):

```text
{"level":"warning","message":"showing the first 3 issues; more match (raise --limit to see them)"}
{"level":"warning","message":"degraded {{mention:2 Jane Doe}}: written as plain text; a token restores existing content only in issue update --description","kind":"degraded","node":"{{mention:2 Jane Doe}}","token":"{{mention:2 Jane Doe}}","detail":"written as plain text; a token restores existing content only in issue update --description"}
```

A usage error (unknown flag, missing argument) exits 1. In text mode stderr
gets the command's usage and the error; with `--json` it gets only the error
line.

### Transitions

`issue transitions` prints one tab-separated line per transition: ID, name and
the status it leads to.

```bash
j4c issue transitions PROJ-123
```

```text
11	Start work	In Progress
21	Resolve	Done
31	Close	Done
41	Review	In Review
```

`--status` matches the target status first, then the transition name, without
regard to case. The result shows the move:

```bash
j4c issue transition PROJ-123 --status "In Review"
```

```text
PROJ-123: In Progress → In Review
https://example.atlassian.net/browse/PROJ-123
```

When two transitions lead to the status asked, the command refuses and lists
them; pick one by ID:

```text
[error] status "Done" is ambiguous: it matches "Resolve" (id 21) → Done, "Close" (id 31) → Done. Nothing was written. Pass --id to choose one
```

```bash
j4c issue transition PROJ-123 --id 21
```

### Attachments

`attachment list` prints one tab-separated line per attachment: ID, size in
bytes, MIME type and filename, with control characters and backslashes in
the filename escaped as in lists.

```bash
j4c attachment list PROJ-123
```

```text
10000	48213	image/png	shot.png
10001	2460	text/plain	debug log.txt
```

`attachment download` saves into an existing directory and prints each saved
path on its own line (with `--json`, an array of `id`, `filename`, `path`,
`size`):

```bash
j4c attachment download PROJ-123 --all --dir /tmp/files
```

```text
/tmp/files/shot.png
/tmp/files/debug log.txt
```

One attachment is picked by ID or by exact filename:

```bash
j4c attachment download PROJ-123 --id 10000 --dir /tmp/files --overwrite
j4c attachment download PROJ-123 --name "debug log.txt" --dir /tmp/files --overwrite
```

Download rules:

- Only the base name of Jira's filename is used, and nothing is written
  outside `--dir`. Two attachments with the same name are saved as
  `report.pdf` and `report-10001.pdf` (the second gets its ID).
- An existing file is not replaced without `--overwrite`. With it, only a
  regular file found there is replaced; a symlink or directory found there
  is refused. A download never writes through a symlink. The check and the
  replacement are separate steps, so an entry another process puts in
  place between them (a symlink included) is replaced as a directory entry,
  its target left untouched; the replacement is not atomic against a
  concurrent writer.
- Credentials are sent only to the configured Jira host, never to a redirect
  target on another host.
- With `--all`, a file that fails is reported and the others are still
  saved, except after an authentication, permission or rate-limit error.
- `--name` matches the exact filename as Jira reports it. When several
  attachments have that name, the command refuses and lists their IDs.

`attachment upload` checks every file before sending anything, sends each
file in its own request under its base name, and prints one line per new
attachment, as `attachment list` does:

```bash
j4c attachment upload PROJ-123 notes.txt
```

```text
10002	6	text/plain	notes.txt
```

An upload is not retried after a server error or a lost connection, because
Jira may already have attached the file. Run `attachment list` before sending
it again. Results print when the run ends, so after an interrupted run
`attachment list` is also the way to see what was attached.

`attachment delete ID` deletes at once and prints the deleted attachment's
line. A delete that Jira answers with not found, after the attachment was
read, counts as done, with a warning: it was already gone.

### Create and comment from a file or stdin

```bash
j4c issue create --summary "Add retry" --description-file body.md
printf 'Deployed build 512.\n' | j4c issue comment PROJ-123 --body-file -
j4c issue update PROJ-123 --append-description-file notes.md
```

## Descriptions as Markdown

Jira stores text as a document tree (ADF). `j4c` reads it as Markdown and
converts Markdown back when writing. Some of what Jira holds has no Markdown
form, and the reader marks it so that a script can edit the text around it
without losing it.

```bash
j4c issue view PROJ-123 --field description
```

```text
Sessions expire after 5 minutes. Screenshot:

{{mediaSingle:1 shot.png}}

Ping {{mention:2 Jane Doe}} before merging.

Now {{status:3 In Review}}, due {{date:4 2026-11-20}} 🚀

Logs at &#104;ttps://logs.example.test/run/42

&#32;Indented by one space in Jira.
```

**Tokens.** Images and other media, mentions, status lozenges, dates,
placeholders (a template's grey hint text), and node types the reader does
not know are each written as a token
`{{kind:number label}}`, numbered in document order. A block token such as
`{{mediaSingle:1 shot.png}}` stands on a line of its own; an inline token such
as `{{mention:2 Jane Doe}}` sits in the text. A status is labelled with its
text and a date with its day in UTC (`{{date:4 2026-11-20}}`); the colour of
a status and the exact timestamp of a date live in the node, not the label.
When you write the description back with `issue update --description` or
`--description-file`, each token is replaced by the original node, unchanged.
Rules:

- Keep each token exactly as printed. The number and the kind are checked; the
  label is only for reading and is ignored.
- Use each token once. Keep a block token on a line of its own.
- Move a token to move its node.
- Leaving a token out deletes its node, so the write is refused unless you pass
  `--force`. With `--force` the result lists the deleted tokens under
  `removed`.
- A token whose number now names another kind of node means the description
  changed since you read it; the write is refused.
- Tokens only work in a full description update. In `issue create`,
  `issue comment` and `--append-description` a token is written as plain text,
  with a `degraded` warning.

**The `!` form.** If the description itself contains text that looks like a
token, the reader adds `!` after `{{` in its tokens until they cannot be
confused with that text, and leaves the text alone:

```text
Template text: {{mention:1 x}} and {{!mention:1 Jane Doe}}
```

Here `{{mention:1 x}}` is text in Jira and `{{!mention:1 Jane Doe}}` is the
token. Keep the `!`.

**What a rewrite keeps.** Much of what Jira holds has no Markdown form, and
the reader shows it in the nearest form or not at all: an emoji as its
character (or `:shortname:`), a smart link (inline card) as a link whose
text is its URL, a decision list as a bullet list, a layout's columns one
after another, a nested expand inside the one around it; a panel's colour or
custom type, a table's layout and column widths, cell backgrounds, text
colour and highlight, inline comments, alignment, indentation, link titles
and the ids Jira gives some nodes are not shown. When you write the
description back, the tool compares your text with the description as it
was read:

- A block you did not change (a paragraph, a list item, a table cell, a
  panel, any block and what it holds) comes back exactly as Jira held it,
  as long as the tool can tell which original block it is. Writing back what
  you read changes nothing. Two cases it cannot tell, and treats the block as
  changed (next bullet): a block identical to a sibling when you add or
  remove one of those copies, and the rows and cells of a table whose shape
  you changed, which lose their widths and spans.
- A block you changed keeps what Markdown cannot show when it matches the
  original: same kind of block, as many of that kind next to it as before,
  between the same unchanged blocks (or the same ends of its container),
  inside a container that matches the same way. A block you moved past an
  unchanged one and changed does not match, so its hidden values are
  reported lost. A
  value you changed (a heading level, a panel type, a list's first number)
  is yours; table cell widths and spans are kept only while every row has as
  many cells as before. A limit, not reported: if you replace a block with
  another of the same kind in the same place, between the same unchanged
  blocks, the new one is taken for the old one edited, and gets its hidden
  values (a custom-colour panel passes its colour to a panel written in its
  place).
- A smart link in a changed paragraph is put back when its link can only be
  that card: the link text is its URL, the link is a whole text node with no
  other formatting, the cards with that URL are identical, and your text
  holds that link exactly as many times as the original's read does (adding
  a copy of the link, or a plain link that reads the same, stops it; a
  paragraph you did not change still comes back with its card, and the copy
  stays a link).
- What still cannot be kept is reported on the write, one line per kind with
  a count: `degraded` when the content stays and its form changes (`2 emoji
  become plain text`, a colour or alignment removed, a layout flattened, a
  table that gains a header row, ids that are not kept); `dropped` for inline
  comments, whose marks would come off the text and detach the comments, and
  for content with no Markdown form at all, so that write is refused unless
  `--force`.

**Character references.** Text is escaped only where Markdown would otherwise
change it, so that writing back what you read gives Jira's text back. Two
forms appear that look unusual:

- `&#32;` is a space (and `&#9;` a tab) at the start or end of a paragraph,
  which Markdown would otherwise drop.
- `&#104;ttps://` is a URL that is plain text in Jira, not a link. Written as
  `https://` it would become a link on the way back.

Others are `\` before a Markdown character (`\*`, `\<`), `&#10;` for a line
break inside text, and letter references such as `&#97;` next to emphasis
markers. Leave them as they are. A hard line break reads as a backslash at the
end of the line.

A few things in text have no Markdown form: a carriage return or control
character, a line break inside inline code, and a line break at the end of a
paragraph or inside a heading or table cell. The read shows them as a line
feed, a space or nothing, with a warning. They come back as Jira held them
while their block is unchanged; in a block you edit they are written as
read, reported as `degraded`.

**Warnings.** Conversion problems are reported on stderr as `dropped` (content
lost) or `degraded` (content kept in another form, for example a `<details>`
block inside another, flattened into it). Reading only warns. Writing refuses on `dropped` unless
`--force`. The new Markdown's warnings are about your text; the warnings that
describe the description being written over say what it held, as above. A
nested expand read back unchanged comes back whole, but your text holds a
`<details>` inside another, so the write still warns that it was flattened.

## Refusals

Every refusal writes nothing. Each error message says what to do.

| Refusal | Exit | What to do |
|---------|------|------------|
| New Markdown has no Jira form, for example an image: `description would drop unsupported content (Image)` | 1 | Remove it, or pass `--force` to write without it. An image can be attached with `attachment upload`, but not placed in the text. |
| A token from the current description is missing: `description would lose existing content ({{mention:2 Jane Doe}})` | 1 | Put the token back, unchanged (a block token on its own line). Pass `--force` to delete the node on purpose. |
| A part of the current description you edited holds content with no Markdown form, or inline comments: `rewriting the description would remove existing content that has no Markdown form (annotation)` | 1 | Leave that part as you read it, or use `--append-description`, which keeps the existing content. `--force` deletes it. |
| `--clear-description` on a description with tokens (images, mentions, statuses, dates, placeholders) | 1 | Pass `--force` to clear it anyway. |
| `--if-updated` does not match: `PROJ-123 was updated at ..., not at ...` | 5 | Read the issue again and redo the edit on the current text. |
| `--field-json` names a field a typed flag also sets: `--field-json priority and --priority both set priority` | 1 | Give one of them. |
| `--field-json description=...` | 1 | Use `--description` or `--description-file`. |
| An empty or blank description or appended text in `issue update`, or comment body, inline or from a file: `--description cannot be empty` (in `issue create` a blank description creates the issue without one) | 1 | Pass text, or `--clear-description` to remove the description. |
| `--status` matches more than one transition: `status "Done" is ambiguous` | 1 | Use `--id` with an ID from `issue transitions`. |

## Exit codes

The code column is the `code` of a `--json` error line.

| Exit | Code | Meaning |
|------|------|---------|
| 0 | | Success |
| 1 | `validation` | Invalid input, usage error, refused write, or Jira answered 400 or 413 |
| 2 | `unauthorized` | Authentication failed |
| 3 | `forbidden` | Permission denied |
| 4 | `not_found` | Issue, attachment or other resource not found |
| 5 | `conflict` | The issue changed since `--if-updated`, Jira answered 409, or two attachments would be saved under one name |
| 6 | `rate_limit` | Jira's rate limit, still hit after the tool's own retries |
| 7 | `internal` | Anything else, including network errors, timeouts (`--timeout`) and other server errors |

## Not verified against a live Jira site

The current behaviour is tested against simulated Jira responses only (test
servers and mocks); it has not been run against a live site. Open questions
include whether Jira accepts an image or other media node sent back unchanged
(which every description edit with a `{{media...}}` token relies on), whether
`"description": null` clears a description, whether a create accepts the
assignee on every project type, and whether comments or transitions change
`updated` (which would make `--if-updated` report conflicts after any
activity). The full list, with the fallback for each, is the smoke-test table
in [the design document](docs/plans/2026-11-09-scripted-use-alignment-design.md#open-questions-for-the-first-live-smoke-test).

## License

MIT
