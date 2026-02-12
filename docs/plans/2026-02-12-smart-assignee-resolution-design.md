# Smart Assignee Resolution

## Problem

AI agents using `j4c` can't assign issues by email or self-assign. The `issue assign` command requires a Jira account ID, forcing agents to call the Jira REST API directly to resolve emails. Additionally, `issue create` has no assignee flag, requiring a separate `issue assign` call after creation.

## Design

### Approach: Smart Single Flag

All assignee-accepting commands use the same resolution logic on a single `--assignee` flag:

- `""` (omitted) → no assignment / unassign
- `"me"` → resolve authenticated user via `/rest/api/3/myself`
- contains `@` → resolve by email via `/rest/api/3/user/search`
- anything else → pass through as account ID

Jira account IDs are 24-char hex strings, so `"me"` is unambiguous.

### Commands Affected

| Command | Flag | Behavior |
|---------|------|----------|
| `issue assign KEY -a me` | `--assignee` / `-a` (renamed from `--account-id`) | resolve then assign |
| `issue create -s "..." -A me` | `--assignee` / `-A` (new) | create, then assign via post-create call |
| `issue update KEY -a me` | `--assignee` / `-a` (existing, gains resolution) | resolve then update |

### Domain Layer

New `UserService` interface in root package (`user.go`):

```go
type UserService interface {
    GetMyself(ctx context.Context) (*User, error)
    FindUsers(ctx context.Context, query string) ([]*User, error)
}
```

Reuses existing `User` struct. No new domain types.

### HTTP Layer

New `http/user.go` with `UserService` struct:

- `GetMyself` → `GET /rest/api/3/myself`
- `FindUsers` → `GET /rest/api/3/user/search?query={value}`

Returns error when `FindUsers` yields zero results.

### CLI Layer

Shared helper in `cmd/j4c/`:

```go
func resolveAssignee(ctx context.Context, value string, userSvc UserService) (string, error)
```

- `issue assign`: rename `AccountID` field to `Assignee` (`--assignee` / `-a`)
- `issue create`: add `Assignee string` field (`--assignee` / `-A`); post-create assign
- `issue update`: existing `Assignee` field gains smart resolution

`IssueContext` gets new `UserService jira4claude.UserService` field. Wired in `main.go`.

### Mock Layer

New `mock/user.go` with `GetMyselfFn` / `FindUsersFn` function fields, matching existing mock conventions.

## Testing

### HTTP tests (`http/user_test.go`)

- GetMyself returns authenticated user
- GetMyself handles API error
- FindUsers returns matching users for email query
- FindUsers returns error when no users found
- FindUsers handles API error

### Command tests (`cmd/j4c/issue_test.go`)

**issue assign:**
- Assigns with `--assignee me` (GetMyself → Assign)
- Assigns with `--assignee user@example.com` (FindUsers → Assign)
- Assigns with `--assignee ACCOUNT_ID` (Assign directly)
- Unassigns when `--assignee` omitted
- Returns error when email resolves to no user

**issue create:**
- Creates and assigns with `--assignee me`
- Creates and assigns with `--assignee user@example.com`
- Creates without assignment when `--assignee` omitted
- Returns error when assignment fails after creation

**issue update:**
- Updates with `--assignee me` (resolves then updates)
- Updates with `--assignee user@example.com` (resolves then updates)
- Existing account ID behavior preserved
