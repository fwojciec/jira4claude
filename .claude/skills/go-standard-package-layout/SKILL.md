---
name: go-standard-package-layout
description: Structure Go applications using Ben Johnson's Standard Package Layout. Use when creating new Go packages, adding features to Go applications, deciding where code belongs, naming packages/files, or when tempted to create packages named after concepts rather than dependencies. Enforces dependency-free domain core in root, dependency-named subpackages, and explicit main wiring.
---

# Ben Johnson's Standard Package Layout

**Core insight**: Packages in Go are layers, not groups. Dependencies flow one direction—toward the domain.

## The Four Principles

1. **Root package**: Domain core—types, interfaces, and pure logic with no external dependencies
2. **Subpackages by dependency**: Each external dependency gets its own package
3. **Shared mock package**: Simple manual mocks with function fields
4. **Main as composition root**: Wires all dependencies explicitly

## Package Naming Decision Tree

```
Need to add code. Where does it go?

Is it a pure domain concept (type, interface, error, or logic with no external deps)?
├─ YES → Root package
└─ NO → Does it depend on an external package?
         ├─ YES → Package named after that dependency
         │        (sqlite/, http/, redis/, kafka/)
         └─ NO → Does it wrap a stdlib package meaningfully?
                  ├─ YES → Package named after stdlib package
                  │        (csv/, json/, http/)
                  └─ NO → Is it a distinct capability/implementation strategy?
                           ├─ YES → Package named after capability
                           │        (inmem/, mock/)
                           └─ NO → Root package
```

## Root Package: The Domain Core

The root package is your application's functional core — domain types, interfaces, and pure logic that depends on nothing external. It contains:

```go
package myapp

// Domain entities
type User struct {
    ID        int
    Name      string
    Email     string
    CreatedAt time.Time
}

// Service interfaces - contracts, not implementations
type UserService interface {
    FindUserByID(ctx context.Context, id int) (*User, error)
    CreateUser(ctx context.Context, user *User) error
}

// Domain errors
const (
    ECONFLICT     = "conflict"
    ENOTFOUND     = "not_found"
    EUNAUTHORIZED = "unauthorized"
)

type Error struct {
    Code    string
    Message string
}

// Context helpers (cross-cutting, used by multiple packages)
func NewContextWithUser(ctx context.Context, user *User) context.Context
func UserFromContext(ctx context.Context) *User
```

**What belongs here**: Domain types, service interfaces, pure domain logic (validation, orchestration, state machines), error codes, context helpers — anything that depends only on stdlib.

**What does NOT belong**: Anything importing external packages. Database queries, HTTP handlers, encoding logic, API clients all go in dependency-named subpackages.

## Subpackage Organization

### Naming: Always After the Dependency

```
myapp/
├── myapp.go              # Domain types
├── user.go               # User entity + UserService interface
├── error.go              # Error codes and Error type
├── context.go            # Context helpers
├── sqlite/               # Named after modernc.org/sqlite or database/sql
│   ├── sqlite.go         # DB struct, Open(), Close(), transactions
│   ├── user.go           # UserService implementation
│   └── user_test.go
├── http/                  # Named after net/http (shadows stdlib - that's OK!)
│   ├── http.go           # Common HTTP utilities
│   ├── server.go         # Server struct, routes, middleware
│   ├── user.go           # User HTTP handlers
│   └── user_test.go
├── redis/                 # Named after go-redis
│   └── cache.go
├── csv/                   # Named after encoding/csv (stdlib counts!)
│   └── user.go           # CSV encoder for users
├── inmem/                 # Capability name (in-memory implementation)
│   └── event.go          # EventService with channels
├── mock/                  # Testing mocks
│   └── user.go
└── cmd/myappd/
    └── main.go           # Wires everything
```

### File Naming Within Packages

Files are named after the **domain entity** they implement:

```
sqlite/
├── sqlite.go       # Package-level: DB, Tx, Open(), migrations
├── user.go         # Implements UserService
├── user_test.go
├── dial.go         # Implements DialService
├── dial_test.go
└── migration/      # SQL migration files
    ├── 0001_init.sql
    └── 0002_add_events.sql
```

NOT: `sqlite/repository.go`, `sqlite/queries.go`, `sqlite/crud.go`

### Shared Utilities Location

**Package-named file** (`foo/foo.go`) contains shared utilities for the package:

```
bubbletea/
├── bubbletea.go    # Shared types, helpers used by multiple components
├── viewer.go       # Model component
├── story.go        # StoryModel component
└── render.go       # Internal shared rendering (extracted from components)
```

**When to use `foo.go`**: Cross-cutting utilities, shared types, common helpers that don't belong to a single entity.

**When to extract to separate file** (like `render.go`): Implementation code shared between multiple components that's too large for `foo.go`. Name after what it does, not a concept.

### Test Utilities Location

**Test package-named file** (`foo/foo_test.go`) contains shared test utilities:

```
bubbletea/
├── bubbletea_test.go       # Shared test helpers (trueColorRenderer, diff builders)
├── model_test.go           # Model tests
├── model_navigation_test.go
├── story_test.go           # StoryModel tests
└── testdata/               # Golden files, fixtures
```

This follows Go convention: `foo_test.go` in package `foo_test` is the entry point for the external test package, containing utilities used across test files.

**What goes in `foo_test.go`**:
- Test helper functions (`MustOpenDB`, `trueColorRenderer`)
- Shared test fixtures and builders
- Common test setup/teardown

**What does NOT go there**:
- Actual tests (those go in entity-named files)
- Test data (use `testdata/` directory)

### Interface Compliance Pattern

Every implementation file starts with compile-time verification:

```go
package sqlite

import "github.com/myorg/myapp"

// Ensure UserService implements the interface
var _ myapp.UserService = (*UserService)(nil)

type UserService struct {
    db *DB
}

func (s *UserService) FindUserByID(ctx context.Context, id int) (*myapp.User, error) {
    // Implementation using s.db
}
```

## Common Scenarios

### Standard Library Dependencies

Wrapping a stdlib package **does** count as a dependency:

```go
// csv/user.go - wraps encoding/csv
package csv

import (
    "encoding/csv"
    "github.com/myorg/myapp"
)

type UserEncoder struct {
    w *csv.Writer
}

func (e *UserEncoder) EncodeUser(u *myapp.User) error {
    return e.w.Write([]string{u.Name, u.Email})
}
```

### Operations Spanning Multiple Dependencies

When an operation needs both S3 and Postgres, you have options:

**Option 1: Layered wrapping** (preferred when one is primary)
```go
// postgres/attachment.go
type AttachmentService struct {
    db        *DB
    blobStore myapp.BlobStore  // Interface satisfied by s3.BlobStore
}
```

**Option 2: Orchestration in main** (when truly equal)
```go
// cmd/main.go
s3Store := s3.NewBlobStore(cfg)
pgStore := postgres.NewMetadataStore(db)
// Application logic coordinates both
```

**NOT**: Creating `attachment/` package that imports both.

### Implementations Without External Dependencies

Use capability-based naming:

```go
// inmem/event.go - no external dep, but distinct capability
package inmem

type EventService struct {
    mu   sync.Mutex
    subs map[int]map[*Subscription]struct{}
}
```

```go
// mock/user.go - testing capability
package mock

type UserService struct {
    FindUserByIDFn      func(ctx context.Context, id int) (*myapp.User, error)
    FindUserByIDInvoked bool
}

func (s *UserService) FindUserByID(ctx context.Context, id int) (*myapp.User, error) {
    s.FindUserByIDInvoked = true
    return s.FindUserByIDFn(ctx, id)
}
```

### The http Package Shadows stdlib

This is intentional and correct:

```go
// http/server.go
package http  // Yes, this shadows net/http

import (
    "net/http"  // Import the real one when needed
    "github.com/myorg/myapp"
)

type Server struct {
    UserService myapp.UserService
    Addr        string
}
```

## Main Package: The Composition Root

```go
// cmd/myappd/main.go
package main

import (
    "github.com/myorg/myapp/http"
    "github.com/myorg/myapp/sqlite"
    "github.com/myorg/myapp/inmem"
)

type Main struct {
    DB         *sqlite.DB
    HTTPServer *http.Server
}

func (m *Main) Run(ctx context.Context) error {
    // Open database
    m.DB = sqlite.NewDB(m.Config.DSN)
    if err := m.DB.Open(); err != nil {
        return err
    }

    // Create services
    userService := sqlite.NewUserService(m.DB)
    eventService := inmem.NewEventService()

    // Wire HTTP server
    m.HTTPServer = &http.Server{
        Addr:         m.Config.Addr,
        UserService:  userService,
        EventService: eventService,
    }

    return m.HTTPServer.Open()
}
```

**Key points**:
- Main knows about ALL implementations
- Wiring is explicit—no magic, no reflection
- Dependency graph visible at compile time
- Testable: tests can call `Run()` with custom config

## Anti-Patterns to Avoid

### Concept-Named Packages (WRONG)

```
myapp/
├── fetcher/          # WRONG: named after concept
├── processor/        # WRONG: named after concept
├── manager/          # WRONG: named after concept
├── services/         # WRONG: layer grouping
├── models/           # WRONG: layer grouping
└── handlers/         # WRONG: layer grouping
```

### External Dependencies in Root Package (WRONG)

```go
// myapp.go - WRONG
package myapp

import "database/sql"  // External dependency in root!

type UserService struct {
    db *sql.DB  // Implementation detail in domain!
}
```

### Circular Dependencies (IMPOSSIBLE with this pattern)

If you follow "dependencies flow toward domain," circular deps can't happen:
- `sqlite/` imports `myapp` (domain) ✓
- `http/` imports `myapp` (domain) ✓
- `myapp` imports neither ✓
- `sqlite/` does NOT import `http/` ✓

## Decision Guide: New Package or Extend Existing?

