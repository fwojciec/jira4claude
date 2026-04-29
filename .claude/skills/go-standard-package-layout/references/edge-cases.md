# Edge Cases and Advanced Patterns

## When Concepts Span Multiple Dependencies

**Scenario**: An "Attachment" feature needs S3 for blob storage and Postgres for metadata.

**Wrong approach**: Create `attachment/` package that imports both s3 and postgres packages.

**Correct approaches**:

### Option A: Primary dependency with injected interface

When one dependency is clearly primary (e.g., Postgres owns the transaction):

```go
// In root package
type BlobStore interface {
    Upload(ctx context.Context, key string, data io.Reader) error
    Download(ctx context.Context, key string) (io.ReadCloser, error)
}

type AttachmentService interface {
    CreateAttachment(ctx context.Context, a *Attachment, data io.Reader) error
}

// postgres/attachment.go
type AttachmentService struct {
    db        *DB
    blobStore myapp.BlobStore  // Interface - implementation injected
}

func (s *AttachmentService) CreateAttachment(ctx context.Context, a *Attachment, data io.Reader) error {
    tx, err := s.db.BeginTx(ctx, nil)
    if err != nil {
        return err
    }
    defer tx.Rollback()

    // Upload blob first
    key := fmt.Sprintf("attachments/%s", uuid.New())
    if err := s.blobStore.Upload(ctx, key, data); err != nil {
        return err
    }
    a.BlobKey = key

    // Store metadata
    if err := createAttachment(ctx, tx, a); err != nil {
        // Note: blob is orphaned on failure - handle with cleanup job
        return err
    }

    return tx.Commit()
}

// s3/blob.go
type BlobStore struct {
    client *s3.Client
    bucket string
}

var _ myapp.BlobStore = (*BlobStore)(nil)

// cmd/main.go
blobStore := s3.NewBlobStore(cfg.S3)
attachmentService := postgres.NewAttachmentService(db, blobStore)
```

### Option B: Coordination in main/application layer

When both dependencies are truly equal and you need sophisticated coordination:

```go
// cmd/main.go or a dedicated app/attachment.go
type AttachmentCoordinator struct {
    blobStore myapp.BlobStore
    metaStore myapp.AttachmentMetadataStore
}

func (c *AttachmentCoordinator) CreateAttachment(...) error {
    // Coordinate both stores with proper error handling
}
```

## Standard Library "Dependencies"

Not all stdlib usage creates a package. The test is: **does it define the package's identity?**

### Creates a package (meaningful wrapping):

```go
// csv/dial.go - encoding/csv defines what this package does
package csv

type DialEncoder struct {
    w *csv.Writer
}

// json/user.go - custom JSON encoding logic
package json

type UserEncoder struct {
    enc *json.Encoder
}
```

### Does NOT create a package (incidental usage):

```go
// sqlite/user.go - uses fmt.Sprintf, strings.Join, etc.
// These don't define the package's identity - sqlite does

package sqlite

import (
    "fmt"
    "strings"
    "database/sql"
)
```

**Rule of thumb**: If removing the stdlib import would fundamentally change what the package does, it deserves its own package.

## Cross-Cutting Concerns

Some code is used by multiple implementation packages but isn't a domain type.

### Context helpers - root package

```go
// context.go in root
package myapp

type contextKey int

const (
    userContextKey contextKey = iota
    flashContextKey
)

func NewContextWithUser(ctx context.Context, user *User) context.Context {
    return context.WithValue(ctx, userContextKey, user)
}

func UserFromContext(ctx context.Context) *User {
    user, _ := ctx.Value(userContextKey).(*User)
    return user
}
```

Why root? Both `http/` and `sqlite/` need these, and they're domain-level concepts (current user, flash messages).

### Shared utilities - evaluate carefully

```go
// If only http uses it → http/util.go
// If multiple packages need it and it's domain-related → root package
// If it's truly generic (not domain-specific) → internal/util/ or copy it
```

## The inmem Pattern

When you need an implementation with no external dependencies:

