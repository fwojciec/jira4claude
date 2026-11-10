# Scripted-Use Alignment and Attachments — Design

**Date:** 2026-11-09
**Status:** Approved 2026-11-09, ready for implementation. Rows marked *Proposed* in Decisions were approved as written.

## Problem

Agents use `j4c` as steps in scripts: read a ticket, change text in code, write
it back, check the result. An audit of `main` from that angle found that the
tool is pleasant to call but not yet safe to chain. Three write paths corrupt
content without any signal, several commands leave a script unable to tell
what happened, and there is no attachment support.

Evidence levels used in this document:

- **Probed**: reproduced by running the converter or HTTP client in-process.
- **Read**: established by reading the code on `main`.
- **Unverified**: depends on Jira behaviour we could not test (no live site).

### Findings that drive the design

| # | Finding | Evidence |
|---|---------|----------|
| F1 | A soft line break is dropped: `"line one\nline two"` is sent as `"line oneline two"` | Probed |
| F2 | Read–edit–write loses `media*`, `mention`, `status`, `date`, `emoji`, `inlineCard`, `placeholder` nodes; they come back as plain text | Probed |
| F3 | Unrecognised inline HTML is deleted with no warning: `pass <word> as` becomes `pass  as` | Probed |
| F4 | Single tildes strike text: `5~10 and 20~30` strikes "10 and 20" | Probed |
| F5 | An unclosed `<u>` marks the rest of the paragraph; a stray `</u>` is deleted; both silent | Probed |
| F6 | A `<details>` nested in a `<details>` closes the outer block early; the rest escapes it | Probed |
| F7 | A POST is retried with its body after a 5xx, so `create` and `comment` can duplicate | Probed |
| F8 | `issue update --description` is a blind replace with no concurrency guard | Read |
| F9 | `update --description ""` does nothing and prints "Updated" | Read |
| F10 | `create --assignee` resolves and assigns after the issue exists; a failure exits non-zero with the issue already created | Read |
| F11 | `list` gives no signal when `--limit` cuts results; `ready` filters after the cut | Read |
| F12 | `list --assignee=me` sends the literal `assignee = "me"` (GitHub issue 137) | Read |
| F13 | In `--json` mode errors go to stdout (Read); usage errors exit 80 and print usage to stdout (Probed) | Mixed |
| F14 | Mutations report only "ok"; `transition` matches the transition name, not the target status | Read |
| F15 | Markdown view omits comment IDs; list cuts summaries at 60 characters and shows unknown priority as `P2` | Read |
| F16 | `issue view` prints every non-null custom field, including empty lists and machine-only blobs | Read |
| F17 | No file or stdin input for long text | Read |

Not reproduced: `#12345` passes through the converter unchanged. If it is a
trap, it is on Jira's side.

## Goals

1. **Every write is lossless or refuses.** No silent change to what the caller wrote or to what was already on the ticket.
2. **Every read is addressable.** A script can ask for exactly one field of one or more issues.
3. **Every long input can come from a file or stdin.**
4. **Every mutation prints the resulting state.**
5. **Attachments** can be listed, downloaded and uploaded.

## Non-goals

- Embedding a newly uploaded file in a description (`media` node creation). Deferred until a live site settles the open questions.
- True atomic compare-and-set. Jira offers no conditional update that we know of; the guard is check-then-write.
- Backward compatibility of default output. This ships as one breaking release.
- Batch mutation commands. Scripts loop.

## Decisions

| Question | Decision | Status |
|----------|----------|--------|
| Order of work | Corruption fixes first, as their own milestone | Approved |
| Default output | Breaking output changes allowed, shipped together in one release (milestone 2) | Approved |
| Lossy writes | Refuse by default; `--force` overrides | Approved |
| Attachments | List, download, upload first; new embeds are a follow-up | Approved |
| What counts as lossy | Content or behaviour would be lost (media, mentions, unknown nodes). A node that changes type but keeps its content (emoji, status, date, inline card) only warns | Proposed |
| Removing an existing rich node | Counts as lossy: refused unless `--force`, because the tool cannot tell a deliberate deletion from a script that never read the ticket | Proposed |
| Milestone 1 release | Ships on its own. It also breaks scripts (refusals, the `""` error), but the fixes should not wait for milestone 2 | Proposed |
| Refactors | No standalone refactor milestone. Each lands inside the fix that needs it, test seam first | Proposed |

## Design

### Milestone 1: Write safety

