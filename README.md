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

A local dashboard for Shelly Gen2+ devices. Run it as a **Home Assistant app**
or **self-host it** on your network.

[Install in Home Assistant](#home-assistant) · [Self-host](#self-hosted) ·
[Documentation](docs/wiki/README.md) ·
[Contributing](CONTRIBUTING.md) · [Changelog](CHANGELOG.md)

## Home Assistant

Install Shellyficator as an experimental app on Home Assistant OS (amd64 or
arm64). It opens inside Home Assistant and uses your Home Assistant sign-in.

[![Add the Shellyficator repository to Home Assistant](https://my.home-assistant.io/badges/supervisor_add_addon_repository.svg)](https://my.home-assistant.io/redirect/supervisor_add_addon_repository/?repository_url=https%3A%2F%2Fgithub.com%2Fdkalinkov%2Fshellyficator)

Add the repository, install and start **Shellyficator** from the app store, then
choose **Open Web UI** or **Show in sidebar**. [Home Assistant setup](docs/wiki/home-assistant.md)

## Self-hosted

Run Shellyficator on Linux as a binary or Docker container. The standalone
dashboard uses its own login with optional OpenID Connect. Use this mode if you
run Home Assistant Container or want an independent deployment.

[Linux downloads](https://github.com/dkalinkov/shellyficator/releases) ·
[Docker setup](docs/wiki/deployment.md#linux-container) ·
[OpenID Connect setup](docs/wiki/configuration.md#openid-connect)

### Quick start

To run from source, install Go 1.27, Node.js 24, and GNU Make 4.3+. Set
`SHELLY_DEVICE_PASSWORD` in `.env` if your devices use a shared password.

```sh
git clone https://github.com/dkalinkov/shellyficator.git
cd shellyficator
cp .env.example .env
make run
```

Open [localhost:8090](http://localhost:8090), create an account, and choose
**Discover**. See [first login and baseline setup](docs/wiki/getting-started.md).

## What it does

- Discover devices by mDNS, LAN neighbor cache, host, or subnet.
- Organize devices with rooms, tags, and notes. Inspect live and saved readings.
- Compare settings with an approved baseline and choose which changes raise
  warnings. [Snapshot coverage](docs/wiki/usage.md#coverage-limits)
- Confirm firmware updates and verify the installed version after restart.

This early-development project is independent of Shelly. See the [roadmap](docs/wiki/roadmap.md).

## Build and develop

Run `make build` for the binary, `make dev` to rebuild on changes, and `make test`
for tests and vet. See the [development guide](docs/wiki/development.md).

File bugs and feature requests on [GitHub Issues](https://github.com/dkalinkov/shellyficator/issues)
and report security issues privately. [Security policy](SECURITY.md)

## License

[MIT](LICENSE). Vendored HTMX and fonts retain their own licenses in
[`internal/web/static`](internal/web/static). See the [brand guide](docs/brand/README.md) for artwork provenance.
