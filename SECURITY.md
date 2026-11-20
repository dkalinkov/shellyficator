# Security

Use the latest release on a network with trusted operators. The project
currently maintains one release line.

## Reporting a vulnerability

Report privately through
[Security → Report a vulnerability](https://github.com/dkalinkov/shellyficator/security/advisories/new).
Include the version, impact, and a minimal reproduction using synthetic data.
Rotate any exposed credentials.

## Verifying releases

Releases are immutable, tags are signed, and builds carry provenance attestations.
[Verification commands](docs/wiki/releases.md#verifying-a-release)

## Deployment considerations

Follow the [remote access and backup guidance](docs/wiki/deployment.md). Protect
data, backups, and credentials. Redacted snapshots may still contain private
network metadata.
