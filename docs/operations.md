# Configuration and releases

## Configuration

The app loads optional `.env.local` and `.env` files from the working directory for all commands. Precedence: existing environment (including empty values), then `.env.local`, then `.env`. Quotes, comments, and `export KEY=value` are supported. Missing files are allowed; invalid or unreadable files stop startup without logging their contents. These files are ignored by Git and excluded from Docker images.

See [`.env.example`](../.env.example) for available variables:

- `SHELLY_DEVICE_PASSWORD`: shared password for protected devices, username `admin`.
- `SHELLY_COOKIE_SECURE=true`: use behind an HTTPS reverse proxy.
- `SHELLY_ADMIN_EMAIL` and `SHELLY_ADMIN_PASSWORD` (12+ characters): initial accounts for `go run . setup --dir=./pb_data`. Data admin has a separate account with the same initial credentials.

Restart after configuration changes; `make dev` restarts automatically. Choose **Discover**; use **Advanced discovery** with an IP, hostname, or private subnet if devices are missing.

### Authelia / OpenID Connect

Set all four variables to show **Sign in with Authelia** alongside password login:

```dotenv
SHELLY_OIDC_ISSUER=https://auth.example.com
SHELLY_OIDC_CLIENT_ID=shellyficator
SHELLY_OIDC_CLIENT_SECRET=<client secret>
SHELLY_OIDC_REDIRECT_URL=https://shellyficator.example.com/auth/oidc/callback
```

Register a confidential client using `authorization_code`, response type `code`,
scopes `openid`, `profile`, `email`, PKCE `S256`, and token authentication
`client_secret_basic`. Register the exact redirect URL above. HTTPS is required;
HTTP loopback URLs are supported for local testing. Set `SHELLY_COOKIE_SECURE=true`
when serving behind an HTTPS proxy. Restrict the client's authorization policy in
Authelia to the permitted users/group; successful provider login grants dashboard
access only to an existing operator.

On first SSO login, the provider's verified email must match an existing verified
operator email. Its issuer and subject are then saved on that operator; subsequent
SSO logins use this stable identity even if the provider email changes. A different
subject or issuer cannot replace an existing link. Unknown users and data-admin-only
accounts cannot sign in or create operators through SSO. Password login and the
separate PocketBase data-admin account remain available.

To deliberately relink an account after changing identity provider, clear both
`oidc_issuer` and `oidc_subject` on the operator through the data admin, then sign
in with the new identity's matching verified email. These fields are hidden from
API responses. Do not clear them as part of an ordinary password/email change.

Provider discovery happens when SSO is used, so its availability does not block
startup or password login. Pending sign-ins expire after ten minutes and are lost
on restart. Local logout ends only the Shellyficator session; sign out of Authelia
separately to end its shared session. Its access-policy changes apply at the next
SSO login; existing Shellyficator sessions retain their usual lifetime.

`make dev`, `make run`, and `make build` rebuild CSS and copy vendor assets only when their inputs change or outputs are missing. Generated `internal/web/static/app.css` is ignored by Git. Development builds omit timestamps by default to reuse cached Go builds; set `BUILD_DATE` to include one.

Use DaisyUI components for controls, cards, badges, and tables, and Tailwind utilities in templates for layout and responsive styling. [`ui/styles.css`](../ui/styles.css) contains only shared typography, keyboard focus outlines, the SVG icon utility, and the cloud animation. Tailwind also scans browser JavaScript and the complete status classes in `internal/web/templates.go`, so dynamically created controls and status colors are included in builds.

## Docker

The [README example](../README.md#docker-on-linux) passes `.env` through `--env-file`. Use unquoted `KEY=value` entries without `export`, or omit the option if there is no file. For full dotenv syntax and `.env.local` overrides, mount the files read-only under `/data`.

The container runs as UID/GID 10001; bind mounts must be writable by that user. Preserve the data volume and reuse it when upgrading an existing install. Change `--http` for LAN access or use a reverse proxy. Health check: `/healthz`. With bridge networking, publish port 8090 and supply the LAN range under **Advanced discovery**.

## Releases

[VERSION](../VERSION) is the version source. Increment the patch for fixes and the minor for features; while the app is `0.x`, breaking changes also increment the minor. Record changes in [CHANGELOG.md](../CHANGELOG.md).

Update those files, run `make test`, commit, and create an annotated tag matching `VERSION`. Build from the clean tagged commit:

```sh
git tag -a "v$(cat VERSION)" -m "Shellyficator $(cat VERSION)"
make build
make docker
```

`make build` and `make docker` report the base version for a clean matching tag, otherwise append `-dev`. `--version` also reports the commit, dirty marker, and UTC build time; the UI shows the app version. `make dev`, plain Go builds, and Docker builds without metadata report a development version. Docker images use `shellyficator:local` and `shellyficator:<app-version>` tags.

Database schema changes use separate dated Go migrations. Preserve app data across upgrades.
