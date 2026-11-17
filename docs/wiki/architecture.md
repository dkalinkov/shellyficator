# Architecture

Shellyficator is one Go process embedding PocketBase, SQLite-backed persistence,
HTML templates, and static assets. HTMX and small JavaScript modules update the
server-rendered UI. Tailwind CSS and daisyUI are build-time dependencies.

## Request flow

A browser request enters `internal/web`, which authenticates the operator, checks
CSRF on writes, parses input, and calls an application service. Services coordinate
repository and device-client interfaces. Persistence and network adapters return
domain values; web view builders turn these into display models and templates.

| Package | Responsibility |
| --- | --- |
| Root executable | Build metadata and application entry point |
| `internal/bootstrap` | Environment, CLI, dependency wiring, startup and shutdown |
| `internal/domain` | Shared types and pure comparison, health, and watch rules |
| `internal/devices` | Discovery, refresh coordination, snapshots, baselines, organization, firmware workflows |
| `internal/inventory` | PocketBase persistence and transactional record mapping |
| `internal/shelly` | Device HTTP/RPC, authentication, identity checks, redaction, LAN discovery |
| `internal/accounts` | Operator setup and authentication services |
| `internal/accountstore` | Operator storage, tokens, and OIDC identity linking |
| `internal/oidc` | Provider discovery, PKCE, token and user-info validation |
| `internal/web` | Routes, cookies, CSRF, view models, templates, static files |
| `migrations` | Dated database schema migrations |

## Storage and workflow rules

Device identity survives address changes. Rediscovery preserves local metadata and
history. Status, current configuration, saved captures, and approved baselines are
separate concepts: polling updates readings, capture saves history, and approval
explicitly chooses a fresh baseline.

Refresh coordination publishes completed results shared by viewers. Failed reads
retain previous values and their timestamps. Shutdown stops admission and drains
active work before storage closes. Firmware installation is serialized, with
persistent attempt state and explicit version verification.

Operators and PocketBase data administrators are separate accounts. OIDC links
an external identity to an existing operator rather than provisioning accounts.
The data admin can modify records outside application invariants.

## Boundaries enforced in code

The domain uses the standard library only. Services depend on interfaces rather
than concrete database/network adapters. Production web handlers do not query
PocketBase records or contact devices directly. Bootstrap composes concrete
implementations. `architecture_test.go` checks package import boundaries.

Full pages and HTMX partials share view builders. Templates consume explicit
fields rather than generic database getters. Concrete layers must not import
bootstrap; typed process configuration stays outside the business domain.
Serving dependencies are lazy: version and setup commands do not create device
services. Startup failures drain device work before releasing storage, just as
normal shutdown does.

## Invariants to preserve

- An address must never rebind a device to a different reported ID/MAC.
- Offline status requires three failed checks and 90 seconds without validated
  contact; identity mismatch is an immediate error.
- Readers see completed refresh passes. Cancellation drains active work without
  recording a device failure caused only by cancellation.
- Status and configuration availability and timestamps remain independent.
  Preserve missing, null, false, zero, stale, and stateless values.
- Capture and approval use fresh reads; capture plus baseline selection is atomic.
  Automatic refreshes neither save snapshots nor approve baselines.
- Room/tag canonicalization and channel-note validation are atomic with their
  writes. Organization changes and polling preserve notes.
- Preserve authentication, CSRF, PocketBase built-in routes, filter/group URLs,
  and browser interaction state across full-page and HTMX responses.

The HTML interface is the current application adapter. A future authenticated
JSON API should call the same services and own its public request/response
schemas; agent endpoints and job contracts remain future work.

See [development](development.md) for checks and contributor guidance.
