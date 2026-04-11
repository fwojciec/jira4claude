# Bidirectional ADF ↔ GFM conversion: a complete technical guide

**The Atlassian Document Format is a ProseMirror-derived JSON tree with 30+ node types and 12+ mark types — roughly half map cleanly to GitHub Flavored Markdown, while the rest require creative workarounds or accept lossy conversion.** For a Go CLI tool using goldmark, the optimal architecture uses goldmark's custom Renderer interface for MD→ADF (stack-based tree building) and manual recursive walking for ADF→MD. A recently published Go library pair (`ajbeck/adf-to-markdown` + `goldmark-adf`, March 2026) directly targets this exact use case and should be evaluated as either a dependency or a reference implementation. With the user's current coverage plus ~9 additional features (hardBreak, rule in MD→ADF, strikethrough, images, inlineCard, tables in MD→ADF, emoji, taskList in MD→ADF, and nested list fixes), **real-world Jira content coverage reaches approximately 95%**.

---

## 1. The complete ADF specification

ADF has exactly **one version** — `version: 1` — and every document follows this envelope structure:

```json
{"version": 1, "type": "doc", "content": [...block nodes...]}
```

There is no v2. The schema evolves through the `@atlaskit/adf-schema` npm package (currently v52.4.x), which uses a "stage 0" mechanism for experimental features before promoting them to the full schema. Atlassian publishes a JSON Schema at `http://go.atlassian.com/adf-json-schema`, though this URL has had stability issues — embed it rather than fetching at runtime.

### Block-level nodes

ADF defines these **top-level block nodes** (valid as direct children of `doc`):

| Node | Required Attrs | Key Optional Attrs | Valid Children |
|------|---------------|-------------------|----------------|
| `paragraph` | — | `localId` | Inline nodes |
| `heading` | `level` (1–6) | `localId` | Inline nodes |
| `bulletList` | — | — | One or more `listItem` |
| `orderedList` | — | `order` (start number) | One or more `listItem` |
| `blockquote` | — | — | One or more `paragraph` only |
| `codeBlock` | — | `language` | `text` nodes **without marks** |
| `rule` | — | — | None (leaf) |
| `panel` | `panelType` (info/note/warning/success/error) | — | `paragraph`, `heading`, `bulletList`, `orderedList` |
| `table` | — | `isNumberColumnEnabled`, `layout`, `width`, `displayMode` | One or more `tableRow` |
| `mediaSingle` | — | `layout`, `width`, `widthType` | Exactly one `media` node |
| `mediaGroup` | — | — | One or more `media` nodes |
| `expand` | `title` | — | Most block nodes including nested `panel`, `table` |
| `taskList` | `localId` | — | One or more `taskItem` |
| `decisionList` | `localId` | — | One or more `decisionItem` |
| `layoutSection` | — | — | Two or three `layoutColumn` |
| `bodiedExtension` | `extensionType`, `extensionKey` | `parameters`, `layout`, `localId` | Block nodes (macro body) |
| `extension` | `extensionType`, `extensionKey` | `parameters`, `text`, `layout` | None (leaf) |
| `multiBodiedExtension` | `extensionType`, `extensionKey` | `parameters`, `layout`, `localId` | `extensionFrame` nodes |

**Child block nodes** include `listItem` (children: paragraph, nested lists, codeBlock, mediaSingle), `tableRow` (children: `tableCell` or `tableHeader`), `tableCell`/`tableHeader` (support `colspan`, `rowspan`, `colwidth`, `background` attrs and can contain most block nodes), `media` (requires `id`, `type`, `collection`), `nestedExpand` (used inside table cells instead of `expand`), and `extensionFrame`.

### Inline nodes

| Node | Required Attrs | Notes |
|------|---------------|-------|
| `text` | `text` (non-empty string) | Carries marks array |
| `hardBreak` | — | Equivalent to `<br/>` |
| `mention` | `id` (account ID) | Optional: `text`, `accessLevel`, `userType` |
| `emoji` | `shortName` (e.g., `:grinning:`) | Optional: `id`, `text` (Unicode fallback) |
| `inlineCard` | `url` OR `data` (not both) | Smart link / Jira issue link |
| `status` | `text`, `color` (neutral/purple/blue/red/yellow/green) | **Cannot carry any marks** |
| `date` | `timestamp` (epoch ms as string) | — |
| `mediaInline` | `id`, `type`, `collection` | Inline file reference |
| `placeholder` | `text` | Editor placeholder text |
| `inlineExtension` | `extensionType`, `extensionKey` | — |

