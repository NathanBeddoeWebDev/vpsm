# Contributing to vpsm

Thanks for contributing to `vpsm`.

## Project Overview

`vpsm` is a Go CLI for managing VPS infrastructure across cloud providers.

- Language: Go (`1.25+`)
- CLI framework: Cobra
- Current primary resource: servers
- Planned resources: DNS, volumes, databases

## Local Setup

### Prerequisites

- Go 1.25 or newer
- `make`
- Optional: `staticcheck` (used by lint target when installed)

### Build and Test

```bash
make dev            # quick dev build
make build          # optimized stripped binary
make test           # go test ./... -count=1
make test-verbose   # verbose + race detector
make lint           # go vet + staticcheck (if installed)
make clean
```

## Domain-First Architecture

vpsm is organized by resource domain so contributors can focus on one area without navigating unrelated code.

### Directory Intent

- `cmd/commands/<domain>/...`: CLI commands for one domain
- `internal/<domain>/domain/...`: domain types and interfaces
- `internal/<domain>/providers/...`: provider implementations and registry for that domain
- `internal/<domain>/services/...`: domain business logic
- `internal/<domain>/tui/...`: domain TUI flows
- `internal/platform/...`: shared cross-domain code only

### Why This Structure

- DNS contributors should mostly touch DNS paths.
- Server contributors should mostly touch server paths.
- Shared platform code stays small and stable.

## Provider Architecture

Providers are modeled per domain capability, not as one giant interface.

- Server capabilities live in server-domain provider interfaces.
- SSH key capabilities live in sshkey-domain provider interfaces.
- Future DNS/volume capabilities should live in their own domain interfaces.
- Shared provider internals (auth/client/retry/cache) should live in platform packages.

This keeps interfaces focused and avoids cross-domain coupling.

## Contribution Scope

Keep changes scoped to one domain whenever possible.

Examples:
- Server feature: prefer `cmd/commands/server/...` and `internal/server/...`
- SSH key feature: prefer `cmd/commands/sshkey/...` and `internal/sshkey/...`
- DNS feature (future): prefer `cmd/commands/dns/...` and `internal/dns/...`

Only change `internal/platform/...` when the change is truly cross-domain.

## Coding Guidelines

- Keep non-CLI logic in `internal/...`
- Keep domain types as pure data where possible
- Keep command handlers thin
- Wrap errors with context using `%w`
- Use `cmd.OutOrStdout()` and `cmd.ErrOrStderr()` in commands
- Do not call `os.Exit` from subcommands
- Avoid `log.Fatal` / `log.Println`

## Testing Guidelines

- Co-locate tests with source files
- Use Go’s `testing` package
- Prefer table-driven tests for scenario coverage
- Use `cmp.Diff` for structural comparisons where useful
- Use `httptest.Server` for provider API tests
- Use `t.Cleanup()` for teardown
- Always run `make test` before opening a PR

## Adding a New Domain

1. Add `cmd/commands/<domain>/`
2. Add `internal/<domain>/domain/` interfaces and types
3. Add `internal/<domain>/providers/` registry and provider implementations
4. Add `internal/<domain>/services/` as needed
5. Add `internal/<domain>/tui/` flows if interactive UX is needed
6. Register built-in providers for that domain in root bootstrap
7. Add tests for command behavior and provider behavior

## Adding Domain Support to an Existing Provider

1. Implement the domain interface in `internal/<domain>/providers/<provider>/...`
2. Reuse shared provider core from `internal/platform/providers/<provider>/...` when possible
3. Register in that domain registry
4. Add tests for:
- happy path
- error mapping (`not found`, `unauthorized`, `rate limited`, `conflict`)
- unsupported/disabled capability behavior where applicable

## Pull Request Checklist

- Feature is scoped to the correct domain
- Interfaces remain domain-focused (no unrelated methods)
- Tests added or updated
- `make test` passes
- `make lint` passes (or note local unavailability)
- Command help/output updated if behavior changed

## Contributor Goal

A contributor interested in DNS should be able to be productive by reading mostly DNS code.  
Use that principle to guide structural and interface decisions.
