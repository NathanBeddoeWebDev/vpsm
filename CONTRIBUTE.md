# Contribute to vpsm

vpsm uses a domain-first architecture so contributors can focus on one resource area at a time.

## Core Principle

Keep code organized by resource domain:

- `server`
- `dns` (planned)
- `volume` (planned)
- `database` (planned)

Shared cross-domain code belongs in `internal/platform/...`.

## Where to Add Code

- `cmd/commands/<domain>/...`: CLI commands
- `internal/<domain>/domain/...`: types and interfaces
- `internal/<domain>/providers/...`: provider implementations + registry
- `internal/<domain>/services/...`: business logic
- `internal/<domain>/tui/...`: domain UI flows

## Provider Rule

Providers are registered per domain capability (server, ssh-key, dns, etc.), not through one monolithic interface.

Use shared provider internals (auth/client/retry/cache) from platform packages when needed.

## Contributor Focus

If you are contributing to DNS, you should mostly work in:

- `cmd/commands/dns/...`
- `internal/dns/...`

Only touch shared/platform code when the change is truly cross-domain.

See `CONTRIBUTING.md` for full setup, testing, and PR workflow.
