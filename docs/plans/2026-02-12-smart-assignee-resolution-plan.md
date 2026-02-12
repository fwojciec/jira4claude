# Smart Assignee Resolution Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add smart assignee resolution (`me`, email, account ID) to `issue assign`, `issue create`, and `issue update` commands.

**Architecture:** New `UserService` interface in root package with HTTP implementation for `/rest/api/3/myself` and `/rest/api/3/user/search`. Shared `resolveAssignee` helper in CLI layer used by all three commands. Post-create assign pattern for `issue create`.

**Tech Stack:** Go, Kong CLI framework, httptest for HTTP tests, testify for assertions, existing mock pattern.

---

### Task 1: Domain — UserService interface

**Files:**
- Create: `user.go`

**Step 1: Write `user.go`**

```go
package jira4claude

import "context"

// UserService defines operations for looking up Jira users.
type UserService interface {
	// GetMyself returns the currently authenticated user.
	GetMyself(ctx context.Context) (*User, error)

	// FindUsers searches for users matching the given query (e.g., email address).
	FindUsers(ctx context.Context, query string) ([]*User, error)
}
```

No new types needed — reuses existing `User` in `issue.go`.

**Step 2: Commit**

```
feat: add UserService interface
```

---

### Task 2: Mock — UserService mock

**Files:**
- Create: `mock/user.go`

**Step 1: Write `mock/user.go`**

```go
package mock

import (
	"context"

	"github.com/fwojciec/jira4claude"
)

// Compile-time interface verification.
var _ jira4claude.UserService = (*UserService)(nil)

// UserService is a mock implementation of jira4claude.UserService.
// Each method delegates to its corresponding function field.
// Calling a method without setting its function field will panic.
type UserService struct {
	GetMyselfFn func(ctx context.Context) (*jira4claude.User, error)
	FindUsersFn func(ctx context.Context, query string) ([]*jira4claude.User, error)
}

func (s *UserService) GetMyself(ctx context.Context) (*jira4claude.User, error) {
	return s.GetMyselfFn(ctx)
}

func (s *UserService) FindUsers(ctx context.Context, query string) ([]*jira4claude.User, error) {
	return s.FindUsersFn(ctx, query)
}
```

**Step 2: Run `go build ./mock/` to verify it compiles**

**Step 3: Commit**

```
feat: add UserService mock
```

---

### Task 3: HTTP — UserService implementation

**Files:**
- Create: `http/user.go`
- Test: `http/user_test.go`

**Step 1: Write failing test for GetMyself in `http/user_test.go`**

Follow the existing pattern in `http/issue_test.go`: use `httptest.NewServer`, `newTestClient` (from `http/client_test.go:429`), and `jirahttp` alias.

```go
package http_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fwojciec/jira4claude"
	jirahttp "github.com/fwojciec/jira4claude/http"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUserService_GetMyself(t *testing.T) {
	t.Parallel()

	t.Run("returns authenticated user", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet || r.URL.Path != "/rest/api/3/myself" {
				w.WriteHeader(http.StatusNotFound)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"accountId": "abc123",
				"displayName": "Filip W",
				"emailAddress": "filip@example.com"
			}`))
		}))
		defer server.Close()

		client := newTestClient(t, server.URL, "user@example.com", "api-token")
		svc := jirahttp.NewUserService(client)

		user, err := svc.GetMyself(context.Background())

		require.NoError(t, err)
		assert.Equal(t, "abc123", user.AccountID)
		assert.Equal(t, "Filip W", user.DisplayName)
		assert.Equal(t, "filip@example.com", user.Email)
	})

	t.Run("returns error on API failure", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"errorMessages": ["Unauthorized"], "errors": {}}`))
		}))
		defer server.Close()

		client := newTestClient(t, server.URL, "user@example.com", "api-token")
		svc := jirahttp.NewUserService(client)

		_, err := svc.GetMyself(context.Background())

		require.Error(t, err)
		assert.Equal(t, jira4claude.EUnauthorized, jira4claude.ErrorCode(err))
	})
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./http/ -run TestUserService_GetMyself -v`
Expected: FAIL — `NewUserService` not defined.

**Step 3: Write minimal GetMyself implementation in `http/user.go`**

