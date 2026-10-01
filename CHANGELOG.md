# Changelog

## 0.1.0 — 2026-10-01

- Initial tracked version of Shellyficator: authenticated inventory, discovery, device/channel notes, configuration captures, baselines, and drift comparison.
- Use the Shellyficator name for the Go module, CLI, binaries, npm tooling, and Docker image/container/data volume.
- Show the app version in the CLI and UI; stamp native and Docker builds with their commit and build time. Clean matching Git tags identify release builds.
- Load optional `.env.local` and `.env` files at startup, preserving existing environment values. Development restarts pick up configuration changes.
