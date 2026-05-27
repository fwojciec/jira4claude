# Custom Fields on Create / Update

## Problem

`j4c issue create` and `issue update` expose a fixed allow-list of fields (`project`, `type`, `summary`, `description`, `priority`, `labels`, `parent`, `assignee`). Projects whose workflow marks custom fields as **required** are unfilable through `j4c` because the create call is rejected before an issue is made.

Concrete: the INT project requires four custom fields on every `Bug` (`customfield_10801` Urgency / Risk, `customfield_10938` blocking-a-deal, `customfield_10838` Product Area(s), `customfield_10768` shared-by-customer). Every INT bug filed via j4c has had to fall back to `--type=Task --labels=bug`, losing the issue type and all of its triage metadata.

Two gaps drive this:

1. There is no way to *supply* values for arbitrary fields on create or update.
2. There is no way to *discover* which fields exist, which are required, or what values they accept.

## Scope

Two new capabilities. Explicitly **out of scope**: a "smart" `--field name=value` flag that resolves human field names and value strings against createmeta. Agents read structured discovery output and emit pre-shaped JSON; the resolver layer is YAGNI for now.

1. **`j4c issue fields`** — calls Jira's createmeta or editmeta endpoint (mode depends on flags), lists fields with IDs, schema types, allowed values, and an `example` JSON snippet.
2. **`--field-json id=<json>`** — repeatable flag on `issue create` and `issue update`. Pass-through escape hatch; value is the exact JSON shape Jira expects.

## Design

### Domain Layer (root package)

New types in `issue.go`:

```go
type IssueField struct {
    ID            string              `json:"id"`
    Name          string              `json:"name"`
    Required      bool                `json:"required"`
    SchemaType    string              `json:"schemaType"`           // "string", "option", "array", "user", "number", "date", "priority", ...
    SchemaItems   string              `json:"schemaItems,omitempty"` // for SchemaType=="array": element type, e.g. "option"
    SchemaCustom  string              `json:"schemaCustom,omitempty"` // custom field type URI; empty for builtins
    AllowedValues []FieldAllowedValue `json:"allowedValues,omitempty"`
    Example       json.RawMessage     `json:"example,omitempty"`     // nil when SchemaType is unknown
}

type FieldAllowedValue struct {
    ID    string `json:"id,omitempty"` // empty when Jira returns a primitive-string allowed value
    Value string `json:"value"`        // populated from the entry's .value or .name, or from the primitive string itself
}
```

The struct tags fix the `json/printer.go` contract — without them, Go emits `"ID"`/`"SchemaType"` and never omits empty fields. The domain package already depends on `encoding/json` for `RawMessage`, so tags don't introduce a new dependency direction. (A separate JSON-projection DTO would have been the more isolationist choice but doubles the field surface for no boundary gain.)

`allowedValues` shapes Jira may return — the parser handles all of them. The rule is orthogonal: preserve `id` whenever present, take the display value from `value` if set otherwise `name`.

| Wire shape | Mapping |
|---|---|
| `["Foo", "Bar"]` (primitive strings) | one entry per string: `{ID:"", Value:"Foo"}` |
| `[{"value":"Foo"}]` | `{ID:"", Value:"Foo"}` |
| `[{"name":"High"}]` (priority, version, etc.) | `{ID:"", Value:"High"}` |
| `[{"id":"10500","value":"High"}]` | `{ID:"10500", Value:"High"}` |
| `[{"id":"10000","name":"frontend"}]` (components, versions) | `{ID:"10000", Value:"frontend"}` |

Cascading-select-style entries with nested `children` are flattened to their top-level value; full cascading support is out of scope.

`IssueService` grows two methods. Discovery is endpoint-dependent: Jira exposes creatable fields via createmeta and editable fields via editmeta, and editability is workflow/screen/permission-dependent — they don't coincide.

```go
GetCreateFields(ctx context.Context, projectKey, issueType string) ([]*IssueField, error)
GetEditFields(ctx context.Context, key string) ([]*IssueField, error)
```

Both return the same `[]*IssueField`. `IssueField` models "a settable field" generically; the two methods differ only in which endpoint they hit and how they unwrap the response.

Existing types grow a write-only map:

```go
type Issue struct {
    // ... existing fields ...
    CustomFields map[string]json.RawMessage // write-only; Get/List leave it empty
}

type IssueUpdate struct {
    // ... existing fields ...
    CustomFields map[string]json.RawMessage // nil = no change
}
```

`CustomFields` semantics: the map is populated by the caller on create/update. The HTTP layer merges it into the request body. `Get`/`List` do not populate it — read-side custom field exposure is a separate future feature.

Root package gains one new stdlib import (`encoding/json`) for `json.RawMessage`. No external dependency added.

### HTTP Layer

**New file** `http/field.go`:

Two endpoints, one shared inner parser. Both Jira responses converge on a `{"fields": {id: {required, name, schema, allowedValues, ...}}}` shape — createmeta nested under `projects[0].issuetypes[0]`, editmeta at the top level.

`GetCreateFields` calls the legacy createmeta endpoint:

```
GET /rest/api/3/issue/createmeta?projectKeys={project}&issuetypeNames={type}&expand=projects.issuetypes.fields
```

This form accepts names directly (no issue-type-ID lookup, no pagination). Atlassian marks it deprecated; we accept the risk because (a) the newer paginated `/issuetypes/{id}` variant would force a second response shape, a name→ID dance, and a pagination loop, and (b) the inner shape matches editmeta, so we get one parser. If Atlassian removes it, migration is contained to this file.

`GetEditFields` calls:

```
GET /rest/api/3/issue/{key}/editmeta
```

Editmeta reflects what's editable *for this specific issue* given the current workflow, screens, and the caller's permissions. This is necessarily different from createmeta and must not be substituted for it.

**Defensive parsing.** Legacy createmeta does *not* reliably 404 on unknown project/issuetype filters — it returns 200 with an empty `projects` array or empty `issuetypes`. Parser must:

- Check `len(projects) > 0` before indexing; on empty return `ENotFound` quoting the project key.
- Check `len(projects[0].issuetypes) > 0`; on empty return `ENotFound` quoting the issue type and project.
- Never bare-index — empty result is an error, not a panic and not a successful empty list.

**Field ID source.** In both endpoint shapes, `fields` is a JSON object keyed by field id (`"customfield_10801": {...}`). The id lives in the map key, not in the entry body. The shared parser extracts it from the key.

Shared inner parser signature:

```go
func parseFieldsMap(raw map[string]json.RawMessage) ([]*IssueField, error)
```

The two outer parsers extract the inner map (different nesting per endpoint) and call this. Per entry: pull `name`, `required`, `schema.type`, `schema.items`, `schema.custom`, `allowedValues` from the entry body; pull `id` from the map key. Note `schema.items` is a *string* (e.g. `"option"`, `"string"`) in Jira's response, not a nested object — array fields look like `"schema": {"type": "array", "items": "option"}`. The parser reads it as a string. Generate `Example` from `SchemaType` / `SchemaItems` / `SchemaCustom`:

| schemaType | items | schema.custom suffix | Example JSON |
|---|---|---|---|
| `string` | — | `:textarea` | `{"type":"doc","version":1,"content":[{"type":"paragraph","content":[{"type":"text","text":"..."}]}]}` |
| `string` | — | anything else (incl. empty) | `"..."` |
| `number` | — | — | `0` |
| `date` | — | — | `"2026-05-27"` |
| `datetime` | — | — | `"2026-05-27T12:00:00.000+0000"` |
| `option` (allowedValues present) | — | — | `{"value":"<first allowedValue>"}` |
| `option` (no allowedValues) | — | — | `{"value":"..."}` |
| `array` | `option` | — | `[{"value":"<first>"}]` |
| `array` | `string` | — | `["..."]` |
| `user` | — | — | `{"accountId":"..."}` |
| `priority` | — | — | `{"name":"High"}` |
| anything else | — | — | omitted |

The textarea distinction matters: Jira renders textarea custom fields through ADF, not plain strings. Both textarea and single-line text fields have `schemaType == "string"`; the only signal that disambiguates them is `schema.custom` (e.g., `com.atlassian.jira.plugin.system.customfieldtypes:textarea` vs `:textfield`). Matching on the `:textarea` suffix avoids tying us to the full URI prefix in case Atlassian ever versions it.

