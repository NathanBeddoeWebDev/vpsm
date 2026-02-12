# AGENTS.md -- Coding Agent Instructions for vpsm

## Project Overview

**vpsm** is a Go CLI tool for managing VPS instances across cloud providers.
Built with Cobra (CLI framework), currently supporting Hetzner Cloud.
Module path: `nathanbeddoewebdev/vpsm`. Go 1.25+.

## Build / Test / Lint Commands

```bash
# Build (optimized, stripped binary)
make build

# Quick dev build (no optimization)
make dev

# Run ALL tests
make test                  # go test ./... -count=1

# Run tests verbose with race detector
make test-verbose          # go test ./... -v -count=1 -race

# Run a SINGLE test by name
go test ./internal/providers/ -run TestListServers_HappyPath -count=1

# Run a SINGLE test file's package
go test ./cmd/commands/server/ -count=1

# Lint
make lint                  # go vet + staticcheck (if installed)

# Clean build artifacts and caches
make clean
```

Always run `make test` after changes to verify nothing is broken.
Use `-count=1` to disable test caching (already included in Makefile targets).

## Project Structure

```
main.go                         # Entry point -- calls cmd.Execute()
cmd/
  root.go                       # Root Cobra command + Execute()
  commands/
    auth/                       # auth login, auth status
    server/                     # server list, server create, server delete
internal/
  domain/                       # Pure domain types and interfaces (no logic)
    provider.go                 # Provider, CatalogProvider interfaces
    server.go                   # Server struct
    catalog.go                  # Location, ServerTypeSpec, ImageSpec, SSHKeySpec
    create_opts.go              # CreateServerOpts
  providers/
    registry.go                 # Global provider registry (thread-safe)
    hetzner.go                  # Hetzner: core + ListServers
    hetzner_create.go           # Hetzner: CreateServer
    hetzner_catalog.go          # Hetzner: catalog methods
  services/auth/                # Token storage via OS keychain
  util/                         # Shared utilities
```

Key architectural rule: all non-CLI logic lives under `internal/` (unexportable).
Domain types are pure data -- no business logic in `internal/domain/`.

## Code Style Guidelines

### Imports

Organize imports in 3 groups separated by blank lines, in this order:
1. Standard library
2. Internal packages (`nathanbeddoewebdev/vpsm/...`)
3. External packages (`github.com/...`, `golang.org/x/...`)

```go
import (
    "context"
    "fmt"

    "nathanbeddoewebdev/vpsm/internal/domain"
    "nathanbeddoewebdev/vpsm/internal/services/auth"

    "github.com/hetznercloud/hcloud-go/v2/hcloud"
)
```

### Naming Conventions

| Element             | Convention     | Example                              |
|---------------------|----------------|--------------------------------------|
| Packages            | lowercase      | `providers`, `auth`, `domain`        |
| Types/Interfaces    | PascalCase     | `HetznerProvider`, `CatalogProvider` |
| Exported funcs      | PascalCase     | `NewHetznerProvider()`, `ListServers()` |
| Unexported funcs    | camelCase      | `toDomainServer()`, `parseLabels()`  |
| Variables           | camelCase      | `providerName`, `hzServers`          |
| Constants           | PascalCase     | `ServiceName`                        |
| Sentinel errors     | `Err` prefix   | `ErrTokenNotFound`                   |
| Files               | snake_case     | `hetzner_create.go`, `mock_store.go` |
| JSON tags           | snake_case     | `json:"public_ipv4,omitempty"`       |
| Test helpers        | `newTest`/`test` prefix | `newTestAPI()`, `testServerJSON()` |
| Test functions      | `Test<Func>_<Scenario>` | `TestListServers_HappyPath`    |
| Method receivers    | Short (1-2 chars) | `(h *HetznerProvider)`, `(k *KeyringStore)` |
| Constructors        | `New<Type>`    | `NewHetznerProvider()`, `NewMockStore()` |

No `I` prefix on interfaces. All method receivers are pointer receivers.

### Error Handling

- Return `(value, error)` -- standard Go pattern, no Result types.
- Wrap errors with context: `fmt.Errorf("failed to <action>: %w", err)`
- Use `%w` for wrappable errors (enables `errors.Is`/`errors.As` upstream).
- Sentinel errors for known conditions: `var ErrTokenNotFound = errors.New(...)`.
- `panic()` only for programmer bugs (nil factory, duplicate registration), never for user errors.
- CLI commands print errors to stderr via `fmt.Fprintf(cmd.ErrOrStderr(), "Error: %v\n", err)`.
- Only `cmd.Execute()` calls `os.Exit(1)` -- subcommands return early, never exit.

### Functions and Methods

- Use named functions for substantial logic; inline `func` literals for Cobra `Run` handlers.
- Constructors return `*Type`: `func NewHetznerProvider(...) *HetznerProvider`.
- Functional options pattern where applicable (e.g., `hcloud.ClientOption`).

### Comments

- Godoc comments on all exported symbols: `// FuncName does X.`
- Section dividers in long files: `// --- CatalogProvider implementation ---`
- Inline comments only for non-obvious logic.
- No TODO comments in committed code.

### Output and Logging

- No logging framework. Use `fmt.Fprintf`/`fmt.Fprintln` directly.
- Stdout for normal output, stderr for errors.
- In commands, use `cmd.OutOrStdout()` and `cmd.ErrOrStderr()` (testable).
- Never use `log.Fatal` or `log.Println`.

### Validation

- Cobra-level: `Args: cobra.ExactArgs(n)` for argument counts.
- Accumulate missing required flags into a `[]string` and print one combined error.
- Normalize all lookup keys via `util.NormalizeKey()` (lowercase + trim).
- Nil-check optional nested fields before accessing (e.g., `if s.ServerType != nil`).

## Testing Conventions

- Tests are co-located with source in the same package (white-box testing).
- Use Go's built-in `testing` package -- no testify.
- Structural comparison via `github.com/google/go-cmp/cmp` (`cmp.Diff`).
- HTTP tests use `net/http/httptest.Server` to mock external APIs.
- Mark helpers with `t.Helper()` for accurate failure line reporting.
- Use `t.Cleanup()` for teardown instead of `defer`.
- Table-driven tests for parameterized scenarios.
- Test naming: `Test<Function>_<Scenario>` (e.g., `TestCreateServer_WithSSHKeys`).
- Provider tests call `providers.Reset()` before registration, with `t.Cleanup(Reset)`.
- CLI tests inject `bytes.Buffer` via `cmd.SetOut`/`cmd.SetErr` to capture output.
- `MockStore` lives in `internal/services/auth/mock_store.go` (shared across packages).

## Adding a New Provider

1. Create `internal/providers/<name>.go` implementing `domain.Provider` (and optionally `domain.CatalogProvider`).
2. Create `internal/providers/<name>_test.go` with httptest-based tests.
3. Register via `Register("<name>", factory)` in an `init()` or explicit function.
4. Add the provider name to CLI help text and any validation lists.

## Adding a New CLI Command

1. Create a file under `cmd/commands/<resource>/<verb>.go`.
2. Export a `<Verb>Command() *cobra.Command` function.
3. Wire it into the parent command's `NewCommand()` via `AddCommand()`.
4. Use `cmd.OutOrStdout()`/`cmd.ErrOrStderr()` for all output (testability).
5. Add a corresponding `<verb>_test.go` with buffer-captured output assertions.
