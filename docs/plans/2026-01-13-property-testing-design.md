# Property Testing Design

## Problem

PR reviews are catching bugs that `make validate` misses:
1. Non-deterministic map iteration (different output each run)
2. Weak test assertions (tests pass but don't verify actual behavior)

The goal is to encode these checks into the test suite so they're caught automatically.

## Solution

Add property-based testing using [Rapid](https://github.com/flyingmutant/rapid) to verify:
- Output determinism (same input → same output)
- Round-trip preservation (encode → decode → original)
- Nil safety (optional fields don't panic)

## Library Choice

**Rapid** (`pgregory.net/rapid`) over alternatives:
- Actively maintained (v1.2.0, Feb 2025)
- Clean API with generics
- Automatic shrinking (no user code needed)
- Zero dependencies
- Integrates with standard `go test`

## Design

### File Convention

Property tests go in existing `*_test.go` files alongside unit tests, grouped in a `// Property tests` section at the bottom.

### Custom Generators

Domain-specific generators in `testutil/generators.go` (new file):

```go
package testutil

import (
    "pgregory.net/rapid"
    jira4claude "github.com/filipekiss/jira4claude"
)

func IssueView() *rapid.Generator[jira4claude.IssueView] {
    return rapid.Custom(func(t *rapid.T) jira4claude.IssueView {
        return jira4claude.IssueView{
            Key:     rapid.StringMatching(`[A-Z]{2,5}-[0-9]{1,5}`).Draw(t, "key"),
            Summary: rapid.String().Draw(t, "summary"),
            Status:  rapid.SampledFrom([]string{"To Do", "In Progress", "Done"}).Draw(t, "status"),
            Type:    rapid.SampledFrom([]string{"Task", "Bug", "Story", "Epic"}).Draw(t, "type"),
            // ... other fields with appropriate generators
        }
    })
}
```

### Test Areas

#### 1. Printer Determinism (`json/printer_test.go`, `markdown/printer_test.go`)

```go
func TestPrinter_Determinism(t *testing.T) {
    t.Parallel()
    rapid.Check(t, func(t *rapid.T) {
        view := testutil.IssueView().Draw(t, "view")

        var buf1, buf2 bytes.Buffer
        json.NewPrinter(&buf1).Issue(view)
        json.NewPrinter(&buf2).Issue(view)

        if buf1.String() != buf2.String() {
            t.Fatalf("non-deterministic output for %+v", view)
        }
    })
}
```

Properties:
- Same `IssueView` → identical output every time
- Empty slices → `[]` not `null`, consistently
- Field ordering is stable

#### 2. Converter Round-Trips (`markdown/converter_test.go`)

```go
func TestConverter_RoundTrip(t *testing.T) {
    t.Parallel()
    rapid.Check(t, func(t *rapid.T) {
        md := testutil.SupportedMarkdown().Draw(t, "markdown")

        conv := markdown.New()
        adf, _ := conv.ToADF(md)
        result, _ := conv.ToMarkdown(adf)

        if md != result {
            t.Fatalf("round-trip failed: %q -> %q", md, result)
        }
    })
}
```

Properties:
- `ToADF(md)` → `ToMarkdown()` → original (for supported syntax)
- Idempotency: multiple round-trips produce same result
- No panics on any input

Generator constraint: Only generate markdown syntax that ADF supports.

#### 3. Domain Transformations (`jira/issue_test.go`)

```go
func TestMapIssue_Determinism(t *testing.T) {
    t.Parallel()
    rapid.Check(t, func(t *rapid.T) {
        apiIssue := testutil.APIIssue().Draw(t, "api_issue")

        result1 := jira.MapIssue(apiIssue)
        result2 := jira.MapIssue(apiIssue)

        if !reflect.DeepEqual(result1, result2) {
            t.Fatalf("non-deterministic mapping for %+v", apiIssue)
        }
    })
}
```

Properties:
- Same API response → identical domain model
- Nil/missing API fields don't panic
- All mapped fields are preserved

## Tasks

1. **Add Rapid dependency and generator utilities**
   - `go get pgregory.net/rapid`
   - Create `testutil/generators.go` with domain generators

2. **Add printer determinism property tests**
   - JSON printer: determinism, empty slice stability
   - Markdown printer: determinism, section ordering

3. **Add converter round-trip property tests**
   - Create supported-markdown generator
   - Test round-trip preservation and idempotency

4. **Add domain transformation property tests**
   - Create API response generators
   - Test mapping determinism and nil safety

## Success Criteria

- All property tests pass with default 100 iterations
- CI runs property tests as part of `make validate`
- No new dependencies beyond Rapid
