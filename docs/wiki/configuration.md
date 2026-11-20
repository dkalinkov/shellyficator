# Configuration

Copy [`.env.example`](../../.env.example) and keep credentials out of Git.
Settings precedence is process environment (including empty values),
`SHELLY_OPTIONS_FILE`, `.env.local`, then `.env`. Dotenv paths are relative to
the working directory and accept quotes and `export`. Missing files are allowed.
Invalid or unreadable files stop startup without logging their contents. Restart
to apply changes.

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
| `SHELLY_OPTIONS_FILE` | Flat JSON settings file | Not read |
| `TZ` | Process/container timezone | Runtime default; example uses UTC |

## Options file

Use a flat JSON object with lowercase keys and no `SHELLY_` prefix.
Home Assistant sets `SHELLY_OPTIONS_FILE` to `/data/options.json`.

```json
{ "device_password": "<shared device password>", "cookie_secure": false }
```

Numbers and booleans become strings. Empty strings and `null` are skipped.

## OpenID Connect

Set all four values to enable **Sign in with Authelia**.

```dotenv
SHELLY_OIDC_ISSUER=https://auth.example.com
SHELLY_OIDC_CLIENT_ID=shellyficator
SHELLY_OIDC_CLIENT_SECRET=<client secret>
SHELLY_OIDC_REDIRECT_URL=https://shellyficator.example.com/auth/oidc/callback
```

Register a confidential client with the exact callback URL, `authorization_code`,
response type `code`, scopes `openid profile email`, PKCE `S256`, and
`client_secret_basic`. Use HTTPS outside loopback and set `SHELLY_COOKIE_SECURE=true`
behind an HTTPS proxy. Restrict access in Authelia to permitted users or groups.

The provider's verified email must match an existing verified operator for the
first sign-in. Subsequent sign-ins use issuer and subject, even if the email
changes. To relink deliberately, clear both `oidc_issuer` and `oidc_subject`
through the data admin, then sign in with the new identity's matching verified email.

Keep the operator auth rule empty and PocketBase MFA disabled for dashboard
sign-in. PocketBase's own APIs support their native rules and MFA. Disabling
operator password auth leaves OIDC available.

Provider discovery happens on demand. Pending sign-ins expire after ten minutes
or a restart. Sign out of Authelia separately to end its session. Policy changes
take effect at the next OIDC sign-in.

## Listen address and data directory

Use `serve --http=127.0.0.1:8090 --dir=./pb_data`. Changing `--dir` leaves
dotenv lookup relative to the working directory. See [deployment](deployment.md).
