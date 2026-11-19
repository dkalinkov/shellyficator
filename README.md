<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="docs/brand/shellyficator-logo.svg">
    <source media="(prefers-color-scheme: light)" srcset="docs/brand/shellyficator-logo-light.svg">
    <img src="docs/brand/shellyficator-logo-light.svg" width="732" alt="Shellyficator">
  </picture>
</p>

<p align="center">
  <a href="https://github.com/dkalinkov/shellyficator/actions/workflows/ci.yml"><img src="https://github.com/dkalinkov/shellyficator/actions/workflows/ci.yml/badge.svg?branch=main" alt="CI"></a>
  <a href="https://github.com/dkalinkov/shellyficator/releases"><img src="https://img.shields.io/github/v/release/dkalinkov/shellyficator?sort=semver&amp;include_prereleases" alt="Latest release"></a>
  <a href="https://scorecard.dev/viewer/?uri=github.com/dkalinkov/shellyficator"><img src="https://api.scorecard.dev/projects/github.com/dkalinkov/shellyficator/badge" alt="OpenSSF Scorecard"></a>
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

Multi-arch images (`linux/amd64`, `linux/arm64`) are published to GitHub
Container Registry with each [release](https://github.com/dkalinkov/shellyficator/releases):

```sh
docker run -d --name shellyficator --restart unless-stopped \
  --env-file .env \
  --network=host \
  -v shellyficator-data:/data \
  ghcr.io/dkalinkov/shellyficator:latest serve --http=127.0.0.1:8090 --dir=/data
```

This binds to the host's loopback interface. For remote access, configure the
listen address and HTTPS proxy deliberately. Host networking helps LAN discovery
on native Linux; use `make run` for Docker Desktop development. Pin an exact
version tag in long-running deployments, preserve the volume across upgrades,
and see [deployment and backups](docs/wiki/deployment.md). `make docker` builds
the image locally as `shellyficator:local`.

## Home Assistant

On Home Assistant OS and Supervised installations, Shellyficator is available as
an experimental app for `amd64` and `arm64`:

[![Add the Shellyficator repository to your Home Assistant.](https://my.home-assistant.io/badges/supervisor_add_addon_repository.svg)](https://my.home-assistant.io/redirect/supervisor_add_addon_repository/?repository_url=https%3A%2F%2Fgithub.com%2Fdkalinkov%2Fshellyficator)

It opens in its own tab on port 8090 with its own login. See
[Home Assistant](docs/wiki/home-assistant.md) for setup and limitations.

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