Unknown shapes get no `Example` rather than a guess — honest about coverage gaps.

**Modified file** `http/request.go`:

`createFields` and `updateFields` each gain a non-marshaled custom-field map:

```go
CustomFields map[string]json.RawMessage `json:"-"`
```

`createRequest` and `updateRequest` get custom `MarshalJSON` methods that merge typed fields with the custom map into a single `"fields":{…}` object. Sketch:

```go
func (r createRequest) MarshalJSON() ([]byte, error) {
    type alias createRequest
    base, err := json.Marshal(alias(r))
    if err != nil { return nil, err }

    if len(r.Fields.CustomFields) == 0 {
        return base, nil
    }

    var wrapper struct {
        Fields map[string]json.RawMessage `json:"fields"`
    }
    if err := json.Unmarshal(base, &wrapper); err != nil { return nil, err }
    for k, v := range r.Fields.CustomFields {
        wrapper.Fields[k] = v // custom wins on collision
    }
    return json.Marshal(wrapper)
}
```

"Custom wins on collision" is documented behavior — agents get a true escape hatch even for fields with typed flags. In practice the CLI's `required:""` on `--summary` prevents the most likely accidental collision.

When `CustomFields` is empty, the marshal path is byte-identical to today. This is the non-regression invariant.

**Modified file** `http/issue.go`:

`Create` and `Update` pass `issue.CustomFields` / `update.CustomFields` into the `createFields` / `updateFields` struct. No other change.

### CLI Layer

**New command** `cmd/j4c/issue.go`:

```go
type IssueFieldsCmd struct {
    Project string `help:"Project key (for create-field discovery)" short:"p" xor:"mode-project"`
    Type    string `help:"Issue type (for create-field discovery)" short:"t" default:"Task" xor:"mode-type"`
    Key     string `help:"Issue key (for edit-field discovery)" short:"k" xor:"mode-project,mode-type"`
    All     bool   `help:"Include all fields (default: required + custom only)"`
}
```

Kong's `xor` rejects any pair of *user-set* flags sharing the same label. Putting `Project`, `Type`, and `Key` all in one label would reject the primary case `--project=INT --type=Bug`. Instead, two pairwise groups: `Key` conflicts with `Project` (via `mode-project`) and with `Type` (via `mode-type`), but `Project` and `Type` don't conflict with each other. `Type`'s default of `Task` is not user-set, so it doesn't conflict with `--key` on its own.

CLI parser tests must cover all three pairings: `--key + --project` rejected, `--key + --type` rejected, `--project + --type` accepted.

Two modes via the Kong `xor` labels declared on the struct above:

- `--key=INT-1118` → edit discovery via `GetEditFields`.
- `--project=INT --type=Bug` (or default) → create discovery via `GetCreateFields`.
- `--project` falls back to `ctx.Config.Project`; `--type` defaults to `Task`. So `j4c issue fields` with no flags = create discovery for the configured project's Task type.

Output goes through new `IssuePrinter.Fields(view IssueFieldsView)` method. The view carries the source label (`"INT / Bug"` or `"INT-1118 (edit)"`) plus the field list.

Default filter (no `--all`): keep field if `Required` **or** `strings.HasPrefix(ID, "customfield_")`. With `--all`: emit everything.

**Modified commands** — add `FieldJSON` to both:

```go
type IssueCreateCmd struct {
    // ... existing ...
    FieldJSON []string `name:"field-json" help:"Set field by ID, value is JSON (repeatable). Example: customfield_10801='{\"value\":\"High\"}'"`
}

type IssueUpdateCmd struct {
    // ... existing ...
    FieldJSON []string `name:"field-json" help:"Set field by ID, value is JSON (repeatable)"`
}
```

