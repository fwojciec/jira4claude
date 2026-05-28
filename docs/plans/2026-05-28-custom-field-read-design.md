# Custom Field Read Support — Design

**Date:** 2026-05-28
**Status:** Approved, ready for implementation

## Problem

Recent work added the ability to **write** custom fields on issues via
`--field-json` on `issue create` and `issue update`. The **read** side was
deliberately deferred — `issue.go` even documents this:

```go
CustomFields map[string]json.RawMessage // write-only on Create; ... Get/List leave empty (read-side exposure is a separate future feature).
```

As a result, `issue view` never shows custom-field values. An agent that sets
Story Points or a select field cannot read it back, and cannot see custom
fields that were set elsewhere. This design closes that gap for `issue view`.

## Goals

- `issue view` displays **populated** custom fields, by default (no flag).
- Each field is shown with both its **friendly name** (for understanding) and
  its **raw value** keyed by field ID (for round-tripping back to
  `--field-json`).
- No regression to `issue list` (kept lean) or to existing output.

## Non-goals (YAGNI)

- Custom fields on `issue list` — `view`-only. `list` keeps its lean,
  hardcoded `fields=` whitelist for speed.
- Prettifying / transforming arbitrary value shapes. Values render as the raw
  API JSON. `{"value":"High"}` is shown as-is.
- Showing empty/null custom fields, or all settable fields (that is what
  `issue fields` already does).

## Decisions

| Question | Decision |
|----------|----------|
| Use case | Both: friendly names **and** round-trippable id/value |
| Which fields | Populated custom fields only (`customfield_` prefix, non-null) |
| Name resolution | `?expand=names` on GET — single response, always in sync, no caching |
| Command scope | `issue view` only |
| Output shape | Map keyed by field ID: `{id: {name, value}}` |

## Design

### 1. Domain model (`issue.go`)

Reads carry a name; writes don't — different shapes, so the write field stays
and a read field is added:

```go
// CustomFieldValue is a read-side custom field: its display name plus
// the raw API value. Keyed by field ID on the issue.
type CustomFieldValue struct {
    Name  string          `json:"name"`
    Value json.RawMessage `json:"value"`
}
```

On `Issue`:

```go
CustomFields     map[string]json.RawMessage  // write-only on Create/Update; from --field-json
ReadCustomFields map[string]CustomFieldValue // read-only; populated by Get via expand=names
```

### 2. HTTP parse layer (`http/issue.go`)

- `Get` request URL gains `?expand=names`. The response then carries a
  top-level `names` map: `{"customfield_10801": "Story Points", ...}`.
- `parseIssueResponse` does a second unmarshal of the same body into:
  ```go
  var raw struct {
      Names  map[string]string          `json:"names"`
      Fields map[string]json.RawMessage `json:"fields"`
  }
  ```
  Then, for every `Fields` key with the `customfield_` prefix and a non-`null`
  value, build `CustomFieldValue{Name: raw.Names[id], Value: rawValue}` into
  `issue.ReadCustomFields`.
- `List` is **untouched**: its whitelist excludes `customfield_*` and it sends
  no `expand=names`, so `ReadCustomFields` stays empty there.
  `parseIssueResponse` is shared, so this works with no list regression.

### 3. View / output layer (`view.go`)

`IssueView` gains:

```go
CustomFields map[string]CustomFieldValue `json:"customFields,omitempty"`
```

`ToIssueView` copies `issue.ReadCustomFields` straight through — no ADF
conversion, no warnings. Rendered output:

```json
"customFields": {
  "customfield_10801": { "name": "Story Points", "value": 5 },
  "customfield_10010": { "name": "Priority",     "value": { "value": "High" } }
}
```

### Edge cases

- **Empty** → `omitempty` drops the key; output identical to today.
- **Missing name** → `Name` is `""`; field still emitted (value is the point).
- **`null` value** → filtered at parse time; never reaches the view.
- **System fields** in the `fields` map → not captured (only `customfield_`).

## Testing (TDD; external `_test` packages; `t.Parallel()` throughout)

**Parse (`http/issue_test.go`)**
- Multiple populated custom fields (scalar, select object, array, user object)
  + `names` map → correct id→{name,value}; `Value` byte-equal to raw API JSON.
- `null`-valued custom field → excluded.
- System fields present → not captured.
- No custom fields / no `names` → `ReadCustomFields` empty, no panic.
- Custom field missing from `names` → captured with empty `Name`.
- `Get` request URL carries `expand=names`.
- List regression: search response (no `names`, whitelist fields) →
  `ReadCustomFields` empty for every issue.

**View (`view_test.go`)**
- `ToIssueView` copies `ReadCustomFields` through unchanged.
- Empty → `customFields` absent from JSON (`omitempty`).
- Populated → JSON matches the keyed-by-ID shape exactly.

**Gate:** `make validate` (build + lint + tests) before PR.

## Execution plan (subagents)

1. **Foundation** (main session): add `CustomFieldValue` type and the two
   `Issue` fields. Tiny, everything depends on it; commit first so agents start
   from a compiling base.
2. **Parallel subagents** (disjoint files, TDD each):
   - **Parse agent** → `http/issue.go` + `http/issue_test.go`.
   - **View agent** → `view.go` + `view_test.go`.
3. **Integrate** (main session): `make validate`, fix any issues, confirm
   `issue view` end-to-end, open PR.