#### 1.1 Test seams (first, because the fixes need failing tests)

- **`run(args, stdin, stdout, stderr, deps) int`** extracted from `main()`. Stream and exit-code behaviour becomes testable per command. Today none of it is covered.
- **Injectable `localId` source** in the converter. `generateLocalID` is random, so task-list output cannot be compared for equality.
- **Converter properties** using Go's built-in fuzzing (no new dependency; the January `rapid` design was never implemented):
  - every `ToADF` output validates against the embedded ADF schema;
  - Markdown → ADF → Markdown → ADF reaches a fixed point;
  - every ADF fixture → Markdown → ADF (with re-attachment, 1.4) is identical, or the loss is reported.

#### 1.2 Typed conversion report

`Converter` currently returns `[]string` and documents that conversion always
succeeds. That contract contradicts "refuse". Replace it:

```go
type ConversionIssue struct {
    Kind   string // "dropped" or "degraded"
    Node   string // e.g. "Image", "nested details"
    Detail string
}

ToADF(markdown string) (*ADFNode, []ConversionIssue)
ToMarkdown(adf *ADFNode) (string, []ConversionIssue)
```

The CLI refuses a write with any `dropped` issue (exit 1) unless `--force`.
`degraded` issues print as warnings.

#### 1.3 Converter fixes

- **Soft break (F1):** emit a space.
- **Single tilde (F4):** only `~~` strikes; a single `~` is literal text.
- **Inline HTML (F3, F5):** one `markTracker` type replaces the mark-stack loop that is copied into four functions. Its transition table is explicit and table-tested. Any tag that is unrecognised, unclosed, stray or mis-nested is emitted as literal text, so nothing is deleted.
- **`<details>` (F6):** the sibling scan keeps a depth count. ADF cannot nest `expand`, so an inner block is flattened into the outer one and reported as `degraded`.
- **Images:** a new `![alt](url)` on write is `dropped` and therefore refused until new embeds exist. Existing media, including media with an external URL, uses the tokens in 1.4.

Found while fixing the above or by the fuzz properties, all in the same milestone:

| Issue | Fix |
|-------|-----|
| #152 | A hard break after an inline element is kept. |
| #153 | Text is decoded on write and minimally escaped on read (`markdown/escape.go`), so literal Markdown characters in Jira text survive a round trip. Edge blanks read as `&#32;`/`&#9;`; a hard break reads as backslash-newline. |
| #154 | An indented code block becomes a `codeBlock`. |
| #155 | A non-`<details>` HTML block becomes a literal paragraph instead of vanishing. |
| #156 | Block content in a task item: paragraphs are flattened (`degraded`), a nested task list becomes a sibling `taskList`, code, tables and rules are `dropped`. |
| #157 | Containers on read: blank line between block children, continuation indent by marker width, `>` prefix, nested task lists rendered. |
| #158 | Adjacent lists must not merge on read; an empty list item must re-parse. |
| #159 | Marks: edge whitespace, adjacent marks, duplicate marks, canonical mark order. |
| #160 | Code: backticks in spans and fences, language with trailing tab, newline in a code span. |
| #161 | Links: hrefs needing escapes, text equal to href, empty href, autolinks re-gained on read. |
| #162 | Unusual input: lone CR, control-character entities, invalid UTF-8, table delimiter row after a hard break. |
| #163 | A blank-only line of a fenced code block in a list item keeps its blanks (write side). |

`TestPropertyKnownFailures` holds the minimised inputs for #158 to #162 as
skipped cases (`J4C_KNOWN_FAILURES=1` runs them). Each fix moves its cases into
the fuzz seed corpus. The milestone is done when the table is empty and
`FuzzToADFFixedPoint` runs clean.

Visible consequences for the release notes: more writes are refused (raw
constructs that used to be dropped silently); `\<` and `&#32;` can appear in
`issue view`, as can `&#10;` and letter references such as `&#97;` next to
emphasis delimiters, because marks and edge blanks read back exactly; media,
mentions and unknown nodes appear as `{{...}}` tokens; `<br>` and HTML comments appear as text; a table in a list item is
flattened with a `degraded` warning; `list --json` and `ready --json` return
`{"issues": [...], "truncated": bool}`.

#### 1.4 Guarded description update

`ToADF` sees only the new Markdown. It cannot know that the ticket's current
description holds a mention or an image. So `update --description` becomes an
explicit four-step sequence, designed as one unit with the concurrency guard:

1. **Read** the current issue (ADF and `updated`).
2. **Check** `--if-updated=<timestamp>` when given. A mismatch exits 5 (conflict) and writes nothing.
3. **Convert and re-attach.**
4. **Write, re-read, report** the resulting state.

Re-attachment works on **protected nodes**: those where plain text would lose
content or behaviour (`media`, `mediaSingle`, `mediaGroup`, `mediaInline`,
`mention`, and any node type the reader does not know).

- **Read side changes.** Today these render as text that is indistinguishable from prose (`@John Doe`, `[image: shot.png]`), or not at all (extensions are skipped). Each protected node will render as a unique token that survives conversion as plain text, numbered in document order, for example `{{media:1 shot.png}}` and `{{mention:2 John Doe}}`. The exact syntax is an implementation choice; uniqueness is the requirement.
- **Write side.** Each token still present in the new text is replaced by the original node, verbatim.
- **A token that is missing from the new text is a loss.** The write is refused unless `--force`, and the error names the missing nodes. This covers the script that writes a fresh description without reading first.
- **Unprotected rich nodes** (`emoji`, `status`, `date`, `inlineCard`) keep their current readable rendering and come back as text or links. That is reported as `degraded`.

Consequences:

- Exact round-trip of protected nodes is a target that the fixture property in 1.1 enforces. It is not true today.
- Re-attachment reuses the original node, so it does not depend on how media IDs relate to attachment IDs.
- The window between steps 1 and 4 is a small race. The help text says so.
- `--if-updated` compares at millisecond precision. The JSON view currently formats `updated` to whole seconds and must emit milliseconds.

Related changes:

- **`--append-description`:** appends converted nodes to the current ADF. Existing content is never converted, so this is the safest partial edit.
- **`--description ""` (F9):** a validation error. `--clear-description` is the explicit form.
- `IssueUpdate`'s nil/empty/value pointers are replaced for description by a small sum type (`keep`, `set`, `append`, `clear`).