```go
package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"

	"github.com/fwojciec/jira4claude"
)

// UserService implements jira4claude.UserService using the Jira REST API.
type UserService struct {
	client *Client
}

// Compile-time interface verification.
var _ jira4claude.UserService = (*UserService)(nil)

// NewUserService creates a new UserService using the provided HTTP client.
func NewUserService(client *Client) *UserService {
	return &UserService{client: client}
}

// GetMyself returns the currently authenticated user.
func (s *UserService) GetMyself(ctx context.Context) (*jira4claude.User, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "/rest/api/3/myself", nil)
	if err != nil {
		return nil, &jira4claude.Error{
			Code:    jira4claude.EInternal,
			Message: "failed to create request",
			Inner:   err,
		}
	}

	body, err := s.client.DoRequest(req, http.StatusOK)
	if err != nil {
		return nil, err
	}

	var resp userResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, &jira4claude.Error{
			Code:    jira4claude.EInternal,
			Message: "failed to parse response",
			Inner:   err,
		}
	}

	return mapUser(&resp), nil
}
```

Note: `userResponse` and `mapUser` already exist in `http/issue.go:395-497`.

Add a stub `FindUsers` that panics so the compile-time check passes:

```go
// FindUsers searches for users matching the given query.
func (s *UserService) FindUsers(ctx context.Context, query string) ([]*jira4claude.User, error) {
	panic("not implemented")
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./http/ -run TestUserService_GetMyself -v`
Expected: PASS

**Step 5: Write failing tests for FindUsers**

Add to `http/user_test.go`:

```go
func TestUserService_FindUsers(t *testing.T) {
	t.Parallel()

	t.Run("returns matching users for email query", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet || r.URL.Path != "/rest/api/3/user/search" {
				w.WriteHeader(http.StatusNotFound)
				return
			}

			assert.Equal(t, "filip@example.com", r.URL.Query().Get("query"))

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[{
				"accountId": "abc123",
				"displayName": "Filip W",
				"emailAddress": "filip@example.com"
			}]`))
		}))
		defer server.Close()

		client := newTestClient(t, server.URL, "user@example.com", "api-token")
		svc := jirahttp.NewUserService(client)

		users, err := svc.FindUsers(context.Background(), "filip@example.com")

		require.NoError(t, err)
		require.Len(t, users, 1)
		assert.Equal(t, "abc123", users[0].AccountID)
		assert.Equal(t, "filip@example.com", users[0].Email)
	})

	t.Run("returns error when no users found", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[]`))
		}))
		defer server.Close()

		client := newTestClient(t, server.URL, "user@example.com", "api-token")
		svc := jirahttp.NewUserService(client)

		_, err := svc.FindUsers(context.Background(), "nobody@example.com")

		require.Error(t, err)
		assert.Equal(t, jira4claude.ENotFound, jira4claude.ErrorCode(err))
	})

	t.Run("returns error on API failure", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"errorMessages": ["Forbidden"], "errors": {}}`))
		}))
		defer server.Close()

		client := newTestClient(t, server.URL, "user@example.com", "api-token")
		svc := jirahttp.NewUserService(client)

		_, err := svc.FindUsers(context.Background(), "filip@example.com")

		require.Error(t, err)
		assert.Equal(t, jira4claude.EForbidden, jira4claude.ErrorCode(err))
	})
}
```

**Step 6: Run test to verify it fails**

Run: `go test ./http/ -run TestUserService_FindUsers -v`
Expected: FAIL — panic "not implemented"

**Step 7: Implement FindUsers**

Replace the stub in `http/user.go`:

```go
// FindUsers searches for users matching the given query.
func (s *UserService) FindUsers(ctx context.Context, query string) ([]*jira4claude.User, error) {
	reqURL := "/rest/api/3/user/search?query=" + url.QueryEscape(query)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, &jira4claude.Error{
			Code:    jira4claude.EInternal,
			Message: "failed to create request",
			Inner:   err,
		}
	}

	body, err := s.client.DoRequest(req, http.StatusOK)
	if err != nil {
		return nil, err
	}

	var resp []userResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, &jira4claude.Error{
			Code:    jira4claude.EInternal,
			Message: "failed to parse response",
			Inner:   err,
		}
	}

	if len(resp) == 0 {
		return nil, &jira4claude.Error{
			Code:    jira4claude.ENotFound,
			Message: "no user found for " + query,
		}
	}

	users := make([]*jira4claude.User, len(resp))
	for i := range resp {
		users[i] = mapUser(&resp[i])
	}
	return users, nil
}
```

**Step 8: Run all user tests**

Run: `go test ./http/ -run TestUserService -v`
Expected: PASS

**Step 9: Commit**

```
feat: add UserService HTTP implementation
```

---

### Task 4: CLI — resolveAssignee helper and IssueContext wiring

**Files:**
- Create: `cmd/j4c/resolve.go`
- Modify: `cmd/j4c/main.go:35-39` (IssueContext), `cmd/j4c/main.go:115-119` (wiring)

**Step 1: Write `cmd/j4c/resolve.go`**

```go
package main