**Create new package when**:
- Introducing a new external dependency
- Adding a completely different implementation strategy
- The dependency would be optional (not all deployments need it)

**Extend existing package when**:
- Adding a new domain entity to an existing dependency
- The implementation uses the same underlying resource (same DB, same HTTP server)

Example: Adding `DialService` to existing `sqlite` package:
```
sqlite/
├── sqlite.go
├── user.go       # existing
├── dial.go       # NEW - same package, new entity
└── dial_test.go  # NEW
```

## Mock Package Pattern

The `mock/` package provides simple, dependency-free mocks using function fields. This avoids external mocking frameworks while maintaining full flexibility.

### File Organization

One file per domain entity, mirroring the interface structure:

```
mock/
├── doc.go          # Package documentation
├── user.go         # Mock for UserService
├── project.go      # Mock for ProjectService
└── event.go        # Mock for EventService
```

### Mock Structure Pattern

Each mock struct has:
1. **Function fields** matching each interface method (suffixed with `Fn`)
2. **Compile-time interface verification** via blank identifier
3. **Delegation methods** that call the function fields

```go
// mock/user.go
package mock

import (
    "context"

    "github.com/myorg/myapp"
)

// Compile-time interface verification
var _ myapp.UserService = (*UserService)(nil)

// UserService is a mock implementation of myapp.UserService.
type UserService struct {
    FindUserByIDFn func(ctx context.Context, id int) (*myapp.User, error)
    CreateUserFn   func(ctx context.Context, user *myapp.User) error
    DeleteUserFn   func(ctx context.Context, id int) error
}

func (s *UserService) FindUserByID(ctx context.Context, id int) (*myapp.User, error) {
    return s.FindUserByIDFn(ctx, id)
}

func (s *UserService) CreateUser(ctx context.Context, user *myapp.User) error {
    return s.CreateUserFn(ctx, user)
}

func (s *UserService) DeleteUser(ctx context.Context, id int) error {
    return s.DeleteUserFn(ctx, id)
}
```

### Why Function Fields?

- **No framework dependencies**: Pure Go, no testify/mock or gomock
- **Explicit behavior**: Test author sees exactly what mock returns
- **Flexible per-test**: Each test sets only the functions it needs
- **Compile-time safety**: Interface changes break compilation, not tests at runtime

### Using Mocks in Tests

```go
// http/user_test.go
package http_test

func TestUserHandler_Get(t *testing.T) {
    t.Parallel()

    userService := &mock.UserService{
        FindUserByIDFn: func(ctx context.Context, id int) (*myapp.User, error) {
            return &myapp.User{ID: id, Name: "Alice"}, nil
        },
    }

    server := &http.Server{UserService: userService}
    // ... test HTTP handler
}

func TestUserHandler_NotFound(t *testing.T) {
    t.Parallel()

    userService := &mock.UserService{
        FindUserByIDFn: func(ctx context.Context, id int) (*myapp.User, error) {
            return nil, &myapp.Error{Code: myapp.ENOTFOUND}
        },
    }

    server := &http.Server{UserService: userService}
    // ... test 404 behavior
}
```

### Optional: Invocation Tracking

Add tracking fields when you need to verify a method was called:

```go
type UserService struct {
    FindUserByIDFn      func(ctx context.Context, id int) (*myapp.User, error)
    FindUserByIDInvoked bool
}

func (s *UserService) FindUserByID(ctx context.Context, id int) (*myapp.User, error) {
    s.FindUserByIDInvoked = true
    return s.FindUserByIDFn(ctx, id)
}
```

Use sparingly—prefer asserting on output rather than implementation details.

## Testing Pattern

```go
// sqlite/user_test.go
package sqlite_test  // External test package

func TestUserService_FindUserByID(t *testing.T) {
    t.Parallel()

    db := MustOpenDB(t)
    defer MustCloseDB(t, db)

    s := sqlite.NewUserService(db)

    // Create test data
    user := &myapp.User{Name: "test"}
    MustCreateUser(t, db, user)

    // Test
    got, err := s.FindUserByID(context.Background(), user.ID)
    require.NoError(t, err)
    assert.Equal(t, "test", got.Name)
}
```

Use `mock/` package to test components in isolation:

```go
// http/user_test.go
func TestUserHandler(t *testing.T) {
    t.Parallel()

    userService := &mock.UserService{
        FindUserByIDFn: func(ctx context.Context, id int) (*myapp.User, error) {
            return &myapp.User{ID: id, Name: "mock"}, nil
        },
    }

    server := &http.Server{UserService: userService}
    // Test HTTP handlers with mocked service
}
```

## Summary Checklist

When adding code, verify:

- [ ] Root package has NO external imports (stdlib only)
- [ ] Package names reflect dependencies, not concepts
- [ ] Each file is named after the domain entity it handles
- [ ] Interface compliance is verified at compile time
- [ ] Main explicitly wires all dependencies
- [ ] No circular dependencies exist (impossible if following pattern)
- [ ] Tests use external test packages (`_test` suffix)
