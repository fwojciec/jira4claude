# HTTP Response Handling Helpers Design

Extract common HTTP response handling patterns into reusable helpers.

## Problem

The `http` package has significant code duplication:
- Response handling pattern (Do → ReadAll → check status → parse error) appears 10+ times
- JSON request building pattern appears 5+ times
- `io.ReadAll()` errors ignored in 5 places (lines 321, 357, 528, 581, 774)
- URL paths built with string concatenation, no escaping

## Solution

Three new helpers in the `http` package.

### `doRequest()` - Response Handler

Method on `*Client` in `http/client.go`:

```go
func (c *Client) doRequest(req *http.Request, expectedStatus int) ([]byte, error)
```

Behavior:
- Executes request via `c.Do(req)`
- Reads body with explicit error handling (fixes ignored errors)
- Checks status code against expected
- Parses API errors on failure
- Returns body bytes on success, nil on error

### `newJSONRequest()` - Request Builder

Method on `*Client` in `http/client.go`:

```go
func (c *Client) newJSONRequest(ctx context.Context, method, path string, body any) (*http.Request, error)
```

Behavior:
- Marshals body to JSON
- Creates request with context
- Sets `Content-Type: application/json`
- Returns domain errors on failure

### `issuePath()` - URL Builder

Package-private function in `http/issue.go`:

```go
func issuePath(key string, segments ...string) string
```

Behavior:
- Builds `/rest/api/3/issue/{escaped-key}`
- Appends optional segments (e.g., `/comment`, `/transitions`)
- Uses `url.PathEscape()` for the key

## Example Transformation

Before (Get method, ~25 lines of boilerplate):
```go
func (s *IssueService) Get(ctx context.Context, key string) (*jira4claude.Issue, error) {
    req, err := http.NewRequestWithContext(ctx, http.MethodGet, "/rest/api/3/issue/"+key, nil)
    // ... 20 lines of error handling ...
    return parseIssueResponse(respBody)
}
```

After (~10 lines):
```go
func (s *IssueService) Get(ctx context.Context, key string) (*jira4claude.Issue, error) {
    req, err := http.NewRequestWithContext(ctx, http.MethodGet, issuePath(key), nil)
    if err != nil {
        return nil, &jira4claude.Error{Code: jira4claude.EInternal, Message: "failed to create request", Inner: err}
    }
    body, err := s.client.doRequest(req, http.StatusOK)
    if err != nil {
        return nil, err
    }
    return parseIssueResponse(body)
}
```

## Testing

New tests:
- `TestClient_doRequest` - success, HTTP errors, ReadAll failure, status mismatches
- `TestClient_newJSONRequest` - success, marshal failure, invalid method
- `Test_issuePath` - simple key, with segments, key needing escape

Existing tests unchanged - they validate through public methods.

## Files Changed

- `http/client.go` - Add `doRequest()`, `newJSONRequest()`
- `http/client_test.go` - Add helper tests
- `http/issue.go` - Add `issuePath()`, refactor all methods

## Validation

- `make validate` passes
- Reduced line count in `http/issue.go`
- All linter checks pass (especially `errcheck`)
- Existing tests still pass
