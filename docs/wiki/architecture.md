# Architecture

One Go process embeds PocketBase, SQLite, and the HTML UI. HTMX updates
server-rendered fragments. Tailwind and daisyUI run at build time.

| Package | Responsibility |
| --- | --- |
| `internal/bootstrap` | Configuration, CLI, wiring, and lifecycle |
| `internal/domain` | Shared types and rules, using only the standard library |
| `internal/devices` | Device queries and workflows |
| `internal/inventory` | PocketBase inventory storage |
| `internal/shelly` | Discovery, device HTTP/RPC, identity, and redaction |
| `internal/accountstore` | Operator setup, authentication, and OIDC links in PocketBase |
| `internal/oidc` | Provider discovery, PKCE, and identity validation |
| `internal/web` | Routes, auth, views, templates, and browser assets |
| `migrations` | Dated schema changes |

## Boundaries enforced in code

Web handlers call services for device and storage operations. Bootstrap wires
the concrete adapters. Full pages and HTMX fragments share view builders.
`architecture_test.go` enforces package boundaries.

## Invariants to preserve

- Validate identity before updating a saved device. Rediscovery preserves
  metadata, captures, and baselines. Late reads may only update the device at the
  address they were issued against.
- Publish completed refresh passes. Preserve independent status and configuration
  timestamps and distinguish missing, null, false, zero, and stale values.
- Stop admitting work and drain before closing storage. Shutdown or caller
  cancellation must not record a device failure.
- Save snapshots only for captures, approvals, and firmware workflows. Save
  the fresh snapshot and baseline selection in one transaction when approving.
- Serialize firmware installation and verify the running version after a command.
- Validate room/tag and component-note edits in the same transaction as the write.
- Keep operator and PocketBase administrator accounts separate. OIDC links
  existing operators. The data admin can edit records directly.
- Preserve CSRF, built-in PocketBase routes, filter URLs, and browser state on
  HTMX updates.
