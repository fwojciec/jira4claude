# Jira Cloud ADF Support

**Date:** 2026-04-10
**Status:** Accepted

## Goal

Update the markdown package to handle Jira Cloud's supported ADF nodes and marks. Scoped to what Jira Cloud actually accepts — not the full `@atlaskit/adf-schema` which includes Confluence-only and experimental nodes. Readability-first — lossy conversion is acceptable where Markdown has no clean equivalent. Round-trip fidelity can be added for specific features later if needed.

**Scope boundary:** The [Jira Cloud ADF docs](https://developer.atlassian.com/cloud/jira/platform/apis/document/structure/) are authoritative. Nodes present in the npm JSON schema but absent from Jira docs (e.g., `bodiedSyncBlock`, `syncBlock`, `blockTaskItem`) are out of scope until proven needed.

## Key Decisions

- **Readability over round-trip** — agents understand natural text better. Accept lossy conversion for mentions, status, dates, etc.
- **Reference only** — study `ajbeck/goldmark-adf` for patterns but don't depend on it (requires Go 1.25+ with `GOEXPERIMENT=jsonv2`).
- **Typed ADF structs** — replace `type ADF = map[string]any` with typed structs using `json.RawMessage` for attrs. Use `*ADFNode` for optional fields to preserve nil semantics.
- **Update in place** — existing tests define the migration contract. Feature branch, not `markdown2` package.
- **Same architecture** — goldmark `NodeRenderer` for MD→ADF, recursive walk for ADF→MD. More handlers, plus a recognition layer for HTML-encoded features (panels from `> [!NOTE]` blockquote text, expands from `<details>` HTML blocks).

## Type Change

Replace the root package `ADF` type alias with typed structs:

```go
type ADFNode struct {
    Type    string          `json:"type"`
    Version int             `json:"version,omitempty"`
    Content []ADFNode       `json:"content"`
    Text    string          `json:"text,omitempty"`
    Marks   []ADFMark       `json:"marks,omitempty"`
    Attrs   json.RawMessage `json:"attrs,omitempty"`
}

type ADFMark struct {
    Type  string          `json:"type"`
    Attrs json.RawMessage `json:"attrs,omitempty"`
}
```

`json.RawMessage` for attrs gives perfect JSON round-trip fidelity for unknown attributes — forward-compatible as Atlassian adds new node types. Add typed helper methods (`HeadingAttrs()`, `LinkAttrs()`, etc.) for known types.

### Nil semantics

The current code relies on `ADF` (i.e. `map[string]any`) being nil to mean "absent" — checked at `view.go:58`, `http/issue.go:49`, `http/issue.go:201`. A value struct `ADFNode` cannot be nil. Fix: use `*ADFNode` in domain types where the field is optional:

```go
type Issue struct {
    Description *ADFNode  // nil = no description
    // ...
}
type Comment struct {
    Body *ADFNode  // nil should not happen but stays pointer for consistency
    // ...
}
type IssueUpdate struct {
    Description **ADFNode  // nil = no change, *nil = clear, *val = set
    // ...
}
```

The `Converter` interface methods take/return `*ADFNode`:
- `ToMarkdown(nil)` → empty string (no description to render)
- `ToMarkdown(&ADFNode{...})` → rendered markdown
- `ToADF("")` → empty doc node (`&ADFNode{Type: "doc", Version: 1}`) — NOT nil, matching current behavior where empty input produces `{"type":"doc","version":1,"content":[]}`
- `ToADF("some text")` → populated doc node

## Conversion Mappings

### Block Nodes

| ADF Node | MD→ADF | ADF→MD |
|----------|--------|--------|
| `paragraph` | Done | Done |
| `heading` | Done | Done |
| `codeBlock` | Done | Done |
| `bulletList` / `orderedList` | Done | Done |
| `blockquote` | Done | Done |
| `rule` | `ThematicBreak` → `rule` | Done |
| `hardBreak` | Newline → `hardBreak` | Done |
| `table` | GFM table → `table`/`tableRow`/`tableCell`/`tableHeader` | Done |
| `taskList` | `- [ ]`/`- [x]` → `taskList`/`taskItem` | Done |
| `panel` | `> [!NOTE]` etc. → `panel` with `panelType` | `panel` → GitHub alert syntax |
| `expand` | `<details><summary>` → `expand` | `expand` → `<details>` HTML |
| `mediaSingle`/`media` | See "Images" section below | Best-effort: `![alt](url)` if URL available, else `[image]` fallback |
| `decisionList` | — (no MD syntax) | `decisionItem` → plain `- text` |
| `layoutSection` | — (no MD syntax) | Flatten content sequentially |
| `mediaGroup` | — | Each `media` → image on own line |
| `listItem` | Done | Done |
| `nestedExpand` | — | Same as `expand` |

### Inline Nodes

| ADF Node | MD→ADF | ADF→MD |
|----------|--------|--------|
| `text` | Done | Done |
| `mention` | — | `@Display Name` |
| `emoji` | — | Unicode fallback (`attrs.text`), else `:shortName:` |
| `inlineCard` | — | `[url](url)` |
| `status` | — | `**STATUS_TEXT**` |
| `date` | — | Formatted date string |
| `mediaInline` | — | `[filename]` or `[attachment]` |
| `placeholder` | — | Drop silently |
| `inlineExtension` | — | Drop with warning |

### Marks

| Mark | MD→ADF | ADF→MD |
|------|--------|--------|
| `strong` | Done | Done |
| `em` | Done | Done |
| `code` | Done | Done |
| `link` | Done | Done |
| `strike` | `~~text~~` → `strike` mark | `strike` → `~~text~~` (neither direction exists today) |
| `underline` | `<u>` HTML → `underline` mark | `underline` → `<u>text</u>` |
| `subsup` | `<sub>`/`<sup>` → `subsup` mark | `subsup` → `<sub>`/`<sup>` |
| `textColor` | — | Drop silently |
| `backgroundColor` | — | Drop silently |
| `annotation` | — | Drop silently |
| `border` | — | Drop silently |

### Images (mediaSingle/media)

**ADF→MD:** Best-effort. The `media` node has `id` (a Media Services ID, not a Jira attachment ID), `type`, and `collection`. There is no straightforward URL construction from these fields. Strategy:
- If the media node or its parent carries a URL in attrs (some Jira responses include this), use it: `![alt](url)`
- Otherwise, render fallback text: `[image: filename]` or `[image]`
- A proper media-to-URL resolution would require Jira's Media Services API, which is out of scope for now.

**MD→ADF:** `![alt](url)` in Markdown cannot produce a valid `mediaSingle` > `media` node because Jira requires `media.attrs.id`, `type`, `collection`, and `mediaSingle.attrs.layout` — none of which exist in a Markdown image. Two options:
1. **Drop with warning** — images in Markdown don't round-trip to ADF. Accept the loss.
2. **External URL workaround** — for external URLs, use `inlineCard` with the URL instead. This renders as a clickable link in Jira, not an embedded image, but preserves the reference.

Start with option 1. If agents need to embed images in Jira, that requires an upload API flow which is a separate feature.

### Extension Nodes

All extension nodes (`bodiedExtension`, `extension`, `inlineExtension`, `multiBodiedExtension`, `extensionFrame`) are opaque app-specific macros. Strategy: drop with warning in ADF→MD. No MD→ADF path.

### Unknown Nodes

Any node type not in the switch statement: drop with warning (current behavior). The typed struct with `json.RawMessage` attrs means unknown nodes survive JSON round-trip even if they don't survive Markdown conversion.

## Mark Ordering

When rendering ADF→MD, enforce consistent sort order for delimiter nesting:

`link` → `strong` → `em` → `strike` → `code`

Link outermost, code innermost. This prevents ambiguous nesting.

Fix whitespace-in-marks: expel trailing/leading whitespace from mark delimiters. `**bold **` becomes `**bold** `.

## ADF Nesting Constraints

Important constraints to enforce in MD→ADF (per [Jira Cloud ADF docs](https://developer.atlassian.com/cloud/jira/platform/apis/document/nodes/)):

- `blockquote` allows `paragraph`, `bulletList`, `orderedList`, `codeBlock`, `mediaGroup`, `mediaSingle` — richer than the research doc claimed, but still no headings or tables
- `listItem` requires at least one valid child (`paragraph`, `bulletList`, `orderedList`, `codeBlock`, `mediaSingle`, `mediaGroup`) — does NOT require a leading paragraph
- `codeBlock` text nodes must have zero marks
- `panel` can contain `paragraph`, `heading`, `bulletList`, `orderedList`
- `code` mark cannot combine with `textColor` or `backgroundColor`

### Panel and Expand Recognition (MD→ADF)

These features need special handling because goldmark doesn't natively parse them:

- **Panel from `> [!NOTE]`**: goldmark parses this as a `blockquote` containing a paragraph starting with `[!NOTE]`. The MD→ADF handler must detect the `[!NOTE]`/`[!WARNING]`/`[!CAUTION]`/`[!TIP]`/`[!IMPORTANT]` prefix in the first paragraph of a blockquote and emit a `panel` node instead. Map: NOTE→info, WARNING→warning, CAUTION→error, TIP→success, IMPORTANT→note.
- **Expand from `<details>`**: goldmark does NOT parse multiline `<details>` as a single HTMLBlock. Instead it produces: HTMLBlock (the `<details><summary>…</summary>` open tag), then normal block nodes for the body content, then HTMLBlock (the `</details>` close tag). This means expand recognition is a **stateful sibling-level transform**: during the AST walk, detect an HTMLBlock starting with `<details>`, collect all subsequent sibling nodes until the `</details>` HTMLBlock, and emit them as the content of an `expand` node. The title comes from parsing the `<summary>` tag in the opening HTMLBlock.

## Validation

Embed the ADF JSON schema from `@atlaskit/adf-schema` v52.4.x via `//go:embed`. Use `santhosh-tekuri/jsonschema/v6` for test-time validation. Don't fetch the schema URL at runtime — it has had stability issues.

**Important:** The npm JSON schema includes nodes/marks not supported by Jira Cloud. Validation should confirm structural correctness (valid JSON, correct nesting) but the Jira Cloud docs are authoritative for what Jira actually accepts.

## Testing

- Existing tests define the migration contract — must pass through the type change
- Golden file tests: `testdata/*.md` ↔ `testdata/*.adf.json` pairs for each new node type
- Round-trip tests: MD→ADF→MD — compare **normalized ASTs** or use semantic comparison, not exact string equality. The current round-trip tests (`converter_test.go:60`) use `assert.Equal` on raw strings, which will reject legitimate normalizations (whitespace expulsion, mark reordering). Update these to compare parsed ASTs or use a normalization function.
- Schema validation: all generated ADF validated against embedded schema

## Implementation Order

1. **Type migration** — `ADF` from `map[string]any` to `ADFNode` with `*ADFNode` for optional fields, update all callers and nil checks, keep tests green
2. **Easy wins** — strikethrough (both directions), hardBreak and rule in MD→ADF (ADF→MD already done)
3. **Medium features** — tables and task lists in MD→ADF; panel and expand in both directions (requires HTML block recognition and blockquote prefix detection); images in ADF→MD only (MD→ADF drops with warning)
4. **Inline nodes** — mention, emoji, inlineCard, status, date in ADF→MD
5. **Marks** — underline, subsup, mark ordering, whitespace expulsion
6. **Test updates** — update round-trip tests to use normalized comparison instead of exact string equality
7. **Long tail** — decisionList, layoutSection, extensions, mediaGroup (drop with warning)
8. **Validation** — embed ADF schema, add schema validation to test suite

## References

- [ADF ↔ GFM Research](../research/markdow-to-adf.md) — full spec analysis and implementation guidance
- [ADF JSON Schema](https://unpkg.com/@atlaskit/adf-schema@52.4.17/dist/json-schema/v1/full.json) — canonical schema
- `ajbeck/adf-to-markdown` + `goldmark-adf` — Go reference implementation (MIT, March 2026)
- `marklas` (Python) — Union AST approach, reports 3-5.8x token reduction