### Mark types

**Nine marks valid on `text` nodes**: `strong`, `em`, `strike`, `code`, `underline`, `link` (requires `href`, optional `title`), `subsup` (requires `type`: "sub" or "sup"), `textColor` (requires `color` hex), `backgroundColor` (requires `color` hex).

**Critical exclusion rules**: `code` mark cannot combine with `textColor` or `backgroundColor`. `textColor` cannot combine with `link`. These constraints must be enforced during MD→ADF conversion.

Additional marks exist for specific contexts: `border` (on `media` nodes, with `color` and `size`), `annotation` (for inline comments, with `id` and `annotationType`), `alignment` (on paragraphs/headings in Confluence), `indentation` (block indentation level), and `breakout` (layout width control on codeBlock, expand, table, panel).

### Nesting constraints

The most important nesting rules that affect conversion:

- **`blockquote`** can only contain `paragraph` — no lists, code blocks, or headings inside blockquotes in ADF (unlike Markdown, which allows arbitrary nesting)
- **`listItem`** must start with a `paragraph`, then optionally nested lists or `codeBlock` or `mediaSingle`
- **`codeBlock`** text nodes must have zero marks — no bold, italic, or links inside code blocks
- **`panel`** can only contain `paragraph`, `heading`, `bulletList`, `orderedList` — no code blocks or tables inside panels
- **Table cells** can contain most block nodes including lists, code blocks, panels, and `nestedExpand` — far richer than Markdown table cells

---

## 2. GFM ↔ ADF mapping: what converts cleanly and what doesn't

### Clean 1:1 mappings

These features translate directly between formats with no information loss:

| GFM | ADF | Direction |
|-----|-----|-----------|
| `# Heading` (h1–h6) | `heading` with `level` 1–6 | Bidirectional |
| Paragraphs | `paragraph` | Bidirectional |
| `**bold**` | `strong` mark | Bidirectional |
| `*italic*` | `em` mark | Bidirectional |
| `~~strikethrough~~` | `strike` mark | Bidirectional |
| `` `code` `` | `code` mark | Bidirectional |
| `[text](url)` | `link` mark | Bidirectional |
| ` ``` ` code blocks | `codeBlock` with `language` | Bidirectional |
| `> blockquote` | `blockquote` > `paragraph` | Bidirectional |
| `- item` | `bulletList` > `listItem` > `paragraph` | Bidirectional |
| `1. item` | `orderedList` > `listItem` > `paragraph` | Bidirectional |
| `---` | `rule` | Bidirectional |
| `- [ ]` / `- [x]` | `taskList` > `taskItem` (TODO/DONE) | Bidirectional |
| Line breaks | `hardBreak` | Bidirectional |
| Nested lists | Nested `bulletList`/`orderedList` in `listItem` | Bidirectional |

### Creative workarounds needed

These ADF features have reasonable Markdown approximations, though some metadata is lost:

- **`panel`** → GitHub alert syntax: `info` → `> [!NOTE]`, `warning` → `> [!WARNING]`, `error` → `> [!CAUTION]`, `success` → `> [!TIP]`, `note` → `> [!IMPORTANT]`. This is the approach used by `ajbeck/adf-to-markdown` and renders beautifully on GitHub.
- **`expand`** → `<details><summary>Title</summary>Content</details>` — works in GFM.
- **`mention`** → `@Display Name` for lossy conversion, or `@[Display Name](accountId)` for round-trip. Note: plain `@name` text does **not** trigger Jira notifications — only a proper ADF mention node does.
- **`emoji`** → `:shortcode:` maps to `attrs.shortName`. Custom emoji IDs are lost.
- **`inlineCard`** → `[title](url)` — loses Smart Link preview rendering.
- **`status`** → `[status:text|color]` custom syntax or lossy `🟢 **Done**`.
- **`date`** → Render as formatted date string, losing the machine-readable timestamp.
- **Images** → `![alt](url)` works for external URLs. Internal Jira attachments (with `collection`, `id`) require API integration.
- **`underline`** → `<u>text</u>` HTML works in GFM.
- **`subsup`** → `<sub>`/`<sup>` HTML works in GFM.
- **Tables** → Basic structure maps, but `colspan`, `rowspan`, `background` colors, and block-level cell content are all lost in standard GFM tables.

### Truly one-way (ADF features with no Markdown equivalent)

- **`textColor`** and **`backgroundColor`** — fundamentally not representable in Markdown. Accept the loss.
- **`annotation`** (inline comments) — completely lost.
- **`layoutSection`/`layoutColumn`** — no column layout in Markdown. Content must be flattened sequentially.
- **Extension nodes** (`bodiedExtension`, `extension`, `inlineExtension`, `multiBodiedExtension`) — opaque Confluence/Jira app-specific macros. Can only be preserved via raw ADF JSON embedding.
- **`decisionList`/`decisionItem`** — no standard equivalent. Custom syntax like `- [!] text` works for round-trip.

### GFM features absent from ADF

Reference-style link definitions are resolved to inline links (definitions lost). Setext headings normalize to ATX style. Indented code blocks become fenced. Different list markers (`*` vs `-` vs `+`) normalize to one. Tight vs. loose list distinction is lost. Raw HTML passes through GFM but has no ADF equivalent unless specifically handled (`<u>` → underline mark, `<sub>`/`<sup>` → subsup mark).

---

## 3. Existing open-source implementations worth studying

### The most relevant: a Go + goldmark pair

**`ajbeck/adf-to-markdown` + `ajbeck/goldmark-adf`** (published March 2026, v1.1.0, MIT) is the most directly relevant implementation. It uses goldmark, targets bidirectional ADF ↔ Markdown conversion, and defines a principled custom extension syntax for ADF-only nodes:

| ADF Node | Markdown Syntax |
|----------|----------------|
| `status` | `[status:text\|color]` |
| `mention` | `@[name](id)` |
| `date` | `[date:timestamp]` |
| `panel` | `> [!NOTE]` (GitHub alerts) |
| `expand` | `<details><summary>` |
| `emoji` | `:shortcode:` |
| `inlineCard` | `[card:url]` |
| `decisionItem` | `- [!] text` |

It provides typed errors with ADF path tracking, built-in schema validation options, fuzzing, and benchmarks. **Caveat**: requires Go 1.25+ with `GOEXPERIMENT=jsonv2` — a bleeding-edge requirement. Evaluate whether to adopt it as a dependency, fork/vendor it, or use it as a reference.

### JavaScript ecosystem

**Atlassian's own `@atlaskit/editor-markdown-transformer`** goes MD→ADF only (no reverse), routes through ProseMirror, and has a massive dependency tree. **`marklassian`** (18.5K weekly npm downloads) is a lightweight MD→ADF converter that supports embedding raw ADF via `<adf>` HTML tags. **`extended-markdown-adf-parser`** achieves bidirectional conversion with 100% test coverage (399 tests) using custom fenced-block syntax (`~~~panel`, `~~~expand`) and HTML comment annotations. **`adf-to-md`** by julianlam handles the ADF→MD direction via tree traversal but discards unsupported elements.

### Python ecosystem

**`marklas`** (v0.5.1) is the most architecturally sophisticated bidirectional converter, using a Union AST approach (`Markdown ⇄ Union AST ⇄ ADF`) with HTML comment annotations for round-trip fidelity. It includes an LLM editing guide and reports **3–5.8× token reduction** vs raw ADF JSON — directly relevant for AI agent use cases. **`atlas_doc_parser`** focuses on ADF→MD for LLM/AI consumption with comprehensive node support.

### Other Go implementations

**`ankitpokhrel/jira-cli`** (pkg/adf) — the popular Jira CLI does ADF→MD for terminal display using a Translator pattern with Open/Close hooks, but is rough and not designed for round-trip fidelity. **`pinpt/adf`** converts ADF→HTML only and is stale (2020). No other significant Go ADF libraries exist.

### Rust

**`htmltoadf`** converts HTML→ADF (not Markdown). **`madfun`** does MD→ADF via the `markdown` crate but cannot convert back. Neither is bidirectional.

---

## 4. Hard problems that will bite you

### Mark ordering matters for serialization, not for ADF

ADF itself does not care about mark order in the JSON array — `[{type:"strong"},{type:"em"}]` renders identically to `[{type:"em"},{type:"strong"}]`. However, **mark order determines Markdown delimiter nesting**. `[strong, em]` produces `***text***`, while `[link, strong]` produces `[**text**](url)` vs `[strong, link]` producing `**[text](url)**`. The `code` mark is special: it should always be innermost to avoid `***` nesting issues. **Recommended sort order for serialization**: `link` → `strong` → `em` → `strike` → `code` (code innermost, link outermost). This matches ProseMirror's default schema ordering.

### Whitespace between marked text nodes

ADF does **not** normalize whitespace — text nodes contain literal content including spaces. No implicit space exists between adjacent text nodes. If two text nodes `{text:"Hello "}` and `{text:"world"}` are adjacent, the space is explicit in the first node. A known ProseMirror issue (#686): marks on whitespace-only nodes emit empty delimiters (`** **`). The fix is to "expel" enclosing whitespace from marks — move trailing/leading whitespace outside the mark delimiters. So `**bold **` should become `**bold** ` in Markdown output.

### Blockquote content restriction in ADF

Unlike Markdown blockquotes (which can contain any block-level content), **ADF blockquotes can only contain `paragraph` nodes**. This means when converting Markdown with code blocks or lists inside blockquotes to ADF, you must either flatten the content into paragraphs or restructure the document. This is a fundamental asymmetry that causes lossy conversion in the MD→ADF direction.

### Tables with block content

ADF table cells can contain lists, code blocks, panels, headings — essentially any block node. **GFM tables support only inline content.** This is the single hardest conversion problem. Strategies from existing implementations: use inline HTML (`<ul><li>item</li></ul>`, `<br>`) within cells, use inline code for code blocks, or flatten block content to single-line text representations. No approach is fully satisfactory.

### Mixed and nested lists

ADF represents nested lists as siblings of `paragraph` within `listItem`, not as children of `paragraph`. The correct ADF structure for a nested list is:

```json
{"type": "listItem", "content": [
  {"type": "paragraph", "content": [{"type": "text", "text": "Parent item"}]},
  {"type": "bulletList", "content": [
    {"type": "listItem", "content": [
      {"type": "paragraph", "content": [{"type": "text", "text": "Child item"}]}
    ]}
  ]}
]}
```

Mixed ordered/unordered nesting works in both formats. The common bug in converters is placing the nested list inside the paragraph rather than as a sibling.

### The atlassian-mcp-server lesson

The `atlassian-mcp-server` project (issue #1208) confirms that converting `@mention` text to plain `@name` in Markdown **does not trigger Jira notifications**. Only a proper ADF mention node with `attrs.id` (the Atlassian account ID) triggers notifications. This means for AI agent tools like jira4claude, lossy mention conversion has real functional consequences — not just formatting loss.

---

## 5. Architecture recommendations for jira4claude

### Type system: hybrid structs with `json.RawMessage` for attrs

```go
type ADFNode struct {
    Type    string          `json:"type"`
    Version int             `json:"version,omitempty"`
    Content []ADFNode       `json:"content,omitempty"`
    Text    string          `json:"text,omitempty"`
    Marks   []ADFMark       `json:"marks,omitempty"`
    Attrs   json.RawMessage `json:"attrs,omitempty"`
}

