# Security

Shellyficator is an early-development application intended for trusted operators
on a local network. Use the latest project version; no maintained older release
branches or response-time guarantees are currently defined.

## Reporting a vulnerability

Do not post credentials, private device data, or exploit details in a public issue.
If GitHub shows **Security → Report a vulnerability** for this repository, use that
private reporting channel. Otherwise, open an issue asking the maintainer to
establish a private channel, without sensitive details. Private vulnerability
reporting availability depends on repository settings.

Include the affected version, impact, and a minimal reproduction using synthetic
data through the private channel. Rotate any exposed credentials independently of
removing them from Git history.

## Deployment considerations

Complete setup before allowing remote access, use HTTPS and secure cookies, and
restrict the data-admin interface. Protect `pb_data/`, backups, dotenv files, and
OIDC/device credentials. Known snapshot secrets are redacted, but private network
metadata can remain. See [deployment](docs/wiki/deployment.md) for details.
