# Layered application refactor

Agreed 2026-10-03. Keep one binary and the embedded PocketBase/SQLite runtime.
Preserve existing routes and behavior while making the same application operations
available to HTML/HTMX handlers, background work, and future agent endpoints.

## Boundaries

| Package | Responsibility |
| --- | --- |
| `internal/bootstrap` | Typed process configuration, CLI commands, concrete dependency assembly, and serving/shutdown lifecycle. |
| `internal/domain` | Shared device, identity, reading, snapshot, comparison, organization, progress, and result types; pure business rules. Standard-library dependencies only. |
| `internal/devices` | Device queries and workflows, discovery, shared refresh coordination, cancellation, capture, baseline approval, and local metadata changes. Consumes small repository/client/discovery interfaces. |
| `internal/inventory` | PocketBase queries, record mapping, and transactional persistence. Returns domain types. |
| `internal/shelly` | Device HTTP and LAN discovery adapters, wire formats, authentication, identity validation, redaction, and network retries. Returns domain types. |
| `internal/accounts` | Operator authentication and account bootstrap services consuming a repository interface. |
| `internal/accountstore` | PocketBase account storage, atomic bootstrap, and token handling. |
| `internal/oidc` | Provider discovery, PKCE authorization, and ID-token/userinfo validation; returns external identities for existing-operator linking. |
| `internal/web` | Routes, request parsing, cookies/CSRF, HTTP errors and redirects, view builders, and full-page/partial template rendering. |
| Future API adapter | Explicit authenticated JSON commands/queries that call the same services. Its public request/response schemas belong to the API adapter. |

The domain must not import services, database adapters, HTTP frameworks, or
presentation. Services must not import concrete persistence or network adapters.
Production web code must not query PocketBase records or contact devices directly.
Page/partial models own formatted labels, colors, and display values; templates
consume explicit fields instead of generic database getters. Full pages and HTMX
partials share view builders. Composition and shutdown belong to bootstrap.
Bootstrap may import the concrete layers; they must not import bootstrap. The
root executable owns embedded/linker build metadata, launches the configured
application, and reports fatal errors. Bootstrap configuration types stay outside
the business domain.

## Implementation sequence

1. Add shared domain types, errors, comparison/organization rules, and health
   transitions. Define adapter contracts before parallel implementation.
2. In parallel, migrate persistence to domain mappings and atomic operations;
   migrate device services to interfaces and add single-device workflows;
   migrate handlers/templates/tests to services and explicit view models.
3. Extract account persistence/bootstrap from HTTP handlers, migrate the Shelly
   adapter to domain readings, and wire dependencies in startup.
4. Integrate, independently review boundaries and concurrency, fix findings,
   and complete behavioral/build verification. Record completion here.

## Required invariants

- Identity is the reported device ID/MAC; an address must never rebind a device.
- Ordinary misses retain cached readings. Offline requires three failures and
  90 seconds without validated contact; identity mismatch is immediate.
- Inventory readers see a completed refresh pass; cancellation drains device
  work before storage shuts down and must not create a device failure.
- Status/configuration availability and read times remain independent. Preserve
  missing, null, false, zero, stale, and stateless values.
- Automatic/status refreshes do not create snapshots or approve baselines.
- Capture and approval use fresh reads; capture plus baseline selection remains
  atomic. Rediscovery preserves local metadata and history.
- Room/tag canonicalization and channel-note validation remain atomic with their
  writes. Preserve notes when editing organization or polling readings.
- Preserve route authentication, CSRF, PocketBase built-in routes, existing HTML,
  filtering/grouping, HTMX replacement URLs, and browser interaction state.

## Verification

Use existing regression suites plus focused service and domain tests for the new
boundaries, failure propagation, and cancellation. Run `go test ./...`,
`go test -race ./...`, `go vet ./...`, `make build`, template formatting checks,
and `git diff --check`. Check a temporary-database browser session covering
setup/login, inventory/detail, metadata/organization, and HTMX updates. Never use
the operator's persistent database or change live device configuration for tests.

## Scope

No schema migration, device writes, scheduling, agent endpoints, or new deployment
is part of this refactor. Add API schemas and long-running job contracts when
their callers and endpoint behavior are defined.

## Completion

Implemented the domain, adapter, service, and presentation boundaries above.
Persistence, device workflows, and web presentation were migrated by separate
subagents and independently reviewed. Startup supplies the concrete adapters and
drains device services on termination. `architecture_test.go` enforces package
imports; fake-interface tests exercise services and HTTP error mapping without
PocketBase.

Regression coverage preserves completed refresh publication, cached readings,
identity checks, caller/shutdown cancellation, fresh capture/approval, atomic
baseline writes, local metadata, authentication, and missing/null/false/zero
settings. Read failures and approval-save failures have distinct domain error
markers so HTTP adapters retain the appropriate responses.

Passed `go test ./...`, `go test -race ./...`, `go vet ./...`, `make build`,
`make format-check`, and `git diff --check` on the integrated implementation.

Temporary-database Playwright checks passed setup/login, inventory/detail,
unfinished-note preservation through status refresh, metadata saves, capture and
approval, filtered/grouped HTMX room edits, refresh replacement URLs, and preserved
channel notes. Inventory and detail fit at 320px and 1440px without horizontal
overflow; the browser reported no console errors or warnings. Fixtures used a
local simulated device and changed no live device settings.

The final built binary also passed a restart against the temporary database,
retaining notes, organization, and the PocketBase health route. Temporary servers
and fixture files were removed. The user approved a local commit after reviewing
the completed refactor.

The subsequent bootstrap extraction moves environment loading, typed process
configuration, CLI registration, dependency assembly, and lifecycle hooks into
`internal/bootstrap`. Serving dependencies remain lazy; version and setup do not
create device services. The web adapter receives version and cookie settings as
typed options. Shutdown and startup failures close admission and drain device
work before PocketBase releases storage. The root executable retains build
metadata and error reporting, and dependency checks enforce its thin boundary.

The extraction passed independent review, `go test ./...`, `go test -race ./...`,
`go vet ./...`, `make build`, formatting checks, and diff checks. Integration tests
cover dotenv precedence, setup/version isolation, secure cookies, and cancellation
of active refresh work during termination and failed startup. The built binary
also passed temporary-database setup/login, occupied-listener failure, serving,
SIGTERM shutdown, and restart checks. Temporary processes and fixtures were removed.
