# Security

Shellyficator is an early-development application intended for trusted operators
on a local network. Use the latest project version; no maintained older release
branches or response-time guarantees are currently defined.

## Reporting a vulnerability

Do not post credentials, private device data, or exploit details in a public issue.
Report vulnerabilities privately through
[**Security → Report a vulnerability**](https://github.com/dkalinkov/shellyficator/security/advisories/new).

Include the affected version, impact, and a minimal reproduction using synthetic
data through the private channel. Rotate any exposed credentials independently of
removing them from Git history.

## Verifying releases

Release tags are signed, and published releases are immutable. Release archives
and container images carry build provenance attestations from the release
workflow. See [verifying a release](docs/wiki/releases.md#verifying-a-release).

## Deployment considerations

Complete setup before allowing remote access, use HTTPS and secure cookies, and
restrict the data-admin interface. Protect `pb_data/`, backups, dotenv files, and
OIDC/device credentials. Known snapshot secrets are redacted, but private network
metadata can remain. See [deployment](docs/wiki/deployment.md) for details.
