# Changelog

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
