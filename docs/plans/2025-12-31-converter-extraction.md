# Converter Extraction Design

Extract GFM↔ADF conversion from HTTP package into a separate `adf` package with an injectable interface.

## Problem

The `http` package has a direct dependency on goldmark (markdown parser). Per Ben Johnson's pattern, `http` should depend on abstractions, not specific libraries.

## Design

### Interface in Root Package

```go
// converter.go
package jira4claude

// Converter handles conversion between markdown and Atlassian Document Format.
type Converter interface {
    ToADF(markdown string) map[string]any
    ToMarkdown(adf map[string]any) string
}
```

### Implementation Package

```
adf/
├── adf.go              # Converter implementation + goldmark wiring
├── adf_test.go         # Shared test helpers
├── to_adf.go           # GFMToADF logic (from http/gfm.go)
├── to_adf_test.go
├── to_markdown.go      # ADFToGFM logic (from http/gfm.go)
└── to_markdown_test.go
```

```go
// adf/adf.go
package adf

var _ jira4claude.Converter = (*Converter)(nil)

type Converter struct{}

func New() *Converter { return &Converter{} }

func (c *Converter) ToADF(markdown string) map[string]any {
    return toADF(markdown)
}

func (c *Converter) ToMarkdown(adf map[string]any) string {
    return toMarkdown(adf)
}
```

### Mock

```go
// mock/converter.go
package mock

var _ jira4claude.Converter = (*Converter)(nil)

type Converter struct {
    ToADFFn      func(markdown string) map[string]any
    ToMarkdownFn func(adf map[string]any) string
}

func (c *Converter) ToADF(markdown string) map[string]any {
    return c.ToADFFn(markdown)
}

func (c *Converter) ToMarkdown(adf map[string]any) string {
    return c.ToMarkdownFn(adf)
}
```

### HTTP Client Changes

```go
// http/client.go
type Client struct {
    baseURL    string
    project    string
    httpClient *http.Client
    converter  jira4claude.Converter  // injected
}

func NewClient(baseURL, project string, converter jira4claude.Converter) *Client
```

The `textOrADF` helper stays in HTTP as a method on Client:

```go
func (c *Client) textOrADF(text string) map[string]any {
    if adf := tryParseADF(text); adf != nil {
        return adf
    }
    return c.converter.ToADF(text)
}
```

### Wiring in Main

```go
// cmd/j4c/main.go
converter := adf.New()
client := http.NewClient(cfg.Server, cfg.Project, converter)
```

## Migration

| Current File | Action |
|--------------|--------|
| `http/gfm.go` | Move to `adf/to_adf.go` and `adf/to_markdown.go` |
| `http/gfm_test.go` | Move to `adf/*_test.go` |
| `http/adf.go` | Delete - inline `textOrADF` into `http/issue.go` |

## Tasks

1. Add `Converter` interface to root package
2. Create `adf` package with implementation
3. Add `mock.Converter`
4. Update HTTP client to accept converter
5. Update main to wire converter
6. Delete old files