As built (#149, #150):

- **Token syntax.** `{{kind:number label}}`, for example `{{mediaSingle:1 error dialog}}`. One token per outermost protected node. On write the number and the kind are checked; the label is never trusted. A kind that no longer matches its number means the description changed since it was read, and the token is refused.
- **Literal token-looking text in Jira is never altered.** Escaping cannot work, because the write side decodes every escape before re-attachment. Instead the reader adds `!` after `{{` (`{{!mention:1 x}}`) until the document's own text cannot be read as one of its tokens. `Reattach` derives the same salt from the original document.
- **`Converter.Reattach(original, edited)`** restores the nodes and returns `dropped` for a token that is missing, duplicated, unknown, of the wrong kind, or placed where its node is not allowed. A block token inside a paragraph splits the paragraph; a block restored into a task item makes it a `blockTaskItem`.
- **Content the reader cannot tokenise** (reported as `dropped` on read) makes a set or a clear refuse without `--force`; the error points to `--append-description`.
- **`--clear-description`** on a description holding protected nodes needs `--force`.
- **Create, comment and append** have no original document. Token text there is written as plain text and reported as `degraded` (`Converter.FindTokens`).
- **`--if-updated`** is honoured on every update, accepts RFC 3339 and Jira's own form, and compares instants at millisecond precision. Only `updated` gained milliseconds in the JSON view.
- **Step 4** came with the mutation results of milestone 2 (jir-mi82): every update now reads first; the result reports the `updated` time Jira holds after the write, and `removed` names what `--force` deleted. Without `--if-updated`, nodes of the same kind that moved between read and write are still not detected.

#### 1.5 Retry policy (F7)

The retry loop gets an idempotency input:

- GET, PUT, DELETE: retry on 429, 5xx and connection errors (as today).
- POST: retry on 429 only. A 5xx or connection error returns at once, with a message that the request may have been applied.

#### 1.6 Create with assignee (F10)

- Resolve the assignee **before** creating.
- Send `assignee` in the create payload, which removes the half-done state. (Unverified that every project's create screen accepts it.)
- If Jira rejects the field, fall back to create-then-assign and report "created KEY, assignment failed" with the key on stdout.

#### 1.7 List correctness (F11, F12)

- Read the pagination fields of the search response. Report truncation on stderr and as a field in JSON.
- `ready` pages until it has `--limit` ready issues or the results end.
- `--assignee=me` maps to `currentUser()`.

### Milestone 2: Scripted I/O (the breaking release)

#### 2.1 Streams and exit codes (F13)

- stdout carries data only. Errors and warnings go to stderr in both modes; in `--json` mode they are JSON objects on stderr.
- Usage errors exit 1 (validation) and print usage to stderr.

#### 2.2 Addressable reads

- `issue view KEY...` accepts several keys.
- `--field NAME` (repeatable) selects fields. With one key and one field the output is the raw value, so `issue view KEY --field description > d.md` works.

  Decided shapes (jir-hodd):
  - Field names are the JSON view's keys (`key`, `project`, `summary`, `description`, `status`, `type`, `priority`, `assignee`, `reporter`, `labels`, `relatedIssues`, `comments`, `attachments`, `created`, `updated`, `url`), matched without case, plus a custom field by ID (`customfield_NNNNN`). Custom fields are not selectable by display name: names are known only after the read, and an unknown field must fail before any request. `--field` also takes a comma-separated list; a repeated field prints once; an empty name is unknown.
  - Raw values (text): strings as they are (`description` is the Markdown `issue update --description` re-attaches from), `labels` one per line, `attachments` as the tab-separated lines `attachment list` prints, `comments`, `relatedIssues` and custom fields as one line of compact JSON (an unset custom field is `null`). A value ends in exactly one newline; an empty string or list prints nothing.
  - Text, one key and one field: the raw value alone. Otherwise each value follows a header line `==> KEY FIELD <==`, issues in the order asked, fields in the order asked. Values are not escaped; `--json` is the form no value can break. Several keys without `--field`: the full pages in the order asked, a blank line apart.
  - JSON: one key gives an object (the full view, or `{"key": ..., <fields in order>}` with `--field`); several keys give an array of those in the order asked, each carrying its `key`. Empty values are kept (`""`, `[]`, `null`).
  - A failed key among several: the others print; each failure is an error on stderr as `KEY: message`, followed by a summary error `could not read N of M issues: KEYS`; the exit code is that of the first failure in the order asked. With every key failed, `--json` prints `[]`. One key that fails behaves as before.
  - Conversion warnings: with `--field`, only the selected Markdown fields (`description`, `comments`) are converted, so only they warn. A view warning names its issue by the key Jira returned: under `--json` the warning line always carries `key`; in text the line reads `[warn] KEY: ...` when several keys were asked, and is unchanged for one key.
- `issue list` items gain `type` and `assignee`. Summaries are not cut, and priority prints as Jira names it (F15).
- Markdown view shows comment IDs (F15).

  Decided shapes (jir-dhy6):
  - Text list (`list`, `ready`): one tab-separated line per issue, `KEY  STATUS  TYPE  PRIORITY  ASSIGNEE  SUMMARY`, no header. The summary is last and never cut; another empty value prints as `-` (a shell `read` with a tab `IFS` merges adjacent tabs); control characters and backslashes are escaped as in `attachment list`, so a line always has six columns. Priority is Jira's name; assignee is the display name.
  - JSON list items: `key, status, type, priority, assignee, summary`; `type` always present, `priority` and `assignee` left out when empty, as in the issue view.
  - Comment header: `**Author** (YYYY-MM-DD HH:MM, id 10042):`, the ID being what `issue delete-comment KEY ID` takes.

#### 2.3 Input

- `--description-file PATH` and `--body-file PATH`; `-` means stdin. Mutually exclusive with the inline flags (F17).
- `--dry-run` on `create`, `update` and `comment`: prints the text as Jira will hold it, plus the conversion report, and writes nothing.

#### 2.4 Mutation results (F14)

`Printer.Success(msg, keys...)` is replaced by a typed result:

```go
type MutationResult struct {
    Key     string
    URL     string
    Action  string            // "created", "updated", "transitioned", ...
    Changes map[string]Change // field -> {From, To}
    Removed []string          // nodes deliberately removed (1.4)
}
```

- `transition` prints `KEY: Backlog → Done`.
- `--status` matches the target status name first, then the transition name. An ambiguous match is a validation error that lists the choices.
- JSON shapes get golden-file tests.

#### 2.5 Custom-field noise (F16)

- `issue view` hides empty values (`[]`, `{}`, `""`) and a short list of machine-only field types (development summary, rank).
- It prints the hidden count, as `issue fields` does. `--all-fields` shows everything.

  Decided shapes (jir-zb7m):
  - The read asks for `expand=names,schema`, so each custom field carries its schema `type` and `custom` key, as `issue fields` reports them. A field is machine-only by its custom key, never its display name: `com.pyxis.greenhopper.jira:gh-lexo-rank` and `com.atlassian.jira.plugins.jira-development-integration-plugin:devsummarycf` (also `:devsummary`, since sources disagree). **Unverified**: the `schema` expand, its per-field `custom` key and the development summary key come from Atlassian's reference and third-party field lists, not from a live site; a wrong key fails silently (the field keeps printing). A field without a schema entry is hidden only when empty. Empty is `[]`, `{}`, `""` or `null` after compaction. Unset (`null`) fields never reach the view, as before, so they are not counted.
  - Text view only. The notice goes to stderr: `[info] N empty or machine-only custom field(s) hidden — use --all-fields to show all.`, with `KEY: ` after `[info] ` when several keys were asked. No `## Custom Fields` header when every field is hidden.
  - `--json` does not hide: it keeps every field it had before, prints no notice, and `--all-fields` leaves it unchanged. Scripts want complete, stable data, and a field whose value goes from `[]` to `["x"]` should not appear and disappear from `customFields`. `--field customfield_N` returns any field, hidden or not, in both modes.

#### 2.6 Documentation

A "Scripting" section in the README and in `--help`: field selection, file
input, the guarded update, exit codes. Two items in the original feedback
already existed and were not found.

### Milestone 3: Attachments

Per the package layout: `attachment.go` in the root (type and
`AttachmentService` interface), `http/attachment.go`, `mock/attachment.go`.

```go
type Attachment struct {
    ID, Filename, MimeType string
    Size                   int64
    Author                 *User
    Created                time.Time
}
```

| Command | Behaviour |
|---------|-----------|
| `issue view KEY` | Adds an attachments section: ID, filename, size, type |
| `attachment list KEY` | The same data alone |
| `attachment download KEY (--id ID \| --name NAME \| --all) --dir DIR` | Saves files and prints one saved path per line |
| `attachment upload KEY FILE...` | Multipart POST with `X-Atlassian-Token: no-check`; prints the new IDs |
| `attachment delete ID` | Deletes one attachment |

Download rules:

- Only the base name of the server's filename is used. Nothing is written outside `--dir`.
- An existing file is not overwritten without `--overwrite`.
- Credentials are sent only to the configured host, never to a redirect target on another host.
- Upload is a POST, so 1.5 applies: no retry after a 5xx.

### Follow-up (not scheduled): new embeds

Referencing an uploaded file from Markdown, for example
`![alt](attachment:shot.png)`. Blocked on the open questions below.

## Open questions for the first live smoke test

| Question | Affects | Fallback if the answer is bad |
|----------|---------|-------------------------------|
| Does Jira accept a `media` node echoed back unchanged? | 1.4 | Refuse edits of descriptions that contain media |
| Does the create payload accept `assignee` on team-managed and company-managed projects? | 1.6 | Create-then-assign with explicit reporting |
| Exact pagination fields of `/search/jql` (`isLast`, `nextPageToken` taken from Atlassian's OpenAPI document) | 1.7 | None needed; adjust field names |
| Does create accept `"assignee": {"id": accountId}`? | 1.6 | The create-then-assign fallback already runs on a 400 naming `assignee` |
| Does `"description": null` clear a description? | `--clear-description` | Send an empty ADF document |
| Does Jira accept the empty-paragraph placeholder in empty list items, table cells and panels? | 1.3 | Emit a paragraph holding one space |
| Do comments or transitions change an issue's `updated` value? | 1.4 | Document that `--if-updated` can report a conflict after any activity |
| Does Jira accept a `taskItem` rewritten as `blockTaskItem`, and task items with new `localId`s? | 1.4 | Refuse block tokens in task items |
| Redirect status and auth of `/attachment/content/{id}` | M3 | Use `redirect=false` |
| Do ADF media IDs match attachment IDs? | Follow-up | Join by filename |
| Are API tokens available on the Free plan? | All live testing | Paid trial for the smoke test |

Everything else is covered by `httptest` and converter tests.

## Testing

- TDD throughout; each finding above becomes a failing test before its fix.
- Converter: fuzz properties (1.1), plus a table test for `markTracker`.
- CLI: `run()` tests for streams and exit codes per command.
- HTTP: `httptest` for retry by method, pagination, multipart upload and redirect handling.
- `make validate` gates every issue.