```go
// inmem/event.go
package inmem

import (
    "sync"
    "github.com/myorg/myapp"
)

var _ myapp.EventService = (*EventService)(nil)

type EventService struct {
    mu   sync.Mutex
    subs map[int]map[*Subscription]struct{}
}
```

Common `inmem/` use cases:
- Event pub/sub with channels
- Caching with `sync.Map`
- Rate limiting with token buckets
- Feature flags with simple maps

## Multiple Implementations of Same Interface

```
myapp/
├── user.go              # UserService interface
├── sqlite/
│   └── user.go          # SQLite implementation
├── postgres/
│   └── user.go          # Postgres implementation (different project, different DB)
├── mock/
│   └── user.go          # Test mock
└── cmd/myappd/
    └── main.go          # Choose which to wire
```

The interface in root enables this flexibility. Main chooses:

```go
// cmd/main.go
var userService myapp.UserService
if cfg.UsePostgres {
    userService = postgres.NewUserService(pgDB)
} else {
    userService = sqlite.NewUserService(sqliteDB)
}
```

## File Organization Within Large Packages

When a package grows large, organize by domain entity, not by function:

```
sqlite/
├── sqlite.go           # DB, Tx, migrations, shared utilities
├── user.go             # UserService
├── user_test.go
├── dial.go             # DialService
├── dial_test.go
├── dial_membership.go  # DialMembershipService
├── dial_membership_test.go
├── auth.go             # AuthService
├── auth_test.go
└── migration/
    ├── 0001_init.sql
    ├── 0002_add_dial.sql
    └── 0003_add_auth.sql
```

NOT:
```
sqlite/
├── models.go        # WRONG: grouping by type
├── queries.go       # WRONG: grouping by function
├── repository.go    # WRONG: abstract concept
└── helpers.go       # WRONG: vague
```

## When to Split a Package

Split when:
- A dependency becomes optional (not all binaries need it)
- Testing the package requires heavy setup unrelated to other code
- The package has grown to >2000 lines and has clear internal boundaries

Example: Extracting event handling from sqlite when it becomes complex:

Before: `sqlite/` handles events inline
After: `sqlite/` for persistence, `inmem/` for event pub/sub

## Domain Types That Reference Implementation

Sometimes domain types need fields that vary by implementation:

```go
// root package
type User struct {
    ID        int
    Name      string
    // Metadata can hold implementation-specific data
    Metadata  map[string]interface{}
}

// Or use separate types per concern
type UserFilter struct {
    ID     *int
    Email  *string
    Offset int
    Limit  int
}

type UserUpdate struct {
    Name  *string
    Email *string
}
```

The Filter and Update patterns keep domain types clean while enabling implementation-specific query/update semantics.

## Handling External APIs

External API clients follow the same pattern:

```go
myapp/
├── book.go             # Book, BookService interface
├── openai/
│   └── parser.go       # OpenAI-based BookParser
├── vertexai/
│   └── parser.go       # Vertex AI-based BookParser
└── cmd/myappd/
    └── main.go         # Choose provider
```

This enabled clean migration between providers with zero domain changes.

## Testing Considerations

### Test Helpers in Implementation Packages

```go
// sqlite/testing.go (or sqlite/sqlite_test.go)
func MustOpenDB(t *testing.T) *DB {
    t.Helper()
    db := NewDB(":memory:")
    if err := db.Open(); err != nil {
        t.Fatal(err)
    }
    return db
}

func MustCreateUser(t *testing.T, db *DB, u *myapp.User) {
    t.Helper()
    // ...
}
```

### Using mock Package

```go
// mock/user.go
type UserService struct {
    FindUserByIDFn      func(ctx context.Context, id int) (*myapp.User, error)
    FindUserByIDInvoked bool

    CreateUserFn      func(ctx context.Context, user *myapp.User) error
    CreateUserInvoked bool
}

func (s *UserService) FindUserByID(ctx context.Context, id int) (*myapp.User, error) {
    s.FindUserByIDInvoked = true
    return s.FindUserByIDFn(ctx, id)
}
```

This allows tests to:
1. Verify methods were called (`Invoked` flags)
2. Control return values (`Fn` fields)
3. Test error handling paths
