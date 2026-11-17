# Configuration

The app loads optional `.env.local` and `.env` files from its working directory
for every command. Precedence is process environment (including empty values),
then `.env.local`, then `.env`. Restart after changes. Invalid or unreadable
files stop startup without logging their contents. Missing files are allowed.
Quotes, comments, and `export KEY=value` are supported. Dotenv files are ignored
by Git and excluded from Docker images.

Use [`.env.example`](../../.env.example) as a starting point. Keep credentials in
local files or your deployment's secret store.

| Variable | Purpose | Default when unset |
| --- | --- | --- |
| `SHELLY_DEVICE_PASSWORD` | Shared device password, using device username `admin` | Empty |
| `SHELLY_ADMIN_EMAIL` | Email for the explicit `setup` command | Empty |
| `SHELLY_ADMIN_PASSWORD` | Password for `setup`, at least 12 characters | Empty |
| `SHELLY_COOKIE_SECURE` | Set exactly `true` behind HTTPS to secure session cookies | False |
| `SHELLY_OIDC_ISSUER` | OIDC provider issuer URL | Disabled unless all four OIDC values are set |
| `SHELLY_OIDC_CLIENT_ID` | OIDC confidential client ID | Empty |
| `SHELLY_OIDC_CLIENT_SECRET` | OIDC client secret | Empty |
| `SHELLY_OIDC_REDIRECT_URL` | Exact callback URL, ending in `/auth/oidc/callback` | Empty |
| `TZ` | Process/container timezone | Runtime default; example uses UTC |

The shared device password is used for protected device requests. It does not
change passwords on devices. Individual stored device passwords and dashboard
credential management are not implemented.

## OpenID Connect

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

## Listen address and data directory

Set these with CLI flags, for example:

```sh
./bin/shellyficator serve --http=127.0.0.1:8090 --dir=./pb_data
```

The working directory determines dotenv discovery; `--dir` determines persistent
data storage. They are separate settings. See [deployment](deployment.md).
