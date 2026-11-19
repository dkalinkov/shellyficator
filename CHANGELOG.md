# Changelog

## Unreleased

- Fix the Home Assistant app staying in "starting" state: its image health check now reaches the app.

## 0.6.0 — 2026-11-19

- Make this repository a Home Assistant app repository, with an experimental Shellyficator app for `amd64` and `arm64`.
- Speed up subnet scans by skipping addresses that don't accept connections.
- **Discover** rescans subnets where a scan found devices; manage them under **Advanced discovery**.
- The Home Assistant app image is served only through Home Assistant, which signs users in. Shellyficator's own login, Authelia sign-in and data admin are off there.

## 0.5.0 — 2026-11-19

- Publish a Home Assistant app image, `ghcr.io/dkalinkov/shellyficator-ha`, for `amd64` and `arm64`. It takes ownership of Home Assistant's root-owned `/data` mount, then runs unprivileged.
- Read settings from a JSON options file named by `SHELLY_OPTIONS_FILE`, such as the `options.json` that Home Assistant writes for apps.
- Attach each release's build provenance as a Sigstore bundle, so archives can be verified without GitHub's attestation API.
- Keep empty configuration keys from merging their settings into another path when comparing configurations.

## 0.4.0 — 2026-11-19

- Publish releases from signed tags through GitHub Actions: Linux amd64/arm64 archives with checksums, SBOMs, and build provenance, and multi-arch container images at `ghcr.io/dkalinkov/shellyficator`.
- Publish an `edge` image from `main` after CI passes.
- Show stable/beta firmware offers as compact Status icons, with detailed tooltips and manual checks.
- Add confirmed single-device and selected-device firmware installation, serial execution, before/after configuration captures, and version verification after reboot.
- Keep progress and results across page reloads; detect unchanged firmware and interrupted verification without automatically retrying update commands.

## 0.3.0 — 2026-11-11

- Animate the header mascot with a smooth wink, a 900 ms open-eye hold, and a 3.5-second loop while keeping its head still.
- Give the login mascot a wink, playful lean, and little hop, with quiet rests between performances.
- Respect reduced motion and pause hidden or offscreen animations; retain static artwork when animation is unavailable.
- Include transparent GIF exports and a reproducible mascot asset builder.

## 0.2.0 — 2026-11-10

- Add optional Authelia/OpenID Connect login using authorization code flow, PKCE and validated ID tokens.
- Link verified external identities to existing operators without creating accounts or changing password and data-admin access.

## 0.1.0 — 2026-10-01

- Initial tracked version of Shellyficator: authenticated inventory, discovery, device/channel notes, configuration captures, baselines, and drift comparison.
- Use the Shellyficator name for the Go module, CLI, binaries, npm tooling, and Docker image/container/data volume.
- Show the app version in the CLI and UI; stamp native and Docker builds with their commit and build time. Clean matching Git tags identify release builds.
- Load optional `.env.local` and `.env` files at startup, preserving existing environment values. Development restarts pick up configuration changes.