**Shared helper** in `cmd/j4c/` (exported, matching the `ResolveAssignee` convention in `cmd/j4c/resolve.go` — tests run in `package main_test` per the repo's `testpackage` lint rule, so CLI helpers under test must be exported):

```go
func ParseFieldJSON(raws []string) (map[string]json.RawMessage, error)
```

Per input:
1. Split on the first `=`. Missing `=` → `EValidation` quoting the raw input.
2. Key must be non-empty.
3. Value must satisfy `json.Valid([]byte(v))`. Invalid → `EValidation` with key and parse context.
4. Duplicate keys across the flag's repetitions → `EValidation` (last-wins would be silently confusing).

No `customfield_` prefix requirement on the key. Users can override typed fields via `--field-json summary='"foo"'` if they really want — it's a documented footgun, not a hidden one.

### Output Layer

**New view struct** in `view.go` (or `fields_view.go`):

```go
type IssueFieldsView struct {
    Source string         // "INT / Bug" or "INT-1118 (edit)"
    Fields []*IssueField
}
```

Source label is constructed by `IssueFieldsCmd.Run` so the printer doesn't need to know about modes. Follows the existing view-struct pattern (`IssueView`, `CommentView`, `IssueListItem`).

**Modified** `printer.go` — `IssuePrinter` interface grows one method:

```go
type IssuePrinter interface {
    Issue(view IssueView)
    Issues(items []IssueListItem)
    Comment(view CommentView)
    Transitions(key string, ts []*Transition)
    Fields(view IssueFieldsView)  // new
}
```

**Modified** `markdown/printer.go`:

```
Fields for INT / Bug:        # or "Fields for INT-1118 (edit):"

REQUIRED
  customfield_10938  Is this blocking a deal...   option
    allowed: "Yes", "No"
  customfield_10801  Urgency / Risk               option
    allowed: "High", "Medium", "Low"
  customfield_10838  Product Area(s)              array<option>
    allowed: "Integrations", "API", "UI", ...
  customfield_10768  Was this shared by a cust…   option
    allowed: "Customer", "Prospect", "Internal"
  summary            Summary                       string
  issuetype          Issue Type                    issuetype

CUSTOM (optional)
  customfield_10010  Story Points                  number
```

Required block leads with customfields (the discoverable surface agents need), then builtins, then optional customs. `allowed:` line truncates past five values with `, …`.

**Modified** `json/printer.go`:

```json
[
  {
    "id": "customfield_10801",
    "name": "Urgency / Risk",
    "required": true,
    "schemaType": "option",
    "allowedValues": [
      {"id": "10500", "value": "High"},
      {"id": "10501", "value": "Medium"}
    ],
    "example": {"value": "High"}
  }
]
```

Direct projection of `IssueField` via its struct tags. `omitempty` on `schemaItems`, `schemaCustom`, `allowedValues`, and `example` keeps the output compact for fields where those don't apply. `id` on `FieldAllowedValue` is `omitempty` because primitive-string allowed values produce entries with no ID.

### Mock Layer

`mock/issue.go` gains:

```go
GetCreateFieldsFn func(ctx context.Context, projectKey, issueType string) ([]*jira4claude.IssueField, error)
GetEditFieldsFn   func(ctx context.Context, key string) ([]*jira4claude.IssueField, error)
```

## Usage

End-to-end agent flow for a previously unfilable INT Bug:

```bash
# 1. Discover
$ j4c issue fields --project=INT --type=Bug --json
[
  {"id":"customfield_10801","name":"Urgency / Risk","required":true,"schemaType":"option",
   "allowedValues":[{"id":"10500","value":"High"},...],"example":{"value":"High"}},
  ...
]

# 2. Create with the JSON shapes the discovery command surfaced
$ j4c issue create --project=INT --type=Bug --summary="Webhook 500s" \
    --field-json customfield_10801='{"value":"High"}' \
    --field-json customfield_10938='{"value":"Yes"}' \
    --field-json customfield_10838='[{"value":"Integrations"}]' \
    --field-json customfield_10768='{"value":"Customer"}'
```

## Testing

All new tests use `package foo_test` and `t.Parallel()` on every function and subtest.

**`http/field_test.go`** (tests exercise the shared parser indirectly through `GetCreateFields` / `GetEditFields` with fixtures — `parseFieldsMap` stays unexported per `testpackage`):

- Schema type variants via `GetCreateFields` fixture: `string` (textfield), `string` (textarea — produces ADF example), `number`, `date`, `datetime`, `option`, `array<option>`, `array<string>`, `user`, `priority`, unknown.
- `Example` generation per schema type, including the textarea-vs-textfield divergence and the unknown→nil case.
- `allowedValues` shape variants: missing, null, primitive-string array, object array with `value`, object array with `name`, object array with `id`+`value`, object array with `id`+`name`.
- Required-field flag mapping.
- Field id sourced from map key (not from an inside field).
- `GetCreateFields`: legacy createmeta fixture (`projects[0].issuetypes[0].fields`) parses correctly; empty `projects` → `ENotFound`; empty `issuetypes` → `ENotFound`.
- `GetEditFields`: editmeta fixture (`fields` at top level) parses correctly via the same inner parser — confirms shape convergence.

**`http/issue_test.go`** (extend):
- `Create` with `CustomFields` populated produces a request body where `fields.customfield_XXX` equals the raw input JSON.
- Same for `Update`.
- Empty `CustomFields` → byte-identical request body to current behavior (golden JSON).
- Collision: typed `Summary` *and* `CustomFields["summary"]` set → custom value lands in the request body (collision rule).

**`cmd/j4c/issue_test.go`** (extend):
- `ParseFieldJSON`: happy path, missing `=`, invalid JSON, duplicate key, empty key, value with embedded `=`.
- Plumbing: `--field-json` populates `Issue.CustomFields` / `IssueUpdate.CustomFields`, reaches the service via the mock.
- New `issue fields` command: filter (`--all` vs default), default project/type fallback, `--key` mode routes to `GetEditFieldsFn`, and three Kong-parser pairing tests — `--key + --project` rejected, `--key + --type` rejected, `--project + --type` accepted.

**`markdown/printer_test.go`** and **`json/printer_test.go`**:
- `Fields` output for each schema type variant, including unknown-schema rendering.

## Error Semantics

- CLI parse errors (`ParseFieldJSON`): `EValidation`, message quotes the offending input verbatim.
- Createmeta returns 200 with an empty/missing project or issuetype: parser returns `ENotFound` quoting the project key and (if applicable) issue type. Legacy createmeta does not 404 on bad filters; the parser must enforce this.
- Editmeta on a non-existent or inaccessible issue: bubble through `DoRequest`'s 404/403 mapping.
- Jira's existing required-field error from `Create` is unchanged — no regression for the issue's "what already works" section.

## Non-Regression

Before merge, confirm:

- `make validate` green (`gochecknoglobals`, `testpackage`, `errcheck`, `paralleltest`).
- Existing create/update calls with no `--field-json` produce byte-identical request bodies (golden test).
- Existing required-field error reporting on create is unchanged.

## Files Touched

```
issue.go                          # + IssueField, FieldAllowedValue, GetCreateFields + GetEditFields, CustomFields on Issue/IssueUpdate
printer.go                        # + Fields method on IssuePrinter
view.go (or fields_view.go)      # + IssueFieldsView
http/field.go                     # new; GetCreateFields, GetEditFields, shared parseFieldsMap
http/field_test.go                # new
http/request.go                   # + CustomFields json:"-" on *Fields; + MarshalJSON on *Request
http/issue.go                     # + pass CustomFields through in Create/Update
http/issue_test.go                # + custom-field merge tests, golden non-regression
mock/issue.go                     # + GetCreateFieldsFn, GetEditFieldsFn
cmd/j4c/issue.go                  # + IssueFieldsCmd, FieldJSON on Create/Update, ParseFieldJSON
cmd/j4c/issue_test.go             # + ParseFieldJSON tests, plumbing tests, issue fields tests
markdown/printer.go               # + Fields implementation
markdown/printer_test.go          # + Fields tests
json/printer.go                   # + Fields implementation
json/printer_test.go              # + Fields tests
```

## Future Work (not in this PR)

- Smart `--field name=value` with createmeta/editmeta-driven resolution (item #3 from the issue).
- Read-side custom field exposure on `Get`/`List` (populate `Issue.CustomFields` from the response).
- Createmeta response cache (today every `issue fields` call is one round trip; agents using it in a loop would benefit).
- Migration off legacy createmeta if Atlassian removes it: the newer paginated `/issuetypes/{id}` endpoint requires (a) name→ID resolution for the issue type, (b) a pagination loop, and (c) a second response parser shape (array of `fieldId`-tagged entries vs. the map-keyed-by-id shape used by editmeta and legacy createmeta). Contained to `http/field.go`.