type ADFMark struct {
    Type  string          `json:"type"`
    Attrs json.RawMessage `json:"attrs,omitempty"`
}
```

Using `json.RawMessage` for attrs gives **perfect JSON round-trip fidelity** — unknown attrs pass through unmodified. Add typed helper methods (`HeadingAttrs()`, `LinkAttrs()`, etc.) for known types. This pattern follows Kubernetes' `Unstructured` precedent: typed core with flexible accessors. Unknown node types survive round-trip without code changes, making the tool forward-compatible as Atlassian adds new node types.

### MD→ADF: goldmark custom Renderer (recommended)

Use goldmark's `renderer.NodeRenderer` interface with the enter/exit visitor pattern. Register per-`NodeKind` handler functions. Build the ADF tree using a stack:

- **On enter**: push a new `ADFNode` onto the stack
- **On text/leaf**: append to the current top-of-stack's `Content`
- **On exit**: pop the node, append it to the new top-of-stack's `Content`
- **At end**: `stack[0]` is the complete `doc` node; serialize to JSON

This leverages goldmark's existing AST traversal, GFM extension support (tables, strikethrough, task lists), and priority-based handler registration. Enable the GFM extension with `extension.GFM`.

### ADF→MD: manual recursive walk (recommended)

Building a goldmark AST from ADF would require constructing segment-based nodes with a backing `[]byte` source — extremely awkward since you're generating text. Instead, use direct recursive rendering. The key challenge is managing indentation for nested lists and blockquotes, and properly spacing block elements with blank lines. The `ajbeck/adf-to-markdown` library validates this approach at scale.

### Testing strategy: four layers

1. **Table-driven unit tests** with golden files (`testdata/heading.md` → `testdata/heading.adf.json`) for deterministic coverage
2. **Round-trip property testing** with `pgregory.net/rapid` (preferred over gopter for better shrinking and Go fuzz integration) — compare **normalized ASTs**, not string equality, since Markdown has many equivalent representations
3. **Go built-in fuzzing** for both directions — verify no panics, valid JSON output, correct root structure
4. **ADF schema validation** using `santhosh-tekuri/jsonschema/v6` with an embedded copy of the Atlassian JSON Schema — validate all generated ADF in tests

### ADF schema validation

Download the schema from `http://go.atlassian.com/adf-json-schema`, embed it in the binary via `//go:embed`, and use it for test-time validation. Don't rely on the URL at runtime — it has been broken before. The `@atlaskit/adf-schema-generator` npm package is the authoritative source; the JSON Schema is derived from it. Use dual-layer validation: typed structs catch structural errors at compile time, JSON Schema catches semantic errors (invalid nesting, missing required attrs) at test time.