import (
	"context"
	"strings"

	"github.com/fwojciec/jira4claude"
)

// resolveAssignee resolves a smart assignee value to a Jira account ID.
//   - "" → return "" (no change / unassign)
//   - "me" → resolve via UserService.GetMyself
//   - contains "@" → resolve via UserService.FindUsers (first match)
//   - anything else → return as-is (account ID passthrough)
func resolveAssignee(ctx context.Context, value string, userSvc jira4claude.UserService) (string, error) {
	if value == "" {
		return "", nil
	}

	if value == "me" {
		user, err := userSvc.GetMyself(ctx)
		if err != nil {
			return "", err
		}
		return user.AccountID, nil
	}

	if strings.Contains(value, "@") {
		users, err := userSvc.FindUsers(ctx, value)
		if err != nil {
			return "", err
		}
		return users[0].AccountID, nil
	}

	return value, nil
}
```

**Step 2: Add UserService to IssueContext**

In `cmd/j4c/main.go`, add to the `IssueContext` struct (line 36):

```go
type IssueContext struct {
	Service     jira4claude.IssueService
	UserService jira4claude.UserService
	Printer     jira4claude.Printer
	Converter   jira4claude.Converter
	Config      *jira4claude.Config
}
```

**Step 3: Wire UserService in main()**

In `cmd/j4c/main.go`, after `svc := http.NewIssueService(client)` (line 115), add:

```go
userSvc := http.NewUserService(client)
```

Update `issueCtx` (line 119):

```go
issueCtx := &IssueContext{Service: svc, UserService: userSvc, Printer: printer, Converter: conv, Config: cfg}
```

**Step 4: Run `go build ./cmd/j4c/` to verify it compiles**

**Step 5: Commit**

```
feat: add resolveAssignee helper and wire UserService
```

---

### Task 5: CLI — Update issue assign command

**Files:**
- Modify: `cmd/j4c/issue.go:295-313`
- Test: `cmd/j4c/issue_test.go`

**Step 1: Write failing tests for smart assign resolution**

Add to `cmd/j4c/issue_test.go`, replacing the existing `TestIssueAssignCmd`:

```go
func TestIssueAssignCmd(t *testing.T) {
	t.Parallel()

	t.Run("assigns with account ID directly", func(t *testing.T) {
		t.Parallel()

		var capturedAccountID string
		svc := &mock.IssueService{
			AssignFn: func(ctx context.Context, key, accountID string) error {
				capturedAccountID = accountID
				return nil
			},
		}

		printer := &mock.Printer{}
		ctx := &main.IssueContext{
			Service:   svc,
			Printer:   printer,
			Converter: mockConverter(),
			Config:    &jira4claude.Config{Project: "TEST", Server: "https://test.atlassian.net"},
		}
		cmd := main.IssueAssignCmd{Key: "TEST-1", Assignee: "abc123"}
		err := cmd.Run(ctx)

		require.NoError(t, err)
		assert.Equal(t, "abc123", capturedAccountID)
		require.Len(t, printer.SuccessCalls, 1)
		assert.Equal(t, "Assigned:", printer.SuccessCalls[0].Msg)
	})

	t.Run("resolves me to authenticated user", func(t *testing.T) {
		t.Parallel()

		var capturedAccountID string
		svc := &mock.IssueService{
			AssignFn: func(ctx context.Context, key, accountID string) error {
				capturedAccountID = accountID
				return nil
			},
		}
		userSvc := &mock.UserService{
			GetMyselfFn: func(ctx context.Context) (*jira4claude.User, error) {
				return &jira4claude.User{AccountID: "myself-123"}, nil
			},
		}

		printer := &mock.Printer{}
		ctx := &main.IssueContext{
			Service:     svc,
			UserService: userSvc,
			Printer:     printer,
			Converter:   mockConverter(),
			Config:      &jira4claude.Config{Project: "TEST", Server: "https://test.atlassian.net"},
		}
		cmd := main.IssueAssignCmd{Key: "TEST-1", Assignee: "me"}
		err := cmd.Run(ctx)

		require.NoError(t, err)
		assert.Equal(t, "myself-123", capturedAccountID)
	})

	t.Run("resolves email to account ID", func(t *testing.T) {
		t.Parallel()

		var capturedAccountID string
		svc := &mock.IssueService{
			AssignFn: func(ctx context.Context, key, accountID string) error {
				capturedAccountID = accountID
				return nil
			},
		}
		userSvc := &mock.UserService{
			FindUsersFn: func(ctx context.Context, query string) ([]*jira4claude.User, error) {
				return []*jira4claude.User{{AccountID: "email-456"}}, nil
			},
		}

		printer := &mock.Printer{}
		ctx := &main.IssueContext{
			Service:     svc,
			UserService: userSvc,
			Printer:     printer,
			Converter:   mockConverter(),
			Config:      &jira4claude.Config{Project: "TEST", Server: "https://test.atlassian.net"},
		}
		cmd := main.IssueAssignCmd{Key: "TEST-1", Assignee: "user@example.com"}
		err := cmd.Run(ctx)

		require.NoError(t, err)
		assert.Equal(t, "email-456", capturedAccountID)
	})

	t.Run("unassigns when assignee is empty", func(t *testing.T) {
		t.Parallel()

		svc := &mock.IssueService{
			AssignFn: func(ctx context.Context, key, accountID string) error {
				return nil
			},
		}

		printer := &mock.Printer{}
		ctx := &main.IssueContext{
			Service:   svc,
			Printer:   printer,
			Converter: mockConverter(),
			Config:    &jira4claude.Config{Project: "TEST", Server: "https://test.atlassian.net"},
		}
		cmd := main.IssueAssignCmd{Key: "TEST-1", Assignee: ""}
		err := cmd.Run(ctx)

		require.NoError(t, err)
		require.Len(t, printer.SuccessCalls, 1)
		assert.Equal(t, "Unassigned:", printer.SuccessCalls[0].Msg)
	})

	t.Run("returns error when email resolves to no user", func(t *testing.T) {
		t.Parallel()

		svc := &mock.IssueService{}
		userSvc := &mock.UserService{
			FindUsersFn: func(ctx context.Context, query string) ([]*jira4claude.User, error) {
				return nil, &jira4claude.Error{Code: jira4claude.ENotFound, Message: "no user found for " + query}
			},
		}

		printer := &mock.Printer{}
		ctx := &main.IssueContext{
			Service:     svc,
			UserService: userSvc,
			Printer:     printer,
			Converter:   mockConverter(),
			Config:      &jira4claude.Config{Project: "TEST", Server: "https://test.atlassian.net"},
		}
		cmd := main.IssueAssignCmd{Key: "TEST-1", Assignee: "nobody@example.com"}
		err := cmd.Run(ctx)

		require.Error(t, err)
		assert.Equal(t, jira4claude.ENotFound, jira4claude.ErrorCode(err))
	})

	t.Run("returns error when service fails", func(t *testing.T) {
		t.Parallel()

		svc := &mock.IssueService{
			AssignFn: func(ctx context.Context, key, accountID string) error {
				return &jira4claude.Error{Code: jira4claude.ENotFound, Message: "issue not found"}
			},
		}

		printer := &mock.Printer{}
		ctx := &main.IssueContext{
			Service:   svc,
			Printer:   printer,
			Converter: mockConverter(),
			Config:    &jira4claude.Config{Project: "TEST", Server: "https://test.atlassian.net"},
		}
		cmd := main.IssueAssignCmd{Key: "NOTFOUND-1", Assignee: "abc123"}
		err := cmd.Run(ctx)

		require.Error(t, err)
		assert.Equal(t, jira4claude.ENotFound, jira4claude.ErrorCode(err))
	})
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./cmd/j4c/ -run TestIssueAssignCmd -v`
Expected: FAIL — `IssueAssignCmd` has field `AccountID`, not `Assignee`.

**Step 3: Update IssueAssignCmd**

In `cmd/j4c/issue.go`, replace lines 295-313:

```go
// IssueAssignCmd assigns an issue.
type IssueAssignCmd struct {
	Key      string `arg:"" help:"Issue key"`
	Assignee string `help:"Assignee (account ID, email, or 'me'; omit to unassign)" short:"a"`
}

// Run executes the assign command.
func (c *IssueAssignCmd) Run(ctx *IssueContext) error {
	accountID, err := resolveAssignee(context.Background(), c.Assignee, ctx.UserService)
	if err != nil {
		return err
	}

	if err := ctx.Service.Assign(context.Background(), c.Key, accountID); err != nil {
		return err
	}

	if accountID == "" {
		ctx.Printer.Success("Unassigned:", c.Key)
	} else {
		ctx.Printer.Success("Assigned:", c.Key)
	}
	return nil
}
```

**Step 4: Run tests to verify they pass**

Run: `go test ./cmd/j4c/ -run TestIssueAssignCmd -v`
Expected: PASS

**Step 5: Commit**

```
feat: add smart assignee resolution to issue assign
```

---

### Task 6: CLI — Add --assignee to issue create

**Files:**
- Modify: `cmd/j4c/issue.go:117-172`
- Test: `cmd/j4c/issue_test.go`

**Step 1: Write failing tests for create with assignee**

Add new subtests to `TestIssueCreateCmd` in `cmd/j4c/issue_test.go`:

```go
	t.Run("creates and assigns with assignee me", func(t *testing.T) {
		t.Parallel()

		var assignedKey, assignedAccountID string
		svc := &mock.IssueService{
			CreateFn: func(ctx context.Context, issue *jira4claude.Issue) (*jira4claude.Issue, error) {
				return &jira4claude.Issue{Key: "TEST-1"}, nil
			},
			AssignFn: func(ctx context.Context, key, accountID string) error {
				assignedKey = key
				assignedAccountID = accountID
				return nil
			},
		}
		userSvc := &mock.UserService{
			GetMyselfFn: func(ctx context.Context) (*jira4claude.User, error) {
				return &jira4claude.User{AccountID: "myself-123"}, nil
			},
		}

		printer := &mock.Printer{}
		ctx := &main.IssueContext{
			Service:     svc,
			UserService: userSvc,
			Printer:     printer,
			Converter:   mockConverter(),
			Config:      &jira4claude.Config{Project: "TEST", Server: "https://test.atlassian.net"},
		}
		cmd := main.IssueCreateCmd{
			Summary:  "Test issue",
			Assignee: "me",
		}
		err := cmd.Run(ctx)

		require.NoError(t, err)
		assert.Equal(t, "TEST-1", assignedKey)
		assert.Equal(t, "myself-123", assignedAccountID)
		require.Len(t, printer.SuccessCalls, 1)
		assert.Equal(t, "Created:", printer.SuccessCalls[0].Msg)
	})

	t.Run("creates and assigns with assignee email", func(t *testing.T) {
		t.Parallel()

		var assignedAccountID string
		svc := &mock.IssueService{
			CreateFn: func(ctx context.Context, issue *jira4claude.Issue) (*jira4claude.Issue, error) {
				return &jira4claude.Issue{Key: "TEST-1"}, nil
			},
			AssignFn: func(ctx context.Context, key, accountID string) error {
				assignedAccountID = accountID
				return nil
			},
		}
		userSvc := &mock.UserService{
			FindUsersFn: func(ctx context.Context, query string) ([]*jira4claude.User, error) {
				return []*jira4claude.User{{AccountID: "email-456"}}, nil
			},
		}

		printer := &mock.Printer{}
		ctx := &main.IssueContext{
			Service:     svc,
			UserService: userSvc,
			Printer:     printer,
			Converter:   mockConverter(),
			Config:      &jira4claude.Config{Project: "TEST", Server: "https://test.atlassian.net"},
		}
		cmd := main.IssueCreateCmd{
			Summary:  "Test issue",
			Assignee: "user@example.com",
		}
		err := cmd.Run(ctx)

		require.NoError(t, err)
		assert.Equal(t, "email-456", assignedAccountID)
	})

	t.Run("creates without assignment when assignee is empty", func(t *testing.T) {
		t.Parallel()

		assignCalled := false
		svc := &mock.IssueService{
			CreateFn: func(ctx context.Context, issue *jira4claude.Issue) (*jira4claude.Issue, error) {
				return &jira4claude.Issue{Key: "TEST-1"}, nil
			},
			AssignFn: func(ctx context.Context, key, accountID string) error {
				assignCalled = true
				return nil
			},
		}

		printer := &mock.Printer{}
		ctx := &main.IssueContext{
			Service:   svc,
			Printer:   printer,
			Converter: mockConverter(),
			Config:    &jira4claude.Config{Project: "TEST", Server: "https://test.atlassian.net"},
		}
		cmd := main.IssueCreateCmd{
			Summary: "Test issue",
		}
		err := cmd.Run(ctx)

		require.NoError(t, err)
		assert.False(t, assignCalled)
	})

	t.Run("returns error when assignment fails after creation", func(t *testing.T) {
		t.Parallel()

		svc := &mock.IssueService{
			CreateFn: func(ctx context.Context, issue *jira4claude.Issue) (*jira4claude.Issue, error) {
				return &jira4claude.Issue{Key: "TEST-1"}, nil
			},
			AssignFn: func(ctx context.Context, key, accountID string) error {
				return &jira4claude.Error{Code: jira4claude.EForbidden, Message: "cannot assign"}
			},
		}
		userSvc := &mock.UserService{
			GetMyselfFn: func(ctx context.Context) (*jira4claude.User, error) {
				return &jira4claude.User{AccountID: "myself-123"}, nil
			},
		}

		printer := &mock.Printer{}
		ctx := &main.IssueContext{
			Service:     svc,
			UserService: userSvc,
			Printer:     printer,
			Converter:   mockConverter(),
			Config:      &jira4claude.Config{Project: "TEST", Server: "https://test.atlassian.net"},
		}
		cmd := main.IssueCreateCmd{
			Summary:  "Test issue",
			Assignee: "me",
		}
		err := cmd.Run(ctx)

		require.Error(t, err)
		// Issue was created, but assignment failed — still print success for creation
		require.Len(t, printer.SuccessCalls, 1)
		assert.Equal(t, "Created:", printer.SuccessCalls[0].Msg)
	})
```

**Step 2: Run tests to verify they fail**

Run: `go test ./cmd/j4c/ -run TestIssueCreateCmd -v`
Expected: FAIL — `IssueCreateCmd` has no `Assignee` field.

**Step 3: Update IssueCreateCmd**

In `cmd/j4c/issue.go`, add `Assignee` field to struct (after line 125):

```go
type IssueCreateCmd struct {
	Project     string   `help:"Project key" short:"p"`
	Type        string   `help:"Issue type" short:"t" default:"Task"`
	Summary     string   `help:"Issue summary" short:"s" required:""`
	Description string   `help:"Issue description" short:"d"`
	Priority    string   `help:"Issue priority"`
	Labels      []string `help:"Issue labels" short:"l"`
	Parent      string   `help:"Parent issue key (creates a Subtask)" short:"P"`
	Assignee    string   `help:"Assignee (account ID, email, or 'me')" short:"A"`
}
```

Update the `Run` method — after `ctx.Printer.Success("Created:", created.Key)` (line 170), add post-create assign:

```go
	ctx.Printer.Success("Created:", created.Key)

	// Post-create assignment
	if c.Assignee != "" {
		accountID, err := resolveAssignee(context.Background(), c.Assignee, ctx.UserService)
		if err != nil {
			return err
		}
		if err := ctx.Service.Assign(context.Background(), created.Key, accountID); err != nil {
			return err
		}
	}

	return nil
```

**Step 4: Run tests to verify they pass**

Run: `go test ./cmd/j4c/ -run TestIssueCreateCmd -v`
Expected: PASS

**Step 5: Commit**

```
feat: add --assignee flag to issue create
```

---

### Task 7: CLI — Add smart resolution to issue update

**Files:**
- Modify: `cmd/j4c/issue.go:188-227`
- Test: `cmd/j4c/issue_test.go`

**Step 1: Write failing tests for update with smart assignee**

Add new subtests to `TestIssueUpdateCmd` in `cmd/j4c/issue_test.go`:

```go
	t.Run("resolves me for assignee update", func(t *testing.T) {
		t.Parallel()

		var capturedUpdate jira4claude.IssueUpdate
		svc := &mock.IssueService{
			UpdateFn: func(ctx context.Context, key string, update jira4claude.IssueUpdate) (*jira4claude.Issue, error) {
				capturedUpdate = update
				return makeIssue(key), nil
			},
		}
		userSvc := &mock.UserService{
			GetMyselfFn: func(ctx context.Context) (*jira4claude.User, error) {
				return &jira4claude.User{AccountID: "myself-123"}, nil
			},
		}

		printer := &mock.Printer{}
		assignee := "me"
		ctx := &main.IssueContext{
			Service:     svc,
			UserService: userSvc,
			Printer:     printer,
			Converter:   mockConverter(),
			Config:      &jira4claude.Config{Project: "TEST", Server: "https://test.atlassian.net"},
		}
		cmd := main.IssueUpdateCmd{Key: "TEST-1", Assignee: &assignee}
		err := cmd.Run(ctx)

		require.NoError(t, err)
		require.NotNil(t, capturedUpdate.Assignee)
		assert.Equal(t, "myself-123", *capturedUpdate.Assignee)
	})

	t.Run("resolves email for assignee update", func(t *testing.T) {
		t.Parallel()

		var capturedUpdate jira4claude.IssueUpdate
		svc := &mock.IssueService{
			UpdateFn: func(ctx context.Context, key string, update jira4claude.IssueUpdate) (*jira4claude.Issue, error) {
				capturedUpdate = update
				return makeIssue(key), nil
			},
		}
		userSvc := &mock.UserService{
			FindUsersFn: func(ctx context.Context, query string) ([]*jira4claude.User, error) {
				return []*jira4claude.User{{AccountID: "email-456"}}, nil
			},
		}

		printer := &mock.Printer{}
		assignee := "user@example.com"
		ctx := &main.IssueContext{
			Service:     svc,
			UserService: userSvc,
			Printer:     printer,
			Converter:   mockConverter(),
			Config:      &jira4claude.Config{Project: "TEST", Server: "https://test.atlassian.net"},
		}
		cmd := main.IssueUpdateCmd{Key: "TEST-1", Assignee: &assignee}
		err := cmd.Run(ctx)

		require.NoError(t, err)
		require.NotNil(t, capturedUpdate.Assignee)
		assert.Equal(t, "email-456", *capturedUpdate.Assignee)
	})

	t.Run("passes account ID through for assignee update", func(t *testing.T) {
		t.Parallel()

		var capturedUpdate jira4claude.IssueUpdate
		svc := &mock.IssueService{
			UpdateFn: func(ctx context.Context, key string, update jira4claude.IssueUpdate) (*jira4claude.Issue, error) {
				capturedUpdate = update
				return makeIssue(key), nil
			},
		}

		printer := &mock.Printer{}
		assignee := "abc123"
		ctx := &main.IssueContext{
			Service:   svc,
			Printer:   printer,
			Converter: mockConverter(),
			Config:    &jira4claude.Config{Project: "TEST", Server: "https://test.atlassian.net"},
		}
		cmd := main.IssueUpdateCmd{Key: "TEST-1", Assignee: &assignee}
		err := cmd.Run(ctx)

		require.NoError(t, err)
		require.NotNil(t, capturedUpdate.Assignee)
		assert.Equal(t, "abc123", *capturedUpdate.Assignee)
	})
```

**Step 2: Run tests to verify they fail**

Run: `go test ./cmd/j4c/ -run TestIssueUpdateCmd -v`
Expected: FAIL — `IssueContext` missing `UserService` in some tests, or update doesn't resolve.

**Step 3: Update IssueUpdateCmd.Run**

In `cmd/j4c/issue.go`, add resolution before building the update struct. Insert before `update := jira4claude.IssueUpdate{` (line 199):

```go
	// Resolve smart assignee value
	if c.Assignee != nil {
		resolved, err := resolveAssignee(context.Background(), *c.Assignee, ctx.UserService)
		if err != nil {
			return err
		}
		c.Assignee = &resolved
	}
```

The rest of the method stays the same since it already uses `c.Assignee`.

**Step 4: Run tests to verify they pass**

Run: `go test ./cmd/j4c/ -run TestIssueUpdateCmd -v`
Expected: PASS

**Step 5: Commit**

```
feat: add smart assignee resolution to issue update
```

---

### Task 8: Validate everything

**Step 1: Run full test suite**

Run: `make validate`
Expected: PASS — linting, all tests pass.

**Step 2: Quick manual smoke test**

Run: `go install ./cmd/j4c && j4c issue assign --help`
Expected: Shows `--assignee` / `-a` flag (not `--account-id`).

Run: `j4c issue create --help`
Expected: Shows `--assignee` / `-A` flag.

**Step 3: Commit any fixes if needed, then final commit**

```
chore: validate smart assignee resolution
```
