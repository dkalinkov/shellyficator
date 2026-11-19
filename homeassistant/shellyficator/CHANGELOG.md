# Changelog

Release notes of the Shellyficator version each app version runs. Full notes and
downloads: <https://github.com/dkalinkov/shellyficator/releases>

## 0.6.1

- Fix the Home Assistant app staying in "starting" state: its image health check now reaches the app.

## 0.6.0

- Make this repository a Home Assistant app repository, with an experimental Shellyficator app for `amd64` and `arm64`.
- Speed up subnet scans by skipping addresses that don't accept connections.
- **Discover** rescans subnets where a scan found devices; manage them under **Advanced discovery**.
- The Home Assistant app image is served only through Home Assistant, which signs users in. Shellyficator's own login, Authelia sign-in and data admin are off there.

## 0.5.0

- Publish a Home Assistant app image, `ghcr.io/dkalinkov/shellyficator-ha`, for `amd64` and `arm64`. It takes ownership of Home Assistant's root-owned `/data` mount, then runs unprivileged.
- Read settings from a JSON options file named by `SHELLY_OPTIONS_FILE`, such as the `options.json` that Home Assistant writes for apps.
- Attach each release's build provenance as a Sigstore bundle, so archives can be verified without GitHub's attestation API.
- Keep empty configuration keys from merging their settings into another path when comparing configurations.