---

## 6. Prioritized implementation roadmap

### Phase 1: high-value easy wins (target ~85% coverage, ~2–3 days)

| Feature | Direction | Freq. | Impact | Difficulty |
|---------|-----------|-------|--------|------------|
| `hardBreak` | MD→ADF | Very high | Line breaks vanish | Easy |
| `rule` | MD→ADF | High | Dividers vanish | Easy |
| `strike` mark | Both | High | Formatting lost | Easy |
| `inlineCard` → link | ADF→MD | High | Smart links render as nothing | Easy |
| `emoji` → `:shortcode:` | ADF→MD | Medium | Emoji disappear | Easy |
| `table` | MD→ADF | High | Tables can't round-trip | Medium |
| `taskList`/`taskItem` | MD→ADF | Medium | Task lists can't round-trip | Medium |
| Images (`mediaSingle`) | MD→ADF | High | Images lost entirely | Medium |

After Phase 1, the converter handles all formatting that appears in the vast majority of Jira tickets: headings, paragraphs, all text formatting marks, links, images, code blocks, lists (with nesting), blockquotes, tables, task lists, horizontal rules, and line breaks.

### Phase 2: medium-value completeness (target ~95% coverage, ~3–5 days)

| Feature | Direction | Approach |
|---------|-----------|----------|
| `panel` | Both | GitHub alert syntax (`> [!NOTE]`, etc.) |
| `expand` | Both | `<details><summary>` HTML |
| `mention` | Both | `@[name](accountId)` for round-trip, `@name` for lossy |
| `status` | ADF→MD | `[status:text\|color]` or bold text with emoji |
| `date` | ADF→MD | Formatted date string |
| `underline` mark | ADF→MD | `<u>text</u>` HTML |
| `subsup` mark | ADF→MD | `<sub>`/`<sup>` HTML |
| `textColor`/`backgroundColor` | ADF→MD | Drop silently (acceptable loss) |
| Table header distinction | Both | First row as `tableHeader` |
| `orderedList` start number | MD→ADF | Populate `attrs.order` |
| Multiple marks on text | Both | Consistent mark ordering |
| Nested list edge cases | Both | Correct sibling placement |

### Phase 3: full spec coverage (target ~100%, ~5–10 days, diminishing returns)

`layoutSection`/`layoutColumn` (flatten to sequential), `decisionList` (custom syntax `- [!] text`), `mediaGroup`, `nestedExpand`, extension nodes (preserve as raw ADF JSON), table `colspan`/`rowspan` (HTML tables or accept loss), `annotation` mark (drop), `border` mark (drop), `mediaInline`, `blockTaskItem`, `bodiedSyncBlock`/`syncBlock`, `multiBodiedExtension`/`extensionFrame`. Most of these appear in **less than 1% of real Jira content** — implement only when a user reports a need.

## Conclusion

The ADF specification is more constrained than Markdown in some dimensions (blockquotes can only hold paragraphs, code blocks can't have marks) and richer in others (panels, mentions, status badges, colored text, column layouts). The key architectural insight is that **bidirectional conversion should be best-effort and lossy by design**, with optional custom syntax extensions for round-trip fidelity when needed. For jira4claude specifically — where AI agents are the primary consumers — **token efficiency matters more than pixel-perfect rendering**. The `marklas` Python library reports 3–5.8× token reduction converting ADF to Markdown, which directly translates to lower LLM costs and better context utilization. The `ajbeck/adf-to-markdown` + `goldmark-adf` Go library pair, published just weeks ago, solves the exact same problem with the exact same technology stack and should be evaluated before building from scratch. Whether you adopt it, fork it, or use it as a reference, its custom extension syntax conventions and error handling patterns represent the current state of the art for Go-based ADF ↔ Markdown conversion.
