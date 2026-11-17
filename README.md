<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="docs/brand/shellyficator-logo.svg">
    <source media="(prefers-color-scheme: light)" srcset="docs/brand/shellyficator-logo-light.svg">
    <img src="docs/brand/shellyficator-logo-light.svg" width="732" alt="Shellyficator">
  </picture>
</p>

# Shellyficator

**Know what changed on your Shelly devices.**

A self-hosted dashboard for discovering Shelly Gen2+ devices, organizing your
inventory, and comparing their configuration with an approved baseline. Built
for local networks, with a Go binary, embedded web assets, and SQLite storage.

[Get started](#quick-start) · [Documentation](docs/wiki/README.md) ·
[Contributing](CONTRIBUTING.md) · [Changelog](CHANGELOG.md)

## What it does

- Discovers devices using mDNS, cached LAN neighbors, and explicit host/subnet scans.
- Organizes devices and inputs with rooms, tags, filters, and notes.
- Shows live status, stale readings, connectivity, and firmware availability.
- Captures configuration and compares it with an explicitly approved baseline.
- Lets you choose which configuration changes produce warnings, globally or per device.
- Installs selected firmware updates serially, with confirmation, configuration
  captures, and verification of the installed version after restart.
- Supports password login and optional OpenID Connect login for existing operators.

Shellyficator is in **early development**. Gen1 devices, switch/input configuration
editing, scheduled issue reports, and device password changes are not implemented.
See the [roadmap](docs/wiki/roadmap.md) for planned work. This is an
independent project and is not affiliated with Shelly.

## Quick start

Install **Go 1.27**, **Node.js 24**, and **GNU Make 4.3+**. The server needs network
access to your devices. Node.js is used to build assets; the compiled application
runs without it.

```sh
git clone https://github.com/dkalinkov/shellyficator.git
cd shellyficator
cp .env.example .env
make run
```

Edit `.env` before starting if your devices share a password. Open
[localhost:8090](http://localhost:8090), create your account with a password of at
least 12 characters, and choose **Discover**. First-run setup also creates a
separate data-admin account at `/_/` if none exists.

Open a device, check its settings and physical controls, then approve its current
configuration as a baseline. Later reads show differences; they never approve a
new baseline automatically. See [usage and monitoring](docs/wiki/usage.md).

## Build and develop

```sh
make build
./bin/shellyficator --version
./bin/shellyficator serve --http=127.0.0.1:8090 --dir=./pb_data
```

Use `make dev` for automatic rebuilds and restarts. Run `make css` before
`make test` on a fresh checkout because the Go binary embeds the generated CSS.
See [development](docs/wiki/development.md) for checks and project structure.

## Docker on Linux

```sh
make docker
docker run --rm --name shellyficator \
  --env-file .env \
  --network=host \
  -v shellyficator-data:/data \
  shellyficator:local serve --http=127.0.0.1:8090 --dir=/data
```

This binds to the host's loopback interface. For remote access, configure the
listen address and HTTPS proxy deliberately. Host networking helps LAN discovery
on native Linux; use `make run` for Docker Desktop development. Preserve the
volume across upgrades. See [deployment and backups](docs/wiki/deployment.md).

## Monitoring boundaries

Monitoring covers configuration returned by `Shelly.GetConfig`. Webhook action
definitions, schedules, script source, and external automations are not captured.
Known password fields and URL credentials are redacted, but saved configuration
can still contain private network information. A matching baseline does not prove
that every physical button action works or explain who changed a setting.

## Documentation

Start with the [repository wiki](docs/wiki/README.md): setup, configuration,
discovery, everyday use, firmware updates, architecture, and development.
[Configuration](docs/wiki/configuration.md) includes OIDC integration;
[releases](docs/wiki/releases.md) covers versioning and release procedures.

For bugs and feature requests, use
[GitHub Issues](https://github.com/dkalinkov/shellyficator/issues). Include the app
version and sanitized reproduction steps. See [SECURITY.md](SECURITY.md) for
sensitive reports.

## License

[MIT](LICENSE). Vendored HTMX and fonts retain their own licenses in
[`internal/web/static`](internal/web/static). Artwork provenance is documented in
[the brand guide](docs/brand/README.md).
